// 数据契约 —— 对齐 PRD §8 DDL / §9 API / §10.2 form_schema（详见 UI_BUILD_SPEC §6）

export type Role = 'annotator' | 'reviewer' | 'admin'
export type DatasetStatus = 'IMPORTING' | 'READY' | 'PAUSED' | 'DONE' | 'FAILED'
export type TaskStatus = 'PENDING' | 'CLAIMED' | 'COMPLETED' | 'NEEDS_REDO'
export type ReviewStatus = 'approved' | 'needs_redo'

export interface User {
  id: number
  username: string
  role: Role
  created_at: string
}

// ---- form_schema v2（列角色补全，PRD §24）----
export type ColumnRole = 'context' | 'fill' | 'hidden' | 'id'
export type FieldKind = 'text' | 'single' | 'multi' | 'number' | 'bool' | 'date'

export interface FieldOption {
  value: string
  label: string
  key?: string // 快捷键
}
export interface FieldConfig {
  kind: FieldKind
  options?: FieldOption[]
  regex?: string
  placeholder?: string
  hint?: string
  min?: number
  max?: number
  step?: number
}
export interface ColumnSpec {
  code: string
  type: string
  role: ColumnRole
  label?: string
  pk?: boolean
  field?: FieldConfig
}
export interface FormSchema {
  version: number
  primary_cols: string[]
  columns: ColumnSpec[]
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
  active_annotators?: number
  form_schema_version: number
  updated_at?: string
}

export interface DatasetFull {
  id: number
  name: string
  source_schema: string
  source_table: string
  source_pk_column: string
  hash_columns: string[]
  form_schema: FormSchema
  form_schema_version: number
  status: DatasetStatus
  total_rows: number
  created_at: string
}

export interface ImportBatch {
  id: number
  dataset_id: number
  file_name?: string | null
  file_size_bytes?: number | null
  new_task_count: number
  updated_task_count: number
  error?: string | null
  created_at: string
}

export interface DatasetDetail {
  dataset: DatasetFull
  progress: { pending: number; claimed: number; completed: number }
  batches: ImportBatch[]
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

// ---- 标注数据（v2：补全 fill 列的值）----
export interface AnnotationData {
  fills: Record<string, unknown> // fill 列 code → 填入值
  _source?: 'human' | 'ai' | 'ai-edited'
  _durationSec?: number
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

// ---- 审核（reviewer 抽检台，C5.1/5.2）----
export interface ReviewItem {
  annotation_id: number
  task_id: number
  source_row_pk: string
  round: number
  annotator: string
  created_at: string
  data: AnnotationData // 标注员补全的 fills + _source
  source_row: Record<string, unknown>
}
export interface ReviewQueueResponse {
  dataset_name: string
  form_schema: FormSchema
  pending_total: number // 抽检池总量（去重 limit 后的可审条数）
  items: ReviewItem[]
}
