import { useQuery } from '@tanstack/react-query'
import { getDashboard } from '@/api/admin'
import { ActivityFeed, Donut, Leaderboard, Legend, SegmentedProgress, StatCard, type Segment } from '@/components/viz'

export function DashboardPage() {
  const { data, isLoading, error } = useQuery({ queryKey: ['dashboard'], queryFn: getDashboard, refetchInterval: 15000 })

  if (isLoading) return <Pad>加载中…</Pad>
  if (error || !data) return <Pad>看板加载失败（需管理员）</Pad>

  const segments: Segment[] = [
    { value: data.completed, color: 'var(--success)', label: '已完成' },
    { value: data.claimed, color: 'var(--info)', label: '进行中' },
    { value: data.pending, color: 'var(--surface-3)', label: '待领' },
    { value: data.needs_redo, color: 'var(--destructive)', label: '打回' },
  ].filter((s) => s.value > 0)
  const totalTasks = data.completed + data.claimed + data.pending + data.needs_redo

  return (
    <div className="mx-auto max-w-5xl px-8 py-8">
      <h1 className="mb-6 text-xl font-semibold tracking-tight">总览</h1>

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

        {/* 今日 */}
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-1">
          <StatCard title="今日提交" value={data.today_submitted} unit="条" />
          <StatCard title="今日活跃标注员" value={data.active_today} unit="人" sub={`${data.datasets} 个数据集`} />
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
