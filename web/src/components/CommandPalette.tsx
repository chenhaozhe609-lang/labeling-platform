import { useEffect } from 'react'
import { Command } from 'cmdk'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Database, PenLine } from 'lucide-react'
import { listDatasets } from '@/api/datasets'
import { useAuth } from '@/stores/auth'
import { NAV, SUPERADMIN_NAV } from './nav'

// ⌘K 命令面板（B3.2）：按角色导航 + 跳数据集 + 进入标注。
export function CommandPalette({ open, onOpenChange }: { open: boolean; onOpenChange: (b: boolean) => void }) {
  const navigate = useNavigate()
  const user = useAuth((s) => s.user)
  const navItems = user?.is_superadmin ? SUPERADMIN_NAV : NAV[user?.role ?? 'annotator']
  const { data: datasets } = useQuery({ queryKey: ['datasets'], queryFn: listDatasets, enabled: open })

  // ⌘K / Ctrl+K 开关；Esc 关闭。
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) {
        e.preventDefault()
        onOpenChange(!open)
      } else if (e.key === 'Escape' && open) {
        onOpenChange(false)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onOpenChange])

  if (!open) return null

  const go = (to: string) => {
    onOpenChange(false)
    navigate(to)
  }
  const itemCls =
    'flex cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-[13px] text-muted-foreground data-[selected=true]:bg-surface-3 data-[selected=true]:text-foreground'

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-black/50 pt-[15vh]"
      onClick={() => onOpenChange(false)}
    >
      <Command
        label="命令面板"
        className="w-full max-w-lg overflow-hidden rounded-xl border border-border bg-popover shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <Command.Input
          autoFocus
          placeholder="搜索数据集 / 跳转…"
          className="w-full border-b border-border bg-transparent px-4 py-3 text-[14px] text-foreground outline-none placeholder:text-text-tertiary"
        />
        <Command.List className="max-h-80 overflow-y-auto p-2 [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1.5 [&_[cmdk-group-heading]]:text-[11px] [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:uppercase [&_[cmdk-group-heading]]:tracking-wide [&_[cmdk-group-heading]]:text-text-tertiary">
          <Command.Empty className="px-3 py-6 text-center text-[13px] text-text-tertiary">无匹配项</Command.Empty>

          <Command.Group heading="导航">
            {navItems.map((it) => (
              <Command.Item key={it.to} value={`导航 ${it.label}`} onSelect={() => go(it.to)} className={itemCls}>
                <it.icon className="size-4" />
                {it.label}
              </Command.Item>
            ))}
          </Command.Group>

          {(datasets ?? []).length > 0 && (
            <Command.Group heading="数据集">
              {(datasets ?? []).map((d) => (
                <Command.Item key={d.id} value={`数据集 ${d.name}`} onSelect={() => go(`/datasets/${d.id}`)} className={itemCls}>
                  <Database className="size-4" />
                  <span className="truncate">{d.name}</span>
                  <span className="ml-auto font-mono text-[11px] tabular text-text-tertiary">{d.completed}/{d.total_rows}</span>
                </Command.Item>
              ))}
            </Command.Group>
          )}

          {(datasets ?? []).filter((d) => d.status === 'READY' && d.pending > 0).length > 0 && (
            <Command.Group heading="开始标注">
              {(datasets ?? [])
                .filter((d) => d.status === 'READY' && d.pending > 0)
                .map((d) => (
                  <Command.Item
                    key={`label-${d.id}`}
                    value={`标注 ${d.name}`}
                    onSelect={() => go(`/workspace?dataset=${d.id}`)}
                    className={itemCls}
                  >
                    <PenLine className="size-4" />
                    标注「{d.name}」
                    <span className="ml-auto font-mono text-[11px] tabular text-text-tertiary">待领 {d.pending}</span>
                  </Command.Item>
                ))}
            </Command.Group>
          )}
        </Command.List>
      </Command>
    </div>
  )
}
