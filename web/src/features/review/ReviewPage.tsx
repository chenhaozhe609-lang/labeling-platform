import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import {
  Check,
  ChevronDown,
  Loader2,
  LogOut,
  PartyPopper,
  RotateCcw,
  Sparkles,
  User,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Textarea } from '@/components/ui/textarea'
import { Kbd } from '@/components/Kbd'
import { cn } from '@/lib/utils'
import type { ColumnSpec, DatasetListItem, FormSchema, ReviewItem, ReviewStatus } from '@/types'
import { listDatasets } from '@/api/tasks'
import { decideReview, getReviewQueue } from '@/api/reviews'
import { useSettings } from '@/stores/settings'

type Phase = 'loading' | 'ready' | 'empty' | 'nodataset'

export function ReviewPage() {
  const nav = useNavigate()
  const [searchParams] = useSearchParams()
  const shortcutsEnabled = useSettings((s) => s.shortcuts)

  const [phase, setPhase] = useState<Phase>('loading')
  const [datasets, setDatasets] = useState<DatasetListItem[]>([])
  const [dataset, setDataset] = useState<DatasetListItem | null>(null)
  const [datasetName, setDatasetName] = useState('')
  const [schema, setSchema] = useState<FormSchema | null>(null)
  const [pendingTotal, setPendingTotal] = useState(0)
  const [queue, setQueue] = useState<ReviewItem[]>([])
  const [idx, setIdx] = useState(0)
  const [note, setNote] = useState('')
  const [deciding, setDeciding] = useState(false)

  const current = queue[idx] ?? null

  const loadQueue = useCallback(async (datasetId: number) => {
    setPhase('loading')
    try {
      const res = await getReviewQueue(datasetId)
      setDatasetName(res.dataset_name)
      setSchema(res.form_schema)
      setPendingTotal(res.pending_total)
      setQueue(res.items)
      setIdx(0)
      setNote('')
      setPhase(res.items.length ? 'ready' : 'empty')
    } catch {
      toast.error('加载审核队列失败')
      setPhase('empty')
    }
  }, [])

  // 首次：选数据集（?dataset= 优先，否则首个有已完成任务的）→ 拉队列
  useEffect(() => {
    void (async () => {
      try {
        const list = await listDatasets()
        setDatasets(list)
        const want = searchParams.get('dataset')
        const ds =
          (want ? list.find((d) => String(d.id) === want) : undefined) ??
          list.find((d) => d.completed > 0) ??
          list[0]
        if (!ds) {
          setPhase('nodataset')
          return
        }
        setDataset(ds)
        await loadQueue(ds.id)
      } catch {
        setPhase('nodataset')
      }
    })()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function select(i: number) {
    setIdx(Math.min(Math.max(i, 0), Math.max(queue.length - 1, 0)))
    setNote('') // 切到别的标注时清空上一条的备注
  }
  function move(delta: number) {
    select(idx + delta)
  }

  async function decide(status: ReviewStatus) {
    if (!current || deciding) return
    setDeciding(true)
    const decided = current
    try {
      await decideReview(decided.annotation_id, status, status === 'needs_redo' ? note : undefined)
      toast.success(status === 'approved' ? `已通过 · #${decided.task_id}` : `已打回重标 · #${decided.task_id}`)
      setPendingTotal((n) => Math.max(n - 1, 0))
      // 从本地队列移除，自动推进到下一条
      const next = queue.filter((it) => it.annotation_id !== decided.annotation_id)
      setQueue(next)
      setIdx((i) => Math.min(i, Math.max(next.length - 1, 0)))
      setNote('')
      if (next.length === 0 && dataset) void loadQueue(dataset.id) // 抽下一批
    } catch (e) {
      const httpStatus = (e as { response?: { status?: number } }).response?.status
      if (httpStatus === 409) {
        toast.error('该标注已被其他审核员处理，已跳过')
        setQueue((q) => q.filter((it) => it.annotation_id !== decided.annotation_id))
      } else if (httpStatus === 403) {
        toast.error('不能审核本人提交的标注')
      } else {
        toast.error('裁决失败，请重试')
      }
    } finally {
      setDeciding(false)
    }
  }

  // 键盘：A 通过 / R 打回 / J·K 切换（备注聚焦时仅 Esc 失焦）
  const keyHandler = (e: KeyboardEvent) => {
    if (phase !== 'ready' || !current || !shortcutsEnabled) return
    const el = document.activeElement as HTMLElement | null
    const inText = el?.tagName === 'TEXTAREA' || el?.tagName === 'INPUT'
    if (inText) {
      if (e.key === 'Escape') el?.blur()
      return
    }
    if (e.metaKey || e.ctrlKey || e.altKey) return
    switch (e.key) {
      case 'a':
      case 'A':
        e.preventDefault()
        void decide('approved')
        break
      case 'r':
      case 'R':
        e.preventDefault()
        void decide('needs_redo')
        break
      case 'j':
      case 'J':
      case 'ArrowDown':
        e.preventDefault()
        move(1)
        break
      case 'k':
      case 'K':
      case 'ArrowUp':
        e.preventDefault()
        move(-1)
        break
      case 'n':
      case 'N':
        e.preventDefault()
        document.getElementById('review-note')?.focus()
        break
    }
  }
  const handlerRef = useRef(keyHandler)
  useEffect(() => {
    handlerRef.current = keyHandler // 每次渲染后同步最新闭包（latest-ref 模式）
  })
  useEffect(() => {
    const fn = (e: KeyboardEvent) => handlerRef.current(e)
    window.addEventListener('keydown', fn)
    return () => window.removeEventListener('keydown', fn)
  }, [])

  if (phase === 'loading') {
    return (
      <div className="flex h-svh items-center justify-center bg-background text-muted-foreground">
        <Loader2 className="mr-2 size-5 animate-spin" />
        正在加载抽检队列…
      </div>
    )
  }
  if (phase === 'nodataset') {
    return <Center title="暂无可审数据集" desc="请等待标注员提交标注后再来抽检。" onBack={() => nav('/datasets')} />
  }

  const contextCols = schema?.columns.filter((c) => c.role === 'context') ?? []
  const fillCols = schema?.columns.filter((c) => c.role === 'fill') ?? []
  const primarySet = new Set(schema?.primary_cols ?? [])

  return (
    <div className="flex h-svh flex-col bg-background text-foreground">
      <header className="flex h-12 shrink-0 items-center gap-3 border-b border-border px-4 text-[13px]">
        <span className="font-medium">{datasetName || dataset?.name || '审核'}</span>
        <Sep />
        <Badge variant="secondary" className="gap-1 font-normal">抽检</Badge>
        <span className="text-muted-foreground">
          待审 <span className="font-mono tabular text-foreground">{pendingTotal}</span>
        </span>
        <DatasetPicker
          datasets={datasets}
          onPick={(d) => {
            setDataset(d)
            void loadQueue(d.id)
          }}
        />
        <div className="ml-auto flex items-center gap-2">
          {phase === 'ready' && (
            <span className="font-mono tabular text-text-tertiary">
              {idx + 1}/{queue.length}
            </span>
          )}
          <Button variant="ghost" size="sm" onClick={() => nav('/datasets')}>
            <LogOut className="size-3.5" />退出
          </Button>
        </div>
      </header>

      {phase === 'empty' || !current ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-4 text-center">
          <PartyPopper className="size-10 text-success" />
          <div>
            <div className="text-lg font-semibold">这批抽检已审完</div>
            <div className="mt-1 text-sm text-muted-foreground">{datasetName || dataset?.name}</div>
          </div>
          {dataset && (
            <Button variant="secondary" size="sm" onClick={() => loadQueue(dataset.id)}>
              <RotateCcw className="size-3.5" />再抽一批
            </Button>
          )}
        </div>
      ) : (
        <div className="flex min-h-0 flex-1">
          {/* 队列栏 */}
          <aside className="flex w-[260px] shrink-0 flex-col overflow-y-auto border-r border-border p-2">
            <div className="px-2 py-1.5 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">
              本批队列（{queue.length}）
            </div>
            {queue.map((it, i) => (
              <button
                key={it.annotation_id}
                onClick={() => select(i)}
                className={cn(
                  'flex flex-col gap-0.5 rounded-md px-2.5 py-2 text-left text-[13px] transition-colors',
                  i === idx ? 'bg-surface-3 text-foreground' : 'text-muted-foreground hover:bg-surface-3/60',
                )}
              >
                <span className="flex items-center gap-1.5">
                  <span className="font-mono tabular text-text-tertiary">#{it.task_id}</span>
                  {it.round > 1 && <Badge variant="secondary" className="h-4 px-1 text-[10px] font-normal">r{it.round}</Badge>}
                </span>
                <span className="flex items-center gap-1 text-[12px] text-text-tertiary">
                  <User className="size-3" />{it.annotator}
                </span>
              </button>
            ))}
          </aside>

          {/* 源（上下文）↔ 标注（补全答案）对比 */}
          <main key={current.annotation_id} className="flex min-w-0 flex-1 duration-200 animate-in fade-in">
            <section className="min-w-0 flex-1 overflow-y-auto border-r border-border">
              <div className="mx-auto max-w-[680px] px-10 py-8">
                <div className="mb-4 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">源数据</div>
                <SourceView row={current.source_row} cols={contextCols} primary={primarySet} />
              </div>
            </section>

            <aside className="flex w-[400px] shrink-0 flex-col overflow-y-auto p-4">
              <div className="mb-3 flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">
                标注答案
                <SourceBadge source={current.data._source} />
                <span className="ml-auto normal-case text-text-tertiary">由 {current.annotator}</span>
              </div>

              <div className="flex flex-col gap-3 rounded-lg border border-border bg-card p-4">
                {fillCols.map((col) => (
                  <div key={col.code} className="flex flex-col gap-1">
                    <span className="text-[12px] text-text-tertiary">{col.label || col.code}</span>
                    <span className="text-[14px] text-foreground">
                      {renderAnswer(col, current.data.fills?.[col.code])}
                    </span>
                  </div>
                ))}
                {fillCols.length === 0 && <div className="text-[13px] text-text-tertiary">无补全列</div>}
              </div>

              <div className="mt-4">
                <Textarea
                  id="review-note"
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  placeholder="打回备注（可选）"
                  className="min-h-16 resize-none bg-background text-[13px]"
                />
              </div>

              <div className="sticky bottom-0 mt-3 flex gap-2 bg-background pt-2">
                <Button
                  onClick={() => decide('approved')}
                  disabled={deciding}
                  className="flex-1 bg-success text-primary-foreground hover:bg-success/90"
                >
                  {deciding ? <Loader2 className="size-4 animate-spin" /> : <Check className="size-4" />}
                  通过
                  <Kbd className="ml-1 border-primary-foreground/20 bg-primary-foreground/10 text-primary-foreground">A</Kbd>
                </Button>
                <Button
                  onClick={() => decide('needs_redo')}
                  disabled={deciding}
                  variant="outline"
                  className="flex-1 border-destructive/40 text-destructive hover:bg-destructive/10 hover:text-destructive"
                >
                  <RotateCcw className="size-4" />
                  打回
                  <Kbd className="ml-1">R</Kbd>
                </Button>
              </div>
            </aside>
          </main>
        </div>
      )}

      {phase === 'ready' && current && (
        <footer className="flex h-8 shrink-0 items-center gap-4 border-t border-border px-4 text-[11px] text-text-tertiary">
          <Hint k="A" label="通过" />
          <Hint k="R" label="打回" />
          <Hint k="J / K" label="切换队列" />
          <Hint k="N" label="备注" />
        </footer>
      )}
    </div>
  )
}

/** 源数据：primary 列作标题，其余 context 列定义式罗列。 */
function SourceView({
  row,
  cols,
  primary,
}: {
  row: Record<string, unknown>
  cols: ColumnSpec[]
  primary: Set<string>
}) {
  const head = cols.filter((c) => primary.has(c.code))
  const rest = cols.filter((c) => !primary.has(c.code))
  const [title, ...subHead] = head
  return (
    <div>
      {title && (
        <h2 className="text-2xl font-semibold tracking-tight">{String(row[title.code] ?? '—')}</h2>
      )}
      {subHead.map((c) => (
        <p key={c.code} className="mt-3 whitespace-pre-wrap text-[16px] leading-[1.7] text-foreground/90">
          {String(row[c.code] ?? '')}
        </p>
      ))}
      <div className="mt-6 flex flex-col gap-3 border-t border-border-subtle pt-5">
        {rest.map((c) => (
          <div key={c.code} className="flex flex-col gap-1">
            <span className="text-[11px] uppercase tracking-wide text-text-tertiary">{c.label || c.code}</span>
            <span className="whitespace-pre-wrap text-[14px] leading-[1.7] text-foreground/90">
              {String(row[c.code] ?? '—')}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

function SourceBadge({ source }: { source?: string }) {
  if (source === 'ai')
    return (
      <span className="inline-flex items-center gap-1 rounded bg-ai/10 px-1.5 text-[11px] normal-case text-ai">
        <Sparkles className="size-3" />AI 预填
      </span>
    )
  if (source === 'ai-edited')
    return (
      <span className="inline-flex items-center gap-1 rounded bg-ai/10 px-1.5 text-[11px] normal-case text-ai">
        <Sparkles className="size-3" />AI+人工
      </span>
    )
  return <span className="rounded bg-surface-2 px-1.5 text-[11px] normal-case text-muted-foreground">人工</span>
}

/** 把 fill 值渲染成可读文本（选项→标签 / 布尔→是否）。 */
function renderAnswer(col: ColumnSpec, value: unknown): string {
  const kind = col.field?.kind ?? 'text'
  const opts = col.field?.options ?? []
  const empty = value === undefined || value === null || value === '' || (Array.isArray(value) && value.length === 0)
  if (empty) return '—'
  switch (kind) {
    case 'single':
      return opts.find((o) => o.value === value)?.label ?? String(value)
    case 'multi':
      return (Array.isArray(value) ? (value as string[]) : [])
        .map((v) => opts.find((o) => o.value === v)?.label ?? v)
        .join('、')
    case 'bool':
      return value ? '是' : '否'
    default:
      return String(value)
  }
}

function Sep() {
  return <span className="text-text-tertiary">·</span>
}

function Hint({ k, label }: { k: string; label: string }) {
  return (
    <span className="flex items-center gap-1.5">
      <Kbd>{k}</Kbd>
      {label}
    </span>
  )
}

function DatasetPicker({
  datasets,
  onPick,
}: {
  datasets: DatasetListItem[]
  onPick: (d: DatasetListItem) => void
}) {
  const [open, setOpen] = useState(false)
  const reviewable = datasets.filter((d) => d.completed > 0)
  return (
    <div className="relative">
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-1.5 rounded-md px-2 py-1 text-[13px] text-muted-foreground hover:bg-surface-3"
      >
        切换数据集
        <ChevronDown className="size-3.5" />
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-20" onClick={() => setOpen(false)} />
          <div className="absolute left-0 top-9 z-30 w-72 rounded-lg border border-border bg-popover p-1 shadow-lg">
            {reviewable.map((d) => (
              <button
                key={d.id}
                onClick={() => {
                  onPick(d)
                  setOpen(false)
                }}
                className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-surface-3"
              >
                <span className="truncate">{d.name}</span>
                <span className="ml-auto font-mono text-[11px] tabular text-text-tertiary">{d.completed} 已标</span>
              </button>
            ))}
            {reviewable.length === 0 && (
              <div className="px-2 py-2 text-[12px] text-text-tertiary">暂无有已完成标注的数据集</div>
            )}
          </div>
        </>
      )}
    </div>
  )
}

function Center({ title, desc, onBack }: { title: string; desc: string; onBack: () => void }) {
  return (
    <div className="flex h-svh flex-col items-center justify-center gap-3 bg-background text-center">
      <div className="text-lg font-semibold">{title}</div>
      <div className="text-sm text-muted-foreground">{desc}</div>
      <Button variant="secondary" size="sm" onClick={onBack}>返回数据集</Button>
    </div>
  )
}
