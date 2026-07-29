import { useState } from 'react'
import { useAppState, useAppActions } from '../../store/AppContext'
import { NAV_ITEMS } from '../layout/navItems'

// navLabel looks up the Spanish sidebar label for a catalog id, matching
// purely by id (the server-side MenuOptionStatus.label is English and not
// shown here) — falls back to the server label for any id the sidebar
// doesn't know about yet.
function navLabel(id: string, fallback: string): string {
  return NAV_ITEMS.find(n => n.id === id)?.label ?? fallback
}

export function MenuOptionsSection() {
  const { menuOptions } = useAppState()
  const { toggleMenuOption } = useAppActions()
  const [busyId, setBusyId] = useState<string | null>(null)

  async function handleToggle(id: string, enabled: boolean) {
    setBusyId(id)
    try {
      await toggleMenuOption(id, enabled)
    } finally {
      setBusyId(null)
    }
  }

  if (menuOptions.length === 0) {
    return (
      <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
        No se pudo cargar el catálogo de secciones.
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      {menuOptions.map(option => (
        <label
          key={option.id}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            padding: '8px 10px',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border)',
            background: 'var(--bg-sunken)',
            cursor: busyId === option.id ? 'default' : 'pointer',
          }}
        >
          <input
            type="checkbox"
            checked={option.enabled}
            disabled={busyId === option.id}
            onChange={e => handleToggle(option.id, e.target.checked)}
            aria-label={navLabel(option.id, option.label)}
          />
          <span style={{ flex: 1, font: 'var(--text-body)', color: 'var(--fg1)', fontSize: '0.9em' }}>
            {navLabel(option.id, option.label)}
          </span>
        </label>
      ))}
    </div>
  )
}
