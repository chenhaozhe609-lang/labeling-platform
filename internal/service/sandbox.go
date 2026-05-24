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
