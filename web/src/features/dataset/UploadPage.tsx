import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, FileUp, Loader2 } from 'lucide-react'
import { uploadDataset } from '@/api/datasets'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export function UploadPage() {
  const nav = useNavigate()
  const qc = useQueryClient()
  const [name, setName] = useState('')
  const [file, setFile] = useState<File | null>(null)

  const m = useMutation({
    mutationFn: () => uploadDataset(name.trim(), file!),
    onSuccess: (detail) => {
      qc.invalidateQueries({ queryKey: ['datasets'] })
      nav(`/datasets/${detail.dataset.id}`, { replace: true })
    },
  })

  const errMsg =
    (m.error as { response?: { data?: { error?: string } } } | null)?.response?.data?.error ??
    (m.error ? '上传失败' : '')

  return (
    <div className="mx-auto max-w-xl px-8 py-8">
      <Link to="/datasets" className="mb-6 inline-flex items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" />
        数据集
      </Link>
      <PageHeader eyebrow="NEW DATASET" title="新建数据集" />

      <label className="mb-1.5 block text-[13px] text-muted-foreground">数据集名称</label>
      <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="如：新闻分类" className="mb-5" />

      <label className="mb-1.5 block text-[13px] text-muted-foreground">数据 dump 文件</label>
      <label className="mb-2 flex cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border-strong bg-card/50 px-4 py-8 text-center transition-colors hover:border-primary/50">
        <FileUp className="size-6 text-text-tertiary" />
        {file ? (
          <span className="text-sm">{file.name} · {(file.size / 1024).toFixed(0)} KB</span>
        ) : (
          <span className="text-sm text-muted-foreground">点击选择 .sql / .backup / .dump 文件</span>
        )}
        <input
          type="file"
          accept=".sql,.backup,.dump"
          className="hidden"
          onChange={(e) => setFile(e.target.files?.[0] ?? null)}
        />
      </label>
      <p className="mb-5 text-[12px] text-text-tertiary">
        上传后自动：沙箱恢复 → 反射生成表单 → 生成标注任务。大文件可能需要数十秒。
      </p>

      {errMsg && <p className="mb-3 text-[13px] text-destructive">{errMsg}</p>}

      <Button
        onClick={() => m.mutate()}
        disabled={!name.trim() || !file || m.isPending}
        className="w-full"
      >
        {m.isPending ? <Loader2 className="size-4 animate-spin" /> : <FileUp className="size-4" />}
        {m.isPending ? '导入中…（恢复 + 反射 + 生成任务）' : '上传并导入'}
      </Button>
    </div>
  )
}
