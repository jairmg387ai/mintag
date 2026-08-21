import { useEffect, useState, useRef, type CSSProperties } from 'react'
import { X, Trash2, Plus, Star, Pencil, Check, Download } from 'lucide-react'
import type { ActivityCatalog, AssignedAzureWorkItem, AzureActivity, CatalogProject, TimelogCategory } from '../../types'
import {
  addCatalogProject,
  removeCatalogProject,
  addCatalogCategory,
  removeCatalogCategory,
  updateCatalogCategoryDescription,
  addAzureActivity,
  updateAzureActivity,
  deactivateAzureActivity,
  setDefaultAzureActivity,
  listAssignedAzureWorkItems,
  getActivityCatalog,
  listAzureActivities,
} from '../../api/client'
import { friendlyCatalogErrorMessage } from './azureActivity'
import { AzureWorkItemLink } from './AzureActivityLink'

interface CatalogManagementModalProps {
  open: boolean
  onClose: () => void
  catalog: ActivityCatalog | null
  onCatalogChanged: () => void
  azureActivities: AzureActivity[]
  onAzureActivitiesChanged: () => void
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

function AzureActivitySection({
  activities,
  catalog,
  onChanged,
  showInactive,
  onShowInactiveChange,
}: {
  activities: AzureActivity[]
  catalog: ActivityCatalog | null
  onChanged: () => void
  showInactive: boolean
  onShowInactiveChange: (value: boolean) => void
}) {
  const [org, setOrg] = useState('')
  const [workItemId, setWorkItemId] = useState('')
  const [label, setLabel] = useState('')
  const [workItemType, setWorkItemType] = useState('')
  const [project, setProject] = useState('')
  const [category, setCategory] = useState('')
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState('')
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editOrg, setEditOrg] = useState('')
  const [editLabel, setEditLabel] = useState('')
  const [editWorkItemType, setEditWorkItemType] = useState('')
  const [editProject, setEditProject] = useState('')
  const [editCategory, setEditCategory] = useState('')
  const [busyId, setBusyId] = useState<number | null>(null)

  const [importOpen, setImportOpen] = useState(false)
  const [importLoading, setImportLoading] = useState(false)
  const [importError, setImportError] = useState('')
  const [importOrg, setImportOrg] = useState('')
  const [importItems, setImportItems] = useState<AssignedAzureWorkItem[] | null>(null)
  const [importingId, setImportingId] = useState<number | null>(null)
  const [importTypeFilter, setImportTypeFilter] = useState<'all' | 'Bug' | 'Task'>('all')
  const [importSearch, setImportSearch] = useState('')

  const existingWorkItemKeys = new Set(
    activities.map(a => `${a.org.trim().toLowerCase()}#${a.work_item_id}`),
  )
  const normalizedImportSearch = importSearch.trim().toLowerCase()
  const filteredImportItems = importItems?.filter(item => {
    const matchesType = importTypeFilter === 'all' || item.type === importTypeFilter
    if (!matchesType) return false
    if (!normalizedImportSearch) return true
    return [item.id, item.title, item.type, item.state]
      .join(' ')
      .toLowerCase()
      .includes(normalizedImportSearch)
  }) ?? null

  async function handleToggleImport() {
    if (importOpen) {
      setImportOpen(false)
      return
    }
    setImportOpen(true)
    setImportLoading(true)
    setImportError('')
    setImportTypeFilter('all')
    setImportSearch('')
    try {
      const { org: fetchedOrg, items } = await listAssignedAzureWorkItems()
      setImportOrg(fetchedOrg)
      setImportItems(items)
    } catch (e: unknown) {
      setImportError(friendlyCatalogErrorMessage(e, 'No se pudieron obtener los work items asignados'))
    } finally {
      setImportLoading(false)
    }
  }

  async function handleImportItem(item: AssignedAzureWorkItem) {
    setImportingId(item.id)
    setImportError('')
    try {
      await addAzureActivity({ org: importOrg, work_item_id: item.id, label: item.title, work_item_type: item.type })
      onChanged()
    } catch (e: unknown) {
      setImportError(friendlyCatalogErrorMessage(e, 'No se pudo importar el work item'))
    } finally {
      setImportingId(null)
    }
  }

  async function handleAdd() {
    const wid = parseInt(workItemId, 10)
    if (!org.trim() || !label.trim() || !workItemId.trim() || isNaN(wid)) {
      setError('La organización, el ID del work item y la etiqueta son obligatorios')
      return
    }
    setAdding(true)
    setError('')
    try {
      await addAzureActivity({
        org: org.trim(),
        work_item_id: wid,
        label: label.trim(),
        work_item_type: workItemType || undefined,
        project: project || undefined,
        category_id: category ? Number(category) : undefined,
      })
      setOrg('')
      setWorkItemId('')
      setLabel('')
      setWorkItemType('')
      setProject('')
      setCategory('')
      onChanged()
    } catch (e: unknown) {
      setError(friendlyCatalogErrorMessage(e, 'No se pudo agregar la actividad'))
    } finally {
      setAdding(false)
    }
  }

  function startEdit(a: AzureActivity) {
    setEditingId(a.id)
    setEditOrg(a.org)
    setEditLabel(a.label)
    setEditWorkItemType(a.work_item_type ?? '')
    setEditProject(a.project ?? '')
    setEditCategory(a.category_id != null ? String(a.category_id) : '')
    setError('')
  }

  // UpdateAzureActivity is a full replace: project/category_id are always
  // sent (empty string / '' -> null), never omitted, so an unedited field
  // isn't silently cleared by relying on "undefined drops the key" like the
  // add row does — that behavior only applies to inserts.
  async function saveEdit(id: number) {
    if (!editOrg.trim() || !editLabel.trim()) {
      setError('La organización y la etiqueta son obligatorias')
      return
    }
    setBusyId(id)
    setError('')
    try {
      await updateAzureActivity(id, {
        org: editOrg.trim(),
        label: editLabel.trim(),
        work_item_type: editWorkItemType,
        project: editProject.trim() || null,
        category_id: editCategory ? Number(editCategory) : null,
      })
      setEditingId(null)
      onChanged()
    } catch (e: unknown) {
      setError(friendlyCatalogErrorMessage(e, 'No se pudo actualizar la actividad'))
    } finally {
      setBusyId(null)
    }
  }

  async function handleSetDefault(id: number) {
    setBusyId(id)
    setError('')
    try {
      await setDefaultAzureActivity(id)
      onChanged()
    } catch (e: unknown) {
      setError(friendlyCatalogErrorMessage(e, 'No se pudo definir la actividad predeterminada'))
    } finally {
      setBusyId(null)
    }
  }

  async function handleDeactivate(id: number) {
    setBusyId(id)
    setError('')
    try {
      await deactivateAzureActivity(id)
      onChanged()
    } catch (e: unknown) {
      setError(friendlyCatalogErrorMessage(e, 'No se pudo desactivar la actividad'))
    } finally {
      setBusyId(null)
    }
  }

  return (
    <div>
      {/* Import from Azure */}
      <div style={{ marginBottom: 10 }}>
        <button
          className="btn btn-ghost btn-sm"
          onClick={handleToggleImport}
          style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}
        >
          <Download size={14} strokeWidth={1.75} />
          {importOpen ? 'Ocultar work items asignados' : 'Importar mis work items de Azure'}
        </button>

        {importOpen && (
          <div
            style={{
              marginTop: 8,
              display: 'flex',
              flexDirection: 'column',
              gap: 4,
              maxHeight: 260,
              overflowY: 'auto',
            }}
          >
            {importLoading && (
              <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', padding: '8px 0' }}>
                Consultando Azure…
              </div>
            )}
            {!importLoading && importError && (
              <div style={{ font: 'var(--text-caption)', color: 'var(--block-solid)', padding: '4px 0' }}>
                {importError}
              </div>
            )}
            {!importLoading && !importError && importItems?.length === 0 && (
              <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', padding: '8px 0' }}>
                No tienes work items abiertos asignados
              </div>
            )}
            {!importLoading && !importError && importItems && importItems.length > 0 && (
              <>
                <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginBottom: 4 }}>
                  <select
                    aria-label="Filtrar work items por tipo"
                    value={importTypeFilter}
                    onChange={e => setImportTypeFilter(e.target.value as 'all' | 'Bug' | 'Task')}
                    style={{ ...inputStyle, flex: '0 1 120px', fontSize: '0.85em', padding: '6px 8px' }}
                  >
                    <option value="all">Todos</option>
                    <option value="Bug">Bug</option>
                    <option value="Task">Task</option>
                  </select>
                  <input
                    type="search"
                    value={importSearch}
                    onChange={e => setImportSearch(e.target.value)}
                    placeholder="Buscar por ID, título o estado..."
                    aria-label="Buscar work items asignados"
                    style={{ ...inputStyle, flex: '1 1 180px', fontSize: '0.85em', padding: '6px 8px' }}
                  />
                </div>
                <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', padding: '0 0 4px' }}>
                  Mostrando {filteredImportItems?.length ?? 0} de {importItems.length}
                </div>
              </>
            )}
            {!importLoading && !importError && importItems && importItems.length > 0 && filteredImportItems?.length === 0 && (
              <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', padding: '8px 0' }}>
                No hay work items que coincidan con los filtros
              </div>
            )}
            {!importLoading && !importError && filteredImportItems?.map(item => {
              const alreadyAdded = existingWorkItemKeys.has(`${importOrg.trim().toLowerCase()}#${item.id}`)
              return (
                <div
                  key={item.id}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                    padding: '6px 10px',
                    background: 'var(--bg-sunken)',
                    borderRadius: 'var(--radius-md)',
                    border: '1px solid var(--border)',
                  }}
                >
                  <span style={{ flex: 1, font: 'var(--text-body)', color: 'var(--fg1)', fontSize: '0.9em' }}>
                    {item.title}{' '}
                    <span style={{ color: 'var(--fg3)' }}>
                      — {item.type} #{item.id} · {item.state}
                    </span>
                  </span>
                  {alreadyAdded ? (
                    <span className="chip chip-done" style={{ fontSize: '0.75em' }}>En catálogo</span>
                  ) : (
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => handleImportItem(item)}
                      disabled={importingId === item.id}
                      title="Agregar al catálogo"
                      aria-label={`Agregar ${item.title} al catálogo`}
                      style={{ padding: '2px 4px' }}
                    >
                      <Plus size={13} strokeWidth={1.75} />
                    </button>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Add row */}
      <div style={{ display: 'flex', gap: 6, marginBottom: 10, flexWrap: 'wrap' }}>
        <input
          type="text"
          value={org}
          onChange={e => setOrg(e.target.value)}
          placeholder="Organización (ej. RUNT2PSW)"
          style={{ ...inputStyle, flex: '1 1 140px' }}
          disabled={adding}
        />
        <input
          type="number"
          value={workItemId}
          onChange={e => setWorkItemId(e.target.value)}
          placeholder="ID del work item"
          style={{ ...inputStyle, flex: '1 1 120px' }}
          disabled={adding}
        />
        <input
          type="text"
          value={label}
          onChange={e => setLabel(e.target.value)}
          placeholder="Etiqueta"
          style={{ ...inputStyle, flex: '1 1 160px' }}
          disabled={adding}
        />
        <select
          aria-label="Tipo de work item"
          value={workItemType}
          onChange={e => setWorkItemType(e.target.value)}
          style={{ ...inputStyle, flex: '0 1 120px' }}
          disabled={adding}
        >
          <option value="">Sin tipo</option>
          <option value="Bug">Bug</option>
          <option value="Task">Task</option>
        </select>
        {catalog !== null ? (
          <select
            aria-label="Proyecto"
            value={project}
            onChange={e => setProject(e.target.value)}
            style={{ ...inputStyle, flex: '0 1 140px' }}
            disabled={adding}
          >
            <option value="">Sin proyecto</option>
            {catalog.projects.map(p => (
              <option key={p.name} value={p.name}>{p.name}</option>
            ))}
          </select>
        ) : (
          <input
            type="text"
            value={project}
            onChange={e => setProject(e.target.value)}
            placeholder="Proyecto (opcional)"
            style={{ ...inputStyle, flex: '1 1 140px' }}
            disabled={adding}
          />
        )}
        <select
          aria-label="Categoría"
          value={category}
          onChange={e => setCategory(e.target.value)}
          style={{ ...inputStyle, flex: '0 1 140px' }}
          disabled={adding}
        >
          <option value="">Sin categoría</option>
          {(catalog?.categories ?? []).map(c => (
            <option key={c.id} value={c.id}>{c.name}</option>
          ))}
        </select>
        <button
          className="btn btn-primary btn-sm"
          onClick={handleAdd}
          disabled={adding || !org.trim() || !workItemId.trim() || !label.trim()}
          aria-label="Agregar actividad de Azure"
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
        {activities.length === 0 ? (
          <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', padding: '8px 0' }}>
            Sin registros
          </div>
        ) : (
          activities.map(a => (
            <div
              key={a.id}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                padding: '6px 10px',
                background: 'var(--bg-sunken)',
                borderRadius: 'var(--radius-md)',
                border: '1px solid var(--border)',
                opacity: a.is_active ? 1 : 0.55,
              }}
            >
              {editingId === a.id ? (
                <>
                  <input
                    type="text"
                    value={editOrg}
                    onChange={e => setEditOrg(e.target.value)}
                    style={{ ...inputStyle, flex: '1 1 120px' }}
                    disabled={busyId === a.id}
                  />
                  <input
                    type="text"
                    value={editLabel}
                    onChange={e => setEditLabel(e.target.value)}
                    style={{ ...inputStyle, flex: '1 1 160px' }}
                    disabled={busyId === a.id}
                  />
                  <select
                    aria-label="Tipo de work item"
                    value={editWorkItemType}
                    onChange={e => setEditWorkItemType(e.target.value)}
                    style={{ ...inputStyle, flex: '0 1 120px' }}
                    disabled={busyId === a.id}
                  >
                    <option value="">Sin tipo</option>
                    <option value="Bug">Bug</option>
                    <option value="Task">Task</option>
                  </select>
                  {catalog !== null ? (
                    <select
                      aria-label="Proyecto"
                      value={editProject}
                      onChange={e => setEditProject(e.target.value)}
                      style={{ ...inputStyle, flex: '0 1 140px' }}
                      disabled={busyId === a.id}
                    >
                      <option value="">Sin proyecto</option>
                      {catalog.projects.map(p => (
                        <option key={p.name} value={p.name}>{p.name}</option>
                      ))}
                      {editProject && !catalog.projects.some(p => p.name === editProject) && (
                        <option value={editProject}>{editProject}</option>
                      )}
                    </select>
                  ) : (
                    <input
                      type="text"
                      value={editProject}
                      onChange={e => setEditProject(e.target.value)}
                      placeholder="Proyecto (opcional)"
                      style={{ ...inputStyle, flex: '1 1 140px' }}
                      disabled={busyId === a.id}
                    />
                  )}
                  <select
                    aria-label="Categoría"
                    value={editCategory}
                    onChange={e => setEditCategory(e.target.value)}
                    style={{ ...inputStyle, flex: '0 1 140px' }}
                    disabled={busyId === a.id}
                  >
                    <option value="">Sin categoría</option>
                    {(catalog?.categories ?? []).map(c => (
                      <option key={c.id} value={c.id}>{c.name}</option>
                    ))}
                    {/* Existence-only validation on the backend: a stored
                        category_id can dangle once a category is hard-deleted.
                        Keep it selectable/visible instead of silently
                        collapsing it to "Sin categoría" and clearing it on save. */}
                    {editCategory && !(catalog?.categories ?? []).some(c => String(c.id) === editCategory) && (
                      <option value={editCategory}>{`Categoría desconocida (#${editCategory})`}</option>
                    )}
                  </select>
                  <button
                    className="btn btn-ghost btn-sm"
                    onClick={() => saveEdit(a.id)}
                    disabled={busyId === a.id}
                    title="Guardar"
                    aria-label={`Guardar ${a.label}`}
                    style={{ padding: '2px 4px' }}
                  >
                    <Check size={13} strokeWidth={1.75} />
                  </button>
                </>
              ) : (
                <>
                  <span style={{ flex: 1, font: 'var(--text-body)', color: 'var(--fg1)', fontSize: '0.9em' }}>
                    {a.label}{' '}
                    <span style={{ color: 'var(--fg3)' }}>
                      — {a.org} / <AzureWorkItemLink activity={a}>#{a.work_item_id}</AzureWorkItemLink>
                    </span>{' '}
                    {a.work_item_type && (
                      <span className="chip" style={{ fontSize: '0.7em' }}>{a.work_item_type}</span>
                    )}
                    {!a.is_active && (
                      <span className="chip chip-todo" style={{ fontSize: '0.7em' }}>Inactivo</span>
                    )}
                  </span>
                  {a.is_default ? (
                    <span className="chip chip-done" style={{ fontSize: '0.75em' }}>Predeterminada</span>
                  ) : (
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => handleSetDefault(a.id)}
                      disabled={busyId === a.id}
                      title="Definir como predeterminada"
                      aria-label={`Definir ${a.label} como predeterminada`}
                      style={{ padding: '2px 4px' }}
                    >
                      <Star size={13} strokeWidth={1.75} />
                    </button>
                  )}
                  <button
                    className="btn btn-ghost btn-sm"
                    onClick={() => startEdit(a)}
                    disabled={busyId === a.id}
                    title="Editar"
                    aria-label={`Editar ${a.label}`}
                    style={{ padding: '2px 4px' }}
                  >
                    <Pencil size={13} strokeWidth={1.75} />
                  </button>
                  <button
                    className="btn btn-ghost btn-sm"
                    onClick={() => handleDeactivate(a.id)}
                    disabled={busyId === a.id || a.is_default || !a.is_active}
                    title={!a.is_active ? 'Ya está inactiva' : a.is_default ? 'Primero define otra actividad como predeterminada' : 'Desactivar'}
                    aria-label={`Desactivar ${a.label}`}
                    style={{ padding: '2px 4px' }}
                  >
                    <Trash2 size={13} strokeWidth={1.75} />
                  </button>
                </>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  )
}

type CatalogTab = 'projects' | 'categories' | 'azure'

const TABS: { id: CatalogTab; label: string }[] = [
  { id: 'projects', label: 'Proyectos' },
  { id: 'categories', label: 'Categorías' },
  { id: 'azure', label: 'Work items de Azure' },
]

export function CatalogManagementModal({ open, onClose, catalog, onCatalogChanged, azureActivities, onAzureActivitiesChanged }: CatalogManagementModalProps) {
  const pressedOnOverlay = useRef(false)
  const [activeTab, setActiveTab] = useState<CatalogTab>('projects')

  // Both lists the parent gives us (`catalog.projects` / `azureActivities`)
  // are fetched active-only, since that's what every other consumer (the
  // new/edit activity forms, the add-Azure-activity row) needs. "Mostrar
  // inactivos" pulls a separate include_inactive=true snapshot on demand
  // instead of widening the parent's fetch, so those other consumers never
  // see deactivated entries. Re-fetching whenever the parent's active-only
  // data changes (via the catalog/azureActivities deps below) keeps the
  // inactive-augmented view in sync after every add/remove/deactivate.
  const [showInactiveProjects, setShowInactiveProjects] = useState(false)
  const [projectsWithInactive, setProjectsWithInactive] = useState<CatalogProject[] | null>(null)
  const [showInactiveAzure, setShowInactiveAzure] = useState(false)
  const [azureWithInactive, setAzureWithInactive] = useState<AzureActivity[] | null>(null)

  useEffect(() => {
    if (!open || !showInactiveProjects) return
    getActivityCatalog(true).then(c => setProjectsWithInactive(c.projects)).catch(() => setProjectsWithInactive([]))
  }, [open, showInactiveProjects, catalog])

  useEffect(() => {
    if (!open || !showInactiveAzure) return
    listAzureActivities(true).then(setAzureWithInactive).catch(() => setAzureWithInactive([]))
  }, [open, showInactiveAzure, azureActivities])

  if (!open) return null

  const visibleProjects = showInactiveProjects ? (projectsWithInactive ?? catalog?.projects ?? []) : (catalog?.projects ?? [])
  const visibleAzureActivities = showInactiveAzure ? (azureWithInactive ?? azureActivities) : azureActivities

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
          {activeTab === 'azure' && (
            <AzureActivitySection
              activities={visibleAzureActivities}
              catalog={catalog}
              onChanged={onAzureActivitiesChanged}
              showInactive={showInactiveAzure}
              onShowInactiveChange={setShowInactiveAzure}
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
