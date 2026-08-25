import { useEffect, useState, useRef, type CSSProperties } from 'react'
import { X, Trash2, Plus, RotateCcw, Pencil, Check, Search } from 'lucide-react'
import type { ActivityCatalog, CatalogProject, TimelogCategory } from '../../types'
import {
  addCatalogProject,
  removeCatalogProject,
  reactivateTimelogProject,
  renameCatalogProject,
  addCatalogCategory,
  removeCatalogCategory,
  reactivateCatalogCategory,
  updateCatalogCategory,
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
  onReactivate,
  onRename,
  showInactive,
  onShowInactiveChange,
}: {
  title: string
  items: CatalogProject[]
  onAdd: (name: string) => Promise<void>
  onRemove: (name: string) => Promise<void>
  onReactivate: (name: string) => Promise<void>
  onRename: (oldName: string, newName: string) => Promise<void>
  showInactive: boolean
  onShowInactiveChange: (value: boolean) => void
}) {
  const [newName, setNewName] = useState('')
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState<Record<string, boolean>>({})
  const [searchQuery, setSearchQuery] = useState('')

  const [editingName, setEditingName] = useState<string | null>(null)
  const [editValue, setEditValue] = useState('')
  const [savingEdit, setSavingEdit] = useState(false)

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
    setBusy(prev => ({ ...prev, [name]: true }))
    try {
      await onRemove(name)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'No se pudo eliminar el registro')
    } finally {
      setBusy(prev => ({ ...prev, [name]: false }))
    }
  }

  async function handleReactivate(name: string) {
    setBusy(prev => ({ ...prev, [name]: true }))
    try {
      await onReactivate(name)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'No se pudo reactivar el registro')
    } finally {
      setBusy(prev => ({ ...prev, [name]: false }))
    }
  }

  function startEdit(name: string) {
    setEditingName(name)
    setEditValue(name)
    setError('')
  }

  function cancelEdit() {
    setEditingName(null)
  }

  async function saveEdit(oldName: string) {
    const name = editValue.trim()
    if (!name) {
      setError('El nombre no puede estar vacío')
      return
    }
    if (name === oldName) {
      setEditingName(null)
      return
    }
    setSavingEdit(true)
    setError('')
    try {
      await onRename(oldName, name)
      setEditingName(null)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'No se pudo renombrar el registro')
    } finally {
      setSavingEdit(false)
    }
  }

  const normalizedSearch = searchQuery.trim().toLowerCase()
  const visibleItems = normalizedSearch
    ? items.filter(item => item.name.toLowerCase().includes(normalizedSearch))
    : items

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

      <div style={{ position: 'relative', marginBottom: 10 }}>
        <Search size={14} strokeWidth={1.75} style={{ position: 'absolute', left: 8, top: '50%', transform: 'translateY(-50%)', color: 'var(--fg3)' }} />
        <input
          type="search"
          value={searchQuery}
          onChange={e => setSearchQuery(e.target.value)}
          placeholder={`Buscar ${title.toLowerCase()}...`}
          aria-label={`Buscar ${title.toLowerCase()}`}
          style={{ ...inputStyle, paddingLeft: 28 }}
        />
      </div>

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
        {visibleItems.length === 0 ? (
          <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', padding: '8px 0' }}>
            Sin registros
          </div>
        ) : (
          visibleItems.map(item => {
            const isEditing = editingName === item.name
            return (
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
                {isEditing ? (
                  <>
                    <input
                      type="text"
                      aria-label={`Nombre de ${item.name}`}
                      value={editValue}
                      onChange={e => setEditValue(e.target.value)}
                      onKeyDown={e => { if (e.key === 'Enter') saveEdit(item.name) }}
                      style={{ ...inputStyle, fontSize: '0.9em', padding: '4px 8px', flex: 1 }}
                      disabled={savingEdit}
                      autoFocus
                    />
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => saveEdit(item.name)}
                      disabled={savingEdit}
                      title="Guardar"
                      aria-label={`Guardar ${item.name}`}
                      style={{ padding: '2px 4px' }}
                    >
                      <Check size={13} strokeWidth={1.75} />
                    </button>
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={cancelEdit}
                      disabled={savingEdit}
                      title="Cancelar"
                      aria-label={`Cancelar edición de ${item.name}`}
                      style={{ padding: '2px 4px' }}
                    >
                      <X size={13} strokeWidth={1.75} />
                    </button>
                  </>
                ) : (
                  <>
                    <span style={{ flex: 1, font: 'var(--text-body)', color: 'var(--fg1)', fontSize: '0.9em' }}>
                      {item.name}
                    </span>
                    {!item.is_active && (
                      <span className="chip chip-todo" style={{ fontSize: '0.7em' }}>Inactivo</span>
                    )}
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => startEdit(item.name)}
                      title="Editar"
                      aria-label={`Editar ${item.name}`}
                      style={{ padding: '2px 4px' }}
                    >
                      <Pencil size={13} strokeWidth={1.75} />
                    </button>
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => handleRemove(item.name)}
                      disabled={busy[item.name] || !item.is_active}
                      title={item.is_active ? `Eliminar ${item.name}` : `${item.name} ya está inactivo`}
                      aria-label={`Eliminar ${item.name}`}
                      style={{ padding: '2px 4px' }}
                    >
                      <Trash2 size={13} strokeWidth={1.75} />
                    </button>
                    {!item.is_active && (
                      <button
                        className="btn btn-ghost btn-sm"
                        onClick={() => handleReactivate(item.name)}
                        disabled={busy[item.name]}
                        title={`Reactivar ${item.name}`}
                        aria-label={`Reactivar ${item.name}`}
                        style={{ padding: '2px 4px' }}
                      >
                        <RotateCcw size={13} strokeWidth={1.75} />
                      </button>
                    )}
                  </>
                )}
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}

function CategorySection({
  categories,
  onAdd,
  onRemove,
  onReactivate,
  onSaved,
  showInactive,
  onShowInactiveChange,
}: {
  categories: TimelogCategory[]
  onAdd: (name: string, description: string) => Promise<void>
  onRemove: (name: string) => Promise<void>
  onReactivate: (name: string) => Promise<void>
  onSaved: () => void
  showInactive: boolean
  onShowInactiveChange: (value: boolean) => void
}) {
  const [newName, setNewName] = useState('')
  const [newDescription, setNewDescription] = useState('')
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState<Record<string, boolean>>({})
  const [searchQuery, setSearchQuery] = useState('')

  const [editingId, setEditingId] = useState<number | null>(null)
  const [editName, setEditName] = useState('')
  const [editDescription, setEditDescription] = useState('')
  const [savingEdit, setSavingEdit] = useState(false)

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

  function startEdit(category: TimelogCategory) {
    setEditingId(category.id)
    setEditName(category.name)
    setEditDescription(category.description ?? '')
    setError('')
  }

  function cancelEdit() {
    setEditingId(null)
  }

  async function saveEdit(category: TimelogCategory) {
    const name = editName.trim()
    if (!name) {
      setError('El nombre no puede estar vacío')
      return
    }
    setSavingEdit(true)
    setError('')
    try {
      await updateCatalogCategory(category.id, name, editDescription.trim())
      setEditingId(null)
      onSaved()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'No se pudo guardar la categoría')
    } finally {
      setSavingEdit(false)
    }
  }

  async function handleRemove(name: string) {
    setBusy(prev => ({ ...prev, [name]: true }))
    try {
      await onRemove(name)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'No se pudo eliminar el registro')
    } finally {
      setBusy(prev => ({ ...prev, [name]: false }))
    }
  }

  async function handleReactivate(name: string) {
    setBusy(prev => ({ ...prev, [name]: true }))
    try {
      await onReactivate(name)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'No se pudo reactivar el registro')
    } finally {
      setBusy(prev => ({ ...prev, [name]: false }))
    }
  }

  const normalizedSearch = searchQuery.trim().toLowerCase()
  const visibleCategories = normalizedSearch
    ? categories.filter(c =>
        c.name.toLowerCase().includes(normalizedSearch) ||
        (c.description ?? '').toLowerCase().includes(normalizedSearch),
      )
    : categories

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

      <div style={{ position: 'relative', marginBottom: 10 }}>
        <Search size={14} strokeWidth={1.75} style={{ position: 'absolute', left: 8, top: '50%', transform: 'translateY(-50%)', color: 'var(--fg3)' }} />
        <input
          type="search"
          value={searchQuery}
          onChange={e => setSearchQuery(e.target.value)}
          placeholder="Buscar categorías..."
          aria-label="Buscar categorías"
          style={{ ...inputStyle, paddingLeft: 28 }}
        />
      </div>

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
        {visibleCategories.length === 0 ? (
          <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', padding: '8px 0' }}>
            Sin registros
          </div>
        ) : (
          visibleCategories.map(c => {
            const isEditing = editingId === c.id
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
                  opacity: c.is_active ? 1 : 0.55,
                }}
              >
                {isEditing ? (
                  <>
                    <div style={{ display: 'flex', gap: 6 }}>
                      <input
                        type="text"
                        aria-label={`Nombre de ${c.name}`}
                        value={editName}
                        onChange={e => setEditName(e.target.value)}
                        style={{ ...inputStyle, fontSize: '0.85em', padding: '4px 8px', flex: 1 }}
                        disabled={savingEdit}
                      />
                      <button
                        className="btn btn-ghost btn-sm"
                        onClick={() => saveEdit(c)}
                        disabled={savingEdit}
                        title="Guardar"
                        aria-label={`Guardar ${c.name}`}
                        style={{ padding: '2px 4px' }}
                      >
                        <Check size={13} strokeWidth={1.75} />
                      </button>
                      <button
                        className="btn btn-ghost btn-sm"
                        onClick={cancelEdit}
                        disabled={savingEdit}
                        title="Cancelar"
                        aria-label={`Cancelar edición de ${c.name}`}
                        style={{ padding: '2px 4px' }}
                      >
                        <X size={13} strokeWidth={1.75} />
                      </button>
                    </div>
                    <input
                      type="text"
                      aria-label={`Descripción de ${c.name}`}
                      value={editDescription}
                      onChange={e => setEditDescription(e.target.value)}
                      onKeyDown={e => { if (e.key === 'Enter') saveEdit(c) }}
                      placeholder="Descripción..."
                      disabled={savingEdit}
                      style={{ ...inputStyle, fontSize: '0.85em', padding: '4px 8px' }}
                    />
                  </>
                ) : (
                  <>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span style={{ flex: 1, font: 'var(--text-body)', color: 'var(--fg1)', fontSize: '0.9em' }}>
                        {c.name}
                      </span>
                      {!c.is_active && (
                        <span className="chip chip-todo" style={{ fontSize: '0.7em' }}>Inactivo</span>
                      )}
                      <button
                        className="btn btn-ghost btn-sm"
                        onClick={() => startEdit(c)}
                        title="Editar"
                        aria-label={`Editar ${c.name}`}
                        style={{ padding: '2px 4px' }}
                      >
                        <Pencil size={13} strokeWidth={1.75} />
                      </button>
                      <button
                        className="btn btn-ghost btn-sm"
                        onClick={() => handleRemove(c.name)}
                        disabled={busy[c.name] || !c.is_active}
                        title={c.is_active ? `Eliminar ${c.name}` : `${c.name} ya está inactivo`}
                        aria-label={`Eliminar ${c.name}`}
                        style={{ padding: '2px 4px' }}
                      >
                        <Trash2 size={13} strokeWidth={1.75} />
                      </button>
                      {!c.is_active && (
                        <button
                          className="btn btn-ghost btn-sm"
                          onClick={() => handleReactivate(c.name)}
                          disabled={busy[c.name]}
                          title={`Reactivar ${c.name}`}
                          aria-label={`Reactivar ${c.name}`}
                          style={{ padding: '2px 4px' }}
                        >
                          <RotateCcw size={13} strokeWidth={1.75} />
                        </button>
                      )}
                    </div>
                    {c.description && (
                      <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
                        {c.description}
                      </span>
                    )}
                  </>
                )}
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

  // `catalog.projects`/`catalog.categories` are fetched active-only, since
  // that's what every other consumer (the new/edit activity forms) needs.
  // "Mostrar inactivos" pulls a separate include_inactive=true snapshot on
  // demand instead of widening the parent's fetch, so those other consumers
  // never see deactivated entries. Both tabs share one snapshot fetch since
  // GET /api/activities/catalog?include_inactive=true returns both projects
  // and categories together.
  const [showInactiveProjects, setShowInactiveProjects] = useState(false)
  const [showInactiveCategories, setShowInactiveCategories] = useState(false)
  const [catalogWithInactive, setCatalogWithInactive] = useState<ActivityCatalog | null>(null)

  useEffect(() => {
    if (!open || !(showInactiveProjects || showInactiveCategories)) return
    getActivityCatalog(true).then(setCatalogWithInactive).catch(() => setCatalogWithInactive(null))
  }, [open, showInactiveProjects, showInactiveCategories, catalog])

  if (!open) return null

  const visibleProjects = showInactiveProjects ? (catalogWithInactive?.projects ?? catalog?.projects ?? []) : (catalog?.projects ?? [])
  const visibleCategories = showInactiveCategories ? (catalogWithInactive?.categories ?? catalog?.categories ?? []) : (catalog?.categories ?? [])

  // refreshInactiveSnapshot re-pulls the include_inactive=true snapshot after
  // a mutation, so a reactivated/deactivated row's is_active flips in the UI
  // immediately without closing and reopening the modal.
  function refreshInactiveSnapshot() {
    if (showInactiveProjects || showInactiveCategories) {
      getActivityCatalog(true).then(setCatalogWithInactive).catch(() => {})
    }
  }

  async function handleAddProject(name: string) {
    await addCatalogProject(name)
    onCatalogChanged()
    refreshInactiveSnapshot()
  }

  async function handleRemoveProject(name: string) {
    await removeCatalogProject(name)
    onCatalogChanged()
    refreshInactiveSnapshot()
  }

  async function handleReactivateProject(name: string) {
    await reactivateTimelogProject(name)
    onCatalogChanged()
    refreshInactiveSnapshot()
  }

  async function handleRenameProject(oldName: string, newName: string) {
    await renameCatalogProject(oldName, newName)
    onCatalogChanged()
    refreshInactiveSnapshot()
  }

  async function handleAddCategory(name: string, description: string) {
    await addCatalogCategory(name, description)
    onCatalogChanged()
    refreshInactiveSnapshot()
  }

  async function handleRemoveCategory(name: string) {
    await removeCatalogCategory(name)
    onCatalogChanged()
    refreshInactiveSnapshot()
  }

  async function handleReactivateCategory(name: string) {
    await reactivateCatalogCategory(name)
    onCatalogChanged()
    refreshInactiveSnapshot()
  }

  function handleCatalogSaved() {
    onCatalogChanged()
    refreshInactiveSnapshot()
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
              onReactivate={handleReactivateProject}
              onRename={handleRenameProject}
              showInactive={showInactiveProjects}
              onShowInactiveChange={setShowInactiveProjects}
            />
          )}
          {activeTab === 'categories' && (
            <CategorySection
              categories={visibleCategories}
              onAdd={handleAddCategory}
              onRemove={handleRemoveCategory}
              onReactivate={handleReactivateCategory}
              onSaved={handleCatalogSaved}
              showInactive={showInactiveCategories}
              onShowInactiveChange={setShowInactiveCategories}
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
