# API 契约（A5）

后端 REST 契约冻结，供前后端对齐。所有业务接口前缀 `/api`，鉴权用 `Authorization: Bearer <access_token>`。
错误统一返回 `{"error": "<文案>"}` + 对应 HTTP 状态码。JSON 字段类型以 [`web/src/types.ts`](../web/src/types.ts) 为权威定义。

约定：
- **角色**：`annotator | reviewer | admin`（组织内角色，JWT 内固定）。下表「角色」列为额外 RBAC 限制；空 = 任意已登录。
- **多租户**：每个用户归属一个组织（`org_id`）；除超管外，所有业务数据按 `org_id` 严格隔离——`datasets/tasks/reviews/export/dashboard/users` 全面 org 化。跨组织访问数据集/任务一律视同不存在（`404`），跨组织标注裁决/改写视同失效（`409`），claim 他组织数据集视同无任务。**超管**（`is_superadmin`，`org_id=null`）跨组织旁路过滤，仅用 `/platform/*`。
- **登录标识**：`email`（全局唯一，大小写不敏感）；`username` 为显示名。
- **会话/吊销**：JWT claims = `{uid, role, org_id, tv, typ}`。`logout-all` / 改密 / 重置密码会 bump `token_version`，使该用户所有旧 token 失效（`refresh` 比对 `tv`；`access` 至多再活一个 TTL）。
- 时间为 RFC3339 字符串；金额/计数为整数。
- 分页/流式见各接口说明。

## 鉴权

| 方法 | 路径 | 角色 | 请求 | 响应 |
|---|---|---|---|---|
| POST | `/auth/signup` | — | `{org_name, email, username, password(≥8)}` | `{access_token, refresh_token, user, org}`：建组织 + 注册人即该组织 admin |
| POST | `/auth/login` | — | `{email, password}` | `{access_token, refresh_token, user, org}` |
| POST | `/auth/refresh` | — | `{refresh_token}` | `{access_token, refresh_token, user, org}`（校验 `tv`，失效→`401`） |
| POST | `/auth/accept-invite` | — | `{token, email, username, password(≥8)}` | `{access_token, refresh_token, user, org}`：凭邀请入既有组织（角色取自邀请） |
| POST | `/auth/logout-all` | 登录 | — | `{ok:true}`：bump `token_version`，吊销本人所有会话 |
| GET | `/me` | 登录 | — | `User`（含 `email/org_id/is_superadmin`） |
| GET | `/me/tasks` | 登录 | — | `{in_progress: MyTaskInProgress[], completed: MyTaskDone[]}`（B3.8） |

错误：登录失败 `401 {"error":"邮箱或密码错误"}`；登录失败过多（按 邮箱+IP 计，5 次锁定起、指数退避）`429`；缺/坏 token `401`；注册/接受邀请邮箱已存在 `409`；邀请无效或过期 `400`；邮箱与邀请限定不符 `400`。

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
| GET | `/admin/dashboard` | admin | — | `Dashboard`（**本组织**：分段进度 + 今日吞吐 + 审核通过/打回 + 排行 + 活动流） |
| GET | `/admin/users` | admin | — | `{items: User[]}`（仅本组织） |
| POST | `/admin/users` | admin | `{username, email, password(≥8), role}` | `User`（建到本组织）；邮箱重复 `409` |
| PATCH | `/admin/users/:id` | admin | `{role?, password?(≥8)}` | `{ok:true}`；改密会吊销该用户旧会话；降本组织末位 admin `409`；跨组织用户 `404` |
| DELETE | `/admin/users/:id` | admin | — | `{ok:true}`；删自己 `400`；本组织末位 admin/有关联数据 `409`；跨组织用户 `404` |

`/admin/users` 仅作用于调用者所在组织（按 `org_id` 隔离）。

## 邀请（admin，按组织限定）

| 方法 | 路径 | 角色 | 请求 | 响应 |
|---|---|---|---|---|
| POST | `/admin/invites` | admin | `{role, email?}` | `{invite: Invite, accept_path}`：生成邀请 token，`email` 可限定受邀邮箱 |
| GET | `/admin/invites` | admin | — | `{items: Invite[]}`（本组织，新到旧） |
| DELETE | `/admin/invites/:id` | admin | — | `{ok:true}`；撤销本组织邀请；不存在 `404` |

`Invite` 含 `token`，受邀人据 `accept_path`（`/accept-invite?token=…`）打开前端页并 `POST /auth/accept-invite`。邀请默认 7 天有效。

## 平台（超管）

| 方法 | 路径 | 角色 | 请求 | 响应 |
|---|---|---|---|---|
| GET | `/platform/orgs` | 超管 | — | `{items: Organization[]}`（跨组织） |

非超管访问 `/platform/*` → `403`。

## 健康检查

| 方法 | 路径 | 响应 |
|---|---|---|
| GET | `/healthz` | `{"status":"ok"}`（无需鉴权） |
