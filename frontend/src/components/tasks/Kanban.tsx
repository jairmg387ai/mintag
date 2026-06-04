import { useState } from 'react'
import {
  DndContext,
  DragOverlay,
  closestCenter,
  useDraggable,
  useDroppable,
  type DragStartEvent,
  type DragEndEvent,
} from '@dnd-kit/core'
import type { Task, Priority, Status } from '../../types'
import { PriorityDot } from '../shared/PriorityDot'
import { Avatar } from '../shared/Avatar'
import { useAppActions } from '../../store/AppContext'

interface KanbanProps {
  tasks: Task[]
  onOpen: (id: number) => void
}

const COLUMNS: { status: Status; label: string; color: string; bg: string }[] = [
  { status: 'todo',        label: 'To Do',      color: 'var(--color-text2)', bg: 'transparent' },
  { status: 'in_progress', label: 'In Progress', color: 'var(--color-blue)',  bg: 'rgba(59,130,246,0.06)' },
  { status: 'blocked',     label: 'Blocked',     color: 'var(--color-red)',   bg: 'rgba(239,68,68,0.06)' },
  { status: 'done',        label: 'Done',        color: 'var(--color-green)', bg: 'rgba(34,197,94,0.06)' },
]

export function Kanban({ tasks, onOpen }: KanbanProps) {
  const { updateTaskStatus } = useAppActions()
  const [activeTask, setActiveTask] = useState<Task | null>(null)
  const [overId, setOverId] = useState<string | null>(null)

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
      <DndContext collisionDetection={closestCenter} onDragStart={handleDragStart} onDragOver={handleDragOver} onDragEnd={handleDragEnd}>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4,1fr)', gap: 14, alignItems: 'start' }} className="kanban-grid">
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
        @media (max-width: 900px) {
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
      className="card"
      style={{
        background: highlight ? col.bg : undefined,
        borderColor: highlight ? col.color : undefined,
        transition: 'background 0.15s, border-color 0.15s',
      }}
    >
      <div className="card-header-sm" style={{ color: col.color }}>
        {col.label}
        <span className="badge" style={{ fontSize: '0.85em' }}>{tasks.length}</span>
      </div>

      <div style={{ padding: 10, display: 'flex', flexDirection: 'column', gap: 8, minHeight: 80 }}>
        {tasks.length === 0 && !highlight && (
          <div style={{ textAlign: 'center', padding: 20, color: 'var(--color-text3)', fontSize: '0.78em' }}>Empty</div>
        )}
        {tasks.map(t => <KanbanCard key={t.id} task={t} onClick={() => onOpen(t.id)} />)}
      </div>
    </div>
  )
}

function KanbanCard({ task: t, onClick, overlay }: { task: Task; onClick: () => void; overlay?: boolean }) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({ id: t.id })

  const cls = [
    'kanban-card',
    isDragging ? 'kanban-card--dragging' : '',
    overlay ? 'kanban-card--overlay' : '',
  ].filter(Boolean).join(' ')

  return (
    <div
      ref={overlay ? undefined : setNodeRef}
      className={cls}
      {...(overlay ? {} : { ...listeners, ...attributes })}
      onClick={overlay ? undefined : onClick}
    >
      <div style={{ fontSize: '0.88em', fontWeight: 500, marginBottom: 6, lineHeight: 1.4 }}>{t.title}</div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
        <PriorityDot priority={t.priority as Priority} />
        {t.owner && <Avatar name={t.owner} />}
        {t.project_name && <span className="badge">{t.project_name}</span>}
      </div>
    </div>
  )
}
