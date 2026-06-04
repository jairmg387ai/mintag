export type Status = 'todo' | 'in_progress' | 'blocked' | 'done' | 'cancelled'
export type Priority = 'low' | 'medium' | 'high' | 'critical'

export interface Project {
  id: number
  name: string
  description: string
  color: string
  created_at: string
}

export interface Meeting {
  id: number
  project_id: number | null
  filename: string
  date: string
  title: string
  raw_content?: string
  summary: string
  task_count?: number
  created_at: string
  rich_content?: string
  content_type?: string
}

export interface Task {
  id: number
  meeting_id: number | null
  project_id: number | null
  title: string
  description: string
  status: Status
  priority: Priority
  owner: string
  due_date: string
  created_at: string
  updated_at: string
  project_name?: string
  meeting_title?: string
}

export interface TaskHistory {
  id: number
  task_id: number
  source_meeting_id: number | null
  old_status: string
  new_status: string
  note: string
  author: string
  created_at: string
  source_meeting_title?: string
}

export interface SearchResult {
  kind: 'task' | 'meeting'
  id: number
  title: string
  snippet: string
}

export interface Stats {
  total_tasks: number
  todo_tasks: number
  in_progress_tasks: number
  blocked_tasks: number
  done_tasks: number
  total_meetings: number
  total_projects: number
}

export type ViewName = 'dashboard' | 'tasks' | 'meetings'
export type TaskViewName = 'list' | 'kanban'
export type ModalName = 'task' | 'new-task' | 'import' | 'new-project' | 'meeting' | null

export interface Toast {
  id: number
  message: string
  isError: boolean
}
