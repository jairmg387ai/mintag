import { useEffect, useMemo, useState } from 'react'
import { Gauge, Hourglass } from 'lucide-react'
import { fetchWorkItemEffort, listAzureActivities } from '../../api/client'
import type { AzureActivity, DailyActivity, WorkItemEffort } from '../../types'
import { idleWorkItems, openWorkItems, overBudgetWorkItems } from './timeLogKpis'

interface WorkItemAlertsProps {
  // Activities of the current week, used to spot work items without hours.
  weekActivities: DailyActivity[]
}

const muted = { font: 'var(--text-sm)', color: 'var(--fg3)' } as const

// WorkItemAlerts flags open catalog work items with no hours this week and
// those that already consumed >= 80% of their estimate. Each section fails
// silently with a muted note so the dashboard never breaks on these reads.
export function WorkItemAlerts({ weekActivities }: WorkItemAlertsProps) {
  const [catalog, setCatalog] = useState<AzureActivity[] | null>(null)
  const [catalogFailed, setCatalogFailed] = useState(false)
  const [effort, setEffort] = useState<WorkItemEffort[] | null>(null)
  const [effortFailed, setEffortFailed] = useState(false)

  useEffect(() => {
    let cancelled = false
    listAzureActivities()
      .then(list => {
        if (cancelled) return
        setCatalog(list)
        const ids = Array.from(new Set(openWorkItems(list).map(w => w.work_item_id)))
        return fetchWorkItemEffort(ids)
          .then(res => { if (!cancelled) setEffort(res.items ?? []) })
          .catch(() => { if (!cancelled) setEffortFailed(true) })
      })
      .catch(() => { if (!cancelled) setCatalogFailed(true) })
    return () => { cancelled = true }
  }, [])

  const idle = useMemo(() => (catalog ? idleWorkItems(catalog, weekActivities) : []), [catalog, weekActivities])
  const overBudget = useMemo(
    () => (catalog && effort ? overBudgetWorkItems(catalog, effort) : []),
    [catalog, effort],
  )

  return (
    <section className="card" style={{ padding: 18, display: 'grid', gap: 18 }}>
      <div>
        <h3 style={{ font: 'var(--text-h3)', margin: '0 0 10px', display: 'flex', alignItems: 'center', gap: 8 }}>
          <Hourglass size={17} color="var(--amber-700)" />
          Sin horas esta semana
        </h3>
        {catalogFailed ? (
          <div style={muted}>No se pudo cargar el catálogo de work items.</div>
        ) : catalog === null ? null : idle.length === 0 ? (
          <div style={muted}>Todos los work items activos tienen horas esta semana.</div>
        ) : (
          <ul aria-label="Work items sin horas esta semana" style={listStyle}>
            {idle.map(w => (
              <li key={w.id} style={rowStyle}>
                <span style={idStyle}>#{w.work_item_id}</span>
                <span style={labelStyle} title={w.label}>{w.label}</span>
              </li>
            ))}
          </ul>
        )}
      </div>

      {!catalogFailed && (
        <div>
          <h3 style={{ font: 'var(--text-h3)', margin: '0 0 10px', display: 'flex', alignItems: 'center', gap: 8 }}>
            <Gauge size={17} color="var(--rose-700)" />
            80% o más del estimado
          </h3>
          {effortFailed ? (
            <div style={muted}>No se pudo calcular el consumo de los work items.</div>
          ) : effort === null ? null : overBudget.length === 0 ? (
            <div style={muted}>Ningún work item supera el 80% del estimado.</div>
          ) : (
            <ul aria-label="Work items con 80% o más del estimado" style={listStyle}>
              {overBudget.map(r => {
                const over = r.pct >= 100
                return (
                  <li key={r.workItemId} data-over={over ? 'true' : 'false'} style={rowStyle}>
                    <span style={idStyle}>#{r.workItemId}</span>
                    <span style={labelStyle} title={r.label}>{r.label}</span>
                    <span
                      style={{
                        font: 'var(--text-caption)',
                        fontWeight: 600,
                        color: over ? 'var(--rose-700)' : 'var(--amber-700)',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      usado {r.used} de {r.estimate}h ({r.pct}%)
                    </span>
                  </li>
                )
              })}
            </ul>
          )}
        </div>
      )}
    </section>
  )
}

const listStyle = { listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 8 } as const
const rowStyle = { display: 'flex', alignItems: 'baseline', gap: 8, font: 'var(--text-sm)' } as const
const idStyle = { color: 'var(--fg3)', fontFamily: 'var(--font-mono)', flex: 'none' } as const
const labelStyle = {
  flex: 1,
  minWidth: 0,
  color: 'var(--fg1)',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
} as const
