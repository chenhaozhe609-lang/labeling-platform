//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// E10：沙箱隔离——受限角色（NOSUPERUSER，对应 sandbox_role）下，恶意 dump 常用的提权操作必须被拒。
// 这里用 `SET LOCAL ROLE` 在事务内降权到一个同属性角色逐条验证（每条独立事务，失败即回滚）。
// 注：容器级资源上限（mem 2g/cpu 1）按 PRD §19 走 docker stats 手测，不在自动化范围。

func TestSandbox_PrivilegeBarrier(t *testing.T) {
	ctx := context.Background()
	// 建一个与 sandbox_role 同属性的受限角色（NOLOGIN 即可，靠 SET ROLE 降权）。
	if _, err := testPool.Exec(ctx, `DO $$ BEGIN
		CREATE ROLE sbx_test NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
	EXCEPTION WHEN duplicate_object THEN NULL; END $$;`); err != nil {
		t.Fatalf("create restricted role: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(context.Background(), `DROP ROLE IF EXISTS sbx_test`) })

	// 这些都是不可信 dump 可能塞进来的提权语句，受限角色下应 42501 insufficient_privilege。
	cases := []struct{ name, sql string }{
		{"COPY TO PROGRAM（命令执行）", `COPY (SELECT 1) TO PROGRAM 'echo pwned'`},
		{"CREATE EXTENSION（需超级用户）", `CREATE EXTENSION IF NOT EXISTS file_fdw`},
		{"pg_read_file（读服务器文件）", `SELECT pg_read_file('/etc/hostname')`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := execAsRole(ctx, "sbx_test", c.sql)
			if err == nil {
				t.Errorf("受限角色执行 %q 竟成功，期望被拒", c.sql)
				return
			}
			var pg *pgconn.PgError
			if errors.As(err, &pg) {
				t.Logf("被拒 code=%s msg=%s", pg.Code, pg.Message)
				if pg.Code != "42501" {
					t.Logf("（非 42501 insufficient_privilege，但已被拒，仍满足隔离）")
				}
			}
		})
	}
}

// execAsRole 在独立事务内 SET LOCAL ROLE 降权后执行 sql，返回其错误（事务总是回滚）。
func execAsRole(ctx context.Context, role, sql string) error {
	tx, err := testPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+role); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, sql)
	return err
}

// 运行超时：statement_timeout 取消跑飞的语句（恶意/超大 dump 的兜底，PRD §13.2）。
func TestSandbox_StatementTimeoutCancels(t *testing.T) {
	ctx := context.Background()
	conn, err := testPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SET statement_timeout = '300ms'`); err != nil {
		t.Fatalf("set timeout: %v", err)
	}
	defer conn.Exec(ctx, `RESET statement_timeout`) // 还回池前复位，避免污染其他用例

	_, err = conn.Exec(ctx, `SELECT pg_sleep(3)`)
	if err == nil {
		t.Fatal("pg_sleep(3) 未被 statement_timeout 取消")
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code != "57014" { // query_canceled
		t.Errorf("取消错误码=%s，期望 57014 query_canceled", pg.Code)
	}
}
