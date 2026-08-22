import { useState, useEffect, useCallback } from 'react'
import { Plus, FilePlus2, RefreshCw, ExternalLink, UserPlus } from 'lucide-react'
import type { ActivityCatalog, AzureActivity, AssignedAzureWorkItem, CreatedWorkItemResponse } from '../../types'
import {
  getActivityCatalog,
  listAzureActivities,
  fetchAzureWorkItemStates,
  closeAzureWorkItem,
  recreateAzureWorkItem,
  listAssignedAzureWorkItems,
  addAzureActivity,
} from '../../api/client'
import { useAppActions } from '../../store/AppContext'
import { Card, CardHeader } from '../ui/Card'
import { CreateWorkItemModal } from './CreateWorkItemModal'
import { AzureWorkItemStateBadge } from './AzureWorkItemStateBadge'

// isClosedAzureState mirrors azure.IsClosedState (Go) for the subset of
// state strings the frontend needs to reason about — shared by the "hide
// closed" filter and the Close/Recreate row-disable logic below.
function isClosedAzureState(state: string | undefined): boolean {
  return state === 'Closed' || state === 'Cerrado'
}

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
  const [rowBusy, setRowBusy] = useState<Record<number, boolean>>({})
  // org/team_project are only known once a states refresh has run at least
  // once — until then we omit the "open in Azure DevOps" link rather than
  // guess at a fallback org/project.
  const [azureLinkBase, setAzureLinkBase] = useState<{ org: string; teamProject: string } | null>(null)
  const [hideClosed, setHideClosed] = useState(false)
  const [hideBugs, setHideBugs] = useState(false)
  const [hideTasks, setHideTasks] = useState(false)
  const [pendingAssigned, setPendingAssigned] = useState<AssignedAzureWorkItem[] | null>(null)
  const [assignedOrg, setAssignedOrg] = useState('')
  const [syncingAssigned, setSyncingAssigned] = useState(false)
  const [addingWorkItemId, setAddingWorkItemId] = useState<number | null>(null)

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
      const { items, org, team_project } = await fetchAzureWorkItemStates(ids)
      const byId: Record<number, AssignedAzureWorkItem> = {}
      for (const item of items) byId[item.id] = item
      setLiveStates(byId)
      if (org && team_project) setAzureLinkBase({ org, teamProject: team_project })
    } catch (e: unknown) {
      pushToast(e instanceof Error ? e.message : 'No se pudo refrescar el estado de los work items', true)
    } finally {
      setStatesLoading(false)
    }
  }

  // syncAssigned discovers work items assigned to the current identity in
  // Azure that aren't in the local catalog yet, so they can be added with
  // one click instead of typed in by hand (mirrors the coworker reference
  // implementation's syncAssignedWorkItems, but only proposes — the actual
  // catalog write happens per-item via handleAddAssigned below).
  async function syncAssigned() {
    setSyncingAssigned(true)
    try {
      const { org, items } = await listAssignedAzureWorkItems()
      setAssignedOrg(org)
      const knownIds = new Set(azureActivities.map(a => a.work_item_id))
      setPendingAssigned(items.filter(item => !knownIds.has(item.id)))
    } catch (e: unknown) {
      pushToast(e instanceof Error ? e.message : 'No se pudo sincronizar los work items asignados', true)
    } finally {
      setSyncingAssigned(false)
    }
  }

  async function handleAddAssigned(item: AssignedAzureWorkItem) {
    setAddingWorkItemId(item.id)
    try {
      await addAzureActivity({ org: assignedOrg, work_item_id: item.id, label: item.title, work_item_type: item.type })
      setPendingAssigned(prev => (prev ? prev.filter(p => p.id !== item.id) : prev))
      loadAzureActivities()
      pushToast(`Work item ${item.id} agregado al catálogo.`, false)
    } catch (e: unknown) {
      pushToast(e instanceof Error ? e.message : 'No se pudo agregar el work item al catálogo', true)
    } finally {
      setAddingWorkItemId(null)
    }
  }

  // handleClose/handleRecreate are destructive, irreversible Azure mutations
  // (see azure.Client.CloseWorkItem/CreateAndActivateWorkItem doc comments) —
  // both require an explicit confirm, mirroring ActivitiesView's
  // window.confirm convention for the delete action.
  async function handleClose(workItemId: number, label: string) {
    const message = `Se cerrará el work item ${workItemId} (${label}) en Azure DevOps. Antes se sincronizarán Completed Work y Remaining Work con las horas registradas en TimeLog. Esta acción no se puede deshacer.`
    if (!window.confirm(message)) return

    setRowBusy(prev => ({ ...prev, [workItemId]: true }))
    try {
      const result = await closeAzureWorkItem(workItemId)
      setLiveStates(prev => ({
        ...prev,
        [workItemId]: { ...(prev[workItemId] ?? { id: workItemId, title: '', type: '' }), state: result.state },
      }))
      pushToast(
        result.already_closed
          ? `El work item ${workItemId} ya estaba cerrado.`
          : `Work item ${workItemId} cerrado (${result.hours_synced} h sincronizadas).`,
        false,
      )
      if (result.effort_sync_error) {
        pushToast(`No se pudo sincronizar el esfuerzo: ${result.effort_sync_error}`, true)
      }
    } catch (e: unknown) {
      pushToast(e instanceof Error ? e.message : 'No se pudo cerrar el work item', true)
    } finally {
      setRowBusy(prev => ({ ...prev, [workItemId]: false }))
    }
  }

  async function handleRecreate(workItemId: number, label: string) {
    const message = `Se cerrará el work item ${workItemId} (${label}) y se creará uno nuevo con la misma información, ajustando las fechas al mes actual. Antes de cerrarlo se sincronizarán Completed Work y Remaining Work con las horas registradas en TimeLog. Esta acción no se puede deshacer.`
    if (!window.confirm(message)) return

    setRowBusy(prev => ({ ...prev, [workItemId]: true }))
    try {
      const result = await recreateAzureWorkItem(workItemId)
      pushToast(`Work item ${workItemId} cerrado (${result.hours_synced} h sincronizadas) y recreado como #${result.id}.`, false)
      if (result.activation_error) {
        pushToast(`No se pudo activar el nuevo work item: ${result.activation_error}`, true)
      }
      if (result.catalog_error) {
        pushToast(`No se pudo reasignar el catálogo al nuevo work item: ${result.catalog_error}`, true)
      }
      loadAzureActivities()
    } catch (e: unknown) {
      pushToast(e instanceof Error ? e.message : 'No se pudo recrear el work item', true)
    } finally {
      setRowBusy(prev => ({ ...prev, [workItemId]: false }))
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
            {azureActivities.length > 0 && (
              <div style={{ display: 'flex', gap: 16, marginBottom: 14, flexWrap: 'wrap' }}>
                <label style={{ display: 'flex', alignItems: 'center', gap: 6, font: 'var(--text-sm)', color: 'var(--fg2)' }}>
                  <input type="checkbox" checked={hideClosed} onChange={e => setHideClosed(e.target.checked)} />
                  Ocultar cerrados
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: 6, font: 'var(--text-sm)', color: 'var(--fg2)' }}>
                  <input type="checkbox" checked={hideBugs} onChange={e => setHideBugs(e.target.checked)} />
                  Ocultar bugs
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: 6, font: 'var(--text-sm)', color: 'var(--fg2)' }}>
                  <input type="checkbox" checked={hideTasks} onChange={e => setHideTasks(e.target.checked)} />
                  Ocultar tasks
                </label>
              </div>
            )}
            {(() => {
              const visibleActivities = azureActivities.filter(a => {
                if (hideBugs && a.work_item_type === 'Bug') return false
                if (hideTasks && a.work_item_type === 'Task') return false
                // A row whose state hasn't been fetched yet is never hidden
                // by this filter — unknown is not "closed".
                if (hideClosed && isClosedAzureState(liveStates[a.work_item_id]?.state)) return false
                return true
              })

              if (activitiesLoading) {
                return (
                  <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--fg3)', font: 'var(--text-body)' }}>
                    Cargando work items...
                  </div>
                )
              }
              if (azureActivities.length === 0) {
                return (
                  <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--fg3)', font: 'var(--text-body)' }}>
                    No hay work items registrados en el catálogo todavía.
                  </div>
                )
              }
              if (visibleActivities.length === 0) {
                return (
                  <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--fg3)', font: 'var(--text-body)' }}>
                    Ningún work item coincide con los filtros seleccionados.
                  </div>
                )
              }
              return (
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
                        <th>ACCIONES</th>
                      </tr>
                    </thead>
                    <tbody>
                      {visibleActivities.map(a => {
                        // Close/Recreate only apply to Tasks — mirrors the
                        // coworker reference implementation's isBug exclusion;
                        // Bugs have no such action here either.
                        const isTask = a.work_item_type === 'Task'
                        const closedAlready = isClosedAzureState(liveStates[a.work_item_id]?.state)
                        const busy = !!rowBusy[a.work_item_id]
                        return (
                          <tr key={a.id}>
                            <td style={{ font: 'var(--text-mono)', color: 'var(--fg1)' }}>
                              <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                                {a.work_item_id}
                                {azureLinkBase && (
                                  <a
                                    href={`https://dev.azure.com/${azureLinkBase.org}/${azureLinkBase.teamProject}/_workitems/edit/${a.work_item_id}`}
                                    target="_blank"
                                    rel="noreferrer"
                                    title="Abrir en Azure DevOps"
                                    style={{ color: 'var(--fg3)', display: 'inline-flex' }}
                                  >
                                    <ExternalLink size={13} strokeWidth={1.75} />
                                  </a>
                                )}
                              </div>
                            </td>
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
                            <td>
                              {isTask ? (
                                <div style={{ display: 'flex', gap: 6 }}>
                                  <button
                                    className="btn btn-ghost btn-sm"
                                    disabled={busy || closedAlready}
                                    onClick={() => handleClose(a.work_item_id, a.label)}
                                  >
                                    Cerrar
                                  </button>
                                  <button
                                    className="btn btn-ghost btn-sm"
                                    disabled={busy || closedAlready}
                                    onClick={() => handleRecreate(a.work_item_id, a.label)}
                                  >
                                    Recrear
                                  </button>
                                </div>
                              ) : (
                                <span style={{ color: 'var(--fg3)' }}>—</span>
                              )}
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
              )
            })()}
          </div>
        </Card>

        <Card>
          <CardHeader
            icon={<UserPlus size={16} strokeWidth={1.75} />}
            right={
              <button
                onClick={syncAssigned}
                className="btn btn-secondary btn-sm"
                disabled={syncingAssigned}
              >
                {syncingAssigned ? 'Sincronizando...' : 'Sincronizar asignados'}
              </button>
            }
          >
            Asignados en Azure sin catalogar
          </CardHeader>
          <div style={{ padding: 18 }}>
            {pendingAssigned === null ? (
              <div style={{ textAlign: 'center', padding: '12px 0', color: 'var(--fg3)', font: 'var(--text-body)' }}>
                Sincroniza para ver los work items que Azure ya te tiene asignados y que aún no están en tu catálogo.
              </div>
            ) : pendingAssigned.length === 0 ? (
              <div style={{ textAlign: 'center', padding: '12px 0', color: 'var(--fg3)', font: 'var(--text-body)' }}>
                Todo al día: todos tus work items asignados ya están en el catálogo.
              </div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {pendingAssigned.map(item => (
                  <div
                    key={item.id}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 10,
                      padding: '8px 10px',
                      borderRadius: 'var(--radius-md)',
                      border: '1px solid var(--border)',
                      background: 'var(--bg-sunken)',
                    }}
                  >
                    <span style={{ font: 'var(--text-mono)', color: 'var(--fg1)' }}>#{item.id}</span>
                    <span style={{ flex: 1, color: 'var(--fg1)', font: 'var(--text-sm)' }}>{item.title}</span>
                    <span style={{ color: 'var(--fg3)', font: 'var(--text-caption)' }}>{item.type}</span>
                    <button
                      className="btn btn-ghost btn-sm"
                      disabled={addingWorkItemId === item.id}
                      onClick={() => handleAddAssigned(item)}
                    >
                      {addingWorkItemId === item.id ? 'Agregando...' : 'Agregar'}
                    </button>
                  </div>
                ))}
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
