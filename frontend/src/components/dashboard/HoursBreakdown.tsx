import { useMemo } from 'react'
import type { DailyActivity } from '../../types'
import { groupHours, type HoursGroup } from './timeLogKpis'

interface HoursBreakdownProps {
  activities: DailyActivity[]
}

// HoursBreakdown shows where the period's hours went, by category and by
// project. Bars are relative to the largest group in each list.
export function HoursBreakdown({ activities }: HoursBreakdownProps) {
  const byCategory = useMemo(() => groupHours(activities, 'category'), [activities])
  const byProject = useMemo(() => groupHours(activities, 'project'), [activities])

  return (
    <section
      className="card"
      style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: 24, padding: 18 }}
    >
      <GroupList title="Horas por categoría" rows={byCategory} />
      <GroupList title="Horas por proyecto" rows={byProject} />
    </section>
  )
}

function GroupList({ title, rows }: { title: string; rows: HoursGroup[] }) {
  const max = rows.reduce((m, r) => Math.max(m, r.hours), 0)
  return (
    <div style={{ minWidth: 0 }}>
      <h3 style={{ font: 'var(--text-h3)', margin: '0 0 12px' }}>{title}</h3>
      {rows.length === 0 ? (
        <div style={{ font: 'var(--text-sm)', color: 'var(--fg3)' }}>Sin horas registradas en este período</div>
      ) : (
        <ul aria-label={title} style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 10 }}>
          {rows.map(r => (
            <li key={r.label}>
              <div style={{ display: 'flex', gap: 8, alignItems: 'baseline', font: 'var(--text-sm)' }}>
                <span
                  title={r.label}
                  style={{ flex: 1, minWidth: 0, color: 'var(--fg1)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
                >
                  {r.label}
                </span>
                <span style={{ color: 'var(--fg1)', fontWeight: 600 }}>{r.hours.toFixed(1)}h</span>
                <span style={{ color: 'var(--fg3)', minWidth: 44, textAlign: 'right' }}>{Math.round(r.pct)}%</span>
              </div>
              <div style={{ height: 6, marginTop: 4, borderRadius: 3, background: 'var(--bg-sunken)' }}>
                <div
                  style={{
                    width: `${max > 0 ? (r.hours / max) * 100 : 0}%`,
                    height: '100%',
                    borderRadius: 3,
                    background: 'var(--indigo-500)',
                  }}
                />
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
