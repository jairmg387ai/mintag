import type { Task, Status, Priority } from '../../types'
import { StatusBadge } from '../shared/StatusBadge'
import { PriorityDot } from '../shared/PriorityDot'
import { Avatar } from '../shared/Avatar'
import { priorityLabel } from '../shared/PriorityDot'

interface TaskListProps {
  tasks: Task[]
  onOpen: (id: number) => void
}

export function TaskList({ tasks, onOpen }: TaskListProps) {
  return (
    <div style={{ background: 'var(--color-surface)', border: '1px solid var(--color-border)', borderRadius: 10, overflow: 'hidden' }}>
      {/* Header */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: '28px 1fr 110px 120px 90px 90px 32px',
        gap: 12,
        padding: '8px 16px',
        background: 'var(--color-surface2)',
        fontSize: '0.72em',
        fontWeight: 600,
        textTransform: 'uppercase',
        letterSpacing: '0.5px',
        color: 'var(--color-text3)',
      }} className="task-list-header">
        <span></span>
        <span>Task</span>
        <span>Status</span>
        <span className="col-meeting">Meeting</span>
        <span>Owner</span>
        <span>Priority</span>
        <span></span>
      </div>

      {/* Rows */}
      {tasks.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '48px 24px', color: 'var(--color-text3)' }}>No tasks</div>
      ) : (
        tasks.map(t => (
          <TaskRow key={t.id} task={t} onClick={() => onOpen(t.id)} />
        ))
      )}

      <style>{`
        @media (max-width: 900px) {
          .task-list-header, .task-row-grid { grid-template-columns: 28px 1fr 90px 90px 32px !important; }
          .col-meeting { display: none !important; }
        }
      `}</style>
    </div>
  )
}

function TaskRow({ task: t, onClick }: { task: Task; onClick: () => void }) {
  return (
    <div
      className="task-row-grid"
      onClick={onClick}
      style={{
        display: 'grid',
        gridTemplateColumns: '28px 1fr 110px 120px 90px 90px 32px',
        alignItems: 'center',
        gap: 12,
        padding: '11px 16px',
        borderBottom: '1px solid var(--color-border)',
        transition: 'background 0.1s',
        cursor: 'pointer',
      }}
      onMouseEnter={e => (e.currentTarget.style.background = 'var(--color-surface2)')}
      onMouseLeave={e => (e.currentTarget.style.background = '')}
    >
      <PriorityDot priority={t.priority as Priority} />
      <div>
        <div style={{ fontSize: '0.9em', fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{t.title}</div>
        <div style={{ fontSize: '0.78em', color: 'var(--color-text3)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {t.project_name ?? ''}{t.meeting_title ? ` · ${t.meeting_title}` : ''}
        </div>
      </div>
      <div><StatusBadge status={t.status as Status} /></div>
      <div className="col-meeting" style={{ fontSize: '0.78em', color: 'var(--color-text3)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {t.meeting_title ?? '—'}
      </div>
      <div>{t.owner ? <Avatar name={t.owner} /> : <span style={{ color: 'var(--color-text3)', fontSize: '0.78em' }}>—</span>}</div>
      <div style={{ fontSize: '0.78em', color: 'var(--color-text3)' }}>{priorityLabel(t.priority as Priority)}</div>
      <div>
        <button
          onClick={e => { e.stopPropagation(); onClick() }}
          style={{ background: 'var(--color-surface2)', border: '1px solid var(--color-border)', borderRadius: 7, color: 'var(--color-text2)', cursor: 'pointer', padding: '3px 6px', display: 'flex' }}
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
            <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
          </svg>
        </button>
      </div>
    </div>
  )
}
