import { ChevronRight, History } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { AnnotationHistory } from '@/types'

export function ContextPane({
  datasetName,
  pk,
  round,
  history,
}: {
  datasetName: string
  pk: string
  round: number
  history: AnnotationHistory[]
}) {
  return (
    <div className="flex h-full flex-col gap-6 overflow-y-auto p-4 text-[13px]">
      <Section title="上下文">
        <Meta k="数据集" v={datasetName} />
        <Meta k="主键 pk" v={pk} mono />
        <Meta k="重标轮次" v={`round ${round}`} />
      </Section>

      {history.length > 0 && (
        <Section title="历史">
          <div className="flex flex-col gap-2">
            {history.map((h, i) => (
              <div key={i} className="flex items-start gap-2 text-text-tertiary">
                <History className="mt-0.5 size-3.5 shrink-0" />
                <div>
                  <div>
                    round {h.round} · {h.annotator}
                  </div>
                  <div className="text-[12px]">
                    {h.created_at}
                    {h.superseded && (
                      <span className="ml-1.5 rounded bg-surface-2 px-1 text-[10px]">已废弃</span>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </Section>
      )}
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="mb-2.5 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">
        {title}
      </div>
      <div className="flex flex-col gap-2">{children}</div>
    </div>
  )
}

function Meta({ k, v, mono }: { k: string; v: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <span className="text-text-tertiary">{k}</span>
      <span className={cn('text-muted-foreground', mono && 'font-mono tabular')}>{v}</span>
    </div>
  )
}

export interface ReadingField {
  code: string
  label: string
  primary?: boolean
}

export function ReadingPane({
  sourceRow,
  fields,
  detailsOpen,
  onToggleDetails,
  scrollRef,
}: {
  sourceRow: Record<string, unknown>
  fields: ReadingField[]
  detailsOpen: boolean
  onToggleDetails: () => void
  scrollRef: React.RefObject<HTMLDivElement | null>
}) {
  const primary = fields.filter((f) => f.primary)
  const others = fields.filter((f) => !f.primary)
  const [head, ...rest] = primary

  return (
    <div ref={scrollRef} className="h-full overflow-y-auto scroll-smooth">
      <article className="mx-auto max-w-[680px] px-6 py-8">
        {head && (
          <h2 className="text-2xl font-semibold tracking-tight text-foreground">
            {String(sourceRow[head.code] ?? '')}
          </h2>
        )}
        {rest.map((f) => (
          <p
            key={f.code}
            className="mt-5 whitespace-pre-wrap text-[17px] leading-[1.75] text-foreground/90"
          >
            {String(sourceRow[f.code] ?? '')}
          </p>
        ))}

        <button
          onClick={onToggleDetails}
          className="mt-8 flex items-center gap-1 text-[13px] text-text-tertiary transition-colors hover:text-muted-foreground"
        >
          <ChevronRight className={cn('size-4 transition-transform', detailsOpen && 'rotate-90')} />
          详情（全部源字段）
        </button>

        {detailsOpen && (
          <div className="mt-3 flex flex-col gap-2 rounded-lg border border-border-subtle bg-card/50 p-4">
            {others.map((f) => (
              <div key={f.code} className="flex items-start justify-between gap-4 text-[13px]">
                <span className="shrink-0 text-text-tertiary">{f.label}</span>
                <span className="text-right font-mono tabular text-muted-foreground">
                  {String(sourceRow[f.code] ?? '—')}
                </span>
              </div>
            ))}
          </div>
        )}
      </article>
    </div>
  )
}
