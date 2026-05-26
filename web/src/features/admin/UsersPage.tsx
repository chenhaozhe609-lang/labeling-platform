import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Copy, Loader2, Trash2, UserPlus } from 'lucide-react'
import {
  createInvite,
  createUser,
  deleteInvite,
  deleteUser,
  listInvites,
  listUsers,
  updateUser,
} from '@/api/admin'
import { useAuth } from '@/stores/auth'
import type { Invite, Role } from '@/types'

const ROLES: Role[] = ['annotator', 'reviewer', 'admin']
const ROLE_LABEL: Record<Role, string> = { admin: '管理员', reviewer: '审核员', annotator: '标注员' }

function errMsg(e: unknown, fallback: string): string {
  return (e as { response?: { data?: { error?: string } } }).response?.data?.error ?? fallback
}

export function UsersPage() {
  const qc = useQueryClient()
  const meId = useAuth((s) => s.user?.id)
  const { data, isLoading, error } = useQuery({ queryKey: ['users'], queryFn: listUsers })

  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<Role>('annotator')

  const invalidate = () => qc.invalidateQueries({ queryKey: ['users'] })

  const create = useMutation({
    mutationFn: () => createUser(username.trim(), email.trim(), password, role),
    onSuccess: () => {
      toast.success('已创建用户')
      setUsername('')
      setEmail('')
      setPassword('')
      setRole('annotator')
      invalidate()
    },
    onError: (e) => toast.error(errMsg(e, '创建失败')),
  })

  const changeRole = useMutation({
    mutationFn: ({ id, role }: { id: number; role: Role }) => updateUser(id, { role }),
    onSuccess: () => {
      toast.success('已更新角色')
      invalidate()
    },
    onError: (e) => toast.error(errMsg(e, '更新失败')),
  })

  const remove = useMutation({
    mutationFn: (id: number) => deleteUser(id),
    onSuccess: () => {
      toast.success('已删除')
      invalidate()
    },
    onError: (e) => toast.error(errMsg(e, '删除失败')),
  })

  if (isLoading) return <Pad>加载中…</Pad>
  if (error || !data) return <Pad>加载失败（需管理员）</Pad>

  const canSubmit = username.trim() !== '' && email.trim() !== '' && password.length >= 8

  return (
    <div className="mx-auto max-w-3xl px-8 py-8">
      <h1 className="mb-6 text-xl font-semibold tracking-tight">用户管理</h1>

      {/* 邀请成员 */}
      <InvitesSection />

      {/* 新建 */}
      <section className="mb-6 rounded-lg border border-border bg-card p-4">
        <div className="mb-3 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">直接新建用户</div>
        <div className="flex flex-wrap items-center gap-2">
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="姓名 / 显示名"
            className="h-9 flex-1 rounded-md border border-border bg-surface-1 px-3 text-[13px] outline-none focus:border-primary"
          />
          <input
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            type="email"
            placeholder="邮箱（登录用）"
            className="h-9 flex-1 rounded-md border border-border bg-surface-1 px-3 text-[13px] outline-none focus:border-primary"
          />
          <input
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            type="password"
            placeholder="密码（≥8位）"
            className="h-9 flex-1 rounded-md border border-border bg-surface-1 px-3 text-[13px] outline-none focus:border-primary"
          />
          <select
            value={role}
            onChange={(e) => setRole(e.target.value as Role)}
            className="h-9 rounded-md border border-border bg-surface-1 px-2 text-[13px] outline-none focus:border-primary"
          >
            {ROLES.map((r) => (
              <option key={r} value={r}>{ROLE_LABEL[r]}</option>
            ))}
          </select>
          <button onClick={() => create.mutate()} disabled={!canSubmit || create.isPending} className="btn-primary">
            {create.isPending ? <Loader2 className="size-3.5 animate-spin" /> : <UserPlus className="size-3.5" />}
            创建
          </button>
        </div>
      </section>

      {/* 列表 */}
      <section className="rounded-lg border border-border bg-card p-4">
        <table className="w-full text-[13px]">
          <thead>
            <tr className="text-left text-[11px] uppercase tracking-wide text-text-tertiary">
              <th className="pb-2 font-medium">#</th>
              <th className="pb-2 font-medium">用户名</th>
              <th className="pb-2 font-medium">角色</th>
              <th className="pb-2 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {data.map((u) => {
              const isSelf = u.id === meId
              return (
                <tr key={u.id} className="border-t border-border-subtle">
                  <td className="py-2 font-mono tabular text-text-tertiary">{u.id}</td>
                  <td className="py-2">
                    <span className="inline-flex items-center gap-2">
                      <span className="grid size-6 place-items-center rounded-full bg-surface-2 text-[11px] uppercase">{u.username[0]}</span>
                      <span className="flex flex-col leading-tight">
                        <span className="inline-flex items-center gap-1.5">
                          {u.username}
                          {isSelf && <span className="rounded bg-surface-2 px-1.5 text-[10px] text-text-tertiary">我</span>}
                        </span>
                        <span className="font-mono text-[11px] text-text-tertiary">{u.email}</span>
                      </span>
                    </span>
                  </td>
                  <td className="py-2">
                    <select
                      value={u.role}
                      onChange={(e) => changeRole.mutate({ id: u.id, role: e.target.value as Role })}
                      className="h-7 rounded-md border border-border bg-surface-1 px-2 text-[12px] outline-none focus:border-primary"
                    >
                      {ROLES.map((r) => (
                        <option key={r} value={r}>{ROLE_LABEL[r]}</option>
                      ))}
                    </select>
                  </td>
                  <td className="py-2 text-right">
                    <button
                      onClick={() => {
                        if (confirm(`删除用户 ${u.username}？`)) remove.mutate(u.id)
                      }}
                      disabled={isSelf || remove.isPending}
                      title={isSelf ? '不能删除自己' : '删除'}
                      className="inline-grid size-7 place-items-center rounded-md text-muted-foreground hover:bg-destructive/10 hover:text-destructive disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-muted-foreground"
                    >
                      <Trash2 className="size-3.5" />
                    </button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </section>
    </div>
  )
}

// InvitesSection 邀请成员：生成邀请链接（角色 + 可选限定邮箱），列出/撤销未接受的邀请。
function InvitesSection() {
  const qc = useQueryClient()
  const { data: invites } = useQuery({ queryKey: ['invites'], queryFn: listInvites })
  const [role, setRole] = useState<Role>('annotator')
  const [email, setEmail] = useState('')

  const invalidate = () => qc.invalidateQueries({ queryKey: ['invites'] })

  const create = useMutation({
    mutationFn: () => createInvite(role, email.trim() || undefined),
    onSuccess: (res) => {
      const url = window.location.origin + res.accept_path
      void navigator.clipboard?.writeText(url).catch(() => {})
      toast.success('邀请链接已生成并复制到剪贴板')
      setEmail('')
      invalidate()
    },
    onError: (e) => toast.error(errMsg(e, '生成失败')),
  })

  const revoke = useMutation({
    mutationFn: (id: number) => deleteInvite(id),
    onSuccess: () => {
      toast.success('已撤销邀请')
      invalidate()
    },
    onError: (e) => toast.error(errMsg(e, '撤销失败')),
  })

  const pending = (invites ?? []).filter((i) => !i.accepted_at)

  return (
    <section className="mb-6 rounded-lg border border-border bg-card p-4">
      <div className="mb-3 text-[11px] font-medium uppercase tracking-wide text-text-tertiary">邀请成员</div>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <select
          value={role}
          onChange={(e) => setRole(e.target.value as Role)}
          className="h-9 rounded-md border border-border bg-surface-1 px-2 text-[13px] outline-none focus:border-primary"
        >
          {ROLES.map((r) => (
            <option key={r} value={r}>{ROLE_LABEL[r]}</option>
          ))}
        </select>
        <input
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          type="email"
          placeholder="限定邮箱（可选）"
          className="h-9 flex-1 rounded-md border border-border bg-surface-1 px-3 text-[13px] outline-none focus:border-primary"
        />
        <button onClick={() => create.mutate()} disabled={create.isPending} className="btn-primary">
          {create.isPending ? <Loader2 className="size-3.5 animate-spin" /> : <Copy className="size-3.5" />}
          生成邀请链接
        </button>
      </div>

      {pending.length > 0 && (
        <ul className="flex flex-col gap-1.5">
          {pending.map((inv) => (
            <InviteRow key={inv.id} inv={inv} onRevoke={() => revoke.mutate(inv.id)} disabled={revoke.isPending} />
          ))}
        </ul>
      )}
    </section>
  )
}

function InviteRow({ inv, onRevoke, disabled }: { inv: Invite; onRevoke: () => void; disabled: boolean }) {
  const url = `${window.location.origin}/accept-invite?token=${inv.token}`
  return (
    <li className="flex items-center gap-2 rounded-md border border-border-subtle bg-surface-1 px-3 py-1.5 text-[12px]">
      <span className="rounded bg-surface-2 px-1.5 py-0.5 text-[11px] text-muted-foreground">{ROLE_LABEL[inv.role]}</span>
      {inv.email && <span className="text-text-tertiary">{inv.email}</span>}
      <span className="ml-auto truncate font-mono text-[11px] text-text-tertiary" title={url}>
        …{inv.token.slice(0, 8)}
      </span>
      <button
        onClick={() => {
          void navigator.clipboard?.writeText(url).catch(() => {})
          toast.success('已复制邀请链接')
        }}
        title="复制邀请链接"
        className="grid size-7 place-items-center rounded-md text-muted-foreground hover:bg-surface-3 hover:text-foreground"
      >
        <Copy className="size-3.5" />
      </button>
      <button
        onClick={onRevoke}
        disabled={disabled}
        title="撤销邀请"
        className="grid size-7 place-items-center rounded-md text-muted-foreground hover:bg-destructive/10 hover:text-destructive disabled:opacity-40"
      >
        <Trash2 className="size-3.5" />
      </button>
    </li>
  )
}

function Pad({ children }: { children: React.ReactNode }) {
  return <div className="px-8 py-8 text-sm text-muted-foreground">{children}</div>
}
