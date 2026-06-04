import type { Status } from '../../types'

const LABELS: Record<Status, string> = {
  todo: 'To Do',
  in_progress: 'In Progress',
  blocked: 'Blocked',
  done: 'Done',
  cancelled: 'Cancelled',
}

const STATUS_CLASS: Record<Status, string> = {
  todo: 'status-badge-todo',
  in_progress: 'status-badge-in_progress',
  blocked: 'status-badge-blocked',
  done: 'status-badge-done',
  cancelled: 'status-badge-cancelled',
}

export function StatusBadge({ status }: { status: Status }) {
  return (
    <span className={`status-badge ${STATUS_CLASS[status]}`}>
      {LABELS[status]}
    </span>
  )
}

export function statusLabel(s: Status): string {
  return LABELS[s] ?? s
}
