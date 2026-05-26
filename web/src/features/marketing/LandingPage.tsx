import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight, ArrowUpRight, Check } from 'lucide-react'
import { cn } from '@/lib/utils'

/**
 * 着陆页（公开门面）——视觉方向 v3：编辑感极简（Editorial Minimal）。
 * 暖中性纸面 + 发丝线分层（不靠阴影）+ Instrument Serif(衬线) / Inter(无衬线) / JetBrains Mono(等宽微标签) 混排。
 * 全程浅色；不依赖全局深色 token。无玻璃/模糊/渐变/3D/WebGL。
 * 品牌名 BRAND 为占位，确定后改这一处。标题用英文衬线，其余中文 + 英文等宽小标签。
 */
const BRAND = 'labelo'
const SERIF = "'Instrument Serif', Georgia, serif"
const MONO = "'JetBrains Mono', ui-monospace, monospace"

/** 滚动进入视口时淡入上移；prefers-reduced-motion 下由全局 CSS 退化为瞬显。 */
function Reveal({ children, className, delay = 0 }: { children: React.ReactNode; className?: string; delay?: number }) {
  const ref = useRef<HTMLDivElement>(null)
  const [shown, setShown] = useState(false)
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const io = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setShown(true)
          io.disconnect()
        }
      },
      { threshold: 0.12, rootMargin: '0px 0px -8% 0px' },
    )
    io.observe(el)
    return () => io.disconnect()
  }, [])
  return (
    <div
      ref={ref}
      style={{ transitionDelay: `${delay}ms` }}
      className={cn('transition-all duration-700 ease-out', shown ? 'translate-y-0 opacity-100' : 'translate-y-3 opacity-0', className)}
    >
      {children}
    </div>
  )
}

function Eyebrow({ children }: { children: React.ReactNode }) {
  return (
    <span className="text-[11px] uppercase tracking-[0.22em] text-[#6A6157]" style={{ fontFamily: MONO }}>
      {children}
    </span>
  )
}

function InkButton({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <Link
      to={to}
      className="group inline-flex items-center gap-2 rounded-md bg-[#221F1B] px-5 py-2.5 text-sm font-medium text-[#F6F3EE] transition-colors hover:bg-[#3A352F]"
    >
      {children}
      <ArrowRight className="size-4 transition-transform group-hover:translate-x-0.5" />
    </Link>
  )
}

function TextLink({ to, children }: { to: string; children: React.ReactNode }) {
  const cls =
    'group inline-flex items-center gap-1.5 text-sm font-medium text-[#6A6157] transition-colors hover:text-[#221F1B]'
  const inner = (
    <>
      {children}
      <ArrowUpRight className="size-4 opacity-50 transition-opacity group-hover:opacity-100" />
    </>
  )
  return to.startsWith('/') ? (
    <Link to={to} className={cls}>
      {inner}
    </Link>
  ) : (
    <a href={to} className={cls}>
      {inner}
    </a>
  )
}

export function LandingPage() {
  const year = new Date().getFullYear()
  return (
    <div
      className="min-h-dvh font-sans antialiased"
      style={{ background: '#F6F3EE', color: '#221F1B', '--background': '#F6F3EE', '--ring': '#221F1B' } as React.CSSProperties}
    >
      {/* —— 顶栏（实色纸面 + 发丝下边线，编辑式）—— */}
      <header className="sticky top-0 z-50 border-b border-[#E6DFD4] bg-[#F6F3EE]">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-6">
          <a href="#top" className="text-[17px] font-semibold tracking-tight text-[#221F1B]">
            {BRAND}
          </a>
          <nav className="hidden items-center gap-9 md:flex">
            <a href="#features" className="text-sm text-[#6A6157] transition-colors hover:text-[#221F1B]">能力</a>
            <a href="#workflow" className="text-sm text-[#6A6157] transition-colors hover:text-[#221F1B]">工作流</a>
            <a href="#trust" className="text-sm text-[#6A6157] transition-colors hover:text-[#221F1B]">安全</a>
            <a href="#faq" className="text-sm text-[#6A6157] transition-colors hover:text-[#221F1B]">常见问题</a>
          </nav>
          <div className="flex items-center gap-5">
            <Link to="/login" className="text-sm text-[#6A6157] transition-colors hover:text-[#221F1B]">登录</Link>
            <InkButton to="/signup">申请试用</InkButton>
          </div>
        </div>
      </header>

      <main id="top">
        {/* —— Hero（左对齐编辑式，靠排版撑场）—— */}
        <section className="px-6 pb-20 pt-28 md:pb-28 md:pt-40">
          <div className="mx-auto max-w-5xl">
            <Reveal>
              <Eyebrow>AI-ASSISTED DATA ANNOTATION</Eyebrow>
            </Reveal>
            <Reveal delay={80}>
              <h1
                className="mt-6 max-w-3xl text-[clamp(2.75rem,7vw,5.25rem)] leading-[1.02] tracking-[-0.01em] text-[#221F1B]"
                style={{ fontFamily: SERIF }}
              >
                AI drafts. <span className="italic text-[#B06A78]">You</span> decide.
              </h1>
            </Reveal>
            <Reveal delay={150}>
              <p className="mt-7 max-w-xl text-[17px] leading-relaxed text-[#6A6157]">
                列角色补全 · LLM 预填 · 抽检审核——让 AI 起草，由你定稿，把专家的时间，留给判断本身。
              </p>
            </Reveal>
            <Reveal delay={220}>
              <div className="mt-9 flex flex-wrap items-center gap-x-6 gap-y-4">
                <InkButton to="/signup">申请试用</InkButton>
                <TextLink to="#workflow">看看怎么运转</TextLink>
              </div>
            </Reveal>
          </div>
        </section>

        {/* —— 产品演示：三栏工作台 —— */}
        <section className="px-6 pb-24">
          <div className="mx-auto max-w-6xl">
            <Reveal className="flex items-baseline gap-3 border-t border-[#E6DFD4] pt-5">
              <Eyebrow>THE WORKBENCH</Eyebrow>
              <span className="text-sm text-[#6A6157]">三栏 · 键盘优先 · 来源可追溯</span>
            </Reveal>
            <Reveal delay={120} className="mt-8">
              <Workbench />
            </Reveal>
          </div>
        </section>

        {/* —— 能力（非对称网格 + 编号）—— */}
        <section id="features" className="border-t border-[#E6DFD4] px-6 py-24 md:py-28">
          <div className="mx-auto grid max-w-6xl gap-12 md:grid-cols-[300px_1fr]">
            <Reveal>
              <Eyebrow>CAPABILITIES · 核心能力</Eyebrow>
              <h2 className="mt-4 text-[clamp(1.8rem,3vw,2.6rem)] leading-tight text-[#221F1B]" style={{ fontFamily: SERIF }}>
                不是又一个标注工具，<br />而是一条<span className="italic"> 数据补全流水线</span>。
              </h2>
            </Reveal>
            <div>
              <Reveal>
                <Capability n="01" title="列角色补全" desc="为每一列指定角色：context 喂给模型、fill 待补全、id 主键、hidden 隐藏。任务自动从有空缺的行生成。" />
              </Reveal>
              <Reveal delay={80}>
                <Capability n="02" title="LLM 预填" desc="上下文交给模型预填空缺字段。可采纳、可修改、可清空——每个值都标记来源：AI / 修订 / 人工。" />
              </Reveal>
              <Reveal delay={160}>
                <Capability n="03" title="抽检审核" desc="Reviewer 随机抽检，质量可追溯。补全后的完整表一键导出，沙箱只读、不写回原始库。" />
              </Reveal>
            </div>
          </div>
        </section>

        {/* —— 工作流（编号清单）—— */}
        <section id="workflow" className="border-t border-[#E6DFD4] px-6 py-24 md:py-28">
          <div className="mx-auto max-w-6xl">
            <Reveal>
              <Eyebrow>HOW IT WORKS · 工作流</Eyebrow>
              <h2 className="mt-4 text-[clamp(1.8rem,3vw,2.6rem)] leading-tight text-[#221F1B]" style={{ fontFamily: SERIF }}>
                上传到导出，<span className="italic">四步闭环</span>。
              </h2>
            </Reveal>
            <div className="mt-12">
              <Reveal>
                <WorkflowStep n="01" title="上传数据" desc="导入你的表格或库，自动识别列与数据类型。" />
              </Reveal>
              <Reveal delay={70}>
                <WorkflowStep n="02" title="AI 预填" desc="模型读取上下文列，为待补字段生成初稿。" />
              </Reveal>
              <Reveal delay={140}>
                <WorkflowStep n="03" title="人工精修" desc="标注员逐行采纳或修改，按列类型自动渲染录入控件。" />
              </Reveal>
              <Reveal delay={210}>
                <WorkflowStep n="04" title="抽检导出" desc="Reviewer 抽检通过，叠加生成补全后的完整表导出。" />
              </Reveal>
            </div>
          </div>
        </section>

        {/* —— 为连续工作而生 —— */}
        <section className="border-t border-[#E6DFD4] px-6 py-24 md:py-28">
          <div className="mx-auto grid max-w-6xl gap-12 md:grid-cols-[300px_1fr]">
            <Reveal>
              <Eyebrow>BUILT FOR FOCUS · 为连续工作而生</Eyebrow>
              <h2 className="mt-4 text-[clamp(1.8rem,3vw,2.6rem)] leading-tight text-[#221F1B]" style={{ fontFamily: SERIF }}>
                一整批刷下来，<span className="italic">手不离键盘</span>。
              </h2>
            </Reveal>
            <div>
              <Reveal>
                <FocusPoint kicker="KEYBOARD-FIRST" title="键盘优先" desc="翻题、采纳、跳转、⌘K 命令面板——全程不用碰鼠标，一批刷到底。" />
              </Reveal>
              <Reveal delay={80}>
                <FocusPoint kicker="DARK MODE" title="深色护眼模式" desc="工作台支持浅/深双模，专为长时间注视的标注台保留。" />
              </Reveal>
              <Reveal delay={160}>
                <FocusPoint kicker="TRACEABLE" title="来源可追溯" desc="每个填值都留有来源（AI / 修订 / 人工）与时间，质量全程留痕。" />
              </Reveal>
            </div>
          </div>
        </section>

        {/* —— 安全可信 —— */}
        <section id="trust" className="border-t border-[#E6DFD4] px-6 py-24 md:py-28">
          <div className="mx-auto max-w-6xl">
            <Reveal>
              <Eyebrow>TRUST · 安全可信</Eyebrow>
              <h2 className="mt-4 max-w-2xl text-[clamp(1.8rem,3vw,2.6rem)] leading-tight text-[#221F1B]" style={{ fontFamily: SERIF }}>
                你的数据，<span className="italic">始终在你手里</span>。
              </h2>
            </Reveal>
            <div className="mt-14 grid gap-x-10 gap-y-12 sm:grid-cols-2 lg:grid-cols-4">
              <Reveal>
                <Trust kicker="SANDBOX" title="沙箱只读" desc="标注在副本上进行，绝不写回你的原始库。" />
              </Reveal>
              <Reveal delay={80}>
                <Trust kicker="PROVENANCE" title="来源可追溯" desc="每个值标记 AI / 修订 / 人工 + 时间，质量全程留痕。" />
              </Reveal>
              <Reveal delay={160}>
                <Trust kicker="ISOLATION" title="多租户隔离" desc="数据按租户严格隔离，团队之间互不可见。" />
              </Reveal>
              <Reveal delay={240}>
                <Trust kicker="EXPORT" title="导出可控" desc="补全后的完整表一键导出，数据流向你掌控。" />
              </Reveal>
            </div>
          </div>
        </section>

        {/* —— FAQ —— */}
        <section id="faq" className="border-t border-[#E6DFD4] px-6 py-24 md:py-28">
          <div className="mx-auto max-w-3xl">
            <Reveal>
              <Eyebrow>FAQ · 常见问题</Eyebrow>
            </Reveal>
            <Reveal delay={100} className="mt-10">
              <Faq q="我的原始数据安全吗？" a="标注在只读沙箱副本上进行，绝不写回你的原始库；数据按租户严格隔离，团队之间互不可见。" />
              <Faq q="AI 预填错了怎么办？" a="预填只是初稿，可采纳、修改或清空——最终以人工定稿为准，每个值都会标记来源（AI / 修订 / 人工）。" />
              <Faq q="支持哪些数据 / 列类型？" a="表格数据，按列角色配置：context 喂给模型、fill 待补全、id 主键、hidden 隐藏；录入控件按列的数据类型自动渲染。" />
              <Faq q="质量怎么保证？" a="Reviewer 随机抽检，配合来源与时间留痕，质量全程可追溯。" />
              <Faq q="怎么导出结果？" a="抽检通过后，补全后的完整表一键导出，数据流向由你掌控。" />
            </Reveal>
          </div>
        </section>

        {/* —— 收尾 CTA —— */}
        <section className="border-t border-[#E6DFD4] px-6 py-28 text-center md:py-36">
          <Reveal>
            <h2 className="mx-auto max-w-3xl text-[clamp(2rem,4.6vw,3.4rem)] leading-[1.06] text-[#221F1B]" style={{ fontFamily: SERIF }}>
              准备好让标注，<span className="italic">回到判断本身</span>了吗？
            </h2>
            <div className="mt-9 flex flex-wrap items-center justify-center gap-x-6 gap-y-4">
              <InkButton to="/signup">申请试用</InkButton>
              <TextLink to="/login">已有账号，登录</TextLink>
            </div>
          </Reveal>
        </section>
      </main>

      {/* —— 页脚 —— */}
      <footer className="border-t border-[#E6DFD4] px-6">
        <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-4 py-10 text-sm text-[#6A6157] sm:flex-row">
          <span className="text-[15px] font-semibold tracking-tight text-[#221F1B]">{BRAND}</span>
          <span className="text-xs text-[#8A8073]" style={{ fontFamily: MONO }}>© {year} · 数据标注工作台</span>
          <div className="flex items-center gap-6">
            <a href="#features" className="transition-colors hover:text-[#221F1B]">能力</a>
            <a href="#workflow" className="transition-colors hover:text-[#221F1B]">工作流</a>
            <Link to="/login" className="transition-colors hover:text-[#221F1B]">登录</Link>
          </div>
        </div>
      </footer>
    </div>
  )
}

/* ——————————————————————————— 区块小组件 ——————————————————————————— */

function Capability({ n, title, desc }: { n: string; title: string; desc: string }) {
  return (
    <div className="grid grid-cols-[auto_1fr] gap-x-6 border-t border-[#E6DFD4] py-8 first:border-t-0 first:pt-0">
      <span className="pt-1.5 text-sm text-[#8A8073]" style={{ fontFamily: MONO }}>{n}</span>
      <div>
        <h3 className="text-xl text-[#221F1B]" style={{ fontFamily: SERIF }}>{title}</h3>
        <p className="mt-2 max-w-md text-sm leading-relaxed text-[#6A6157]">{desc}</p>
      </div>
    </div>
  )
}

function WorkflowStep({ n, title, desc }: { n: string; title: string; desc: string }) {
  return (
    <div className="grid items-baseline gap-x-8 gap-y-1.5 border-t border-[#E6DFD4] py-8 first:border-t-0 first:pt-0 md:grid-cols-[64px_200px_1fr]">
      <span className="text-sm text-[#8A8073]" style={{ fontFamily: MONO }}>{n}</span>
      <h3 className="text-lg text-[#221F1B]" style={{ fontFamily: SERIF }}>{title}</h3>
      <p className="text-sm leading-relaxed text-[#6A6157]">{desc}</p>
    </div>
  )
}

function FocusPoint({ kicker, title, desc }: { kicker: string; title: string; desc: string }) {
  return (
    <div className="border-t border-[#E6DFD4] py-7 first:border-t-0 first:pt-0">
      <div className="text-[11px] uppercase tracking-[0.18em] text-[#8A8073]" style={{ fontFamily: MONO }}>{kicker}</div>
      <h3 className="mt-2.5 text-base font-medium text-[#221F1B]">{title}</h3>
      <p className="mt-1.5 max-w-md text-sm leading-relaxed text-[#6A6157]">{desc}</p>
    </div>
  )
}

function Trust({ kicker, title, desc }: { kicker: string; title: string; desc: string }) {
  return (
    <div>
      <div className="text-[11px] uppercase tracking-[0.18em] text-[#B06A78]" style={{ fontFamily: MONO }}>{kicker}</div>
      <h3 className="mt-3 text-lg text-[#221F1B]" style={{ fontFamily: SERIF }}>{title}</h3>
      <p className="mt-2 text-sm leading-relaxed text-[#6A6157]">{desc}</p>
    </div>
  )
}

function Faq({ q, a }: { q: string; a: string }) {
  return (
    <details className="group border-t border-[#E6DFD4] py-5 first:border-t-0">
      <summary className="flex cursor-pointer list-none items-center justify-between gap-6 [&::-webkit-details-marker]:hidden">
        <span className="text-lg text-[#221F1B]" style={{ fontFamily: SERIF }}>{q}</span>
        <span
          className="text-xl leading-none text-[#8A8073] transition-transform duration-200 group-open:rotate-45"
          style={{ fontFamily: MONO }}
        >
          +
        </span>
      </summary>
      <p className="mt-3 max-w-2xl text-sm leading-relaxed text-[#6A6157]">{a}</p>
    </details>
  )
}

/* ——————————————————————————— 三栏工作台演示 ——————————————————————————— */

function Workbench() {
  return (
    <div className="overflow-hidden rounded-xl border border-[#E6DFD4] bg-white shadow-[0_1px_2px_rgba(34,31,27,0.04),0_18px_40px_-24px_rgba(34,31,27,0.18)]">
      {/* 工具条 */}
      <div className="flex items-center justify-between border-b border-[#E6DFD4] bg-[#F0EBE3] px-4 py-2.5">
        <div className="flex items-center gap-2 text-xs text-[#6A6157]">
          <span className="font-semibold tracking-tight text-[#221F1B]">{BRAND}</span>
          <span className="text-[#8A8073]">/</span>
          <span>物种分类库</span>
        </div>
        <div className="flex items-center gap-3 text-xs text-[#6A6157]">
          <span className="hidden items-center gap-1.5 sm:inline-flex">
            <span className="size-1.5 rounded-full bg-[#5E8C7B]" />
            本批 62%
          </span>
          <span className="rounded border border-[#E6DFD4] px-1.5 py-0.5 text-[10px] text-[#8A8073]" style={{ fontFamily: MONO }}>⌘K</span>
        </div>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-[180px_1fr] lg:grid-cols-[180px_1fr_184px]">
        {/* 左：任务队列 */}
        <div className="hidden flex-col gap-0.5 border-r border-[#E6DFD4] p-3 md:flex">
          <div className="mb-1.5 rounded border border-[#E6DFD4] px-2.5 py-1.5 text-[11px] text-[#8A8073]">搜索任务…</div>
          <QueueItem id="#1284" label="待补全" active />
          <QueueItem id="#1285" label="已完成" done />
          <QueueItem id="#1286" label="抽检中" />
          <QueueItem id="#1287" label="待补全" />
          <QueueItem id="#1288" label="待补全" />
        </div>
        {/* 中：当前记录 */}
        <div className="p-5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2 text-xs text-[#6A6157]">
              <span className="tabular-nums text-[#221F1B]" style={{ fontFamily: MONO }}>#1284</span>
              <span className="text-[#8A8073]">·</span>
              <span>待补全 1 / 1</span>
            </div>
            <span className="rounded-full bg-[#F0EBE3] px-2 py-0.5 text-[11px] text-[#6A6157]">context 只读</span>
          </div>
          <div className="mt-4 grid grid-cols-3 overflow-hidden rounded-lg border border-[#E6DFD4]">
            <Cell label="学名" value="Panthera uncia" />
            <Cell label="采集地" value="青藏高原" />
            <Cell label="年份" value="2023" mono last />
          </div>
          <div className="mt-5 flex items-center gap-2">
            <span className="text-xs font-medium text-[#6A6157]">保护等级</span>
            <span className="inline-flex items-center gap-1 rounded-full bg-[#8A6FB0]/10 px-2 py-0.5 text-[11px] font-medium text-[#8A6FB0]" style={{ fontFamily: MONO }}>
              AI 预填
            </span>
          </div>
          <div className="mt-2 flex items-center justify-between rounded-lg border border-[#E6DFD4] bg-[#FCFAF7] px-4 py-3">
            <span className="text-[15px] text-[#221F1B]">易危（VU）· 国家一级保护</span>
            <div className="flex items-center gap-2">
              <span className="inline-flex items-center gap-1 rounded-md bg-[#221F1B] px-2.5 py-1 text-xs font-medium text-[#F6F3EE]">
                <Check className="size-3.5" />
                采纳
              </span>
              <span className="rounded-md border border-[#E6DFD4] px-2.5 py-1 text-xs text-[#6A6157]">改</span>
            </div>
          </div>
          <div className="mt-4 flex flex-wrap items-center gap-4 text-[11px] text-[#8A8073]">
            <span className="inline-flex items-center gap-1"><Kbd>J</Kbd><Kbd>K</Kbd> 翻题</span>
            <span className="inline-flex items-center gap-1"><Kbd>Enter</Kbd> 采纳</span>
            <span className="inline-flex items-center gap-1"><Kbd>⌘K</Kbd> 命令面板</span>
          </div>
        </div>
        {/* 右：图例 */}
        <div className="hidden flex-col gap-5 border-l border-[#E6DFD4] p-4 lg:flex">
          <div>
            <div className="text-[11px] uppercase tracking-[0.16em] text-[#8A8073]" style={{ fontFamily: MONO }}>列角色</div>
            <div className="mt-2.5 flex flex-col gap-2 text-xs text-[#6A6157]">
              <Legend dot="#6F9BB5" text="context · 只读" />
              <Legend dot="#8A6FB0" text="fill · 待补全" />
              <Legend dot="#B08A4A" text="id · 主键" />
              <Legend dot="#8A8073" text="hidden · 隐藏" />
            </div>
          </div>
          <div>
            <div className="text-[11px] uppercase tracking-[0.16em] text-[#8A8073]" style={{ fontFamily: MONO }}>来源</div>
            <div className="mt-2.5 flex flex-col gap-2 text-xs text-[#6A6157]">
              <Legend dot="#8A6FB0" text="AI 预填" />
              <Legend dot="#B08A4A" text="人工修订" />
              <Legend dot="#5E8C7B" text="人工录入" />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function QueueItem({ id, label, active, done }: { id: string; label: string; active?: boolean; done?: boolean }) {
  return (
    <div className={cn('flex items-center justify-between rounded px-2.5 py-1.5 text-xs', active ? 'bg-[#F0EBE3] text-[#221F1B]' : 'text-[#6A6157]')}>
      <span className="tabular-nums" style={{ fontFamily: MONO }}>{id}</span>
      <span className="inline-flex items-center gap-1 text-[#8A8073]">
        {done && <Check className="size-3 text-[#5E8C7B]" />}
        {label}
      </span>
    </div>
  )
}

function Cell({ label, value, mono, last }: { label: string; value: string; mono?: boolean; last?: boolean }) {
  return (
    <div className={cn('bg-[#FCFAF7] px-3.5 py-3', !last && 'border-r border-[#E6DFD4]')}>
      <div className="text-[10px] uppercase tracking-wide text-[#8A8073]" style={{ fontFamily: MONO }}>{label}</div>
      <div className={cn('mt-1 text-sm text-[#221F1B]', mono && 'tabular-nums')} style={mono ? { fontFamily: MONO } : undefined}>
        {value}
      </div>
    </div>
  )
}

function Kbd({ children }: { children: React.ReactNode }) {
  return (
    <kbd
      className="inline-flex h-[18px] min-w-[18px] items-center justify-center rounded border border-[#E6DFD4] bg-[#F6F3EE] px-1 text-[10px] leading-none text-[#6A6157]"
      style={{ fontFamily: MONO }}
    >
      {children}
    </kbd>
  )
}

function Legend({ dot, text }: { dot: string; text: string }) {
  return (
    <div className="flex items-center gap-2">
      <span className="size-2 rounded-[3px]" style={{ background: dot }} />
      <span>{text}</span>
    </div>
  )
}
