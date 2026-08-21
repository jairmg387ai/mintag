import { useState, useEffect } from 'react'
import { Plus, FilePlus2 } from 'lucide-react'
import type { ActivityCatalog, CreatedWorkItemResponse } from '../../types'
import { getActivityCatalog } from '../../api/client'
import { useAppActions } from '../../store/AppContext'
import { Card, CardHeader } from '../ui/Card'
import { CreateWorkItemModal } from './CreateWorkItemModal'

// WorkItemsView is a stateless create action against Azure DevOps — mintag
// does not persist work items locally, so there is no list here, only the
// last result of this session's most recent creation.
export function WorkItemsView() {
  const { pushToast } = useAppActions()
  const [modalOpen, setModalOpen] = useState(false)
  const [lastResult, setLastResult] = useState<CreatedWorkItemResponse | null>(null)
  const [catalog, setCatalog] = useState<ActivityCatalog | null>(null)

  useEffect(() => {
    getActivityCatalog().then(setCatalog).catch(() => setCatalog(null))
  }, [])

  function handleCreated(result: CreatedWorkItemResponse) {
    setLastResult(result)
    if (result.activation_error) {
      pushToast(`Work item #${result.id} creado, pero no se pudo activar: ${result.activation_error}`, true)
    } else {
      pushToast(`Work item #${result.id} creado y activado`, false)
    }
    if (result.catalog_error) {
      pushToast(`No se pudo registrar el work item en el catálogo: ${result.catalog_error}`, true)
    }
  }

  return (
    <div className="content-pad">
      <div style={{ display: 'flex', flexDirection: 'column', gap: 20, maxWidth: 640 }}>
        <Card>
          <CardHeader icon={<FilePlus2 size={16} strokeWidth={1.75} />}>Crear Work Item</CardHeader>
          <div style={{ padding: 18 }}>
            <p style={{ font: 'var(--text-body)', color: 'var(--fg2)', marginTop: 0 }}>
              Crea una tarea (Task) en Azure DevOps y la activa automáticamente.
            </p>
            <button onClick={() => setModalOpen(true)} className="btn btn-primary">
              <Plus size={16} strokeWidth={1.75} />
              Crear Work Item
            </button>

            {lastResult && (
              <div
                style={{
                  marginTop: 16,
                  padding: '10px 14px',
                  background: 'var(--bg-sunken)',
                  border: '1px solid var(--border)',
                  borderRadius: 'var(--radius-md)',
                  font: 'var(--text-sm)',
                  color: 'var(--fg1)',
                }}
              >
                Último creado: <strong>#{lastResult.id}</strong> — estado {lastResult.state}
                {lastResult.azure_activity_id && (
                  <div style={{ color: 'var(--fg2)', marginTop: 4 }}>
                    Registrado en el catálogo de actividades.
                  </div>
                )}
                {lastResult.activation_error && (
                  <div style={{ color: 'var(--block-solid)', marginTop: 4 }}>
                    No se pudo activar: {lastResult.activation_error}
                  </div>
                )}
                {lastResult.catalog_error && (
                  <div style={{ color: 'var(--block-solid)', marginTop: 4 }}>
                    No se pudo registrar en el catálogo: {lastResult.catalog_error}
                  </div>
                )}
              </div>
            )}
          </div>
        </Card>
      </div>

      <CreateWorkItemModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onCreated={handleCreated}
        catalog={catalog}
      />
    </div>
  )
}
