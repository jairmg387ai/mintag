import { useEffect, useMemo, useState } from 'react'
import {
  ListTodo,
  CircleDot,
  CircleAlert,
  CalendarCheck,
  CalendarX,
  CloudUpload,
  Users,
  Clock,
  Target,
  TriangleAlert,
} from 'lucide-react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { listActivitiesRange } from '../../api/client'
import { StatCard } from '../shared/StatCard'
import { StatusBadge } from '../shared/StatusBadge'
import { Avatar } from '../shared/Avatar'
import type { DailyActivity, Status, ViewName } from '../../types'
import {
  type Period,
  expectedBusinessHours,
  filterByRange,
  gapDays,
  isBusinessDay,
  loadRange,
  pendingUpload,
  periodRange,
  round1,
  sumHours,
  toYMD,
} from './timeLogKpis'
import { PeriodSelector } from './PeriodSelector'
import { HoursBreakdown } from './HoursBreakdown'
import { WorkItemAlerts } from './WorkItemAlerts'

const PERIOD_KEY = 'mintag.dashboard.period'

function readStoredPeriod(): Period {
  try {
    return localStorage.getItem(PERIOD_KEY) === 'month' ? 'month' : 'week'
  } catch {
    return 'week'
  }
}

function storePeriod(p: Period) {
  try {
    localStorage.setItem(PERIOD_KEY, p)
  } catch {
    // Storage unavailable (private mode, blocked site data): keep it in memory only.
  }
}

function fmt(dt: string) {
  if (!dt) return '—'
  try {
    return new Date(dt).toLocaleDateString('es-CO', { day: '2-digit', month: 'short', year: 'numeric' })
  } catch {
    return dt
  }
}

// fmtDay renders a YYYY-MM-DD activity date as a short local weekday + day.
function fmtDay(ymd: string) {
  const [y, m, d] = ymd.split('-').map(Number)
  return new Date(y, m - 1, d).toLocaleDateString('es-CO', { weekday: 'short', day: '2-digit', month: 'short' })
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

  const [period, setPeriodState] = useState<Period>(readStoredPeriod)
  function setPeriod(p: Period) {
    setPeriodState(p)
    storePeriod(p)
  }

  // One fetch covers both the current week and month; each period filters it.
  const [loadedActivities, setLoadedActivities] = useState<DailyActivity[]>([])
  useEffect(() => {
    const range = loadRange(new Date())
    listActivitiesRange(toYMD(range.from), toYMD(range.to))
      .then(setLoadedActivities)
      .catch(() => setLoadedActivities([]))
  }, [])

  const weekActivities = useMemo(() => {
    const r = periodRange('week', new Date())
    return filterByRange(loadedActivities, r.from, r.to)
  }, [loadedActivities])

  const timeLogKpis = useMemo(() => {
    const today = new Date()
    const range = periodRange(period, today)
    const activities = filterByRange(loadedActivities, range.from, range.to)

    const expectedHours = expectedBusinessHours(range.from, range.to)
    const registeredHours = sumHours(activities)
    const compliancePct = expectedHours > 0 ? Math.round((registeredHours / expectedHours) * 100) : 0
    const todayStr = toYMD(today)
    const loggedToday = loadedActivities.some(a => a.date === todayStr)

    return {
      activities,
      expectedHours,
      registeredHours,
      compliancePct,
      gaps: gapDays(activities, range.from, today),
      showTodayAlert: isBusinessDay(today) && !loggedToday,
    }
  }, [loadedActivities, period])

  // Approved-not-uploaded over the whole loaded range (week ∪ month).
  const upload = useMemo(() => pendingUpload(loadedActivities), [loadedActivities])
  const periodLabel = period === 'week' ? 'semana' : 'mes'

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

      {/* Time log KPIs */}
      {timeLogKpis.showTodayAlert && (
        <button
          onClick={() => navTo('activities')}
          className="card"
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            width: '100%',
            textAlign: 'left',
            padding: '12px 18px',
            marginBottom: 16,
            border: '1px solid var(--amber-200)',
            background: 'var(--amber-50)',
            cursor: 'pointer',
          }}
        >
          <TriangleAlert size={18} color="var(--amber-700)" style={{ flex: 'none' }} />
          <span style={{ font: 'var(--text-sm)', color: 'var(--amber-700)', fontWeight: 600 }}>
            Todavía no registraste actividades hoy — hazlo antes de cerrar el día.
          </span>
        </button>
      )}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
        <h3 style={{ font: 'var(--text-h3)', margin: 0 }}>Registro de horas</h3>
        <div style={{ marginLeft: 'auto' }}>
          <PeriodSelector value={period} onChange={setPeriod} />
        </div>
      </div>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
          gap: 16,
          marginBottom: 16,
        }}
      >
        <StatCard
          icon={Clock}
          iconBg="var(--indigo-50)"
          iconFg="var(--indigo-700)"
          value={`${timeLogKpis.registeredHours}h`}
          label={`Horas registradas (${periodLabel})`}
          onClick={() => navTo('activities')}
        />
        <StatCard
          icon={Target}
          iconBg="var(--emerald-50)"
          iconFg="var(--emerald-700)"
          value={`${timeLogKpis.expectedHours}h`}
          label={`Meta ${period === 'week' ? 'de la semana' : 'del mes'} (L-J 8h, V 7.5h)`}
          onClick={() => navTo('activities')}
        />
        <StatCard
          icon={CircleDot}
          iconBg={timeLogKpis.compliancePct < 90 ? 'var(--rose-50)' : 'var(--emerald-50)'}
          iconFg={timeLogKpis.compliancePct < 90 ? 'var(--rose-700)' : 'var(--emerald-700)'}
          value={`${timeLogKpis.compliancePct}%`}
          label="Cumplimiento de registro"
          emphasize={timeLogKpis.compliancePct < 90}
          onClick={() => navTo('activities')}
        />
        <StatCard
          icon={CloudUpload}
          iconBg="var(--amber-50)"
          iconFg="var(--amber-700)"
          value={`${upload.approvedHours}h`}
          label="Pendiente de subir"
          delta={`${upload.approvedCount} ${upload.approvedCount === 1 ? 'aprobada' : 'aprobadas'}${
            upload.pendingHours > 0 ? ` · ${upload.pendingHours}h sin aprobar` : ''
          }`}
          onClick={() => navTo('activities')}
        />
        <StatCard
          icon={CalendarX}
          iconBg={timeLogKpis.gaps.length > 0 ? 'var(--rose-50)' : 'var(--emerald-50)'}
          iconFg={timeLogKpis.gaps.length > 0 ? 'var(--rose-700)' : 'var(--emerald-700)'}
          value={timeLogKpis.gaps.length}
          label="Días por debajo de la meta"
          emphasize={timeLogKpis.gaps.length > 0}
          onClick={() => navTo('activities')}
        />
      </div>

      <div style={{ marginBottom: 16 }}>
        <HoursBreakdown activities={timeLogKpis.activities} />
      </div>

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
          gap: 16,
          alignItems: 'start',
          marginBottom: 24,
        }}
      >
        <section className="card" style={{ padding: 18 }}>
          <h3 style={{ font: 'var(--text-h3)', margin: '0 0 10px' }}>Días bajo la meta</h3>
          {timeLogKpis.gaps.length === 0 ? (
            <div style={{ font: 'var(--text-sm)', color: 'var(--fg3)' }}>
              Todos los días hábiles del período cumplen la meta.
            </div>
          ) : (
            <ul
              aria-label="Días bajo la meta"
              style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 6 }}
            >
              {timeLogKpis.gaps.map(g => (
                <li key={g.date} style={{ display: 'flex', gap: 8, font: 'var(--text-sm)' }}>
                  <span style={{ flex: 1, color: 'var(--fg1)' }}>{fmtDay(g.date)}</span>
                  <span style={{ color: 'var(--rose-700)', fontWeight: 600 }}>
                    {round1(g.hours)}h / {g.target}h
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>
        <WorkItemAlerts weekActivities={weekActivities} />
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
