import { useState } from 'react'
import {
  DndContext,
  DragOverlay,
  closestCenter,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
  MouseSensor,
  TouchSensor,
  type DragStartEvent,
  type DragEndEvent,
} from '@dnd-kit/core'
import type { Task, Priority, Status } from '../../types'
import { StatusBadge } from '../shared/StatusBadge'
import { PriorityTag } from '../shared/PriorityTag'
import { Avatar } from '../shared/Avatar'
import { Badge } from '../ui'
import { useAppActions } from '../../store/AppContext'

interface KanbanProps {
  tasks: Task[]
  onOpen: (id: number) => void
}

const COLUMNS: { status: Status; label: string; highlightBg: string; highlightBorder: string }[] = [
  { status: 'todo',        label: 'To Do',       highlightBg: 'rgba(100,116,139,0.08)', highlightBorder: 'var(--slate-400)' },
  { status: 'in_progress', label: 'In Progress',  highlightBg: 'rgba(59,130,246,0.08)',  highlightBorder: 'var(--prog-solid)' },
  { status: 'blocked',     label: 'Blocked',      highlightBg: 'rgba(239,68,68,0.08)',   highlightBorder: 'var(--block-solid)' },
  { status: 'in_testing',  label: 'In Testing',   highlightBg: 'rgba(14,165,233,0.08)',  highlightBorder: 'var(--info-solid)' },
  { status: 'done',        label: 'Done',         highlightBg: 'rgba(34,197,94,0.08)',   highlightBorder: 'var(--done-solid)' },
]

export function Kanban({ tasks, onOpen }: KanbanProps) {
  const { updateTaskStatus } = useAppActions()
  const [activeTask, setActiveTask] = useState<Task | null>(null)
  const [overId, setOverId] = useState<string | null>(null)
  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 5 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 250, tolerance: 5 } }),
  )

  function handleDragStart(e: DragStartEvent) {
    setActiveTask(tasks.find(t => t.id === Number(e.active.id)) ?? null)
  }

  function handleDragOver(e: { over: { id: string | number } | null }) {
    setOverId(e.over ? String(e.over.id) : null)
  }

  function handleDragEnd(e: DragEndEvent) {
    setActiveTask(null)
    setOverId(null)
    if (!e.over || !activeTask) return
    const newStatus = String(e.over.id).replace('col-', '') as Status
    if (newStatus !== activeTask.status) updateTaskStatus(activeTask.id, newStatus)
  }

  return (
    <>
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragStart={handleDragStart} onDragOver={handleDragOver} onDragEnd={handleDragEnd}>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5,1fr)', gap: 16, alignItems: 'start' }} className="kanban-grid">
          {COLUMNS.map(col => (
            <KanbanColumn
              key={col.status}
              col={col}
              tasks={tasks.filter(t => t.status === col.status)}
              onOpen={onOpen}
              isDragTarget={overId === `col-${col.status}` && activeTask?.status !== col.status}
            />
          ))}
        </div>

        <DragOverlay dropAnimation={{ duration: 180, easing: 'ease' }}>
          {activeTask && <KanbanCard task={activeTask} onClick={() => {}} overlay />}
        </DragOverlay>
      </DndContext>

      <style>{`
        @media (max-width: 1100px) {
          .kanban-grid { grid-template-columns: repeat(3,1fr) !important; }
        }
        @media (max-width: 750px) {
          .kanban-grid { grid-template-columns: 1fr 1fr !important; }
        }
      `}</style>
    </>
  )
}

function KanbanColumn({
  col, tasks, onOpen, isDragTarget,
}: {
  col: typeof COLUMNS[number]
  tasks: Task[]
  onOpen: (id: number) => void
  isDragTarget: boolean
}) {
  const { setNodeRef, isOver } = useDroppable({ id: `col-${col.status}` })
  const highlight = isOver && isDragTarget

  return (
    <div
      ref={setNodeRef}
      style={{
        background: highlight ? col.highlightBg : 'var(--bg-sunken)',
        border: `1px solid ${highlight ? col.highlightBorder : 'var(--border)'}`,
        borderRadius: 'var(--radius-lg)',
        padding: 12,
        display: 'flex',
        flexDirection: 'column',
        transition: 'background 0.15s, border-color 0.15s',
      }}
    >
      {/* Column header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '4px 6px 12px' }}>
        <span className="mt-label" style={{ flex: 1 }}>{col.label}</span>
        <Badge className="text-[0.85em]">{tasks.length}</Badge>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 10, minHeight: 80 }}>
        {tasks.length === 0 && !highlight && (
          <div style={{ textAlign: 'center', padding: '20px 0', color: 'var(--fg3)', font: 'var(--text-caption)' }}>
            Empty
          </div>
        )}
        {tasks.map(t => <KanbanCard key={t.id} task={t} onClick={() => onOpen(t.id)} />)}
      </div>
    </div>
  )
}

function KanbanCard({ task: t, onClick, overlay }: { task: Task; onClick: () => void; overlay?: boolean }) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({ id: t.id })

  const cls = [
    'kb-card',
    isDragging ? 'kanban-card--dragging' : '',
    overlay    ? 'kanban-card--overlay'  : '',
  ].filter(Boolean).join(' ')

  return (
    <button
      ref={overlay ? undefined : setNodeRef}
      className={cls}
      {...(overlay ? {} : { ...listeners, ...attributes })}
      onClick={overlay ? undefined : onClick}
    >
      {/* top: status chip + priority tag */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
        <StatusBadge status={t.status} />
        <PriorityTag priority={t.priority as Priority} />
      </div>

      {/* title */}
      <div style={{ font: 'var(--text-h4)', color: 'var(--fg1)', lineHeight: 1.35, marginBottom: 10 }}>
        {t.title}
      </div>

      {/* footer: project name + avatar */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
          {t.project_name ?? ''}
        </span>
        {t.owner && <Avatar name={t.owner} size={24} />}
      </div>
    </button>
  )
}
