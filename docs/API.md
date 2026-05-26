# API 契约（A5）

后端 REST 契约冻结，供前后端对齐。所有业务接口前缀 `/api`，鉴权用 `Authorization: Bearer <access_token>`。
错误统一返回 `{"error": "<文案>"}` + 对应 HTTP 状态码。JSON 字段类型以 [`web/src/types.ts`](../web/src/types.ts) 为权威定义。

约定：
- **角色**：`annotator | reviewer | admin`（JWT 内固定）。下表「角色」列为额外 RBAC 限制；空 = 任意已登录。
- 时间为 RFC3339 字符串；金额/计数为整数。
- 分页/流式见各接口说明。

## 鉴权

| 方法 | 路径 | 角色 | 请求 | 响应 |
|---|---|---|---|---|
| POST | `/auth/login` | — | `{username, password}` | `{access_token, refresh_token, user}` |
| POST | `/auth/refresh` | — | `{refresh_token}` | `{access_token}` |
| GET | `/me` | 登录 | — | `User` |
| GET | `/me/tasks` | 登录 | — | `{in_progress: MyTaskInProgress[], completed: MyTaskDone[]}`（B3.8） |

错误：登录失败 `401 {"error":"用户名或密码错误"}`；缺/坏 token `401`。

## 数据集

| 方法 | 路径 | 角色 | 请求 | 响应 |
|---|---|---|---|---|
| GET | `/datasets` | 登录 | — | `{items: DatasetListItem[]}` |
| GET | `/datasets/:id` | 登录 | — | `{dataset: DatasetFull, progress:{pending,claimed,completed}, batches: ImportBatch[]}` |
| POST | `/datasets` | admin | multipart `name` + `file`(.sql/.backup/.dump) | DatasetDetail（同上） |
| POST | `/datasets/:id/sync` | admin | multipart `file` | DatasetDetail |
| POST | `/datasets/:id/generate-tasks` | admin | — | DatasetDetail |
| POST | `/datasets/:id/pause` | admin | — | DatasetDetail（`status=PAUSED`） |
| POST | `/datasets/:id/resume` | admin | — | DatasetDetail（`status=READY`） |
| PUT | `/datasets/:id/form-schema` | admin | `FormSchema`（JSON body） | `{form_schema_version}` |
| GET | `/datasets/:id/export` | admin/reviewer | query `format=jsonl\|csv`、`only_approved=true?` | **流式**：jsonl(application/x-ndjson) / csv(text/csv, UTF-8 BOM)，附 `Content-Disposition` |

导出语义（C5.3）：仅 COMPLETED + 有效标注的行；源行 + fills 叠加成「补全后的表」；隐藏列剔除；按 `form_schema_version` 排序分桶；每行附 `_pk/_round/_form_schema_version/_review_status/_annotator/_source`。
错误：非 READY 暂停/恢复 `409`；上传非法格式 `400`；导入失败 `500` 且数据集置 `FAILED`、隔离 schema 已清理。

## 任务（标注台）

| 方法 | 路径 | 角色 | 请求 | 响应 |
|---|---|---|---|---|
| POST | `/tasks/claim` | 登录 | `{dataset_id}` | `TaskBundle`；池空 `{task:null}`；暂停 `{task:null, paused:true}` |
| GET | `/tasks/:id` | 登录 | — | `TaskBundle` |
| POST | `/tasks/:id/heartbeat` | 持有者 | — | `{lease_expires_at}` |
| POST | `/tasks/:id/submit` | 持有者 | `{data: AnnotationData, form_schema_version}` | `{ok:true}` |
| POST | `/tasks/:id/release` | 持有者 | — | `{ok:true}`（幂等） |

`TaskBundle = {task, source_row, form_schema, ai_suggestion?}`。
`AnnotationData = {fills:{code→值}, _source?, _ai?}`（提交时 `_ai` 持久化 AI 预填值，供审核 AI↔人 对比）。
并发/租约语义（PRD §11）：claim 用 `FOR UPDATE SKIP LOCKED` + 数据集须 `READY`；超时/被回收/越权 submit → `409`。

## 审核（reviewer 抽检台）

| 方法 | 路径 | 角色 | 请求 | 响应 |
|---|---|---|---|---|
| GET | `/reviews/queue` | reviewer/admin | query `dataset_id`、`limit?`(≤50) | `{dataset_name, form_schema, pending_total, items: ReviewItem[]}` |
| POST | `/reviews/:id/decision` | reviewer/admin | `{status:"approved"\|"needs_redo", note?}` | `{ok:true}` |
| POST | `/reviews/:id/edit` | reviewer/admin | `{data: AnnotationData, note?}` | `{ok:true}` |

抽检（C5.1）：`ORDER BY random()` 取样、仅 COMPLETED + 有效未审 + 非本人；`ReviewItem` 含 `data`、`source_row`、`previous?`（上一版 superseded 标注，供旧↔新）。
裁决（C5.2）：`approved` 标记通过；`needs_redo` → 标注 superseded + 任务回 PENDING + round+1。
改写并通过（B4.4）：原标注 superseded+approved、新插 reviewer 署名有效 approved 标注，task 仍 COMPLETED。
错误：审/改本人标注 `403`；已审/已失效 `409`。

## 管理（admin）

| 方法 | 路径 | 角色 | 请求 | 响应 |
|---|---|---|---|---|
| GET | `/admin/dashboard` | admin | — | `Dashboard`（分段进度 + 今日吞吐 + 审核通过/打回 + 排行 + 活动流） |
| GET | `/admin/users` | admin | — | `{items: User[]}` |
| POST | `/admin/users` | admin | `{username, password(≥6), role}` | `User`；用户名重复 `409` |
| PATCH | `/admin/users/:id` | admin | `{role?, password?}` | `{ok:true}`；降末位 admin `409` |
| DELETE | `/admin/users/:id` | admin | — | `{ok:true}`；删自己 `400`；末位 admin/有关联数据 `409` |

## 健康检查

| 方法 | 路径 | 响应 |
|---|---|---|
| GET | `/healthz` | `{"status":"ok"}`（无需鉴权） |
