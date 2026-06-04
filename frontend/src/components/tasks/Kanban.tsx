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
import { CSS } from '@dnd-kit/utilities'
import type { Task, Priority, Status } from '../../types'
import { PriorityDot } from '../shared/PriorityDot'
import { Avatar } from '../shared/Avatar'
import { useAppActions } from '../../store/AppContext'

interface KanbanProps {
  tasks: Task[]
  onOpen: (id: number) => void
}

const COLUMNS: { status: Status; label: string; color: string; bg: string }[] = [
  { status: 'todo',        label: 'To Do',       color: 'var(--color-text2)',  bg: 'transparent' },
  { status: 'in_progress', label: 'In Progress',  color: 'var(--color-blue)',   bg: 'rgba(59,130,246,0.06)' },
  { status: 'blocked',     label: 'Blocked',      color: 'var(--color-red)',    bg: 'rgba(239,68,68,0.06)' },
  { status: 'done',        label: 'Done',         color: 'var(--color-green)',  bg: 'rgba(34,197,94,0.06)' },
]

export function Kanban({ tasks, onOpen }: KanbanProps) {
  const { updateTaskStatus } = useAppActions()
  const [activeTask, setActiveTask] = useState<Task | null>(null)
  const [overId, setOverId] = useState<string | null>(null)

  function handleDragStart(e: DragStartEvent) {
    const task = tasks.find(t => t.id === Number(e.active.id))
    setActiveTask(task ?? null)
  }

  function handleDragOver(e: { over: { id: string | number } | null }) {
    setOverId(e.over ? String(e.over.id) : null)
  }

  function handleDragEnd(e: DragEndEvent) {
    setActiveTask(null)
    setOverId(null)
    if (!e.over || !activeTask) return
    const newStatus = String(e.over.id).replace('col-', '') as Status
    if (newStatus !== activeTask.status) {
      updateTaskStatus(activeTask.id, newStatus)
    }
  }

  return (
    <>
      <DndContext
        collisionDetection={closestCenter}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
        onDragEnd={handleDragEnd}
      >
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
          {activeTask && (
            <KanbanCard task={activeTask} onClick={() => {}} overlay />
          )}
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
  col,
  tasks,
  onOpen,
  isDragTarget,
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
        background: highlight ? col.bg : 'var(--color-surface)',
        border: `1px solid ${highlight ? col.color : 'var(--color-border)'}`,
        borderRadius: 10,
        overflow: 'hidden',
        transition: 'background 0.15s, border-color 0.15s',
      }}
    >
      <div style={{
        padding: '10px 14px',
        fontSize: '0.78em',
        fontWeight: 600,
        textTransform: 'uppercase',
        letterSpacing: '0.5px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        borderBottom: '1px solid var(--color-border)',
        color: col.color,
      }}>
        {col.label}
        <span style={{ background: 'var(--color-surface3)', borderRadius: 12, padding: '1px 8px', color: 'var(--color-text3)', fontSize: '0.85em' }}>
          {tasks.length}
        </span>
      </div>

      <div style={{ padding: 10, display: 'flex', flexDirection: 'column', gap: 8, minHeight: 80 }}>
        {tasks.length === 0 && !highlight && (
          <div style={{ textAlign: 'center', padding: 20, color: 'var(--color-text3)', fontSize: '0.78em' }}>Empty</div>
        )}
        {tasks.map(t => (
          <KanbanCard key={t.id} task={t} onClick={() => onOpen(t.id)} />
        ))}
      </div>
    </div>
  )
}

function KanbanCard({ task: t, onClick, overlay }: { task: Task; onClick: () => void; overlay?: boolean }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({ id: t.id })

  const style: React.CSSProperties = {
    background: 'var(--color-surface2)',
    border: '1px solid var(--color-border)',
    borderRadius: 8,
    padding: 12,
    cursor: overlay ? 'grabbing' : 'grab',
    transition: overlay ? 'none' : 'opacity 0.15s, border-color 0.15s, background 0.15s',
    opacity: isDragging ? 0.35 : 1,
    transform: overlay ? CSS.Translate.toString(transform) : undefined,
    boxShadow: overlay ? '0 8px 24px rgba(0,0,0,0.35)' : undefined,
    userSelect: 'none',
  }

  return (
    <div
      ref={overlay ? undefined : setNodeRef}
      style={style}
      {...(overlay ? {} : { ...listeners, ...attributes })}
      onClick={overlay ? undefined : onClick}
      onMouseEnter={overlay ? undefined : e => {
        e.currentTarget.style.borderColor = 'var(--color-border2)'
        e.currentTarget.style.background = 'var(--color-surface3)'
      }}
      onMouseLeave={overlay ? undefined : e => {
        e.currentTarget.style.borderColor = 'var(--color-border)'
        e.currentTarget.style.background = 'var(--color-surface2)'
      }}
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
