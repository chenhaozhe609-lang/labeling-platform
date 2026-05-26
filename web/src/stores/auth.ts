import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { Organization, User } from '@/types'

interface AuthState {
  token: string | null
  refreshToken: string | null
  user: User | null
  org: Organization | null // 当前组织（超管为 null）；顶栏显示组织名
  setAuth: (token: string, refreshToken: string, user: User, org?: Organization | null) => void
  logout: () => void
}

export const useAuth = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      refreshToken: null,
      user: null,
      org: null,
      setAuth: (token, refreshToken, user, org = null) => set({ token, refreshToken, user, org }),
      logout: () => set({ token: null, refreshToken: null, user: null, org: null }),
    }),
    { name: 'labeling-auth' },
  ),
)
