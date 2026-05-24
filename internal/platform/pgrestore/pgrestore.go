// Package pgrestore 用 os/exec 封装 psql / pg_restore，把用户上传的 dump 恢复进
// source-db 的隔离 schema。支持两种调用方式：
//
//	local  —— 直接调用本机 psql/pg_restore（生产：后端容器内装 postgresql-client）
//	docker —— 经 `docker exec` 调用 source-db 容器内的 psql/pg_restore（开发期，宿主无客户端）
package pgrestore

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type Config struct {
	Mode      string // docker | local
	Container string // docker 模式容器名
	DB        string // 目标库
	User      string // 恢复角色
	Password  string
	Timeout   time.Duration
}

type Restorer struct {
	cfg Config
}

func New(cfg Config) *Restorer {
	return &Restorer{cfg: cfg}
}

// Restore 把 dumpPath 恢复进已存在的 schema。
// custom=true 走 pg_restore（.backup 自定义格式），否则走 psql（.sql 纯文本）。
// 通过 PGOPTIONS 把 search_path 钉到目标 schema，并施加 statement_timeout。
func (r *Restorer) Restore(ctx context.Context, schema, dumpPath string, custom bool) error {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()

	f, err := os.Open(dumpPath)
	if err != nil {
		return fmt.Errorf("打开 dump: %w", err)
	}
	defer f.Close()

	pgOptions := fmt.Sprintf("-c search_path=%s -c statement_timeout=%d", schema, r.cfg.Timeout.Milliseconds())

	bin := "psql"
	binArgs := []string{"-v", "ON_ERROR_STOP=1", "-q", "-U", r.cfg.User, "-d", r.cfg.DB}
	if custom {
		bin = "pg_restore"
		binArgs = []string{"--no-owner", "--no-privileges", "-U", r.cfg.User, "-d", r.cfg.DB}
	}

	var cmd *exec.Cmd
	switch r.cfg.Mode {
	case "docker":
		args := []string{
			"exec", "-i",
			"-e", "PGPASSWORD=" + r.cfg.Password,
			"-e", "PGOPTIONS=" + pgOptions,
			r.cfg.Container, bin,
		}
		args = append(args, binArgs...)
		cmd = exec.CommandContext(ctx, "docker", args...)
	default: // local
		cmd = exec.CommandContext(ctx, bin, binArgs...)
		cmd.Env = append(os.Environ(), "PGPASSWORD="+r.cfg.Password, "PGOPTIONS="+pgOptions)
	}

	cmd.Stdin = f
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("恢复超时（>%s）", r.cfg.Timeout)
		}
		return fmt.Errorf("恢复失败: %w: %s", err, truncate(stderr.String(), 800))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
