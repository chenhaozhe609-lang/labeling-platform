// Package db 提供 meta-db 连接池与迁移执行。
package db

import (
	"context"
	"errors"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // 注册 pgx5 driver
	_ "github.com/golang-migrate/migrate/v4/source/file"     // 注册 file source
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool 建立 pgx 连接池并 Ping 校验。
func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Migrate 用 golang-migrate 执行迁移。sourceURL 形如 "file://migrations"。
func Migrate(databaseURL, sourceURL string, down bool) error {
	m, err := migrate.New(sourceURL, toPgxURL(databaseURL))
	if err != nil {
		return err
	}
	defer m.Close()

	if down {
		err = m.Down()
	} else {
		err = m.Up()
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// golang-migrate 的 pgx/v5 驱动使用 pgx5:// scheme。
func toPgxURL(databaseURL string) string {
	for _, p := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(databaseURL, p) {
			return "pgx5://" + strings.TrimPrefix(databaseURL, p)
		}
	}
	return databaseURL
}
