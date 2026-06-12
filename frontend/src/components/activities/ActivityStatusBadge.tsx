import { Clock, CheckCircle, Upload, Sparkles, PenLine } from 'lucide-react'
import type { ActivityStatus, ActivitySource } from '../../types'

interface ActivityStatusBadgeProps {
  status: ActivityStatus
}

const STATUS_META: Record<ActivityStatus, { label: string; cls: string; icon: React.ElementType }> = {
  pending:  { label: 'Pending',  cls: 'chip-pending', icon: Clock },
  approved: { label: 'Approved', cls: 'chip-prog',    icon: CheckCircle },
  uploaded: { label: 'Uploaded', cls: 'chip-done',    icon: Upload },
}

export function ActivityStatusBadge({ status }: ActivityStatusBadgeProps) {
  const meta = STATUS_META[status] ?? STATUS_META.pending
  const Icon = meta.icon
  return (
    <span className={`chip ${meta.cls}`}>
      <Icon size={13} strokeWidth={1.75} />
      {meta.label}
    </span>
  )
}

interface SourceBadgeProps {
  source: ActivitySource
}

export function ActivitySourceBadge({ source }: SourceBadgeProps) {
  if (source === 'llm_auto') {
    return (
      <span className="chip chip-todo">
        <Sparkles size={13} strokeWidth={1.75} />
        LLM
      </span>
    )
  }
  return (
    <span className="chip chip-todo">
      <PenLine size={13} strokeWidth={1.75} />
      Manual
    </span>
  )
}
