import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { CornerDownLeft, LogOut, PartyPopper, RefreshCw, SkipForward } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Kbd } from '@/components/Kbd'
import type { AnnotationData, AnnotationField } from '@/types'
import { mockDatasetName, mockHistory, mockQueue } from '@/mocks/workbench'
import { clearDraft, loadDraft, useAutosave } from '@/hooks/useDraft'
import { useLeaseTimer } from '@/hooks/useLeaseTimer'
import { SchemaForm } from './SchemaForm'
import { ContextPane, ReadingPane } from './panes'
import { AutosaveIndicator, LeaseTimer, ShortcutHintBar } from './statusbar'

export type FocusContext = 'reading' | 'widget' | 'field'

const isEmpty = (v: unknown) => v === undefined || v === null || v === ''

function defaultsFor(fields: AnnotationField[]): AnnotationData {
  const v: AnnotationData = {}
  for (const f of fields) if (f.default !== undefined) v[f.code] = f.default
  return v
}

export function AnnotationWorkbench() {
  const [index, setIndex] = useState(0)
  const [values, setValues] = useState<AnnotationData>({})
  const [activeFieldCode, setActiveFieldCode] = useState<string | null>(null)
  const [detailsOpen, setDetailsOpen] = useState(false)
  const [errors, setErrors] = useState<Record<string, boolean>>({})
  const [dirty, setDirty] = useState(false)
  const [restored, setRestored] = useState(false)
  const [ctx, setCtx] = useState<FocusContext>('reading')

  const readingRef = useRef<HTMLDivElement | null>(null)

  const current = index < mockQueue.length ? mockQueue[index] : null
  const fields = current ? current.form_schema.annotation_fields : []

  const lease = useLeaseTimer(30, current?.task.id)
  const saveState = useAutosave(current?.task.id ?? 0, values, dirty)
  const displaySave = restored && !dirty ? 'restored' : saveState

  // 切到新任务：恢复草稿 / 套默认值 / 复位
  useEffect(() => {
    const cur = mockQueue[index]
    if (!cur) return
    const flds = cur.form_schema.annotation_fields
    const draft = loadDraft(cur.task.id)
    setValues(draft ?? defaultsFor(flds))
    setActiveFieldCode(flds[0]?.code ?? null)
    setErrors({})
    setDirty(false)
    setRestored(!!draft)
    setDetailsOpen(false)
  }, [index])

  // 焦点态跟踪（驱动底部快捷键提示 + 键盘分流）
  useEffect(() => {
    const handler = () => {
      const el = document.activeElement as HTMLElement | null
      const tag = el?.tagName
      if (tag === 'TEXTAREA' || (tag === 'INPUT' && (el as HTMLInputElement).type === 'text')) {
        setCtx('field')
      } else if (el?.closest('[data-fieldwrap]')) {
        setCtx('widget')
      } else {
        setCtx('reading')
      }
    }
    document.addEventListener('focusin', handler)
    document.addEventListener('focusout', () => window.setTimeout(handler, 0))
    return () => {
      document.removeEventListener('focusin', handler)
      document.removeEventListener('focusout', handler)
    }
  }, [])

  // ---- 动作 ----
  function setValue(code: string, v: unknown) {
    setValues((p) => ({ ...p, [code]: v }))
    setDirty(true)
    setErrors((e) => (e[code] ? { ...e, [code]: false } : e))
  }

  function focusField(code: string) {
    setActiveFieldCode(code)
    requestAnimationFrame(() => {
      const wrap = document.querySelector<HTMLElement>(`[data-fieldwrap="${code}"]`)
      wrap?.querySelector<HTMLElement>('button, textarea, [role="slider"], input')?.focus()
    })
  }

  function focusNext(fromCode: string) {
    const i = fields.findIndex((f) => f.code === fromCode)
    const next = fields[i + 1]
    if (next) focusField(next.code)
  }

  function acceptAi() {
    if (!current?.ai_suggestion) return
    const ai = current.ai_suggestion
    const merged: AnnotationData = { ...values }
    for (const f of fields) if (ai[f.code] !== undefined) merged[f.code] = ai[f.code]
    merged._source = 'ai'
    setValues(merged)
    setDirty(true)
    toast.success('已采纳 AI 建议')
  }

  function validate(): boolean {
    const missing: Record<string, boolean> = {}
    for (const f of fields) if (f.required && isEmpty(values[f.code])) missing[f.code] = true
    if (Object.keys(missing).length) {
      setErrors(missing)
      toast.error('请先填写必填项')
      focusField(Object.keys(missing)[0])
      return false
    }
    return true
  }

  function advance() {
    setIndex((i) => i + 1)
  }

  function submit() {
    if (!current) return
    if (lease.state === 'expired') {
      toast.error('租约已过期，请重新领取任务')
      return
    }
    if (!validate()) return
    clearDraft(current.task.id)
    toast.success(`已提交 · #${current.task.id}`)
    advance()
  }

  function releaseTo(action: string) {
    const at = index
    toast(action, { action: { label: '撤销', onClick: () => setIndex(at) } })
    advance()
  }

  // ---- 键盘（三焦点态模型）----
  const keyHandler = (e: KeyboardEvent) => {
    if (!current) return
    const el = document.activeElement as HTMLElement | null
    const tag = el?.tagName
    const isTextInput =
      tag === 'TEXTAREA' || (tag === 'INPUT' && (el as HTMLInputElement).type === 'text')
    const mod = e.metaKey || e.ctrlKey

    if (mod && e.key === 'Enter') {
      e.preventDefault()
      submit()
      return
    }
    if (mod && (e.key === 'a' || e.key === 'A')) {
      e.preventDefault()
      acceptAi()
      return
    }
    if (isTextInput) {
      if (e.key === 'Escape') {
        el?.blur()
        setActiveFieldCode(null)
      }
      return // FIELD 态：单键全禁用
    }
    if (e.altKey || mod) return

    const key = e.key
    if (key === 'Enter') {
      e.preventDefault()
      submit()
      return
    }
    if (key === 'Escape') {
      if (activeFieldCode) {
        setActiveFieldCode(null)
        el?.blur()
      } else {
        releaseTo('已放回任务池')
      }
      return
    }
    if (key === ' ') {
      e.preventDefault()
      setDetailsOpen((o) => !o)
      return
    }
    if (key === 'j' || key === 'J') {
      readingRef.current?.scrollBy({ top: 140, behavior: 'smooth' })
      return
    }
    if (key === 'k' || key === 'K') {
      readingRef.current?.scrollBy({ top: -140, behavior: 'smooth' })
      return
    }
    if (key === 's' || key === 'S' || key === 'ArrowRight') {
      releaseTo('已跳过 · 放回任务池')
      return
    }

    // 数字键：路由到当前活动字段，否则首个选项/评分字段
    if (/^[1-9]$/.test(key)) {
      const n = Number(key)
      const target =
        fields.find((f) => f.code === activeFieldCode) ??
        fields.find((f) => f.widget === 'Select' || f.widget === 'Rating')
      if (!target) return
      if (target.widget === 'Select') {
        const opt = target.options?.[n - 1]
        if (opt) {
          e.preventDefault()
          setValue(target.code, opt.value)
          focusNext(target.code)
        }
      } else if (target.widget === 'Rating') {
        if (n <= (target.max ?? 5)) {
          e.preventDefault()
          setValue(target.code, n)
          focusNext(target.code)
        }
      } else if (target.widget === 'Confidence') {
        e.preventDefault()
        setValue(target.code, Number((n / 10).toFixed(1)))
      }
      return
    }

    // 字母快捷键：confidence 预设优先，否则查字段 hotkeys
    const K = key.toUpperCase()
    const activeF = fields.find((f) => f.code === activeFieldCode)
    if (activeF?.widget === 'Confidence' && (K === 'Q' || K === 'W' || K === 'E')) {
      setValue(activeF.code, { Q: 0.3, W: 0.6, E: 0.9 }[K])
      return
    }
    const hot = fields.find((f) => f.hotkeys && f.hotkeys[K] !== undefined)
    if (hot) {
      setValue(hot.code, hot.hotkeys![K])
      focusNext(hot.code)
    }
  }

  // 稳定 listener，始终调用最新闭包
  const handlerRef = useRef(keyHandler)
  handlerRef.current = keyHandler
  useEffect(() => {
    const fn = (e: KeyboardEvent) => handlerRef.current(e)
    window.addEventListener('keydown', fn)
    return () => window.removeEventListener('keydown', fn)
  }, [])

  // ---- 队列标完：空态 ----
  if (!current) {
    return (
      <div className="flex h-svh flex-col items-center justify-center gap-4 bg-background text-center">
        <PartyPopper className="size-10 text-success" />
        <div>
          <div className="text-lg font-semibold">本数据集已全部标完</div>
          <div className="mt-1 text-sm text-muted-foreground">
            共完成 {mockQueue.length} 条 · 按 <Kbd>⌘K</Kbd> 切换数据集
          </div>
        </div>
        <Button variant="secondary" size="sm" onClick={() => setIndex(0)}>
          <RefreshCw className="size-3.5" />
          重新开始（演示）
        </Button>
      </div>
    )
  }

  const { task } = current

  return (
    <div className="flex h-svh flex-col bg-background text-foreground">
      {/* 顶栏 */}
      <header className="flex h-12 shrink-0 items-center gap-3 border-b border-border px-4 text-[13px]">
        <span className="font-medium">{mockDatasetName}</span>
        <Sep />
        <span className="font-mono tabular text-muted-foreground">#{task.id}</span>
        {task.round > 1 && (
          <Badge variant="secondary" className="gap-1 font-normal">
            <RefreshCw className="size-3" />
            round {task.round}
          </Badge>
        )}
        <Sep />
        <LeaseTimer mmss={lease.mmss} state={lease.state} onExtend={lease.extend} />

        <div className="ml-auto flex items-center gap-2">
          <AutosaveIndicator state={displaySave} />
          <Button variant="ghost" size="sm" onClick={() => releaseTo('已跳过 · 放回任务池')}>
            <SkipForward className="size-3.5" />
            跳过
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="text-destructive hover:text-destructive"
            onClick={() => releaseTo('已放回任务池')}
          >
            <LogOut className="size-3.5" />
            释放
          </Button>
          <span className="ml-1 font-mono text-[12px] tabular text-text-tertiary">
            {index + 1}/{mockQueue.length}
          </span>
        </div>
      </header>

      {/* 三栏 */}
      <div className="flex min-h-0 flex-1">
        <aside className="w-[280px] shrink-0 border-r border-border">
          <ContextPane
            pk={task.source_row_pk}
            sourceVersion="v3"
            batch="#5"
            round={task.round}
            history={mockHistory[task.id] ?? []}
          />
        </aside>

        <main
          key={task.id}
          className="min-w-0 flex-1 border-r border-border duration-200 animate-in fade-in slide-in-from-right-2"
        >
          <ReadingPane
            sourceRow={current.source_row}
            sourceFields={current.form_schema.source_fields}
            detailsOpen={detailsOpen}
            onToggleDetails={() => setDetailsOpen((o) => !o)}
            scrollRef={readingRef}
          />
        </main>

        <aside
          key={`form-${task.id}`}
          className="flex w-[380px] shrink-0 flex-col overflow-y-auto p-4 duration-200 animate-in fade-in slide-in-from-right-2"
        >
          <div className="flex-1">
            <SchemaForm
              fields={fields}
              values={values}
              activeFieldCode={activeFieldCode}
              errors={errors}
              aiSuggestion={current.ai_suggestion}
              onChange={setValue}
              onFieldFocus={setActiveFieldCode}
              onAcceptAi={acceptAi}
            />
          </div>
          <div className="sticky bottom-0 mt-4 bg-background pt-2">
            <Button
              onClick={submit}
              className="w-full bg-success text-primary-foreground hover:bg-success/90"
            >
              <CornerDownLeft className="size-4" />
              提交并下一条
              <Kbd className="ml-1 border-primary-foreground/20 bg-primary-foreground/10 text-primary-foreground">
                ↵
              </Kbd>
            </Button>
          </div>
        </aside>
      </div>

      {/* 底栏：随焦点态变化 */}
      <ShortcutHintBar context={ctx} />
    </div>
  )
}

function Sep() {
  return <span className="text-text-tertiary">·</span>
}
