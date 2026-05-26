import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { CheckCircle2, Clock, PenLine } from 'lucide-react'
import { getMyTasks } from '@/api/tasks'
import { PageHeader } from '@/components/PageHeader'
import type { ReviewStatus } from '@/types'

const REVIEW_LABEL: Record<ReviewStatus, { text: string; cls: string }> = {
  approved: { text: '已通过', cls: 'bg-success/15 text-success' },
  needs_redo: { text: '已打回', cls: 'bg-destructive/15 text-destructive' },
}

export function MyTasksPage() {
  const { data, isLoading, error } = useQuery({ queryKey: ['my-tasks'], queryFn: getMyTasks, refetchInterval: 20000 })

  if (isLoading) return <Pad>加载中…</Pad>
  if (error || !data) return <Pad>加载失败</Pad>

  return (
    <div className="mx-auto max-w-3xl px-8 py-8">
      <PageHeader eyebrow="MY TASKS" title="我的任务" />

      {/* 进行中（CLAIMED 给我，未提交） */}
      <section className="mb-6">
        <div className="mb-2 flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">
          <Clock className="size-3.5" />进行中（{data.in_progress.length}）
        </div>
        {data.in_progress.length === 0 ? (
          <div className="rounded-lg border border-border-subtle bg-card/50 px-4 py-3 text-[13px] text-text-tertiary">
            没有进行中的任务。去
            <Link to="/workspace" className="mx-1 text-primary hover:underline">标注台</Link>
            领取下一条。
          </div>
        ) : (
          <div className="flex flex-col gap-2">
            {data.in_progress.map((t) => (
              <div key={t.task_id} className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-2.5 text-[13px]">
                <span className="font-mono tabular text-text-tertiary">#{t.task_id}</span>
                <span className="truncate">{t.dataset_name}</span>
                <span className="font-mono text-[12px] text-text-tertiary">pk={t.source_row_pk}</span>
                <Link to={`/workspace?dataset=${t.dataset_id}`} className="btn-primary ml-auto">
                  <PenLine className="size-3.5" />继续
                </Link>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* 已完成（我提交、当前有效的标注） */}
      <section>
        <div className="mb-2 flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">
          <CheckCircle2 className="size-3.5" />已完成（{data.completed.length}）
        </div>
        {data.completed.length === 0 ? (
          <div className="rounded-lg border border-border-subtle bg-card/50 px-4 py-3 text-[13px] text-text-tertiary">还没有已完成的标注。</div>
        ) : (
          <div className="overflow-hidden rounded-lg border border-border bg-card">
            <table className="w-full text-[13px]">
              <thead>
                <tr className="border-b border-border-subtle text-left text-[11px] uppercase tracking-wide text-text-tertiary">
                  <th className="px-4 py-2 font-medium">任务</th>
                  <th className="px-4 py-2 font-medium">数据集</th>
                  <th className="px-4 py-2 font-medium">轮次</th>
                  <th className="px-4 py-2 font-medium">提交时间</th>
                  <th className="px-4 py-2 text-right font-medium">审核</th>
                </tr>
              </thead>
              <tbody>
                {data.completed.map((t) => {
                  const rv = t.review_status ? REVIEW_LABEL[t.review_status] : null
                  return (
                    <tr key={`${t.task_id}-${t.round}`} className="border-t border-border-subtle">
                      <td className="px-4 py-2">
                        <Link to={`/datasets/${t.dataset_id}`} className="font-mono tabular text-text-tertiary hover:text-foreground">#{t.task_id}</Link>
                        <span className="ml-2 font-mono text-[12px] text-text-tertiary">pk={t.source_row_pk}</span>
                      </td>
                      <td className="px-4 py-2"><span className="truncate">{t.dataset_name}</span></td>
                      <td className="px-4 py-2 font-mono tabular text-text-tertiary">r{t.round}</td>
                      <td className="px-4 py-2 text-text-tertiary">{relTime(t.created_at)}</td>
                      <td className="px-4 py-2 text-right">
                        {rv ? (
                          <span className={`rounded px-1.5 py-0.5 text-[11px] ${rv.cls}`}>{rv.text}</span>
                        ) : (
                          <span className="text-[11px] text-text-tertiary">待审</span>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  )
}

function relTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const m = Math.floor(diff / 60000)
  if (m < 1) return '刚刚'
  if (m < 60) return `${m} 分钟前`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h} 小时前`
  return `${Math.floor(h / 24)} 天前`
}

function Pad({ children }: { children: React.ReactNode }) {
  return <div className="px-8 py-8 text-sm text-muted-foreground">{children}</div>
}
