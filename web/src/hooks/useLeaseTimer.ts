import { useCallback, useEffect, useState } from 'react'

export type LeaseState = 'healthy' | 'warning' | 'critical' | 'expired'

function format(sec: number): string {
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

/** 租约倒计时。resetKey 变化（切到新任务）时重置。 */
export function useLeaseTimer(durationMin: number, resetKey: unknown) {
  const total = durationMin * 60
  const [remaining, setRemaining] = useState(total)

  useEffect(() => {
    setRemaining(total)
  }, [resetKey, total])

  useEffect(() => {
    const id = window.setInterval(() => {
      setRemaining((r) => (r > 0 ? r - 1 : 0))
    }, 1000)
    return () => window.clearInterval(id)
  }, [])

  const extend = useCallback(() => setRemaining(total), [total])

  const state: LeaseState =
    remaining <= 0 ? 'expired' : remaining <= 60 ? 'critical' : remaining <= 300 ? 'warning' : 'healthy'

  return { mmss: format(remaining), state, remaining, extend }
}
