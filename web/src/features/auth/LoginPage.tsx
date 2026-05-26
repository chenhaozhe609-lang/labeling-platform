import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Eye, EyeOff, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { login } from '@/api/auth'
import { useAuth } from '@/stores/auth'
import { landingFor } from './landing'

function errMsg(e: unknown, fallback: string): string {
  return (e as { response?: { data?: { error?: string } } }).response?.data?.error ?? fallback
}

export function LoginPage() {
  const navigate = useNavigate()
  const setAuth = useAuth((s) => s.setAuth)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [show, setShow] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await login(email.trim(), password)
      setAuth(res.access_token, res.refresh_token, res.user, res.org)
      navigate(landingFor(res.user), { replace: true })
    } catch (err) {
      setError(errMsg(err, '邮箱或密码错误'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-svh items-center justify-center bg-background">
      <form onSubmit={onSubmit} className="w-80 rounded-lg border border-border bg-card p-7">
        <div className="mb-6 text-center">
          <h1 className="font-serif text-[26px] leading-tight tracking-tight">labelo</h1>
          <p className="mt-1 text-[13px] text-muted-foreground">登录以进入工作台</p>
        </div>

        <label className="mb-1.5 block text-[13px] text-muted-foreground">邮箱</label>
        <Input
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          autoComplete="username"
          autoFocus
          className="mb-4"
        />

        <label className="mb-1.5 block text-[13px] text-muted-foreground">密码</label>
        <div className="relative mb-4">
          <Input
            type={show ? 'text' : 'password'}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
          />
          <button
            type="button"
            onClick={() => setShow((s) => !s)}
            className="absolute right-2 top-1/2 -translate-y-1/2 text-text-tertiary hover:text-foreground"
            aria-label={show ? '隐藏密码' : '显示密码'}
          >
            {show ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
          </button>
        </div>

        {error && <p className="mb-3 text-[13px] text-destructive">{error}</p>}

        <Button type="submit" disabled={loading} className="w-full">
          {loading && <Loader2 className="size-4 animate-spin" />}
          登录
        </Button>

        <p className="mt-4 text-center text-[13px] text-muted-foreground">
          还没有组织？
          <Link to="/signup" className="ml-1 text-primary hover:underline">
            创建组织
          </Link>
        </p>
      </form>
    </div>
  )
}
