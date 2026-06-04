import type { Task, Priority } from '../../types'
import { PriorityDot } from '../shared/PriorityDot'
import { Avatar } from '../shared/Avatar'

interface KanbanProps {
  tasks: Task[]
  onOpen: (id: number) => void
}

const COLUMNS = [
  { status: 'todo', label: 'To Do', color: 'var(--color-text2)' },
  { status: 'in_progress', label: 'In Progress', color: 'var(--color-blue)' },
  { status: 'blocked', label: 'Blocked', color: 'var(--color-red)' },
  { status: 'done', label: 'Done', color: 'var(--color-green)' },
] as const

export function Kanban({ tasks, onOpen }: KanbanProps) {
  return (
    <>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4,1fr)', gap: 14, alignItems: 'start' }} className="kanban-grid">
        {COLUMNS.map(col => {
          const colTasks = tasks.filter(t => t.status === col.status)
          return (
            <div key={col.status} style={{ background: 'var(--color-surface)', border: '1px solid var(--color-border)', borderRadius: 10, overflow: 'hidden' }}>
              <div style={{ padding: '10px 14px', fontSize: '0.78em', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.5px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--color-border)', color: col.color }}>
                {col.label}
                <span style={{ background: 'var(--color-surface3)', borderRadius: 12, padding: '1px 8px', color: 'var(--color-text3)', fontSize: '0.85em' }}>{colTasks.length}</span>
              </div>
              <div style={{ padding: 10, display: 'flex', flexDirection: 'column', gap: 8, minHeight: 60 }}>
                {colTasks.length === 0 && (
                  <div style={{ textAlign: 'center', padding: 20, color: 'var(--color-text3)', fontSize: '0.78em' }}>Empty</div>
                )}
                {colTasks.map(t => (
                  <KanbanCard key={t.id} task={t} onClick={() => onOpen(t.id)} />
                ))}
              </div>
            </div>
          )
        })}
      </div>
      <style>{`
        @media (max-width: 900px) {
          .kanban-grid { grid-template-columns: 1fr 1fr !important; }
        }
      `}</style>
    </>
  )
}

function KanbanCard({ task: t, onClick }: { task: Task; onClick: () => void }) {
  return (
    <div
      onClick={onClick}
      style={{ background: 'var(--color-surface2)', border: '1px solid var(--color-border)', borderRadius: 8, padding: 12, cursor: 'pointer', transition: 'all 0.15s' }}
      onMouseEnter={e => { e.currentTarget.style.borderColor = 'var(--color-border2)'; e.currentTarget.style.background = 'var(--color-surface3)' }}
      onMouseLeave={e => { e.currentTarget.style.borderColor = 'var(--color-border)'; e.currentTarget.style.background = 'var(--color-surface2)' }}
    >
      <div style={{ fontSize: '0.88em', fontWeight: 500, marginBottom: 6, lineHeight: 1.4 }}>{t.title}</div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
        <PriorityDot priority={t.priority as Priority} />
        {t.owner && <Avatar name={t.owner} />}
        {t.project_name && (
          <span style={{ background: 'var(--color-surface2)', border: '1px solid var(--color-border)', borderRadius: 12, padding: '2px 9px', fontSize: '0.72em', color: 'var(--color-text2)' }}>
            {t.project_name}
          </span>
        )}
      </div>
    </div>
  )
}
