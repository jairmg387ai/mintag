import { useEffect, useMemo, useState, type CSSProperties } from 'react'
import { ChevronLeft, ChevronRight, AlertTriangle } from 'lucide-react'
import type { DailyActivity } from '../../types'
import { listActivitiesRange } from '../../api/client'
import { useAppActions } from '../../store/AppContext'
import { toYMD } from '../dashboard/timeLogKpis'
import { ActivityStatusBadge } from './ActivityStatusBadge'
import { buildMonthCalendar, monthRange, type CalendarDay, type CalendarDayState } from './activityCalendar'

const WEEKDAY_SHORT = ['Do', 'Lu', 'Ma', 'Mi', 'Ju', 'Vi', 'Sá']

const ROW_BG: Record<CalendarDayState, string> = {
  complete: 'transparent',
  pending: 'transparent',
  short: 'var(--amber-50)',
  empty: 'var(--block-bg)',
  weekend: 'var(--bg-sunken)',
}

function fmtHours(h: number): string {
  return `${h.toFixed(1)}h`
}

// Read-only month overview of logged activities: one row per day, top to
// bottom, with a subtotal row closing each Monday-Sunday week. Clicking a day
// opens the existing daily ActivitiesView on that date, where approving,
// editing and uploading keep living.
export function ActivityCalendarView() {
  const { openActivitiesOn } = useAppActions()
  const [month, setMonth] = useState(() => {
    const now = new Date()
    return { year: now.getFullYear(), month: now.getMonth() }
  })
  // The fetched month is tagged with its key so `loading` derives from it
  // (no synchronous setState inside the effect) and a response that arrives
  // after the user already moved to another month is simply ignored.
  const monthKey = `${month.year}-${month.month}`
  const [result, setResult] = useState<{ key: string; activities: DailyActivity[]; error: string | null } | null>(null)

  useEffect(() => {
    let ignore = false
    const { from, to } = monthRange(month.year, month.month)
    listActivitiesRange(toYMD(from), toYMD(to))
      .then(data => { if (!ignore) setResult({ key: monthKey, activities: data, error: null }) })
      .catch((e: unknown) => {
        if (ignore) return
        const message = e instanceof Error ? e.message : 'No se pudieron cargar las actividades'
        setResult({ key: monthKey, activities: [], error: message })
      })
    return () => { ignore = true }
  }, [month, monthKey])

  const current = result?.key === monthKey ? result : null
  const loading = current === null
  const activities = useMemo(() => current?.activities ?? [], [current])
  const error = current?.error ?? null

  const calendar = useMemo(
    () => buildMonthCalendar(month.year, month.month, activities, new Date()),
    [month, activities],
  )

  function shiftMonth(delta: number) {
    setMonth(m => {
      const d = new Date(m.year, m.month + delta, 1)
      return { year: d.getFullYear(), month: d.getMonth() }
    })
  }

  function goToCurrentMonth() {
    const now = new Date()
    setMonth({ year: now.getFullYear(), month: now.getMonth() })
  }

  const title = new Date(month.year, month.month, 1).toLocaleDateString('es-CO', { month: 'long', year: 'numeric' })

  return (
    <div className="content-pad">
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16, flexWrap: 'wrap' }}>
        <button className="btn btn-ghost btn-sm" onClick={() => shiftMonth(-1)} aria-label="Mes anterior">
          <ChevronLeft size={16} strokeWidth={1.75} />
        </button>
        <h2 style={{ font: 'var(--text-h2)', color: 'var(--fg1)', margin: 0, minWidth: 200, textAlign: 'center', textTransform: 'capitalize' }}>
          {title}
        </h2>
        <button className="btn btn-ghost btn-sm" onClick={() => shiftMonth(1)} aria-label="Mes siguiente">
          <ChevronRight size={16} strokeWidth={1.75} />
        </button>
        <button className="btn btn-ghost btn-sm" onClick={goToCurrentMonth}>Hoy</button>

        <div style={{ marginLeft: 'auto', display: 'flex', gap: 16, alignItems: 'center', font: 'var(--text-body)', color: 'var(--fg2)' }}>
          <span>
            Total del mes: <strong style={{ color: 'var(--fg1)' }}>{fmtHours(calendar.hours)}</strong> / {fmtHours(calendar.target)}
          </span>
          {calendar.incompleteDays > 0 && (
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, color: 'var(--block-solid)' }}>
              <AlertTriangle size={15} strokeWidth={1.75} />
              {calendar.incompleteDays} {calendar.incompleteDays === 1 ? 'día incompleto' : 'días incompletos'}
            </span>
          )}
        </div>
      </div>

      {error && (
        <div style={{ font: 'var(--text-caption)', color: 'var(--block-solid)', marginBottom: 12 }}>{error}</div>
      )}

      <div style={{ border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)', overflow: 'hidden', opacity: loading ? 0.6 : 1 }}>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <tbody>
            {calendar.weeks.map(week => (
              <WeekRows
                key={week.days[0].date}
                days={week.days}
                hours={week.hours}
                target={week.target}
                incompleteDays={week.incompleteDays}
                onOpenDay={openActivitiesOn}
              />
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

interface WeekRowsProps {
  days: CalendarDay[]
  hours: number
  target: number
  incompleteDays: number
  onOpenDay: (date: string) => void
}

function WeekRows({ days, hours, target, incompleteDays, onOpenDay }: WeekRowsProps) {
  const first = days[0]
  const last = days[days.length - 1]
  return (
    <>
      {days.map(d => <DayRow key={d.date} day={d} onOpen={onOpenDay} />)}
      <tr style={{ background: 'var(--bg-sunken)', borderBottom: '2px solid var(--border-strong)' }}>
        <td colSpan={2} style={{ padding: '8px 12px', font: 'var(--text-caption)', color: 'var(--fg2)', fontWeight: 600 }}>
          Semana del {first.dayOfMonth} al {last.dayOfMonth}
          {incompleteDays > 0 && (
            <span style={{ marginLeft: 8, color: 'var(--block-solid)', fontWeight: 400 }}>
              · {incompleteDays} {incompleteDays === 1 ? 'día incompleto' : 'días incompletos'}
            </span>
          )}
        </td>
        <td style={{ padding: '8px 12px', textAlign: 'right', font: 'var(--text-body)', color: 'var(--fg1)', fontWeight: 600, whiteSpace: 'nowrap' }}>
          {fmtHours(hours)} <span style={{ color: 'var(--fg3)', fontWeight: 400 }}>/ {fmtHours(target)}</span>
        </td>
      </tr>
    </>
  )
}

function DayRow({ day, onOpen }: { day: CalendarDay; onOpen: (date: string) => void }) {
  const flagged = day.state === 'empty' || day.state === 'short'
  const label = new Date(`${day.date}T00:00:00`).toLocaleDateString('es-CO', { weekday: 'long', day: 'numeric', month: 'long' })
  return (
    <tr
      onClick={() => onOpen(day.date)}
      style={{ background: ROW_BG[day.state], borderBottom: '1px solid var(--border)', cursor: 'pointer' }}
    >
      <td style={{ width: 72, padding: '8px 12px', whiteSpace: 'nowrap', verticalAlign: 'top' }}>
        <button
          type="button"
          onClick={e => { e.stopPropagation(); onOpen(day.date) }}
          aria-label={`Ver actividades del ${label}`}
          style={{ all: 'unset', cursor: 'pointer', display: 'flex', gap: 6, alignItems: 'baseline' }}
        >
          <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)', width: 18 }}>{WEEKDAY_SHORT[day.weekday]}</span>
          <span style={{ font: 'var(--text-body)', fontWeight: 600, color: day.isToday ? 'var(--brand)' : 'var(--fg1)' }}>
            {day.dayOfMonth}
          </span>
        </button>
      </td>
      <td style={{ padding: '6px 12px' }}>
        {day.activities.length > 0 ? (
          <DayActivitiesTable activities={day.activities} label={label} />
        ) : day.state === 'empty' && (
          <span style={{ font: 'var(--text-caption)', color: 'var(--block-solid)' }}>Sin registros</span>
        )}
      </td>
      <td style={{ width: 120, padding: '8px 12px', textAlign: 'right', whiteSpace: 'nowrap', verticalAlign: 'top' }}>
        {day.state === 'weekend' && day.hours === 0 ? (
          <span style={{ color: 'var(--fg3)' }}>–</span>
        ) : (
          <span style={{ font: 'var(--text-body)', color: flagged ? 'var(--block-solid)' : 'var(--fg1)', fontWeight: flagged ? 600 : 400 }}>
            {flagged && <AlertTriangle size={13} strokeWidth={1.75} style={{ marginRight: 4, verticalAlign: '-2px' }} />}
            {fmtHours(day.hours)}
            {day.target > 0 && <span style={{ color: 'var(--fg3)', fontWeight: 400 }}> / {fmtHours(day.target)}</span>}
          </span>
        )}
      </td>
    </tr>
  )
}

const CELL: CSSProperties = {
  padding: '3px 8px',
  font: 'var(--text-caption)',
  color: 'var(--fg2)',
  borderBottom: '1px dashed var(--border)',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
  verticalAlign: 'middle',
}

// Same columns as the daily ActivitiesView list, compacted and read-only.
// Fixed layout keeps the columns aligned from one day to the next.
function DayActivitiesTable({ activities, label }: { activities: DailyActivity[]; label: string }) {
  return (
    <table aria-label={`Actividades del ${label}`} style={{ width: '100%', borderCollapse: 'collapse', tableLayout: 'fixed' }}>
      <colgroup>
        <col style={{ width: '18%' }} />
        <col style={{ width: 56 }} />
        <col style={{ width: '18%' }} />
        <col />
        <col style={{ width: 112 }} />
      </colgroup>
      <tbody>
        {activities.map(a => (
          <tr key={a.id}>
            <td style={{ ...CELL, color: 'var(--fg1)', fontWeight: 600 }} title={a.project}>{a.project}</td>
            <td style={{ ...CELL, font: 'var(--text-mono)', color: 'var(--fg1)', textAlign: 'right' }}>{a.hours.toFixed(2)}</td>
            <td style={CELL} title={a.category}>{a.category}</td>
            <td style={CELL} title={a.registro_diario || undefined}>{a.registro_diario}</td>
            <td style={{ ...CELL, textOverflow: 'clip' }}><ActivityStatusBadge status={a.status} /></td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
