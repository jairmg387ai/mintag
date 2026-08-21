export type Status = 'todo' | 'in_progress' | 'blocked' | 'in_testing' | 'done' | 'cancelled'
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

// --- Graph types ---

export interface GraphNode {
  id: number
  namespace: string
  kind: string
  key: string
  label: string
  summary?: string
  attrs?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface GraphNeighbor {
  relation: string
  direction: 'in' | 'out'
  edge_attrs?: Record<string, unknown>
  node: GraphNode
}

export interface GraphNodeSearchResult {
  node: GraphNode
  snippet: string
  score: number
}

export interface GraphImpactSlimRow {
  depth: number
  kind: string
  key: string
  label: string
}

export interface GraphImpactResult {
  node: { id: number; kind: string; key: string; label: string }
  impacted_count: number
  impacted_by_kind: Record<string, number>
  impacted: GraphImpactSlimRow[]
  truncated: boolean
}

export interface GraphStats {
  namespace: string
  node_count: number
  edge_count: number
  nodes_by_kind: Record<string, number>
  edges_by_relation: Record<string, number>
}

export interface GraphStatsResponse {
  stats: GraphStats
  namespaces: string[]
}

export interface GraphNodeDetail {
  node: GraphNode
  relations: Record<string, GraphNeighbor[]>
  neighbor_count: number
}

export type ViewName = 'dashboard' | 'tasks' | 'meetings' | 'graph' | 'activities' | 'deployment-windows' | 'work-items' | 'settings'

// --- Work Items (manual Azure DevOps Task creation) ---

export interface ClassificationNode {
  name: string
  children?: ClassificationNode[]
}

export interface CreateWorkItemInput {
  title: string
  description?: string
  area_path: string
  iteration_path: string
  original_estimate?: number
  project?: string
  category_id?: number
}

export interface CreatedWorkItemResponse {
  id: number
  state: string
  activation_error?: string
  azure_activity_id?: number
  catalog_error?: string
}

export interface CloseWorkItemResponse {
  state: string
  hours_synced: number
  already_closed?: boolean
  effort_sync_error?: string
}

export interface RecreateWorkItemResponse {
  id: number
  state: string
  hours_synced: number
  catalog_reassigned: boolean
  azure_activity_id?: number
  activation_error?: string
  effort_sync_error?: string
  catalog_error?: string
}

// --- Menu Options (configurable sidebar catalog) ---

export interface MenuOptionStatus {
  id: ViewName
  label: string
  enabled: boolean
}

// --- Deployment Window types ---

export type DWState = 'draft' | 'submitted' | 'approved' | 'deployed'

export interface DeploymentWindow {
  id: number
  title: string
  description: string
  state: DWState
  created_by: string
  planned_at: string
  deployed_at?: string
  rejection_note?: string
  created_at: string
  updated_at: string
}

export interface DWTask {
  dw_id: number
  task_id: number
  note: string
  task_title?: string
  task_status?: string
}

export interface DWRepo {
  id: number
  dw_id: number
  graph_node_key: string
  version: string
  notes: string
}

export interface DWArtifact {
  id: number
  dw_id: number
  kind: 'db_script' | 'blob' | 'config' | 'other'
  name: string
  path: string
  content: string
  created_at: string
}

export interface DWTestScenario {
  id: number
  dw_id: number
  title: string
  description: string
  expected: string
  result: 'pending' | 'pass' | 'fail'
  signed_off_by: string
  sort_order: number
}

export interface DeploymentWindowDetail extends DeploymentWindow {
  tasks: DWTask[]
  repos: DWRepo[]
  artifacts: DWArtifact[]
  test_scenarios: DWTestScenario[]
}

// --- Activity (Time Log) types ---

export type ActivityStatus = 'pending' | 'approved' | 'uploaded'
export type ActivitySource = 'manual' | 'llm_auto'

export interface DailyActivity {
  id: number
  date: string
  hours: number
  project: string
  category: string
  registro_diario: string
  source: ActivitySource
  status: ActivityStatus
  created_at: string
  uploaded_at?: string
  azure_document_id?: string
  azure_activity_id?: number | null
  reference_id?: string | null
}

export interface AzureActivity {
  id: number
  org: string
  work_item_id: number
  label: string
  work_item_type: string
  is_active: boolean
  is_default: boolean
  project?: string | null
  category_id?: number | null
}

export interface AssignedAzureWorkItem {
  id: number
  title: string
  type: string
  state: string
}

export interface AssignedAzureWorkItemsResponse {
  org: string
  items: AssignedAzureWorkItem[]
}

export interface UploadResult {
  uploaded_count: number
  failed_ids: number[]
  errors: string[]
  azure_document_ids?: Record<string, string>
}

export interface AzureTimeLogConfigStatus {
  configured: boolean
  auth_mode: 'bearer' | 'basic' | 'oauth'
  source: string
  oauth_connected?: boolean
  oauth_access_token_expires_at?: string
  oauth_tenant?: string
  oauth_client_id?: string
  user_display_name?: string
}

export interface AzureDeviceCodeStartResponse {
  device_code: string
  user_code: string
  verification_uri: string
  expires_in: number
  interval: number
  message: string
}

export interface AzureDeviceCodeCompleteResponse {
  status: 'pending' | 'complete' | 'declined' | 'expired'
}

export interface TimelogCategory {
  id: number
  name: string
  description?: string
}

export interface CatalogProject {
  name: string
  is_active: boolean
}

export interface ActivityCatalog {
  projects: CatalogProject[]
  categories: TimelogCategory[]
}

export interface CatalogRetentionSettings {
  bug_retention_days: number | null
  project_retention_days: number | null
}
export interface ActivityValidationSettings {
  max_hours_per_entry: boolean
  weekend_confirm: boolean
  block_closed_work_item: boolean
}
export type TaskViewName = 'list' | 'kanban'
export type ModalName = 'task' | 'new-task' | 'import' | 'new-project' | 'meeting' | null

export interface Toast {
  id: number
  message: string
  isError: boolean
}
