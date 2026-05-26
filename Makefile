.PHONY: build run migrate-up migrate-down createuser tidy lint test test-integration test-load fe-dev fe-build fe-test backup restore

# ---- 后端 ----
build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

migrate-up:
	go run ./cmd/server migrate up

migrate-down:
	go run ./cmd/server migrate down

# 用法: make createuser ARGS="admin admin123 admin"
createuser:
	go run ./cmd/server createuser $(ARGS)

tidy:
	go mod tidy

lint:
	go vet ./...

test:
	go test ./...

# 集成测试（需 Docker；testcontainers 起临时 postgres）
test-integration:
	go test -tags=integration -race -timeout 360s ./tests/integration/...

# 导出压测（手动；LOAD_ROWS 可调，默认 10 万）。百万行：make test-load LOAD_ROWS=1000000
test-load:
	go test -tags=load -timeout 600s -v ./tests/load/...

# ---- 前端 ----
fe-dev:
	cd web && npm run dev

fe-build:
	cd web && npm run build

fe-test:
	cd web && npm test

# ---- 运维（D11）----
# meta-db 备份/还原（经 docker compose 对 meta-db 跑 pg_dump/pg_restore）
backup:
	bash scripts/backup-meta-db.sh

# 用法: make restore DUMP=backups/labeling_meta_YYYYmmdd_HHMMSS.dump
restore:
	bash scripts/restore-meta-db.sh $(DUMP)
