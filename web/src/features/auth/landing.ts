import type { User } from '@/types'

// 登录/注册/接受邀请成功后的落地路由：超管 → 平台区；admin → 数据集；其余 → 标注台。
export function landingFor(user: User): string {
  if (user.is_superadmin) return '/platform/orgs'
  if (user.role === 'admin') return '/datasets'
  return '/workspace'
}
