import { LayoutDashboard } from 'lucide-react'

export function DashboardPage() {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 text-center">
      <LayoutDashboard className="size-9 text-text-tertiary" />
      <div className="text-lg font-semibold">进度看板</div>
      <p className="text-sm text-muted-foreground">覆盖率、吞吐与产能图表 · 开发中</p>
    </div>
  )
}
