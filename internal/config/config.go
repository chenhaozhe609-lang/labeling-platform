// Package config 从环境变量加载运行配置（带开发期默认值）。
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env               string // dev | prod
	HTTPAddr          string // 监听地址，如 :8080
	DatabaseURL       string // meta-db 连接串 postgres://...
	SourceDatabaseURL string // source-db 只读连接串（沙箱查表）
	JWTSecret         string
	AccessTTL         time.Duration
	RefreshTTL        time.Duration
	CORSOrigin        string // 开发期允许的前端源
	LeaseMinutes      int    // 任务租约时长（分钟）
}

// Load 读取环境变量，缺失则用默认值。生产环境（ENV=prod）下 JWT_SECRET 必填。
func Load() (Config, error) {
	cfg := Config{
		Env:         env("ENV", "dev"),
		HTTPAddr:    env("HTTP_ADDR", ":8090"),
		DatabaseURL:       env("DATABASE_URL", "postgres://labeling:labeling@localhost:5442/labeling_meta?sslmode=disable"),
		SourceDatabaseURL: env("SOURCE_DATABASE_URL", "postgres://labeling_reader:reader@localhost:5433/sandbox_template?sslmode=disable"),
		JWTSecret:         env("JWT_SECRET", ""),
		AccessTTL:         envDuration("JWT_ACCESS_TTL", 15*time.Minute),
		RefreshTTL:        envDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		CORSOrigin:        env("CORS_ORIGIN", "http://localhost:5173"),
		LeaseMinutes:      envInt("LEASE_MINUTES", 30),
	}

	if cfg.JWTSecret == "" {
		if cfg.Env == "prod" {
			return cfg, fmt.Errorf("JWT_SECRET 在生产环境必填")
		}
		cfg.JWTSecret = "dev-insecure-secret-change-me" // 仅开发期
	}
	return cfg, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
