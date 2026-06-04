import { Calendar, History } from 'lucide-react'
import type { TaskHistory } from '../../types'
import { StatusBadge } from './StatusBadge'
import type { Status } from '../../types'

function fmt(dt: string) {
  if (!dt) return '—'
  try {
    return new Date(dt).toLocaleString('en-US', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return dt
  }
}

export function Timeline({ history }: { history: TaskHistory[] }) {
  return (
    <div className="timeline">
      {history.map(h => (
        <div key={h.id} className="tl-item">
          <div className="tl-rail">
            <div className="tl-node">
              <History size={14} strokeWidth={1.75} style={{ color: 'var(--fg3)' }} />
            </div>
          </div>
          <div className="tl-body">
            <div
              style={{
                fontSize: '0.72em',
                color: 'var(--fg3)',
                marginBottom: 4,
                fontFamily: 'var(--font-mono)',
              }}
            >
              {fmt(h.created_at)}
              {h.author ? ` · ${h.author}` : ''}
              {h.source_meeting_title && (
                <>
                  {' · '}
                  <Calendar size={11} strokeWidth={1.75} style={{ display: 'inline', verticalAlign: 'middle', marginRight: 3 }} />
                  {h.source_meeting_title}
                </>
              )}
            </div>
            <div
              style={{
                background: 'var(--bg-sunken)',
                borderRadius: 'var(--radius-md)',
                padding: '10px 14px',
              }}
            >
              {h.old_status !== h.new_status && h.old_status && (
                <div
                  style={{
                    fontSize: '0.78em',
                    fontWeight: 600,
                    marginBottom: 4,
                    display: 'flex',
                    alignItems: 'center',
                    gap: 6,
                  }}
                >
                  <StatusBadge status={h.old_status as Status} />
                  <span style={{ color: 'var(--fg3)' }}>{'→'}</span>
                  <StatusBadge status={h.new_status as Status} />
                </div>
              )}
              {h.note && (
                <div style={{ fontSize: '0.83em', color: 'var(--fg2)' }}>{h.note}</div>
              )}
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
