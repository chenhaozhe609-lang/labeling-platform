import { useState } from 'react'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { Database, LayoutDashboard, LogOut, Moon, PenLine, Sun } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuth } from '@/stores/auth'

export function AppShell() {
  const loc = useLocation()
  const nav = useNavigate()
  const user = useAuth((s) => s.user)
  const logout = useAuth((s) => s.logout)
  const [dark, setDark] = useState(true)

  function toggleTheme() {
    const d = !dark
    setDark(d)
    document.documentElement.classList.toggle('dark', d)
  }

  return (
    <div className="flex h-svh bg-background text-foreground">
      <nav className="flex w-14 shrink-0 flex-col items-center gap-1 border-r border-border py-3">
        <RailItem to="/datasets" active={loc.pathname.startsWith('/datasets')} label="数据集">
          <Database className="size-5" />
        </RailItem>
        <RailItem to="/workspace" active={loc.pathname.startsWith('/workspace')} label="标注">
          <PenLine className="size-5" />
        </RailItem>
        <RailItem to="/dashboard" active={loc.pathname.startsWith('/dashboard')} label="看板">
          <LayoutDashboard className="size-5" />
        </RailItem>

        <div className="mt-auto flex flex-col items-center gap-2">
          <button
            onClick={toggleTheme}
            title="切换主题"
            className="grid size-9 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-surface-3 hover:text-foreground"
          >
            {dark ? <Moon className="size-4" /> : <Sun className="size-4" />}
          </button>
          <div
            title={user?.username}
            className="grid size-8 place-items-center rounded-full bg-surface-2 text-xs font-medium uppercase"
          >
            {user?.username?.[0] ?? '?'}
          </div>
          <button
            onClick={() => {
              logout()
              nav('/login', { replace: true })
            }}
            title="退出登录"
            className="grid size-9 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
          >
            <LogOut className="size-4" />
          </button>
        </div>
      </nav>

      <main className="min-w-0 flex-1 overflow-y-auto">
        <Outlet />
      </main>
    </div>
  )
}

function RailItem({
  to,
  active,
  label,
  children,
}: {
  to: string
  active: boolean
  label: string
  children: React.ReactNode
}) {
  return (
    <Link
      to={to}
      title={label}
      className={cn(
        'relative grid size-10 place-items-center rounded-md transition-colors',
        active
          ? 'bg-surface-3 text-foreground'
          : 'text-muted-foreground hover:bg-surface-3/60 hover:text-foreground',
      )}
    >
      {active && <span className="absolute left-0 h-5 w-0.5 rounded-r bg-primary" />}
      {children}
    </Link>
  )
}
