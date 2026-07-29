import { useEffect } from 'react'
import { useAppState, useAppActions } from './store/AppContext'
import { Sidebar } from './components/layout/Sidebar'
import { TopBar } from './components/layout/TopBar'
import { Dashboard } from './components/dashboard/Dashboard'
import { TasksView } from './components/tasks/TasksView'
import { MeetingsView } from './components/meetings/MeetingsView'
import { GraphView } from './components/graph/GraphView'
import { ActivitiesView } from './components/activities/ActivitiesView'
import { DeploymentWindowsView } from './components/deployment-windows/DeploymentWindowsView'
import { SettingsView } from './components/settings/SettingsView'
import { TaskDetailModal } from './components/modals/TaskDetailModal'
import { NewTaskModal } from './components/modals/NewTaskModal'
import { ImportMeetingModal } from './components/modals/ImportMeetingModal'
import { NewProjectModal } from './components/modals/NewProjectModal'
import { MeetingDetailModal } from './components/modals/MeetingDetailModal'
import { ToastContainer } from './components/shared/Toast'

const VIEW_TITLES: Record<string, string> = {
  dashboard: 'Panel',
  tasks: 'Tareas',
  meetings: 'Reuniones',
  graph: 'Grafo de Conocimiento',
  activities: 'Actividades',
  'deployment-windows': 'Ventanas de Mantenimiento',
  settings: 'Configuración',
}

function AppInner() {
  const { currentView, activeModal } = useAppState()
  const { loadAll } = useAppActions()

  useEffect(() => {
    loadAll()
  }, [loadAll])

  const title = VIEW_TITLES[currentView] ?? 'Mintag'

  return (
    <>
      <div className="app">
        <Sidebar />
        <div className="main">
          <TopBar title={title} />
          <div className="content">
            {currentView === 'dashboard' && <Dashboard />}
            {currentView === 'tasks' && <TasksView />}
            {currentView === 'meetings' && <MeetingsView />}
            {currentView === 'graph' && <GraphView />}
            {currentView === 'activities' && <ActivitiesView />}
            {currentView === 'deployment-windows' && <DeploymentWindowsView />}
            {currentView === 'settings' && <SettingsView />}
          </div>
        </div>
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
