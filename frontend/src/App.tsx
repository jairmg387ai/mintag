import { useEffect } from 'react'
import { useAppState, useAppActions } from './store/AppContext'
import { Sidebar } from './components/layout/Sidebar'
import { Dashboard } from './components/dashboard/Dashboard'
import { TasksView } from './components/tasks/TasksView'
import { MeetingsView } from './components/meetings/MeetingsView'
import { TaskDetailModal } from './components/modals/TaskDetailModal'
import { NewTaskModal } from './components/modals/NewTaskModal'
import { ImportMeetingModal } from './components/modals/ImportMeetingModal'
import { NewProjectModal } from './components/modals/NewProjectModal'
import { MeetingDetailModal } from './components/modals/MeetingDetailModal'
import { ToastContainer } from './components/shared/Toast'

function AppInner() {
  const { currentView, activeModal } = useAppState()
  const { loadAll } = useAppActions()

  useEffect(() => {
    loadAll()
  }, [loadAll])

  return (
    <>
      <Sidebar />
      <div style={{ flex: 1, overflowY: 'auto', minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>
        {currentView === 'dashboard' && <Dashboard />}
        {currentView === 'tasks' && <TasksView />}
        {currentView === 'meetings' && <MeetingsView />}
      </div>

      {activeModal === 'task' && <TaskDetailModal />}
      {activeModal === 'new-task' && <NewTaskModal />}
      {activeModal === 'import' && <ImportMeetingModal />}
      {activeModal === 'new-project' && <NewProjectModal />}
      {activeModal === 'meeting' && <MeetingDetailModal />}

      <ToastContainer />
    </>
  )
}

export default AppInner
