import { Sparkles } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'
import type { ColumnSpec } from '@/types'

interface SchemaFormProps {
  fields: ColumnSpec[] // fill 列
  values: Record<string, unknown>
  activeFieldCode: string | null
  errors: Record<string, string> // code → 错误文案（空串/缺失 = 通过）
  aiFills?: Record<string, unknown>
  onChange: (code: string, value: unknown) => void
  onFieldFocus: (code: string | null) => void
  onFieldBlur?: (code: string) => void // 失焦内联校验（B2.10）
}

const isEmpty = (v: unknown) => v === undefined || v === null || v === '' || (Array.isArray(v) && v.length === 0)

export function SchemaForm({
  fields,
  values,
  activeFieldCode,
  errors,
  aiFills,
  onChange,
  onFieldFocus,
  onFieldBlur,
}: SchemaFormProps) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="mb-3 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">
        待补全（{fields.length}）
      </div>
      <div className="flex flex-col gap-5">
        {fields.map((f) => {
          const v = values[f.code]
          const fromAI = aiFills != null && !isEmpty(aiFills[f.code]) && eq(aiFills[f.code], v)
          return (
            <div
              key={f.code}
              data-fieldwrap={f.code}
              onFocus={() => onFieldFocus(f.code)}
              onBlur={() => onFieldBlur?.(f.code)}
              className={cn(
                '-mx-2 rounded-md p-2 transition-colors',
                activeFieldCode === f.code && 'bg-surface-3/40 ring-1 ring-primary/40',
              )}
            >
              <div className="mb-2 flex items-center gap-1.5 text-sm">
                <span className="font-medium">{f.label || f.code}</span>
                <span className="text-destructive">*</span>
                <span className="font-mono text-[11px] text-text-tertiary">{f.type}</span>
                {fromAI && !errors[f.code] && (
                  <span className="ml-auto inline-flex items-center gap-1 rounded bg-ai/10 px-1.5 text-[11px] text-ai">
                    <Sparkles className="size-3" />
                    AI
                  </span>
                )}
                {errors[f.code] && <span className="ml-auto text-xs text-destructive">{errors[f.code]}</span>}
              </div>
              <FieldControl field={f} value={v} onChange={(nv) => onChange(f.code, nv)} />
              {f.field?.hint && <div className="mt-1 text-[11px] text-text-tertiary">{f.field.hint}</div>}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function eq(a: unknown, b: unknown) {
  return JSON.stringify(a) === JSON.stringify(b)
}

function FieldControl({
  field,
  value,
  onChange,
}: {
  field: ColumnSpec
  value: unknown
  onChange: (v: unknown) => void
}) {
  const kind = field.field?.kind ?? 'text'
  const opts = field.field?.options ?? []

  switch (kind) {
    case 'single':
      // 选项过多（>5）改用下拉，避免按钮墙（B2.9 Combobox）。
      if (opts.length > 5) {
        return (
          <select
            value={(value as string) ?? ''}
            onChange={(e) => onChange(e.target.value === '' ? null : e.target.value)}
            className="h-9 w-full rounded-md border border-border bg-surface-1 px-2 text-[13px] outline-none focus:border-primary"
          >
            <option value="" disabled>
              选择…
            </option>
            {opts.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        )
      }
      return (
        <div className="flex flex-wrap gap-1.5">
          {opts.map((o, i) => {
            const on = value === o.value
            return (
              <button
                key={o.value}
                type="button"
                onClick={() => onChange(o.value)}
                aria-pressed={on}
                className={cn(
                  'inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-[13px] transition-colors',
                  on
                    ? 'border-primary bg-primary text-primary-foreground'
                    : 'border-border bg-card text-muted-foreground hover:bg-surface-3 hover:text-foreground',
                )}
              >
                <span className={cn('font-mono text-[10px]', on ? 'opacity-70' : 'opacity-50')}>
                  {o.key ?? i + 1}
                </span>
                {o.label}
              </button>
            )
          })}
        </div>
      )
    case 'multi': {
      const arr = Array.isArray(value) ? (value as string[]) : []
      return (
        <div className="flex flex-wrap gap-1.5">
          {opts.map((o) => {
            const on = arr.includes(o.value)
            return (
              <button
                key={o.value}
                type="button"
                onClick={() => onChange(on ? arr.filter((x) => x !== o.value) : [...arr, o.value])}
                className={cn(
                  'rounded-md border px-2.5 py-1.5 text-[13px] transition-colors',
                  on
                    ? 'border-primary bg-primary text-primary-foreground'
                    : 'border-border bg-card text-muted-foreground hover:bg-surface-3',
                )}
              >
                {o.label}
              </button>
            )
          })}
        </div>
      )
    }
    case 'bool':
      return (
        <div className="flex gap-1.5">
          {[
            { v: true, l: '是' },
            { v: false, l: '否' },
          ].map((b) => (
            <button
              key={b.l}
              type="button"
              onClick={() => onChange(b.v)}
              className={cn(
                'rounded-md border px-4 py-1.5 text-[13px] transition-colors',
                value === b.v
                  ? 'border-primary bg-primary text-primary-foreground'
                  : 'border-border bg-card text-muted-foreground hover:bg-surface-3',
              )}
            >
              {b.l}
            </button>
          ))}
        </div>
      )
    case 'number':
      return (
        <Input
          type="number"
          value={value == null ? '' : String(value)}
          min={field.field?.min}
          max={field.field?.max}
          step={field.field?.step}
          onChange={(e) => onChange(e.target.value === '' ? null : Number(e.target.value))}
          className="w-40"
        />
      )
    case 'date':
      return (
        <Input
          type="date"
          value={(value as string) ?? ''}
          onChange={(e) => onChange(e.target.value)}
          className="w-44"
        />
      )
    default: {
      // text
      const long = (field.field?.regex == null) && String(field.type).toLowerCase() === 'text'
      return long ? (
        <Textarea
          value={(value as string) ?? ''}
          placeholder={field.field?.placeholder}
          onChange={(e) => onChange(e.target.value)}
          className="min-h-20 resize-none bg-background"
        />
      ) : (
        <Input
          value={(value as string) ?? ''}
          placeholder={field.field?.placeholder}
          onChange={(e) => onChange(e.target.value)}
        />
      )
    }
  }
}
