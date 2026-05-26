import { describe, it, expect, beforeEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { loadDraft, clearDraft, useAutosave } from './useDraft'

// E12：草稿自动保存 / 恢复（PRD §16，标注台断网不丢稿）。
beforeEach(() => localStorage.clear())

describe('草稿读写', () => {
  it('loadDraft 在无草稿时返回 null', () => {
    expect(loadDraft(1)).toBeNull()
  })

  it('loadDraft 在脏 JSON 时容错返回 null', () => {
    localStorage.setItem('draft:1', '{不是合法json')
    expect(loadDraft(1)).toBeNull()
  })

  it('clearDraft 删除指定任务草稿', () => {
    localStorage.setItem('draft:9', JSON.stringify({ a: 1 }))
    clearDraft(9)
    expect(loadDraft(9)).toBeNull()
  })
})

describe('useAutosave', () => {
  it('debounce 写入后可被 loadDraft 恢复，状态流转 editing→saving→saved', () => {
    vi.useFakeTimers()
    try {
      const vals = { discipline: 'engineering', difficulty: 4 }
      const { result } = renderHook(({ v, d }) => useAutosave(7, v, d), {
        initialProps: { v: vals, d: true },
      })
      expect(result.current).toBe('editing')

      act(() => vi.advanceTimersByTime(600)) // 过 debounce → 写入 + saving
      expect(result.current).toBe('saving')
      expect(loadDraft(7)).toEqual(vals) // 草稿恢复：写入的值能读回

      act(() => vi.advanceTimersByTime(200)) // saving → saved
      expect(result.current).toBe('saved')
    } finally {
      vi.useRealTimers()
    }
  })

  it('dirty=false 不写草稿', () => {
    vi.useFakeTimers()
    try {
      renderHook(() => useAutosave(8, { a: 1 }, false))
      act(() => vi.advanceTimersByTime(1000))
      expect(loadDraft(8)).toBeNull()
    } finally {
      vi.useRealTimers()
    }
  })
})
