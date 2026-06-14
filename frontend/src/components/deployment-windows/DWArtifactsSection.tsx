import { useState } from 'react'
import { Trash2, Plus } from 'lucide-react'
import type { DWArtifact, DWState } from '../../types'
import { addDWArtifact, removeDWArtifact } from '../../api/client'

interface DWArtifactsSectionProps {
  dwId: number
  state: DWState
  artifacts: DWArtifact[]
  onRefresh: () => void
}

const KIND_OPTIONS = ['db_script', 'blob', 'config', 'other'] as const
const KIND_LABELS: Record<string, string> = {
  db_script: 'DB Script',
  blob: 'Blob',
  config: 'Config',
  other: 'Otro',
}

type ArtifactKind = typeof KIND_OPTIONS[number]

function groupByKind(artifacts: DWArtifact[]): Record<string, DWArtifact[]> {
  const groups: Record<string, DWArtifact[]> = {}
  for (const a of artifacts) {
    if (!groups[a.kind]) groups[a.kind] = []
    groups[a.kind].push(a)
  }
  return groups
}

export function DWArtifactsSection({ dwId, state, artifacts, onRefresh }: DWArtifactsSectionProps) {
  const isDraft = state === 'draft'
  const [kindInput, setKindInput] = useState<ArtifactKind>('db_script')
  const [nameInput, setNameInput] = useState('')
  const [pathInput, setPathInput] = useState('')
  const [contentInput, setContentInput] = useState('')
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleAdd() {
    if (!nameInput.trim()) { setError('El nombre es requerido'); return }
    setAdding(true)
    setError(null)
    try {
      await addDWArtifact(dwId, {
        kind: kindInput,
        name: nameInput.trim(),
        path: pathInput.trim(),
        content: contentInput.trim(),
      })
      setNameInput('')
      setPathInput('')
      setContentInput('')
      onRefresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Error al agregar artefacto')
    } finally {
      setAdding(false)
    }
  }

  async function handleRemove(artifactId: number) {
    setError(null)
    try {
      await removeDWArtifact(dwId, artifactId)
      onRefresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Error al eliminar artefacto')
    }
  }

  const groups = groupByKind(artifacts)
  const orderedKinds = KIND_OPTIONS.filter(k => groups[k]?.length)

  return (
    <div>
      <div style={{ font: 'var(--text-h4)', color: 'var(--fg1)', marginBottom: 10, fontWeight: 600 }}>
        Artefactos
      </div>

      {artifacts.length === 0 ? (
        <div style={{ color: 'var(--fg3)', font: 'var(--text-body)', marginBottom: 12 }}>
          Sin artefactos agregados.
        </div>
      ) : (
        <div style={{ marginBottom: 12 }}>
          {orderedKinds.map(kind => (
            <div key={kind} style={{ marginBottom: 12 }}>
              <div style={{
                font: 'var(--text-label)',
                color: 'var(--fg3)',
                textTransform: 'uppercase',
                letterSpacing: 'var(--tracking-label)',
                marginBottom: 6,
              }}>
                {KIND_LABELS[kind]}
              </div>
              {groups[kind].map(a => (
                <div
                  key={a.id}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 10,
                    padding: '8px 12px',
                    background: 'var(--bg-sunken)',
                    borderRadius: 'var(--radius-md)',
                    marginBottom: 4,
                    border: '1px solid var(--border)',
                  }}
                >
                  <span style={{ font: 'var(--text-h4)', color: 'var(--fg1)', flex: 1 }}>{a.name}</span>
                  {a.path && (
                    <span style={{ font: 'var(--text-mono)', color: 'var(--fg2)', fontSize: 12 }}>{a.path}</span>
                  )}
                  {isDraft && (
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => handleRemove(a.id)}
                      aria-label="Eliminar artefacto"
                    >
                      <Trash2 size={14} strokeWidth={1.75} />
                    </button>
                  )}
                </div>
              ))}
            </div>
          ))}
        </div>
      )}

      {isDraft && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10, padding: 14, background: 'var(--bg-sunken)', borderRadius: 'var(--radius-md)', border: '1px solid var(--border)' }}>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
              <label style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>Tipo</label>
              <select
                value={kindInput}
                onChange={e => setKindInput(e.target.value as ArtifactKind)}
                style={{
                  padding: '7px 10px',
                  border: '1px solid var(--border-strong)',
                  borderRadius: 'var(--radius-md)',
                  font: 'var(--text-body)',
                  color: 'var(--fg1)',
                  background: 'var(--bg-sunken)',
                }}
              >
                {KIND_OPTIONS.map(k => (
                  <option key={k} value={k}>{KIND_LABELS[k]}</option>
                ))}
              </select>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flex: 2, minWidth: 120 }}>
              <label style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>Nombre *</label>
              <input
                type="text"
                value={nameInput}
                onChange={e => setNameInput(e.target.value)}
                placeholder="001_migrate_users.sql"
                style={{
                  padding: '7px 10px',
                  border: '1px solid var(--border-strong)',
                  borderRadius: 'var(--radius-md)',
                  font: 'var(--text-body)',
                  color: 'var(--fg1)',
                  background: 'var(--bg-surface)',
                }}
              />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flex: 2, minWidth: 120 }}>
              <label style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>Ruta</label>
              <input
                type="text"
                value={pathInput}
                onChange={e => setPathInput(e.target.value)}
                placeholder="migrations/001_migrate_users.sql"
                style={{
                  padding: '7px 10px',
                  border: '1px solid var(--border-strong)',
                  borderRadius: 'var(--radius-md)',
                  font: 'var(--text-body)',
                  color: 'var(--fg1)',
                  background: 'var(--bg-surface)',
                }}
              />
            </div>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
            <label style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>Contenido</label>
            <textarea
              value={contentInput}
              onChange={e => setContentInput(e.target.value)}
              placeholder="Contenido del artefacto..."
              rows={3}
              style={{
                padding: '7px 10px',
                border: '1px solid var(--border-strong)',
                borderRadius: 'var(--radius-md)',
                font: 'var(--text-mono)',
                fontSize: 12,
                color: 'var(--fg1)',
                background: 'var(--bg-surface)',
                resize: 'vertical',
              }}
            />
          </div>
          <div>
            <button
              className="btn btn-secondary btn-sm"
              onClick={handleAdd}
              disabled={adding || !nameInput.trim()}
            >
              <Plus size={14} strokeWidth={1.75} />
              Agregar artefacto
            </button>
          </div>
        </div>
      )}

      {error && (
        <div style={{ marginTop: 8, color: 'var(--block-solid)', font: 'var(--text-caption)' }}>{error}</div>
      )}
    </div>
  )
}
