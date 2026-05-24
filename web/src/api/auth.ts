import { api } from './client'
import type { User } from '@/types'

interface LoginResponse {
  access_token: string
  refresh_token: string
  user: User
}

export async function login(username: string, password: string): Promise<LoginResponse> {
  const { data } = await api.post<LoginResponse>('/auth/login', { username, password })
  return data
}
