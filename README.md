# 数据标注平台 / Labeling Platform

通用数据标注平台：上传任意 PostgreSQL dump → 自动生成动态表单 → 多人并发标注 → 导出训练数据集。
定位为「知识生产工作台」（沉浸式、键盘优先、深色优先），而非传统后台。

- 后端：Go + Gin + pgx + golang-migrate
- 前端：Vite + React + TS + Tailwind v4 + shadcn/ui
- 数据：PostgreSQL 17 ×2（meta-db / source-db）+ Redis 7

## 端口约定（本机与 admission 项目并存，刻意避开 5432/6379/8080）

| 服务 | 宿主端口 | 说明 |
|---|---|---|
| meta-db | 5442 | 平台自身数据库 |
| source-db | 5433 | 沙箱（恢复用户 dump） |
| redis | 6380 | 限流/缓存 |
| 后端 | 8090 | HTTP API |
| 前端 dev | 5173（被占则顺延 5174/5175） | Vite |

## 本地启动

前置：Docker Desktop、Go 1.26+、Node 20+。

```bash
# 1) 起数据库栈
docker compose -f deployments/docker-compose.yml up -d

# 2) 执行迁移（建表）
go run ./cmd/server migrate up

# 3) 建一个管理员（用法：createuser <用户名> <密码> <角色>）
go run ./cmd/server createuser admin admin123 admin

# 4) 起后端（:8090）
go run ./cmd/server          # 或 make run

# 5) 起前端（另开终端）
cd web && npm install && npm run dev
```

也可用 Makefile：`make migrate-up` / `make run` / `make createuser ARGS="admin admin123 admin"` / `make fe-dev`。

配置通过环境变量覆盖（见 `.env.example`）：`DATABASE_URL` / `HTTP_ADDR` / `JWT_SECRET` / `CORS_ORIGIN` 等。

## 验证后端

```bash
curl http://localhost:8090/healthz
curl -X POST http://localhost:8090/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'
```

## 目录

```
cmd/server/          入口（server / migrate / createuser）
internal/            config · domain · platform(jwt,db) · repository/store · middleware · handler
migrations/          meta-db DDL（golang-migrate）
deployments/         docker-compose + source-db 初始化
web/                 前端
planning/            设计文档（本地保留，不入库）
```

## 文档

设计与任务文档在 `planning/`（本地）：`PRD.md`、`UI_UX_DESIGN.md`、`UI_BUILD_SPEC.md`、`TASKS.md`。
