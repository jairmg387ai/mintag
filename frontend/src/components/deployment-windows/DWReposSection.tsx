import { useState } from 'react'
import { Trash2, Plus } from 'lucide-react'
import type { DWRepo, DWState } from '../../types'
import { addDWRepo, removeDWRepo } from '../../api/client'

interface DWReposSectionProps {
  dwId: number
  state: DWState
  repos: DWRepo[]
  onRefresh: () => void
}

export function DWReposSection({ dwId, state, repos, onRefresh }: DWReposSectionProps) {
  const isDraft = state === 'draft'
  const [keyInput, setKeyInput] = useState('')
  const [versionInput, setVersionInput] = useState('')
  const [notesInput, setNotesInput] = useState('')
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleAdd() {
    if (!keyInput.trim()) { setError('La clave del repositorio es requerida'); return }
    if (!versionInput.trim()) { setError('La versión es requerida'); return }
    setAdding(true)
    setError(null)
    try {
      await addDWRepo(dwId, { graph_node_key: keyInput.trim(), version: versionInput.trim(), notes: notesInput.trim() })
      setKeyInput('')
      setVersionInput('')
      setNotesInput('')
      onRefresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Error al agregar repositorio')
    } finally {
      setAdding(false)
    }
  }

  async function handleRemove(repoId: number) {
    setError(null)
    try {
      await removeDWRepo(dwId, repoId)
      onRefresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Error al eliminar repositorio')
    }
  }

  return (
    <div>
      <div style={{ font: 'var(--text-h4)', color: 'var(--fg1)', marginBottom: 10, fontWeight: 600 }}>
        Repositorios
      </div>

      {repos.length === 0 ? (
        <div style={{ color: 'var(--fg3)', font: 'var(--text-body)', marginBottom: 12 }}>
          Sin repositorios referenciados.
        </div>
      ) : (
        <table className="mt-table" style={{ marginBottom: 12 }}>
          <thead>
            <tr>
              <th>REPOSITORIO</th>
              <th>VERSIÓN</th>
              <th>NOTAS</th>
              {isDraft && <th></th>}
            </tr>
          </thead>
          <tbody>
            {repos.map(r => (
              <tr key={r.id}>
                <td style={{ font: 'var(--text-mono)', color: 'var(--fg1)' }}>{r.graph_node_key}</td>
                <td style={{ font: 'var(--text-mono)', color: 'var(--info-fg)' }}>{r.version}</td>
                <td style={{ color: 'var(--fg2)' }}>{r.notes}</td>
                {isDraft && (
                  <td style={{ width: 40 }}>
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => handleRemove(r.id)}
                      aria-label="Eliminar repositorio"
                    >
                      <Trash2 size={14} strokeWidth={1.75} />
                    </button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {isDraft && (
        <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end', flexWrap: 'wrap' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flex: 2, minWidth: 140 }}>
            <label style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>
              Clave de nodo
            </label>
            <input
              type="text"
              value={keyInput}
              onChange={e => setKeyInput(e.target.value)}
              placeholder="repo:mi-servicio"
              style={{
                padding: '7px 10px',
                border: '1px solid var(--border-strong)',
                borderRadius: 'var(--radius-md)',
                font: 'var(--text-body)',
                color: 'var(--fg1)',
                background: 'var(--bg-sunken)',
              }}
            />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4, minWidth: 100 }}>
            <label style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>
              Versión *
            </label>
            <input
              type="text"
              value={versionInput}
              onChange={e => setVersionInput(e.target.value)}
              placeholder="v1.2.3"
              style={{
                padding: '7px 10px',
                border: '1px solid var(--border-strong)',
                borderRadius: 'var(--radius-md)',
                font: 'var(--text-body)',
                color: 'var(--fg1)',
                background: 'var(--bg-sunken)',
              }}
            />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flex: 2, minWidth: 120 }}>
            <label style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>
              Notas
            </label>
            <input
              type="text"
              value={notesInput}
              onChange={e => setNotesInput(e.target.value)}
              placeholder="Cambios incluidos..."
              style={{
                padding: '7px 10px',
                border: '1px solid var(--border-strong)',
                borderRadius: 'var(--radius-md)',
                font: 'var(--text-body)',
                color: 'var(--fg1)',
                background: 'var(--bg-sunken)',
              }}
            />
          </div>
          <button
            className="btn btn-secondary btn-sm"
            onClick={handleAdd}
            disabled={adding || !keyInput.trim() || !versionInput.trim()}
          >
            <Plus size={14} strokeWidth={1.75} />
            Agregar
          </button>
        </div>
      )}

      {error && (
        <div style={{ marginTop: 8, color: 'var(--block-solid)', font: 'var(--text-caption)' }}>{error}</div>
      )}
    </div>
  )
}
