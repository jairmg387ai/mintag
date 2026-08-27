import { useState, useEffect, useCallback, type CSSProperties } from 'react'
import { Plus, FilePlus2, RefreshCw, ExternalLink, UserPlus, Pencil, Check, X, Star, Trash2, RotateCcw, Search, ChevronLeft, ChevronRight, Bug, ListTodo } from 'lucide-react'
import type { ActivityCatalog, AzureActivity, AssignedAzureWorkItem, CreatedWorkItemResponse } from '../../types'
import {
  getActivityCatalog,
  listAzureActivities,
  fetchAzureWorkItemStates,
  closeAzureWorkItem,
  recreateAzureWorkItem,
  listAssignedAzureWorkItems,
  addAzureActivity,
  updateAzureActivity,
  deactivateAzureActivity,
  reactivateAzureActivity,
  setDefaultAzureActivity,
} from '../../api/client'
import { friendlyCatalogErrorMessage } from '../activities/azureActivity'
import { useAppActions, useAppState } from '../../store/AppContext'
import { Card, CardHeader } from '../ui/Card'
import { CreateWorkItemModal } from './CreateWorkItemModal'
import { AzureWorkItemStateBadge } from './AzureWorkItemStateBadge'

const PAGE_SIZE = 20

const inputStyle: CSSProperties = {
  width: '100%',
  padding: '6px 8px',
  border: '1px solid var(--border-strong)',
  borderRadius: 'var(--radius-md)',
  font: 'var(--text-body)',
  fontSize: '0.9em',
  color: 'var(--fg1)',
  background: 'var(--bg-sunken)',
  outline: 'none',
  boxSizing: 'border-box',
}

// isClosedAzureState mirrors azure.IsClosedState (Go) for the subset of
// state strings the frontend needs to reason about — shared by the "hide
// closed" filter and the Close/Recreate row-disable logic below.
function isClosedAzureState(state: string | undefined): boolean {
  return state === 'Closed' || state === 'Cerrado'
}

// WorkItemsView creates Azure DevOps work items and is the single place to
// manage the local azure_activities catalog (add/edit/deactivate/default),
// plus an opt-in live-state refresh against Azure; it is not a full
// sync/history of every work item ever touched. CatalogManagementModal used
// to own a second, overlapping "Work items de Azure" tab for the same
// catalog — that tab was removed so this table is the only place left.
export function WorkItemsView() {
  const { pushToast, openModal, setActiveBugEvidenceId } = useAppActions()
  const { azureConfig } = useAppState()
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
  const [showInactive, setShowInactive] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [page, setPage] = useState(1)
  const [pendingAssigned, setPendingAssigned] = useState<AssignedAzureWorkItem[] | null>(null)
  const [assignedOrg, setAssignedOrg] = useState('')
  const [syncingAssigned, setSyncingAssigned] = useState(false)
  const [addingWorkItemId, setAddingWorkItemId] = useState<number | null>(null)
  const [assignedSearchQuery, setAssignedSearchQuery] = useState('')
  const [assignedPage, setAssignedPage] = useState(1)

  // Manual add — mirrors the row the removed CatalogManagementModal "Work
  // items de Azure" tab used to offer, for work items that never show up in
  // "Sincronizar asignados" (e.g. registering someone else's work item).
  const [manualOpen, setManualOpen] = useState(false)
  const [manualOrg, setManualOrg] = useState('')
  const [manualWorkItemId, setManualWorkItemId] = useState('')
  const [manualLabel, setManualLabel] = useState('')
  const [manualWorkItemType, setManualWorkItemType] = useState('')
  const [manualProject, setManualProject] = useState('')
  const [manualCategory, setManualCategory] = useState('')
  const [manualAdding, setManualAdding] = useState(false)
  const [manualError, setManualError] = useState('')

  // Catalog edit / default / deactivate — same actions the removed modal tab
  // had, keyed by azure_activity id (not work_item_id, which rowBusy above
  // already keys the Close/Recreate Azure mutations by).
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editOrg, setEditOrg] = useState('')
  const [editLabel, setEditLabel] = useState('')
  const [editWorkItemType, setEditWorkItemType] = useState('')
  const [editProject, setEditProject] = useState('')
  const [editCategory, setEditCategory] = useState('')
  const [catalogBusyId, setCatalogBusyId] = useState<number | null>(null)
  const [catalogError, setCatalogError] = useState('')

  const loadAzureActivities = useCallback((includeInactive: boolean) => {
    setActivitiesLoading(true)
    listAzureActivities(includeInactive)
      .then(setAzureActivities)
      .catch(() => setAzureActivities([]))
      .finally(() => setActivitiesLoading(false))
  }, [])

  useEffect(() => {
    getActivityCatalog().then(setCatalog).catch(() => setCatalog(null))
  }, [])

  useEffect(() => {
    loadAzureActivities(showInactive)
  }, [loadAzureActivities, showInactive])

  // Any filter/search change invalidates the current page.
  useEffect(() => {
    setPage(1)
  }, [searchQuery, hideClosed, hideBugs, hideTasks, showInactive])

  // A new search or a fresh sync (new pendingAssigned list) invalidates the
  // current page of the "Asignados sin catalogar" list.
  useEffect(() => {
    setAssignedPage(1)
  }, [assignedSearchQuery, pendingAssigned])

  function categoryName(categoryId: AzureActivity['category_id']): string {
    if (!categoryId || !catalog) return '—'
    return catalog.categories.find(c => c.id === categoryId)?.name ?? '—'
  }

  // knownState prefers this session's live refresh (liveStates) over the
  // catalog's persisted last_known_state from a previous refresh, so a
  // manual "Refrescar estados" always wins, but the portal still shows a
  // state on load instead of "—" every time before the user refreshes again.
  function knownState(a: AzureActivity): string | undefined {
    return liveStates[a.work_item_id]?.state ?? (a.last_known_state || undefined)
  }

  // knownAssignee mirrors knownState's live-over-cached preference, for the
  // assignee display name backing the Close/Recreate "only the assignee"
  // gate below.
  function knownAssignee(a: AzureActivity): string | undefined {
    return liveStates[a.work_item_id]?.assigned_to_display_name ?? (a.last_known_assigned_to || undefined)
  }

  // canCloseOrRecreate is a UI-only hint, not the enforcement — the backend
  // (ensureAssignedToCaller in work_items.go) always re-checks the live
  // assignee id at close/recreate time and is the actual source of truth.
  // When either side of the comparison is unknown (no states refresh has run
  // yet, or the connected identity's display name isn't loaded), this stays
  // permissive rather than blocking on a guess — the backend still guards.
  function canCloseOrRecreate(a: AzureActivity): boolean {
    const assignee = knownAssignee(a)
    const me = azureConfig?.user_display_name
    if (!assignee || !me) return true
    return assignee.trim().toLowerCase() === me.trim().toLowerCase()
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
      // The backend persists each item's live state/type into the catalog
      // (see SyncAzureActivityLiveState) — reload so a work item reclassified
      // in Azure (e.g. Bug -> Task) picks up its corrected type here too,
      // not just its state.
      loadAzureActivities(showInactive)
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
      loadAzureActivities(showInactive)
      pushToast(`Work item ${item.id} agregado al catálogo.`, false)
    } catch (e: unknown) {
      pushToast(e instanceof Error ? e.message : 'No se pudo agregar el work item al catálogo', true)
    } finally {
      setAddingWorkItemId(null)
    }
  }

  async function handleManualAdd() {
    const wid = parseInt(manualWorkItemId, 10)
    if (!manualOrg.trim() || !manualLabel.trim() || !manualWorkItemId.trim() || isNaN(wid)) {
      setManualError('La organización, el ID del work item y la etiqueta son obligatorios')
      return
    }
    setManualAdding(true)
    setManualError('')
    try {
      await addAzureActivity({
        org: manualOrg.trim(),
        work_item_id: wid,
        label: manualLabel.trim(),
        work_item_type: manualWorkItemType || undefined,
        project: manualProject || undefined,
        category_id: manualCategory ? Number(manualCategory) : undefined,
      })
      setManualOrg('')
      setManualWorkItemId('')
      setManualLabel('')
      setManualWorkItemType('')
      setManualProject('')
      setManualCategory('')
      loadAzureActivities(showInactive)
    } catch (e: unknown) {
      setManualError(friendlyCatalogErrorMessage(e, 'No se pudo agregar el work item'))
    } finally {
      setManualAdding(false)
    }
  }

  function startEdit(a: AzureActivity) {
    setEditingId(a.id)
    setEditOrg(a.org)
    setEditLabel(a.label)
    setEditWorkItemType(a.work_item_type ?? '')
    setEditProject(a.project ?? '')
    setEditCategory(a.category_id != null ? String(a.category_id) : '')
    setCatalogError('')
  }

  function cancelEdit() {
    setEditingId(null)
    setCatalogError('')
  }

  // updateAzureActivity is a full replace: project/category_id are always
  // sent (empty string / '' -> null), never omitted, so an unedited field
  // isn't silently cleared by relying on "undefined drops the key" like the
  // manual-add call above does — that behavior only applies to inserts.
  async function saveEdit(id: number) {
    if (!editOrg.trim() || !editLabel.trim()) {
      setCatalogError('La organización y la etiqueta son obligatorias')
      return
    }
    setCatalogBusyId(id)
    setCatalogError('')
    try {
      await updateAzureActivity(id, {
        org: editOrg.trim(),
        label: editLabel.trim(),
        work_item_type: editWorkItemType,
        project: editProject.trim() || null,
        category_id: editCategory ? Number(editCategory) : null,
      })
      setEditingId(null)
      loadAzureActivities(showInactive)
    } catch (e: unknown) {
      setCatalogError(friendlyCatalogErrorMessage(e, 'No se pudo actualizar el work item'))
    } finally {
      setCatalogBusyId(null)
    }
  }

  async function handleSetDefault(id: number) {
    setCatalogBusyId(id)
    setCatalogError('')
    try {
      await setDefaultAzureActivity(id)
      loadAzureActivities(showInactive)
    } catch (e: unknown) {
      setCatalogError(friendlyCatalogErrorMessage(e, 'No se pudo definir el work item predeterminado'))
    } finally {
      setCatalogBusyId(null)
    }
  }

  async function handleDeactivate(id: number) {
    setCatalogBusyId(id)
    setCatalogError('')
    try {
      await deactivateAzureActivity(id)
      loadAzureActivities(showInactive)
    } catch (e: unknown) {
      setCatalogError(friendlyCatalogErrorMessage(e, 'No se pudo desactivar el work item'))
    } finally {
      setCatalogBusyId(null)
    }
  }

  async function handleReactivate(id: number) {
    setCatalogBusyId(id)
    setCatalogError('')
    try {
      await reactivateAzureActivity(id)
      loadAzureActivities(showInactive)
    } catch (e: unknown) {
      setCatalogError(friendlyCatalogErrorMessage(e, 'No se pudo reactivar el work item'))
    } finally {
      setCatalogBusyId(null)
    }
  }

  // handleClose/handleRecreate are destructive, irreversible Azure mutations
  // (see azure.Client.CloseWorkItem/CreateAndActivateWorkItem doc comments) —
  // both require an explicit confirm, mirroring ActivitiesView's
  // window.confirm convention for the delete action.
  //
  // Both take the full AzureActivity row (not just id/label) so they can
  // run the "assigned to me?" check locally before anything else: when we
  // already know the answer (a states refresh has run), clicking gives an
  // immediate toast instead of a round trip that just comes back as a 403 —
  // the backend's ensureAssignedToCaller still re-checks live and is the
  // real enforcement (see its doc comment), this is purely a faster no.
  async function handleClose(a: AzureActivity) {
    if (!canCloseOrRecreate(a)) {
      pushToast(`No puedes cerrar el work item ${a.work_item_id}: está asignado a ${knownAssignee(a)}, no a ti.`, true)
      return
    }
    const workItemId = a.work_item_id
    const label = a.label
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

  async function handleRecreate(a: AzureActivity) {
    if (!canCloseOrRecreate(a)) {
      pushToast(`No puedes recrear el work item ${a.work_item_id}: está asignado a ${knownAssignee(a)}, no a ti.`, true)
      return
    }
    const workItemId = a.work_item_id
    const label = a.label
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
      loadAzureActivities(showInactive)
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
      loadAzureActivities(showInactive)
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
            {/* Manual add */}
            <div style={{ marginBottom: 14 }}>
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => setManualOpen(v => !v)}
                style={{ display: 'inline-flex', alignItems: 'center', gap: 6, marginBottom: manualOpen ? 8 : 0 }}
              >
                <Plus size={14} strokeWidth={1.75} />
                {manualOpen ? 'Ocultar alta manual' : 'Agregar manualmente'}
              </button>

              {manualOpen && (
                <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', alignItems: 'flex-start' }}>
                  <input
                    type="text"
                    value={manualOrg}
                    onChange={e => setManualOrg(e.target.value)}
                    placeholder="Organización (ej. RUNT2PSW)"
                    style={{ ...inputStyle, flex: '1 1 140px' }}
                    disabled={manualAdding}
                  />
                  <input
                    type="number"
                    value={manualWorkItemId}
                    onChange={e => setManualWorkItemId(e.target.value)}
                    placeholder="ID del work item"
                    style={{ ...inputStyle, flex: '1 1 120px' }}
                    disabled={manualAdding}
                  />
                  <input
                    type="text"
                    value={manualLabel}
                    onChange={e => setManualLabel(e.target.value)}
                    placeholder="Etiqueta"
                    style={{ ...inputStyle, flex: '1 1 160px' }}
                    disabled={manualAdding}
                  />
                  <select
                    aria-label="Tipo de work item"
                    value={manualWorkItemType}
                    onChange={e => setManualWorkItemType(e.target.value)}
                    style={{ ...inputStyle, flex: '0 1 120px' }}
                    disabled={manualAdding}
                  >
                    <option value="">Sin tipo</option>
                    <option value="Bug">Bug</option>
                    <option value="Task">Task</option>
                  </select>
                  {catalog !== null ? (
                    <select
                      aria-label="Proyecto"
                      value={manualProject}
                      onChange={e => setManualProject(e.target.value)}
                      style={{ ...inputStyle, flex: '0 1 140px' }}
                      disabled={manualAdding}
                    >
                      <option value="">Sin proyecto</option>
                      {catalog.projects.map(p => (
                        <option key={p.name} value={p.name}>{p.name}</option>
                      ))}
                    </select>
                  ) : (
                    <input
                      type="text"
                      value={manualProject}
                      onChange={e => setManualProject(e.target.value)}
                      placeholder="Proyecto (opcional)"
                      style={{ ...inputStyle, flex: '1 1 140px' }}
                      disabled={manualAdding}
                    />
                  )}
                  <select
                    aria-label="Categoría"
                    value={manualCategory}
                    onChange={e => setManualCategory(e.target.value)}
                    style={{ ...inputStyle, flex: '0 1 140px' }}
                    disabled={manualAdding}
                  >
                    <option value="">Sin categoría</option>
                    {(catalog?.categories ?? []).map(c => (
                      <option key={c.id} value={c.id}>{c.name}</option>
                    ))}
                  </select>
                  <button
                    className="btn btn-primary btn-sm"
                    onClick={handleManualAdd}
                    disabled={manualAdding || !manualOrg.trim() || !manualWorkItemId.trim() || !manualLabel.trim()}
                    aria-label="Agregar work item"
                  >
                    <Plus size={14} strokeWidth={1.75} />
                  </button>
                </div>
              )}

              {manualOpen && manualError && (
                <div style={{ font: 'var(--text-caption)', color: 'var(--block-solid)', marginTop: 8 }}>
                  {manualError}
                </div>
              )}
            </div>

            {catalogError && (
              <div style={{ font: 'var(--text-caption)', color: 'var(--block-solid)', marginBottom: 10 }}>
                {catalogError}
              </div>
            )}

            {azureActivities.length > 0 && (
              <div style={{ display: 'flex', gap: 16, marginBottom: 14, flexWrap: 'wrap', alignItems: 'center' }}>
                <div style={{ position: 'relative', flex: '1 1 220px', minWidth: 180 }}>
                  <Search size={14} strokeWidth={1.75} style={{ position: 'absolute', left: 8, top: '50%', transform: 'translateY(-50%)', color: 'var(--fg3)' }} />
                  <input
                    type="search"
                    value={searchQuery}
                    onChange={e => setSearchQuery(e.target.value)}
                    placeholder="Buscar por ID, etiqueta, tipo, proyecto o categoría..."
                    aria-label="Buscar work items registrados"
                    style={{ ...inputStyle, paddingLeft: 28 }}
                  />
                </div>
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
                <label style={{ display: 'flex', alignItems: 'center', gap: 6, font: 'var(--text-sm)', color: 'var(--fg2)' }}>
                  <input type="checkbox" checked={showInactive} onChange={e => setShowInactive(e.target.checked)} />
                  Mostrar inactivos
                </label>
              </div>
            )}
            {(() => {
              const normalizedSearch = searchQuery.trim().toLowerCase()
              const visibleActivities = azureActivities.filter(a => {
                if (hideBugs && a.work_item_type === 'Bug') return false
                if (hideTasks && a.work_item_type === 'Task') return false
                // A row whose state hasn't been fetched yet is never hidden
                // by this filter — unknown is not "closed".
                if (hideClosed && isClosedAzureState(knownState(a))) return false
                if (normalizedSearch) {
                  const haystack = [
                    String(a.work_item_id),
                    a.org,
                    a.label,
                    a.work_item_type,
                    a.project ?? '',
                    categoryName(a.category_id),
                  ].join(' ').toLowerCase()
                  if (!haystack.includes(normalizedSearch)) return false
                }
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

              const totalPages = Math.max(1, Math.ceil(visibleActivities.length / PAGE_SIZE))
              const currentPage = Math.min(page, totalPages)
              const pageStart = (currentPage - 1) * PAGE_SIZE
              const pagedActivities = visibleActivities.slice(pageStart, pageStart + PAGE_SIZE)

              return (
                <>
                  <div style={{ overflowX: 'auto' }}>
                    <table className="mt-table">
                      <thead>
                        <tr>
                          <th>ID</th>
                          <th>ORG</th>
                          <th>LABEL</th>
                          <th>PROYECTO</th>
                          <th>CATEGORÍA</th>
                          <th>ESTADO</th>
                          <th>CATÁLOGO</th>
                          <th>AZURE</th>
                        </tr>
                      </thead>
                      <tbody>
                        {pagedActivities.map(a => {
                          // Close/Recreate only apply to Tasks — mirrors the
                          // coworker reference implementation's isBug exclusion;
                          // Bugs have no such action here either.
                          const isTask = a.work_item_type === 'Task'
                          const isBug = a.work_item_type === 'Bug'
                          const closedAlready = isClosedAzureState(knownState(a))
                          const busy = !!rowBusy[a.work_item_id]
                          const isEditing = editingId === a.id
                          const catalogBusy = catalogBusyId === a.id

                          return (
                            <tr key={a.id} style={{ opacity: a.is_active ? 1 : 0.55 }}>
                              <td style={{ font: 'var(--text-mono)', color: 'var(--fg1)' }}>
                                <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                                  {isEditing ? (
                                    <select
                                      aria-label="Tipo de work item"
                                      value={editWorkItemType}
                                      onChange={e => setEditWorkItemType(e.target.value)}
                                      style={{ ...inputStyle, width: 76, padding: '2px 4px', fontSize: '0.8em' }}
                                      disabled={catalogBusy}
                                    >
                                      <option value="">Sin tipo</option>
                                      <option value="Bug">Bug</option>
                                      <option value="Task">Task</option>
                                    </select>
                                  ) : a.work_item_type === 'Bug' ? (
                                    <Bug size={14} strokeWidth={1.75} style={{ color: 'var(--block-solid)', flexShrink: 0 }} aria-label="Bug" />
                                  ) : a.work_item_type === 'Task' ? (
                                    <ListTodo size={14} strokeWidth={1.75} style={{ color: 'var(--fg3)', flexShrink: 0 }} aria-label="Task" />
                                  ) : null}
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
                              {isEditing ? (
                                <>
                                  <td>
                                    <input
                                      type="text"
                                      value={editOrg}
                                      onChange={e => setEditOrg(e.target.value)}
                                      style={{ ...inputStyle, minWidth: 100 }}
                                      disabled={catalogBusy}
                                    />
                                  </td>
                                  <td>
                                    <input
                                      type="text"
                                      value={editLabel}
                                      onChange={e => setEditLabel(e.target.value)}
                                      style={{ ...inputStyle, minWidth: 140 }}
                                      disabled={catalogBusy}
                                    />
                                  </td>
                                  <td>
                                    {catalog !== null ? (
                                      <select
                                        aria-label="Proyecto"
                                        value={editProject}
                                        onChange={e => setEditProject(e.target.value)}
                                        style={{ ...inputStyle, minWidth: 120 }}
                                        disabled={catalogBusy}
                                      >
                                        <option value="">Sin proyecto</option>
                                        {catalog.projects.map(p => (
                                          <option key={p.name} value={p.name}>{p.name}</option>
                                        ))}
                                        {editProject && !catalog.projects.some(p => p.name === editProject) && (
                                          <option value={editProject}>{editProject}</option>
                                        )}
                                      </select>
                                    ) : (
                                      <input
                                        type="text"
                                        value={editProject}
                                        onChange={e => setEditProject(e.target.value)}
                                        placeholder="Proyecto (opcional)"
                                        style={{ ...inputStyle, minWidth: 120 }}
                                        disabled={catalogBusy}
                                      />
                                    )}
                                  </td>
                                  <td>
                                    <select
                                      aria-label="Categoría"
                                      value={editCategory}
                                      onChange={e => setEditCategory(e.target.value)}
                                      style={{ ...inputStyle, minWidth: 120 }}
                                      disabled={catalogBusy}
                                    >
                                      <option value="">Sin categoría</option>
                                      {(catalog?.categories ?? []).map(c => (
                                        <option key={c.id} value={c.id}>{c.name}</option>
                                      ))}
                                      {/* Existence-only validation on the backend: a
                                          stored category_id can dangle once a category
                                          is hard-deleted. Keep it selectable/visible
                                          instead of silently collapsing it to "Sin
                                          categoría" and clearing it on save. */}
                                      {editCategory && !(catalog?.categories ?? []).some(c => String(c.id) === editCategory) && (
                                        <option value={editCategory}>{`Categoría desconocida (#${editCategory})`}</option>
                                      )}
                                    </select>
                                  </td>
                                </>
                              ) : (
                                <>
                                  <td style={{ color: 'var(--fg2)' }}>{a.org}</td>
                                  <td style={{ color: 'var(--fg1)' }}>
                                    {a.label}
                                    {!a.is_active && (
                                      <span className="chip chip-todo" style={{ fontSize: '0.7em', marginLeft: 6 }}>Inactivo</span>
                                    )}
                                  </td>
                                  <td style={{ color: 'var(--fg2)' }}>{a.project || '—'}</td>
                                  <td style={{ color: 'var(--fg2)' }}>{categoryName(a.category_id)}</td>
                                </>
                              )}
                              <td>
                                {knownState(a)
                                  ? <AzureWorkItemStateBadge state={knownState(a)!} />
                                  : <span style={{ color: 'var(--fg3)' }}>—</span>}
                              </td>
                              <td>
                                {isEditing ? (
                                  <div style={{ display: 'flex', gap: 4 }}>
                                    <button
                                      className="btn btn-ghost btn-sm"
                                      onClick={() => saveEdit(a.id)}
                                      disabled={catalogBusy}
                                      title="Guardar"
                                      aria-label={`Guardar ${a.label}`}
                                      style={{ padding: '2px 4px' }}
                                    >
                                      <Check size={13} strokeWidth={1.75} />
                                    </button>
                                    <button
                                      className="btn btn-ghost btn-sm"
                                      onClick={cancelEdit}
                                      disabled={catalogBusy}
                                      title="Cancelar"
                                      aria-label={`Cancelar edición de ${a.label}`}
                                      style={{ padding: '2px 4px' }}
                                    >
                                      <X size={13} strokeWidth={1.75} />
                                    </button>
                                  </div>
                                ) : (
                                  <div style={{ display: 'flex', gap: 4 }}>
                                    {a.is_default ? (
                                      <span
                                        title="Predeterminado"
                                        aria-label={`${a.label} es el predeterminado`}
                                        style={{ display: 'inline-flex', alignItems: 'center', padding: '2px 4px', color: 'var(--prog-solid)' }}
                                      >
                                        <Star size={13} strokeWidth={1.75} fill="currentColor" />
                                      </span>
                                    ) : (
                                      <button
                                        className="btn btn-ghost btn-sm"
                                        onClick={() => handleSetDefault(a.id)}
                                        disabled={catalogBusy}
                                        title="Definir como predeterminado"
                                        aria-label={`Definir ${a.label} como predeterminado`}
                                        style={{ padding: '2px 4px' }}
                                      >
                                        <Star size={13} strokeWidth={1.75} />
                                      </button>
                                    )}
                                    <button
                                      className="btn btn-ghost btn-sm"
                                      onClick={() => startEdit(a)}
                                      disabled={catalogBusy}
                                      title="Editar"
                                      aria-label={`Editar ${a.label}`}
                                      style={{ padding: '2px 4px' }}
                                    >
                                      <Pencil size={13} strokeWidth={1.75} />
                                    </button>
                                    <button
                                      className="btn btn-ghost btn-sm"
                                      onClick={() => handleDeactivate(a.id)}
                                      disabled={catalogBusy || a.is_default || !a.is_active}
                                      title={!a.is_active ? 'Ya está inactivo' : a.is_default ? 'Primero define otro work item como predeterminado' : 'Desactivar'}
                                      aria-label={`Desactivar ${a.label}`}
                                      style={{ padding: '2px 4px' }}
                                    >
                                      <Trash2 size={13} strokeWidth={1.75} />
                                    </button>
                                    {!a.is_active && (
                                      <button
                                        className="btn btn-ghost btn-sm"
                                        onClick={() => handleReactivate(a.id)}
                                        disabled={catalogBusy}
                                        title="Reactivar"
                                        aria-label={`Reactivar ${a.label}`}
                                        style={{ padding: '2px 4px' }}
                                      >
                                        <RotateCcw size={13} strokeWidth={1.75} />
                                      </button>
                                    )}
                                  </div>
                                )}
                              </td>
                              <td>
                                {isTask ? (
                                  <div style={{ display: 'flex', gap: 6 }}>
                                    <button
                                      className="btn btn-ghost btn-sm"
                                      disabled={busy || closedAlready || isEditing}
                                      onClick={() => handleClose(a)}
                                    >
                                      Cerrar
                                    </button>
                                    <button
                                      className="btn btn-ghost btn-sm"
                                      disabled={busy || closedAlready || isEditing}
                                      onClick={() => handleRecreate(a)}
                                    >
                                      Recrear
                                    </button>
                                  </div>
                                ) : isBug ? (
                                  <button
                                    className="btn btn-ghost btn-sm"
                                    disabled={isEditing}
                                    onClick={() => {
                                      setActiveBugEvidenceId(a.work_item_id)
                                      openModal('bug-evidence')
                                    }}
                                  >
                                    Evidencia DSW-PR-017
                                  </button>
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

                  {totalPages > 1 && (
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 10, marginTop: 12 }}>
                      <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
                        {pageStart + 1}–{Math.min(pageStart + PAGE_SIZE, visibleActivities.length)} de {visibleActivities.length}
                      </span>
                      <button
                        className="btn btn-ghost btn-sm"
                        onClick={() => setPage(p => Math.max(1, p - 1))}
                        disabled={currentPage <= 1}
                        aria-label="Página anterior"
                      >
                        <ChevronLeft size={14} strokeWidth={1.75} />
                      </button>
                      <span style={{ font: 'var(--text-caption)', color: 'var(--fg2)' }}>
                        {currentPage} / {totalPages}
                      </span>
                      <button
                        className="btn btn-ghost btn-sm"
                        onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                        disabled={currentPage >= totalPages}
                        aria-label="Página siguiente"
                      >
                        <ChevronRight size={14} strokeWidth={1.75} />
                      </button>
                    </div>
                  )}
                </>
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
            ) : (() => {
              const normalizedAssignedSearch = assignedSearchQuery.trim().toLowerCase()
              const visibleAssigned = normalizedAssignedSearch
                ? pendingAssigned.filter(item =>
                    [String(item.id), item.title, item.type].join(' ').toLowerCase().includes(normalizedAssignedSearch),
                  )
                : pendingAssigned

              if (visibleAssigned.length === 0) {
                return (
                  <div style={{ textAlign: 'center', padding: '12px 0', color: 'var(--fg3)', font: 'var(--text-body)' }}>
                    Ningún work item asignado coincide con la búsqueda.
                  </div>
                )
              }

              const totalAssignedPages = Math.max(1, Math.ceil(visibleAssigned.length / PAGE_SIZE))
              const currentAssignedPage = Math.min(assignedPage, totalAssignedPages)
              const assignedPageStart = (currentAssignedPage - 1) * PAGE_SIZE
              const pagedAssigned = visibleAssigned.slice(assignedPageStart, assignedPageStart + PAGE_SIZE)

              return (
                <>
                  <div style={{ position: 'relative', marginBottom: 12 }}>
                    <Search size={14} strokeWidth={1.75} style={{ position: 'absolute', left: 8, top: '50%', transform: 'translateY(-50%)', color: 'var(--fg3)' }} />
                    <input
                      type="search"
                      value={assignedSearchQuery}
                      onChange={e => setAssignedSearchQuery(e.target.value)}
                      placeholder="Buscar por ID, título o tipo..."
                      aria-label="Buscar work items asignados sin catalogar"
                      style={{ ...inputStyle, paddingLeft: 28 }}
                    />
                  </div>

                  <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                    {pagedAssigned.map(item => (
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

                  {totalAssignedPages > 1 && (
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 10, marginTop: 12 }}>
                      <span style={{ font: 'var(--text-caption)', color: 'var(--fg3)' }}>
                        {assignedPageStart + 1}–{Math.min(assignedPageStart + PAGE_SIZE, visibleAssigned.length)} de {visibleAssigned.length}
                      </span>
                      <button
                        className="btn btn-ghost btn-sm"
                        onClick={() => setAssignedPage(p => Math.max(1, p - 1))}
                        disabled={currentAssignedPage <= 1}
                        aria-label="Página anterior de asignados"
                      >
                        <ChevronLeft size={14} strokeWidth={1.75} />
                      </button>
                      <span style={{ font: 'var(--text-caption)', color: 'var(--fg2)' }}>
                        {currentAssignedPage} / {totalAssignedPages}
                      </span>
                      <button
                        className="btn btn-ghost btn-sm"
                        onClick={() => setAssignedPage(p => Math.min(totalAssignedPages, p + 1))}
                        disabled={currentAssignedPage >= totalAssignedPages}
                        aria-label="Página siguiente de asignados"
                      >
                        <ChevronRight size={14} strokeWidth={1.75} />
                      </button>
                    </div>
                  )}
                </>
              )
            })()}
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
