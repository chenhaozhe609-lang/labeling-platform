import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface SettingsState {
  shortcuts: boolean // 标注键盘快捷键开关
  density: 'compact' | 'cozy'
  setShortcuts: (v: boolean) => void
  setDensity: (d: 'compact' | 'cozy') => void
}

export const useSettings = create<SettingsState>()(
  persist(
    (set) => ({
      shortcuts: true,
      density: 'compact',
      setShortcuts: (v) => set({ shortcuts: v }),
      setDensity: (d) => {
        document.documentElement.dataset.density = d
        set({ density: d })
      },
    }),
    { name: 'labeling-settings' },
  ),
)
