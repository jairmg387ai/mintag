import type { CSSProperties } from 'react'
import type { WorkItemEffort as WorkItemEffortData } from '../../types'

// Remaining hours below this share of the estimate are shown as a warning.
const LOW_REMAINING_RATIO = 0.2

// formatHours renders hours compactly: 24 -> "24h", 2.5 -> "2.5h", 1/3 -> "0.33h".
function formatHours(hours: number): string {
  return `${Number(hours.toFixed(2))}h`
}

// isLowRemaining is true when a work item with an estimate has nothing left
// or less than LOW_REMAINING_RATIO of it.
function isLowRemaining(effort: WorkItemEffortData): boolean {
  if (!effort.has_estimate) return false
  return effort.remaining <= 0 || effort.remaining < effort.original_estimate * LOW_REMAINING_RATIO
}

interface WorkItemEffortProps {
  effort: WorkItemEffortData
  // Hours about to be logged; when they exceed the remaining hours of an
  // estimated work item a non-blocking warning is shown below the summary.
  enteredHours?: number
  style?: CSSProperties
}

// WorkItemEffort is a compact, presentational summary of one work item's
// effort: estimate, hours registered in TimeLog (plus local hours not
// uploaded yet) and remaining hours.
export function WorkItemEffort({ effort, enteredHours, style }: WorkItemEffortProps) {
  const low = isLowRemaining(effort)
  const exceeds =
    effort.has_estimate && enteredHours !== undefined && !isNaN(enteredHours) && enteredHours > effort.remaining
  const summary = (
    <span
      data-testid="work-item-effort"
      style={{ font: 'var(--text-caption)', color: 'var(--fg2)', ...style }}
    >
      <span>{effort.has_estimate ? `Estimado ${formatHours(effort.original_estimate)}` : 'Sin estimado'}</span>
      {' · '}
      <span>
        {`Registrado ${formatHours(effort.uploaded_hours)}`}
        {effort.local_hours > 0 && ` (+${formatHours(effort.local_hours)} sin subir)`}
      </span>
      {effort.has_estimate && (
        <>
          {' · '}
          <span
            data-warning={low ? 'true' : 'false'}
            style={low ? { color: 'var(--amber-700)', fontWeight: 600 } : undefined}
          >
            {`Quedan ${formatHours(effort.remaining)}`}
          </span>
        </>
      )}
    </span>
  )
  if (!exceeds) return summary
  return (
    <>
      {summary}
      <div
        role="status"
        style={{
          marginTop: 6,
          padding: '6px 10px',
          background: 'var(--amber-50)',
          color: 'var(--amber-700)',
          borderRadius: 'var(--radius-md)',
          font: 'var(--text-caption)',
        }}
      >
        {`Estas horas superan lo que queda en el work item (quedan ${formatHours(effort.remaining)}).`}
      </div>
    </>
  )
}
