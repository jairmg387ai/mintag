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
import { Badge, Card, CardHeader } from '../ui'
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
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4,1fr)', gap: 16, alignItems: 'start' }} className="kanban-grid">
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
    <Card
      ref={setNodeRef}
      style={{
        background: highlight ? col.bg : undefined,
        borderColor: highlight ? col.color : undefined,
        transition: 'background 0.15s, border-color 0.15s',
      }}
    >
      <CardHeader
        size="sm"
        style={{ color: col.color }}
        right={<Badge className="text-[0.85em]">{tasks.length}</Badge>}
      >
        {col.label}
      </CardHeader>

      <div className="p-3 flex flex-col gap-2.5" style={{ minHeight: 80 }}>
        {tasks.length === 0 && !highlight && (
          <div className="text-center py-5 text-text3 text-[0.78em]">Empty</div>
        )}
        {tasks.map(t => <KanbanCard key={t.id} task={t} onClick={() => onOpen(t.id)} />)}
      </div>
    </Card>
  )
}

function KanbanCard({ task: t, onClick, overlay }: { task: Task; onClick: () => void; overlay?: boolean }) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({ id: t.id })

  const cls = [
    'kanban-card',
    isDragging ? 'kanban-card--dragging' : '',
    overlay    ? 'kanban-card--overlay'  : '',
  ].filter(Boolean).join(' ')

  return (
    <div
      ref={overlay ? undefined : setNodeRef}
      className={cls}
      {...(overlay ? {} : { ...listeners, ...attributes })}
      onClick={overlay ? undefined : onClick}
    >
      <div className="text-[0.88em] font-medium mb-2.5 leading-snug">{t.title}</div>
      <div className="flex items-center gap-1.5 flex-wrap">
        <PriorityDot priority={t.priority as Priority} />
        {t.owner && <Avatar name={t.owner} />}
        {t.project_name && <Badge>{t.project_name}</Badge>}
      </div>
    </div>
  )
}
