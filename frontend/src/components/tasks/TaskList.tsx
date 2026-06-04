import type { Task, Status, Priority } from '../../types'
import { StatusBadge } from '../shared/StatusBadge'
import { PriorityDot } from '../shared/PriorityDot'
import { Avatar } from '../shared/Avatar'
import { priorityLabel } from '../shared/PriorityDot'
import { Card, Button } from '../ui'

interface TaskListProps {
  tasks: Task[]
  onOpen: (id: number) => void
}

export function TaskList({ tasks, onOpen }: TaskListProps) {
  return (
    <Card>
      <div
        className="task-list-header grid gap-3 px-4 py-2 bg-surface2 text-[0.72em] font-semibold uppercase tracking-[0.5px] text-text3"
        style={{ gridTemplateColumns: '28px 1fr 110px 120px 90px 90px 32px' }}
      >
        {['', 'Task', 'Status', 'Meeting', 'Owner', 'Priority', ''].map((h, i) => (
          <span key={i} className={h === 'Meeting' ? 'col-meeting' : undefined}>{h}</span>
        ))}
      </div>

      {tasks.length === 0 ? (
        <div className="text-center py-12 text-text3">No tasks</div>
      ) : (
        tasks.map(t => <TaskRow key={t.id} task={t} onClick={() => onOpen(t.id)} />)
      )}

      <style>{`
        @media (max-width: 900px) {
          .task-list-header, .task-row-grid { grid-template-columns: 28px 1fr 90px 90px 32px !important; }
          .col-meeting { display: none !important; }
        }
      `}</style>
    </Card>
  )
}

function TaskRow({ task: t, onClick }: { task: Task; onClick: () => void }) {
  return (
    <div
      className="row-clickable task-row-grid grid items-center gap-3 px-4 py-3"
      onClick={onClick}
      style={{ gridTemplateColumns: '28px 1fr 110px 120px 90px 90px 32px' }}
    >
      <PriorityDot priority={t.priority as Priority} />
      <div>
        <div className="text-[0.9em] font-medium truncate">{t.title}</div>
        <div className="text-[0.78em] text-text3 truncate">
          {t.project_name ?? ''}{t.meeting_title ? ` · ${t.meeting_title}` : ''}
        </div>
      </div>
      <div><StatusBadge status={t.status as Status} /></div>
      <div className="col-meeting text-[0.78em] text-text3 truncate">{t.meeting_title ?? '—'}</div>
      <div>{t.owner ? <Avatar name={t.owner} /> : <span className="text-text3 text-[0.78em]">—</span>}</div>
      <div className="text-[0.78em] text-text3">{priorityLabel(t.priority as Priority)}</div>
      <div>
        <Button variant="ghost" size="sm" onClick={e => { e.stopPropagation(); onClick() }} style={{ padding: '3px 6px' }}>
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
            <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
          </svg>
        </Button>
      </div>
    </div>
  )
}
