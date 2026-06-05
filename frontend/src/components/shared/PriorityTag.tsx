import type { Priority } from '../../types'

interface PriorityMeta {
  label: string
  cls: string
}

const PRIORITY_META: Record<Priority, PriorityMeta> = {
  low:      { label: 'Low',      cls: 'pri-Low' },
  medium:   { label: 'Medium',   cls: 'pri-Medium' },
  high:     { label: 'High',     cls: 'pri-High' },
  critical: { label: 'Critical', cls: 'pri-High' },
}

export function PriorityTag({ priority }: { priority: Priority }) {
  const meta = PRIORITY_META[priority] ?? PRIORITY_META.low
  return (
    <span className={`pri ${meta.cls}`}>
      {meta.label}
    </span>
  )
}

export function priorityLabel(p: Priority): string {
  return PRIORITY_META[p]?.label ?? p
}
