import { api } from './client'

export interface Dashboard {
  datasets: number
  pending: number
  claimed: number
  completed: number
  needs_redo: number
  today_submitted: number
  active_today: number
  leaderboard: { user_id: number; username: string; count: number }[]
  activity: { username: string; task_id: number; dataset_id: number; at: string }[]
}

export async function getDashboard(): Promise<Dashboard> {
  const { data } = await api.get<Dashboard>('/admin/dashboard')
  return data
}
