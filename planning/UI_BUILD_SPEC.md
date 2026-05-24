# 数据标注平台 · 前端构建规格（Build Spec）

> 版本：v1 · 配套 `PRD.md` 与 `UI_UX_DESIGN.md`
> 作用：把设计意图固化成**可直接喂给 Claude / 工程师生成一致前端**的工程基准。
> 关系：`UI_UX_DESIGN.md` 讲「做成什么样、怎么交互」；**本文件讲「用什么栈、token 多少、组件几态、数据什么形状」**。
>
> ⚠️ **本文件 §1 的技术栈决策正式取代 PRD §6.2 / §18(M1) 中的「Antd + Formily」方案。** 后续以本文件为准。

## 目录
1. 技术栈锁定（解决冲突）
2. Design Tokens（颜色/间距/圆角/阴影/z-index/动效/字号）
3. Tailwind & CSS 变量落地
4. 组件级规格 + 全状态
5. 动态表单：widget → 组件映射 + 渲染契约
6. TypeScript 数据契约
7. Mock 数据
8. 微文案与图标映射
9. 目录结构（前端，修订版）
10. 交付清单（Definition of Done）

---

## 1. 技术栈锁定

| 关注点 | 选型 | 说明 |
|---|---|---|
| 构建 | **Vite + React 18 + TypeScript** | 沿用 PRD |
| 样式 | **Tailwind CSS v3** + CSS 变量 | token 用 CSS 变量，便于深/浅主题切换 |
| 组件库 | **shadcn/ui**（底层 Radix UI） | 复制式组件，深色易主题化，最快达 Linear/Cursor 质感 |
| 路由 | **react-router v6** | |
| 客户端状态 | **zustand** | 沿用 PRD（auth、当前任务、草稿） |
| 服务端状态 | **TanStack Query (react-query)** | claim/submit/查询缓存/重试；配 **axios** 实例 |
| 表单引擎 | **react-hook-form + zod** | 由 `form_schema` 动态构建；**弃用 Formily** |
| 图标 | **lucide-react** | 唯一图标集，stroke 1.5 |
| 命令面板 | **cmdk**（shadcn `Command`） | ⌘K |
| Toast | **sonner** | 支持 Undo action |
| 图表（看板） | **Recharts** | 轻量、React 原生 |
| 字体 | **Inter** + **JetBrains Mono** | 自托管或 Google Fonts，`font-display: swap` |

> 为什么弃 Antd/Formily：Antd 控件交互（必须点开下拉、无单键选项、企业外观）与本平台「键盘优先 / 沉浸式 / 深色」目标冲突；shadcn 是 headless，可完全定制单键选择与深色主题。详见 `UI_UX_DESIGN.md` §9.1。

shadcn 需要的组件（`npx shadcn@latest add ...`）：
`button input textarea select command dialog sheet popover tooltip sonner slider toggle-group badge card progress avatar tabs scroll-area separator skeleton`

---

## 2. Design Tokens

### 2.1 颜色（深色主力，token 名对齐 shadcn 约定 + 业务语义扩展）

> 数值沿用 `UI_UX_DESIGN.md` §11，此处补齐 shadcn 映射与语义色全集。

| 语义 | Hex | shadcn / 自定义变量 | 用途 |
|---|---|---|---|
| 画布 | `#0C0E12` | `--background` | 全局底 |
| 面 1 | `#14171C` | `--card` / `--surface-1` | 左右栏、卡片底 |
| 面 2 | `#1B1F26` | `--surface-2` | 字段组、抬升卡 |
| 面 3 | `#232830` | `--popover` / `--surface-3` | hover、浮层、命令面板 |
| 边框淡 | `#20242B` | `--border-subtle` | 分隔线 |
| 边框 | `#2C313A` | `--border` / `--input` | 卡片/输入边框 |
| 边框强 | `#3A414C` | `--border-strong` | 强调边框 |
| 正文 | `#E6E9EF` | `--foreground` | 标题/正文（~14:1） |
| 次要 | `#A0A7B4` | `--muted-foreground` | 次要文字（~7:1） |
| 弱 | `#6B7280` | `--text-tertiary` | 元信息/占位（~4.6:1） |
| 焦点/品牌 | `#818CF8` | `--primary` / `--ring` | 焦点环、active、选中、主按钮 |
| 成功/提交 | `#3FB950` | `--success` | 提交完成、READY、approve |
| 警示 | `#E3B341` | `--warning` | lease<5min、离线 |
| 危险 | `#F0626B` | `--destructive` | needs_redo、release、删除 |
| AI | `#A78BFA` | `--ai` | AI 建议、reasoning |

前景配对（按钮文字色）：`--primary-foreground:#0C0E12`、`--success-foreground:#0C0E12`、`--destructive-foreground:#FFFFFF`。

> 浅色主题（管理区可选）：另起一套变量，正文 `#1A1D23` on `#FFFFFF`，**独立验证对比**，不由深色反相。

### 2.2 间距（4px 基准；用法分级）

| 层级 | 值 | 场景 |
|---|---|---|
| 组件内 | 4 / 8 / 12 | 图标-文字间隙、输入内边距、chip 间距 |
| 组件间 | 16 / 20 | 字段之间、卡内元素 |
| 区块间 | 24 / 32 | 卡片之间、区块标题与内容 |
| 页面级 | 40 / 48 | 页面外边距、栏间距 |

固定尺寸：Rail 48px · 顶栏 48px · 底栏 36px · 左栏 280px · 右栏 380px · 阅读区 max 720px(measure 65–75ch)。

### 2.3 圆角 · 阴影 · z-index

```
圆角  --radius: 8px   →  sm 6 · md 8 · lg 12 · pill 9999
阴影（深色靠面提亮，阴影克制）
  shadow-sm  0 1px 2px rgba(0,0,0,.30)
  shadow-md  0 4px 12px rgba(0,0,0,.35)          ← popover/dropdown
  shadow-lg  0 12px 32px rgba(0,0,0,.45)         ← modal/命令面板
  focus-ring 0 0 0 2px var(--ring) (offset 2px var(--background))
z-index   base 0 · sticky 10 · dropdown 20 · overlay 30 · modal 40 · toast 50 · palette 60
```

### 2.4 动效（exit 取 enter 的 ~70%；尊重 prefers-reduced-motion）

| token | 值 | 用途 |
|---|---|---|
| `--dur-fast` | 120ms | hover、按压反馈 |
| `--dur-base` | 180ms | 进入、展开、toast |
| `--dur-slow` | 240ms | 任务切换滑入 |
| ease-out | `cubic-bezier(.16,1,.3,1)` | 进入 |
| ease-in | `cubic-bezier(.4,0,1,1)` | 退出 |
| spring | `cubic-bezier(.34,1.56,.64,1)` | 卡片按压回弹 |

只用 `transform`/`opacity` 动画；任务切换用 crossfade + 右移 8px 入场。

### 2.5 字号（px / line-height / weight / tracking）

| 角色 | 字号/行高 | 字重 | 字距 | 字体 |
|---|---|---|---|---|
| display | 30 / 36 | 600 | -0.5 | Inter |
| h1 | 24 / 32 | 600 | -0.3 | Inter |
| h2 | 20 / 28 | 600 | -0.2 | Inter |
| h3 | 16 / 24 | 600 | 0 | Inter |
| **reading**（中栏正文） | **17 / 1.7** | 400 | 0 | Inter |
| body（UI 默认） | 14 / 20 | 400 | 0 | Inter |
| sm | 13 / 18 | 400 | 0 | Inter |
| label（大写小标签） | 12 / 16 | 500 | +0.6, uppercase | Inter |
| mono（pk/hash/计时器/JSON） | 13 / 18 | 400 | 0 | JetBrains Mono, `font-variant-numeric: tabular-nums` |

---

## 3. Tailwind & CSS 变量落地

`globals.css`（深色为 `:root`，浅色挂 `.light`）：

```css
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap');

:root {
  --background:#0C0E12; --foreground:#E6E9EF;
  --card:#14171C; --surface-2:#1B1F26; --popover:#232830;
  --muted-foreground:#A0A7B4; --text-tertiary:#6B7280;
  --border:#2C313A; --border-subtle:#20242B; --border-strong:#3A414C; --input:#2C313A;
  --primary:#818CF8; --primary-foreground:#0C0E12; --ring:#818CF8;
  --success:#3FB950; --warning:#E3B341; --destructive:#F0626B; --destructive-foreground:#fff; --ai:#A78BFA;
  --radius:8px;
}
* { border-color: var(--border); }
body { background:var(--background); color:var(--foreground); font-family:Inter,system-ui,sans-serif; font-size:14px; }
:focus-visible { outline:none; box-shadow:0 0 0 2px var(--background),0 0 0 4px var(--ring); }
@media (prefers-reduced-motion: reduce){ *{animation-duration:.01ms!important;transition-duration:.01ms!important} }
```

`tailwind.config.ts` extend：

```ts
extend: {
  colors: {
    background:'var(--background)', foreground:'var(--foreground)',
    card:'var(--card)', surface2:'var(--surface-2)', popover:'var(--popover)',
    muted:'var(--muted-foreground)', tertiary:'var(--text-tertiary)',
    border:'var(--border)', borderSubtle:'var(--border-subtle)',
    primary:'var(--primary)', success:'var(--success)', warning:'var(--warning)',
    destructive:'var(--destructive)', ai:'var(--ai)',
  },
  fontFamily:{ sans:['Inter','system-ui','sans-serif'], mono:['JetBrains Mono','monospace'] },
  borderRadius:{ sm:'6px', DEFAULT:'8px', lg:'12px' },
  transitionTimingFunction:{ out:'cubic-bezier(.16,1,.3,1)', spring:'cubic-bezier(.34,1.56,.64,1)' },
  zIndex:{ sticky:'10',dropdown:'20',overlay:'30',modal:'40',toast:'50',palette:'60' },
}
```

---

## 4. 组件级规格 + 全状态

> 通用：所有可交互元素 `cursor-pointer`；焦点态用统一 `:focus-visible` ring；禁用态 opacity .45 + `cursor-not-allowed` + `aria-disabled`；过渡 `--dur-fast`。

### 4.1 Button

| 变体 | 用途 | 默认 | hover | active | disabled | loading |
|---|---|---|---|---|---|---|
| `primary` | 一般主操作 | bg `--primary`/字深 | 亮度 +6% | scale .98 | opacity .45 | spinner+禁点 |
| `success` | 提交/approve | bg `--success`/字深 | +6% | .98 | — | spinner |
| `destructive` | 打回/释放/删除 | bg `--destructive`/字白 | +6% | .98 | — | spinner |
| `secondary` | 次操作 | bg `--surface-2`/边框 | bg `--popover` | — | — | — |
| `ghost` | 图标/低强度 | 透明 | bg rgba(255,255,255,.06) | — | — | — |

尺寸：`sm h-7(28) px-2.5 text-[13px]` · `md h-8(32) px-3 text-sm` · `lg h-10(40) px-4 text-sm`。图标按钮正方 + `aria-label`，hitslop≥44。

### 4.2 Input / Textarea

状态：default(边 `--input`) · focus(ring `--primary`) · error(边+ring `--destructive`，下方红字) · disabled · readonly(无边框、`--muted` 字，区别于 disabled)。
高度 `h-8`(32)；Textarea 自增高(min 3 行)、右下角字数 `n/max`（接近上限变 `--warning`）。

### 4.3 ToggleGroup（Segmented，枚举 ≤5 的首选）

单选；选项常驻横排；每项左上角浮 `1`–`N` 角标（READING/WIDGET 焦点态下单键选中）。
状态：未选(透明+边框、字 `--muted`) · hover(bg .06) · **selected(bg `--primary`/字深)** · disabled。
选中后自动 `Tab` 到下一字段（可配 `autoAdvance`）。

### 4.4 Combobox（枚举 >5，Command 实现）

闭合时像 Input；聚焦/输入弹下拉，type-ahead 过滤，↑↓ 选，`⏎` 定，`Esc` 关。空结果给「无匹配」。

### 4.5 TagInput（multi-select / tags）

已选项为 chip（`--surface-2` bg、pill、右侧 × ）；输入框 type-ahead，`⏎` 加、`Backspace` 删末项。超出换行。

### 4.6 Slider（Confidence）

轨道 `--surface-2`、已填 `--primary`、thumb 14px 圆 + focus ring；右侧 mono 显当前值（0.0–1.0）。
配三个快捷 chip：`低 .3 / 中 .6 / 高 .9`（单键 `Q/W/E` 可映射）。

### 4.7 RatingControl（1–5）

5 段 Segmented 或星级，`1`–`5` 单键选；当前值实色填充至该档；hover 预览。

### 4.8 Badge（状态/计数）

形态：dot+文字（状态）/ pill（计数）。状态色：READY/DONE→success/ai；IMPORTING→primary 脉冲；PAUSED→muted；FAILED/NEEDS_REDO→destructive。**始终带文字，不靠颜色单一编码。**

### 4.9 Card / FieldGroup

bg `--card`、边 `--border`、radius lg、padding 16；hover 抬升 = 边框转 `--border-strong`（**不位移**）。
FieldGroup：标题用 label 样式（大写小字）；`核心`常显、`补充`可折叠（chevron + `Space`/点击）。

### 4.10 Toast（sonner）

变体 success/error/warning/info（左色条 + Lucide 图标）；位置右下；自动消 4s；可带 `撤销` 按钮（释放任务/删除）；`aria-live="polite"`，不抢焦点。

### 4.11 InlineBanner

就地反馈（字段下方错误、顶栏离线条）。变体 error/warning/info；不自动消，随状态变化。

### 4.12 LeaseTimer

mono tabular `MM:SS`。`>5min` 字 `--muted` · `≤5min` 字 `--warning` + 轻脉冲 + 旁现「延长」按钮 · `≤1min` 字 `--destructive` · 到期 → 阻断输入 + InlineBanner「租约已过期，请重新领取」。

### 4.13 AutosaveIndicator

圆点 + 文字三态：`editing…`(muted) → `saving…`(primary 旋转点) → `● saved`(success)；崩溃恢复显 `已恢复草稿`。

### 4.14 ProgressRing / CoverageBar

Ring：donut，已完成 `--success`，轨道 `--surface-2`，中心 mono 百分比。
Bar：横向，分段色（PENDING muted / CLAIMED primary / DONE success），带图例。

### 4.15 CommandPalette（⌘K）

居中浮层（`--popover` + shadow-lg + 背景 scrim 50%）；顶部搜索框，分组结果（数据集 / 导航 / 动作 / 快捷键）；↑↓ 选 `⏎` 执行 `Esc` 关；右侧显对应快捷键 `Kbd`。

### 4.16 ActivityRail

48px 竖条；项 = 图标 + tooltip；active 态：左缘 2px `--primary` 条 + 图标实色 + bg .06；底部头像/主题/帮助。Workspace 路由下默认 `w-0` 收起，hover 左缘 4px 触发区或 `\` 展开。

### 4.17 Kbd（键帽）

`min-w-5 h-5 px-1 rounded-sm bg-surface2 border text-[11px] mono` 居中；用于底栏提示与命令面板。

---

## 5. 动态表单：widget → 组件映射 + 渲染契约

| `form_schema` widget | 组件 | 键盘 |
|---|---|---|
| `Select`（options ≤5） | ToggleGroup(4.3) | `1`–`N` |
| `Select`（options >5） | Combobox(4.4) | type-ahead |
| `MultiSelect` / tags | TagInput(4.5) | `⏎`/`Backspace` |
| `Rating` | RatingControl(4.7) | `1`–`5` |
| `Confidence` | Slider(4.6) | `Q/W/E` 预设 |
| `TextArea` | Textarea(4.2) | `⌘⏎` 提交 |
| `Input` | Input(4.2) | — |
| `InputNumber` | Input[number](4.2) | — |
| `Switch` | ToggleGroup 二段 | `Y/N` |
| `DatePicker` | shadcn Popover+日历 | — |
| `RelationLink` | Combobox 异步(4.4) | type-ahead |

**渲染流程**：`form_schema.annotation_fields` →（按 `group` 分 `核心`/`补充` 两卡）→ 逐字段查表渲染 → 用 zod 按 `required/min/max/max_length` 动态建 schema → react-hook-form 接管 → `onBlur` 校验 → 改值即 debounce 写 localStorage 草稿。进入自动聚焦第一个 `required` 字段。

---

## 6. TypeScript 数据契约

> 对齐 PRD §8 DDL / §9 API / §10.2 form_schema。`form_schema` 在 PRD 基础上**扩展** 3 个可选字段：`SourceField.primary`（阅读区聚光）、`AnnotationField.group`（核心/补充分组）、`AnnotationField.hotkeys`（Q/W/E 映射）。

```ts
// ---- 枚举 ----
export type Role = 'annotator' | 'reviewer' | 'admin';
export type DatasetStatus = 'IMPORTING'|'READY'|'PAUSED'|'DONE'|'FAILED';
export type TaskStatus = 'PENDING'|'CLAIMED'|'COMPLETED'|'NEEDS_REDO';
export type ReviewStatus = 'approved'|'needs_redo';
export type AnnotationWidget =
  | 'Select'|'MultiSelect'|'Rating'|'Confidence'|'TextArea'
  | 'Input'|'InputNumber'|'Switch'|'DatePicker'|'RelationLink';

// ---- 用户 ----
export interface User { id:number; username:string; role:Role; created_at:string; }

// ---- form_schema ----
export interface FieldOption { value:string; label:string; }
export interface SourceField {
  code:string; type:string; widget:string; label:string;
  primary?:boolean;                 // 扩展：是否中栏阅读聚光字段
}
export interface AnnotationField {
  code:string; label:string; widget:AnnotationWidget;
  required?:boolean; options?:FieldOption[];
  min?:number; max?:number; step?:number; max_length?:number;
  default?:unknown;
  group?:'core'|'extra';            // 扩展：卡片分组
  hotkeys?:Record<string,string>;   // 扩展：{ "Q":"政治", "W":"历史" }
}
export interface FormSchema { version:number; source_fields:SourceField[]; annotation_fields:AnnotationField[]; }

// ---- 数据集 ----
export interface DatasetListItem {
  id:number; name:string; status:DatasetStatus;
  total_rows:number; completed:number; pending:number; claimed:number;
  active_annotators:number; form_schema_version:number; updated_at:string;
}
export interface Dataset extends DatasetListItem {
  source_schema:string; source_table:string; source_pk_column:string;
  hash_columns:string[]; form_schema:FormSchema; created_by:number; created_at:string;
}
export interface ImportBatch {
  id:number; dataset_id:number; file_name:string|null; file_size_bytes:number|null;
  new_task_count:number; updated_task_count:number; imported_by:number|null;
  error:string|null; created_at:string;
}

// ---- 任务（标注台核心）----
export interface Task {
  id:number; dataset_id:number; source_row_pk:string; status:TaskStatus;
  round:number; assigned_to:number|null; lease_expires_at:string|null;
}
/** GET /api/tasks/:id 与 claim 返回的渲染包 */
export interface TaskBundle {
  task:Task;
  source_row:Record<string, unknown>;   // 源表该行所有列
  form_schema:FormSchema;
  draft?:AnnotationData|null;            // 服务端无；前端从 localStorage 注入
  ai_suggestion?:AnnotationData|null;    // 预留：AI 预标
}

// ---- 标注数据（含 AI 预留元字段）----
export interface AnnotationData {
  [fieldCode:string]: unknown;
  _source?:'human'|'ai'|'ai-edited';     // 预留
  _ai_confidence?:number;                // 预留
  _ai_reasoning?:string;                 // 预留
}
export interface SubmitPayload { data:AnnotationData; form_schema_version:number; }
export interface Annotation {
  id:number; task_id:number; dataset_id:number; user_id:number;
  data:AnnotationData; form_schema_version:number; round:number;
  superseded_at:string|null; reviewed_at:string|null; reviewed_by:number|null;
  review_status:ReviewStatus|null; review_note:string|null; created_at:string;
}

// ---- 审核 ----
export interface ReviewQueueItem {
  annotation_id:number; task_id:number; dataset_id:number;
  annotator:string; submitted_at:string; round:number;
}

// ---- 看板 ----
export interface DashboardStats {
  today_throughput:number; active_annotators:number; avg_seconds:number; pending_reviews:number;
  datasets:Array<Pick<DatasetListItem,'id'|'name'|'completed'|'total_rows'>>;
  annotator_productivity:Array<{ user_id:number; username:string; today_count:number }>;
  throughput_7d:Array<{ date:string; count:number }>;
}

// ---- 通用 ----
export interface Paginated<T>{ items:T[]; total:number; }
export interface ApiError{ code:string; message:string; }
```

---

## 7. Mock 数据

`web/src/mocks/` 放以下样例，供原型在无后端时渲染（后续接真 API 直接替换 query fn）。

```ts
// taskBundle.mock.ts
export const mockTaskBundle: TaskBundle = {
  task: { id:88123, dataset_id:1, source_row_pk:'4501', status:'CLAIMED',
          round:2, assigned_to:7, lease_expires_at:'2026-05-24T12:28:42Z' },
  source_row: {
    id:4501, title:'计算机科学与技术专业介绍',
    body:'本专业培养具备扎实的数学与计算机理论基础……（约 600 字正文，用于中栏沉浸阅读）',
    category:'工学', updated_at:'2026-05-20T08:00:00Z',
  },
  form_schema: {
    version:3,
    source_fields:[
      { code:'id', type:'int', widget:'Input', label:'ID' },
      { code:'title', type:'text', widget:'Input', label:'标题', primary:true },
      { code:'body', type:'text', widget:'TextArea', label:'正文', primary:true },
      { code:'category', type:'text', widget:'Input', label:'原类别' },
    ],
    annotation_fields:[
      { code:'category', label:'类别', widget:'Select', required:true, group:'core',
        options:[{value:'A',label:'理论'},{value:'B',label:'工程'},{value:'C',label:'交叉'},{value:'D',label:'其他'}],
        hotkeys:{Q:'A',W:'B',E:'C',R:'D'} },
      { code:'confidence', label:'置信度', widget:'Confidence', min:0, max:1, step:0.1, group:'core', default:0.7 },
      { code:'note', label:'备注', widget:'TextArea', max_length:500, group:'extra' },
    ],
  },
  ai_suggestion: { category:'B', confidence:0.82, _source:'ai', _ai_confidence:0.82,
                   _ai_reasoning:'正文强调“工程实践”“系统开发”，倾向工程类。' },
};

// datasets.mock.ts —— 列表 3 项（READY/IMPORTING/READY）
// dashboard.mock.ts —— DashboardStats 一份
// reviewQueue.mock.ts —— ReviewQueueItem ×5
```

---

## 8. 微文案与图标映射

### 8.1 图标（lucide-react）

| 位置 | 图标 |
|---|---|
| Rail: Datasets / Annotate / Review / Dashboard / Admin | `Database` / `PenLine` / `CheckCheck` / `LayoutDashboard` / `Users` |
| 状态 READY/IMPORTING/PAUSED/FAILED/DONE | `Circle`(实) / `Loader`(转) / `PauseCircle` / `AlertCircle` / `CheckCircle2` |
| 动作 提交/跳过/释放/同步/导出/上传 | `CornerDownLeft` / `SkipForward` / `LogOut` / `RefreshCw` / `Download` / `Upload` |
| 审核 通过/打回/编辑 | `Check` / `RotateCcw` / `Pencil` |
| AI | `Sparkles` |
| 计时/保存/主题 | `Timer` / `Cloud`(saved)·`CloudOff` / `Moon`·`Sun` |

### 8.2 关键文案（中文）

| 场景 | 文案 |
|---|---|
| 提交主按钮 | `提交并下一条` |
| 提交成功 toast | `已提交 · 第 {n} 条` |
| 提交失败（超时） | `该任务已超时或被回收，正在为你领取下一条` |
| 跳过/释放 toast | `已放回任务池` + `[撤销]` |
| 草稿恢复 | `已恢复未提交的草稿` |
| 队列空态 | `🎉 本数据集已全部标完 · 按 ⌘K 切换数据集` |
| 审核 approve | `已通过 · 下一条` |
| 审核 redo | `已打回重标` |
| lease 过期 | `租约已过期，请重新领取任务` |
| 离线条 | `网络已断开，提交将在恢复后自动同步` |
| 删除确认 | `删除后不可恢复，确定？` |
| 移动端拦截 | `标注工作台需在桌面端使用（建议宽度 ≥ 1280px）` |

---

## 9. 目录结构（前端，修订版 —— 取代 PRD §6.2 web/）

```
web/src/
├── api/                 axios 实例 + 各资源 query/mutation（TanStack Query）
├── components/ui/       shadcn 生成的基础组件
├── components/          业务通用：LeaseTimer / AutosaveIndicator / ProgressRing /
│                         CommandPalette / ActivityRail / Kbd / DiffView / EmptyState
├── features/
│   ├── auth/            登录
│   ├── dataset/         列表 / 上传向导 / 详情 / schema 编辑器 / 导出抽屉
│   ├── annotation/      标注工作台（三栏）+ SchemaForm + FieldWidgets/
│   ├── review/          审核工作台（队列栏 + 对比 + 决策）
│   └── dashboard/       看板（Recharts + SSE）
├── hooks/               useHeartbeat / useDraft / useClaim / useHotkeys(焦点态) / useLeaseTimer
├── stores/              zustand：authStore / workbenchStore
├── schema/              form_schema → widget 渲染 + zod 构建
├── lib/                 utils / shortcuts 注册表 / focus-context
├── mocks/               §7 样例数据
├── router/              react-router + 角色 Guard
├── types.ts             §6 全部契约
└── main.tsx
```

---

## 10. 交付清单（Definition of Done）

原型/前端验收按此勾：

- [ ] Tailwind config + globals.css 落地 §2/§3 全部 token；深色为默认
- [ ] shadcn 基础组件就位；§4 业务组件全状态可见（含 hover/focus/disabled/loading/error）
- [ ] 标注工作台：三栏 + 顶栏(lease/autosave) + 底栏(随焦点态变化的快捷键提示)
- [ ] 焦点态模型(READING/WIDGET/FIELD)生效，单键不与文本输入冲突
- [ ] SchemaForm 由 mock `form_schema` 驱动渲染（Segmented/Confidence/TextArea 至少三类）
- [ ] 提交 → 乐观清空 → 自动下一条（用 mock 模拟 claim）
- [ ] 草稿 localStorage 持久化 + 恢复
- [ ] 审核工作台：左源右标对比 + A/R 单键裁决 + J/K 切换
- [ ] ⌘K 命令面板可搜索/导航
- [ ] 进度看板：覆盖率 bar + 产能 + 7 日折线（Recharts，mock）
- [ ] 全程键盘可完成「领→标→交→下一条」，不碰鼠标
- [ ] `prefers-reduced-motion` 生效；正文/次要文字对比达标；焦点环可见
- [ ] <1024px 显示桌面端拦截/只读提示

---

**结束**。三份文档分工：`PRD.md`=业务与后端契约 · `UI_UX_DESIGN.md`=设计意图与交互 · 本文件=工程构建基准。可据此直接进入原型实现（建议 `annotation` 工作台优先）。
