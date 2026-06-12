import { useState, useEffect, useCallback } from 'react'
import DOMPurify from 'dompurify'
import { Search, Network, ChevronRight, ArrowLeft, Zap, ListTree, GitBranch } from 'lucide-react'
import { useDebounce } from '../../hooks/useDebounce'
import * as api from '../../api/client'
import { TreeExplorer } from './TreeExplorer'
import { HierarchyView } from './HierarchyView'
import type {
  GraphNodeSearchResult,
  GraphNodeDetail,
  GraphNeighbor,
  GraphImpactResult,
  GraphStatsResponse,
} from '../../types'
import { Input } from '../ui/Input'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type PanelView = 'overview' | 'neighbors' | 'impact'
type ExploreMode = 'search' | 'tree' | 'hierarchy'

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function KindBadge({ kind }: { kind: string }) {
  const colors: Record<string, string> = {
    repo: 'var(--accent)',
    portal: '#8b5cf6',
    menu_option: '#f59e0b',
    use_case: '#10b981',
    team_project: '#06b6d4',
  }
  const bg = colors[kind] ?? 'var(--fg3)'
  return (
    <span
      style={{
        display: 'inline-block',
        padding: '1px 7px',
        borderRadius: 999,
        fontSize: '0.72em',
        fontWeight: 600,
        letterSpacing: '0.3px',
        background: bg + '22',
        color: bg,
        border: `1px solid ${bg}44`,
        flexShrink: 0,
      }}
    >
      {kind}
    </span>
  )
}

function NodeRow({
  node,
  snippet,
  onClick,
}: {
  node: { id?: number; kind: string; key: string; label: string }
  snippet?: string
  onClick?: () => void
}) {
  return (
    <button
      onClick={onClick}
      style={{
        display: 'flex',
        alignItems: 'flex-start',
        gap: 10,
        width: '100%',
        textAlign: 'left',
        padding: '10px 14px',
        background: 'none',
        border: 'none',
        borderBottom: '1px solid var(--border)',
        cursor: onClick ? 'pointer' : 'default',
        transition: 'background 0.12s',
      }}
      onMouseEnter={e => {
        if (onClick) (e.currentTarget as HTMLElement).style.background = 'var(--bg-hover)'
      }}
      onMouseLeave={e => {
        ;(e.currentTarget as HTMLElement).style.background = 'none'
      }}
    >
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <span style={{ fontWeight: 600, color: 'var(--fg1)', fontSize: '0.875rem' }}>
            {node.label || node.key}
          </span>
          <KindBadge kind={node.kind} />
        </div>
        <div
          style={{
            fontSize: '0.78rem',
            color: 'var(--fg3)',
            marginTop: 2,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {node.key}
        </div>
        {snippet && (
          <div
            style={{ fontSize: '0.78rem', color: 'var(--fg2)', marginTop: 4 }}
            dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(snippet, { ALLOWED_TAGS: ['mark'] }) }}
          />
        )}
      </div>
      {onClick && <ChevronRight size={15} style={{ color: 'var(--fg3)', flexShrink: 0, marginTop: 2 }} />}
    </button>
  )
}

// ---------------------------------------------------------------------------
// Stats panel (shown when nothing is selected)
// ---------------------------------------------------------------------------

function StatsPanel({ statsResp }: { statsResp: GraphStatsResponse | null }) {
  if (!statsResp) {
    return (
      <div style={{ padding: 24, color: 'var(--fg3)', fontSize: '0.875rem' }}>
        The knowledge graph is empty. Use the MCP tools to populate it.
      </div>
    )
  }
  const { stats, namespaces } = statsResp

  return (
    <div style={{ padding: 16 }}>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gap: 12,
          marginBottom: 16,
        }}
      >
        {[
          { label: 'Nodes', value: stats.node_count },
          { label: 'Edges', value: stats.edge_count },
        ].map(c => (
          <div key={c.label} className="card" style={{ padding: '12px 16px' }}>
            <div style={{ fontSize: '1.5rem', fontWeight: 700, color: 'var(--fg1)' }}>{c.value}</div>
            <div style={{ fontSize: '0.78rem', color: 'var(--fg3)', marginTop: 2 }}>{c.label}</div>
          </div>
        ))}
      </div>

      {Object.keys(stats.nodes_by_kind).length > 0 && (
        <div className="card" style={{ marginBottom: 12 }}>
          <div
            style={{
              padding: '8px 14px',
              borderBottom: '1px solid var(--border)',
              fontSize: '0.72em',
              fontWeight: 600,
              textTransform: 'uppercase',
              letterSpacing: '0.5px',
              color: 'var(--fg2)',
            }}
          >
            Nodes by kind
          </div>
          {Object.entries(stats.nodes_by_kind).map(([kind, count]) => (
            <div
              key={kind}
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                padding: '7px 14px',
                borderBottom: '1px solid var(--border)',
              }}
            >
              <KindBadge kind={kind} />
              <span style={{ fontSize: '0.875rem', fontWeight: 600, color: 'var(--fg1)' }}>{count}</span>
            </div>
          ))}
        </div>
      )}

      {namespaces.length > 0 && (
        <div style={{ fontSize: '0.78rem', color: 'var(--fg3)' }}>
          <span style={{ fontWeight: 600, color: 'var(--fg2)' }}>Namespaces: </span>
          {namespaces.join(', ')}
        </div>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Node detail panel
// ---------------------------------------------------------------------------

function NodeDetailPanel({
  detail,
  impact,
  panelView,
  onPanelChange,
  onNodeClick,
}: {
  detail: GraphNodeDetail
  impact: GraphImpactResult | null
  panelView: PanelView
  onPanelChange: (v: PanelView) => void
  onNodeClick: (id: number) => void
}) {
  const { node, relations, neighbor_count } = detail

  const tabs: { id: PanelView; label: string }[] = [
    { id: 'overview', label: 'Overview' },
    { id: 'neighbors', label: `Neighbors (${neighbor_count})` },
    { id: 'impact', label: impact ? `Impact (${impact.impacted_count})` : 'Impact' },
  ]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Node header */}
      <div style={{ padding: '14px 16px', borderBottom: '1px solid var(--border)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
          <Network size={16} style={{ color: 'var(--accent)', flexShrink: 0 }} />
          <span style={{ fontWeight: 700, color: 'var(--fg1)', fontSize: '0.95rem' }}>
            {node.label || node.key}
          </span>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <KindBadge kind={node.kind} />
          <span style={{ fontSize: '0.75rem', color: 'var(--fg3)' }}>{node.key}</span>
        </div>
        {node.summary && (
          <p style={{ margin: '8px 0 0', fontSize: '0.8125rem', color: 'var(--fg2)', lineHeight: 1.5 }}>
            {node.summary}
          </p>
        )}
      </div>

      {/* Tabs */}
      <div
        style={{
          display: 'flex',
          borderBottom: '1px solid var(--border)',
          flexShrink: 0,
        }}
      >
        {tabs.map(tab => (
          <button
            key={tab.id}
            onClick={() => onPanelChange(tab.id)}
            style={{
              flex: 1,
              padding: '8px 4px',
              background: 'none',
              border: 'none',
              borderBottom: panelView === tab.id ? '2px solid var(--accent)' : '2px solid transparent',
              color: panelView === tab.id ? 'var(--fg1)' : 'var(--fg3)',
              fontWeight: panelView === tab.id ? 600 : 400,
              fontSize: '0.78rem',
              cursor: 'pointer',
              transition: 'color 0.12s',
            }}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div style={{ flex: 1, overflow: 'auto' }}>
        {panelView === 'overview' && (
          <OverviewTab node={node} />
        )}
        {panelView === 'neighbors' && (
          <NeighborsTab relations={relations} onNodeClick={onNodeClick} />
        )}
        {panelView === 'impact' && (
          <ImpactTab impact={impact} />
        )}
      </div>
    </div>
  )
}

function OverviewTab({ node }: { node: GraphNodeDetail['node'] }) {
  const attrs = node.attrs ? Object.entries(node.attrs) : []
  return (
    <div style={{ padding: 14 }}>
      <div className="card" style={{ marginBottom: 12 }}>
        <Row label="ID" value={String(node.id)} />
        <Row label="Namespace" value={node.namespace} />
        <Row label="Kind" value={node.kind} />
        <Row label="Key" value={node.key} isLast />
      </div>
      {attrs.length > 0 && (
        <div className="card">
          <div
            style={{
              padding: '8px 14px',
              borderBottom: '1px solid var(--border)',
              fontSize: '0.72em',
              fontWeight: 600,
              textTransform: 'uppercase',
              letterSpacing: '0.5px',
              color: 'var(--fg2)',
            }}
          >
            Attributes
          </div>
          {attrs.map(([k, v], i) => (
            <Row key={k} label={k} value={JSON.stringify(v)} isLast={i === attrs.length - 1} />
          ))}
        </div>
      )}
    </div>
  )
}

function Row({ label, value, isLast }: { label: string; value: string; isLast?: boolean }) {
  return (
    <div
      style={{
        display: 'flex',
        gap: 12,
        padding: '7px 14px',
        borderBottom: isLast ? 'none' : '1px solid var(--border)',
        alignItems: 'flex-start',
      }}
    >
      <span
        style={{
          fontSize: '0.78rem',
          color: 'var(--fg3)',
          flexShrink: 0,
          width: 90,
          paddingTop: 1,
        }}
      >
        {label}
      </span>
      <span
        style={{
          fontSize: '0.8125rem',
          color: 'var(--fg1)',
          wordBreak: 'break-all',
        }}
      >
        {value}
      </span>
    </div>
  )
}

interface Occurrence { file: string; lines: number[] }
interface EdgeSource { path: string; file: string; detected_by: string }

function EdgeEvidence({ attrs }: { attrs: Record<string, unknown> }) {
  const occs = attrs.occurrences as Occurrence[] | undefined
  const sources = attrs.sources as EdgeSource[] | undefined
  const [expanded, setExpanded] = useState(false)

  if (!occs?.length && !sources?.length) return null

  const items = occs
    ? occs.map(o => {
        const parts = o.file.split('/')
        const short = parts.slice(-2).join('/')
        return { label: short, detail: o.file, sub: o.lines.length ? `lines: ${o.lines.slice(0, 8).join(', ')}${o.lines.length > 8 ? '…' : ''}` : '' }
      })
    : (sources ?? []).map(s => {
        const parts = s.file.split('/')
        const short = parts.slice(-2).join('/')
        return { label: short, detail: s.file, sub: s.path }
      })

  const preview = items.slice(0, 2)
  const rest = items.slice(2)

  return (
    <div style={{ padding: '4px 14px 8px 28px', borderBottom: '1px solid var(--border)' }}>
      {preview.map((it, i) => (
        <div key={i} style={{ marginBottom: 3 }}>
          <span style={{ fontSize: '0.72rem', color: 'var(--fg1)', fontFamily: 'monospace' }} title={it.detail}>{it.label}</span>
          {it.sub && <span style={{ fontSize: '0.68rem', color: 'var(--fg3)', marginLeft: 6 }}>{it.sub}</span>}
        </div>
      ))}
      {rest.length > 0 && (
        <>
          {expanded && rest.map((it, i) => (
            <div key={i} style={{ marginBottom: 3 }}>
              <span style={{ fontSize: '0.72rem', color: 'var(--fg1)', fontFamily: 'monospace' }} title={it.detail}>{it.label}</span>
              {it.sub && <span style={{ fontSize: '0.68rem', color: 'var(--fg3)', marginLeft: 6 }}>{it.sub}</span>}
            </div>
          ))}
          <button
            onClick={e => { e.stopPropagation(); setExpanded(v => !v) }}
            style={{ fontSize: '0.68rem', color: 'var(--fg3)', background: 'none', border: 'none', padding: 0, cursor: 'pointer', marginTop: 2 }}
          >
            {expanded ? '▲ show less' : `▼ +${rest.length} more`}
          </button>
        </>
      )}
    </div>
  )
}

function NeighborsTab({
  relations,
  onNodeClick,
}: {
  relations: Record<string, GraphNeighbor[]>
  onNodeClick: (id: number) => void
}) {
  const groups = Object.entries(relations)
  if (groups.length === 0) {
    return (
      <div style={{ padding: 24, color: 'var(--fg3)', fontSize: '0.875rem' }}>
        No neighbors found.
      </div>
    )
  }

  return (
    <div>
      {groups.map(([groupKey, nbs]) => {
        const [direction, ...rest] = groupKey.split(':')
        const relation = rest.join(':')
        return (
          <div key={groupKey}>
            <div
              style={{
                padding: '7px 14px',
                background: 'var(--bg-sunken)',
                fontSize: '0.72em',
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.5px',
                color: 'var(--fg2)',
                borderBottom: '1px solid var(--border)',
                display: 'flex',
                gap: 8,
                alignItems: 'center',
              }}
            >
              <span
                style={{
                  padding: '1px 6px',
                  borderRadius: 4,
                  background: direction === 'out' ? '#3b82f622' : '#f59e0b22',
                  color: direction === 'out' ? '#3b82f6' : '#f59e0b',
                  fontSize: '0.9em',
                }}
              >
                {direction === 'out' ? 'OUT' : 'IN'}
              </span>
              {relation}
            </div>
            {nbs.map(nb => (
              <div key={nb.node.id}>
                <NodeRow node={nb.node} onClick={() => onNodeClick(nb.node.id)} />
                {nb.edge_attrs && Object.keys(nb.edge_attrs).length > 0 && (
                  <EdgeEvidence attrs={nb.edge_attrs as Record<string, unknown>} />
                )}
              </div>
            ))}
          </div>
        )
      })}
    </div>
  )
}

function ImpactTab({ impact }: { impact: GraphImpactResult | null }) {
  if (!impact) {
    return (
      <div style={{ padding: 24, color: 'var(--fg3)', fontSize: '0.875rem' }}>
        Loading impact…
      </div>
    )
  }
  if (impact.impacted_count === 0) {
    return (
      <div style={{ padding: 24, color: 'var(--fg3)', fontSize: '0.875rem' }}>
        No dependents found — nothing currently depends on this node.
      </div>
    )
  }

  return (
    <div>
      <div
        style={{
          display: 'flex',
          gap: 12,
          padding: '10px 14px',
          borderBottom: '1px solid var(--border)',
          flexWrap: 'wrap',
        }}
      >
        <div>
          <div style={{ fontSize: '1.25rem', fontWeight: 700, color: 'var(--fg1)' }}>
            {impact.impacted_count}
          </div>
          <div style={{ fontSize: '0.75rem', color: 'var(--fg3)' }}>total dependents</div>
        </div>
        {Object.entries(impact.impacted_by_kind).map(([kind, count]) => (
          <div key={kind}>
            <div style={{ fontSize: '1.25rem', fontWeight: 700, color: 'var(--fg1)' }}>{count}</div>
            <KindBadge kind={kind} />
          </div>
        ))}
      </div>

      {impact.truncated && (
        <div
          style={{
            padding: '6px 14px',
            background: '#f59e0b11',
            borderBottom: '1px solid var(--border)',
            fontSize: '0.78rem',
            color: '#f59e0b',
            display: 'flex',
            gap: 6,
            alignItems: 'center',
          }}
        >
          <Zap size={13} />
          Results truncated — showing top {impact.impacted.length} of {impact.impacted_count}
        </div>
      )}

      {impact.impacted.map(row => (
        <div
          key={`${row.kind}:${row.key}:${row.depth}`}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            padding: '8px 14px',
            borderBottom: '1px solid var(--border)',
          }}
        >
          <span
            style={{
              width: 24,
              height: 24,
              borderRadius: '50%',
              background: 'var(--bg-sunken)',
              border: '1px solid var(--border-strong)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '0.72rem',
              fontWeight: 700,
              color: 'var(--fg2)',
              flexShrink: 0,
            }}
          >
            {row.depth}
          </span>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
              <span style={{ fontWeight: 600, color: 'var(--fg1)', fontSize: '0.875rem' }}>
                {row.label || row.key}
              </span>
              <KindBadge kind={row.kind} />
            </div>
            <div
              style={{
                fontSize: '0.75rem',
                color: 'var(--fg3)',
                marginTop: 2,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {row.key}
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main GraphView
// ---------------------------------------------------------------------------

export function GraphView() {
  const [mode, setMode] = useState<ExploreMode>('search')
  const [query, setQuery] = useState('')
  const [namespace, setNamespace] = useState('')
  const [results, setResults] = useState<GraphNodeSearchResult[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const [selectedNodeID, setSelectedNodeID] = useState<number | null>(null)
  const [detail, setDetail] = useState<GraphNodeDetail | null>(null)
  const [impact, setImpact] = useState<GraphImpactResult | null>(null)
  const [panelView, setPanelView] = useState<PanelView>('overview')
  const [statsResp, setStatsResp] = useState<GraphStatsResponse | null>(null)
  const [namespaces, setNamespaces] = useState<string[]>([])
  const [error, setError] = useState<string | null>(null)

  const debouncedQuery = useDebounce(query, 300)

  // Load stats + namespaces on mount
  useEffect(() => {
    api
      .graphStats()
      .then(r => {
        setStatsResp(r)
        setNamespaces(r.namespaces)
        // If only one namespace, pre-select it
        if (r.namespaces.length === 1) {
          setNamespace(r.namespaces[0])
        }
      })
      .catch(() => {
        // graph may be empty; that's fine
      })
  }, [])

  // Search when debounced query changes
  useEffect(() => {
    if (!debouncedQuery.trim()) {
      setResults([])
      return
    }
    setIsSearching(true)
    setError(null)
    api
      .graphSearch({ q: debouncedQuery, namespace: namespace || undefined, limit: 30 })
      .then(r => {
        setResults(r)
      })
      .catch(e => {
        setError(String(e))
      })
      .finally(() => setIsSearching(false))
  }, [debouncedQuery, namespace])

  // Load node detail + impact when selection changes
  const selectNode = useCallback((id: number) => {
    setSelectedNodeID(id)
    setDetail(null)
    setImpact(null)
    setPanelView('overview')
    setError(null)

    Promise.all([api.graphNodeByID(id), api.graphImpact(id, { limit: 100 })])
      .then(([d, imp]) => {
        setDetail(d)
        setImpact(imp)
      })
      .catch(e => setError(String(e)))
  }, [])

  const clearSelection = useCallback(() => {
    setSelectedNodeID(null)
    setDetail(null)
    setImpact(null)
  }, [])

  const showDetail = selectedNodeID !== null && mode !== 'hierarchy'

  return (
    <div className="content-pad" style={{ display: 'flex', flexDirection: 'column', height: '100%', gap: 0, padding: 0 }}>
      <div style={{ display: 'flex', height: '100%', overflow: 'hidden' }}>
        {/* Left panel — search / tree / hierarchy */}
        <div
          style={{
            width: showDetail ? 340 : '100%',
            maxWidth: showDetail ? 340 : undefined,
            flexShrink: 0,
            display: 'flex',
            flexDirection: 'column',
            borderRight: showDetail ? '1px solid var(--border)' : 'none',
            overflow: 'hidden',
          }}
        >
          {/* Mode tabs */}
          <div style={{ display: 'flex', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
            {(
              [
                { id: 'search', label: 'Search', icon: <Search size={13} /> },
                { id: 'tree', label: 'Tree', icon: <ListTree size={13} /> },
                { id: 'hierarchy', label: 'Hierarchy', icon: <GitBranch size={13} /> },
              ] as { id: ExploreMode; label: string; icon: React.ReactNode }[]
            ).map(tab => (
              <button
                key={tab.id}
                onClick={() => setMode(tab.id)}
                style={{
                  flex: 1,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: 6,
                  padding: '8px 4px',
                  background: 'none',
                  border: 'none',
                  borderBottom: mode === tab.id ? '2px solid var(--accent)' : '2px solid transparent',
                  color: mode === tab.id ? 'var(--fg1)' : 'var(--fg3)',
                  fontWeight: mode === tab.id ? 600 : 400,
                  fontSize: '0.78rem',
                  cursor: 'pointer',
                }}
              >
                {tab.icon}
                {tab.label}
              </button>
            ))}
          </div>

          {mode === 'hierarchy' ? (
            selectedNodeID !== null ? (
              <HierarchyView
                rootNodeId={selectedNodeID}
                namespace={namespace || undefined}
                onSelectNode={selectNode}
              />
            ) : (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', padding: 24, color: 'var(--fg3)', fontSize: '0.875rem', textAlign: 'center' }}>
                Select a node in Search or Tree first
              </div>
            )
          ) : mode === 'tree' ? (
            <TreeExplorer namespace={namespace || undefined} onSelectNode={selectNode} />
          ) : (
            <>
              {/* Search bar */}
              <div
                style={{
                  padding: '14px 16px',
                  borderBottom: '1px solid var(--border)',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 8,
                }}
              >
                <Input
                  prefix={<Search size={15} />}
                  placeholder="Search nodes…"
                  value={query}
                  onChange={e => setQuery(e.target.value)}
                  autoFocus
                />
                {namespaces.length > 1 && (
                  <select
                    aria-label="Filter by namespace"
                    value={namespace}
                    onChange={e => setNamespace(e.target.value)}
                    style={{
                      padding: '6px 10px',
                      border: '1px solid var(--border-strong)',
                      borderRadius: 'var(--radius-md)',
                      background: 'var(--bg-sunken)',
                      color: 'var(--fg1)',
                      fontSize: '0.8125rem',
                      cursor: 'pointer',
                    }}
                  >
                    <option value="">All namespaces</option>
                    {namespaces.map(ns => (
                      <option key={ns} value={ns}>
                        {ns}
                      </option>
                    ))}
                  </select>
                )}
              </div>

              {/* Results / stats */}
              <div style={{ flex: 1, overflow: 'auto' }}>
                {error && (
                  <div
                    style={{
                      padding: '10px 14px',
                      color: 'var(--red)',
                      fontSize: '0.8125rem',
                      borderBottom: '1px solid var(--border)',
                    }}
                  >
                    {error}
                  </div>
                )}

                {isSearching && (
                  <div style={{ padding: '10px 14px', color: 'var(--fg3)', fontSize: '0.8125rem' }}>
                    Searching…
                  </div>
                )}

                {!isSearching && query.trim() && results.length === 0 && !error && (
                  <div style={{ padding: '24px 16px', color: 'var(--fg3)', fontSize: '0.875rem', textAlign: 'center' }}>
                    No nodes found for &ldquo;{query}&rdquo;
                  </div>
                )}

                {!query.trim() && (
                  <StatsPanel statsResp={statsResp} />
                )}

                {results.map(r => (
                  <NodeRow
                    key={r.node.id}
                    node={r.node}
                    snippet={r.snippet}
                    onClick={() => selectNode(r.node.id)}
                  />
                ))}
              </div>
            </>
          )}
        </div>

        {/* Right panel — node detail */}
        {showDetail && (
          <div style={{ flex: 1, overflow: 'auto', display: 'flex', flexDirection: 'column' }}>
            {/* Back button */}
            <div style={{ padding: '10px 14px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
              <button
                onClick={clearSelection}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 6,
                  background: 'none',
                  border: 'none',
                  color: 'var(--fg2)',
                  fontSize: '0.8125rem',
                  cursor: 'pointer',
                  padding: '2px 0',
                }}
              >
                <ArrowLeft size={14} />
                Back to results
              </button>
            </div>

            {!detail && !error && (
              <div style={{ padding: 24, color: 'var(--fg3)', fontSize: '0.875rem' }}>Loading…</div>
            )}

            {error && (
              <div style={{ padding: 24, color: 'var(--red)', fontSize: '0.875rem' }}>{error}</div>
            )}

            {detail && (
              <NodeDetailPanel
                detail={detail}
                impact={impact}
                panelView={panelView}
                onPanelChange={setPanelView}
                onNodeClick={selectNode}
              />
            )}
          </div>
        )}
      </div>
    </div>
  )
}
