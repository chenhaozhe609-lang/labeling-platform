import { X } from 'lucide-react'
import { useSettings } from '@/stores/settings'
import { useTheme } from '@/stores/theme'
import { cn } from '@/lib/utils'

export function TweaksPanel({ open, onClose }: { open: boolean; onClose: () => void }) {
  const shortcuts = useSettings((s) => s.shortcuts)
  const density = useSettings((s) => s.density)
  const setShortcuts = useSettings((s) => s.setShortcuts)
  const setDensity = useSettings((s) => s.setDensity)
  const themePref = useTheme((s) => s.pref)
  const setThemePref = useTheme((s) => s.setPref)
  if (!open) return null

  return (
    <>
      <div className="fixed inset-0 z-40 bg-black/50" onClick={onClose} />
      <div className="fixed right-0 top-0 z-50 flex h-svh w-72 flex-col border-l border-border bg-popover p-5 animate-in slide-in-from-right-4 duration-200">
        <div className="mb-5 flex items-center justify-between">
          <h2 className="text-sm font-semibold">设置</h2>
          <button onClick={onClose} className="text-text-tertiary hover:text-foreground">
            <X className="size-4" />
          </button>
        </div>

        <Row label="主题">
          <div className="flex gap-1">
            {(
              [
                ['system', '系统'],
                ['light', '浅色'],
                ['dark', '深色'],
              ] as const
            ).map(([v, label]) => (
              <button
                key={v}
                onClick={() => setThemePref(v)}
                className={cn(
                  'rounded-md border px-2.5 py-1 text-[12px] transition-colors',
                  themePref === v ? 'border-primary bg-primary text-primary-foreground' : 'border-border text-muted-foreground hover:bg-surface-3',
                )}
              >
                {label}
              </button>
            ))}
          </div>
        </Row>

        <Row label="标注快捷键">
          <Toggle on={shortcuts} onChange={setShortcuts} />
        </Row>

        <Row label="界面密度">
          <div className="flex gap-1">
            {(['compact', 'cozy'] as const).map((d) => (
              <button
                key={d}
                onClick={() => setDensity(d)}
                className={cn(
                  'rounded-md border px-2.5 py-1 text-[12px] transition-colors',
                  density === d ? 'border-primary bg-primary text-primary-foreground' : 'border-border text-muted-foreground hover:bg-surface-3',
                )}
              >
                {d === 'compact' ? '紧凑' : '宽松'}
              </button>
            ))}
          </div>
        </Row>

        <Row label="任务池超时">
          <span className="font-mono text-[12px] tabular text-text-tertiary">30 分钟 · 服务端</span>
        </Row>

        <p className="mt-auto text-[11px] text-text-tertiary">主题默认跟随系统，可在此手动覆盖。</p>
      </div>
    </>
  )
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="mb-4 flex items-center justify-between">
      <span className="text-[13px] text-muted-foreground">{label}</span>
      {children}
    </div>
  )
}

function Toggle({ on, onChange }: { on: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      onClick={() => onChange(!on)}
      className={cn('relative h-5 w-9 rounded-full transition-colors', on ? 'bg-primary' : 'bg-surface-3')}
    >
      <span className={cn('absolute top-0.5 size-4 rounded-full bg-white transition-all', on ? 'left-4' : 'left-0.5')} />
    </button>
  )
}
