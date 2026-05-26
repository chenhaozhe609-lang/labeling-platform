import { api } from './client'
import type { Organization } from '@/types'

// 平台超管：跨组织运营/排障。
export async function listOrgs(): Promise<Organization[]> {
  const { data } = await api.get<{ items: Organization[] }>('/platform/orgs')
  return data.items
}
