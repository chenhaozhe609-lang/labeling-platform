import { useEffect } from 'react'
import { heartbeat } from '@/api/tasks'

/** 每 60s 给当前任务续约；成功后回调最新 lease 到期时间。 */
export function useHeartbeat(
  taskId: number | null,
  enabled: boolean,
  onLease: (expiresAt: string) => void,
) {
  useEffect(() => {
    if (!enabled || !taskId) return
    const id = window.setInterval(async () => {
      try {
        const { lease_expires_at } = await heartbeat(taskId)
        onLease(lease_expires_at)
      } catch {
        /* 失败由 submit/claim 时的冲突处理兜底 */
      }
    }, 60_000)
    return () => window.clearInterval(id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [taskId, enabled])
}
