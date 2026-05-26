import { useQuery } from '@tanstack/react-query'
import { listOrgs } from '@/api/platform'
import { PageHeader } from '@/components/PageHeader'

// 平台超管：组织列表（跨组织运营/排障）。
export function PlatformOrgsPage() {
  const { data, isLoading, error } = useQuery({ queryKey: ['platform-orgs'], queryFn: listOrgs })

  if (isLoading) return <Pad>加载中…</Pad>
  if (error || !data) return <Pad>加载失败（需平台超管）</Pad>

  return (
    <div className="mx-auto max-w-3xl px-8 py-8">
      <PageHeader eyebrow="PLATFORM · ORGS" title="组织" description={`平台超管视图 · 共 ${data.length} 个组织`} />

      <section className="rounded-lg border border-border bg-card p-4">
        <table className="w-full text-[13px]">
          <thead>
            <tr className="text-left text-[11px] uppercase tracking-wide text-text-tertiary">
              <th className="pb-2 font-medium">#</th>
              <th className="pb-2 font-medium">名称</th>
              <th className="pb-2 font-medium">Owner</th>
              <th className="pb-2 text-right font-medium">创建时间</th>
            </tr>
          </thead>
          <tbody>
            {data.map((o) => (
              <tr key={o.id} className="border-t border-border-subtle">
                <td className="py-2 font-mono tabular text-text-tertiary">{o.id}</td>
                <td className="py-2">{o.name}</td>
                <td className="py-2 font-mono tabular text-text-tertiary">{o.owner_id ?? '—'}</td>
                <td className="py-2 text-right font-mono tabular text-text-tertiary">
                  {new Date(o.created_at).toLocaleDateString()}
                </td>
              </tr>
            ))}
            {data.length === 0 && (
              <tr>
                <td colSpan={4} className="py-4 text-center text-text-tertiary">
                  暂无组织
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </section>
    </div>
  )
}

function Pad({ children }: { children: React.ReactNode }) {
  return <div className="px-8 py-8 text-sm text-muted-foreground">{children}</div>
}
