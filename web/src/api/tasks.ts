import { api } from './client'
import type { AnnotationData, DatasetListItem, TaskBundle } from '@/types'

export async function listDatasets(): Promise<DatasetListItem[]> {
  const { data } = await api.get<{ items: DatasetListItem[] }>('/datasets')
  return data.items
}

// claim 成功返回完整 bundle；池空返回 { task: null }（暂停时附 paused:true）
export type ClaimResult = TaskBundle | { task: null; paused?: boolean }

export async function claimTask(datasetId: number): Promise<ClaimResult> {
  const { data } = await api.post<ClaimResult>('/tasks/claim', { dataset_id: datasetId })
  return data
}

export async function getTask(id: number): Promise<TaskBundle> {
  const { data } = await api.get<TaskBundle>(`/tasks/${id}`)
  return data
}

export async function heartbeat(id: number): Promise<{ lease_expires_at: string }> {
  const { data } = await api.post<{ lease_expires_at: string }>(`/tasks/${id}/heartbeat`)
  return data
}

export async function submitTask(
  id: number,
  data: AnnotationData,
  formSchemaVersion: number,
): Promise<void> {
  await api.post(`/tasks/${id}/submit`, { data, form_schema_version: formSchemaVersion })
}

export async function releaseTask(id: number): Promise<void> {
  await api.post(`/tasks/${id}/release`)
}
