import { useState, useEffect, useCallback } from 'react'
import { Send, CheckCircle, Rocket, RotateCcw, RefreshCw } from 'lucide-react'
import type { DeploymentWindowDetail as DWDetail } from '../../types'
import { getDeploymentWindow, updateDWState } from '../../api/client'
import { DWStateBadge } from './DWStateBadge'
import { DWTasksSection } from './DWTasksSection'
import { DWReposSection } from './DWReposSection'
import { DWArtifactsSection } from './DWArtifactsSection'
import { DWScenariosSection } from './DWScenariosSection'
import { ExportButton } from './ExportButton'

interface DeploymentWindowDetailProps {
  dwId: number
}

export function DeploymentWindowDetailPanel({ dwId }: DeploymentWindowDetailProps) {
  const [dw, setDw] = useState<DWDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [transitioning, setTransitioning] = useState(false)
  const [transitionError, setTransitionError] = useState<string | null>(null)
  const [showRejectForm, setShowRejectForm] = useState(false)
  const [rejectionNote, setRejectionNote] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await getDeploymentWindow(dwId)
      setDw(data)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Error al cargar ventana')
    } finally {
      setLoading(false)
    }
  }, [dwId])

  useEffect(() => { load() }, [load])

  async function handleTransition(toState: string, note?: string) {
    setTransitioning(true)
    setTransitionError(null)
    try {
      await updateDWState(dwId, toState, note)
      setShowRejectForm(false)
      setRejectionNote('')
      await load()
    } catch (e) {
      setTransitionError(e instanceof Error ? e.message : 'Error al cambiar estado')
    } finally {
      setTransitioning(false)
    }
  }

  if (loading) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--fg3)', gap: 10 }}>
        <RefreshCw size={20} strokeWidth={1.75} style={{ animation: 'spin 1s linear infinite' }} />
        <span style={{ font: 'var(--text-body)' }}>Cargando...</span>
      </div>
    )
  }

  if (error) {
    return (
      <div style={{ flex: 1, padding: 24 }}>
        <div style={{ color: 'var(--block-solid)', font: 'var(--text-body)', marginBottom: 12 }}>{error}</div>
        <button className="btn btn-secondary btn-sm" onClick={load}>Reintentar</button>
      </div>
    )
  }

  if (!dw) return null

  const divider = (
    <div style={{ borderTop: '1px solid var(--border)', margin: '20px 0' }} />
  )

  return (
    <div style={{ flex: 1, overflowY: 'auto', padding: '20px 24px', display: 'flex', flexDirection: 'column', gap: 0 }}>

      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12, marginBottom: 8 }}>
        <div style={{ flex: 1 }}>
          <h2 style={{ font: 'var(--text-h2)', color: 'var(--fg1)', margin: 0, marginBottom: 6 }}>{dw.title}</h2>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
            <DWStateBadge state={dw.state} />
            {dw.planned_at && (
              <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
                Planificada: {dw.planned_at}
              </span>
            )}
            {dw.created_by && (
              <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
                Por: {dw.created_by}
              </span>
            )}
            {dw.deployed_at && (
              <span style={{ font: 'var(--text-caption)', color: 'var(--done-fg)' }}>
                Desplegada: {dw.deployed_at}
              </span>
            )}
          </div>
        </div>
        <ExportButton dwId={dw.id} title={dw.title} />
      </div>

      {dw.description && (
        <p style={{ font: 'var(--text-body)', color: 'var(--fg2)', margin: '0 0 8px' }}>{dw.description}</p>
      )}

      {dw.rejection_note && (
        <div style={{
          padding: '10px 14px',
          background: 'var(--block-bg)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--block-solid)',
          marginBottom: 8,
        }}>
          <div style={{ font: 'var(--text-label)', color: 'var(--block-fg)', textTransform: 'uppercase', letterSpacing: 'var(--tracking-label)', marginBottom: 4 }}>
            Nota de rechazo
          </div>
          <div style={{ font: 'var(--text-body)', color: 'var(--block-fg)' }}>{dw.rejection_note}</div>
        </div>
      )}

      {/* State transition buttons */}
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap', marginBottom: 4 }}>
        {dw.state === 'draft' && (
          <button
            className="btn btn-primary btn-sm"
            onClick={() => handleTransition('submitted')}
            disabled={transitioning}
          >
            <Send size={14} strokeWidth={1.75} />
            Enviar a Interventoría
          </button>
        )}
        {dw.state === 'submitted' && (
          <>
            <button
              className="btn btn-secondary btn-sm"
              onClick={() => handleTransition('approved')}
              disabled={transitioning}
              style={{ borderColor: 'var(--done-solid)', color: 'var(--done-fg)' }}
            >
              <CheckCircle size={14} strokeWidth={1.75} />
              Aprobar
            </button>
            {!showRejectForm ? (
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => setShowRejectForm(true)}
                disabled={transitioning}
              >
                <RotateCcw size={14} strokeWidth={1.75} />
                Rechazar
              </button>
            ) : (
              <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
                <input
                  type="text"
                  value={rejectionNote}
                  onChange={e => setRejectionNote(e.target.value)}
                  placeholder="Motivo del rechazo..."
                  style={{
                    padding: '6px 10px',
                    border: '1px solid var(--border-strong)',
                    borderRadius: 'var(--radius-md)',
                    font: 'var(--text-sm)',
                    color: 'var(--fg1)',
                    background: 'var(--bg-sunken)',
                    minWidth: 200,
                  }}
                />
                <button
                  className="btn btn-danger btn-sm"
                  onClick={() => handleTransition('draft', rejectionNote)}
                  disabled={transitioning || !rejectionNote.trim()}
                >
                  Confirmar rechazo
                </button>
                <button className="btn btn-ghost btn-sm" onClick={() => { setShowRejectForm(false); setRejectionNote('') }}>
                  Cancelar
                </button>
              </div>
            )}
          </>
        )}
        {dw.state === 'approved' && (
          <button
            className="btn btn-primary btn-sm"
            onClick={() => handleTransition('deployed')}
            disabled={transitioning}
          >
            <Rocket size={14} strokeWidth={1.75} />
            Marcar Desplegado
          </button>
        )}
      </div>

      {transitionError && (
        <div style={{ color: 'var(--block-solid)', font: 'var(--text-caption)', marginBottom: 8 }}>{transitionError}</div>
      )}

      {divider}

      {/* Tasks section */}
      <DWTasksSection dwId={dw.id} state={dw.state} tasks={dw.tasks ?? []} onRefresh={load} />
      {divider}

      {/* Repos section */}
      <DWReposSection dwId={dw.id} state={dw.state} repos={dw.repos ?? []} onRefresh={load} />
      {divider}

      {/* Artifacts section */}
      <DWArtifactsSection dwId={dw.id} state={dw.state} artifacts={dw.artifacts ?? []} onRefresh={load} />
      {divider}

      {/* Scenarios section */}
      <DWScenariosSection dwId={dw.id} state={dw.state} scenarios={dw.test_scenarios ?? []} onRefresh={load} />
    </div>
  )
}
