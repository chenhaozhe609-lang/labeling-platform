function App() {
  return (
    <div className="flex min-h-svh flex-col items-center justify-center gap-4 bg-background text-foreground">
      <div className="rounded-lg border border-border bg-card px-8 py-6 text-center">
        <h1 className="text-xl font-semibold tracking-tight">数据标注平台</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          前端骨架就绪 · React + TS + Tailwind v4 + shadcn/ui
        </p>
        <p className="mt-3 font-mono text-[13px] text-text-tertiary tabular">
          下一步：标注工作台原型
        </p>
      </div>
    </div>
  )
}

export default App
