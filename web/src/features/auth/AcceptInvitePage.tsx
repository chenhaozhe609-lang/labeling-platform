import { useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { Eye, EyeOff, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { acceptInvite } from '@/api/auth'
import { useAuth } from '@/stores/auth'
import { landingFor } from './landing'

function errMsg(e: unknown, fallback: string): string {
  return (e as { response?: { data?: { error?: string } } }).response?.data?.error ?? fallback
}

// 受邀人凭邀请链接（?token=）加入既有组织（角色取自邀请）。
export function AcceptInvitePage() {
  const navigate = useNavigate()
  const setAuth = useAuth((s) => s.setAuth)
  const [params] = useSearchParams()
  const token = params.get('token') ?? ''

  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [show, setShow] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const canSubmit = email.trim() !== '' && username.trim() !== '' && password.length >= 8

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await acceptInvite({ token, email: email.trim(), username: username.trim(), password })
      setAuth(res.access_token, res.refresh_token, res.user, res.org)
      navigate(landingFor(res.user), { replace: true })
    } catch (err) {
      setError(errMsg(err, '加入失败'))
    } finally {
      setLoading(false)
    }
  }

  if (!token) {
    return (
      <div className="flex min-h-svh items-center justify-center bg-background">
        <div className="w-80 rounded-lg border border-border bg-card p-7 text-center">
          <h1 className="mb-2 text-lg font-semibold tracking-tight">邀请链接无效</h1>
          <p className="text-[13px] text-muted-foreground">链接缺少邀请凭证，请向管理员索取新的邀请链接。</p>
          <Link to="/login" className="mt-4 inline-block text-[13px] text-primary hover:underline">
            去登录
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-svh items-center justify-center bg-background">
      <form onSubmit={onSubmit} className="w-80 rounded-lg border border-border bg-card p-7">
        <div className="mb-6 text-center">
          <h1 className="text-lg font-semibold tracking-tight">接受邀请</h1>
          <p className="mt-1 text-[13px] text-muted-foreground">设置你的账号以加入组织</p>
        </div>

        <label className="mb-1.5 block text-[13px] text-muted-foreground">邮箱（登录用）</label>
        <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" autoFocus className="mb-4" />

        <label className="mb-1.5 block text-[13px] text-muted-foreground">姓名 / 显示名</label>
        <Input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="name" className="mb-4" />

        <label className="mb-1.5 block text-[13px] text-muted-foreground">密码（≥8 位）</label>
        <div className="relative mb-4">
          <Input
            type={show ? 'text' : 'password'}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="new-password"
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

        <Button type="submit" disabled={loading || !canSubmit} className="w-full">
          {loading && <Loader2 className="size-4 animate-spin" />}
          加入组织
        </Button>
      </form>
    </div>
  )
}
