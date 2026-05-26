import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Eye, EyeOff, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { signup } from '@/api/auth'
import { useAuth } from '@/stores/auth'
import { landingFor } from './landing'

function errMsg(e: unknown, fallback: string): string {
  return (e as { response?: { data?: { error?: string } } }).response?.data?.error ?? fallback
}

// 开放注册：建组织，注册人即该组织管理员。
export function SignupPage() {
  const navigate = useNavigate()
  const setAuth = useAuth((s) => s.setAuth)
  const [orgName, setOrgName] = useState('')
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [show, setShow] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const canSubmit =
    orgName.trim() !== '' && email.trim() !== '' && username.trim() !== '' && password.length >= 8

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await signup({
        org_name: orgName.trim(),
        email: email.trim(),
        username: username.trim(),
        password,
      })
      setAuth(res.access_token, res.refresh_token, res.user, res.org)
      navigate(landingFor(res.user), { replace: true })
    } catch (err) {
      setError(errMsg(err, '注册失败'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-svh items-center justify-center bg-background">
      <form onSubmit={onSubmit} className="w-80 rounded-lg border border-border bg-card p-7">
        <div className="mb-6 text-center">
          <h1 className="text-lg font-semibold tracking-tight">创建组织</h1>
          <p className="mt-1 text-[13px] text-muted-foreground">你将成为该组织的管理员</p>
        </div>

        <label className="mb-1.5 block text-[13px] text-muted-foreground">组织名称</label>
        <Input value={orgName} onChange={(e) => setOrgName(e.target.value)} autoFocus className="mb-4" />

        <label className="mb-1.5 block text-[13px] text-muted-foreground">邮箱（登录用）</label>
        <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" className="mb-4" />

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
          创建组织并进入
        </Button>

        <p className="mt-4 text-center text-[13px] text-muted-foreground">
          已有账号？
          <Link to="/login" className="ml-1 text-primary hover:underline">
            去登录
          </Link>
        </p>
      </form>
    </div>
  )
}
