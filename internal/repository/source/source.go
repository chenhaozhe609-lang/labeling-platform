// Package source 是 source-db（沙箱）只读访问层：按主键取源行用于标注展示。
package source

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Reader struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Reader {
	return &Reader{pool: pool}
}

// GetRow 取 schema.table 中 pk 对应行（列名作 key）。无行返回 (nil, nil)。
// schema/table/pkCol 来自 dataset 配置（admin 控制），仍做标识符转义防注入。
func (r *Reader) GetRow(ctx context.Context, schema, table, pkCol, pk string) (map[string]any, error) {
	q := fmt.Sprintf(`SELECT * FROM %s.%s WHERE CAST(%s AS text) = $1 LIMIT 1`,
		ident(schema), ident(table), ident(pkCol))

	rows, err := r.pool.Query(ctx, q, pk)
	if err != nil {
		return nil, err
	}
	m, err := pgx.CollectOneRow(rows, pgx.RowToMap)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return m, nil
}

// ident 转义并双引号包裹标识符。
func ident(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
