import { Building2, ClipboardCheck, Database, LayoutDashboard, ListTodo, PenLine, Users } from 'lucide-react'
import type { Role } from '@/types'

// 每个角色的侧栏/命令面板导航项（AppShell 与 CommandPalette 共用，避免漂移）。
export interface NavItem {
  to: string
  label: string
  icon: typeof Database
}

export const NAV: Record<Role, NavItem[]> = {
  admin: [
    { to: '/dashboard', label: '总览', icon: LayoutDashboard },
    { to: '/datasets', label: '数据集', icon: Database },
    { to: '/review', label: '审核', icon: ClipboardCheck },
    { to: '/admin/users', label: '用户', icon: Users },
  ],
  reviewer: [
    { to: '/review', label: '审核', icon: ClipboardCheck },
    { to: '/datasets', label: '数据集', icon: Database },
    { to: '/me/tasks', label: '我的', icon: ListTodo },
  ],
  annotator: [
    { to: '/workspace', label: '标注', icon: PenLine },
    { to: '/me/tasks', label: '我的', icon: ListTodo },
    { to: '/datasets', label: '数据集', icon: Database },
  ],
}

// 平台超管导航（跨组织，不属于任何业务组织）。
export const SUPERADMIN_NAV: NavItem[] = [{ to: '/platform/orgs', label: '组织', icon: Building2 }]

export const ROLE_LABEL: Record<Role, string> = { admin: '管理员', reviewer: '审核员', annotator: '标注员' }
