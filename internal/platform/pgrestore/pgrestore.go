// Package pgrestore 用 os/exec 封装 psql / pg_restore，把用户上传的 dump 恢复进
// source-db 的隔离 schema。支持两种调用方式：
//
//	local  —— 直接调用本机 psql/pg_restore（生产：后端容器内装 postgresql-client）
//	docker —— 经 `docker exec` 调用 source-db 容器内的 psql/pg_restore（开发期，宿主无客户端）
package pgrestore

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Config struct {
	Mode      string // docker | local
	Container string // docker 模式容器名
	Host      string // local 模式连接主机（容器化部署连 source-db 服务名；空=本机 socket）
	Port      string // local 模式端口（空=默认 5432）
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

// Restore 以受限角色（cfg.User，默认 sandbox_role）把 dumpPath 恢复进 source-db。
// custom=true 走 pg_restore（.backup 自定义格式，--no-owner --no-privileges），否则走 psql（.sql）。
// .sql 走流式消毒：剥离 OWNER/GRANT/REVOKE/EXTENSION 等非超级用户跑不动、也不该让不可信 dump 跑的语句。
// 不再钉 search_path——让 dump 自建其 schema，由上层快照对比发现后改名为隔离 schema（C6.1）。
func (r *Restorer) Restore(ctx context.Context, dumpPath string, custom bool) error {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()

	f, err := os.Open(dumpPath)
	if err != nil {
		return fmt.Errorf("打开 dump: %w", err)
	}
	defer f.Close()

	pgOptions := fmt.Sprintf("-c statement_timeout=%d", r.cfg.Timeout.Milliseconds())

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
		env := append(os.Environ(), "PGPASSWORD="+r.cfg.Password, "PGOPTIONS="+pgOptions)
		if r.cfg.Host != "" {
			env = append(env, "PGHOST="+r.cfg.Host)
		}
		if r.cfg.Port != "" {
			env = append(env, "PGPORT="+r.cfg.Port)
		}
		cmd.Env = env
	}

	// .sql 纯文本流式消毒；custom 二进制由 pg_restore 的 --no-owner/--no-privileges 兜底。
	if custom {
		cmd.Stdin = f
	} else {
		cmd.Stdin = sanitizingReader(f)
	}
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

// sanitizingReader 返回一个剥离危险/越权语句后的 .sql 流（COPY 数据块原样透传）。
func sanitizingReader(src io.Reader) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		err := sanitizeSQL(pw, src)
		pw.CloseWithError(err) // err 为 nil 时即正常 EOF
	}()
	return pr
}

// sanitizeSQL 逐行过滤：COPY ... FROM stdin 与其后到 `\.` 之间的数据原样保留；
// 其余行中剥离 OWNER TO / GRANT / REVOKE / *EXTENSION 语句（保留 CREATE SCHEMA 供上层发现+改名）。
func sanitizeSQL(dst io.Writer, src io.Reader) error {
	sc := bufio.NewScanner(src)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20) // 容忍长行（宽表/长文本）
	w := bufio.NewWriter(dst)
	inCopy := false
	for sc.Scan() {
		line := sc.Text()
		if inCopy {
			if _, err := w.WriteString(line + "\n"); err != nil {
				return err
			}
			if line == `\.` {
				inCopy = false
			}
			continue
		}
		up := strings.ToUpper(strings.TrimSpace(line))
		if strings.HasPrefix(up, "COPY ") && strings.Contains(up, "FROM STDIN") {
			inCopy = true
		} else if strippableStmt(up) {
			continue // 丢弃
		}
		if _, err := w.WriteString(line + "\n"); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return w.Flush()
}

func strippableStmt(up string) bool {
	switch {
	case strings.HasPrefix(up, "GRANT "), strings.HasPrefix(up, "REVOKE "):
		return true
	case strings.HasPrefix(up, "ALTER ") && strings.Contains(up, " OWNER TO "):
		return true
	case strings.Contains(up, "CREATE EXTENSION"), strings.Contains(up, "DROP EXTENSION"),
		strings.Contains(up, "COMMENT ON EXTENSION"):
		return true
	default:
		return false
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
