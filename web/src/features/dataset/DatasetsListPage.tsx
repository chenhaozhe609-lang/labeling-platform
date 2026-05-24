import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { listDatasets } from '@/api/datasets'
import { useAuth } from '@/stores/auth'
import { cn } from '@/lib/utils'
import type { DatasetListItem, DatasetStatus } from '@/types'

const STATUS: Record<DatasetStatus, { label: string; dot: string }> = {
  READY: { label: '就绪', dot: 'bg-success' },
  IMPORTING: { label: '导入中', dot: 'bg-primary animate-pulse' },
  PAUSED: { label: '已暂停', dot: 'bg-muted-foreground' },
  DONE: { label: '已完成', dot: 'bg-ai' },
  FAILED: { label: '失败', dot: 'bg-destructive' },
}

export function DatasetsListPage() {
  const isAdmin = useAuth((s) => s.user?.role === 'admin')
  const { data, isLoading, error } = useQuery({ queryKey: ['datasets'], queryFn: listDatasets })

  return (
    <div className="mx-auto max-w-5xl px-8 py-8">
      <header className="mb-6 flex items-center justify-between">
        <h1 className="text-xl font-semibold tracking-tight">数据集</h1>
        {isAdmin && (
          <Link
            to="/datasets/upload"
            className="inline-flex items-center gap-1.5 rounded-md bg-primary px-3 py-1.5 text-[13px] font-medium text-primary-foreground transition-opacity hover:opacity-90"
          >
            <Plus className="size-4" />
            新建数据集
          </Link>
        )}
      </header>

      {isLoading && <p className="text-sm text-muted-foreground">加载中…</p>}
      {error && <p className="text-sm text-destructive">加载失败</p>}
      {data && data.length === 0 && (
        <div className="rounded-lg border border-border-subtle bg-card/50 p-10 text-center text-sm text-muted-foreground">
          还没有数据集{isAdmin && '，点右上角「新建数据集」上传一个 dump'}
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {data?.map((d) => (
          <DatasetCard key={d.id} d={d} />
        ))}
      </div>
    </div>
  )
}

function DatasetCard({ d }: { d: DatasetListItem }) {
  const s = STATUS[d.status]
  const pct = d.total_rows > 0 ? Math.round((d.completed / d.total_rows) * 100) : 0
  return (
    <Link
      to={`/datasets/${d.id}`}
      className="block rounded-lg border border-border bg-card p-4 transition-colors hover:border-border-strong"
    >
      <div className="mb-3 flex items-center gap-2 text-[12px] text-muted-foreground">
        <span className={cn('size-2 rounded-full', s.dot)} />
        {s.label}
        <span className="ml-auto font-mono tabular">v{d.form_schema_version}</span>
      </div>
      <div className="mb-3 truncate text-[15px] font-medium">{d.name}</div>

      <div className="mb-1.5 flex items-center justify-between text-[12px]">
        <span className="text-text-tertiary">覆盖率</span>
        <span className="font-mono tabular text-muted-foreground">{pct}%</span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-surface-2">
        <div className="h-full rounded-full bg-success" style={{ width: `${pct}%` }} />
      </div>

      <div className="mt-3 flex gap-3 font-mono text-[12px] tabular text-text-tertiary">
        <span>{d.completed}/{d.total_rows} 已标</span>
        <span className="ml-auto">待 {d.pending} · 进行 {d.claimed}</span>
      </div>
    </Link>
  )
}
