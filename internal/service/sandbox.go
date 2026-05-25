package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateSchema 在 source-db 建隔离 schema（用 admin 连接）。
func CreateSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, ident(schema)))
	return err
}

// GrantReader 把 schema 的只读权限授予 backend 的 reader 角色（恢复完成后调用，PRD §13.3）。
func GrantReader(ctx context.Context, pool *pgxpool.Pool, schema, reader string) error {
	stmts := []string{
		fmt.Sprintf(`GRANT USAGE ON SCHEMA %s TO %s`, ident(schema), ident(reader)),
		fmt.Sprintf(`GRANT SELECT ON ALL TABLES IN SCHEMA %s TO %s`, ident(schema), ident(reader)),
	}
	for _, q := range stmts {
		if _, err := pool.Exec(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

// DropSchema 删除 schema（恢复失败回滚 / 清理旧版本）。
func DropSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, ident(schema)))
	return err
}

// ListSchemas 列出当前用户 schema（排除 pg_*/information_schema）。用于恢复前后快照对比，
// 发现 dump 自建的 schema（C6.1）。
func ListSchemas(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx,
		`SELECT nspname FROM pg_namespace WHERE nspname NOT LIKE 'pg\_%' AND nspname <> 'information_schema'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	set := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		set[n] = true
	}
	return set, rows.Err()
}

// NewSchemas 返回 after 中相对 before 新增的 schema 名。
func NewSchemas(before, after map[string]bool) []string {
	var out []string
	for s := range after {
		if !before[s] {
			out = append(out, s)
		}
	}
	return out
}

// RenameSchema 把 dump 自建的 schema 改名为隔离命名（ds_<id>_v<batch>），保证隔离且避免重名碰撞（C6.1）。
func RenameSchema(ctx context.Context, pool *pgxpool.Pool, from, to string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`ALTER SCHEMA %s RENAME TO %s`, ident(from), ident(to)))
	return err
}
