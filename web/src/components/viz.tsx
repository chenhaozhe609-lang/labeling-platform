import { cn } from '@/lib/utils'

export interface Segment {
  value: number
  color: string // CSS color
  label: string
}

/** 完成度环（SVG 多段甜甜圈）。 */
export function Donut({ size = 132, stroke = 16, segments }: { size?: number; stroke?: number; segments: Segment[] }) {
  const total = segments.reduce((a, s) => a + s.value, 0) || 1
  const r = (size - stroke) / 2
  const c = 2 * Math.PI * r
  let offset = 0
  const pct = ((segments[0]?.value ?? 0) / total) * 100
  return (
    <div className="relative" style={{ width: size, height: size }}>
      <svg width={size} height={size}>
        <circle cx={size / 2} cy={size / 2} r={r} stroke="var(--surface-2)" strokeWidth={stroke} fill="none" />
        {segments.map((s, i) => {
          const len = (s.value / total) * c
          const el = (
            <circle
              key={i}
              cx={size / 2}
              cy={size / 2}
              r={r}
              stroke={s.color}
              strokeWidth={stroke}
              fill="none"
              strokeDasharray={`${len} ${c - len}`}
              strokeDashoffset={-offset}
              transform={`rotate(-90 ${size / 2} ${size / 2})`}
            />
          )
          offset += len
          return el
        })}
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <div className="font-mono text-2xl font-semibold tabular text-foreground">{pct.toFixed(0)}%</div>
        <div className="text-[11px] text-text-tertiary">完成度</div>
      </div>
    </div>
  )
}

/** 多状态分段进度条。 */
export function SegmentedProgress({ segments, height = 8 }: { segments: Segment[]; height?: number }) {
  const total = segments.reduce((a, s) => a + s.value, 0) || 1
  return (
    <div className="flex w-full overflow-hidden rounded-full bg-surface-2" style={{ height }}>
      {segments.map((s, i) => (
        <div key={i} title={`${s.label}: ${s.value}`} style={{ width: `${(s.value / total) * 100}%`, background: s.color }} />
      ))}
    </div>
  )
}

export function Legend({ segments }: { segments: Segment[] }) {
  return (
    <div className="flex flex-wrap gap-x-4 gap-y-1.5 text-[12px] text-muted-foreground">
      {segments.map((s, i) => (
        <span key={i} className="flex items-center gap-1.5">
          <span className="size-2.5 rounded-sm" style={{ background: s.color }} />
          {s.label} <span className="font-mono tabular text-text-tertiary">{s.value}</span>
        </span>
      ))}
    </div>
  )
}

export function StatCard({ title, value, unit, sub }: { title: string; value: string | number; unit?: string; sub?: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="text-[12px] text-text-tertiary">{title}</div>
      <div className="mt-1 font-mono text-2xl font-semibold tabular text-foreground">
        {value}
        {unit && <span className="ml-1 text-sm font-normal text-muted-foreground">{unit}</span>}
      </div>
      {sub && <div className="mt-1 text-[12px] text-muted-foreground">{sub}</div>}
    </div>
  )
}

export function Leaderboard({ rows }: { rows: { user_id: number; username: string; count: number }[] }) {
  return (
    <table className="w-full text-[13px]">
      <thead>
        <tr className="text-left text-[11px] uppercase tracking-wide text-text-tertiary">
          <th className="pb-2 font-medium">#</th>
          <th className="pb-2 font-medium">成员</th>
          <th className="pb-2 text-right font-medium">已标</th>
        </tr>
      </thead>
      <tbody>
        {rows.length === 0 && (
          <tr><td colSpan={3} className="py-3 text-text-tertiary">暂无数据</td></tr>
        )}
        {rows.map((r, i) => (
          <tr key={r.user_id} className="border-t border-border-subtle">
            <td className="py-1.5">
              <span className={cn('inline-grid size-5 place-items-center rounded font-mono text-[11px]', i === 0 ? 'bg-warning/20 text-warning' : 'text-text-tertiary')}>{i + 1}</span>
            </td>
            <td className="py-1.5">
              <span className="inline-flex items-center gap-2">
                <span className="grid size-5 place-items-center rounded-full bg-surface-2 text-[10px] uppercase">{r.username[0]}</span>
                {r.username}
              </span>
            </td>
            <td className="py-1.5 text-right font-mono tabular">{r.count}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

export function ActivityFeed({ items }: { items: { username: string; task_id: number; at: string }[] }) {
  return (
    <div className="flex flex-col gap-2 text-[13px]">
      {items.length === 0 && <div className="text-text-tertiary">暂无动态</div>}
      {items.map((a, i) => (
        <div key={i} className="flex items-center gap-2">
          <span className="size-1.5 rounded-full bg-success" />
          <span className="text-muted-foreground">
            <b className="font-medium text-foreground">{a.username}</b> 提交了 <span className="font-mono text-text-tertiary">#{a.task_id}</span>
          </span>
          <span className="ml-auto text-[11px] text-text-tertiary">{relTime(a.at)}</span>
        </div>
      ))}
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
