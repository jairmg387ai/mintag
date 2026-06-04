import type { Meeting } from '../../types'

interface MeetingCardProps {
  meeting: Meeting
  projectColor?: string
  projectName?: string
  onClick: () => void
}

export function MeetingCard({ meeting: m, projectColor, projectName, onClick }: MeetingCardProps) {
  return (
    <div
      onClick={onClick}
      style={{
        background: 'var(--color-surface)',
        border: '1px solid var(--color-border)',
        borderRadius: 10,
        padding: '14px 18px',
        display: 'flex',
        alignItems: 'center',
        gap: 14,
        cursor: 'pointer',
        transition: 'all 0.15s',
      }}
      onMouseEnter={e => { e.currentTarget.style.background = 'var(--color-surface2)'; e.currentTarget.style.borderColor = 'var(--color-border2)' }}
      onMouseLeave={e => { e.currentTarget.style.background = 'var(--color-surface)'; e.currentTarget.style.borderColor = 'var(--color-border)' }}
    >
      <span style={{ fontSize: '0.75em', color: 'var(--color-text3)', fontWeight: 600, minWidth: 84 }}>{m.date || '—'}</span>
      <span style={{ fontWeight: 500, fontSize: '0.92em', flex: 1 }}>{m.title}</span>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
        {(m.task_count ?? 0) > 0 && (
          <span style={{ background: 'var(--color-surface2)', border: '1px solid var(--color-border)', borderRadius: 12, padding: '2px 9px', fontSize: '0.72em', color: 'var(--color-text2)' }}>
            {m.task_count} tasks
          </span>
        )}
        {projectColor && (
          <>
            <span style={{ width: 8, height: 8, borderRadius: '50%', background: projectColor, display: 'inline-block' }} />
            {projectName && <span style={{ fontSize: '0.75em', color: 'var(--color-text2)' }}>{projectName}</span>}
          </>
        )}
      </div>
    </div>
  )
}
