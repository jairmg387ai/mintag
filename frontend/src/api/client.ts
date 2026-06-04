import type { Project, Meeting, Task, TaskHistory, Stats, SearchResult } from '../types'

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
