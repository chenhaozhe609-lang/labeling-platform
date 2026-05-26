import { api } from './client'
import type { Organization, Role, User } from '@/types'

export interface AuthResponse {
  access_token: string
  refresh_token: string
  user: User
  org?: Organization | null
}

// 开放注册：建组织 + owner(admin)。
export async function signup(input: {
  org_name: string
  email: string
  username: string
  password: string
}): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>('/auth/signup', input)
  return data
}

// 按邮箱登录。
export async function login(email: string, password: string): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>('/auth/login', { email, password })
  return data
}

// refresh：换新 token（后端校验 token_version；失效返回 401）。
export async function refresh(refreshToken: string): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>('/auth/refresh', { refresh_token: refreshToken })
  return data
}

// 吊销本人所有会话（bump token_version）。
export async function logoutAll(): Promise<void> {
  await api.post('/auth/logout-all')
}

// 凭邀请 token 加入既有组织（角色取自邀请）。
export async function acceptInvite(input: {
  token: string
  email: string
  username: string
  password: string
}): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>('/auth/accept-invite', input)
  return data
}

export type { Role }
