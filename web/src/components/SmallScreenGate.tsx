import { useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { Monitor } from 'lucide-react'

// 公开营销页（着陆页）需在任意尺寸可访问，豁免拦截。
const PUBLIC_PATHS = ['/']

// B2.13：标注/审核为桌面端高效流设计，窗口 <1024px 时全屏拦截。
export function SmallScreenGate() {
  const { pathname } = useLocation()
  const [narrow, setNarrow] = useState(() => typeof window !== 'undefined' && window.innerWidth < 1024)
  useEffect(() => {
    const onResize = () => setNarrow(window.innerWidth < 1024)
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  if (PUBLIC_PATHS.includes(pathname)) return null
  if (!narrow) return null
  return (
    <div className="fixed inset-0 z-[100] flex flex-col items-center justify-center gap-3 bg-background px-8 text-center">
      <Monitor className="size-10 text-muted-foreground" />
      <div className="text-lg font-semibold">请在更宽的屏幕使用</div>
      <div className="max-w-sm text-sm text-muted-foreground">
        标注 / 审核工作流为桌面端键盘高效操作设计，建议窗口宽度 ≥ 1024px。
      </div>
    </div>
  )
}
