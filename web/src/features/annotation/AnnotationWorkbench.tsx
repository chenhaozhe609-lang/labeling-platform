import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { CornerDownLeft, Loader2, LogOut, PartyPopper, Pause, RefreshCw, SkipForward, X } from 'lucide-react'
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
  const [paused, setPaused] = useState(false)

  const [values, setValues] = useState<Values>({})
  const [activeFieldCode, setActiveFieldCode] = useState<string | null>(null)
  const [detailsOpen, setDetailsOpen] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [dirty, setDirty] = useState(false)
  const [restored, setRestored] = useState(false)
  const [ctx, setCtx] = useState<FocusContext>('reading')
  const [showHelp, setShowHelp] = useState(false)

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
        setPaused('paused' in res ? !!res.paused : false)
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
    setErrors((e) => (e[code] ? { ...e, [code]: '' } : e)) // 改动即清除该列错误
  }

  // 单列校验：必填 + text 的 regex + number 的 min/max（B2.10）。
  function fieldError(col: ColumnSpec, v: unknown): string {
    if (isEmpty(v)) return '必填'
    const f = col.field
    if (!f) return ''
    if ((f.kind ?? 'text') === 'text' && f.regex) {
      try {
        if (!new RegExp(f.regex).test(String(v))) return '格式不符'
      } catch {
        /* 非法 regex：跳过 */
      }
    }
    if (f.kind === 'number') {
      const n = Number(v)
      if (f.min != null && n < f.min) return `不小于 ${f.min}`
      if (f.max != null && n > f.max) return `不大于 ${f.max}`
    }
    return ''
  }

  function validateField(code: string) {
    const col = fillCols.find((c) => c.code === code)
    if (!col) return
    const msg = fieldError(col, values[code])
    setErrors((e) => ({ ...e, [code]: msg }))
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
    const errs: Record<string, string> = {}
    for (const f of fillCols) {
      const msg = fieldError(f, values[f.code])
      if (msg) errs[f.code] = msg
    }
    if (Object.keys(errs).length) {
      setErrors(errs)
      toast.error('请检查标红的列后再提交')
      focusField(Object.keys(errs)[0])
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
      await submitTask(
        id,
        { fills: values, _source: computeSource(), ...(aiFills ? { _ai: aiFills } : {}) },
        bundle.form_schema.version,
      )
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

  function activeField(): ColumnSpec | undefined {
    return fillCols.find((f) => f.code === activeFieldCode) ?? fillCols[0]
  }
  // Tab / ⇧Tab 在补全字段间循环
  function moveField(dir: 1 | -1) {
    if (fillCols.length === 0) return
    const cur = activeFieldCode ?? fillCols[0].code
    const i = fillCols.findIndex((f) => f.code === cur)
    const ni = i < 0 ? 0 : (i + dir + fillCols.length) % fillCols.length
    focusField(fillCols[ni].code)
  }
  // 数字键作用于当前字段：single 选第 N 项（自动跳下一字段）/ multi 切第 N 项 / bool 1是 2否
  function applyOption(f: ColumnSpec, idx: number) {
    const kind = f.field?.kind ?? 'text'
    if (kind === 'single') {
      const o = f.field?.options?.[idx]
      if (o) {
        setValue(f.code, o.value)
        focusNext(f.code)
      }
    } else if (kind === 'multi') {
      const o = f.field?.options?.[idx]
      if (!o) return
      const arr = Array.isArray(values[f.code]) ? (values[f.code] as string[]) : []
      setValue(f.code, arr.includes(o.value) ? arr.filter((x) => x !== o.value) : [...arr, o.value])
    } else if (kind === 'bool') {
      if (idx === 0) {
        setValue(f.code, true)
        focusNext(f.code)
      } else if (idx === 1) {
        setValue(f.code, false)
        focusNext(f.code)
      }
    }
  }
  function clearActive() {
    if (!activeFieldCode) return
    const f = fillCols.find((c) => c.code === activeFieldCode)
    if (f) setValue(f.code, f.field?.kind === 'multi' ? [] : null)
  }
  function adoptAI() {
    if (!aiFills || Object.keys(aiFills).length === 0) return
    setValues((p) => ({ ...p, ...aiFills }))
    setDirty(true)
    setErrors({})
    toast('已采纳 AI 预填')
  }

  const keyHandler = (e: KeyboardEvent) => {
    if (phase !== 'ready' || !bundle || !shortcutsEnabled) return
    const el = document.activeElement as HTMLElement | null
    const tag = el?.tagName
    const inText = tag === 'TEXTAREA' || tag === 'INPUT'
    const mod = e.metaKey || e.ctrlKey

    // 帮助浮层打开时：吞掉所有键，仅 ? / Esc 关闭
    if (showHelp) {
      if (e.key === 'Escape' || e.key === '?') {
        e.preventDefault()
        setShowHelp(false)
      }
      return
    }
    // 提交（文本框内用 ⌘↵）
    if (mod && e.key === 'Enter') {
      e.preventDefault()
      void submit()
      return
    }
    // Tab / ⇧Tab 字段循环（文本框内也生效，便于跳出输入）
    if (e.key === 'Tab') {
      e.preventDefault()
      moveField(e.shiftKey ? -1 : 1)
      return
    }
    // ⌘A 采纳 AI 预填（仅非文本；文本框内留给原生全选）
    if (mod && (e.key === 'a' || e.key === 'A')) {
      if (!inText && aiFills) {
        e.preventDefault()
        adoptAI()
      }
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

    // ? 打开全部快捷键（非文本时）
    if (e.key === '?') {
      e.preventDefault()
      setShowHelp(true)
      return
    }

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
    if (key === 's' || key === 'S') return void releaseTo('已跳过 · 放回任务池')
    if (key === 'Backspace' && activeFieldCode) {
      e.preventDefault()
      clearActive()
      return
    }

    // 数字键：作用于当前字段
    if (/^[1-9]$/.test(key)) {
      const f = activeField()
      if (f) {
        e.preventDefault()
        applyOption(f, Number(key) - 1)
      }
      return
    }
    // 字母键：匹配任一 fill 列选项的自定义 key（schema 配了才有）
    const K = key.toUpperCase()
    for (const f of fillCols) {
      const opt = f.field?.options?.find((o) => o.key?.toUpperCase() === K)
      if (opt) {
        if (f.field?.kind === 'multi') {
          const arr = Array.isArray(values[f.code]) ? (values[f.code] as string[]) : []
          setValue(f.code, arr.includes(opt.value) ? arr.filter((x) => x !== opt.value) : [...arr, opt.value])
        } else {
          setValue(f.code, opt.value)
          focusNext(f.code)
        }
        return
      }
    }
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
        {paused ? <Pause className="size-10 text-warning" /> : <PartyPopper className="size-10 text-success" />}
        <div>
          <div className="text-lg font-semibold">{paused ? '该数据集已暂停' : '该数据集已全部标完'}</div>
          <div className="mt-1 text-sm text-muted-foreground">
            {paused ? `${dataset?.name ?? ''} · 请等管理员恢复后再标` : dataset?.name}
          </div>
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
        {/* 中：补全表单 —— 视觉重心（最宽栏，内容居中） */}
        <main key={`form-${task!.id}`} className="flex min-w-0 flex-1 flex-col overflow-y-auto border-r border-border duration-200 animate-in fade-in slide-in-from-right-2">
          <div className="mx-auto flex w-full max-w-xl flex-1 flex-col px-6 py-8">
            <div className="flex-1">
              <SchemaForm fields={fillCols} values={values} activeFieldCode={activeFieldCode} errors={errors} aiFills={aiFills} onChange={setValue} onFieldFocus={setActiveFieldCode} onFieldBlur={validateField} />
            </div>
            <div className="sticky bottom-0 mt-6 bg-background pt-2">
              <Button onClick={submit} disabled={submitting} className="w-full bg-success text-primary-foreground hover:bg-success/90">
                {submitting ? <Loader2 className="size-4 animate-spin" /> : <CornerDownLeft className="size-4" />}
                提交并下一条
                <Kbd className="ml-1 border-primary-foreground/20 bg-primary-foreground/10 text-primary-foreground">↵</Kbd>
              </Button>
            </div>
          </div>
        </main>
        {/* 右：源内容（context 列）只读阅读 */}
        <aside key={task!.id} className="w-[400px] shrink-0 duration-200 animate-in fade-in slide-in-from-right-2">
          <ReadingPane sourceRow={bundle.source_row} fields={contextFields} detailsOpen={detailsOpen} onToggleDetails={() => setDetailsOpen((o) => !o)} scrollRef={readingRef} />
        </aside>
      </div>

      {showHelp && <HelpOverlay onClose={() => setShowHelp(false)} />}
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

const HELP_GROUPS: Array<[string, Array<[string, string]>]> = [
  ['导航', [
    ['Tab / ⇧Tab', '切换上一/下一个补全字段'],
    ['J / K', '上下滚动右侧正文'],
    ['Space', '展开 / 收起全部源字段'],
  ]],
  ['填值', [
    ['1 – 9', '当前字段选第 N 项（单选选中 · 多选切换 · 布尔 1是 2否）'],
    ['字母键', '按选项自定义快捷键选中（需在 schema 配置）'],
    ['⌫', '清空当前字段'],
    ['⌘A', '采纳 AI 预填'],
  ]],
  ['提交 / 流转', [
    ['↵', '提交并自动领下一条'],
    ['⌘↵', '在文本框内提交'],
    ['S', '跳过当前任务（放回池）'],
    ['Esc', '退出输入 / 释放任务'],
  ]],
]

function HelpOverlay({ onClose }: { onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onClose}>
      <div className="w-full max-w-lg rounded-lg border border-border bg-popover p-6 shadow-lg" onClick={(e) => e.stopPropagation()}>
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-sm font-semibold">键盘快捷键</h2>
          <button onClick={onClose} className="text-text-tertiary hover:text-foreground" aria-label="关闭">
            <X className="size-4" />
          </button>
        </div>
        <div className="flex flex-col gap-5">
          {HELP_GROUPS.map(([title, rows]) => (
            <div key={title}>
              <div className="mb-2 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">{title}</div>
              <div className="flex flex-col gap-2">
                {rows.map(([k, label]) => (
                  <div key={k} className="flex items-center gap-3 text-[13px]">
                    <Kbd className="shrink-0">{k}</Kbd>
                    <span className="text-muted-foreground">{label}</span>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
        <div className="mt-5 text-[11px] text-text-tertiary">
          按 <Kbd>?</Kbd> 或 <Kbd>Esc</Kbd> 关闭
        </div>
      </div>
    </div>
  )
}
