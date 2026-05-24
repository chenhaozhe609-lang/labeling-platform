import { useEffect, useState } from 'react'

export type LeaseState = 'healthy' | 'warning' | 'critical' | 'expired'

function format(sec: number): string {
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

/** 基于服务端返回的绝对到期时间倒计时（每秒刷新）。 */
export function useLeaseTimer(expiresAt: string | null) {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(id)
  }, [])

  const remaining = expiresAt
    ? Math.max(0, Math.floor((new Date(expiresAt).getTime() - now) / 1000))
    : 0

  const state: LeaseState =
    !expiresAt || remaining <= 0
      ? expiresAt
        ? 'expired'
        : 'healthy'
      : remaining <= 60
        ? 'critical'
        : remaining <= 300
          ? 'warning'
          : 'healthy'

  return { mmss: format(remaining), state, remaining }
}
