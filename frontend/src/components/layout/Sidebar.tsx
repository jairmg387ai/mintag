import { LayoutDashboard, CheckSquare, Users, Network, Plus } from 'lucide-react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { Avatar } from '../shared/Avatar'
import type { ViewName } from '../../types'

interface NavEntry {
  id: ViewName
  label: string
  icon: React.ReactNode
}

const NAV_ITEMS: NavEntry[] = [
  { id: 'dashboard', label: 'Dashboard',       icon: <LayoutDashboard size={18} strokeWidth={1.75} /> },
  { id: 'tasks',     label: 'Tasks',           icon: <CheckSquare    size={18} strokeWidth={1.75} /> },
  { id: 'meetings',  label: 'Meetings',        icon: <Users          size={18} strokeWidth={1.75} /> },
  { id: 'graph',     label: 'Knowledge Graph', icon: <Network        size={18} strokeWidth={1.75} /> },
]

export function Sidebar() {
  const { currentView, projects, activeProject } = useAppState()
  const { setView, setActiveProject, openModal } = useAppActions()

  return (
    <nav className="sidebar">
      {/* Brand */}
      <div className="sb-brand">
        <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <rect x="3" y="3" width="18" height="18" rx="3" />
          <path d="M9 12l2 2 4-4" />
        </svg>
        <span className="name">mintag</span>
        <span className="pro">PRO</span>
      </div>

      {/* Main nav */}
      <div className="sb-section">
        <div className="head">Workspace</div>
        {NAV_ITEMS.map(n => (
          <button
            key={n.id}
            className={`sb-item${currentView === n.id ? ' active' : ''}`}
            onClick={() => setView(n.id)}
          >
            {n.icon}
            {n.label}
          </button>
        ))}
      </div>

      {/* Projects */}
      <div className="sb-section">
        <div className="head">Projects</div>
        {projects.map(p => (
          <button
            key={p.id}
            className={`sb-item${activeProject === p.id ? ' active' : ''}`}
            onClick={() => setActiveProject(p.id)}
          >
            <span style={{ width: 8, height: 8, borderRadius: '50%', background: p.color, flexShrink: 0, display: 'inline-block' }} />
            {p.name}
          </button>
        ))}
        <button className="sb-item" onClick={() => openModal('new-project')}>
          <Plus size={16} strokeWidth={1.75} />
          New project
        </button>
      </div>

      {/* Footer */}
      <div className="sb-foot">
        <Avatar name="Local workspace" size={32} />
        <div>
          <div className="who">Local workspace</div>
          <div className="role">Personal</div>
        </div>
      </div>
    </nav>
  )
}
