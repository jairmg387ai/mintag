import React, { useMemo } from 'react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { FilterBar } from './FilterBar'
import { TaskList } from './TaskList'
import { Kanban } from './Kanban'
import { Button } from '../ui'
import { List, Columns, Plus } from 'lucide-react'
import type { Status } from '../../types'

export function TasksView() {
  const { tasks, taskView, filterStatus, activeProject } = useAppState()
  const { setTaskView, setEditingTaskId, openModal } = useAppActions()

  const filteredTasks = useMemo(() => {
    let t = tasks
    if (activeProject != null) t = t.filter(task => task.project_id === activeProject)
    if (filterStatus !== 'all') t = t.filter(task => task.status === filterStatus as Status)
    return t
  }, [tasks, activeProject, filterStatus])

  function openTask(id: number) {
    setEditingTaskId(id)
    openModal('task')
  }

  return (
    <div className="content-pad">
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 16 }}>
        {/* View toggle */}
        <div style={{
          display: 'flex',
          background: 'var(--bg-sunken)',
          borderRadius: 'var(--radius-md)',
          padding: 2,
          gap: 2,
        }}>
          <ViewBtn active={taskView === 'list'} onClick={() => setTaskView('list')} title="List">
            <List size={14} strokeWidth={2} />
          </ViewBtn>
          <ViewBtn active={taskView === 'kanban'} onClick={() => setTaskView('kanban')} title="Kanban">
            <Columns size={14} strokeWidth={2} />
          </ViewBtn>
        </div>

        <div style={{ marginLeft: 'auto' }}>
          <Button variant="primary" size="sm" onClick={() => openModal('new-task')}>
            <Plus size={14} strokeWidth={2} />
            New Task
          </Button>
        </div>
      </div>

      <FilterBar />

      {taskView === 'list' ? (
        <TaskList tasks={filteredTasks} onOpen={openTask} />
      ) : (
        <Kanban tasks={filteredTasks} onOpen={openTask} />
      )}
    </div>
  )
}

function ViewBtn({
  active,
  onClick,
  title,
  children,
}: {
  active: boolean
  onClick: () => void
  title: string
  children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      title={title}
      style={{
        background: active ? 'var(--brand-subtle)' : 'none',
        border: active ? '1px solid var(--border)' : '1px solid transparent',
        color: active ? 'var(--brand)' : 'var(--fg3)',
        cursor: 'pointer',
        padding: '5px 8px',
        borderRadius: 'var(--radius-sm)',
        display: 'flex',
        alignItems: 'center',
        transition: 'all 0.15s',
      }}
    >
      {children}
    </button>
  )
}
