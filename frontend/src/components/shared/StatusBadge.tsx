import { Circle, CircleDot, CircleAlert, CircleCheck, Ban, FlaskConical } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { Status } from '../../types'

interface StatusMeta {
  label: string
  cls: string
  icon: LucideIcon
}

export const STATUS_META: Record<Status, StatusMeta> = {
  todo:        { label: 'To Do',       cls: 'chip-todo',  icon: Circle },
  in_progress: { label: 'In Progress', cls: 'chip-prog',  icon: CircleDot },
  blocked:     { label: 'Blocked',     cls: 'chip-block', icon: CircleAlert },
  in_testing:  { label: 'In Testing',  cls: 'chip-test',  icon: FlaskConical },
  done:        { label: 'Done',        cls: 'chip-done',  icon: CircleCheck },
  cancelled:   { label: 'Cancelled',   cls: 'chip-todo',  icon: Ban },
}

export function StatusBadge({ status }: { status: Status }) {
  const meta = STATUS_META[status] ?? STATUS_META.todo
  const Icon = meta.icon
  return (
    <span className={`chip ${meta.cls}`}>
      <Icon size={13} strokeWidth={1.75} />
      {meta.label}
    </span>
  )
}

export function statusLabel(s: Status): string {
  return STATUS_META[s]?.label ?? s
}
