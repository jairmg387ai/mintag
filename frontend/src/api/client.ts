import type {
  Project,
  Meeting,
  Task,
  TaskHistory,
  Stats,
  SearchResult,
  GraphNode,
  GraphNodeSearchResult,
  GraphNodeDetail,
  GraphNeighbor,
  GraphImpactResult,
  GraphStatsResponse,
  DailyActivity,
  UploadResult,
  ActivityCatalog,
  TimelogCategory,
  AzureActivity,
  AzureTimeLogConfigStatus,
  AzureDeviceCodeStartResponse,
  AzureDeviceCodeCompleteResponse,
  ActivityStatus,
  DeploymentWindow,
  DeploymentWindowDetail,
  DWRepo,
  DWArtifact,
  DWTestScenario,
} from '../types'

async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  })
  if (!res.ok) {
    throw new Error(await res.text())
  }
  return res.json() as Promise<T>
}

export function listProjects(): Promise<Project[]> {
  return request<Project[]>('/api/projects')
}

export function createProject(body: { name: string; description: string; color: string }): Promise<Project> {
  return request<Project>('/api/projects', { method: 'POST', body: JSON.stringify(body) })
}

export function listMeetings(): Promise<Meeting[]> {
  return request<Meeting[]>('/api/meetings')
}

export function getMeeting(id: number): Promise<Meeting> {
  return request<Meeting>(`/api/meetings/${id}`)
}

export function importMeeting(body: { path: string; project_id: number | null; summary: string }): Promise<Meeting> {
  return request<Meeting>('/api/meetings/import', { method: 'POST', body: JSON.stringify(body) })
}

export function listTasks(): Promise<Task[]> {
  return request<Task[]>('/api/tasks')
}

export function getTask(id: number): Promise<Task> {
  return request<Task>(`/api/tasks/${id}`)
}

export function createTask(body: Partial<Task>): Promise<Task> {
  return request<Task>('/api/tasks', { method: 'POST', body: JSON.stringify(body) })
}

export function updateTask(id: number, body: Partial<Task> & { note?: string; author?: string; source_meeting_id?: number | null }): Promise<Task> {
  return request<Task>(`/api/tasks/${id}`, { method: 'PUT', body: JSON.stringify(body) })
}

export function getTaskHistory(id: number): Promise<TaskHistory[]> {
  return request<TaskHistory[]>(`/api/tasks/${id}/history`)
}

export function getStats(): Promise<Stats> {
  return request<Stats>('/api/stats')
}

export function search(q: string): Promise<SearchResult[]> {
  return request<SearchResult[]>(`/api/search?q=${encodeURIComponent(q)}`)
}

export function setMeetingRichContent(id: number, body: { content: string; content_type: string }): Promise<Meeting> {
  return request<Meeting>(`/api/meetings/${id}/rich-content`, { method: 'PUT', body: JSON.stringify(body) })
}

// --- Graph ---

export function graphSearch(params: {
  q: string
  namespace?: string
  kind?: string
  limit?: number
}): Promise<GraphNodeSearchResult[]> {
  const qs = new URLSearchParams({ q: params.q })
  if (params.namespace) qs.set('namespace', params.namespace)
  if (params.kind) qs.set('kind', params.kind)
  if (params.limit) qs.set('limit', String(params.limit))
  return request<GraphNodeSearchResult[]>(`/api/graph/search?${qs}`)
}

export function graphNodesByKind(params: {
  kind: string
  namespace?: string
  limit?: number
}): Promise<{ nodes: GraphNode[]; count: number }> {
  const qs = new URLSearchParams({ kind: params.kind })
  if (params.namespace) qs.set('namespace', params.namespace)
  if (params.limit) qs.set('limit', String(params.limit))
  return request(`/api/graph/nodes?${qs}`)
}

export function graphNodeByID(id: number, neighborLimit?: number): Promise<GraphNodeDetail> {
  const qs = neighborLimit ? `?neighbor_limit=${neighborLimit}` : ''
  return request<GraphNodeDetail>(`/api/graph/nodes/${id}${qs}`)
}

export function graphNeighbors(
  id: number,
  params?: { relation?: string; direction?: string; limit?: number },
): Promise<{ neighbors: GraphNeighbor[]; count: number }> {
  const qs = new URLSearchParams()
  if (params?.relation) qs.set('relation', params.relation)
  if (params?.direction) qs.set('direction', params.direction)
  if (params?.limit) qs.set('limit', String(params.limit))
  const q = qs.toString() ? `?${qs}` : ''
  return request(`/api/graph/nodes/${id}/neighbors${q}`)
}

export function graphImpact(
  id: number,
  params?: { max_depth?: number; limit?: number; kind_filter?: string },
): Promise<GraphImpactResult> {
  const qs = new URLSearchParams()
  if (params?.max_depth) qs.set('max_depth', String(params.max_depth))
  if (params?.limit) qs.set('limit', String(params.limit))
  if (params?.kind_filter) qs.set('kind_filter', params.kind_filter)
  const q = qs.toString() ? `?${qs}` : ''
  return request<GraphImpactResult>(`/api/graph/nodes/${id}/impact${q}`)
}

export function graphStats(namespace?: string): Promise<GraphStatsResponse> {
  const qs = namespace ? `?namespace=${encodeURIComponent(namespace)}` : ''
  return request<GraphStatsResponse>(`/api/graph/stats${qs}`)
}

export function graphNamespaces(): Promise<string[]> {
  return request<string[]>('/api/graph/namespaces')
}

// --- Activities (Time Log) ---

export function listActivities(date: string, status?: ActivityStatus): Promise<DailyActivity[]> {
  const qs = new URLSearchParams({ date })
  if (status && status !== ('all' as ActivityStatus)) qs.set('status', status)
  return request<DailyActivity[]>(`/api/activities?${qs}`)
}

export function listActivitiesRange(from: string, to: string, status?: ActivityStatus): Promise<DailyActivity[]> {
  const qs = new URLSearchParams({ from, to })
  if (status && status !== ('all' as ActivityStatus)) qs.set('status', status)
  return request<DailyActivity[]>(`/api/activities?${qs}`)
}

export function createActivity(body: Omit<DailyActivity, 'id' | 'created_at' | 'uploaded_at' | 'status'>): Promise<DailyActivity> {
  return request<DailyActivity>('/api/activities', { method: 'POST', body: JSON.stringify(body) })
}

export function approveActivity(id: number): Promise<{ approved: number }> {
  return request<{ approved: number }>(`/api/activities/${id}`, { method: 'PATCH', body: JSON.stringify({ action: 'approve' }) })
}

export function updateActivity(
  id: number,
  // azure_activity_id: number sets it, null clears it back to "use the
  // current default", undefined (i.e. omitted) leaves it untouched —
  // JSON.stringify drops undefined keys but keeps explicit null ones, and
  // the backend's PATCH handler distinguishes the two the same way.
  body: { hours?: number; project?: string; category?: string; registro_diario?: string; azure_activity_id?: number | null },
): Promise<DailyActivity> {
  return request<DailyActivity>(`/api/activities/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
}

export function unapproveActivity(id: number): Promise<DailyActivity> {
  return request<DailyActivity>(`/api/activities/${id}`, { method: 'PATCH', body: JSON.stringify({ action: 'unapprove' }) })
}

export function uploadActivities(date: string): Promise<UploadResult> {
  return request<UploadResult>(`/api/activities/upload?date=${encodeURIComponent(date)}`, { method: 'POST' })
}

export function getAzureTimeLogConfig(): Promise<AzureTimeLogConfigStatus> {
  return request<AzureTimeLogConfigStatus>('/api/activities/azure-config')
}

export function saveAzureTimeLogConfig(body: { token: string; auth_mode: 'bearer' | 'basic' }): Promise<AzureTimeLogConfigStatus> {
  return request<AzureTimeLogConfigStatus>('/api/activities/azure-config', { method: 'PUT', body: JSON.stringify(body) })
}

export function clearAzureTimeLogConfig(): Promise<AzureTimeLogConfigStatus> {
  return request<AzureTimeLogConfigStatus>('/api/activities/azure-config', { method: 'DELETE' })
}

export function startAzureDeviceAuth(): Promise<AzureDeviceCodeStartResponse> {
  return request<AzureDeviceCodeStartResponse>('/api/activities/azure-auth/device/start', { method: 'POST', body: JSON.stringify({}) })
}

export function completeAzureDeviceAuth(deviceCode: string): Promise<AzureDeviceCodeCompleteResponse> {
  return request<AzureDeviceCodeCompleteResponse>('/api/activities/azure-auth/device/complete', {
    method: 'POST',
    body: JSON.stringify({ device_code: deviceCode }),
  })
}

export function getActivityCatalog(): Promise<ActivityCatalog> {
  return request<ActivityCatalog>('/api/activities/catalog')
}

export async function deleteActivity(id: number): Promise<void> {
  const res = await fetch(`/api/activities/${id}`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
  })
  if (!res.ok) throw new Error(await res.text())
}

export function addCatalogProject(name: string): Promise<{ name: string }> {
  return request<{ name: string }>('/api/activities/catalog/projects', { method: 'POST', body: JSON.stringify({ name }) })
}

export async function removeCatalogProject(name: string): Promise<void> {
  const res = await fetch(`/api/activities/catalog/projects/${encodeURIComponent(name)}`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
  })
  if (!res.ok) throw new Error(await res.text())
}

export function addCatalogCategory(name: string): Promise<{ name: string }> {
  return request<{ name: string }>('/api/activities/catalog/categories', { method: 'POST', body: JSON.stringify({ name }) })
}

export async function removeCatalogCategory(name: string): Promise<void> {
  const res = await fetch(`/api/activities/catalog/categories/${encodeURIComponent(name)}`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
  })
  if (!res.ok) throw new Error(await res.text())
}

// setCategoryAzureActivity assigns (azureActivityId a number) or clears
// (azureActivityId null) a category's default Azure activity mapping.
export function setCategoryAzureActivity(id: number, azureActivityId: number | null): Promise<TimelogCategory> {
  return request<TimelogCategory>(`/api/activities/catalog/categories/${id}/azure-activity`, {
    method: 'PUT',
    body: JSON.stringify({ azure_activity_id: azureActivityId }),
  })
}

// --- Azure Activity Catalog (work items) ---

export function listAzureActivities(): Promise<AzureActivity[]> {
  return request<AzureActivity[]>('/api/activities/azure-catalog')
}

export function addAzureActivity(body: { org: string; work_item_id: number; label: string }): Promise<AzureActivity> {
  return request<AzureActivity>('/api/activities/azure-catalog', { method: 'POST', body: JSON.stringify(body) })
}

export function updateAzureActivity(id: number, body: { org: string; label: string }): Promise<AzureActivity> {
  return request<AzureActivity>(`/api/activities/azure-catalog/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
}

export async function deactivateAzureActivity(id: number): Promise<void> {
  const res = await fetch(`/api/activities/azure-catalog/${id}`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
  })
  if (!res.ok) throw new Error(await res.text())
}

export function setDefaultAzureActivity(id: number): Promise<AzureActivity> {
  return request<AzureActivity>(`/api/activities/azure-catalog/${id}/default`, { method: 'POST' })
}

// exportActivities downloads the .xlsx export for [from, to] (inclusive) and
// triggers a browser download using the filename the server sent via
// Content-Disposition, falling back to a locally built name if that header
// is ever missing.
export async function exportActivities(from: string, to: string): Promise<void> {
  const qs = new URLSearchParams({ from, to })
  const res = await fetch(`/api/activities/export?${qs}`)
  if (!res.ok) throw new Error(await res.text())

  const disposition = res.headers.get('Content-Disposition') ?? ''
  const match = /filename="?([^"]+)"?/.exec(disposition)
  const filename = match ? match[1] : `actividades_${from}_${to}.xlsx`

  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

export function updateMeetingSummary(id: number, summary: string): Promise<Meeting> {
  return request<Meeting>(`/api/meetings/${id}/summary`, { method: 'PUT', body: JSON.stringify({ summary }) })
}

// --- Deployment Windows ---

export function listDeploymentWindows(state?: string): Promise<DeploymentWindow[]> {
  const qs = state ? `?state=${encodeURIComponent(state)}` : ''
  return request<DeploymentWindow[]>(`/api/deployment-windows${qs}`)
}

export function getDeploymentWindow(id: number): Promise<DeploymentWindowDetail> {
  return request<DeploymentWindowDetail>(`/api/deployment-windows/${id}`)
}

export function createDeploymentWindow(data: {
  title: string
  description?: string
  created_by?: string
  planned_at?: string
}): Promise<DeploymentWindow> {
  return request<DeploymentWindow>('/api/deployment-windows', { method: 'POST', body: JSON.stringify(data) })
}

export function updateDWState(id: number, state: string, rejection_note?: string): Promise<DeploymentWindow> {
  return request<DeploymentWindow>(`/api/deployment-windows/${id}/state`, {
    method: 'PATCH',
    body: JSON.stringify({ state, rejection_note: rejection_note ?? '' }),
  })
}

export async function exportDWMarkdown(id: number): Promise<string> {
  const res = await fetch(`/api/deployment-windows/${id}/export`, {
    headers: { 'Content-Type': 'application/json' },
  })
  if (!res.ok) throw new Error(await res.text())
  return res.text()
}

export function addDWTask(dwId: number, taskId: number, note?: string): Promise<void> {
  return request<void>(`/api/deployment-windows/${dwId}/tasks`, {
    method: 'POST',
    body: JSON.stringify({ task_id: taskId, note: note ?? '' }),
  })
}

export async function removeDWTask(dwId: number, taskId: number): Promise<void> {
  const res = await fetch(`/api/deployment-windows/${dwId}/tasks/${taskId}`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
  })
  if (!res.ok) throw new Error(await res.text())
}

export function addDWRepo(dwId: number, data: { graph_node_key: string; version: string; notes?: string }): Promise<DWRepo> {
  return request<DWRepo>(`/api/deployment-windows/${dwId}/repos`, { method: 'POST', body: JSON.stringify(data) })
}

export function updateDWRepo(dwId: number, repoId: number, data: { version: string; notes?: string }): Promise<DWRepo> {
  return request<DWRepo>(`/api/deployment-windows/${dwId}/repos/${repoId}`, { method: 'PATCH', body: JSON.stringify(data) })
}

export async function removeDWRepo(dwId: number, repoId: number): Promise<void> {
  const res = await fetch(`/api/deployment-windows/${dwId}/repos/${repoId}`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
  })
  if (!res.ok) throw new Error(await res.text())
}

export function addDWArtifact(dwId: number, data: { kind: string; name: string; path?: string; content?: string }): Promise<DWArtifact> {
  return request<DWArtifact>(`/api/deployment-windows/${dwId}/artifacts`, { method: 'POST', body: JSON.stringify(data) })
}

export async function removeDWArtifact(dwId: number, artifactId: number): Promise<void> {
  const res = await fetch(`/api/deployment-windows/${dwId}/artifacts/${artifactId}`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
  })
  if (!res.ok) throw new Error(await res.text())
}

export function addDWTestScenario(
  dwId: number,
  data: { title: string; description?: string; expected?: string; sort_order?: number },
): Promise<DWTestScenario> {
  return request<DWTestScenario>(`/api/deployment-windows/${dwId}/test-scenarios`, { method: 'POST', body: JSON.stringify(data) })
}

export async function removeDWTestScenario(dwId: number, scenarioId: number): Promise<void> {
  const res = await fetch(`/api/deployment-windows/${dwId}/test-scenarios/${scenarioId}`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
  })
  if (!res.ok) throw new Error(await res.text())
}

export function signOffScenario(dwId: number, scenarioId: number, result: string, signed_off_by: string): Promise<DWTestScenario> {
  return request<DWTestScenario>(`/api/deployment-windows/${dwId}/test-scenarios/${scenarioId}/sign-off`, {
    method: 'PATCH',
    body: JSON.stringify({ result, signed_off_by }),
  })
}
