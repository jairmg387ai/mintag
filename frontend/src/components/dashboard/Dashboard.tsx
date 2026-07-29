import { useMemo } from 'react'
import {
  ListTodo,
  CircleDot,
  CircleAlert,
  CalendarCheck,
  Users,
} from 'lucide-react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { StatCard } from '../shared/StatCard'
import { StatusBadge } from '../shared/StatusBadge'
import { Avatar } from '../shared/Avatar'
import type { Status, ViewName } from '../../types'

function fmt(dt: string) {
  if (!dt) return '—'
  try {
    return new Date(dt).toLocaleDateString('es-CO', { day: '2-digit', month: 'short', year: 'numeric' })
  } catch {
    return dt
  }
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
          label="Total de tareas"
          onClick={() => navTo('tasks')}
        />
        <StatCard
          icon={CircleDot}
          iconBg="var(--amber-50)"
          iconFg="var(--amber-700)"
          value={stats?.in_progress_tasks ?? 0}
          label="En progreso"
          onClick={() => navTo('tasks')}
        />
        <StatCard
          icon={CircleAlert}
          iconBg="var(--rose-50)"
          iconFg="var(--rose-700)"
          value={stats?.blocked_tasks ?? 0}
          label="Bloqueadas"
          emphasize={!!stats?.blocked_tasks}
          onClick={() => navTo('tasks')}
        />
        <StatCard
          icon={CalendarCheck}
          iconBg="var(--emerald-50)"
          iconFg="var(--emerald-700)"
          value={stats?.total_meetings ?? 0}
          label="Reuniones"
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
        {/* Bloqueadas / En progreso */}
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
            <h3 style={{ font: 'var(--text-h3)', margin: 0 }}>Bloqueadas / En progreso</h3>
            <span className="chip chip-block" style={{ marginLeft: 4 }}>
              {activeTasks.length}
            </span>
            <button
              className="btn btn-ghost btn-sm"
              style={{ marginLeft: 'auto' }}
              onClick={() => navTo('tasks')}
            >
              Ver todo
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
              No hay tareas activas. Todo está al día.
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

        {/* Reuniones recientes */}
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
            <h3 style={{ font: 'var(--text-h3)', margin: 0 }}>Reuniones recientes</h3>
            <button
              className="btn btn-ghost btn-sm"
              style={{ marginLeft: 'auto' }}
              onClick={() => navTo('meetings')}
            >
              Todas las reuniones
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
              Aún no hay reuniones.
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
                    {m.task_count ?? 0} tareas
                  </span>
                </button>
              ))}
            </div>
          )}
        </section>
      </div>

    </div>
  )
}
