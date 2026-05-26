import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { AlertTriangle, ArrowLeft, Loader2, Plus, Trash2 } from 'lucide-react'
import { getDatasetDetail, updateFormSchema } from '@/api/datasets'
import { queryClient } from '@/lib/query'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import type { ColumnRole, ColumnSpec, FieldKind, FieldOption } from '@/types'

const ROLES: { value: ColumnRole; label: string }[] = [
  { value: 'context', label: '上下文' },
  { value: 'fill', label: '补全' },
  { value: 'hidden', label: '隐藏' },
]
const KINDS: FieldKind[] = ['text', 'single', 'multi', 'number', 'bool', 'date']

export function SchemaEditorPage() {
  const { id } = useParams<{ id: string }>()
  const dsId = Number(id)
  const nav = useNavigate()
  const { data, isLoading } = useQuery({ queryKey: ['dataset', dsId], queryFn: () => getDatasetDetail(dsId) })

  const [cols, setCols] = useState<ColumnSpec[]>([])
  const [saving, setSaving] = useState(false)
  const [confirm, setConfirm] = useState<{ changes: string[]; affected: number } | null>(null)

  useEffect(() => {
    if (data) setCols(data.dataset.form_schema?.columns ?? [])
  }, [data])

  if (isLoading || !data) return <div className="px-8 py-8 text-sm text-muted-foreground">加载中…</div>

  function update(i: number, patch: Partial<ColumnSpec>) {
    setCols((cs) => cs.map((c, j) => (j === i ? { ...c, ...patch } : c)))
  }
  function setRole(i: number, role: ColumnRole) {
    setCols((cs) =>
      cs.map((c, j) => {
        if (j !== i) return c
        const next: ColumnSpec = { ...c, role }
        if (role === 'fill' && !next.field) next.field = { kind: 'text' }
        return next
      }),
    )
  }
  function setKind(i: number, kind: FieldKind) {
    setCols((cs) => cs.map((c, j) => (j === i ? { ...c, field: { ...(c.field ?? { kind }), kind } } : c)))
  }
  function setOptions(i: number, options: FieldOption[]) {
    setCols((cs) => cs.map((c, j) => (j === i ? { ...c, field: { ...(c.field ?? { kind: 'single' }), options } } : c)))
  }

  const fillCount = cols.filter((c) => c.role === 'fill').length
  const valid =
    fillCount > 0 &&
    cols.every((c) => c.role !== 'fill' || c.field?.kind !== 'single' || (c.field.options?.length ?? 0) > 0)

  async function doSave(force = false) {
    setSaving(true)
    try {
      const fs = { ...data!.dataset.form_schema, columns: cols }
      const res = await updateFormSchema(dsId, fs, force)
      toast.success(`已保存 · form_schema v${res.form_schema_version}（记得「生成任务」）`)
      queryClient.invalidateQueries({ queryKey: ['dataset', dsId] })
      nav(`/datasets/${dsId}`)
    } catch (e) {
      const r = (e as { response?: { status?: number; data?: { destructive?: string[]; affected?: number } } }).response
      if (r?.status === 409 && r.data?.destructive?.length) {
        setConfirm({ changes: r.data.destructive, affected: r.data.affected ?? 0 })
      } else {
        toast.error('保存失败')
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="mx-auto max-w-3xl px-8 py-8">
      <Link to={`/datasets/${dsId}`} className="mb-5 inline-flex items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" />
        {data.dataset.name}
      </Link>
      <PageHeader
        eyebrow="SCHEMA"
        title="列与字段"
        description={
          <>
            为每列指定角色：<b>上下文</b>（只读，也作 AI 预填依据）·<b>补全</b>（待标注，按类型配置控件）·<b>隐藏</b>。保存后回详情页点「生成任务」。
          </>
        }
      />

      <div className="flex flex-col gap-2">
        {cols.map((c, i) => (
          <div key={c.code} className="rounded-lg border border-border bg-card p-3">
            <div className="flex items-center gap-2">
              <span className="font-medium">{c.label || c.code}</span>
              <span className="font-mono text-[11px] text-text-tertiary">{c.code} · {c.type}</span>
              {c.pk && <span className="rounded bg-surface-2 px-1.5 text-[11px] text-text-tertiary">主键</span>}
              <div className="ml-auto flex gap-1">
                {c.pk ? (
                  <span className="rounded-md border border-border px-2 py-1 text-[12px] text-text-tertiary">id</span>
                ) : (
                  ROLES.map((r) => (
                    <button
                      key={r.value}
                      onClick={() => setRole(i, r.value)}
                      className={cn(
                        'rounded-md border px-2.5 py-1 text-[12px] transition-colors',
                        c.role === r.value
                          ? 'border-primary bg-primary text-primary-foreground'
                          : 'border-border text-muted-foreground hover:bg-surface-3',
                      )}
                    >
                      {r.label}
                    </button>
                  ))
                )}
              </div>
            </div>

            {c.role === 'fill' && (
              <div className="mt-3 border-t border-border-subtle pt-3">
                <div className="flex items-center gap-2 text-[13px]">
                  <span className="text-text-tertiary">控件</span>
                  <select
                    value={c.field?.kind ?? 'text'}
                    onChange={(e) => setKind(i, e.target.value as FieldKind)}
                    className="h-8 rounded-md border border-input bg-transparent px-2 text-sm outline-none focus-visible:border-ring"
                  >
                    {KINDS.map((k) => (
                      <option key={k} value={k} className="bg-popover">{k}</option>
                    ))}
                  </select>
                  {c.field?.kind === 'text' && (
                    <Input
                      value={c.field?.placeholder ?? ''}
                      onChange={(e) => update(i, { field: { ...(c.field ?? { kind: 'text' }), placeholder: e.target.value } })}
                      placeholder="placeholder（可选）"
                      className="h-8 flex-1"
                    />
                  )}
                </div>
                {(c.field?.kind === 'single' || c.field?.kind === 'multi') && (
                  <OptionsEditor options={c.field?.options ?? []} onChange={(o) => setOptions(i, o)} />
                )}
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="mt-6 flex items-center gap-2">
        <Button onClick={() => doSave(false)} disabled={!valid || saving}>
          {saving && <Loader2 className="size-4 animate-spin" />}
          保存
        </Button>
        <Link to={`/datasets/${dsId}`} className="btn-ghost">取消</Link>
        {fillCount === 0 && <span className="text-[12px] text-warning">至少需要一个「补全」列</span>}
      </div>

      {confirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={() => setConfirm(null)}>
          <div className="w-full max-w-md rounded-lg border border-border bg-popover p-5 shadow-lg" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center gap-2 text-warning">
              <AlertTriangle className="size-4" />
              <h2 className="text-sm font-semibold text-foreground">破坏性变更确认</h2>
            </div>
            <p className="mt-2 text-[13px] text-muted-foreground">
              以下改动会让该数据集已有的 <b className="text-foreground">{confirm.affected}</b> 条标注失配（旧值对不上新结构）：
            </p>
            <ul className="mt-3 flex flex-col gap-1.5 rounded-md border border-border-subtle bg-card/50 p-3 text-[13px]">
              {confirm.changes.map((c, i) => (
                <li key={i} className="flex gap-2 text-foreground">
                  <span className="text-warning">·</span>
                  {c}
                </li>
              ))}
            </ul>
            <p className="mt-3 text-[12px] text-text-tertiary">旧标注会保留为历史，但不再与当前结构对应。确认要保存吗？</p>
            <div className="mt-4 flex justify-end gap-2">
              <button onClick={() => setConfirm(null)} className="btn-ghost">取消</button>
              <button
                onClick={() => { setConfirm(null); void doSave(true) }}
                className="inline-flex items-center gap-1.5 rounded-md bg-destructive px-3 py-1.5 text-[13px] font-medium text-destructive-foreground hover:opacity-90"
              >
                仍要保存
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function OptionsEditor({ options, onChange }: { options: FieldOption[]; onChange: (o: FieldOption[]) => void }) {
  return (
    <div className="mt-2">
      <div className="mb-1.5 text-[11px] uppercase tracking-wide text-text-tertiary">选项</div>
      <div className="flex flex-col gap-1.5">
        {options.map((o, i) => (
          <div key={i} className="flex gap-2">
            <Input value={o.label} onChange={(e) => onChange(options.map((x, j) => (j === i ? { ...x, label: e.target.value, value: x.value || e.target.value } : x)))} placeholder="显示" className="h-8 flex-1" />
            <Input value={o.value} onChange={(e) => onChange(options.map((x, j) => (j === i ? { ...x, value: e.target.value } : x)))} placeholder="value" className="h-8 w-28 font-mono" />
            <Input value={o.key ?? ''} onChange={(e) => onChange(options.map((x, j) => (j === i ? { ...x, key: e.target.value } : x)))} placeholder="键" className="h-8 w-14" />
            <button onClick={() => onChange(options.filter((_, j) => j !== i))} className="grid size-8 place-items-center rounded-md text-text-tertiary hover:text-destructive">
              <Trash2 className="size-3.5" />
            </button>
          </div>
        ))}
      </div>
      <button onClick={() => onChange([...options, { value: '', label: '' }])} className="mt-1.5 inline-flex items-center gap-1 text-[12px] text-primary hover:underline">
        <Plus className="size-3" />选项
      </button>
    </div>
  )
}
