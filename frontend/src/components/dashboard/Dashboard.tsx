import { useMemo, useState, useRef, useEffect, useCallback } from 'react'
import {
  ListTodo,
  CircleDot,
  CircleAlert,
  CalendarCheck,
  Users,
  Search,
} from 'lucide-react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { StatCard } from '../shared/StatCard'
import { StatusBadge } from '../shared/StatusBadge'
import { Avatar } from '../shared/Avatar'
import * as api from '../../api/client'
import type { Status, ViewName, SearchResult } from '../../types'

function fmt(dt: string) {
  if (!dt) return '—'
  try {
    return new Date(dt).toLocaleDateString('en-US', { day: '2-digit', month: 'short', year: 'numeric' })
  } catch {
    return dt
  }
}

// ── Inline search panel ──────────────────────────────────────────────────────

interface DashboardSearchProps {
  onSelectTask: (id: number) => void
  onSelectMeeting: (id: number) => void
}

function DashboardSearch({ onSelectTask, onSelectMeeting }: DashboardSearchProps) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<SearchResult[]>([])
  const [open, setOpen] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const doSearch = useCallback(async (q: string) => {
    if (!q.trim()) { setResults([]); setOpen(false); return }
    try {
      const res = await api.search(q)
      setResults(res ?? [])
      setOpen(true)
    } catch {
      setResults([])
    }
  }, [])

  useEffect(() => {
    if (timerRef.current) clearTimeout(timerRef.current)
    timerRef.current = setTimeout(() => doSearch(query), 300)
    return () => { if (timerRef.current) clearTimeout(timerRef.current) }
  }, [query, doSearch])

  function handleSelect(r: SearchResult) {
    if (r.kind === 'task') onSelectTask(r.id)
    else onSelectMeeting(r.id)
    setOpen(false)
    setQuery('')
  }

  return (
    <div style={{ marginTop: 24 }}>
      <div className="tb-search" style={{ width: 320, margin: '0 auto' }}>
        <span
          className="mt-icon"
          style={{
            position: 'absolute',
            left: 11,
            top: '50%',
            transform: 'translateY(-50%)',
            color: 'var(--fg3)',
          }}
        >
          <Search size={16} />
        </span>
        <input
          value={query}
          onChange={e => setQuery(e.target.value)}
          placeholder="Search tasks & meetings…"
          onBlur={() => setTimeout(() => setOpen(false), 150)}
          onFocus={() => results.length > 0 && setOpen(true)}
        />
      </div>

      {open && results.length > 0 && (
        <div
          style={{
            width: 320,
            margin: '4px auto 0',
            background: 'var(--bg-surface)',
            border: '1px solid var(--border-strong)',
            borderRadius: 'var(--radius-md)',
            boxShadow: 'var(--shadow-md)',
            overflow: 'hidden',
          }}
        >
          {results.map(r => (
            <button
              key={`${r.kind}-${r.id}`}
              onMouseDown={() => handleSelect(r)}
              style={{
                display: 'block',
                width: '100%',
                textAlign: 'left',
                padding: '10px 14px',
                border: 'none',
                borderBottom: '1px solid var(--border)',
                background: 'none',
                cursor: 'pointer',
              }}
              onMouseEnter={e => { e.currentTarget.style.background = 'var(--bg-hover)' }}
              onMouseLeave={e => { e.currentTarget.style.background = 'none' }}
            >
              <div style={{ font: 'var(--text-h4)', color: 'var(--fg1)' }}>{r.title}</div>
              <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', marginTop: 2 }}>
                {r.snippet}
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

export function Dashboard() {
  const { stats, tasks, meetings } = useAppState()
  const { setEditingTaskId, openModal, setActiveMeetingId, setView } = useAppActions()

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
  function navTo(view: ViewName) { setView(view) }

  return (
    <div className="content-pad">
      {/* Stat cards */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(4, 1fr)',
          gap: 16,
          marginBottom: 24,
        }}
      >
        <StatCard
          icon={ListTodo}
          iconBg="var(--indigo-50)"
          iconFg="var(--indigo-700)"
          value={stats?.total_tasks ?? 0}
          label="Total Tasks"
          onClick={() => navTo('tasks')}
        />
        <StatCard
          icon={CircleDot}
          iconBg="var(--amber-50)"
          iconFg="var(--amber-700)"
          value={stats?.in_progress_tasks ?? 0}
          label="In Progress"
          onClick={() => navTo('tasks')}
        />
        <StatCard
          icon={CircleAlert}
          iconBg="var(--rose-50)"
          iconFg="var(--rose-700)"
          value={stats?.blocked_tasks ?? 0}
          label="Blocked"
          emphasize={!!stats?.blocked_tasks}
          onClick={() => navTo('tasks')}
        />
        <StatCard
          icon={CalendarCheck}
          iconBg="var(--emerald-50)"
          iconFg="var(--emerald-700)"
          value={stats?.total_meetings ?? 0}
          label="Meetings"
          onClick={() => navTo('meetings')}
        />
      </div>

      {/* Two-column layout */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '1.4fr 1fr',
          gap: 16,
          alignItems: 'start',
        }}
      >
        {/* Blocked / In Progress */}
        <section className="card" style={{ padding: 0 }}>
          <header
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              padding: '16px 18px',
              borderBottom: '1px solid var(--border)',
            }}
          >
            <CircleAlert size={17} color="var(--block-fg)" />
            <h3 style={{ font: 'var(--text-h3)', margin: 0 }}>Blocked / In Progress</h3>
            <span className="chip chip-block" style={{ marginLeft: 4 }}>
              {activeTasks.length}
            </span>
            <button
              className="btn btn-ghost btn-sm"
              style={{ marginLeft: 'auto' }}
              onClick={() => navTo('tasks')}
            >
              View all
            </button>
          </header>

          {activeTasks.length === 0 ? (
            <div
              style={{
                textAlign: 'center',
                padding: '32px 18px',
                font: 'var(--text-sm)',
                color: 'var(--fg3)',
              }}
            >
              No active tasks. You're all clear.
            </div>
          ) : (
            <div>
              {activeTasks.map((t, i) => (
                <button
                  key={t.id}
                  onClick={() => openTask(t.id)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 12,
                    width: '100%',
                    textAlign: 'left',
                    padding: '13px 18px',
                    border: 'none',
                    background: 'none',
                    borderBottom: i < activeTasks.length - 1 ? '1px solid var(--border)' : 'none',
                    cursor: 'pointer',
                  }}
                  onMouseEnter={e => { e.currentTarget.style.background = 'var(--bg-hover)' }}
                  onMouseLeave={e => { e.currentTarget.style.background = 'none' }}
                >
                  <span
                    style={{
                      width: 8,
                      height: 8,
                      borderRadius: 2,
                      background: t.status === 'blocked' ? 'var(--block-solid)' : 'var(--amber-500)',
                      flex: 'none',
                    }}
                  />
                  <div style={{ minWidth: 0, flex: 1 }}>
                    <div
                      style={{
                        font: 'var(--text-h4)',
                        color: 'var(--fg1)',
                        whiteSpace: 'nowrap',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                      }}
                    >
                      {t.title}
                    </div>
                    {t.project_name && (
                      <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', marginTop: 2 }}>
                        {t.project_name}
                      </div>
                    )}
                  </div>
                  <StatusBadge status={t.status as Status} />
                  {t.owner ? (
                    <Avatar name={t.owner} size={26} />
                  ) : (
                    <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>—</span>
                  )}
                </button>
              ))}
            </div>
          )}
        </section>

        {/* Recent Meetings */}
        <section className="card" style={{ padding: 0 }}>
          <header
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              padding: '16px 18px',
              borderBottom: '1px solid var(--border)',
            }}
          >
            <Users size={17} color="var(--indigo-700)" />
            <h3 style={{ font: 'var(--text-h3)', margin: 0 }}>Recent Meetings</h3>
            <button
              className="btn btn-ghost btn-sm"
              style={{ marginLeft: 'auto' }}
              onClick={() => navTo('meetings')}
            >
              All meetings
            </button>
          </header>

          {recentMeetings.length === 0 ? (
            <div
              style={{
                textAlign: 'center',
                padding: '32px 18px',
                font: 'var(--text-sm)',
                color: 'var(--fg3)',
              }}
            >
              No meetings yet.
            </div>
          ) : (
            <div>
              {recentMeetings.map((m, i) => (
                <button
                  key={m.id}
                  onClick={() => openMeeting(m.id)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 12,
                    width: '100%',
                    textAlign: 'left',
                    padding: '13px 18px',
                    border: 'none',
                    background: 'none',
                    borderBottom: i < recentMeetings.length - 1 ? '1px solid var(--border)' : 'none',
                    cursor: 'pointer',
                  }}
                  onMouseEnter={e => { e.currentTarget.style.background = 'var(--bg-hover)' }}
                  onMouseLeave={e => { e.currentTarget.style.background = 'none' }}
                >
                  <span
                    style={{
                      font: 'var(--text-caption)',
                      color: 'var(--fg3)',
                      fontFamily: 'var(--font-mono)',
                      minWidth: 80,
                      flex: 'none',
                    }}
                  >
                    {fmt(m.date)}
                  </span>
                  <span
                    style={{
                      font: 'var(--text-h4)',
                      color: 'var(--fg1)',
                      flex: 1,
                      whiteSpace: 'nowrap',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                    }}
                  >
                    {m.title}
                  </span>
                  <span className="chip chip-todo" style={{ fontSize: 11 }}>
                    {m.task_count ?? 0} tasks
                  </span>
                </button>
              ))}
            </div>
          )}
        </section>
      </div>

      {/* Search panel */}
      <DashboardSearch
        onSelectTask={id => openTask(id)}
        onSelectMeeting={id => openMeeting(id)}
      />
    </div>
  )
}
