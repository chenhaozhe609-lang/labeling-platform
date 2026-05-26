import { create } from 'zustand'

export type ThemePref = 'system' | 'light' | 'dark'

const KEY = 'theme'

function readPref(): ThemePref {
  try {
    const t = localStorage.getItem(KEY)
    return t === 'light' || t === 'dark' ? t : 'system'
  } catch {
    return 'system'
  }
}

function resolveDark(pref: ThemePref): boolean {
  if (pref === 'light') return false
  if (pref === 'dark') return true
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function applyTheme(pref: ThemePref) {
  document.documentElement.classList.toggle('dark', resolveDark(pref))
}

interface ThemeState {
  pref: ThemePref
  setPref: (p: ThemePref) => void
}

/**
 * 主题偏好。用裸 localStorage 'theme' 键，与 index.html 的 pre-paint 脚本保持一致：
 * 'light'/'dark' 显式存储；'system' 删除该键，脚本回落到 prefers-color-scheme。
 */
export const useTheme = create<ThemeState>((set) => ({
  pref: readPref(),
  setPref: (p) => {
    try {
      if (p === 'system') localStorage.removeItem(KEY)
      else localStorage.setItem(KEY, p)
    } catch {
      /* ignore */
    }
    applyTheme(p)
    set({ pref: p })
  },
}))

// system 模式下，跟随操作系统主题的实时变化
if (typeof window !== 'undefined') {
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (readPref() === 'system') applyTheme('system')
  })
}
