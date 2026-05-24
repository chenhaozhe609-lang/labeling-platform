import { useEffect, useRef, useState } from 'react'

type Draft = Record<string, unknown> // fill 列 → 值

const key = (taskId: number) => `draft:${taskId}`

export function loadDraft(taskId: number): Draft | null {
  try {
    const raw = localStorage.getItem(key(taskId))
    return raw ? (JSON.parse(raw) as Draft) : null
  } catch {
    return null
  }
}

export function clearDraft(taskId: number): void {
  try {
    localStorage.removeItem(key(taskId))
  } catch {
    /* ignore */
  }
}

export type SaveState = 'idle' | 'editing' | 'saving' | 'saved' | 'restored'

/** 监听 values 变化，debounce 写入 localStorage，并暴露给 AutosaveIndicator 的状态 */
export function useAutosave(taskId: number, values: Draft, dirty: boolean): SaveState {
  const [state, setState] = useState<SaveState>('idle')
  const timer = useRef<number | undefined>(undefined)

  useEffect(() => {
    if (!dirty) return
    setState('editing')
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => {
      setState('saving')
      try {
        localStorage.setItem(key(taskId), JSON.stringify(values))
      } catch {
        /* ignore */
      }
      window.setTimeout(() => setState('saved'), 200)
    }, 600)
    return () => window.clearTimeout(timer.current)
  }, [taskId, values, dirty])

  return state
}
