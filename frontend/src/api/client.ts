import type {
  Project,
  Meeting,
  Task,
  TaskHistory,
  Stats,
  SearchResult,
  GraphNodeSearchResult,
  GraphNodeDetail,
  GraphNeighbor,
  GraphImpactResult,
  GraphStatsResponse,
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
