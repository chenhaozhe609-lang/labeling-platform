import { api } from './client'
import type { Invite, Role, User } from '@/types'

// ---- 用户管理（admin，C5.4b；多租户后限本组织）----
export async function listUsers(): Promise<User[]> {
  const { data } = await api.get<{ items: User[] }>('/admin/users')
  return data.items
}

export async function createUser(
  username: string,
  email: string,
  password: string,
  role: Role,
): Promise<User> {
  const { data } = await api.post<User>('/admin/users', { username, email, password, role })
  return data
}

export async function updateUser(id: number, patch: { role?: Role; password?: string }): Promise<void> {
  await api.patch(`/admin/users/${id}`, patch)
}

export async function deleteUser(id: number): Promise<void> {
  await api.delete(`/admin/users/${id}`)
}

export interface Dashboard {
  datasets: number
  pending: number
  claimed: number
  completed: number
  approved: number // 审核通过（累计）
  needs_redo: number // 审核打回（累计）；非 task 状态
  today_submitted: number
  active_today: number
  leaderboard: { user_id: number; username: string; count: number }[]
  activity: { username: string; task_id: number; dataset_id: number; at: string }[]
}

export async function getDashboard(): Promise<Dashboard> {
  const { data } = await api.get<Dashboard>('/admin/dashboard')
  return data
}

// ---- 邀请成员（admin，按本组织）----
export async function listInvites(): Promise<Invite[]> {
  const { data } = await api.get<{ items: Invite[] }>('/admin/invites')
  return data.items
}

export interface CreateInviteResponse {
  invite: Invite
  accept_path: string // 形如 /accept-invite?token=xxx
}

export async function createInvite(role: Role, email?: string): Promise<CreateInviteResponse> {
  const { data } = await api.post<CreateInviteResponse>('/admin/invites', { role, email: email || undefined })
  return data
}

export async function deleteInvite(id: number): Promise<void> {
  await api.delete(`/admin/invites/${id}`)
}
