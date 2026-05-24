# 数据标注平台 PRD（工程版）

> 版本：v1（基于老师方案 v1.1 + 8 条工程优化）
> 仓库：`d:/Trae_Projects/labeling-platform/`（待初始化）
> 关系：与 `admission` 仓库**完全独立**；通过用户上传的 PG dump 拿数据，不调 admission 任何接口

---

## 1. 项目背景

业务诉求：给定**任意来源**的 PostgreSQL 数据库快照，让运营 / 标注员对里面的行打结构化标签，产出训练用数据集。

为什么不复用 admission 内嵌方案：

| 维度 | admission 内嵌 | 独立平台 |
|---|---|---|
| 数据源 | 硬绑 `university_major_admissions` | 通用 PG dump 上传 |
| 任务粒度 | 业务三维（uma × dim × round） | 通用一维（源表的每行） |
| 复用性 | 仅服务志愿推荐 | 任何用 PG 存数据的项目 |
| 依赖 | 与 admission DB / auth 强耦合 | 零外部依赖 |

本平台的产出形态：上传 `.sql` / `.backup` → 自动生成动态表单 → 多人并发标注 → 导出 jsonl / CSV → 给训练管线消费。

---

## 2. 业务目标

| KPI | 目标值 | 来源 |
|---|---|---|
| 单批上线时长 | 上传 → 第一个标注员领到任务 < 10 min | 工艺要求 |
| 并发能力 | 50 标注员同时在线，claim 延迟 P95 < 200ms | M3 验收 |
| 增量同步零重复 | 同一 dump 重复上传 / 加了 K 行的新 dump → 任务总数差异 = K | AC-5, AC-6 |
| 草稿不丢 | 浏览器崩溃后回到工作台，已填字段恢复 | 工程优化 #6 |
| 标注吞吐 | 单人单任务平均 < 90 秒（无复杂表单） | 工艺设计 |
| 沙箱安全 | 恶意 dump 不能拖垮平台 / 不能逃逸到 meta-db | 工程优化 #4 |

---

## 3. 核心概念

| 实体 | 一句话 |
|---|---|
| **dataset** | 一次数据导入产生的标注项目；含 source_schema / form_schema / version |
| **import_batch** | 一次 dump 上传记录（首次 + 后续每次增量）；审计单位 |
| **task** | 源表的**一行** = 一个 task；可被 claim / submit |
| **annotation** | 一次标注提交；带 form_schema_version 和 round（重标计数） |
| **source-db** | 隔离的 PG 实例，沙箱恢复用户上传的 dump |
| **meta-db** | 平台自己的 PG，存 users / datasets / tasks / annotations |

---

## 4. 用户角色

| 角色 code | 主要能力 |
|---|---|
| `admin` | 创建/管理 dataset、查看所有进度、导出、用户管理 |
| `reviewer` | 抽检已标注 task（标 approved / needs_redo） |
| `annotator` | 领取任务、提交标注、查看自己产能 |

JWT claims 包含 `user_id` + `role`，role 三选一存在 users 表（不挂多角色，保持简单；reviewer 默认也能 annotate）。

---

## 5. 核心业务流程

```
┌─────────────────────────────────────────────────────────────────────────┐
│  阶段 0：admin 上传                                                       │
│  POST /api/datasets/upload (multipart: .sql / .backup)                  │
│   → sandbox 恢复（source-db 独立 schema）                                │
│   → 扫 information_schema 生成 form_schema 雏形                          │
│   → admin 在 UI 编辑 form_schema（增标注字段）                            │
│   → 状态 IMPORTING → READY                                              │
└─────────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  阶段 1：首次任务生成（一次 INSERT...SELECT...ON CONFLICT DO NOTHING）    │
│   每行 → 1 task；source_row_pk + content_hash 落库                      │
│   import_batch 记录：new_task_count = N, updated_task_count = 0         │
└─────────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  阶段 2：标注（多人并发）                                                 │
│   annotator 进 /workspace                                               │
│   POST /tasks/claim {dataset_id} → SKIP LOCKED 抢一个 + 30 min lease    │
│   GET /tasks/:id → form_schema + source row data                        │
│   填表 → POST /tasks/:id/submit (幂等 + 校验 assigned_to)                │
│   60s 心跳 / 关浏览器 sendBeacon release / 超时 reaper 回收               │
└─────────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  阶段 3：(可选) reviewer 抽检                                             │
│   reviewer 进 /review                                                   │
│   随机/按规则取 COMPLETED 任务 → mark approved / needs_redo              │
│   needs_redo → 任务回 PENDING + annotation 标 superseded                │
└─────────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  阶段 4：增量同步（v1.1 核心）                                            │
│   admin 上传新版本 dump → 进 sandbox 恢复到新 schema                      │
│   INSERT ... ON CONFLICT DO NOTHING → 仅新增 N 行任务                    │
│   (可选) 扫 content_hash 差异 → M 行被改 → 任务回 PENDING + round +1    │
│   import_batch 记 new=N, updated=M                                      │
└─────────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  阶段 5：导出                                                            │
│   GET /datasets/:id/export?format=jsonl                                 │
│   流式 chunked transfer，给训练管线消费                                   │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 6. 功能模块

### 6.1 后端目录

```
labeling-platform/
├── cmd/server/main.go              入口；DI、HTTP server、reaper goroutine
├── internal/
│   ├── config/                     env / yaml 加载
│   ├── handler/
│   │   ├── auth.go                 login / refresh
│   │   ├── dataset.go              upload / sync / list / detail / export
│   │   ├── task.go                 claim / get / heartbeat / submit / release
│   │   ├── annotation.go           review 操作（reviewer）
│   │   └── admin.go                user 管理
│   ├── service/
│   │   ├── parser.go               dump 恢复 + information_schema 反射
│   │   ├── ingest.go               幂等任务生成 + content_hash 差异检测
│   │   ├── taskpool.go             SKIP LOCKED + lease 续约 + 释放
│   │   ├── annotation.go           submit 幂等 + 校验
│   │   ├── review.go               reviewer 抽检逻辑
│   │   └── exporter.go             jsonl / csv 流式
│   ├── repository/
│   │   ├── store/                  pgx 实现的 Store 接口（v1 不引 sqlc）
│   │   └── source/                 source-db 访问层（sandbox 查表）
│   ├── domain/                     领域模型（Dataset / Task / Annotation）
│   ├── middleware/                 JWT / RBAC / Logger / Recovery / RateLimit
│   ├── platform/
│   │   ├── pgrestore/              os/exec 封装 pg_restore + psql（带超时）
│   │   ├── jwt/                    token 签发 + 解析
│   │   └── schemahash/             content_hash 计算（修正版，见 §12）
│   └── job/
│       └── reaper.go               lease 超时回收 + advisory lock
├── migrations/                     meta-db DDL（golang-migrate）
├── tests/
│   ├── integration/                带 source-db / meta-db 容器的端到端
│   └── load/                       50 并发 claim 压测
├── deployments/
│   ├── docker-compose.yml          dev：meta-db + source-db + redis
│   └── Dockerfile
├── Makefile
├── go.mod
└── README.md
```

### 6.2 前端目录

```
labeling-platform/web/
├── src/
│   ├── api/                        axios + 类型
│   ├── components/                 通用组件
│   ├── features/
│   │   ├── auth/                   登录页
│   │   ├── dataset/                列表 / 上传 / 详情 / 批次历史 / form_schema 编辑
│   │   ├── annotation/             工作台（核心）
│   │   ├── review/                 reviewer 抽检页
│   │   └── dashboard/              进度看板（SSE）
│   ├── hooks/
│   │   ├── useHeartbeat.ts         60s 心跳 + sendBeacon release
│   │   ├── useDraft.ts             localStorage 草稿持久化（工程优化 #6）
│   │   └── useClaim.ts             抢任务 + 跳工作台
│   ├── stores/                     zustand
│   ├── schema/                     PG type → Formily schema 转换
│   ├── router/
│   └── main.tsx
├── package.json
└── vite.config.ts
```

---

## 7. 页面设计

> **本节为概要。完整的 UI/UX 与页面设计（信息架构、页面树、三栏标注工作台、reviewer 工作台、快捷键体系、动态表单交互、深色模式 token、移动端策略、设计 rationale）见配套文档 [`UI_UX_DESIGN.md`](./UI_UX_DESIGN.md)，详见 §23。** 平台定位为「知识生产工作台」而非传统后台——沉浸式、键盘优先、阅读优先、连续刷题式工作流、深色优先。

### 7.1 路由表

| 路径 | 组件 | Guard | 说明 |
|---|---|---|---|
| `/login` | LoginPage | none | 用户名密码登录 |
| `/datasets` | DatasetListPage | annotator+ | 我能访问的数据集 |
| `/datasets/upload` | UploadPage | admin | 上传 dump 起新 dataset |
| `/datasets/:id` | DatasetDetailPage | admin | 进度 / batches / form_schema 编辑 |
| `/datasets/:id/schema-editor` | SchemaEditorPage | admin | 编辑 form_schema（标注字段） |
| `/workspace` | AnnotationWorkbench | annotator+ | 标注主页（领任务 → 表单 → 提交） |
| `/review` | ReviewQueuePage | reviewer | 待抽检任务列表 |
| `/review/:taskId` | ReviewDetailPage | reviewer | 看 annotation + approve/redo |
| `/admin/users` | AdminUsersPage | admin | 用户增删改角色 |
| `/admin/datasets/:id/export` | ExportPage | admin | 下载 jsonl/csv |

### 7.2 标注工作台布局

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Header: dataset 名 │ 任务 #88123 │ lease 28:42 │ [跳过] [释放]            │
├──────────────────────────────┬──────────────────────────────────────────┤
│  左：源数据（只读）           │  右：标注表单（Formily 动态渲染）          │
│                              │                                          │
│  pk: 4501                    │  类别 (Select):  [v]                     │
│  text: 待标注的原文...        │  置信度 (Number): [   ]                  │
│  category: 历史                │  备注 (TextArea):                       │
│  ...所有源表列                │  [                          ]            │
│                              │                                          │
│                              │  [保存草稿]  [提交并取下一个]              │
└──────────────────────────────┴──────────────────────────────────────────┘
```

- 左栏：从源表当前行渲染所有列，readonly，按字段类型用合适展示组件
- 右栏：Formily 用 `form_schema` 渲染；用户改字段时**自动写 localStorage 草稿**（key = `draft:${taskId}`）
- 进入页面：优先 localStorage 草稿 → 否则空表单
- 提交成功：清草稿；失败：保留草稿
- 60s 自动心跳；剩余 < 5 min 时弹"延长 lease"
- 关浏览器：`navigator.sendBeacon('/api/tasks/:id/release')` 立即放回池

---

## 8. 数据结构设计（meta-db）

完整 DDL（首个 migration 文件 `001_init.up.sql`）：

```sql
-- ============================================================================
-- 001_init: 用户 / 数据集 / 任务 / 标注 / 导入批次
-- ============================================================================

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'annotator'
                  CHECK (role IN ('annotator', 'reviewer', 'admin')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE datasets (
    id                  BIGSERIAL PRIMARY KEY,
    name                TEXT NOT NULL,
    source_schema       TEXT NOT NULL,    -- source-db 里的 schema 名
    source_table        TEXT NOT NULL,
    source_pk_column    TEXT NOT NULL,    -- 源表主键列名（若无主键，置 '__row_hash'）
    -- hash_columns 是 content_hash 计算依据列；空数组 = 全列（不推荐）
    hash_columns        TEXT[] NOT NULL DEFAULT '{}',
    form_schema         JSONB NOT NULL,
    form_schema_version INT  NOT NULL DEFAULT 1,
    last_imported_pk    TEXT,             -- 增量水位线（大表优化）
    status              TEXT NOT NULL DEFAULT 'IMPORTING'
                        CHECK (status IN ('IMPORTING', 'READY', 'PAUSED', 'DONE', 'FAILED')),
    total_rows          INT  NOT NULL DEFAULT 0,
    created_by          BIGINT REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_datasets_status ON datasets(status, created_at DESC);

CREATE TABLE import_batches (
    id                 BIGSERIAL PRIMARY KEY,
    dataset_id         BIGINT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    file_name          TEXT,
    file_size_bytes    BIGINT,
    new_task_count     INT NOT NULL DEFAULT 0,
    updated_task_count INT NOT NULL DEFAULT 0,
    imported_by        BIGINT REFERENCES users(id),
    error              TEXT,              -- 失败原因（恢复失败 / hash 计算异常）
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_import_batches_dataset ON import_batches(dataset_id, created_at DESC);

CREATE TABLE tasks (
    id               BIGSERIAL PRIMARY KEY,
    dataset_id       BIGINT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    source_row_pk    TEXT   NOT NULL,
    content_hash     TEXT,
    status           TEXT   NOT NULL DEFAULT 'PENDING'
                     CHECK (status IN ('PENDING', 'CLAIMED', 'COMPLETED', 'NEEDS_REDO')),
    assigned_to      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    claimed_at       TIMESTAMPTZ,
    lease_expires_at TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ,
    import_batch_id  BIGINT REFERENCES import_batches(id),
    round            INT    NOT NULL DEFAULT 1,  -- 被重标次数
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (dataset_id, source_row_pk)
);
-- 队列认领的主索引：partial 让 SKIP LOCKED 只扫 PENDING
CREATE INDEX idx_tasks_pending  ON tasks(dataset_id, id) WHERE status = 'PENDING';
-- reaper 扫超时 lease
CREATE INDEX idx_tasks_lease    ON tasks(lease_expires_at) WHERE status = 'CLAIMED';
-- 我的待办 / reviewer 抽检
CREATE INDEX idx_tasks_assigned ON tasks(assigned_to, status);
CREATE INDEX idx_tasks_completed ON tasks(dataset_id, status, completed_at DESC) WHERE status = 'COMPLETED';

CREATE TABLE annotations (
    id                  BIGSERIAL PRIMARY KEY,
    task_id             BIGINT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    dataset_id          BIGINT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    user_id             BIGINT NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    data                JSONB  NOT NULL,
    form_schema_version INT    NOT NULL,
    round               INT    NOT NULL DEFAULT 1,
    -- 当源行被修改重标时，旧 annotation 标 superseded 但保留（审计）
    superseded_at       TIMESTAMPTZ,
    -- reviewer 抽检结果
    reviewed_at         TIMESTAMPTZ,
    reviewed_by         BIGINT REFERENCES users(id) ON DELETE SET NULL,
    review_status       TEXT CHECK (review_status IN ('approved', 'needs_redo')),
    review_note         TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_annotations_task    ON annotations(task_id, round DESC);
CREATE INDEX idx_annotations_review  ON annotations(dataset_id, reviewed_at) WHERE review_status IS NULL;
CREATE INDEX idx_annotations_active  ON annotations(task_id) WHERE superseded_at IS NULL;
```

**关键设计决策（与老师方案的差异）**：
1. `hash_columns TEXT[]`：解决工程优化 #1 —— content_hash 计算时只对显式列 hash，避免列顺序变化炸全表
2. `tasks.status` 增 `NEEDS_REDO`：reviewer 标 needs_redo 时用，便于过滤
3. `annotations.superseded_at`：解决工程优化 #2 —— 行被改重标时旧 annotation 保留但标 superseded
4. `annotations.reviewed_at/by/status/note`：v1 内置 reviewer 抽检（工程优化 #5）
5. `idx_annotations_active`：常用查询"task 当前有效 annotation"走这个 partial index

---

## 9. API 设计

### 9.1 认证 `/api/auth/*`

| Method | Path | 说明 |
|---|---|---|
| POST | `/api/auth/login` | 用户名 + 密码 → JWT |
| POST | `/api/auth/refresh` | refresh token 换 access token |

### 9.2 数据集 `/api/datasets/*`

| Method | Path | 角色 | 说明 |
|---|---|---|---|
| GET | `/api/datasets` | annotator+ | 列表（admin 看全部；annotator 看自己有任务的） |
| POST | `/api/datasets/upload` | admin | multipart 上传 dump（首次） |
| GET | `/api/datasets/:id` | annotator+ | 详情含 form_schema / 进度 / 最新 batch |
| PUT | `/api/datasets/:id/form-schema` | admin | 改 form_schema（带 schema diff 预览） |
| POST | `/api/datasets/:id/sync` | admin | multipart 上传新 dump 触发增量 |
| GET | `/api/datasets/:id/batches` | admin | 导入批次历史 |
| GET | `/api/datasets/:id/progress` | annotator+ | 进度数字（用于 SSE 推送） |
| GET | `/api/datasets/:id/stream` | annotator+ | SSE 推进度 |
| GET | `/api/datasets/:id/export` | admin | 流式 jsonl / csv |
| POST | `/api/datasets/:id/pause` | admin | dataset 状态 → PAUSED（停发任务） |
| POST | `/api/datasets/:id/resume` | admin | → READY |

### 9.3 任务 `/api/tasks/*`

| Method | Path | 角色 | 说明 |
|---|---|---|---|
| POST | `/api/tasks/claim` | annotator+ | body: `{dataset_id}` → SKIP LOCKED 抢一个 |
| GET | `/api/tasks/:id` | annotator+ | bundle: task + source row + form_schema |
| POST | `/api/tasks/:id/heartbeat` | annotator+ | 续 lease |
| POST | `/api/tasks/:id/submit` | annotator+ | 幂等提交标注 |
| POST | `/api/tasks/:id/release` | annotator+ | 主动放回池 |
| GET | `/api/tasks/my` | annotator+ | 我的 CLAIMED/COMPLETED 任务 |

### 9.4 审核 `/api/reviews/*`

| Method | Path | 角色 | 说明 |
|---|---|---|---|
| GET | `/api/reviews/queue` | reviewer | 待抽检任务列表（COMPLETED 且未 reviewed） |
| POST | `/api/reviews/:annotation_id/approve` | reviewer | 通过 + 可选 note |
| POST | `/api/reviews/:annotation_id/needs-redo` | reviewer | 驳回 → task 回 PENDING + 原 annotation 标 superseded |

### 9.5 Admin `/api/admin/*`

| Method | Path | 说明 |
|---|---|---|
| GET / POST | `/api/admin/users` | 列表 / 创建 |
| PUT | `/api/admin/users/:id` | 改 role / 密码 |
| DELETE | `/api/admin/users/:id` | 软删 |
| GET | `/api/admin/dashboard` | 全局看板：每个 dataset 的覆盖率 + 用户产能 |

---

## 10. 动态 Schema 设计

### 10.1 PG type → Formily widget 映射表

| PG 类型 | Formily widget | 说明 |
|---|---|---|
| `text` / `varchar(N)` | `Input` (N≤80) / `Input.TextArea` (N>80) | |
| `int` / `bigint` / `numeric` | `InputNumber` | |
| `bool` | `Switch` | |
| `date` | `DatePicker` | |
| `timestamptz` / `timestamp` | `DatePicker showTime` | |
| `text[]` | `Select mode=tags` | 自由打标 |
| `jsonb` | `Input.TextArea`（admin 自行约束） | 复杂结构由 admin 写 schema 显式定 |
| 枚举（CHECK IN (...)） | `Select` 强枚举 | parser 解析 CHECK 约束 |
| 外键 → 小表 | `Select` 下拉（异步加载） | v1 不做，admin 手工配 |

源字段默认 `x-read-pretty: true`（只读展示）。

### 10.2 form_schema 结构（JSONB，存 datasets.form_schema）

```json
{
  "version": 1,
  "source_fields": [
    { "code": "id", "type": "int", "widget": "Input", "label": "ID" },
    { "code": "text", "type": "text", "widget": "TextArea", "label": "正文" }
  ],
  "annotation_fields": [
    {
      "code": "category",
      "label": "类别",
      "widget": "Select",
      "required": true,
      "options": [
        {"value": "A", "label": "类别 A"},
        {"value": "B", "label": "类别 B"}
      ]
    },
    {
      "code": "confidence",
      "label": "置信度",
      "widget": "InputNumber",
      "min": 0, "max": 1, "step": 0.1
    },
    {
      "code": "note",
      "label": "备注",
      "widget": "TextArea",
      "max_length": 500
    }
  ]
}
```

平台用 `source_fields` 渲染左栏（只读），`annotation_fields` 渲染右栏（可编辑）。

### 10.3 Schema 演进规则

| 操作 | 影响 | 系统行为 |
|---|---|---|
| 加新标注字段 | 兼容（旧 annotation 该字段为 null） | `form_schema_version + 1`，无需重标 |
| 加可选源字段（源 schema 改动） | 兼容 | hash_columns 不动 → content_hash 不变 → 不触发重标 |
| 改字段类型 / 删字段 | 破坏 | admin 必须人工"确认应用"；旧 annotation 标 superseded |
| 改 hash_columns | 中等 | 下次 sync 会触发 K 行的"被修改"判定 → 需 admin 显式 confirm |

`form_schema_version` 永远只增不减；annotations 表记当时版本，导出时按版本分桶。

---

## 11. 任务池设计

### 11.1 状态机

```
PENDING ──claim──> CLAIMED ──submit──> COMPLETED ──approve──> COMPLETED (approved)
   ▲                  │                    │             ↘
   │                  │ release/超时        │ needs_redo  ↘
   ├──────────────────┘                    │              ↘
   │                                       │               annotations[].superseded_at
   ├──────────────────────────────────────┘
   │                  needs_redo
   │
   └── PENDING (重新抢)
```

### 11.2 Claim SQL（核心）

```sql
WITH next AS (
    SELECT id FROM tasks
    WHERE dataset_id = $1 AND status = 'PENDING'
    ORDER BY id
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
UPDATE tasks t
SET status = 'CLAIMED',
    assigned_to = $2,
    claimed_at = now(),
    lease_expires_at = now() + ($3::int * interval '1 minute'),
    updated_at = now()
FROM next n
WHERE t.id = n.id
RETURNING t.*;
```

默认 lease = 30 min。

### 11.3 反双重领（同一用户不能同时持多个任务，可选）

v1 不强制（一个 annotator 可以同时持多个任务，但前端引导一次只标一个）。若后续加：claim 时检查该用户当前 CLAIMED 数量 < N。

### 11.4 Submit SQL（幂等）

```sql
UPDATE tasks
SET status = 'COMPLETED', completed_at = now(), updated_at = now()
WHERE id = $1 AND assigned_to = $2 AND status = 'CLAIMED'
RETURNING id;

-- 同事务插 annotations 行
INSERT INTO annotations (task_id, dataset_id, user_id, data, form_schema_version, round)
VALUES (...);
```

返回 0 行 → 任务已被回收或被他人提交 → 前端提示"已超时，请重新领取"。

### 11.5 Lease Reaper

```go
func (r *Reaper) tick(ctx context.Context) {
    // 跨实例互斥（多实例部署时只一个 reaper 跑）
    var got bool
    r.pool.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, reaperLockKey).Scan(&got)
    if !got { return }
    defer r.pool.Exec(ctx, `SELECT pg_advisory_unlock($1)`, reaperLockKey)

    tag, _ := r.pool.Exec(ctx, `
        UPDATE tasks
        SET status = 'PENDING', assigned_to = NULL,
            claimed_at = NULL, lease_expires_at = NULL,
            updated_at = now()
        WHERE status = 'CLAIMED' AND lease_expires_at < now()
    `)
    if n := tag.RowsAffected(); n > 0 {
        slog.Info("lease reaped", "count", n)
    }
}
```

周期 60s，启动立即跑一次。

---

## 12. 增量同步设计（v1.1 核心 + 工程优化 #1 修复）

### 12.1 流程

```
admin 上传 v2 dump
     │
     ▼
[1] sandbox 恢复到新 schema (source_schema_v2)
     │
     ▼
[2] 计算每行的稳定 content_hash（基于 datasets.hash_columns）
     │
     ▼
[3] INSERT ... ON CONFLICT DO NOTHING → 仅插新行
     │
     ▼
[4] 扫描已有 task 的 content_hash 与新 hash 不一致的 → UPDATE 回 PENDING + round + 1
    旧 annotation → 标 superseded_at = now()
     │
     ▼
[5] 写 import_batch (new=N, updated=M)
     │
     ▼
[6] 切 datasets.source_schema 指向 v2，删除 v1 sandbox schema
```

### 12.2 稳定 content_hash（修复方案）

老师方案的 `md5(s::text)` 在源 schema 加列时全表 hash 失效。修正：

```sql
-- datasets.hash_columns = ARRAY['title', 'body', 'category']
-- 显式列拼接，列顺序固定，缺失列以空串占位
SELECT
  s.id::text AS pk,
  md5(concat_ws('|',
    COALESCE(s.title::text, ''),
    COALESCE(s.body::text,  ''),
    COALESCE(s.category::text, '')
  )) AS h
FROM source_schema_v2.source_table s
```

`hash_columns` 由 admin 在创建 dataset 时显式选定（默认全列，但建议排除 updated_at / created_at 类时间戳）。

### 12.3 增量 INSERT

```sql
INSERT INTO tasks (dataset_id, source_row_pk, content_hash, status, import_batch_id)
SELECT
  $1,                                            -- dataset_id
  s.id::text,                                    -- source_pk_column
  md5(concat_ws('|', COALESCE(s.title::text,''), ...)),
  'PENDING',
  $2                                             -- import_batch_id
FROM source_schema_v2.source_table s
ON CONFLICT (dataset_id, source_row_pk) DO NOTHING;
-- 返回插入行数 = new_task_count
```

### 12.4 已标行被修改的处理

```sql
WITH changed AS (
    UPDATE tasks t
    SET status = 'PENDING', content_hash = src.h,
        assigned_to = NULL, claimed_at = NULL, lease_expires_at = NULL,
        round = t.round + 1,
        updated_at = now()
    FROM (
      SELECT s.id::text AS pk,
             md5(concat_ws('|', COALESCE(s.title::text,''), ...)) AS h
      FROM source_schema_v2.source_table s
    ) src
    WHERE t.dataset_id = $1 AND t.source_row_pk = src.pk
      AND t.content_hash IS DISTINCT FROM src.h
    RETURNING t.id
)
UPDATE annotations
SET superseded_at = now()
WHERE task_id IN (SELECT id FROM changed) AND superseded_at IS NULL;
```

两条 UPDATE 在同一事务里，保证一致。

### 12.5 大表性能优化

`datasets.last_imported_pk` 作水位线：

```sql
WHERE s.id > $last_imported_pk
```

仅在源表主键单调递增（如 BIGSERIAL）时启用；admin 可在 dataset 配置里 toggle。

---

## 13. 沙箱与隔离

### 13.1 物理拓扑

```
docker-compose.yml:
  meta-db:    PostgreSQL 17  容器名 labeling-meta-db   持久 volume meta_pgdata
  source-db:  PostgreSQL 17  容器名 labeling-source-db 持久 volume source_pgdata
              ↑ 资源限制：memory: 2g, cpus: '1.0'
              ↑ 独立 volume，可定期 truncate 旧 schema 释放
  redis:      Redis 7        rate limit / 心跳缓存
  backend:    Go 二进制       唯一同时连两个 PG 的服务
  frontend:   nginx 静态     vite build 产物
```

### 13.2 sandbox 恢复策略

每个 dataset 上传时：

```bash
# v1: .sql
PGOPTIONS='-c statement_timeout=600000' \
  psql -h source-db -U sandbox_role -d sandbox_template \
  -v ON_ERROR_STOP=1 \
  -c "CREATE SCHEMA ds_${dataset_id}_v${batch_id};" \
  -c "SET search_path = ds_${dataset_id}_v${batch_id};" \
  -f /tmp/upload.sql

# v1: .backup (custom format)
pg_restore --no-owner --no-privileges \
  -h source-db -U sandbox_role -d sandbox_template \
  --schema=ds_${dataset_id}_v${batch_id} \
  /tmp/upload.backup
```

要点：
- `sandbox_role` 只在 `sandbox_template` 数据库内有权限，无 superuser、不能 CREATE DATABASE、不能 DROP 其他 schema
- 每次 sandbox 恢复用独立 schema，并发上传不冲突
- 超时通过 `statement_timeout` + 进程级 `context.WithTimeout`
- 失败：log + 删 schema + import_batch 标 error

### 13.3 后端只读访问 sandbox

backend 连 source-db 用第二个独立 connection pool，role 仅 SELECT 权限，不能 INSERT/UPDATE：

```sql
GRANT USAGE ON SCHEMA ds_<n>_v<n> TO labeling_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA ds_<n>_v<n> TO labeling_reader;
```

恢复完后才 GRANT，避免恢复期间 backend 提前查到半截数据。

---

## 14. 质量控制（v1 内置，工程优化 #5）

### 14.1 reviewer 角色权限

- reviewer 可见所有 COMPLETED task（不限自己标的）
- reviewer 不能 review 自己的标注（防止自审）

### 14.2 审核操作

| 操作 | 后端动作 |
|---|---|
| approve | `annotations.reviewed_at/by/status=approved/note` |
| needs_redo | annotation 标 review_status=needs_redo + superseded_at；task 回 PENDING；round 不增（重标不视为新轮次） |

### 14.3 抽检策略（前端 review queue）

v1 仅按"已完成时间倒序"列出未审核的 task；admin 可随时调 `GET /api/reviews/queue?strategy=random` 启用随机抽样（v1.1 加）。

---

## 15. 权限设计

JWT payload：
```json
{ "sub": 1, "role": "admin", "exp": ..., "iat": ... }
```

中间件链：
1. `JWTMiddleware`：解 token，写 `gin.Context`
2. `RequireRole(roles...)`：按 role 拦截

| 端点前缀 | 最低 role |
|---|---|
| `/api/auth/*` | none |
| `/api/datasets/*/upload` `/sync` `/form-schema` `/pause` `/resume` `/export` | admin |
| `/api/datasets/:id` `/progress` `/stream` `/batches` | annotator+ |
| `/api/tasks/*` | annotator+ |
| `/api/reviews/*` | reviewer+ |
| `/api/admin/*` | admin |

无"reviewer 默认能 annotate"硬规则；统一按 role 判定（reviewer 调 `/api/tasks/claim` 也允许）。

---

## 16. 状态机汇总

### 16.1 dataset.status

```
IMPORTING ──parse 完成──> READY ──pause──> PAUSED
                            │                │
                            └──complete──> DONE
                            ↘ admin 主动归档 ↗
                              FAILED（恢复失败时）
```

### 16.2 task.status

```
PENDING ──claim──> CLAIMED ──submit──> COMPLETED ──needs_redo──> PENDING
                       │                                         (回到队列)
                       └─lease 超时/release─> PENDING
```

### 16.3 annotation 的"有效"判定

`WHERE task_id = ? AND superseded_at IS NULL ORDER BY created_at DESC LIMIT 1` → 当前有效 annotation。

导出走这个 partial index，旧的不导。

---

## 17. 非功能需求

| 类别 | 指标 |
|---|---|
| 性能 - claim 接口 | P95 < 200ms（50 并发） |
| 性能 - 增量同步 | 10 万行 dump < 5 min（含 pg_restore） |
| 性能 - 导出 | 100 万行 jsonl < 60s（流式 chunked） |
| 并发 | 50 标注员同时在线，0 重复领取 |
| 数据一致 | submit / superseded 标记 / round +1 同一事务 |
| 沙箱安全 | source-db 容器 memory 2g cpus 1，恶意 dump 隔离 |
| 可观测 | structured slog；指标埋点：tasks_pending_total / claim_p95 / reaper_reclaimed_total |
| 浏览器 | Chrome / Edge / Safari 近 2 个版本 |
| 备份 | meta-db 每日 pg_dump；source-db 可选（数据可重新上传重建） |

---

## 18. 开发里程碑

### M1 · 基础设施（1 周）
- [ ] Go 项目骨架：`cmd/server` + `internal/{config,handler,service,repository,middleware}`
- [ ] Vite + React + TS + Antd + Formily 前端骨架
- [ ] `docker-compose.yml`：meta-db + source-db（含资源限制）+ redis
- [ ] golang-migrate 接入；落 `001_init.up.sql`（§8 全表）
- [ ] JWT 中间件 + RBAC + login 端点
- [ ] CI：Conventional Commits + Go lint + frontend lint
- [ ] README + Makefile

### M2 · 数据导入（1.5 周）
- [ ] **上传接口**：multipart，校验大小/扩展名
- [ ] **sandbox 恢复**：`pgrestore` 包装 `os/exec`，超时 + 错误捕获
- [ ] **information_schema 反射**：扫 columns → form_schema 雏形
- [ ] **form_schema 编辑器**：admin UI（JSON 编辑器 + 字段类型映射预览）
- [ ] **首次任务生成**：INSERT...SELECT...ON CONFLICT，单 SQL 完成
- [ ] **content_hash 稳定方案**：§12.2 修正版
- [ ] AC-1: 上传 → READY → 可见 form_schema

### M3 · 任务池（2 周，核心）
- [ ] **SKIP LOCKED claim**：§11.2 SQL + handler
- [ ] **lease 续约**：heartbeat 端点
- [ ] **主动 release**：端点 + 前端 sendBeacon
- [ ] **submit 幂等**：§11.4 + 同事务插 annotation
- [ ] **lease reaper goroutine**：60s + advisory lock
- [ ] **集成测试**：50 并发 claim 0 重复 / lease 超时 / 越权 submit 403
- [ ] AC-1, AC-2, AC-3, AC-4 验收

### M4 · 标注前端（2 周）
- [ ] Formily 集成 + widget 注册
- [ ] 标注工作台三栏布局
- [ ] **useDraft hook**：localStorage 草稿
- [ ] useHeartbeat 60s + sendBeacon
- [ ] 进度看板 + SSE
- [ ] 我的待办 / 已完成页

### M5 · 增量迭代（1.5 周，核心）
- [ ] `/api/datasets/:id/sync` 端点
- [ ] **content_hash 差异 UPDATE** + annotation superseded
- [ ] import_batches 审计表 + UI
- [ ] last_imported_pk 水位优化（toggle）
- [ ] AC-5, AC-6, AC-7, AC-8 验收

### M6 · 收尾（1 周）
- [ ] **reviewer 角色**：review queue + approve/needs_redo
- [ ] **导出**：jsonl + csv 流式
- [ ] **admin 看板**：每个 dataset 进度 + 用户产能
- [ ] AC-9 压测：100 万行 dump 端到端验证

总计 **9 周**（含 buffer）。

---

## 19. 测试矩阵

| AC | 场景 | 测试类型 | 文件位置 |
|---|---|---|---|
| AC-1 | N 用户并发高频领取 | integration + -race | `tests/integration/concurrent_claim_test.go` |
| AC-2 | 用户领后断网/关页 | integration | `tests/integration/lease_reaper_test.go` |
| AC-3 | 用户领后正常标注 | integration | `tests/integration/heartbeat_test.go` |
| AC-4 | 租约过期后客户端提交 | integration | `tests/integration/submit_idempotency_test.go` |
| AC-5 | 重复上传同备份 | integration | `tests/integration/sync_idempotent_test.go` |
| AC-6 | 上传含 K 条新行的备份 | integration | `tests/integration/sync_incremental_test.go` |
| AC-7 | 源中已标注行被修改 | integration | `tests/integration/sync_content_change_test.go` |
| AC-8 | 新备份增加字段 | integration + 手测 | `tests/integration/schema_evolution_test.go` |
| AC-9 | 百万行数据集 | 压测脚本 + 手测 | `tests/load/big_dataset_load.go` |
| - | sandbox 越权 | integration | `tests/integration/sandbox_isolation_test.go` |
| - | sandbox 资源耗尽 | 手测 | docker stats 验证容器 cap |
| - | content_hash 列顺序变化 | unit | `internal/platform/schemahash/hash_test.go` |

CI 必跑：unit + lint。Integration 在 PR 上跑（用 service container）；load test 手动触发。

---

## 20. 风险与守则

1. **content_hash 计算必须显式列**：不能用 `s::text` 全行。schema 加列时 hash 必须不变（否则全表重标）。
2. **sandbox role 权限收紧**：只能在 sandbox_template DB 操作，不能跨库，不能 CREATE DATABASE。任何放宽必须代码 review。
3. **reaper 多实例**：必须用 `pg_try_advisory_lock`，否则多实例同时 UPDATE 会有冲突（无逻辑错但浪费 CPU）。
4. **submit 幂等的 RETURNING**：必须检查影响行数 = 1，否则前端要明确知道"提交未生效"。
5. **annotations 不要硬删**：审计需求；用 `superseded_at` 软标记。
6. **form_schema 改动须 admin 显式确认**：删字段会让旧 annotation 的部分数据失效，必须有 confirm 弹窗。
7. **dump 文件不要 commit**：`.gitignore` 加 `*.sql.backup` `*.dump`。
8. **上传大小硬上限**：默认 500MB；超大需另接 S3 + presigned URL（v2）。
9. **前端 lint 在本地必跑**：CI 严格模式，PR 前 `npm run lint` 不通过别提（admission 项目就因 React 19 lint 翻车）。

---

## 21. 附录 A：与老师方案 v1.1 的差异清单

| 差异点 | 老师方案 | 本 PRD | 原因 |
|---|---|---|---|
| content_hash | `md5(s::text)` | 显式列 `md5(concat_ws(...))` | 工程优化 #1：避免列顺序触发误判 |
| 已标行重标后历史 annotation | 未定义 | `superseded_at` 软标 + UI 默认隐藏 | 工程优化 #2：审计 + 避免信息泄漏 |
| sqlc | 引入 | v1 不引入，纯 pgx + Store 接口 | 工程优化 #3：减学习曲线 |
| sandbox 资源限制 | 提到未细化 | docker-compose 显式 memory 2g cpus 1 | 工程优化 #4：防恶意 dump |
| 质量控制 | 仅 round 字段预留 | v1 内置 reviewer 角色 + needs_redo 工作流 | 工程优化 #5：数据可用性 |
| 草稿 | 未提 | localStorage `useDraft` hook | 工程优化 #6：用户体验 |
| 测试矩阵 | 9 个 AC 未分层 | 每个 AC 明确 unit/integration/load | 工程优化 #7 |
| dev compose 范围 | source-db M2 才起 | M1 就起（含空 source-db 容器） | 工程优化 #8：开发期可调试 |
| reviewer 自审 | 未提 | 显式禁止（防自审） | 数据质量 |
| 角色模型 | annotator/reviewer/admin | 同（但 reviewer 默认能 annotate） | 简化 |

## 22. 附录 B：MVP 完成时新增/修改文件清单（预估）

```
labeling-platform/
├── cmd/server/main.go                                    ~150 行
├── internal/
│   ├── config/config.go                                  ~80
│   ├── handler/{auth,dataset,task,annotation,admin}.go   ~900 总
│   ├── service/{parser,ingest,taskpool,annotation,review,exporter}.go  ~1500 总
│   ├── repository/store/{users,datasets,tasks,annotations,batches}.go  ~800 总
│   ├── repository/source/source.go                       ~200
│   ├── domain/{dataset,task,annotation,user}.go          ~300
│   ├── middleware/{jwt,rbac,logger,recover,ratelimit}.go ~400
│   ├── platform/pgrestore/restore.go                     ~150
│   ├── platform/jwt/jwt.go                               ~100
│   ├── platform/schemahash/hash.go                       ~80
│   └── job/reaper.go                                     ~80
├── migrations/001_init.up.sql + .down.sql                ~250 + 30
├── tests/integration/                                    ~1000
├── tests/load/                                           ~300
├── deployments/docker-compose.yml + Dockerfile           ~150
├── web/src/
│   ├── api/                                              ~400
│   ├── components/                                       ~600
│   ├── features/auth/                                    ~150
│   ├── features/dataset/                                 ~900
│   ├── features/annotation/                              ~700
│   ├── features/review/                                  ~400
│   ├── features/dashboard/                               ~300
│   ├── hooks/                                            ~250
│   ├── stores/                                           ~150
│   ├── schema/                                           ~200
│   └── router/ + main.tsx                                ~150
├── web/package.json                                      —
├── Makefile                                              ~80
└── README.md                                             ~150

预估总量：后端 ~5000 行（含 ~1300 测试），前端 ~4400 行
```

---

---

## 23. UI/UX 与页面设计（配套文档）

完整设计规格见 [`UI_UX_DESIGN.md`](./UI_UX_DESIGN.md)。核心要点：

- **定位**：知识生产工作台（Knowledge Production Workspace），心智模型 = 代码编辑器 + 阅读器，非企业后台。
- **五原则**：沉浸式工作流 / 键盘优先 / 阅读优先 / 连续刷题式工作流 / 长时深色友好。
- **信息架构**：沉浸区（标注台、审核台，外壳自动隐藏）与管理区（数据集、看板、用户，带极窄 Activity Rail）物理分离；命令面板 `⌘K` 取代多级菜单。
- **标注工作台（核心）**：三栏（左=上下文/历史，中=阅读 surface，右=schema 驱动表单）+ 顶栏（lease 计时/自动保存）+ 底栏（随焦点态变化的快捷键提示）；submit → 自动加载下一条，永不回列表。
- **快捷键**：三焦点态模型（READING/WIDGET/FIELD）杜绝单键与文本输入冲突；`1-9` 选项、`Tab` 走字段、`⏎` 提交、`A/R` 审核裁决。
- **动态表单**：schema 驱动；「能用 Segmented 就不用下拉」；卡片分组 + 渐进展示；引擎建议 react-hook-form+zod 或 Formily 注册自绘控件，**放弃 Antd 默认企业外观**。
- **视觉**：深色优先；Inter（UI/正文）+ JetBrains Mono（数据/计时器）；表面分层提亮（非重阴影）；语义 token；indigo 焦点 / green 提交 / amber 警示 / red 打回 / violet AI。
- **移动端**：工作台仅桌面（≥1024px）；手机/平板可只读看进度，不可标注。
- **AI 预标注**：UI 已预留（幽灵填充 + 接受/改/拒 + reasoning），`annotations.data` JSONB 加 `_source` 元字段即可零重构接入。

> 设计实现影响 M1 前端骨架与 M4 标注前端：组件库从 Antd 改为 headless 基元（Radix）+ Tailwind 自绘主题 + `cmdk` 命令面板；动态表单渲染层需自绘以支持键盘优先交互。

---

**结束**。下一步：基于本 PRD 出**任务拆分文档**（类比之前 admission 那份 `LABELING_SYSTEM_TASKS.md`），然后 M1 落地。
