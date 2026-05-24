import { Sparkles } from 'lucide-react'
import { Slider } from '@/components/ui/slider'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'
import type { AnnotationData, AnnotationField } from '@/types'

interface SchemaFormProps {
  fields: AnnotationField[]
  values: AnnotationData
  activeFieldCode: string | null
  errors: Record<string, boolean>
  aiSuggestion?: AnnotationData | null
  onChange: (code: string, value: unknown) => void
  onFieldFocus: (code: string | null) => void
  onAcceptAi: () => void
}

const CONF_PRESETS = [
  { label: '低', value: 0.3, key: 'Q' },
  { label: '中', value: 0.6, key: 'W' },
  { label: '高', value: 0.9, key: 'E' },
]

export function SchemaForm({
  fields,
  values,
  activeFieldCode,
  errors,
  aiSuggestion,
  onChange,
  onFieldFocus,
  onAcceptAi,
}: SchemaFormProps) {
  const core = fields.filter((f) => f.group !== 'extra')
  const extra = fields.filter((f) => f.group === 'extra')
  const hasAi = !!aiSuggestion

  return (
    <div className="flex h-full flex-col gap-4">
      {hasAi && (
        <div className="flex items-center justify-between rounded-md border border-ai/40 bg-ai/10 px-3 py-2">
          <div className="flex items-center gap-2 text-[13px] text-ai">
            <Sparkles className="size-4" />
            <span>AI 已生成建议</span>
          </div>
          <button
            onClick={onAcceptAi}
            className="rounded px-2 py-1 text-xs font-medium text-ai transition-colors hover:bg-ai/15"
          >
            一键采纳 <kbd className="ml-1 font-mono text-[11px] opacity-70">⌘A</kbd>
          </button>
        </div>
      )}

      <FieldGroup label="核心">
        {core.map((f) => (
          <Field
            key={f.code}
            field={f}
            value={values[f.code]}
            active={activeFieldCode === f.code}
            error={errors[f.code]}
            aiValue={aiSuggestion?.[f.code]}
            onChange={(v) => onChange(f.code, v)}
            onFocus={() => onFieldFocus(f.code)}
          />
        ))}
      </FieldGroup>

      {extra.length > 0 && (
        <FieldGroup label="补充">
          {extra.map((f) => (
            <Field
              key={f.code}
              field={f}
              value={values[f.code]}
              active={activeFieldCode === f.code}
              error={errors[f.code]}
              aiValue={aiSuggestion?.[f.code]}
              onChange={(v) => onChange(f.code, v)}
              onFocus={() => onFieldFocus(f.code)}
            />
          ))}
        </FieldGroup>
      )}
    </div>
  )
}

function FieldGroup({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="mb-3 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">
        {label}
      </div>
      <div className="flex flex-col gap-5">{children}</div>
    </div>
  )
}

interface FieldProps {
  field: AnnotationField
  value: unknown
  active: boolean
  error?: boolean
  aiValue?: unknown
  onChange: (v: unknown) => void
  onFocus: () => void
}

function Field({ field, value, active, error, aiValue, onChange, onFocus }: FieldProps) {
  const isEmpty = value === undefined || value === null || value === ''
  const showAiHint = isEmpty && aiValue !== undefined && aiValue !== null

  return (
    <div
      data-fieldwrap={field.code}
      onFocus={onFocus}
      className={cn(
        'rounded-md p-2 transition-colors -mx-2',
        active && 'bg-surface-3/40 ring-1 ring-primary/40',
      )}
    >
      <div className="mb-2 flex items-center gap-1.5 text-sm">
        <span className="font-medium">{field.label}</span>
        {field.required && <span className="text-destructive">*</span>}
        {error && <span className="ml-auto text-xs text-destructive">必填</span>}
      </div>

      {field.widget === 'Select' && (
        <SelectWidget field={field} value={value as string} onChange={onChange} />
      )}
      {field.widget === 'Rating' && (
        <RatingWidget field={field} value={value as number} onChange={onChange} />
      )}
      {field.widget === 'Confidence' && (
        <ConfidenceWidget value={value as number} onChange={onChange} />
      )}
      {field.widget === 'TextArea' && (
        <TextAreaWidget field={field} value={(value as string) ?? ''} onChange={onChange} />
      )}

      {showAiHint && (
        <div className="mt-1.5 flex items-center gap-1 text-[11px] text-ai/80">
          <Sparkles className="size-3" />
          AI 建议：{labelForAi(field, aiValue)}
        </div>
      )}
    </div>
  )
}

function labelForAi(field: AnnotationField, aiValue: unknown): string {
  if (field.widget === 'Select') {
    return field.options?.find((o) => o.value === aiValue)?.label ?? String(aiValue)
  }
  return String(aiValue)
}

/** Select（枚举 ≤5）→ Segmented，带 1-N 角标 */
function SelectWidget({
  field,
  value,
  onChange,
}: {
  field: AnnotationField
  value: string
  onChange: (v: string) => void
}) {
  const opts = field.options ?? []
  return (
    <div className="flex flex-wrap gap-1.5">
      {opts.map((o, i) => {
        const selected = value === o.value
        return (
          <button
            key={o.value}
            type="button"
            onClick={() => onChange(o.value)}
            aria-pressed={selected}
            className={cn(
              'inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-[13px] transition-colors',
              selected
                ? 'border-primary bg-primary text-primary-foreground'
                : 'border-border bg-card text-muted-foreground hover:bg-surface-3 hover:text-foreground',
            )}
          >
            <span
              className={cn(
                'font-mono text-[10px]',
                selected ? 'opacity-70' : 'opacity-50',
              )}
            >
              {i + 1}
            </span>
            {o.label}
          </button>
        )
      })}
    </div>
  )
}

/** Rating 1-5 → 分段 */
function RatingWidget({
  field,
  value,
  onChange,
}: {
  field: AnnotationField
  value: number
  onChange: (v: number) => void
}) {
  const max = field.max ?? 5
  return (
    <div className="flex gap-1.5">
      {Array.from({ length: max }, (_, i) => i + 1).map((n) => {
        const on = value >= n
        return (
          <button
            key={n}
            type="button"
            onClick={() => onChange(n)}
            aria-label={`${n} 分`}
            className={cn(
              'flex h-9 w-9 items-center justify-center rounded-md border text-sm font-medium transition-colors',
              on
                ? 'border-primary bg-primary text-primary-foreground'
                : 'border-border bg-card text-muted-foreground hover:bg-surface-3',
            )}
          >
            {n}
          </button>
        )
      })}
    </div>
  )
}

/** Confidence 0-1 → Slider + 预设 */
function ConfidenceWidget({
  value,
  onChange,
}: {
  value: number
  onChange: (v: number) => void
}) {
  const v = typeof value === 'number' ? value : 0.7
  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex items-center gap-3">
        <Slider
          value={[v]}
          min={0}
          max={1}
          step={0.1}
          onValueChange={([nv]) => onChange(Number(nv.toFixed(1)))}
          className="flex-1"
        />
        <span className="w-9 text-right font-mono text-sm tabular text-foreground">
          {v.toFixed(1)}
        </span>
      </div>
      <div className="flex gap-1.5">
        {CONF_PRESETS.map((p) => (
          <button
            key={p.label}
            type="button"
            onClick={() => onChange(p.value)}
            className={cn(
              'rounded px-2 py-0.5 text-xs transition-colors',
              Math.abs(v - p.value) < 0.001
                ? 'bg-surface-3 text-foreground'
                : 'text-muted-foreground hover:bg-surface-3/60',
            )}
          >
            {p.label}
            <kbd className="ml-1 font-mono text-[10px] opacity-50">{p.key}</kbd>
          </button>
        ))}
      </div>
    </div>
  )
}

function TextAreaWidget({
  field,
  value,
  onChange,
}: {
  field: AnnotationField
  value: string
  onChange: (v: string) => void
}) {
  const max = field.max_length
  const near = max ? value.length > max * 0.9 : false
  return (
    <div className="flex flex-col gap-1">
      <Textarea
        value={value}
        maxLength={max}
        onChange={(e) => onChange(e.target.value)}
        placeholder="输入备注…（⌘↵ 提交）"
        className="min-h-20 resize-none bg-background"
      />
      {max && (
        <span
          className={cn(
            'self-end font-mono text-[11px] tabular',
            near ? 'text-warning' : 'text-text-tertiary',
          )}
        >
          {value.length}/{max}
        </span>
      )}
    </div>
  )
}
