import { useState, useEffect, useCallback } from 'react'
import { Plus, FilePlus2, RefreshCw } from 'lucide-react'
import type { ActivityCatalog, AzureActivity, AssignedAzureWorkItem, CreatedWorkItemResponse } from '../../types'
import { getActivityCatalog, listAzureActivities, fetchAzureWorkItemStates } from '../../api/client'
import { useAppActions } from '../../store/AppContext'
import { Card, CardHeader } from '../ui/Card'
import { CreateWorkItemModal } from './CreateWorkItemModal'
import { AzureWorkItemStateBadge } from './AzureWorkItemStateBadge'

// WorkItemsView creates Azure DevOps work items and lists the ones already
// registered in the local azure_activities catalog. mintag does not persist
// work items itself — the table below reflects the catalog (see
// CatalogManagementModal for CRUD on it) plus an opt-in live-state refresh
// against Azure; it is not a full sync/history of every work item ever
// touched.
export function WorkItemsView() {
  const { pushToast } = useAppActions()
  const [modalOpen, setModalOpen] = useState(false)
  const [lastResult, setLastResult] = useState<CreatedWorkItemResponse | null>(null)
  const [catalog, setCatalog] = useState<ActivityCatalog | null>(null)
  const [azureActivities, setAzureActivities] = useState<AzureActivity[]>([])
  const [activitiesLoading, setActivitiesLoading] = useState(true)
  const [liveStates, setLiveStates] = useState<Record<number, AssignedAzureWorkItem>>({})
  const [statesLoading, setStatesLoading] = useState(false)

  const loadAzureActivities = useCallback(() => {
    setActivitiesLoading(true)
    listAzureActivities(false)
      .then(setAzureActivities)
      .catch(() => setAzureActivities([]))
      .finally(() => setActivitiesLoading(false))
  }, [])

  useEffect(() => {
    getActivityCatalog().then(setCatalog).catch(() => setCatalog(null))
    loadAzureActivities()
  }, [loadAzureActivities])

  function categoryName(categoryId: AzureActivity['category_id']): string {
    if (!categoryId || !catalog) return '—'
    return catalog.categories.find(c => c.id === categoryId)?.name ?? '—'
  }

  async function refreshStates() {
    setStatesLoading(true)
    try {
      const ids = azureActivities.map(a => a.work_item_id)
      const { items } = await fetchAzureWorkItemStates(ids)
      const byId: Record<number, AssignedAzureWorkItem> = {}
      for (const item of items) byId[item.id] = item
      setLiveStates(byId)
    } catch (e: unknown) {
      pushToast(e instanceof Error ? e.message : 'No se pudo refrescar el estado de los work items', true)
    } finally {
      setStatesLoading(false)
    }
  }

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
    if (result.azure_activity_id) {
      loadAzureActivities()
    }
  }

  return (
    <div className="content-pad">
      <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
        <Card style={{ maxWidth: 640 }}>
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

        <Card>
          <CardHeader
            icon={<RefreshCw size={16} strokeWidth={1.75} />}
            right={
              <button
                onClick={refreshStates}
                className="btn btn-secondary btn-sm"
                disabled={statesLoading || azureActivities.length === 0}
              >
                {statesLoading ? 'Refrescando...' : 'Refrescar estados'}
              </button>
            }
          >
            Work Items registrados
          </CardHeader>
          <div style={{ padding: 18 }}>
            {activitiesLoading ? (
              <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--fg3)', font: 'var(--text-body)' }}>
                Cargando work items...
              </div>
            ) : azureActivities.length === 0 ? (
              <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--fg3)', font: 'var(--text-body)' }}>
                No hay work items registrados en el catálogo todavía.
              </div>
            ) : (
              <div style={{ overflowX: 'auto' }}>
                <table className="mt-table">
                  <thead>
                    <tr>
                      <th>ID</th>
                      <th>LABEL</th>
                      <th>TIPO</th>
                      <th>PROYECTO</th>
                      <th>CATEGORÍA</th>
                      <th>PREDETERMINADO</th>
                      <th>ESTADO</th>
                    </tr>
                  </thead>
                  <tbody>
                    {azureActivities.map(a => (
                      <tr key={a.id}>
                        <td style={{ font: 'var(--text-mono)', color: 'var(--fg1)' }}>{a.work_item_id}</td>
                        <td style={{ color: 'var(--fg1)' }}>{a.label}</td>
                        <td style={{ color: 'var(--fg2)' }}>{a.work_item_type || '—'}</td>
                        <td style={{ color: 'var(--fg2)' }}>{a.project || '—'}</td>
                        <td style={{ color: 'var(--fg2)' }}>{categoryName(a.category_id)}</td>
                        <td style={{ color: 'var(--fg2)' }}>{a.is_default ? 'Sí' : '—'}</td>
                        <td>
                          {liveStates[a.work_item_id]
                            ? <AzureWorkItemStateBadge state={liveStates[a.work_item_id].state} />
                            : <span style={{ color: 'var(--fg3)' }}>—</span>}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
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
