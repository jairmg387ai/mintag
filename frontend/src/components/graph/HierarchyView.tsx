import { useState, useEffect, useCallback, useRef } from 'react'
import { hierarchy, tree } from 'd3-hierarchy'
import * as api from '../../api/client'
import type { GraphNeighbor } from '../../types'

// ---------------------------------------------------------------------------
// Expansion rules (same as TreeExplorer)
// ---------------------------------------------------------------------------

interface ExpandRule {
  relation: string
  direction: 'in' | 'out'
}

const EXPAND_RULES: Record<string, ExpandRule[]> = {
  portal: [
    { relation: 'belongs_to', direction: 'in' },
    { relation: 'exposes', direction: 'out' },
  ],
  menu_option: [{ relation: 'implemented_by', direction: 'out' }],
  repo: [
    { relation: 'has_app', direction: 'out' },
    { relation: 'consumes', direction: 'out' },
    { relation: 'calls', direction: 'out' },
  ],
  microfront: [{ relation: 'consumes', direction: 'out' }],
}

const KIND_COLORS: Record<string, string> = {
  portal: '#8b5cf6',
  menu_option: '#f59e0b',
  repo: '#6366f1',
  microfront: '#06b6d4',
  use_case: '#10b981',
  external_service: '#ef4444',
  messaging: '#ec4899',
}

// ---------------------------------------------------------------------------
// Tree node data
// ---------------------------------------------------------------------------

interface HNode {
  id: string
  nodeId: number
  label: string
  kind: string
  relation?: string
  isCycle: boolean
  children: HNode[] | null
  expanded: boolean
  loading: boolean
}

function makeHNode(nodeId: number, label: string, kind: string, id: string, relation?: string, isCycle = false): HNode {
  return { id, nodeId, label, kind, relation, isCycle, children: null, expanded: false, loading: false }
}

function isExpandable(node: HNode): boolean {
  return !node.isCycle && EXPAND_RULES[node.kind] !== undefined
}

async function fetchHChildren(node: HNode, ancestorIDs: Set<number>): Promise<HNode[]> {
  const rules = EXPAND_RULES[node.kind] ?? []
  const responses = await Promise.all(
    rules.map(rule =>
      api
        .graphNeighbors(node.nodeId, { relation: rule.relation, direction: rule.direction, limit: 500 })
        .then(r => r.neighbors)
        .catch(() => [] as GraphNeighbor[]),
    ),
  )
  const seen = new Set<string>()
  const children: HNode[] = []
  for (const neighbors of responses) {
    for (const nb of neighbors) {
      const dedupeKey = `${nb.relation}:${nb.node.id}`
      if (seen.has(dedupeKey)) continue
      seen.add(dedupeKey)
      const childId = `${node.id}/${nb.relation}:${nb.node.id}`
      const child = makeHNode(nb.node.id, nb.node.label || nb.node.key, nb.node.kind, childId, nb.relation)
      child.isCycle = ancestorIDs.has(nb.node.id)
      children.push(child)
    }
  }
  children.sort((a, b) => a.label.localeCompare(b.label))
  return children
}

// ---------------------------------------------------------------------------
// Flatten visible tree for d3-hierarchy
// ---------------------------------------------------------------------------

interface FlatNode {
  data: HNode
  children?: FlatNode[]
}

function buildVisible(node: HNode): FlatNode {
  const flat: FlatNode = { data: node }
  if (node.expanded && node.children && node.children.length > 0) {
    flat.children = node.children.map(buildVisible)
  }
  return flat
}

// ---------------------------------------------------------------------------
// SVG node dimensions
// ---------------------------------------------------------------------------

const NODE_W = 160
const NODE_H = 36
const NODE_SIZE_X = 52
const NODE_SIZE_Y = 200

// ---------------------------------------------------------------------------
// HierarchyView
// ---------------------------------------------------------------------------

export function HierarchyView({
  rootNodeId,
  namespace: _namespace,
  onSelectNode,
}: {
  rootNodeId: number
  namespace?: string
  onSelectNode: (id: number) => void
}) {
  const [root, setRoot] = useState<HNode | null>(null)
  const [, forceRender] = useState(0)
  const rerender = useCallback(() => forceRender(n => n + 1), [])

  const [viewport, setViewport] = useState({ tx: 40, ty: 200, scale: 1 })
  const svgRef = useRef<SVGSVGElement>(null)
  const dragRef = useRef<{ startX: number; startY: number; startTx: number; startTy: number } | null>(null)

  // Load root node and immediately expand one level
  useEffect(() => {
    setRoot(null)
    setViewport({ tx: 40, ty: 200, scale: 1 })

    api.graphNodeByID(rootNodeId).then(detail => {
      const n = detail.node
      const rootNode = makeHNode(n.id, n.label || n.key, n.kind, 'root')
      rootNode.expanded = true
      rootNode.loading = true
      setRoot(rootNode)

      fetchHChildren(rootNode, new Set([n.id])).then(children => {
        rootNode.children = children
        rootNode.loading = false
        forceRender(v => v + 1)
      }).catch(() => {
        rootNode.children = []
        rootNode.loading = false
        forceRender(v => v + 1)
      })
    }).catch(() => {
      // silently ignore load failure
    })
  }, [rootNodeId])

  const toggleNode = useCallback(async (node: HNode, ancestorIDs: Set<number>) => {
    if (node.expanded) {
      node.expanded = false
      rerender()
      return
    }
    node.expanded = true
    if (node.children === null && !node.loading) {
      node.loading = true
      rerender()
      try {
        node.children = await fetchHChildren(node, ancestorIDs)
      } catch {
        node.children = []
      } finally {
        node.loading = false
      }
    }
    rerender()
  }, [rerender])

  // Pan handlers
  const onMouseDown = useCallback((e: React.MouseEvent<SVGSVGElement>) => {
    if (e.button !== 0) return
    dragRef.current = { startX: e.clientX, startY: e.clientY, startTx: viewport.tx, startTy: viewport.ty }
    e.preventDefault()
  }, [viewport])

  const onMouseMove = useCallback((e: React.MouseEvent<SVGSVGElement>) => {
    if (!dragRef.current) return
    const dx = e.clientX - dragRef.current.startX
    const dy = e.clientY - dragRef.current.startY
    setViewport(v => ({ ...v, tx: dragRef.current!.startTx + dx, ty: dragRef.current!.startTy + dy }))
  }, [])

  const onMouseUp = useCallback(() => { dragRef.current = null }, [])

  // Zoom handler
  const onWheel = useCallback((e: React.WheelEvent<SVGSVGElement>) => {
    e.preventDefault()
    setViewport(v => {
      const delta = e.deltaY > 0 ? 0.9 : 1.1
      const newScale = Math.min(2.5, Math.max(0.3, v.scale * delta))
      return { ...v, scale: newScale }
    })
  }, [])

  if (!root) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: 'var(--fg3)', fontSize: '0.875rem' }}>
        Loading…
      </div>
    )
  }

  // Build d3 layout
  const flatRoot = buildVisible(root)
  const root3 = hierarchy(flatRoot, d => d.children ?? null)
  const treeLayout = tree<FlatNode>().nodeSize([NODE_SIZE_X, NODE_SIZE_Y])
  treeLayout(root3)

  const d3nodes = root3.descendants()
  const d3links = root3.links()

  // Collect ancestor sets per node path for toggleNode
  function getAncestors(d3node: typeof d3nodes[0]): Set<number> {
    const set = new Set<number>()
    let cur = d3node.parent
    while (cur) {
      set.add(cur.data.data.nodeId)
      cur = cur.parent
    }
    return set
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
      <div style={{ padding: '6px 14px', borderBottom: '1px solid var(--border)', flexShrink: 0, fontSize: '0.75rem', color: 'var(--fg3)' }}>
        Drag to pan · scroll to zoom · click to expand · Ctrl+click to open detail
      </div>
      <svg
        ref={svgRef}
        style={{ flex: 1, width: '100%', cursor: dragRef.current ? 'grabbing' : 'grab', display: 'block' }}
        onMouseDown={onMouseDown}
        onMouseMove={onMouseMove}
        onMouseUp={onMouseUp}
        onMouseLeave={onMouseUp}
        onWheel={onWheel}
      >
        <g transform={`translate(${viewport.tx},${viewport.ty}) scale(${viewport.scale})`}>
          {/* Links */}
          {d3links.map((link, i) => {
            const sx = (link.source as any).y + NODE_W / 2
            const sy = (link.source as any).x
            const dx = (link.target as any).y
            const dy = (link.target as any).x
            const mx = (sx + dx) / 2
            return (
              <path
                key={i}
                d={`M ${sx} ${sy} C ${mx} ${sy}, ${mx} ${dy}, ${dx} ${dy}`}
                style={{ fill: 'none', stroke: 'var(--border-strong)', strokeWidth: 1.5 }}
              />
            )
          })}

          {/* Nodes */}
          {d3nodes.map(d3n => {
            const hn = d3n.data.data
            const x = (d3n as any).y
            const y = (d3n as any).x
            const expandable = isExpandable(hn)
            const kindColor = KIND_COLORS[hn.kind] ?? '#888'
            const ancestors = getAncestors(d3n)
            const truncated = hn.label.length > 17 ? hn.label.slice(0, 16) + '…' : hn.label

            return (
              <g
                key={hn.id}
                transform={`translate(${x},${y - NODE_H / 2})`}
                style={{ cursor: 'pointer' }}
                onClick={e => {
                  if (e.ctrlKey || e.metaKey) {
                    onSelectNode(hn.nodeId)
                  } else if (expandable) {
                    const fullAncestors = new Set(ancestors)
                    fullAncestors.add(hn.nodeId)
                    toggleNode(hn, fullAncestors)
                  }
                }}
                onDoubleClick={() => onSelectNode(hn.nodeId)}
              >
                <rect
                  width={NODE_W}
                  height={NODE_H}
                  rx={6}
                  ry={6}
                  style={{ fill: 'var(--bg-surface)', stroke: kindColor + '99', strokeWidth: 1.5 }}
                />

                {/* Kind dot */}
                <circle cx={12} cy={NODE_H / 2} r={4} style={{ fill: kindColor }} />

                {/* Label — SVG text avoids foreignObject CSS-var issues */}
                <text
                  x={22}
                  y={NODE_H / 2}
                  dominantBaseline="middle"
                  style={{ fontSize: '12px', fontWeight: 500, fill: 'var(--fg1)', pointerEvents: 'none' }}
                >
                  {truncated}
                </text>

                {/* Chevron / loading indicator */}
                {hn.isCycle ? (
                  <text x={NODE_W - 14} y={NODE_H / 2} dominantBaseline="middle" style={{ fontSize: '10px', fill: '#f59e0b' }}>↺</text>
                ) : hn.loading ? (
                  <text x={NODE_W - 14} y={NODE_H / 2} dominantBaseline="middle" style={{ fontSize: '10px', fill: 'var(--fg3)' }}>…</text>
                ) : expandable ? (
                  <text x={NODE_W - 14} y={NODE_H / 2} dominantBaseline="middle" style={{ fontSize: '10px', fill: 'var(--fg3)' }}>
                    {hn.expanded ? '▾' : '▸'}
                  </text>
                ) : null}
              </g>
            )
          })}
        </g>
      </svg>
    </div>
  )
}
