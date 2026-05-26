import { useState } from 'react'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown, LogOut, Search, Settings2 } from 'lucide-react'
import { CommandPalette } from './CommandPalette'
import { NAV, ROLE_LABEL, SUPERADMIN_NAV } from './nav'
import { cn } from '@/lib/utils'
import { useAuth } from '@/stores/auth'
import { listDatasets } from '@/api/datasets'
import { TweaksPanel } from './TweaksPanel'

export function AppShell() {
  const loc = useLocation()
  const user = useAuth((s) => s.user)
  const [tweaks, setTweaks] = useState(false)
  const [cmdk, setCmdk] = useState(false)
  const nav = user?.is_superadmin ? SUPERADMIN_NAV : NAV[user?.role ?? 'annotator']

  return (
    <div className="flex h-svh flex-col bg-background text-foreground">
      <TopBar onTweaks={() => setTweaks(true)} onSearch={() => setCmdk(true)} />
      <div className="flex min-h-0 flex-1">
        <aside className="flex w-[220px] shrink-0 flex-col border-r border-border p-2">
          {nav.map((it) => {
            const active = loc.pathname === it.to || (it.to !== '/' && loc.pathname.startsWith(it.to))
            return (
              <Link
                key={it.to}
                to={it.to}
                className={cn(
                  'relative flex items-center gap-2.5 rounded-md px-3 py-2 text-[13px] transition-colors',
                  active ? 'bg-surface-3 text-foreground' : 'text-muted-foreground hover:bg-surface-3/60 hover:text-foreground',
                )}
              >
                {active && <span className="absolute left-0 h-4 w-0.5 rounded-r bg-primary" />}
                <it.icon className="size-4" />
                {it.label}
              </Link>
            )
          })}
        </aside>
        <main className="min-w-0 flex-1 overflow-y-auto">
          <Outlet />
        </main>
      </div>
      <TweaksPanel open={tweaks} onClose={() => setTweaks(false)} />
      <CommandPalette open={cmdk} onOpenChange={setCmdk} />
    </div>
  )
}

function TopBar({ onTweaks, onSearch }: { onTweaks: () => void; onSearch: () => void }) {
  const nav = useNavigate()
  const user = useAuth((s) => s.user)
  const org = useAuth((s) => s.org)
  const logout = useAuth((s) => s.logout)

  return (
    <header className="flex h-12 shrink-0 items-center gap-3 border-b border-border px-3">
      <div className="flex items-center gap-1.5 font-semibold tracking-tight">
        <span className="grid size-5 place-items-center rounded bg-primary text-[11px] text-primary-foreground">L</span>
        labelo<span className="text-primary">.</span>
      </div>
      <div className="mx-1 h-5 w-px bg-border" />
      {/* 组织名（顶栏）：超管不属于任何组织 */}
      <span className="max-w-[180px] truncate text-[13px] font-medium" title={org?.name ?? undefined}>
        {user?.is_superadmin ? '平台超管' : (org?.name ?? '—')}
      </span>
      {!user?.is_superadmin && (
        <>
          <div className="mx-1 h-5 w-px bg-border" />
          <ProjectPicker />
        </>
      )}

      <button
        onClick={onSearch}
        className="mx-auto flex w-full max-w-md items-center gap-2 rounded-md border border-border bg-surface-1 px-2.5 py-1.5 text-left text-[13px] text-text-tertiary hover:border-primary/40 hover:bg-surface-2"
      >
        <Search className="size-3.5" />
        <span className="flex-1">搜索数据集 / 跳转…</span>
        <kbd className="rounded border border-border bg-surface-2 px-1 font-mono text-[10px]">⌘K</kbd>
      </button>

      <span className="rounded-md border border-border px-2 py-1 text-[12px] text-muted-foreground">
        {ROLE_LABEL[user?.role ?? 'annotator']}
      </span>
      <button onClick={onTweaks} title="设置" className="grid size-8 place-items-center rounded-md text-muted-foreground hover:bg-surface-3 hover:text-foreground">
        <Settings2 className="size-4" />
      </button>
      <div className="grid size-7 place-items-center rounded-full bg-surface-2 text-xs uppercase" title={user?.username}>
        {user?.username?.[0] ?? '?'}
      </div>
      <button onClick={() => { logout(); nav('/login', { replace: true }) }} title="退出" className="grid size-8 place-items-center rounded-md text-muted-foreground hover:bg-destructive/10 hover:text-destructive">
        <LogOut className="size-4" />
      </button>
    </header>
  )
}

function ProjectPicker() {
  const nav = useNavigate()
  const [open, setOpen] = useState(false)
  const { data } = useQuery({ queryKey: ['datasets'], queryFn: listDatasets })
  return (
    <div className="relative">
      <button onClick={() => setOpen((o) => !o)} className="flex items-center gap-1.5 rounded-md px-2 py-1 text-[13px] text-muted-foreground hover:bg-surface-3">
        <span className="size-2 rounded-full bg-primary" />
        切换数据集
        <ChevronDown className="size-3.5" />
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-20" onClick={() => setOpen(false)} />
          <div className="absolute left-0 top-9 z-30 w-72 rounded-lg border border-border bg-popover p-1 shadow-lg">
            {(data ?? []).map((d) => (
              <button
                key={d.id}
                onClick={() => { nav(`/datasets/${d.id}`); setOpen(false) }}
                className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-surface-3"
              >
                <span className="truncate">{d.name}</span>
                <span className="ml-auto font-mono text-[11px] tabular text-text-tertiary">{d.completed}/{d.total_rows}</span>
              </button>
            ))}
            {(data ?? []).length === 0 && <div className="px-2 py-2 text-[12px] text-text-tertiary">暂无数据集</div>}
          </div>
        </>
      )}
    </div>
  )
}
