import { useState } from 'react'
import { Trash2, Plus, CheckCircle } from 'lucide-react'
import type { DWTestScenario, DWState } from '../../types'
import { addDWTestScenario, removeDWTestScenario, signOffScenario } from '../../api/client'

interface DWScenariosSectionProps {
  dwId: number
  state: DWState
  scenarios: DWTestScenario[]
  onRefresh: () => void
}

const RESULT_META: Record<string, { label: string; cls: string }> = {
  pending: { label: 'Pendiente', cls: 'chip-pending' },
  pass:    { label: 'Pasó',      cls: 'chip-done' },
  fail:    { label: 'Falló',     cls: 'chip-block' },
}

export function DWScenariosSection({ dwId, state, scenarios, onRefresh }: DWScenariosSectionProps) {
  const canModify = state !== 'deployed'
  const canSignOff = state === 'submitted' || state === 'approved'

  const [titleInput, setTitleInput] = useState('')
  const [descInput, setDescInput] = useState('')
  const [expectedInput, setExpectedInput] = useState('')
  const [adding, setAdding] = useState(false)
  const [addError, setAddError] = useState<string | null>(null)

  // Per-scenario sign-off state
  const [signoffState, setSignoffState] = useState<Record<number, { result: string; name: string }>>({})
  const [signoffErrors, setSignoffErrors] = useState<Record<number, string>>({})

  const sorted = [...scenarios].sort((a, b) => a.sort_order - b.sort_order)

  async function handleAdd() {
    if (!titleInput.trim()) { setAddError('El título es requerido'); return }
    setAdding(true)
    setAddError(null)
    try {
      await addDWTestScenario(dwId, {
        title: titleInput.trim(),
        description: descInput.trim(),
        expected: expectedInput.trim(),
        sort_order: scenarios.length + 1,
      })
      setTitleInput('')
      setDescInput('')
      setExpectedInput('')
      onRefresh()
    } catch (e) {
      setAddError(e instanceof Error ? e.message : 'Error al agregar escenario')
    } finally {
      setAdding(false)
    }
  }

  async function handleRemove(scenarioId: number) {
    try {
      await removeDWTestScenario(dwId, scenarioId)
      onRefresh()
    } catch (e) {
      setSignoffErrors(prev => ({ ...prev, [scenarioId]: e instanceof Error ? e.message : 'Error al eliminar' }))
    }
  }

  function updateSignoff(id: number, field: 'result' | 'name', value: string) {
    setSignoffState(prev => ({
      ...prev,
      [id]: { result: prev[id]?.result ?? 'pass', name: prev[id]?.name ?? '', [field]: value },
    }))
  }

  async function handleSignOff(scenarioId: number) {
    const s = signoffState[scenarioId] ?? { result: 'pass', name: '' }
    if (!s.name.trim()) {
      setSignoffErrors(prev => ({ ...prev, [scenarioId]: 'El nombre del firmante es requerido' }))
      return
    }
    setSignoffErrors(prev => { const n = { ...prev }; delete n[scenarioId]; return n })
    try {
      await signOffScenario(dwId, scenarioId, s.result, s.name.trim())
      onRefresh()
    } catch (e) {
      setSignoffErrors(prev => ({ ...prev, [scenarioId]: e instanceof Error ? e.message : 'Error al firmar' }))
    }
  }

  return (
    <div>
      <div style={{ font: 'var(--text-h4)', color: 'var(--fg1)', marginBottom: 10, fontWeight: 600 }}>
        Escenarios de prueba
      </div>

      {sorted.length === 0 ? (
        <div style={{ color: 'var(--fg3)', font: 'var(--text-body)', marginBottom: 12 }}>
          Sin escenarios de prueba.
        </div>
      ) : (
        <div style={{ marginBottom: 12, display: 'flex', flexDirection: 'column', gap: 10 }}>
          {sorted.map((sc, idx) => {
            const resultMeta = RESULT_META[sc.result] ?? RESULT_META.pending
            const so = signoffState[sc.id] ?? { result: 'pass', name: '' }
            return (
              <div
                key={sc.id}
                className="card"
                style={{ padding: '14px 16px' }}
              >
                <div style={{ display: 'flex', alignItems: 'flex-start', gap: 10, marginBottom: 6 }}>
                  <span style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)', paddingTop: 3, minWidth: 20 }}>
                    {idx + 1}.
                  </span>
                  <div style={{ flex: 1 }}>
                    <span style={{ font: 'var(--text-h4)', color: 'var(--fg1)', fontWeight: 600 }}>{sc.title}</span>
                    {sc.description && (
                      <div style={{ font: 'var(--text-body)', color: 'var(--fg2)', marginTop: 2 }}>{sc.description}</div>
                    )}
                    {sc.expected && (
                      <div style={{ font: 'var(--text-sm)', color: 'var(--fg3)', marginTop: 4 }}>
                        <span style={{ font: 'var(--text-label)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)', marginRight: 6 }}>Esperado:</span>
                        {sc.expected}
                      </div>
                    )}
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span className={`chip ${resultMeta.cls}`}>{resultMeta.label}</span>
                    {canModify && (
                      <button
                        className="btn btn-ghost btn-sm"
                        onClick={() => handleRemove(sc.id)}
                        aria-label="Eliminar escenario"
                      >
                        <Trash2 size={14} strokeWidth={1.75} />
                      </button>
                    )}
                  </div>
                </div>

                {sc.signed_off_by && (
                  <div style={{ font: 'var(--text-caption)', color: 'var(--fg3)', marginTop: 4, paddingLeft: 30 }}>
                    Firmado por: {sc.signed_off_by}
                  </div>
                )}

                {canSignOff && !sc.signed_off_by && (
                  <div style={{ paddingLeft: 30, marginTop: 8, display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
                    <select
                      value={so.result}
                      onChange={e => updateSignoff(sc.id, 'result', e.target.value)}
                      style={{
                        padding: '5px 8px',
                        border: '1px solid var(--border-strong)',
                        borderRadius: 'var(--radius-md)',
                        font: 'var(--text-sm)',
                        color: 'var(--fg1)',
                        background: 'var(--bg-sunken)',
                      }}
                    >
                      <option value="pass">Pasó</option>
                      <option value="fail">Falló</option>
                    </select>
                    <input
                      type="text"
                      value={so.name}
                      onChange={e => updateSignoff(sc.id, 'name', e.target.value)}
                      placeholder="Nombre del firmante"
                      style={{
                        padding: '5px 8px',
                        border: '1px solid var(--border-strong)',
                        borderRadius: 'var(--radius-md)',
                        font: 'var(--text-sm)',
                        color: 'var(--fg1)',
                        background: 'var(--bg-sunken)',
                        flex: 1,
                        minWidth: 120,
                      }}
                    />
                    <button
                      className="btn btn-secondary btn-sm"
                      onClick={() => handleSignOff(sc.id)}
                    >
                      <CheckCircle size={13} strokeWidth={1.75} />
                      Firmar
                    </button>
                  </div>
                )}

                {signoffErrors[sc.id] && (
                  <div style={{ paddingLeft: 30, marginTop: 4, color: 'var(--block-solid)', font: 'var(--text-caption)' }}>
                    {signoffErrors[sc.id]}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {canModify && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, padding: 14, background: 'var(--bg-sunken)', borderRadius: 'var(--radius-md)', border: '1px solid var(--border)' }}>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flex: 2, minWidth: 160 }}>
              <label style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>Título *</label>
              <input
                type="text"
                value={titleInput}
                onChange={e => setTitleInput(e.target.value)}
                placeholder="Login con credenciales válidas"
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
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flex: 2, minWidth: 160 }}>
              <label style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>Descripción</label>
              <input
                type="text"
                value={descInput}
                onChange={e => setDescInput(e.target.value)}
                placeholder="Verificar que el usuario puede autenticarse..."
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
            <label style={{ font: 'var(--text-label)', color: 'var(--fg3)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)' }}>Resultado esperado</label>
            <input
              type="text"
              value={expectedInput}
              onChange={e => setExpectedInput(e.target.value)}
              placeholder="El sistema redirige al dashboard..."
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
          <div>
            <button
              className="btn btn-secondary btn-sm"
              onClick={handleAdd}
              disabled={adding || !titleInput.trim()}
            >
              <Plus size={14} strokeWidth={1.75} />
              Agregar escenario
            </button>
          </div>
          {addError && (
            <div style={{ color: 'var(--block-solid)', font: 'var(--text-caption)' }}>{addError}</div>
          )}
        </div>
      )}
    </div>
  )
}
