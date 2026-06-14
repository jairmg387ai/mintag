import { useState, useEffect, useCallback } from 'react'
import { Plus, RefreshCw } from 'lucide-react'
import type { DeploymentWindow, DWState } from '../../types'
import { listDeploymentWindows, createDeploymentWindow } from '../../api/client'
import { DWStateBadge } from './DWStateBadge'
import { DeploymentWindowDetailPanel } from './DeploymentWindowDetail'

type FilterState = DWState | 'all'

const FILTER_TABS: { value: FilterState; label: string }[] = [
  { value: 'all',       label: 'Todas' },
  { value: 'draft',     label: 'Borrador' },
  { value: 'submitted', label: 'En revisión' },
  { value: 'approved',  label: 'Aprobadas' },
  { value: 'deployed',  label: 'Desplegadas' },
]

export function DeploymentWindowsView() {
  const [windows, setWindows] = useState<DeploymentWindow[]>([])
  const [filter, setFilter] = useState<FilterState>('all')
  const [loading, setLoading] = useState(false)
  const [fetchError, setFetchError] = useState<string | null>(null)
  const [selectedId, setSelectedId] = useState<number | null>(null)

  // Create form
  const [showCreate, setShowCreate] = useState(false)
  const [createTitle, setCreateTitle] = useState('')
  const [createDesc, setCreateDesc] = useState('')
  const [createBy, setCreateBy] = useState('')
  const [createAt, setCreateAt] = useState('')
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

  const loadWindows = useCallback(async () => {
    setLoading(true)
    setFetchError(null)
    try {
      const data = await listDeploymentWindows(filter === 'all' ? undefined : filter)
      setWindows(data)
    } catch (e) {
      setFetchError(e instanceof Error ? e.message : 'Error al cargar ventanas')
    } finally {
      setLoading(false)
    }
  }, [filter])

  useEffect(() => { loadWindows() }, [loadWindows])

  // If selected window is no longer in list after filter change, deselect
  useEffect(() => {
    if (selectedId !== null && !windows.find(w => w.id === selectedId)) {
      setSelectedId(null)
    }
  }, [windows, selectedId])

  async function handleCreate() {
    if (!createTitle.trim()) { setCreateError('El título es requerido'); return }
    setCreating(true)
    setCreateError(null)
    try {
      const created = await createDeploymentWindow({
        title: createTitle.trim(),
        description: createDesc.trim(),
        created_by: createBy.trim(),
        planned_at: createAt.trim(),
      })
      setShowCreate(false)
      setCreateTitle('')
      setCreateDesc('')
      setCreateBy('')
      setCreateAt('')
      await loadWindows()
      setSelectedId(created.id)
    } catch (e) {
      setCreateError(e instanceof Error ? e.message : 'Error al crear ventana')
    } finally {
      setCreating(false)
    }
  }

  const selected = windows.find(w => w.id === selectedId) ?? null

  return (
    <div style={{ display: 'flex', height: '100%', overflow: 'hidden' }}>
      {/* Left panel — list */}
      <div
        style={{
          width: 320,
          flexShrink: 0,
          display: 'flex',
          flexDirection: 'column',
          borderRight: '1px solid var(--border)',
          background: 'var(--bg-surface)',
          overflow: 'hidden',
        }}
      >
        {/* List header */}
        <div style={{ padding: '16px 16px 10px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
            <span style={{ font: 'var(--text-h3)', color: 'var(--fg1)', flex: 1 }}>Ventanas</span>
            <button
              className="btn btn-primary btn-sm"
              onClick={() => setShowCreate(v => !v)}
            >
              <Plus size={14} strokeWidth={1.75} />
              Nueva
            </button>
          </div>

          {/* Filter tabs */}
          <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
            {FILTER_TABS.map(tab => (
              <button
                key={tab.value}
                onClick={() => setFilter(tab.value)}
                style={{
                  padding: '4px 10px',
                  borderRadius: 'var(--radius-full)',
                  border: 'none',
                  font: 'var(--text-label)',
                  fontWeight: 600,
                  cursor: 'pointer',
                  background: filter === tab.value ? 'var(--brand)' : 'var(--bg-sunken)',
                  color: filter === tab.value ? '#fff' : 'var(--fg2)',
                  transition: 'background .15s',
                }}
              >
                {tab.label}
              </button>
            ))}
          </div>
        </div>

        {/* Create form (inline, collapsible) */}
        {showCreate && (
          <div style={{ padding: '14px 16px', borderBottom: '1px solid var(--border)', background: 'var(--bg-sunken)', flexShrink: 0 }}>
            <div style={{ font: 'var(--text-h4)', color: 'var(--fg1)', marginBottom: 10, fontWeight: 600 }}>
              Nueva ventana de mantenimiento
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              <input
                type="text"
                value={createTitle}
                onChange={e => setCreateTitle(e.target.value)}
                placeholder="Título *"
                style={inputStyle}
              />
              <input
                type="text"
                value={createDesc}
                onChange={e => setCreateDesc(e.target.value)}
                placeholder="Descripción"
                style={inputStyle}
              />
              <input
                type="text"
                value={createBy}
                onChange={e => setCreateBy(e.target.value)}
                placeholder="Creado por"
                style={inputStyle}
              />
              <input
                type="text"
                value={createAt}
                onChange={e => setCreateAt(e.target.value)}
                placeholder="Fecha planificada (ej: 2026-07-01)"
                style={inputStyle}
              />
              {createError && (
                <div style={{ color: 'var(--block-solid)', font: 'var(--text-caption)' }}>{createError}</div>
              )}
              <div style={{ display: 'flex', gap: 8 }}>
                <button
                  className="btn btn-primary btn-sm"
                  onClick={handleCreate}
                  disabled={creating || !createTitle.trim()}
                >
                  {creating ? 'Creando...' : 'Crear'}
                </button>
                <button
                  className="btn btn-ghost btn-sm"
                  onClick={() => { setShowCreate(false); setCreateError(null) }}
                >
                  Cancelar
                </button>
              </div>
            </div>
          </div>
        )}

        {/* List body */}
        <div style={{ flex: 1, overflowY: 'auto' }}>
          {loading ? (
            <div style={{ padding: '32px 16px', textAlign: 'center', color: 'var(--fg3)', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8 }}>
              <RefreshCw size={20} strokeWidth={1.75} style={{ animation: 'spin 1s linear infinite' }} />
              <span style={{ font: 'var(--text-body)' }}>Cargando...</span>
            </div>
          ) : fetchError ? (
            <div style={{ padding: 16 }}>
              <div style={{ color: 'var(--block-solid)', font: 'var(--text-body)', marginBottom: 8 }}>{fetchError}</div>
              <button className="btn btn-secondary btn-sm" onClick={loadWindows}>Reintentar</button>
            </div>
          ) : windows.length === 0 ? (
            <div style={{ padding: '32px 16px', textAlign: 'center', color: 'var(--fg3)', font: 'var(--text-body)' }}>
              Sin ventanas de mantenimiento. Creá una con "Nueva".
            </div>
          ) : (
            windows.map(w => (
              <button
                key={w.id}
                onClick={() => setSelectedId(w.id)}
                style={{
                  display: 'block',
                  width: '100%',
                  textAlign: 'left',
                  padding: '12px 16px',
                  borderBottom: '1px solid var(--border)',
                  border: 'none',
                  borderBottomWidth: 1,
                  borderBottomStyle: 'solid',
                  borderBottomColor: 'var(--border)',
                  background: selectedId === w.id ? 'var(--bg-selected)' : 'transparent',
                  cursor: 'pointer',
                  transition: 'background .12s',
                }}
                onMouseEnter={e => { if (selectedId !== w.id) (e.currentTarget as HTMLButtonElement).style.background = 'var(--bg-hover)' }}
                onMouseLeave={e => { (e.currentTarget as HTMLButtonElement).style.background = selectedId === w.id ? 'var(--bg-selected)' : 'transparent' }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                  <span style={{ font: 'var(--text-h4)', color: 'var(--fg1)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {w.title}
                  </span>
                  <DWStateBadge state={w.state} />
                </div>
                {w.planned_at && (
                  <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
                    {w.planned_at}
                  </div>
                )}
              </button>
            ))
          )}
        </div>
      </div>

      {/* Right panel — detail */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', background: 'var(--bg-app)' }}>
        {selected ? (
          <DeploymentWindowDetailPanel key={selected.id} dwId={selected.id} />
        ) : (
          <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--fg3)', font: 'var(--text-body)' }}>
            Seleccioná una ventana para ver el detalle.
          </div>
        )}
      </div>
    </div>
  )
}

const inputStyle: React.CSSProperties = {
  padding: '7px 10px',
  border: '1px solid var(--border-strong)',
  borderRadius: 'var(--radius-md)',
  font: 'var(--text-body)',
  color: 'var(--fg1)',
  background: 'var(--bg-surface)',
  width: '100%',
  boxSizing: 'border-box',
}
