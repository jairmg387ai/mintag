import type { Meeting } from '../../types'

interface MeetingCardProps {
  meeting: Meeting
  projectColor?: string
  projectName?: string
  onClick: () => void
}

function formatDate(dateStr: string): string {
  if (!dateStr) return '—'
  try {
    return new Date(dateStr).toLocaleDateString('en-US', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
    })
  } catch {
    return dateStr
  }
}

export function MeetingCard({ meeting: m, projectColor, projectName, onClick }: MeetingCardProps) {
  return (
    <div
      className="card"
      onClick={onClick}
      style={{ cursor: 'pointer', transition: 'box-shadow .15s, border-color .15s', padding: 'var(--space-4)' }}
      onMouseEnter={e => {
        e.currentTarget.style.boxShadow = 'var(--shadow-md)'
        e.currentTarget.style.borderColor = 'var(--border-strong)'
      }}
      onMouseLeave={e => {
        e.currentTarget.style.boxShadow = ''
        e.currentTarget.style.borderColor = 'var(--border)'
      }}
    >
      {/* Date */}
      <div
        style={{
          font: 'var(--text-caption)',
          color: 'var(--fg3)',
          marginBottom: 8,
          fontFamily: 'var(--font-mono)',
        }}
      >
        {formatDate(m.date)}
      </div>

      {/* Title */}
      <div style={{ font: 'var(--text-h4)', color: 'var(--fg1)', marginBottom: 8 }}>
        {m.title}
      </div>

      {/* Summary snippet */}
      {m.summary && (
        <div
          style={{
            font: 'var(--text-sm)',
            color: 'var(--fg2)',
            marginBottom: 12,
            display: '-webkit-box',
            WebkitLineClamp: 2,
            WebkitBoxOrient: 'vertical',
            overflow: 'hidden',
          }}
        >
          {m.summary}
        </div>
      )}

      {/* Footer */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <span className="chip chip-todo" style={{ fontSize: 11 }}>
          {m.task_count ?? 0} tasks
        </span>
        {projectColor && (
          <>
            <span
              style={{
                width: 8,
                height: 8,
                borderRadius: '50%',
                background: projectColor,
                display: 'inline-block',
                flex: 'none',
              }}
            />
            {projectName && (
              <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
                {projectName}
              </span>
            )}
          </>
        )}
      </div>
    </div>
  )
}
