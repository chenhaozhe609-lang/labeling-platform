import { cn } from '@/lib/utils'

/**
 * 编辑式页眉：mono 大写 eyebrow + 衬线标题（+ 可选描述 / 右侧操作）。
 * 跨管理屏统一调用，作为编辑感极简的杠杆点。
 */
export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
  className,
}: {
  eyebrow?: string
  title: React.ReactNode
  description?: React.ReactNode
  actions?: React.ReactNode
  className?: string
}) {
  return (
    <header className={cn('mb-6 flex items-start justify-between gap-4', className)}>
      <div className="min-w-0">
        {eyebrow && (
          <div className="mb-1.5 font-mono text-[11px] uppercase tracking-[0.18em] text-text-tertiary">{eyebrow}</div>
        )}
        <h1 className="font-serif text-[27px] leading-[1.1] tracking-tight text-foreground">{title}</h1>
        {description && <p className="mt-2 max-w-prose text-[13px] leading-relaxed text-text-tertiary">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </header>
  )
}
