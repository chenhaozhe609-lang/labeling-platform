import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { ArrowLeft, Loader2, Plus, Trash2 } from 'lucide-react'
import { getDatasetDetail, updateFormSchema } from '@/api/datasets'
import { queryClient } from '@/lib/query'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { AnnotationField, AnnotationWidget, FieldOption } from '@/types'

const WIDGETS: AnnotationWidget[] = ['Select', 'MultiSelect', 'Rating', 'Confidence', 'TextArea', 'Input', 'Switch']

interface Draft {
  code: string
  label: string
  widget: AnnotationWidget
  required: boolean
  options: FieldOption[]
}

export function SchemaEditorPage() {
  const { id } = useParams<{ id: string }>()
  const dsId = Number(id)
  const nav = useNavigate()
  const { data, isLoading } = useQuery({ queryKey: ['dataset', dsId], queryFn: () => getDatasetDetail(dsId) })

  const [drafts, setDrafts] = useState<Draft[]>([])
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!data) return
    setDrafts(
      (data.dataset.form_schema?.annotation_fields ?? []).map((f) => ({
        code: f.code,
        label: f.label,
        widget: f.widget,
        required: !!f.required,
        options: f.options ?? [],
      })),
    )
  }, [data])

  if (isLoading || !data) return <div className="px-8 py-8 text-sm text-muted-foreground">加载中…</div>

  function update(i: number, patch: Partial<Draft>) {
    setDrafts((ds) => ds.map((d, j) => (j === i ? { ...d, ...patch } : d)))
  }
  function addField() {
    setDrafts((ds) => [...ds, { code: '', label: '', widget: 'Select', required: false, options: [] }])
  }
  function removeField(i: number) {
    setDrafts((ds) => ds.filter((_, j) => j !== i))
  }

  const codes = drafts.map((d) => d.code.trim())
  const valid =
    drafts.every((d) => d.code.trim() && d.label.trim()) &&
    new Set(codes).size === codes.length &&
    drafts.every((d) => !needsOptions(d.widget) || d.options.length > 0)

  async function save() {
    setSaving(true)
    try {
      const annotation_fields: AnnotationField[] = drafts.map((d) => {
        const f: AnnotationField = { code: d.code.trim(), label: d.label.trim(), widget: d.widget, group: 'core' }
        if (d.required) f.required = true
        if (needsOptions(d.widget)) f.options = d.options
        if (d.widget === 'Rating') {
          f.min = 1
          f.max = 5
        }
        if (d.widget === 'Confidence') {
          f.min = 0
          f.max = 1
          f.step = 0.1
        }
        return f
      })
      const fs = { ...data!.dataset.form_schema, annotation_fields }
      const res = await updateFormSchema(dsId, fs)
      toast.success(`已保存 · form_schema v${res.form_schema_version}`)
      queryClient.invalidateQueries({ queryKey: ['dataset', dsId] })
      nav(`/datasets/${dsId}`)
    } catch {
      toast.error('保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="mx-auto max-w-2xl px-8 py-8">
      <Link to={`/datasets/${dsId}`} className="mb-5 inline-flex items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" />
        {data.dataset.name}
      </Link>
      <h1 className="mb-1 text-xl font-semibold tracking-tight">编辑标注字段</h1>
      <p className="mb-6 text-[13px] text-text-tertiary">保存后 form_schema 版本号 +1；源字段（只读）不在此编辑。</p>

      <div className="flex flex-col gap-3">
        {drafts.map((d, i) => (
          <div key={i} className="rounded-lg border border-border bg-card p-4">
            <div className="mb-3 flex items-center gap-2">
              <Input value={d.label} onChange={(e) => update(i, { label: e.target.value })} placeholder="显示名，如 主题分类" className="flex-1" />
              <Input value={d.code} onChange={(e) => update(i, { code: e.target.value })} placeholder="code，如 topic" className="w-36 font-mono" />
              <button onClick={() => removeField(i)} className="grid size-9 place-items-center rounded-md text-text-tertiary hover:bg-destructive/10 hover:text-destructive">
                <Trash2 className="size-4" />
              </button>
            </div>
            <div className="flex items-center gap-4 text-[13px]">
              <select
                value={d.widget}
                onChange={(e) => update(i, { widget: e.target.value as AnnotationWidget })}
                className="h-9 rounded-md border border-input bg-transparent px-2 text-sm outline-none focus-visible:border-ring"
              >
                {WIDGETS.map((w) => (
                  <option key={w} value={w} className="bg-popover">{w}</option>
                ))}
              </select>
              <label className="flex items-center gap-1.5 text-muted-foreground">
                <input type="checkbox" checked={d.required} onChange={(e) => update(i, { required: e.target.checked })} />
                必填
              </label>
            </div>

            {needsOptions(d.widget) && (
              <OptionsEditor options={d.options} onChange={(opts) => update(i, { options: opts })} />
            )}
          </div>
        ))}
      </div>

      <button onClick={addField} className="mt-3 inline-flex items-center gap-1.5 text-[13px] text-primary hover:underline">
        <Plus className="size-4" />
        添加字段
      </button>

      <div className="mt-6 flex gap-2">
        <Button onClick={save} disabled={!valid || saving}>
          {saving && <Loader2 className="size-4 animate-spin" />}
          保存
        </Button>
        <Link to={`/datasets/${dsId}`} className="btn-ghost">取消</Link>
      </div>
    </div>
  )
}

function needsOptions(w: AnnotationWidget) {
  return w === 'Select' || w === 'MultiSelect'
}

function OptionsEditor({ options, onChange }: { options: FieldOption[]; onChange: (o: FieldOption[]) => void }) {
  return (
    <div className="mt-3 border-t border-border-subtle pt-3">
      <div className="mb-1.5 text-[11px] uppercase tracking-wide text-text-tertiary">选项</div>
      <div className="flex flex-col gap-1.5">
        {options.map((o, i) => (
          <div key={i} className="flex gap-2">
            <Input value={o.label} onChange={(e) => onChange(options.map((x, j) => (j === i ? { ...x, label: e.target.value } : x)))} placeholder="显示" className="flex-1" />
            <Input value={o.value} onChange={(e) => onChange(options.map((x, j) => (j === i ? { ...x, value: e.target.value } : x)))} placeholder="value" className="w-32 font-mono" />
            <button onClick={() => onChange(options.filter((_, j) => j !== i))} className="grid size-9 place-items-center rounded-md text-text-tertiary hover:text-destructive">
              <Trash2 className="size-3.5" />
            </button>
          </div>
        ))}
      </div>
      <button onClick={() => onChange([...options, { value: '', label: '' }])} className="mt-1.5 text-[12px] text-primary hover:underline">
        + 选项
      </button>
    </div>
  )
}
