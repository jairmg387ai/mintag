import type { Status } from '../../types'

const LABELS: Record<Status, string> = {
  todo: 'To Do',
  in_progress: 'In Progress',
  blocked: 'Blocked',
  done: 'Done',
  cancelled: 'Cancelled',
}

const STYLES: Record<Status, React.CSSProperties> = {
  todo: { background: 'var(--color-surface3)', color: 'var(--color-text2)' },
  in_progress: { background: 'rgba(59,130,246,0.12)', color: 'var(--color-blue)' },
  blocked: { background: 'rgba(239,68,68,0.12)', color: 'var(--color-red)' },
  done: { background: 'rgba(34,197,94,0.12)', color: 'var(--color-green)' },
  cancelled: { background: 'var(--color-surface3)', color: 'var(--color-text3)' },
}

import React from 'react'

export function StatusBadge({ status }: { status: Status }) {
  return (
    <span style={{
      display: 'inline-flex',
      alignItems: 'center',
      gap: 4,
      padding: '3px 9px',
      borderRadius: 20,
      fontSize: '0.75em',
      fontWeight: 600,
      whiteSpace: 'nowrap',
      ...STYLES[status],
    }}>
      {LABELS[status]}
    </span>
  )
}

export function statusLabel(s: Status): string {
  return LABELS[s] ?? s
}
