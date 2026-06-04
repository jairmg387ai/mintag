import { useAppState, useAppActions } from '../../store/AppContext'
import type { ViewName } from '../../types'

export function Sidebar() {
  const { currentView, projects, activeProject } = useAppState()
  const { setView, setActiveProject, openModal } = useAppActions()

  return (
    <nav className="sidebar" style={{
      width: 220,
      background: 'var(--color-surface)',
      borderRight: '1px solid var(--color-border)',
      display: 'flex',
      flexDirection: 'column',
      flexShrink: 0,
      height: '100vh',
      position: 'sticky',
      top: 0,
    }}>
      {/* Logo */}
      <div style={{ padding: '20px 16px 12px', fontSize: '1.2em', fontWeight: 700, color: 'var(--color-blue)', letterSpacing: '-0.5px', display: 'flex', alignItems: 'center', gap: 8 }}>
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0 }}>
          <rect x="3" y="3" width="18" height="18" rx="3"/>
          <path d="M9 12l2 2 4-4"/>
        </svg>
        <span className="sidebar-text">mintag</span>
      </div>

      {/* Nav */}
      <div style={{ borderTop: '1px solid var(--color-border)', paddingTop: 4, paddingBottom: 4 }}>
        <NavItem active={currentView === 'dashboard'} onClick={() => setView('dashboard')} label="Dashboard" icon={
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <rect x="3" y="3" width="7" height="7" rx="1"/>
            <rect x="14" y="3" width="7" height="7" rx="1"/>
            <rect x="14" y="14" width="7" height="7" rx="1"/>
            <rect x="3" y="14" width="7" height="7" rx="1"/>
          </svg>
        } />
        <NavItem active={currentView === 'tasks'} onClick={() => setView('tasks')} label="Tasks" icon={
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M9 12l2 2 4-4"/>
            <rect x="3" y="3" width="18" height="18" rx="3"/>
          </svg>
        } />
        <NavItem active={currentView === 'meetings'} onClick={() => setView('meetings')} label="Meetings" icon={
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
            <circle cx="9" cy="7" r="4"/>
            <path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
            <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
          </svg>
        } />
      </div>

      {/* Projects */}
      <div style={{ borderTop: '1px solid var(--color-border)', paddingTop: 4 }}>
        <div className="sidebar-text" style={{ fontSize: '0.68em', fontWeight: 600, letterSpacing: '0.8px', color: 'var(--color-text3)', padding: '6px 18px 2px', textTransform: 'uppercase' }}>
          Projects
        </div>
        {projects.map(p => (
          <button
            key={p.id}
            onClick={() => setActiveProject(p.id)}
            className={`nav-btn${activeProject === p.id ? ' active' : ''}`}
            style={{ paddingLeft: 14 }}
          >
            <span style={{ width: 8, height: 8, borderRadius: '50%', background: p.color, flexShrink: 0, display: 'inline-block' }} />
            <span className="sidebar-text">{p.name}</span>
          </button>
        ))}
        <button onClick={() => openModal('new-project')} className="nav-btn" style={{ paddingLeft: 14 }}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" style={{ flexShrink: 0 }}>
            <line x1="12" y1="5" x2="12" y2="19"/>
            <line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          <span className="sidebar-text">New project</span>
        </button>
      </div>

      <style>{`
        @media (max-width: 900px) {
          .sidebar { width: 56px !important; }
          .sidebar-text { display: none !important; }
        }
      `}</style>
    </nav>
  )
}

function NavItem({ active, onClick, icon, label }: {
  active: boolean
  onClick: () => void
  icon: React.ReactNode
  label: string
}) {
  return (
    <button onClick={onClick} className={`nav-btn${active ? ' active' : ''}`}>
      <span style={{ flexShrink: 0 }}>{icon}</span>
      <span className="sidebar-text">{label}</span>
    </button>
  )
}
