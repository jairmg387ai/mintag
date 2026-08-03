import { Plus, Settings } from 'lucide-react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { Avatar } from '../shared/Avatar'
import { NAV_ITEMS } from './navItems'

export function Sidebar() {
  const { currentView, projects, activeProject, menuOptions, azureConfig } = useAppState()
  const { setView, setActiveProject, openModal } = useAppActions()

  // Fail open: an empty menuOptions list (fetch hasn't resolved yet, or the
  // request failed) must never produce an unusable, empty sidebar — render
  // every nav item until the resolved catalog says otherwise.
  const enabledIds = new Set(menuOptions.filter(m => m.enabled).map(m => m.id))
  const visibleItems = enabledIds.size === 0 ? NAV_ITEMS : NAV_ITEMS.filter(n => enabledIds.has(n.id))

  // Shorten "Jair Reinel Muñoz Gomez" to "Jair Muñoz" — first given name +
  // first surname — so it fits the sidebar footer without wrapping.
  const shortName = (full: string) => {
    const parts = full.trim().split(/\s+/)
    if (parts.length <= 2) return full
    return `${parts[0]} ${parts[Math.ceil(parts.length / 2)]}`
  }
  const workspaceName = azureConfig?.user_display_name ? shortName(azureConfig.user_display_name) : 'Espacio local'

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
        <div className="head">Espacio de trabajo</div>
        {visibleItems.map(n => (
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
        <div className="head">Proyectos</div>
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
          Nuevo proyecto
        </button>
      </div>

      {/* Footer */}
      <div className="sb-foot">
        <Avatar name={workspaceName} size={32} />
        <div style={{ flex: 1, minWidth: 0 }}>
          <div className="who">{workspaceName}</div>
          <div className="role">Personal</div>
        </div>
        <button
          className="sb-item"
          style={{ width: 'auto', padding: 8, marginBottom: 0 }}
          onClick={() => setView('settings')}
          title="Configuración"
          aria-label="Configuración"
        >
          <Settings size={18} strokeWidth={1.75} />
        </button>
      </div>
    </nav>
  )
}
