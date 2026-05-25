# 后端镜像：多阶段构建 → 静态二进制 + psql/pg_restore 客户端（C2 沙箱恢复用，SANDBOX_MODE=local）。
# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO 关闭 → 纯静态（pgx/migrate/bcrypt 均纯 Go）；-trimpath/-s -w 瘦身。
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.21
# postgresql17-client：与 source-db(PG17) 同主版本的 psql/pg_restore，供本机模式恢复 dump。
RUN apk add --no-cache postgresql17-client ca-certificates tzdata \
 && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/server /app/server
COPY migrations /app/migrations
USER app
EXPOSE 8090
# migrations 以 file://migrations 相对 CWD(/app) 解析。
ENTRYPOINT ["/app/server"]
