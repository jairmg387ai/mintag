import type { Task, Status, Priority } from '../../types'
import { StatusBadge } from '../shared/StatusBadge'
import { PriorityTag } from '../shared/PriorityTag'
import { Avatar } from '../shared/Avatar'

interface TaskListProps {
  tasks: Task[]
  onOpen: (id: number) => void
}

function fmt(dateStr: string | undefined): string {
  if (!dateStr) return '—'
  try {
    return new Date(dateStr).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
  } catch {
    return dateStr
  }
}

export function TaskList({ tasks, onOpen }: TaskListProps) {
  if (tasks.length === 0) {
    return (
      <div className="card" style={{ textAlign: 'center', padding: '48px 0', color: 'var(--fg3)' }}>
        No tasks
      </div>
    )
  }

  return (
    <div className="card" style={{ overflow: 'hidden' }}>
      <table className="mt-table">
        <thead>
          <tr>
            <th>Title</th>
            <th>Status</th>
            <th>Priority</th>
            <th>Owner</th>
            <th>Project</th>
            <th>Due Date</th>
          </tr>
        </thead>
        <tbody>
          {tasks.map(t => (
            <tr key={t.id} onClick={() => onOpen(t.id)}>
              <td>
                <div style={{ font: 'var(--text-h4)', color: 'var(--fg1)' }}>{t.title}</div>
                {t.meeting_title && (
                  <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', marginTop: 2 }}>
                    {t.meeting_title}
                  </div>
                )}
              </td>
              <td><StatusBadge status={t.status as Status} /></td>
              <td><PriorityTag priority={t.priority as Priority} /></td>
              <td>{t.owner ? <Avatar name={t.owner} size={26} /> : <span style={{ color: 'var(--fg3)' }}>—</span>}</td>
              <td><span className="mt-mono">{t.project_name ?? '—'}</span></td>
              <td>
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--fg2)' }}>
                  {fmt(t.due_date)}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
