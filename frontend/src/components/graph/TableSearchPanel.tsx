import { useState, useEffect } from 'react'
import { Database, ChevronRight, ChevronDown } from 'lucide-react'
import { useDebounce } from '../../hooks/useDebounce'
import * as api from '../../api/client'
import type { TableUsage, TableUsageResult } from '../../types'
import { Input } from '../ui/Input'

const MAX_FILES_PER_TABLE = 50

// TableSearchPanel answers "which repos/files use table X?" from the
// imported uses_table edges. Clicking a table or repo opens its node detail.
export function TableSearchPanel({
  namespace,
  onSelectNode,
}: {
  namespace?: string
  onSelectNode: (kind: string, key: string) => void
}) {
  const [query, setQuery] = useState('')
  const [repo, setRepo] = useState('')
  // Each response is tagged with the request that produced it, so loading and
  // stale results are derived during render instead of reset inside the effect.
  const [fetched, setFetched] = useState<{ key: string; result: TableUsageResult | null; error: string | null } | null>(null)

  const debouncedQuery = useDebounce(query, 300)
  const debouncedRepo = useDebounce(repo, 300)
  const requestKey = JSON.stringify([debouncedQuery.trim(), debouncedRepo.trim(), namespace ?? ''])

  useEffect(() => {
    if (!debouncedQuery.trim()) return
    let cancelled = false
    api
      .graphTableSearch({
        q: debouncedQuery,
        namespace,
        repo: debouncedRepo.trim() || undefined,
        limit: 30,
        maxFiles: MAX_FILES_PER_TABLE,
      })
      .then(result => {
        if (!cancelled) setFetched({ key: requestKey, result, error: null })
      })
      .catch(e => {
        if (!cancelled) setFetched({ key: requestKey, result: null, error: String(e) })
      })
    return () => {
      cancelled = true
    }
  }, [debouncedQuery, debouncedRepo, namespace, requestKey])

  const current = query.trim() && fetched?.key === requestKey ? fetched : null
  const isSearching = !!query.trim() && !current
  const shown = current?.result ?? null
  const error = current?.error ?? null
  const tables = shown?.tables ?? []

  return (
    <>
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
          prefix={<Database size={15} />}
          placeholder="Table name, e.g. PA_DEPARTAME or RUNTPROD.PA_"
          value={query}
          onChange={e => setQuery(e.target.value)}
          autoFocus
        />
        <Input
          placeholder="Filter by repo (exact), e.g. BlockChain/BCAdministracionMS"
          value={repo}
          onChange={e => setRepo(e.target.value)}
        />
      </div>

      <div style={{ flex: 1, overflow: 'auto' }}>
        {error && (
          <div style={{ padding: '10px 14px', color: 'var(--red)', fontSize: '0.8125rem', borderBottom: '1px solid var(--border)' }}>
            {error}
          </div>
        )}

        {isSearching && (
          <div style={{ padding: '10px 14px', color: 'var(--fg3)', fontSize: '0.8125rem' }}>Searching…</div>
        )}

        {!query.trim() && (
          <div style={{ padding: '24px 16px', color: 'var(--fg3)', fontSize: '0.875rem', textAlign: 'center' }}>
            Search a database table to see which repos and files read or write it.
          </div>
        )}

        {!isSearching && shown && tables.length === 0 && !error && (
          <div style={{ padding: '24px 16px', color: 'var(--fg3)', fontSize: '0.875rem', textAlign: 'center' }}>
            No imported tables match &ldquo;{query}&rdquo;
          </div>
        )}

        {shown?.tables_truncated && (
          <div style={{ padding: '8px 14px', color: 'var(--fg3)', fontSize: '0.78rem', borderBottom: '1px solid var(--border)' }}>
            Showing the first {tables.length} tables — refine the name to narrow results.
          </div>
        )}

        {tables.map(t => (
          <TableRow key={`${t.namespace}:${t.key}`} table={t} onSelectNode={onSelectNode} />
        ))}
      </div>
    </>
  )
}

function TableRow({
  table,
  onSelectNode,
}: {
  table: TableUsage
  onSelectNode: (kind: string, key: string) => void
}) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div style={{ borderBottom: '1px solid var(--border)' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '10px 14px' }}>
        <button
          aria-label={expanded ? 'Collapse table usage' : 'Expand table usage'}
          onClick={() => setExpanded(v => !v)}
          style={{ background: 'none', border: 'none', color: 'var(--fg3)', cursor: 'pointer', padding: 0, display: 'flex' }}
        >
          {expanded ? <ChevronDown size={15} /> : <ChevronRight size={15} />}
        </button>
        <button
          onClick={() => onSelectNode('table', table.key)}
          style={{ flex: 1, minWidth: 0, textAlign: 'left', background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}
        >
          <div style={{ fontWeight: 600, color: 'var(--fg1)', fontSize: '0.875rem', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {table.key}
          </div>
          <div style={{ fontSize: '0.78rem', color: 'var(--fg3)', marginTop: 2 }}>
            {table.repo_count} repos · {table.file_count} files
          </div>
        </button>
      </div>

      {expanded && (
        <div style={{ padding: '0 14px 10px 37px', display: 'flex', flexDirection: 'column', gap: 8 }}>
          {table.repos.map(r => (
            <div key={r.repo}>
              <button
                onClick={() => onSelectNode('repo', r.repo)}
                style={{ background: 'none', border: 'none', padding: 0, cursor: 'pointer', color: 'var(--accent)', fontSize: '0.8125rem', fontWeight: 600 }}
              >
                {r.repo}
              </button>
              {r.files.map(f => (
                <div key={f.path} style={{ marginTop: 4, paddingLeft: 10 }}>
                  <div style={{ fontSize: '0.75rem', color: 'var(--fg2)', wordBreak: 'break-all' }}>
                    {f.path.startsWith(r.repo + '/') ? f.path.slice(r.repo.length + 1) : f.path}
                  </div>
                  {f.evidence.map((ev, i) => (
                    <div
                      key={i}
                      style={{ fontSize: '0.72rem', color: 'var(--fg3)', fontFamily: 'var(--font-mono, monospace)', paddingLeft: 10, wordBreak: 'break-all' }}
                    >
                      L{ev.line} · {ev.type} · {ev.text}
                    </div>
                  ))}
                </div>
              ))}
            </div>
          ))}
          {table.files_truncated && (
            <div style={{ fontSize: '0.75rem', color: 'var(--fg3)' }}>
              Showing {MAX_FILES_PER_TABLE} of {table.file_count} files — filter by repo to see the rest.
            </div>
          )}
        </div>
      )}
    </div>
  )
}
