import React, { useMemo } from 'react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { TopBar } from '../layout/TopBar'
import { FilterBar } from './FilterBar'
import { TaskList } from './TaskList'
import { Kanban } from './Kanban'
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
    <>
      <TopBar title="Tasks">
        {/* View toggle */}
        <div style={{ display: 'flex', background: 'var(--color-surface2)', borderRadius: 7, padding: 2, gap: 2 }}>
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
        <button
          onClick={() => openModal('new-task')}
          style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '4px 10px', borderRadius: 7, border: 'none', cursor: 'pointer', fontSize: '0.78em', fontWeight: 500, background: 'var(--color-blue)', color: '#fff' }}
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
            <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          New Task
        </button>
      </TopBar>

      <div style={{ padding: '24px 28px' }}>
        <FilterBar />
        {taskView === 'list' ? (
          <TaskList tasks={filteredTasks} onOpen={openTask} />
        ) : (
          <Kanban tasks={filteredTasks} onOpen={openTask} />
        )}
      </div>
    </>
  )
}

function ViewBtn({ active, onClick, title, children }: { active: boolean; onClick: () => void; title: string; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      title={title}
      style={{ background: active ? 'var(--color-surface3)' : 'none', border: 'none', color: active ? 'var(--color-text)' : 'var(--color-text3)', cursor: 'pointer', padding: '5px 8px', borderRadius: 5, display: 'flex', transition: 'all 0.15s' }}
    >
      {children}
    </button>
  )
}
