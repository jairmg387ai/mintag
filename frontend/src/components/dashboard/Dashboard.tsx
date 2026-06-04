import { useState, useEffect, useMemo } from 'react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { StatCard } from '../shared/StatCard'
import { StatusBadge } from '../shared/StatusBadge'
import { PriorityDot } from '../shared/PriorityDot'
import { Avatar } from '../shared/Avatar'
import { useDebounce } from '../../hooks/useDebounce'
import { search } from '../../api/client'
import type { SearchResult, Status, Priority } from '../../types'
import { TopBar } from '../layout/TopBar'

function fmt(dt: string) {
  if (!dt) return '—'
  try {
    return new Date(dt).toLocaleString('en-US', { day: '2-digit', month: 'short', year: 'numeric' })
  } catch { return dt }
}

export function Dashboard() {
  const { stats, tasks, meetings } = useAppState()
  const { setEditingTaskId, openModal, setActiveMeetingId } = useAppActions()
  const [query, setQuery] = useState('')
  const [searchResults, setSearchResults] = useState<SearchResult[]>([])
  const debouncedQuery = useDebounce(query, 400)

  useEffect(() => {
    if (!debouncedQuery || debouncedQuery.length < 2) { setSearchResults([]); return }
    search(debouncedQuery).then(setSearchResults).catch(() => setSearchResults([]))
  }, [debouncedQuery])

  const activeTasks = useMemo(
    () => tasks.filter(t => t.status === 'blocked' || t.status === 'in_progress').slice(0, 8),
    [tasks]
  )
  const recentMeetings = useMemo(
    () => [...meetings].sort((a, b) => b.id - a.id).slice(0, 5),
    [meetings]
  )

  function openTask(id: number) { setEditingTaskId(id); openModal('task') }
  function openMeeting(id: number) { setActiveMeetingId(id); openModal('meeting') }

  return (
    <>
      <TopBar title="Dashboard">
        <div style={{ position: 'relative', flex: 1, maxWidth: 380 }}>
          <svg style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: 'var(--color-text3)', pointerEvents: 'none' }}
            width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
          </svg>
          <input
            type="text"
            placeholder="Search tasks and meetings..."
            value={query}
            onChange={e => setQuery(e.target.value)}
            className="input"
            style={{ padding: '8px 12px 8px 34px' }}
          />
        </div>
      </TopBar>

      <div style={{ padding: '24px 28px' }}>
        {/* Stats row */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(150px,1fr))', gap: 14, marginBottom: 24 }}>
          <StatCard label="Total Tasks"  value={stats?.total_tasks ?? 0}       color="var(--color-blue)" />
          <StatCard label="To Do"        value={stats?.todo_tasks ?? 0} />
          <StatCard label="In Progress"  value={stats?.in_progress_tasks ?? 0} color="var(--color-blue)" />
          <StatCard label="Blocked"      value={stats?.blocked_tasks ?? 0}     color="var(--color-red)" />
          <StatCard label="Done"         value={stats?.done_tasks ?? 0}        color="var(--color-green)" />
          <StatCard label="Meetings"     value={stats?.total_meetings ?? 0}    color="var(--color-orange)" />
        </div>

        {/* Search results */}
        {searchResults.length > 0 && (
          <div className="card" style={{ marginBottom: 20 }}>
            <div className="card-header">
              <h2>Search results: "{query}"</h2>
            </div>
            <div style={{ padding: 8 }}>
              {searchResults.map(r => (
                <div
                  key={`${r.kind}-${r.id}`}
                  onClick={() => r.kind === 'task' ? openTask(r.id) : openMeeting(r.id)}
                  className="row-clickable"
                  style={{ padding: '8px 12px', display: 'flex', flexDirection: 'column', gap: 2 }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span className="badge">{r.kind === 'task' ? 'Task' : 'Meeting'}</span>
                    <strong style={{ fontSize: '0.88em' }}>{r.title}</strong>
                  </div>
                  <div style={{ fontSize: '0.78em', color: 'var(--color-text3)', paddingLeft: 2 }}>{r.snippet}</div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Content grid */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 18 }}>
          {/* Active tasks */}
          <div className="card">
            <div className="card-header">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="10"/><polyline points="12,6 12,12 16,14"/>
              </svg>
              <h2>Blocked / In Progress</h2>
            </div>
            {activeTasks.length === 0 ? (
              <div style={{ textAlign: 'center', padding: '48px 24px', color: 'var(--color-text3)' }}>No active tasks</div>
            ) : (
              activeTasks.map(t => (
                <div
                  key={t.id}
                  onClick={() => openTask(t.id)}
                  className="row-clickable"
                  style={{ display: 'grid', gridTemplateColumns: '28px 1fr 110px 90px', alignItems: 'center', gap: 12, padding: '11px 16px' }}
                >
                  <PriorityDot priority={t.priority as Priority} />
                  <div>
                    <div style={{ fontSize: '0.9em', fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{t.title}</div>
                    <div style={{ fontSize: '0.78em', color: 'var(--color-text3)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{t.project_name ?? ''}</div>
                  </div>
                  <StatusBadge status={t.status as Status} />
                  {t.owner ? <Avatar name={t.owner} /> : <span style={{ color: 'var(--color-text3)', fontSize: '0.78em' }}>—</span>}
                </div>
              ))
            )}
          </div>

          {/* Recent meetings */}
          <div className="card">
            <div className="card-header">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/>
              </svg>
              <h2>Recent Meetings</h2>
            </div>
            <div style={{ padding: '0 12px' }}>
              {recentMeetings.length === 0 ? (
                <div style={{ textAlign: 'center', padding: '36px 24px', color: 'var(--color-text3)' }}>No meetings</div>
              ) : (
                recentMeetings.map(m => (
                  <div
                    key={m.id}
                    onClick={() => openMeeting(m.id)}
                    className="row-clickable"
                    style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 4px' }}
                  >
                    <span style={{ fontSize: '0.72em', color: 'var(--color-text3)', minWidth: 80 }}>{fmt(m.date) || '—'}</span>
                    <span style={{ fontSize: '0.85em', flex: 1, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{m.title}</span>
                    <span className="badge">{m.task_count ?? 0} tasks</span>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>
    </>
  )
}
