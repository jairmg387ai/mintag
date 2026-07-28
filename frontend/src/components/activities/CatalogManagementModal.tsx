import { useState, useRef, type CSSProperties } from 'react'
import { X, Trash2, Plus, Star, Pencil, Check } from 'lucide-react'
import type { ActivityCatalog, AzureActivity } from '../../types'
import {
  addCatalogProject,
  removeCatalogProject,
  addCatalogCategory,
  removeCatalogCategory,
  addAzureActivity,
  updateAzureActivity,
  deactivateAzureActivity,
  setDefaultAzureActivity,
} from '../../api/client'
import { friendlyCatalogErrorMessage } from './azureActivity'

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
}: {
  title: string
  items: string[]
  onAdd: (name: string) => Promise<void>
  onRemove: (name: string) => Promise<void>
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
      setError(e instanceof Error ? e.message : 'Error adding entry')
    } finally {
      setAdding(false)
    }
  }

  async function handleRemove(name: string) {
    setRemoving(prev => ({ ...prev, [name]: true }))
    try {
      await onRemove(name)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Error removing entry')
    } finally {
      setRemoving(prev => ({ ...prev, [name]: false }))
    }
  }

  return (
    <div style={{ flex: 1, minWidth: 0 }}>
      <div
        style={{
          font: 'var(--text-h4)',
          fontWeight: 600,
          marginBottom: 12,
          textTransform: 'uppercase',
          letterSpacing: 'var(--tracking-label)',
          fontSize: '0.75em',
          color: 'var(--fg3)',
        } as CSSProperties}
      >
        {title}
      </div>

      {/* Add row */}
      <div style={{ display: 'flex', gap: 6, marginBottom: 10 }}>
        <input
          type="text"
          value={newName}
          onChange={e => setNewName(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') handleAdd() }}
          placeholder={`New ${title.toLowerCase().slice(0, -1)}...`}
          style={{ ...inputStyle, flex: 1 }}
          disabled={adding}
        />
        <button
          className="btn btn-primary btn-sm"
          onClick={handleAdd}
          disabled={adding || !newName.trim()}
          aria-label={`Add ${title}`}
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
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4, maxHeight: 300, overflowY: 'auto' }}>
        {items.length === 0 ? (
          <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', padding: '8px 0' }}>
            No entries
          </div>
        ) : (
          items.map(item => (
            <div
              key={item}
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
                {item}
              </span>
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => handleRemove(item)}
                disabled={removing[item]}
                title={`Remove ${item}`}
                aria-label={`Remove ${item}`}
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

function AzureActivitySection({
  activities,
  onChanged,
}: {
  activities: AzureActivity[]
  onChanged: () => void
}) {
  const [org, setOrg] = useState('')
  const [workItemId, setWorkItemId] = useState('')
  const [label, setLabel] = useState('')
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState('')
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editOrg, setEditOrg] = useState('')
  const [editLabel, setEditLabel] = useState('')
  const [busyId, setBusyId] = useState<number | null>(null)

  async function handleAdd() {
    const wid = parseInt(workItemId, 10)
    if (!org.trim() || !label.trim() || !workItemId.trim() || isNaN(wid)) {
      setError('Org, work item ID, and label are all required')
      return
    }
    setAdding(true)
    setError('')
    try {
      await addAzureActivity({ org: org.trim(), work_item_id: wid, label: label.trim() })
      setOrg('')
      setWorkItemId('')
      setLabel('')
      onChanged()
    } catch (e: unknown) {
      setError(friendlyCatalogErrorMessage(e, 'Error adding activity'))
    } finally {
      setAdding(false)
    }
  }

  function startEdit(a: AzureActivity) {
    setEditingId(a.id)
    setEditOrg(a.org)
    setEditLabel(a.label)
    setError('')
  }

  async function saveEdit(id: number) {
    if (!editOrg.trim() || !editLabel.trim()) {
      setError('Org and label are required')
      return
    }
    setBusyId(id)
    setError('')
    try {
      await updateAzureActivity(id, { org: editOrg.trim(), label: editLabel.trim() })
      setEditingId(null)
      onChanged()
    } catch (e: unknown) {
      setError(friendlyCatalogErrorMessage(e, 'Error updating activity'))
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
      setError(friendlyCatalogErrorMessage(e, 'Error setting default activity'))
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
      setError(friendlyCatalogErrorMessage(e, 'Error deactivating activity'))
    } finally {
      setBusyId(null)
    }
  }

  return (
    <div style={{ marginTop: 24, paddingTop: 20, borderTop: '1px solid var(--border)' }}>
      <div
        style={{
          font: 'var(--text-h4)',
          fontWeight: 600,
          marginBottom: 12,
          textTransform: 'uppercase',
          letterSpacing: 'var(--tracking-label)',
          fontSize: '0.75em',
          color: 'var(--fg3)',
        } as CSSProperties}
      >
        Azure Work Items
      </div>

      {/* Add row */}
      <div style={{ display: 'flex', gap: 6, marginBottom: 10, flexWrap: 'wrap' }}>
        <input
          type="text"
          value={org}
          onChange={e => setOrg(e.target.value)}
          placeholder="Org (e.g. RUNT2PSW)"
          style={{ ...inputStyle, flex: '1 1 140px' }}
          disabled={adding}
        />
        <input
          type="number"
          value={workItemId}
          onChange={e => setWorkItemId(e.target.value)}
          placeholder="Work item ID"
          style={{ ...inputStyle, flex: '1 1 120px' }}
          disabled={adding}
        />
        <input
          type="text"
          value={label}
          onChange={e => setLabel(e.target.value)}
          placeholder="Label"
          style={{ ...inputStyle, flex: '1 1 160px' }}
          disabled={adding}
        />
        <button
          className="btn btn-primary btn-sm"
          onClick={handleAdd}
          disabled={adding || !org.trim() || !workItemId.trim() || !label.trim()}
          aria-label="Add Azure activity"
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
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4, maxHeight: 260, overflowY: 'auto' }}>
        {activities.length === 0 ? (
          <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', padding: '8px 0' }}>
            No entries
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
                  <button
                    className="btn btn-ghost btn-sm"
                    onClick={() => saveEdit(a.id)}
                    disabled={busyId === a.id}
                    title="Save"
                    aria-label={`Save ${a.label}`}
                    style={{ padding: '2px 4px' }}
                  >
                    <Check size={13} strokeWidth={1.75} />
                  </button>
                </>
              ) : (
                <>
                  <span style={{ flex: 1, font: 'var(--text-body)', color: 'var(--fg1)', fontSize: '0.9em' }}>
                    {a.label} <span style={{ color: 'var(--fg3)' }}>— {a.org} / #{a.work_item_id}</span>
                  </span>
                  {a.is_default ? (
                    <span className="chip chip-done" style={{ fontSize: '0.75em' }}>Default</span>
                  ) : (
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => handleSetDefault(a.id)}
                      disabled={busyId === a.id}
                      title="Set as default"
                      aria-label={`Set ${a.label} as default`}
                      style={{ padding: '2px 4px' }}
                    >
                      <Star size={13} strokeWidth={1.75} />
                    </button>
                  )}
                  <button
                    className="btn btn-ghost btn-sm"
                    onClick={() => startEdit(a)}
                    disabled={busyId === a.id}
                    title="Edit"
                    aria-label={`Edit ${a.label}`}
                    style={{ padding: '2px 4px' }}
                  >
                    <Pencil size={13} strokeWidth={1.75} />
                  </button>
                  <button
                    className="btn btn-ghost btn-sm"
                    onClick={() => handleDeactivate(a.id)}
                    disabled={busyId === a.id || a.is_default}
                    title={a.is_default ? 'Promote another activity to default first' : 'Deactivate'}
                    aria-label={`Deactivate ${a.label}`}
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

export function CatalogManagementModal({ open, onClose, catalog, onCatalogChanged, azureActivities, onAzureActivitiesChanged }: CatalogManagementModalProps) {
  const pressedOnOverlay = useRef(false)

  if (!open) return null

  async function handleAddProject(name: string) {
    await addCatalogProject(name)
    onCatalogChanged()
  }

  async function handleRemoveProject(name: string) {
    await removeCatalogProject(name)
    onCatalogChanged()
  }

  async function handleAddCategory(name: string) {
    await addCatalogCategory(name)
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
            Manage Catalog
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
            aria-label="Close"
          >
            <X size={18} strokeWidth={1.75} />
          </button>
        </div>

        {/* Body */}
        <div style={{ padding: '20px 22px', overflowY: 'auto', flex: 1 }}>
          <div style={{ display: 'flex', gap: 24 }}>
            <CatalogSection
              title="Projects"
              items={catalog?.projects ?? []}
              onAdd={handleAddProject}
              onRemove={handleRemoveProject}
            />
            <div style={{ width: 1, background: 'var(--border)', flexShrink: 0 }} />
            <CatalogSection
              title="Categories"
              items={catalog?.categories ?? []}
              onAdd={handleAddCategory}
              onRemove={handleRemoveCategory}
            />
          </div>

          <AzureActivitySection activities={azureActivities} onChanged={onAzureActivitiesChanged} />
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
            Close
          </button>
        </div>
      </div>
    </div>
  )
}
