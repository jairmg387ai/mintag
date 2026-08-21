import { Clock, CircleDot, CheckCircle, Circle } from 'lucide-react'

interface AzureWorkItemStateBadgeProps {
  state: string
}

type StateMeta = { label: string; cls: string; icon: React.ElementType }

// Azure states are process-template-specific (CMMI/Agile/Scrum all differ),
// so this maps loosely by bucket rather than exhaustively — anything not
// recognized falls through to a neutral chip showing the raw state text.
const STATE_META: Record<string, StateMeta> = {
  Proposed: { label: 'Proposed', cls: 'chip-pending', icon: Clock },
  New:      { label: 'New',      cls: 'chip-pending', icon: Clock },
  Active:   { label: 'Active',   cls: 'chip-prog',     icon: CircleDot },
  Accepted: { label: 'Accepted', cls: 'chip-prog',     icon: CircleDot },
  Closed:   { label: 'Closed',   cls: 'chip-done',     icon: CheckCircle },
  Cerrado:  { label: 'Cerrado',  cls: 'chip-done',     icon: CheckCircle },
}

export function AzureWorkItemStateBadge({ state }: AzureWorkItemStateBadgeProps) {
  const meta = STATE_META[state] ?? { label: state, cls: 'chip-todo', icon: Circle }
  const Icon = meta.icon
  return (
    <span className={`chip ${meta.cls}`}>
      <Icon size={13} strokeWidth={1.75} />
      {meta.label}
    </span>
  )
}
