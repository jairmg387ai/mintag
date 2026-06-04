import React, { useMemo } from 'react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { FilterBar } from './FilterBar'
import { TaskList } from './TaskList'
import { Kanban } from './Kanban'
import { Button } from '../ui'
import { Plus } from 'lucide-react'
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
    <div style={{ padding: '24px 28px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 16 }}>
        {/* View toggle */}
        <div style={{ display: 'flex', background: 'var(--bg-sunken)', borderRadius: 'var(--radius-md)', padding: 2, gap: 2 }}>
          <ViewBtn active={taskView === 'list'} onClick={() => setTaskView('list')} title="List">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/>
              <line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/>
            </svg>
          </ViewBtn>
          <ViewBtn active={taskView === 'kanban'} onClick={() => setTaskView('kanban')} title="Kanban">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <rect x="3" y="3" width="5" height="18" rx="1"/>
              <rect x="10" y="3" width="5" height="12" rx="1"/>
              <rect x="17" y="3" width="5" height="15" rx="1"/>
            </svg>
          </ViewBtn>
        </div>

        <div style={{ marginLeft: 'auto' }}>
          <Button variant="primary" onClick={() => openModal('new-task')}>
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

function ViewBtn({ active, onClick, title, children }: { active: boolean; onClick: () => void; title: string; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      title={title}
      style={{ background: active ? 'var(--bg-surface)' : 'none', border: active ? '1px solid var(--border)' : '1px solid transparent', color: active ? 'var(--fg1)' : 'var(--fg3)', cursor: 'pointer', padding: '5px 8px', borderRadius: 'var(--radius-sm)', display: 'flex', transition: 'all 0.15s' }}
    >
      {children}
    </button>
  )
}
