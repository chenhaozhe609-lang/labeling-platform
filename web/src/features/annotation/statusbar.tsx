import { Cloud, Timer } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Kbd } from '@/components/Kbd'
import type { SaveState } from '@/hooks/useDraft'
import type { LeaseState } from '@/hooks/useLeaseTimer'
import type { FocusContext } from './AnnotationWorkbench'

export function LeaseTimer({
  mmss,
  state,
  onExtend,
}: {
  mmss: string
  state: LeaseState
  onExtend: () => void
}) {
  const color =
    state === 'critical' || state === 'expired'
      ? 'text-destructive'
      : state === 'warning'
        ? 'text-warning'
        : 'text-muted-foreground'
  return (
    <div className="flex items-center gap-1.5">
      <Timer className={cn('size-3.5', color)} />
      <span className={cn('font-mono text-[13px] tabular', color, state === 'warning' && 'animate-pulse')}>
        {mmss}
      </span>
      {(state === 'warning' || state === 'critical') && (
        <button
          onClick={onExtend}
          className="rounded px-1.5 py-0.5 text-[11px] text-warning transition-colors hover:bg-warning/10"
        >
          延长
        </button>
      )}
    </div>
  )
}

const SAVE_MAP: Record<SaveState, { t: string; c: string; dot: string } | null> = {
  idle: null,
  editing: { t: '编辑中…', c: 'text-muted-foreground', dot: 'bg-muted-foreground' },
  saving: { t: '保存中…', c: 'text-muted-foreground', dot: 'bg-primary animate-pulse' },
  saved: { t: '已保存', c: 'text-success', dot: 'bg-success' },
  restored: { t: '已恢复草稿', c: 'text-warning', dot: 'bg-warning' },
}

export function AutosaveIndicator({ state }: { state: SaveState }) {
  const m = SAVE_MAP[state]
  if (!m) return <span className="text-text-tertiary" />
  return (
    <div className={cn('flex items-center gap-1.5 text-[13px]', m.c)}>
      {state === 'saved' ? (
        <Cloud className="size-3.5" />
      ) : (
        <span className={cn('size-1.5 rounded-full', m.dot)} />
      )}
      {m.t}
    </div>
  )
}

// 仅列实际实现的快捷键（与 AnnotationWorkbench.keyHandler 一致）。
const HINTS: Record<FocusContext, Array<[string, string]>> = {
  reading: [
    ['Tab', '进入/切换字段'],
    ['1-9', '选项'],
    ['↵', '提交并下一条'],
    ['J / K', '滚动正文'],
    ['S', '跳过'],
  ],
  widget: [
    ['Tab / ⇧Tab', '切换字段'],
    ['1-9', '选/切选项'],
    ['⌫', '清空'],
    ['⌘A', '采纳 AI'],
    ['↵', '提交'],
  ],
  field: [
    ['Tab', '下一字段'],
    ['⌘↵', '提交'],
    ['Esc', '退出输入'],
  ],
}

export function ShortcutHintBar({ context }: { context: FocusContext }) {
  return (
    <div className="flex h-9 items-center gap-4 overflow-x-auto border-t border-border bg-card px-4 text-[12px] text-text-tertiary">
      {HINTS[context].map(([k, label]) => (
        <span key={k} className="flex shrink-0 items-center gap-1.5">
          <Kbd>{k}</Kbd>
          {label}
        </span>
      ))}
      <span className="ml-auto flex shrink-0 items-center gap-1.5 text-text-tertiary/70">
        <Kbd>?</Kbd>全部快捷键
      </span>
    </div>
  )
}
