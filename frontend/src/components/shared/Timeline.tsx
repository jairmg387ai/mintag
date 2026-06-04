import type { TaskHistory } from '../../types'
import { StatusBadge } from './StatusBadge'
import type { Status } from '../../types'

function fmt(dt: string) {
  if (!dt) return '—'
  try {
    return new Date(dt).toLocaleString('en-US', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return dt
  }
}

function dotColor(status: string): string {
  if (status === 'done') return 'var(--color-green)'
  if (status === 'blocked') return 'var(--color-red)'
  if (status === 'in_progress') return 'var(--color-blue)'
  return 'var(--color-blue)'
}

export function Timeline({ history }: { history: TaskHistory[] }) {
  return (
    <div style={{ position: 'relative', paddingLeft: 22 }}>
      <div style={{ content: '', position: 'absolute', left: 7, top: 8, bottom: 0, width: 2, background: 'var(--color-border)' }} />
      {history.map(h => (
        <div key={h.id} style={{ position: 'relative', marginBottom: 18 }}>
          <div style={{
            position: 'absolute',
            left: -18,
            top: 4,
            width: 12,
            height: 12,
            borderRadius: '50%',
            border: '2px solid var(--color-bg)',
            background: dotColor(h.new_status),
          }} />
          <div style={{ fontSize: '0.72em', color: 'var(--color-text3)', marginBottom: 2 }}>
            {fmt(h.created_at)}
            {h.author ? ` · ${h.author}` : ''}
            {h.source_meeting_title ? ` · 📅 ${h.source_meeting_title}` : ''}
          </div>
          <div style={{ background: 'var(--color-surface2)', borderRadius: 8, padding: '10px 14px' }}>
            {h.old_status !== h.new_status && h.old_status && (
              <div style={{ fontSize: '0.78em', fontWeight: 600, marginBottom: 4, display: 'flex', alignItems: 'center', gap: 6 }}>
                <StatusBadge status={h.old_status as Status} />
                <span>→</span>
                <StatusBadge status={h.new_status as Status} />
              </div>
            )}
            {h.note && <div style={{ fontSize: '0.83em', color: 'var(--color-text2)' }}>{h.note}</div>}
          </div>
        </div>
      ))}
    </div>
  )
}
