import axios from 'axios'
import { useAuth } from '@/stores/auth'

// baseURL '/api' 经 Vite 代理转发到后端（见 vite.config.ts）
export const api = axios.create({ baseURL: '/api' })

api.interceptors.request.use((cfg) => {
  const token = useAuth.getState().token
  if (token) cfg.headers.Authorization = `Bearer ${token}`
  return cfg
})

api.interceptors.response.use(
  (r) => r,
  (err) => {
    if (err.response?.status === 401) {
      useAuth.getState().logout() // 触发路由 guard 跳登录
    }
    return Promise.reject(err)
  },
)
