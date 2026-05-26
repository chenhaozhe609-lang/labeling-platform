import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { ArrowLeft, Download, ListPlus, Loader2, Pause, PenLine, Play, RefreshCw, Settings2 } from 'lucide-react'
import { exportDataset, generateTasks, getDatasetDetail, pauseDataset, resumeDataset, syncDataset } from '@/api/datasets'
import { useAuth } from '@/stores/auth'

export function DatasetDetailPage() {
  const { id } = useParams<{ id: string }>()
  const dsId = Number(id)
  const role = useAuth((s) => s.user?.role)
  const isAdmin = role === 'admin'
  const canExport = role === 'admin' || role === 'reviewer'
  const [exporting, setExporting] = useState<'jsonl' | 'csv' | null>(null)
  const qc = useQueryClient()
  const fileRef = useRef<HTMLInputElement>(null)
  const { data, isLoading, error } = useQuery({
    queryKey: ['dataset', dsId],
    queryFn: () => getDatasetDetail(dsId),
  })

  const sync = useMutation({
    mutationFn: (file: File) => syncDataset(dsId, file),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: ['dataset', dsId] })
      qc.invalidateQueries({ queryKey: ['datasets'] })
      const b = d.batches[0]
      toast.success(`同步完成 · 新增 ${b?.new_task_count ?? 0} · 重标 ${b?.updated_task_count ?? 0}`)
    },
    onError: () => toast.error('同步失败'),
  })

  const gen = useMutation({
    mutationFn: () => generateTasks(dsId),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: ['dataset', dsId] })
      qc.invalidateQueries({ queryKey: ['datasets'] })
      toast.success(`已生成任务 · 待领 ${d.progress.pending}`)
    },
    onError: () => toast.error('生成任务失败（需先配置补全列）'),
  })

  const toggle = useMutation({
    mutationFn: (pause: boolean) => (pause ? pauseDataset(dsId) : resumeDataset(dsId)),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: ['dataset', dsId] })
      qc.invalidateQueries({ queryKey: ['datasets'] })
      toast.success(d.dataset.status === 'PAUSED' ? '已暂停' : '已恢复')
    },
    onError: () => toast.error('操作失败'),
  })

  async function doExport(format: 'jsonl' | 'csv') {
    setExporting(format)
    try {
      await exportDataset(dsId, format)
      toast.success(`已导出 ${format.toUpperCase()}`)
    } catch {
      toast.error('导出失败')
    } finally {
      setExporting(null)
    }
  }

  if (isLoading) return <Pad>加载中…</Pad>
  if (error || !data) return <Pad>加载失败</Pad>

  const { dataset: d, progress, batches } = data
  const total = d.total_rows || 0
  const pct = total > 0 ? Math.round((progress.completed / total) * 100) : 0
  const cols = d.form_schema?.columns ?? []
  const fillCols = cols.filter((c) => c.role === 'fill')
  const contextCols = cols.filter((c) => c.role === 'context')

  return (
    <div className="mx-auto max-w-3xl px-8 py-8">
      <Link to="/datasets" className="mb-5 inline-flex items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" />
        数据集
      </Link>

      <div className="mb-6 flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="mb-1.5 font-mono text-[11px] uppercase tracking-[0.18em] text-text-tertiary">DATASET</div>
          <h1 className="truncate font-serif text-[27px] leading-[1.1] tracking-tight">{d.name}</h1>
          <p className="mt-1.5 font-mono text-[12px] tabular text-text-tertiary">
            {d.source_schema}.{d.source_table} · pk={d.source_pk_column} · v{d.form_schema_version} · {d.status}
          </p>
        </div>
        <div className="flex shrink-0 gap-2">
          {isAdmin && (
            <>
              <input
                ref={fileRef}
                type="file"
                accept=".sql,.backup,.dump"
                className="hidden"
                onChange={(e) => {
                  const f = e.target.files?.[0]
                  if (f) sync.mutate(f)
                  e.target.value = ''
                }}
              />
              <button onClick={() => fileRef.current?.click()} disabled={sync.isPending} className="btn-ghost">
                {sync.isPending ? <Loader2 className="size-3.5 animate-spin" /> : <RefreshCw className="size-3.5" />}
                同步新版本
              </button>
              {(d.status === 'READY' || d.status === 'PAUSED') && (
                <button
                  onClick={() => toggle.mutate(d.status === 'READY')}
                  disabled={toggle.isPending}
                  className="btn-ghost"
                >
                  {toggle.isPending ? (
                    <Loader2 className="size-3.5 animate-spin" />
                  ) : d.status === 'READY' ? (
                    <Pause className="size-3.5" />
                  ) : (
                    <Play className="size-3.5" />
                  )}
                  {d.status === 'READY' ? '暂停' : '恢复'}
                </button>
              )}
              <Link to={`/datasets/${dsId}/schema`} className="btn-ghost">
                <Settings2 className="size-3.5" />
                编辑字段
              </Link>
            </>
          )}
          <Link to={`/workspace?dataset=${dsId}`} className="btn-primary">
            <PenLine className="size-3.5" />
            进入标注
          </Link>
        </div>
      </div>

      {/* 进度 */}
      <section className="mb-6 rounded-lg border border-border bg-card p-4">
        <div className="mb-1.5 flex items-center justify-between text-[13px]">
          <span className="text-muted-foreground">覆盖率</span>
          <span className="font-mono tabular">{pct}%</span>
        </div>
        <div className="mb-3 h-2 overflow-hidden rounded-full bg-surface-2">
          <div className="h-full rounded-full bg-success" style={{ width: `${pct}%` }} />
        </div>
        <div className="flex gap-4 font-mono text-[12px] tabular text-text-tertiary">
          <span>共 {total}</span>
          <span>已标 {progress.completed}</span>
          <span>待领 {progress.pending}</span>
          <span>进行 {progress.claimed}</span>
        </div>

        {canExport && progress.completed > 0 && (
          <div className="mt-3 flex items-center gap-2 border-t border-border-subtle pt-3">
            <span className="text-[12px] text-text-tertiary">导出补全后的表</span>
            <button onClick={() => doExport('jsonl')} disabled={exporting !== null} className="btn-ghost ml-auto">
              {exporting === 'jsonl' ? <Loader2 className="size-3.5 animate-spin" /> : <Download className="size-3.5" />}
              JSONL
            </button>
            <button onClick={() => doExport('csv')} disabled={exporting !== null} className="btn-ghost">
              {exporting === 'csv' ? <Loader2 className="size-3.5 animate-spin" /> : <Download className="size-3.5" />}
              CSV
            </button>
          </div>
        )}
      </section>

      {/* 列角色概览 */}
      <section className="mb-6 grid grid-cols-2 gap-4">
        <FieldBox title={`上下文列 · ${contextCols.length}`}>
          {contextCols.length === 0 ? (
            <li className="text-text-tertiary">无</li>
          ) : (
            contextCols.map((c) => (
              <li key={c.code} className="flex justify-between">
                <span>{c.label || c.code}</span>
                <span className="font-mono text-text-tertiary">{c.type}</span>
              </li>
            ))
          )}
        </FieldBox>
        <FieldBox title={`补全列 · ${fillCols.length}`}>
          {fillCols.length === 0 ? (
            <li className="text-text-tertiary">尚未配置{isAdmin && '，点「列与字段」勾选'}</li>
          ) : (
            fillCols.map((c) => (
              <li key={c.code} className="flex justify-between">
                <span>{c.label || c.code}</span>
                <span className="font-mono text-text-tertiary">{c.field?.kind ?? c.type}</span>
              </li>
            ))
          )}
        </FieldBox>
      </section>

      {isAdmin && fillCols.length > 0 && (
        <button onClick={() => gen.mutate()} disabled={gen.isPending} className="btn-ghost mb-6">
          {gen.isPending ? <Loader2 className="size-3.5 animate-spin" /> : <ListPlus className="size-3.5" />}
          生成任务（扫描有空补全列的行）
        </button>
      )}

      {/* 批次时间线 */}
      <section>
        <h2 className="mb-2 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">导入批次</h2>
        <div className="flex flex-col gap-2">
          {batches.map((b) => (
            <div key={b.id} className="flex items-center gap-3 rounded-md border border-border-subtle bg-card/50 px-3 py-2 text-[13px]">
              <span className="font-mono tabular text-text-tertiary">#{b.id}</span>
              <span className="truncate">{b.file_name ?? '—'}</span>
              <span className="ml-auto flex gap-2 font-mono text-[12px] tabular">
                <span className="text-success">+{b.new_task_count}</span>
                {b.updated_task_count > 0 && <span className="text-warning">~{b.updated_task_count}</span>}
              </span>
              {b.error && <span className="text-destructive" title={b.error}>失败</span>}
            </div>
          ))}
        </div>
      </section>
    </div>
  )
}

function FieldBox({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="mb-2 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">{title}</div>
      <ul className="flex flex-col gap-1.5 text-[13px]">{children}</ul>
    </div>
  )
}

function Pad({ children }: { children: React.ReactNode }) {
  return <div className="px-8 py-8 text-sm text-muted-foreground">{children}</div>
}
