import { useEffect, useState, useRef, type CSSProperties } from 'react'
import { X, Trash2, Plus } from 'lucide-react'
import type { ActivityCatalog, CatalogProject, TimelogCategory } from '../../types'
import {
  addCatalogProject,
  removeCatalogProject,
  addCatalogCategory,
  removeCatalogCategory,
  updateCatalogCategoryDescription,
  getActivityCatalog,
} from '../../api/client'

interface CatalogManagementModalProps {
  open: boolean
  onClose: () => void
  catalog: ActivityCatalog | null
  onCatalogChanged: () => void
}

const inputStyle: CSSProperties = {
  width: '100%',
  padding: '8px 12px',
  border: '1px solid var(--border-strong)',
  borderRadius: 'var(--radius-md)',
  font: 'var(--text-body)',
  color: 'var(--fg1)',
  background: 'var(--bg-sunken)',
  outline: 'none',
  boxSizing: 'border-box',
}

function CatalogSection({
  title,
  items,
  onAdd,
  onRemove,
  showInactive,
  onShowInactiveChange,
}: {
  title: string
  items: CatalogProject[]
  onAdd: (name: string) => Promise<void>
  onRemove: (name: string) => Promise<void>
  showInactive: boolean
  onShowInactiveChange: (value: boolean) => void
}) {
  const [newName, setNewName] = useState('')
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState('')
  const [removing, setRemoving] = useState<Record<string, boolean>>({})

  async function handleAdd() {
    const name = newName.trim()
    if (!name) return
    setAdding(true)
    setError('')
    try {
      await onAdd(name)
      setNewName('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'No se pudo agregar el registro')
    } finally {
      setAdding(false)
    }
  }

  async function handleRemove(name: string) {
    setRemoving(prev => ({ ...prev, [name]: true }))
    try {
      await onRemove(name)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'No se pudo eliminar el registro')
    } finally {
      setRemoving(prev => ({ ...prev, [name]: false }))
    }
  }

  return (
    <div>
      {/* Add row */}
      <div style={{ display: 'flex', gap: 6, marginBottom: 10 }}>
        <input
          type="text"
          value={newName}
          onChange={e => setNewName(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') handleAdd() }}
          placeholder={`Nuevo ${title.toLowerCase().slice(0, -1)}...`}
          style={{ ...inputStyle, flex: 1 }}
          disabled={adding}
        />
        <button
          className="btn btn-primary btn-sm"
          onClick={handleAdd}
          disabled={adding || !newName.trim()}
          aria-label={`Agregar ${title}`}
        >
          <Plus size={14} strokeWidth={1.75} />
        </button>
      </div>

      {error && (
        <div style={{ font: 'var(--text-caption)', color: 'var(--block-solid)', marginBottom: 8 }}>
          {error}
        </div>
      )}

      <label style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8, cursor: 'pointer' }}>
        <input
          type="checkbox"
          checked={showInactive}
          onChange={e => onShowInactiveChange(e.target.checked)}
        />
        <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>Mostrar inactivos</span>
      </label>

      {/* Item list */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4, maxHeight: 360, overflowY: 'auto' }}>
        {items.length === 0 ? (
          <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', padding: '8px 0' }}>
            Sin registros
          </div>
        ) : (
          items.map(item => (
            <div
              key={item.name}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                padding: '6px 10px',
                background: 'var(--bg-sunken)',
                borderRadius: 'var(--radius-md)',
                border: '1px solid var(--border)',
                opacity: item.is_active ? 1 : 0.55,
              }}
            >
              <span style={{ flex: 1, font: 'var(--text-body)', color: 'var(--fg1)', fontSize: '0.9em' }}>
                {item.name}
              </span>
              {!item.is_active && (
                <span className="chip chip-todo" style={{ fontSize: '0.7em' }}>Inactivo</span>
              )}
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => handleRemove(item.name)}
                disabled={removing[item.name] || !item.is_active}
                title={item.is_active ? `Eliminar ${item.name}` : `${item.name} ya está inactivo`}
                aria-label={`Eliminar ${item.name}`}
                style={{ padding: '2px 4px' }}
              >
                <Trash2 size={13} strokeWidth={1.75} />
              </button>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

function CategorySection({
  categories,
  onAdd,
  onRemove,
  onDescriptionSaved,
}: {
  categories: TimelogCategory[]
  onAdd: (name: string, description: string) => Promise<void>
  onRemove: (name: string) => Promise<void>
  onDescriptionSaved: () => void
}) {
  const [newName, setNewName] = useState('')
  const [newDescription, setNewDescription] = useState('')
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState('')
  const [removing, setRemoving] = useState<Record<string, boolean>>({})
  const [descDrafts, setDescDrafts] = useState<Record<number, string>>({})
  const [savingDescription, setSavingDescription] = useState<Record<number, boolean>>({})

  async function handleAdd() {
    const name = newName.trim()
    if (!name) return
    setAdding(true)
    setError('')
    try {
      await onAdd(name, newDescription.trim())
      setNewName('')
      setNewDescription('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'No se pudo agregar el registro')
    } finally {
      setAdding(false)
    }
  }

  async function handleDescriptionBlur(category: TimelogCategory) {
    const draft = descDrafts[category.id]
    if (draft === undefined) return
    const trimmed = draft.trim()
    if (trimmed === (category.description ?? '')) {
      setDescDrafts(prev => {
        const next = { ...prev }
        delete next[category.id]
        return next
      })
      return
    }
    setSavingDescription(prev => ({ ...prev, [category.id]: true }))
    setError('')
    try {
      await updateCatalogCategoryDescription(category.id, trimmed)
      setDescDrafts(prev => {
        const next = { ...prev }
        delete next[category.id]
        return next
      })
      onDescriptionSaved()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'No se pudo guardar la descripción')
    } finally {
      setSavingDescription(prev => ({ ...prev, [category.id]: false }))
    }
  }

  async function handleRemove(name: string) {
    setRemoving(prev => ({ ...prev, [name]: true }))
    try {
      await onRemove(name)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'No se pudo eliminar el registro')
    } finally {
      setRemoving(prev => ({ ...prev, [name]: false }))
    }
  }

  return (
    <div>
      {/* Add row */}
      <div style={{ display: 'flex', gap: 6, marginBottom: 10, flexWrap: 'wrap' }}>
        <input
          type="text"
          value={newName}
          onChange={e => setNewName(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') handleAdd() }}
          placeholder="Nueva categoría..."
          style={{ ...inputStyle, flex: '1 1 140px' }}
          disabled={adding}
        />
        <input
          type="text"
          value={newDescription}
          onChange={e => setNewDescription(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') handleAdd() }}
          placeholder="Descripción (opcional)..."
          style={{ ...inputStyle, flex: '1 1 160px' }}
          disabled={adding}
        />
        <button
          className="btn btn-primary btn-sm"
          onClick={handleAdd}
          disabled={adding || !newName.trim()}
          aria-label="Agregar categorías"
        >
          <Plus size={14} strokeWidth={1.75} />
        </button>
      </div>

      {error && (
        <div style={{ font: 'var(--text-caption)', color: 'var(--block-solid)', marginBottom: 8 }}>
          {error}
        </div>
      )}

      {/* Item list */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4, maxHeight: 360, overflowY: 'auto' }}>
        {categories.length === 0 ? (
          <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', padding: '8px 0' }}>
            Sin registros
          </div>
        ) : (
          categories.map(c => {
            return (
              <div
                key={c.id}
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 6,
                  padding: '6px 10px',
                  background: 'var(--bg-sunken)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--border)',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span style={{ flex: 1, font: 'var(--text-body)', color: 'var(--fg1)', fontSize: '0.9em' }}>
                    {c.name}
                  </span>
                  <button
                    className="btn btn-ghost btn-sm"
                    onClick={() => handleRemove(c.name)}
                    disabled={removing[c.name]}
                    title={`Eliminar ${c.name}`}
                    aria-label={`Eliminar ${c.name}`}
                    style={{ padding: '2px 4px' }}
                  >
                    <Trash2 size={13} strokeWidth={1.75} />
                  </button>
                </div>
                <input
                  type="text"
                  aria-label={`Descripción de ${c.name}`}
                  value={descDrafts[c.id] ?? c.description ?? ''}
                  onChange={e => setDescDrafts(prev => ({ ...prev, [c.id]: e.target.value }))}
                  onBlur={() => handleDescriptionBlur(c)}
                  onKeyDown={e => { if (e.key === 'Enter') (e.target as HTMLInputElement).blur() }}
                  placeholder="Descripción..."
                  disabled={!!savingDescription[c.id]}
                  style={{ ...inputStyle, fontSize: '0.85em', padding: '4px 8px' }}
                />
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}

type CatalogTab = 'projects' | 'categories'

const TABS: { id: CatalogTab; label: string }[] = [
  { id: 'projects', label: 'Proyectos' },
  { id: 'categories', label: 'Categorías' },
]

export function CatalogManagementModal({ open, onClose, catalog, onCatalogChanged }: CatalogManagementModalProps) {
  const pressedOnOverlay = useRef(false)
  const [activeTab, setActiveTab] = useState<CatalogTab>('projects')

  // `catalog.projects` is fetched active-only, since that's what every other
  // consumer (the new/edit activity forms) needs. "Mostrar inactivos" pulls a
  // separate include_inactive=true snapshot on demand instead of widening the
  // parent's fetch, so those other consumers never see deactivated entries.
  const [showInactiveProjects, setShowInactiveProjects] = useState(false)
  const [projectsWithInactive, setProjectsWithInactive] = useState<CatalogProject[] | null>(null)

  useEffect(() => {
    if (!open || !showInactiveProjects) return
    getActivityCatalog(true).then(c => setProjectsWithInactive(c.projects)).catch(() => setProjectsWithInactive([]))
  }, [open, showInactiveProjects, catalog])

  if (!open) return null

  const visibleProjects = showInactiveProjects ? (projectsWithInactive ?? catalog?.projects ?? []) : (catalog?.projects ?? [])

  async function handleAddProject(name: string) {
    await addCatalogProject(name)
    onCatalogChanged()
  }

  async function handleRemoveProject(name: string) {
    await removeCatalogProject(name)
    onCatalogChanged()
  }

  async function handleAddCategory(name: string, description: string) {
    await addCatalogCategory(name, description)
    onCatalogChanged()
  }

  async function handleRemoveCategory(name: string) {
    await removeCatalogCategory(name)
    onCatalogChanged()
  }

  return (
    <div
      onMouseDown={e => { pressedOnOverlay.current = e.target === e.currentTarget }}
      onClick={e => {
        if (pressedOnOverlay.current && e.target === e.currentTarget) onClose()
        pressedOnOverlay.current = false
      }}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(15,23,42,0.5)',
        zIndex: 100,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 20,
      }}
    >
      <div
        onClick={e => e.stopPropagation()}
        className="card"
        style={{
          borderRadius: 'var(--radius-xl)',
          boxShadow: 'var(--shadow-xl)',
          width: '100%',
          maxWidth: 720,
          maxHeight: '90vh',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        {/* Header */}
        <div
          style={{
            padding: '16px 20px',
            borderBottom: '1px solid var(--border)',
            display: 'flex',
            alignItems: 'center',
            gap: 10,
          }}
        >
          <h2 style={{ font: 'var(--text-h3)', color: 'var(--fg1)', flex: 1, margin: 0 }}>
            Gestionar catálogo
          </h2>
          <button
            onClick={onClose}
            style={{
              background: 'none',
              border: 'none',
              color: 'var(--fg3)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              padding: 4,
              borderRadius: 'var(--radius-md)',
            }}
            aria-label="Cerrar"
          >
            <X size={18} strokeWidth={1.75} />
          </button>
        </div>

        {/* Tabs */}
        <div
          style={{
            display: 'flex',
            gap: 4,
            padding: '0 22px',
            borderBottom: '1px solid var(--border)',
          }}
        >
          {TABS.map(tab => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              style={{
                background: 'none',
                border: 'none',
                borderBottom: `2px solid ${activeTab === tab.id ? 'var(--accent)' : 'transparent'}`,
                color: activeTab === tab.id ? 'var(--fg1)' : 'var(--fg3)',
                font: 'var(--text-body)',
                fontWeight: activeTab === tab.id ? 600 : 400,
                fontSize: '0.9em',
                padding: '10px 8px',
                cursor: 'pointer',
              }}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Body */}
        <div style={{ padding: '20px 22px', overflowY: 'auto', flex: 1 }}>
          {activeTab === 'projects' && (
            <CatalogSection
              title="Proyectos"
              items={visibleProjects}
              onAdd={handleAddProject}
              onRemove={handleRemoveProject}
              showInactive={showInactiveProjects}
              onShowInactiveChange={setShowInactiveProjects}
            />
          )}
          {activeTab === 'categories' && (
            <CategorySection
              categories={catalog?.categories ?? []}
              onAdd={handleAddCategory}
              onRemove={handleRemoveCategory}
              onDescriptionSaved={onCatalogChanged}
            />
          )}
        </div>

        {/* Footer */}
        <div
          style={{
            padding: '14px 22px',
            borderTop: '1px solid var(--border)',
            display: 'flex',
            justifyContent: 'flex-end',
          }}
        >
          <button onClick={onClose} className="btn btn-ghost">
            Cerrar
          </button>
        </div>
      </div>
    </div>
  )
}
