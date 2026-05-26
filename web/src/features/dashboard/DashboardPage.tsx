import { useQuery } from '@tanstack/react-query'
import { getDashboard } from '@/api/admin'
import { PageHeader } from '@/components/PageHeader'
import { ActivityFeed, Donut, Leaderboard, Legend, SegmentedProgress, StatCard, type Segment } from '@/components/viz'

export function DashboardPage() {
  const { data, isLoading, error } = useQuery({ queryKey: ['dashboard'], queryFn: getDashboard, refetchInterval: 15000 })

  if (isLoading) return <Pad>加载中…</Pad>
  if (error || !data) return <Pad>看板加载失败（需管理员）</Pad>

  // 任务分布饼图只含真实 task 状态；「打回」是审核动作（任务已回到「待领」），单列指标展示。
  const segments: Segment[] = [
    { value: data.completed, color: 'var(--success)', label: '已完成' },
    { value: data.claimed, color: 'var(--info)', label: '进行中' },
    { value: data.pending, color: 'var(--surface-3)', label: '待领' },
  ].filter((s) => s.value > 0)
  const totalTasks = data.completed + data.claimed + data.pending

  return (
    <div className="mx-auto max-w-5xl px-8 py-8">
      <PageHeader eyebrow="OVERVIEW" title="总览" description="本组织全部数据集的汇总进度（非单个数据集）" />

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        {/* 完成度 */}
        <div className="rounded-lg border border-border bg-card p-5 lg:col-span-2">
          <div className="flex items-center gap-6">
            <Donut segments={segments.length ? segments : [{ value: 1, color: 'var(--surface-3)', label: '无' }]} />
            <div className="flex-1">
              <div className="mb-1 text-[12px] text-text-tertiary">总任务</div>
              <div className="mb-3 font-mono text-2xl font-semibold tabular">{totalTasks.toLocaleString()}</div>
              <SegmentedProgress segments={segments} height={8} />
              <div className="mt-3">
                <Legend segments={segments} />
              </div>
            </div>
          </div>
        </div>

        {/* 今日 + 审核 */}
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-1">
          <StatCard title="今日提交" value={data.today_submitted} unit="条" />
          <StatCard title="今日活跃标注员" value={data.active_today} unit="人" sub={`${data.datasets} 个数据集`} />
          <StatCard
            title="审核"
            value={data.approved}
            unit="通过"
            sub={
              <span className={data.needs_redo > 0 ? 'text-destructive' : undefined}>
                打回 {data.needs_redo}
              </span>
            }
          />
        </div>

        {/* 排行榜 */}
        <div className="rounded-lg border border-border bg-card p-4 lg:col-span-2">
          <div className="mb-3 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">标注员排行榜</div>
          <Leaderboard rows={data.leaderboard} />
        </div>

        {/* 动态 */}
        <div className="rounded-lg border border-border bg-card p-4">
          <div className="mb-3 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">实时动态</div>
          <ActivityFeed items={data.activity} />
        </div>
      </div>
    </div>
  )
}

function Pad({ children }: { children: React.ReactNode }) {
  return <div className="px-8 py-8 text-sm text-muted-foreground">{children}</div>
}
