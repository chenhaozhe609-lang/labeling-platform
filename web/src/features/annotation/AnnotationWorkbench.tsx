import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { CornerDownLeft, Loader2, LogOut, PartyPopper, RefreshCw, SkipForward } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Kbd } from '@/components/Kbd'
import type { ColumnSpec, DatasetListItem, TaskBundle } from '@/types'
import { claimTask, heartbeat, listDatasets, releaseTask, submitTask } from '@/api/tasks'
import { clearDraft, loadDraft, useAutosave } from '@/hooks/useDraft'
import { useLeaseTimer } from '@/hooks/useLeaseTimer'
import { useHeartbeat } from '@/hooks/useHeartbeat'
import { useSettings } from '@/stores/settings'
import { SchemaForm } from './SchemaForm'
import { ContextPane, ReadingPane, type ReadingField } from './panes'
import { AutosaveIndicator, LeaseTimer, ShortcutHintBar } from './statusbar'

export type FocusContext = 'reading' | 'widget' | 'field'

type Phase = 'loading' | 'ready' | 'empty' | 'nodataset'
type Values = Record<string, unknown>

const isEmpty = (v: unknown) =>
  v === undefined || v === null || v === '' || (Array.isArray(v) && v.length === 0)
const eq = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b)

export function AnnotationWorkbench() {
  const [phase, setPhase] = useState<Phase>('loading')
  const [dataset, setDataset] = useState<DatasetListItem | null>(null)
  const [bundle, setBundle] = useState<TaskBundle | null>(null)
  const [leaseExpiresAt, setLeaseExpiresAt] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const [values, setValues] = useState<Values>({})
  const [activeFieldCode, setActiveFieldCode] = useState<string | null>(null)
  const [detailsOpen, setDetailsOpen] = useState(false)
  const [errors, setErrors] = useState<Record<string, boolean>>({})
  const [dirty, setDirty] = useState(false)
  const [restored, setRestored] = useState(false)
  const [ctx, setCtx] = useState<FocusContext>('reading')

  const [searchParams] = useSearchParams()
  const shortcutsEnabled = useSettings((s) => s.shortcuts)
  const readingRef = useRef<HTMLDivElement | null>(null)

  const task = bundle?.task ?? null
  const columns = bundle?.form_schema.columns ?? []
  const fillCols = columns.filter((c) => c.role === 'fill')
  const aiFills = bundle?.ai_suggestion?.fills
  const primarySet = new Set(bundle?.form_schema.primary_cols ?? [])
  const contextFields: ReadingField[] = columns
    .filter((c) => c.role === 'context')
    .map((c) => ({ code: c.code, label: c.label || c.code, primary: primarySet.has(c.code) }))

  const lease = useLeaseTimer(leaseExpiresAt)
  const saveState = useAutosave(task?.id ?? 0, values, dirty)
  const displaySave = restored && !dirty ? 'restored' : saveState
  useHeartbeat(phase === 'ready' ? (task?.id ?? null) : null, phase === 'ready', setLeaseExpiresAt)

  useEffect(() => {
    void (async () => {
      try {
        const list = await listDatasets()
        const want = searchParams.get('dataset')
        const ds =
          (want ? list.find((d) => String(d.id) === want) : undefined) ??
          list.find((d) => d.status === 'READY' && d.pending > 0) ??
          list.find((d) => d.status === 'READY') ??
          list[0]
        if (!ds) {
          setPhase('nodataset')
          return
        }
        setDataset(ds)
        await claimNext(ds.id)
      } catch {
        setPhase('nodataset')
      }
    })()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // 切到新任务：草稿 > AI 预填 > 空
  useEffect(() => {
    if (!bundle) return
    const draft = loadDraft(bundle.task.id)
    const ai = bundle.ai_suggestion?.fills
    setValues(draft ?? (ai ? { ...ai } : {}))
    setActiveFieldCode(bundle.form_schema.columns.find((c) => c.role === 'fill')?.code ?? null)
    setErrors({})
    setDirty(false)
    setRestored(!!draft)
    setDetailsOpen(false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bundle?.task.id])

  useEffect(() => {
    const handler = () => {
      const el = document.activeElement as HTMLElement | null
      const tag = el?.tagName
      if (tag === 'TEXTAREA' || tag === 'INPUT') setCtx('field')
      else if (el?.closest('[data-fieldwrap]')) setCtx('widget')
      else setCtx('reading')
    }
    document.addEventListener('focusin', handler)
    return () => document.removeEventListener('focusin', handler)
  }, [])

  async function claimNext(datasetId: number) {
    setPhase('loading')
    try {
      const res = await claimTask(datasetId)
      if (!res.task) {
        setBundle(null)
        setPhase('empty')
        return
      }
      const b = res as TaskBundle
      setBundle(b)
      setLeaseExpiresAt(b.task.lease_expires_at ?? null)
      setPhase('ready')
    } catch {
      toast.error('领取任务失败')
      setPhase('empty')
    }
  }

  function setValue(code: string, v: unknown) {
    setValues((p) => ({ ...p, [code]: v }))
    setDirty(true)
    setErrors((e) => (e[code] ? { ...e, [code]: false } : e))
  }

  function focusField(code: string) {
    setActiveFieldCode(code)
    requestAnimationFrame(() => {
      const wrap = document.querySelector<HTMLElement>(`[data-fieldwrap="${code}"]`)
      wrap?.querySelector<HTMLElement>('button, textarea, input, [role="slider"]')?.focus()
    })
  }
  function focusNext(fromCode: string) {
    const i = fillCols.findIndex((f) => f.code === fromCode)
    const next = fillCols[i + 1]
    if (next) focusField(next.code)
  }

  function validate(): boolean {
    const missing: Record<string, boolean> = {}
    for (const f of fillCols) if (isEmpty(values[f.code])) missing[f.code] = true
    if (Object.keys(missing).length) {
      setErrors(missing)
      toast.error('请补全所有列后再提交')
      focusField(Object.keys(missing)[0])
      return false
    }
    return true
  }

  function computeSource(): 'human' | 'ai' | 'ai-edited' {
    if (!aiFills || Object.keys(aiFills).length === 0) return 'human'
    return eq(values, aiFills) ? 'ai' : 'ai-edited'
  }

  async function submit() {
    if (!bundle || submitting) return
    if (lease.state === 'expired') {
      toast.error('租约已过期，请重新领取任务')
      return
    }
    if (!validate()) return
    setSubmitting(true)
    const id = bundle.task.id
    try {
      await submitTask(id, { fills: values, _source: computeSource() }, bundle.form_schema.version)
      clearDraft(id)
      toast.success(`已提交 · #${id}`)
      await claimNext(bundle.task.dataset_id)
    } catch {
      toast.error('任务已超时或被回收，已为你领取下一条')
      if (dataset) await claimNext(dataset.id)
    } finally {
      setSubmitting(false)
    }
  }

  async function releaseTo(label: string) {
    if (!bundle) return
    const dsId = bundle.task.dataset_id
    try {
      await releaseTask(bundle.task.id)
    } catch {
      /* 幂等 */
    }
    toast(label)
    await claimNext(dsId)
  }

  async function extendLease() {
    if (!task) return
    try {
      const r = await heartbeat(task.id)
      setLeaseExpiresAt(r.lease_expires_at)
      toast.success('已延长租约')
    } catch {
      toast.error('续约失败')
    }
  }

  const keyHandler = (e: KeyboardEvent) => {
    if (phase !== 'ready' || !bundle || !shortcutsEnabled) return
    const el = document.activeElement as HTMLElement | null
    const tag = el?.tagName
    const inText = tag === 'TEXTAREA' || tag === 'INPUT'
    const mod = e.metaKey || e.ctrlKey

    if (mod && e.key === 'Enter') {
      e.preventDefault()
      void submit()
      return
    }
    if (inText) {
      if (e.key === 'Escape') {
        el?.blur()
        setActiveFieldCode(null)
      }
      return
    }
    if (e.altKey || mod) return

    const key = e.key
    if (key === 'Enter') {
      e.preventDefault()
      void submit()
      return
    }
    if (key === 'Escape') {
      if (activeFieldCode) {
        setActiveFieldCode(null)
        el?.blur()
      } else void releaseTo('已放回任务池')
      return
    }
    if (key === ' ') {
      e.preventDefault()
      setDetailsOpen((o) => !o)
      return
    }
    if (key === 'j' || key === 'J') return void readingRef.current?.scrollBy({ top: 140, behavior: 'smooth' })
    if (key === 'k' || key === 'K') return void readingRef.current?.scrollBy({ top: -140, behavior: 'smooth' })
    if (key === 's' || key === 'S' || key === 'ArrowRight') return void releaseTo('已跳过 · 放回任务池')

    // 数字键：当前 single fill 列按序号选项
    if (/^[1-9]$/.test(key)) {
      const target = activeSingle()
      const opt = target?.field?.options?.[Number(key) - 1]
      if (target && opt) {
        e.preventDefault()
        setValue(target.code, opt.value)
        focusNext(target.code)
      }
      return
    }
    // 字母快捷键：匹配任一 single fill 列选项的 key
    const K = key.toUpperCase()
    for (const f of fillCols) {
      const opt = f.field?.options?.find((o) => o.key?.toUpperCase() === K)
      if (opt) {
        setValue(f.code, opt.value)
        focusNext(f.code)
        return
      }
    }
  }
  function activeSingle(): ColumnSpec | undefined {
    const a = fillCols.find((f) => f.code === activeFieldCode)
    if (a?.field?.kind === 'single') return a
    return fillCols.find((f) => f.field?.kind === 'single')
  }

  const handlerRef = useRef(keyHandler)
  handlerRef.current = keyHandler
  useEffect(() => {
    const fn = (e: KeyboardEvent) => handlerRef.current(e)
    window.addEventListener('keydown', fn)
    return () => window.removeEventListener('keydown', fn)
  }, [])

  if (phase === 'loading') {
    return (
      <div className="flex h-svh items-center justify-center bg-background text-muted-foreground">
        <Loader2 className="mr-2 size-5 animate-spin" />
        正在领取任务…
      </div>
    )
  }
  if (phase === 'nodataset') {
    return <Center title="暂无可用数据集" desc="请管理员上传数据集、配置补全列并生成任务后再来。" />
  }
  if (phase === 'empty' || !bundle) {
    return (
      <div className="flex h-svh flex-col items-center justify-center gap-4 bg-background text-center">
        <PartyPopper className="size-10 text-success" />
        <div>
          <div className="text-lg font-semibold">该数据集已全部标完</div>
          <div className="mt-1 text-sm text-muted-foreground">{dataset?.name}</div>
        </div>
        {dataset && (
          <Button variant="secondary" size="sm" onClick={() => claimNext(dataset.id)}>
            <RefreshCw className="size-3.5" />
            再试一次
          </Button>
        )}
      </div>
    )
  }

  return (
    <div className="flex h-svh flex-col bg-background text-foreground">
      <header className="flex h-12 shrink-0 items-center gap-3 border-b border-border px-4 text-[13px]">
        <span className="font-medium">{dataset?.name ?? '标注'}</span>
        <Sep />
        <span className="font-mono tabular text-muted-foreground">#{task!.id}</span>
        {task!.round > 1 && (
          <Badge variant="secondary" className="gap-1 font-normal">
            <RefreshCw className="size-3" />round {task!.round}
          </Badge>
        )}
        <Sep />
        <LeaseTimer mmss={lease.mmss} state={lease.state} onExtend={extendLease} />
        <div className="ml-auto flex items-center gap-2">
          <AutosaveIndicator state={displaySave} />
          <Button variant="ghost" size="sm" onClick={() => releaseTo('已跳过 · 放回任务池')}>
            <SkipForward className="size-3.5" />跳过
          </Button>
          <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive" onClick={() => releaseTo('已放回任务池')}>
            <LogOut className="size-3.5" />释放
          </Button>
        </div>
      </header>

      <div className="flex min-h-0 flex-1">
        <aside className="w-[280px] shrink-0 border-r border-border">
          <ContextPane datasetName={dataset?.name ?? ''} pk={task!.source_row_pk} round={task!.round} history={[]} />
        </aside>
        <main key={task!.id} className="min-w-0 flex-1 border-r border-border duration-200 animate-in fade-in slide-in-from-right-2">
          <ReadingPane sourceRow={bundle.source_row} fields={contextFields} detailsOpen={detailsOpen} onToggleDetails={() => setDetailsOpen((o) => !o)} scrollRef={readingRef} />
        </main>
        <aside key={`form-${task!.id}`} className="flex w-[380px] shrink-0 flex-col overflow-y-auto p-4 duration-200 animate-in fade-in slide-in-from-right-2">
          <div className="flex-1">
            <SchemaForm fields={fillCols} values={values} activeFieldCode={activeFieldCode} errors={errors} aiFills={aiFills} onChange={setValue} onFieldFocus={setActiveFieldCode} />
          </div>
          <div className="sticky bottom-0 mt-4 bg-background pt-2">
            <Button onClick={submit} disabled={submitting} className="w-full bg-success text-primary-foreground hover:bg-success/90">
              {submitting ? <Loader2 className="size-4 animate-spin" /> : <CornerDownLeft className="size-4" />}
              提交并下一条
              <Kbd className="ml-1 border-primary-foreground/20 bg-primary-foreground/10 text-primary-foreground">↵</Kbd>
            </Button>
          </div>
        </aside>
      </div>

      <ShortcutHintBar context={ctx} />
    </div>
  )
}

function Sep() {
  return <span className="text-text-tertiary">·</span>
}
function Center({ title, desc }: { title: string; desc: string }) {
  return (
    <div className="flex h-svh flex-col items-center justify-center gap-2 bg-background text-center">
      <div className="text-lg font-semibold">{title}</div>
      <div className="text-sm text-muted-foreground">{desc}</div>
    </div>
  )
}
