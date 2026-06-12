import { useState, useEffect, useCallback } from 'react'
import { ChevronRight, ChevronDown, ListFilter, RotateCw, Repeat } from 'lucide-react'
import * as api from '../../api/client'
import type { GraphNode, GraphNeighbor, GraphNodeSearchResult } from '../../types'
import { Input } from '../ui/Input'
import { useDebounce } from '../../hooks/useDebounce'

// ---------------------------------------------------------------------------
// Expansion policy: which relations make up the "children" of each node kind.
// The chain portal -> menu -> MR -> microfront -> MS -> MS is walked lazily,
// one neighbors request per rule on first expand.
// ---------------------------------------------------------------------------

interface ExpandRule {
  relation: string
  direction: 'in' | 'out'
}

const EXPAND_RULES: Record<string, ExpandRule[]> = {
  portal: [
    { relation: 'belongs_to', direction: 'in' }, // menu options
    { relation: 'exposes', direction: 'out' }, // MRs without menu entry
  ],
  menu_option: [{ relation: 'implemented_by', direction: 'out' }],
  // "repo" covers both MRs and MSs: MRs answer to has_app/consumes,
  // MSs answer to calls. Whichever returns rows becomes the children.
  repo: [
    { relation: 'has_app', direction: 'out' },
    { relation: 'consumes', direction: 'out' },
    { relation: 'calls', direction: 'out' },
  ],
  microfront: [{ relation: 'consumes', direction: 'out' }],
}

const MAX_DEPTH = 12

// ---------------------------------------------------------------------------
// Tree state
// ---------------------------------------------------------------------------

interface TreeItem {
  node: GraphNode
  relation?: string
  edgeAttrs?: Record<string, unknown>
  children: TreeItem[] | null // null = not loaded yet
  expanded: boolean
  loading: boolean
  isCycle: boolean
}

function makeItem(node: GraphNode, relation?: string, edgeAttrs?: Record<string, unknown>): TreeItem {
  return { node, relation, edgeAttrs, children: null, expanded: false, loading: false, isCycle: false }
}

function isExpandable(item: TreeItem): boolean {
  return !item.isCycle && EXPAND_RULES[item.node.kind] !== undefined
}

async function fetchChildren(item: TreeItem, ancestorIDs: Set<number>): Promise<TreeItem[]> {
  const rules = EXPAND_RULES[item.node.kind] ?? []
  const responses = await Promise.all(
    rules.map(rule =>
      api
        .graphNeighbors(item.node.id, { relation: rule.relation, direction: rule.direction, limit: 500 })
        .then(r => r.neighbors)
        .catch(() => [] as GraphNeighbor[]),
    ),
  )
  const seen = new Set<string>()
  const children: TreeItem[] = []
  for (const neighbors of responses) {
    for (const nb of neighbors) {
      const dedupeKey = `${nb.relation}:${nb.node.id}`
      if (seen.has(dedupeKey)) continue
      seen.add(dedupeKey)
      const child = makeItem(nb.node, nb.relation, nb.edge_attrs)
      child.isCycle = ancestorIDs.has(nb.node.id)
      children.push(child)
    }
  }
  children.sort((a, b) => (a.node.label || a.node.key).localeCompare(b.node.label || b.node.key))
  return children
}

// ---------------------------------------------------------------------------
// Row rendering
// ---------------------------------------------------------------------------

const KIND_COLORS: Record<string, string> = {
  portal: '#8b5cf6',
  menu_option: '#f59e0b',
  repo: 'var(--accent)',
  microfront: '#06b6d4',
  use_case: '#10b981',
  external_service: '#ef4444',
  messaging: '#ec4899',
}

const RELATION_LABELS: Record<string, string> = {
  belongs_to: 'menu',
  exposes: 'exposes',
  implemented_by: 'MR',
  has_app: 'app',
  consumes: 'consumes',
  calls: 'calls',
}

function EdgeChips({ item }: { item: TreeItem }) {
  const attrs = item.edgeAttrs
  if (!attrs) return null
  const chips: string[] = []
  const endpointCount = Number(attrs.endpoint_count ?? 0)
  if (endpointCount > 0) chips.push(`${endpointCount} endpoints`)
  if (typeof attrs.via === 'string') chips.push(String(attrs.via).replace('apiclient:', 'via '))
  if (typeof attrs.match_score === 'number' && attrs.ambiguous === true) chips.push('ambiguous match')
  if (chips.length === 0) return null
  return (
    <>
      {chips.map(chip => (
        <span
          key={chip}
          style={{
            fontSize: '0.68rem',
            color: 'var(--fg3)',
            background: 'var(--bg-sunken)',
            border: '1px solid var(--border)',
            borderRadius: 4,
            padding: '0 5px',
            whiteSpace: 'nowrap',
            flexShrink: 0,
          }}
        >
          {chip}
        </span>
      ))}
    </>
  )
}

function TreeRow({
  item,
  depth,
  onToggle,
  onSelect,
}: {
  item: TreeItem
  depth: number
  onToggle: (item: TreeItem) => void
  onSelect: (id: number) => void
}) {
  const expandable = isExpandable(item) && depth < MAX_DEPTH
  const kindColor = KIND_COLORS[item.node.kind] ?? 'var(--fg3)'
  const relationLabel = item.relation ? RELATION_LABELS[item.relation] ?? item.relation : null

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        padding: '4px 10px 4px 0',
        paddingLeft: 10 + depth * 16,
        borderBottom: '1px solid var(--border)',
        minWidth: 0,
      }}
    >
      <button
        onClick={() => expandable && onToggle(item)}
        aria-label={item.expanded ? 'Collapse' : 'Expand'}
        style={{
          background: 'none',
          border: 'none',
          padding: 2,
          cursor: expandable ? 'pointer' : 'default',
          color: expandable ? 'var(--fg2)' : 'transparent',
          display: 'flex',
          flexShrink: 0,
        }}
      >
        {item.loading ? (
          <RotateCw size={13} style={{ color: 'var(--fg3)' }} />
        ) : item.expanded ? (
          <ChevronDown size={13} />
        ) : (
          <ChevronRight size={13} />
        )}
      </button>

      <span
        style={{
          width: 8,
          height: 8,
          borderRadius: '50%',
          background: kindColor,
          flexShrink: 0,
        }}
        title={item.node.kind}
      />

      <button
        onClick={() => onSelect(item.node.id)}
        title={item.node.key}
        style={{
          background: 'none',
          border: 'none',
          padding: 0,
          cursor: 'pointer',
          color: 'var(--fg1)',
          fontSize: '0.8125rem',
          fontWeight: depth === 0 ? 600 : 400,
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
          textAlign: 'left',
        }}
      >
        {item.node.label || item.node.key}
      </button>

      {relationLabel && (
        <span style={{ fontSize: '0.68rem', color: 'var(--fg3)', flexShrink: 0 }}>{relationLabel}</span>
      )}

      <EdgeChips item={item} />

      {item.isCycle && (
        <span title="Already in this branch (cycle)" style={{ display: 'flex', flexShrink: 0 }}>
          <Repeat size={12} style={{ color: '#f59e0b' }} />
        </span>
      )}

      {item.children !== null && item.children.length > 0 && (
        <span style={{ fontSize: '0.68rem', color: 'var(--fg3)', marginLeft: 'auto', flexShrink: 0 }}>
          {item.children.length}
        </span>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// TreeExplorer
// ---------------------------------------------------------------------------

export function TreeExplorer({
  namespace,
  onSelectNode,
}: {
  namespace?: string
  onSelectNode: (id: number) => void
}) {
  const [roots, setRoots] = useState<TreeItem[] | null>(null)
  const [filter, setFilter] = useState('')
  const [searchResults, setSearchResults] = useState<GraphNodeSearchResult[] | null>(null)
  const [isSearching, setIsSearching] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [, forceRender] = useState(0)
  const rerender = useCallback(() => forceRender(n => n + 1), [])

  const debouncedFilter = useDebounce(filter, 300)

  useEffect(() => {
    setRoots(null)
    setError(null)
    api
      .graphNodesByKind({ kind: 'portal', namespace, limit: 100 })
      .then(r => setRoots(r.nodes.map(n => makeItem(n))))
      .catch(e => setError(String(e)))
  }, [namespace])

  useEffect(() => {
    if (debouncedFilter.trim().length < 3) {
      setSearchResults(null)
      return
    }
    setIsSearching(true)
    api
      .graphSearch({ q: debouncedFilter.trim(), namespace, limit: 30 })
      .then(r => setSearchResults(r))
      .catch(() => setSearchResults([]))
      .finally(() => setIsSearching(false))
  }, [debouncedFilter, namespace])

  const toggle = useCallback(
    async (item: TreeItem, ancestorIDs: Set<number>) => {
      if (item.expanded) {
        item.expanded = false
        rerender()
        return
      }
      item.expanded = true
      if (item.children === null && !item.loading) {
        item.loading = true
        rerender()
        try {
          item.children = await fetchChildren(item, ancestorIDs)
        } catch (e) {
          item.children = []
          setError(String(e))
        } finally {
          item.loading = false
        }
      }
      rerender()
    },
    [rerender],
  )

  // Flatten visible tree into rows, applying the text filter on root level.
  // path makes keys unique even when the same node appears in many branches.
  const rows: { item: TreeItem; depth: number; ancestors: Set<number>; path: string }[] = []
  const pushVisible = (items: TreeItem[], depth: number, ancestors: Set<number>, parentPath: string) => {
    for (const item of items) {
      if (
        depth === 0 &&
        filter.trim() &&
        !(item.node.label || item.node.key).toLowerCase().includes(filter.trim().toLowerCase())
      ) {
        continue
      }
      const path = `${parentPath}/${item.relation ?? 'root'}:${item.node.id}`
      rows.push({ item, depth, ancestors, path })
      if (item.expanded && item.children) {
        const childAncestors = new Set(ancestors)
        childAncestors.add(item.node.id)
        pushVisible(item.children, depth + 1, childAncestors, path)
      }
    }
  }
  if (roots) pushVisible(roots, 0, new Set(), '')

  const isSearchMode = searchResults !== null

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
      <div style={{ padding: '10px 14px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
        <Input
          prefix={<ListFilter size={14} />}
          placeholder="Search nodes or filter portals…"
          value={filter}
          onChange={e => setFilter(e.target.value)}
        />
      </div>

      <div style={{ flex: 1, overflow: 'auto' }}>
        {error && (
          <div style={{ padding: '10px 14px', color: 'var(--red)', fontSize: '0.8125rem' }}>{error}</div>
        )}

        {isSearchMode ? (
          <>
            {isSearching && (
              <div style={{ padding: '10px 14px', color: 'var(--fg3)', fontSize: '0.8125rem' }}>Searching…</div>
            )}
            {!isSearching && searchResults.length === 0 && (
              <div style={{ padding: '24px 16px', color: 'var(--fg3)', fontSize: '0.875rem', textAlign: 'center' }}>
                No results for &ldquo;{filter.trim()}&rdquo;
              </div>
            )}
            {searchResults.map(r => (
              <button
                key={r.node.id}
                onClick={() => onSelectNode(r.node.id)}
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 2,
                  width: '100%',
                  textAlign: 'left',
                  padding: '8px 14px',
                  background: 'none',
                  border: 'none',
                  borderBottom: '1px solid var(--border)',
                  cursor: 'pointer',
                }}
                onMouseEnter={e => { (e.currentTarget as HTMLElement).style.background = 'var(--bg-hover)' }}
                onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = 'none' }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span style={{ fontWeight: 600, color: 'var(--fg1)', fontSize: '0.8125rem', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {r.node.label || r.node.key}
                  </span>
                  <span style={{
                    display: 'inline-block',
                    padding: '1px 6px',
                    borderRadius: 999,
                    fontSize: '0.7em',
                    fontWeight: 600,
                    background: (KIND_COLORS[r.node.kind] ?? 'var(--fg3)') + '22',
                    color: KIND_COLORS[r.node.kind] ?? 'var(--fg3)',
                    border: `1px solid ${KIND_COLORS[r.node.kind] ?? 'var(--fg3)'}44`,
                    flexShrink: 0,
                  }}>
                    {r.node.kind}
                  </span>
                </div>
                {r.snippet && (
                  <div style={{ fontSize: '0.75rem', color: 'var(--fg2)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {r.snippet}
                  </div>
                )}
              </button>
            ))}
          </>
        ) : (
          <>
            {!roots && !error && (
              <div style={{ padding: 24, color: 'var(--fg3)', fontSize: '0.875rem' }}>Loading portals…</div>
            )}
            {roots && roots.length === 0 && (
              <div style={{ padding: 24, color: 'var(--fg3)', fontSize: '0.875rem' }}>
                No portal nodes in the graph. Load an inventory first.
              </div>
            )}
            {rows.map(({ item, depth, ancestors, path }) => (
              <TreeRow
                key={path}
                item={item}
                depth={depth}
                onToggle={it => toggle(it, ancestors)}
                onSelect={onSelectNode}
              />
            ))}
          </>
        )}
      </div>
    </div>
  )
}
