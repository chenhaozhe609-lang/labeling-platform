.PHONY: build run migrate-up migrate-down createuser tidy lint test fe-dev fe-build

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
	go test -tags=integration -race -timeout 300s ./tests/integration/...

# ---- 前端 ----
fe-dev:
	cd web && npm run dev

fe-build:
	cd web && npm run build
