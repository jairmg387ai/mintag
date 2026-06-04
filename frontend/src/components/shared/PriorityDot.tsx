import type { Priority } from '../../types'

const COLORS: Record<Priority, string> = {
  low: 'var(--color-text3)',
  medium: 'var(--color-blue)',
  high: 'var(--color-orange)',
  critical: 'var(--color-red)',
}

const LABELS: Record<Priority, string> = {
  low: 'Low',
  medium: 'Medium',
  high: 'High',
  critical: 'Critical',
}

export function PriorityDot({ priority }: { priority: Priority }) {
  return (
    <span
      title={LABELS[priority]}
      style={{
        width: 8,
        height: 8,
        borderRadius: '50%',
        background: COLORS[priority],
        flexShrink: 0,
        display: 'inline-block',
      }}
    />
  )
}

export function priorityLabel(p: Priority): string {
  return LABELS[p] ?? p
}
