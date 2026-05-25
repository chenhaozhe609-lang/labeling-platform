// Package config 从环境变量加载运行配置（带开发期默认值）。
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	// 沙箱恢复（C2）
	SourceAdminURL   string        // postgres 超级用户连 source-db（建 schema / 反射 / 授权 / 读取）
	SandboxMode      string        // docker | local：psql/pg_restore 的调用方式
	SandboxContainer string        // docker 模式下的容器名
	SandboxHost      string        // local 模式连接主机（容器化时连 source-db 服务名；空=本机）
	SandboxPort      string        // local 模式端口（空=默认 5432）
	SandboxDB        string        // 目标库（sandbox_template）
	SandboxUser      string        // 恢复用角色（dev: postgres）
	SandboxPassword  string        // 对应密码
	RestoreTimeout   time.Duration // 单次恢复超时
	UploadMaxBytes   int64         // 上传大小硬上限
	UploadDir        string        // 上传临时目录

	// LLM 预填（C5/§24.5）：OpenAI 兼容 Chat Completions（默认 DeepSeek）。
	// LLMAPIKey 为空时后端回退到占位 StubPrefiller。
	LLMBaseURL string        // 形如 https://api.deepseek.com（或 .../v1）
	LLMAPIKey  string        // 兼容端点的 API Key
	LLMModel   string        // 如 deepseek-chat
	LLMTimeout time.Duration // 单次预填超时
}

// Load 读取环境变量，缺失则用默认值。生产环境（ENV=prod）下 JWT_SECRET 必填。
// 启动时若当前目录存在 .env，先加载其中的键（真实环境变量优先，不覆盖）。
func Load() (Config, error) {
	loadDotEnv(".env")

	cfg := Config{
		Env:               env("ENV", "dev"),
		HTTPAddr:          env("HTTP_ADDR", ":8090"),
		DatabaseURL:       env("DATABASE_URL", "postgres://labeling:labeling@localhost:5442/labeling_meta?sslmode=disable"),
		SourceDatabaseURL: env("SOURCE_DATABASE_URL", "postgres://labeling_reader:reader@localhost:5433/sandbox_template?sslmode=disable"),
		JWTSecret:         env("JWT_SECRET", ""),
		AccessTTL:         envDuration("JWT_ACCESS_TTL", 15*time.Minute),
		RefreshTTL:        envDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		CORSOrigin:        env("CORS_ORIGIN", "http://localhost:5173"),
		LeaseMinutes:      envInt("LEASE_MINUTES", 30),

		SourceAdminURL:   env("SOURCE_ADMIN_URL", "postgres://postgres:postgres@localhost:5433/sandbox_template?sslmode=disable"),
		SandboxMode:      env("SANDBOX_MODE", "docker"),
		SandboxContainer: env("SANDBOX_CONTAINER", "labeling-source-db"),
		SandboxHost:      env("SANDBOX_HOST", ""),
		SandboxPort:      env("SANDBOX_PORT", ""),
		SandboxDB:        env("SANDBOX_DB", "sandbox_template"),
		// 恢复以受限角色 sandbox_role 跑（NOSUPERUSER，仅限 sandbox_template）——挡住恶意 dump 提权（C6.1）。
		SandboxUser:     env("SANDBOX_USER", "sandbox_role"),
		SandboxPassword: env("SANDBOX_PASSWORD", "sandbox"),
		RestoreTimeout:  envDuration("RESTORE_TIMEOUT", 10*time.Minute),
		UploadMaxBytes:  int64(envInt("UPLOAD_MAX_MB", 500)) << 20,
		UploadDir:       env("UPLOAD_DIR", filepath.Join(os.TempDir(), "labeling-uploads")),

		LLMBaseURL: env("LLM_BASE_URL", "https://api.deepseek.com"),
		LLMAPIKey:  env("LLM_API_KEY", ""),
		LLMModel:   env("LLM_MODEL", "deepseek-chat"),
		LLMTimeout: envDuration("LLM_TIMEOUT", 15*time.Second),
	}

	if cfg.JWTSecret == "" {
		if cfg.Env == "prod" {
			return cfg, fmt.Errorf("JWT_SECRET 在生产环境必填")
		}
		cfg.JWTSecret = "dev-insecure-secret-change-me" // 仅开发期
	}
	return cfg, nil
}

// loadDotEnv 极简 .env 加载：KEY=VALUE 逐行，跳过空行/`#` 注释，去除可选包裹引号。
// 仅设置当前未在环境中的键——真实环境变量优先。文件不存在时静默跳过。
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		if key != "" {
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, val)
			}
		}
	}
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
