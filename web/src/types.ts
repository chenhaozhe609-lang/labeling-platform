// 数据契约 —— 对齐 PRD §8 DDL / §9 API / §10.2 form_schema（详见 UI_BUILD_SPEC §6）

export type Role = 'annotator' | 'reviewer' | 'admin'
export type DatasetStatus = 'IMPORTING' | 'READY' | 'PAUSED' | 'DONE' | 'FAILED'
export type TaskStatus = 'PENDING' | 'CLAIMED' | 'COMPLETED' | 'NEEDS_REDO'
export type ReviewStatus = 'approved' | 'needs_redo'

export type AnnotationWidget =
  | 'Select'
  | 'MultiSelect'
  | 'Rating'
  | 'Confidence'
  | 'TextArea'
  | 'Input'
  | 'InputNumber'
  | 'Switch'
  | 'DatePicker'
  | 'RelationLink'

export interface User {
  id: number
  username: string
  role: Role
  created_at: string
}

// ---- form_schema ----
export interface FieldOption {
  value: string
  label: string
}
export interface SourceField {
  code: string
  type: string
  widget: string
  label: string
  primary?: boolean // 扩展：是否中栏阅读聚光字段
}
export interface AnnotationField {
  code: string
  label: string
  widget: AnnotationWidget
  required?: boolean
  options?: FieldOption[]
  min?: number
  max?: number
  step?: number
  max_length?: number
  default?: unknown
  group?: 'core' | 'extra' // 扩展：卡片分组
  hotkeys?: Record<string, string> // 扩展：{ "Q":"政治", "W":"历史" }
}
export interface FormSchema {
  version: number
  source_fields: SourceField[]
  annotation_fields: AnnotationField[]
}

// ---- 数据集 ----
export interface DatasetListItem {
  id: number
  name: string
  status: DatasetStatus
  total_rows: number
  completed: number
  pending: number
  claimed: number
  active_annotators: number
  form_schema_version: number
  updated_at: string
}

// ---- 任务（标注台核心）----
export interface Task {
  id: number
  dataset_id: number
  source_row_pk: string
  status: TaskStatus
  round: number
  assigned_to: number | null
  lease_expires_at: string | null
}

/** GET /api/tasks/:id 与 claim 返回的渲染包 */
export interface TaskBundle {
  task: Task
  source_row: Record<string, unknown>
  form_schema: FormSchema
  draft?: AnnotationData | null
  ai_suggestion?: AnnotationData | null
}

// ---- 标注数据（含 AI 预留元字段）----
export interface AnnotationData {
  [fieldCode: string]: unknown
  _source?: 'human' | 'ai' | 'ai-edited'
  _ai_confidence?: number
  _ai_reasoning?: string
}
export interface SubmitPayload {
  data: AnnotationData
  form_schema_version: number
}

// ---- 历史（左栏回显）----
export interface AnnotationHistory {
  round: number
  annotator: string
  created_at: string
  superseded: boolean
}
