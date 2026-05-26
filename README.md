# 数据标注平台 / Labeling Platform

通用数据标注平台：上传任意 PostgreSQL dump → 反射生成动态表单 → AI 预填 + 多人并发标注 → 抽检审核 → 导出训练数据集。
定位为「知识生产工作台」——沉浸式、键盘优先；着陆页浅色，工作台跟随系统浅/深双模、可手动切换。

## 技术栈

- **后端**：Go + Gin + pgx + golang-migrate
- **前端**：Vite + React + TS + Tailwind v4 + shadcn/ui（编辑感极简 · 暖中性）
- **数据**：PostgreSQL 17 ×2（meta-db 平台库 / source-db 沙箱）+ Redis 7
- **可选**：OpenAI 兼容 LLM（默认 DeepSeek）做预填；未配 `LLM_API_KEY` 则用占位 stub

## 核心能力

- **列角色补全**：每列指定 `context`（喂模型 / 只读）· `fill`（待补全）· `id` · `hidden`；任务 = 含空 fill 列的行。
- **LLM 预填**：context 列喂模型预填 fill 列，标注员可采纳 / 改 / 清；每个值留来源（AI / 修订 / 人工）。
- **任务池**：claim（`FOR UPDATE SKIP LOCKED`）+ 30min 租约 + 心跳 + 超时回收；提交幂等。
- **增量同步**：再传新 dump，按 `content_hash` 仅新增 / 重标变更行（round+1，旧标注 superseded 留史，未变行不动）。
- **抽检审核**：reviewer 随机抽检，可改写并通过；**沙箱只读，绝不写回原始库**。
- **导出**：补全后的完整表流式导出 jsonl / csv。
- **多租户**：按组织隔离 + 开放注册 / 邀请 + 平台超管；JWT + 登录限流 + 会话吊销。

## 端口约定（与 admission 项目并存，刻意避开 5432/6379/8080）

| 服务 | 宿主端口 | 说明 |
|---|---|---|
| meta-db | 5442 | 平台自身数据库 |
| source-db | 5433 | 沙箱（恢复用户 dump） |
| redis | 6380 | 限流 / 缓存 |
| 后端 | 8090 | HTTP API |
| 前端 dev | 5173（被占顺延 5174…） | Vite |

## 本地启动

前置：Docker Desktop、Go 1.26+、Node 20+。

```bash
# 1) 起数据库栈（meta-db / source-db / redis）
docker compose -f deployments/docker-compose.yml up -d

# 2) 迁移建表
go run ./cmd/server migrate up

# 3) 建管理员（createuser <用户名> <密码≥8> <角色: admin|reviewer|annotator>）
go run ./cmd/server createuser admin admin123 admin
#    平台超管（用邮箱登录、跨组织）：go run ./cmd/server createsuperadmin admin@example.com password123

# 4) 起后端（:8090）
go run ./cmd/server            # 或 make run

# 5) 起前端（另开终端）
cd web && npm install && npm run dev
```

Makefile 快捷方式：`make migrate-up` / `make run` / `make createuser ARGS="admin admin123 admin"` / `make fe-dev`。
配置见 `.env.example`：`DATABASE_URL` / `HTTP_ADDR` / `JWT_SECRET` / `CORS_ORIGIN` / `LLM_*` 等。

> **登录用邮箱**：`createuser` 会生成占位邮箱 `<用户名>@local`（如 `admin@local`）；前端登录页填邮箱 + 密码。

## 验证后端

```bash
curl http://localhost:8090/healthz
curl -X POST http://localhost:8090/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@local","password":"admin123"}'
```

## 跑通一条标注

1. 以 admin 登录 →「新建数据集」上传一个 `.sql` / `.backup` / `.dump`（dump 须创建**独立 schema**，平台拒绝纯 public）。没有现成数据可 `go run ./cmd/server seed` 造 demo。
2. 进数据集 →「列与字段」把要标注的列设为 `fill`（配控件 / 选项）→ 回详情页「生成任务」。
3. 进标注台 `/workspace` 领取任务：左=元信息，中=补全表单（含 AI 预填），右=源内容阅读。
4. （可选）reviewer 在 `/review` 抽检 → 数据集详情页导出 jsonl / csv。

## 标注台快捷键

| 键 | 作用 |
|---|---|
| `Tab` / `⇧Tab` | 切换补全字段 |
| `1`–`9` | 当前字段选第 N 项（单选选中 · 多选切换 · 布尔 1是 2否） |
| 字母键 | 按选项自定义快捷键选中（需在 schema 配置） |
| `⌫` | 清空当前字段 |
| `⌘A` | 采纳 AI 预填 |
| `↵` / `⌘↵` | 提交并下一条 / 文本框内提交 |
| `S` · `Esc` | 跳过 · 退出输入 / 释放任务 |
| `J` / `K` · `Space` | 滚动正文 · 展开源字段详情 |
| `?` | 全部快捷键浮层 |

## 目录

```
cmd/server/    入口（server / migrate / createuser / createsuperadmin / seed）
internal/      config · domain · platform(jwt,db) · repository/store · middleware · handler · service(LLM/沙箱) · job(reaper)
migrations/    meta-db DDL（golang-migrate）
deployments/   docker-compose + source-db 初始化（含 sandbox 角色）
web/           前端（features: auth · dataset · annotation · review · dashboard · admin · marketing 着陆页）
tests/         集成测试（testcontainers）+ 导出压测
planning/      设计 / 任务文档（本地保留，gitignore，不入库）
```

## 文档

设计与任务文档在 `planning/`（本地，不入库）：`PRD.md`、`UI_UX_DESIGN.md`、`UI_BUILD_SPEC.md`（§2 暖中性 token）、`TASKS.md`（进度 + PRD 复核补差待办）。
