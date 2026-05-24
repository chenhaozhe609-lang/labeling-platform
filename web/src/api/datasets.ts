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

export async function syncDataset(id: number, file: File): Promise<DatasetDetail> {
  const fd = new FormData()
  fd.append('file', file)
  const { data } = await api.post<DatasetDetail>(`/datasets/${id}/sync`, fd)
  return data
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
