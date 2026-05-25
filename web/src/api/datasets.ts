import { api } from './client'
import type { DatasetDetail, DatasetListItem, FormSchema } from '@/types'

export async function listDatasets(): Promise<DatasetListItem[]> {
  const { data } = await api.get<{ items: DatasetListItem[] }>('/datasets')
  return data.items
}

export async function getDatasetDetail(id: number): Promise<DatasetDetail> {
  const { data } = await api.get<DatasetDetail>(`/datasets/${id}`)
  return data
}

export async function uploadDataset(name: string, file: File): Promise<DatasetDetail> {
  const fd = new FormData()
  fd.append('name', name)
  fd.append('file', file)
  const { data } = await api.post<DatasetDetail>('/datasets', fd)
  return data
}

export async function generateTasks(id: number): Promise<DatasetDetail> {
  const { data } = await api.post<DatasetDetail>(`/datasets/${id}/generate-tasks`)
  return data
}

// 暂停/恢复数据集（admin，C5.5）。暂停后标注员领不到新任务。
export async function pauseDataset(id: number): Promise<DatasetDetail> {
  const { data } = await api.post<DatasetDetail>(`/datasets/${id}/pause`)
  return data
}

export async function resumeDataset(id: number): Promise<DatasetDetail> {
  const { data } = await api.post<DatasetDetail>(`/datasets/${id}/resume`)
  return data
}

export async function syncDataset(id: number, file: File): Promise<DatasetDetail> {
  const fd = new FormData()
  fd.append('file', file)
  const { data } = await api.post<DatasetDetail>(`/datasets/${id}/sync`, fd)
  return data
}

// 导出「补全后的表」（源行 + fills 叠加），流式拉为 blob 后触发浏览器下载。
export async function exportDataset(
  id: number,
  format: 'jsonl' | 'csv',
  onlyApproved = false,
): Promise<void> {
  const res = await api.get(`/datasets/${id}/export`, {
    params: { format, only_approved: onlyApproved },
    responseType: 'blob',
  })
  const url = URL.createObjectURL(res.data as Blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `dataset_${id}_export.${format}`
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

export async function updateFormSchema(
  id: number,
  formSchema: FormSchema,
): Promise<{ form_schema_version: number }> {
  const { data } = await api.put<{ form_schema_version: number }>(
    `/datasets/${id}/form-schema`,
    formSchema,
  )
  return data
}
