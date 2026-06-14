import { useState } from 'react'
import { Trash2, Plus } from 'lucide-react'
import type { DWTask, DWState } from '../../types'
import { addDWTask, removeDWTask } from '../../api/client'

interface DWTasksSectionProps {
  dwId: number
  state: DWState
  tasks: DWTask[]
  onRefresh: () => void
}

export function DWTasksSection({ dwId, state, tasks, onRefresh }: DWTasksSectionProps) {
  const isDraft = state === 'draft'
  const [taskIdInput, setTaskIdInput] = useState('')
  const [noteInput, setNoteInput] = useState('')
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleAdd() {
    const taskId = parseInt(taskIdInput, 10)
    if (!taskId || isNaN(taskId)) { setError('ID de tarea inválido'); return }
    setAdding(true)
    setError(null)
    try {
      await addDWTask(dwId, taskId, noteInput.trim())
      setTaskIdInput('')
      setNoteInput('')
      onRefresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Error al agregar tarea')
    } finally {
      setAdding(false)
    }
  }

  async function handleRemove(taskId: number) {
    setError(null)
    try {
      await removeDWTask(dwId, taskId)
      onRefresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Error al eliminar tarea')
    }
  }

  return (
    <div>
      <div style={{ font: 'var(--text-h4)', color: 'var(--fg1)', marginBottom: 10, fontWeight: 600 }}>
        Tareas vinculadas
      </div>

      {tasks.length === 0 ? (
        <div style={{ color: 'var(--fg3)', font: 'var(--text-body)', marginBottom: 12 }}>
          Sin tareas vinculadas.
        </div>
      ) : (
        <table className="mt-table" style={{ marginBottom: 12 }}>
          <thead>
            <tr>
              <th>ID</th>
              <th>TÍTULO</th>
              <th>NOTA</th>
              {isDraft && <th></th>}
            </tr>
          </thead>
          <tbody>
            {tasks.map(t => (
              <tr key={t.task_id}>
                <td style={{ color: 'var(--fg3)', font: 'var(--text-mono)', width: 60 }}>#{t.task_id}</td>
                <td style={{ color: 'var(--fg1)' }}>{t.task_title ?? `Tarea #${t.task_id}`}</td>
                <td style={{ color: 'var(--fg2)' }}>{t.note}</td>
                {isDraft && (
                  <td style={{ width: 40 }}>
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => handleRemove(t.task_id)}
                      aria-label="Eliminar tarea"
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
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
            <label style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>
              ID tarea
            </label>
            <input
              type="number"
              value={taskIdInput}
              onChange={e => setTaskIdInput(e.target.value)}
              placeholder="123"
              style={{
                width: 90,
                padding: '7px 10px',
                border: '1px solid var(--border-strong)',
                borderRadius: 'var(--radius-md)',
                font: 'var(--text-body)',
                color: 'var(--fg1)',
                background: 'var(--bg-sunken)',
              }}
            />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flex: 1, minWidth: 140 }}>
            <label style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>
              Nota (opcional)
            </label>
            <input
              type="text"
              value={noteInput}
              onChange={e => setNoteInput(e.target.value)}
              placeholder="Contexto de esta tarea..."
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
            disabled={adding || !taskIdInput}
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
