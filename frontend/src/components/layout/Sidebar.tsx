import { useAppState, useAppActions } from '../../store/AppContext'
import type { ViewName } from '../../types'

export function Sidebar() {
  const { currentView, projects, activeProject } = useAppState()
  const { setView, setActiveProject, openModal } = useAppActions()

  function nav(view: ViewName) {
    setView(view)
  }

  return (
    <nav
      style={{
        width: 220,
        background: 'var(--color-surface)',
        borderRight: '1px solid var(--color-border)',
        display: 'flex',
        flexDirection: 'column',
        flexShrink: 0,
        height: '100vh',
        position: 'sticky',
        top: 0,
      }}
      className="sidebar"
    >
      {/* Logo */}
      <div style={{ padding: '20px 16px 12px', fontSize: '1.2em', fontWeight: 700, color: 'var(--color-blue)', letterSpacing: '-0.5px', display: 'flex', alignItems: 'center', gap: 8 }}>
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0 }}>
          <rect x="3" y="3" width="18" height="18" rx="3"/>
          <path d="M9 12l2 2 4-4"/>
        </svg>
        <span className="sidebar-text">mintag</span>
      </div>

      {/* Nav */}
      <div style={{ padding: '8px 0', borderTop: '1px solid var(--color-border)' }}>
        <NavItem active={currentView === 'dashboard'} onClick={() => nav('dashboard')} icon={
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <rect x="3" y="3" width="7" height="7" rx="1"/>
            <rect x="14" y="3" width="7" height="7" rx="1"/>
            <rect x="14" y="14" width="7" height="7" rx="1"/>
            <rect x="3" y="14" width="7" height="7" rx="1"/>
          </svg>
        } label="Dashboard" />
        <NavItem active={currentView === 'tasks'} onClick={() => nav('tasks')} icon={
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M9 12l2 2 4-4"/>
            <rect x="3" y="3" width="18" height="18" rx="3"/>
          </svg>
        } label="Tasks" />
        <NavItem active={currentView === 'meetings'} onClick={() => nav('meetings')} icon={
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
            <circle cx="9" cy="7" r="4"/>
            <path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
            <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
          </svg>
        } label="Meetings" />
      </div>

      {/* Projects */}
      <div style={{ padding: '8px 0', borderTop: '1px solid var(--color-border)' }}>
        <div className="sidebar-text" style={{ fontSize: '0.68em', fontWeight: 600, letterSpacing: '0.8px', color: 'var(--color-text3)', padding: '6px 16px 2px', textTransform: 'uppercase' }}>
          Projects
        </div>
        {projects.map(p => (
          <button
            key={p.id}
            onClick={() => setActiveProject(p.id)}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              padding: '6px 14px',
              cursor: 'pointer',
              border: 'none',
              background: activeProject === p.id ? 'rgba(59,130,246,0.12)' : 'none',
              width: '100%',
              textAlign: 'left',
              color: activeProject === p.id ? 'var(--color-text)' : 'var(--color-text2)',
              fontSize: '0.85em',
              transition: 'all 0.15s',
            }}
          >
            <span style={{ width: 8, height: 8, borderRadius: '50%', background: p.color, flexShrink: 0, display: 'inline-block' }} />
            <span className="sidebar-text">{p.name}</span>
          </button>
        ))}
        <button
          onClick={() => openModal('new-project')}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            padding: '8px 16px',
            color: 'var(--color-text2)',
            cursor: 'pointer',
            border: 'none',
            background: 'none',
            width: '100%',
            textAlign: 'left',
            fontSize: '0.88em',
            transition: 'all 0.15s',
          }}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" style={{ flexShrink: 0 }}>
            <line x1="12" y1="5" x2="12" y2="19"/>
            <line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          <span className="sidebar-text">New project</span>
        </button>
      </div>

      <style>{`
        .sidebar { }
        @media (max-width: 900px) {
          .sidebar { width: 56px !important; }
          .sidebar-text { display: none !important; }
        }
        nav button:hover { background: var(--color-surface2) !important; color: var(--color-text) !important; }
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
    <button
      onClick={onClick}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        padding: '8px 16px',
        color: active ? 'var(--color-blue)' : 'var(--color-text2)',
        cursor: 'pointer',
        border: 'none',
        background: active ? 'rgba(59,130,246,0.12)' : 'none',
        width: '100%',
        textAlign: 'left',
        fontSize: '0.88em',
        fontWeight: active ? 600 : 400,
        transition: 'all 0.15s',
      }}
    >
      <span style={{ flexShrink: 0, opacity: active ? 1 : 0.7 }}>{icon}</span>
      <span className="sidebar-text">{label}</span>
    </button>
  )
}
