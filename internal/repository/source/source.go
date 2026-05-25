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

// GetRows 一次取多个 pk 对应行，按 pk 文本（与 source_row_pk 同样的 CAST(... AS text)）键返回。
// 供审核台一次性拉取整页队列的源行，避免逐条往返 source-db。无匹配的 pk 不出现在结果中。
func (r *Reader) GetRows(ctx context.Context, schema, table, pkCol string, pks []string) (map[string]map[string]any, error) {
	out := make(map[string]map[string]any, len(pks))
	if len(pks) == 0 {
		return out, nil
	}
	q := fmt.Sprintf(`SELECT *, CAST(%s AS text) AS __pk FROM %s.%s WHERE CAST(%s AS text) = ANY($1::text[])`,
		ident(pkCol), ident(schema), ident(table), ident(pkCol))

	rows, err := r.pool.Query(ctx, q, pks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ms, err := pgx.CollectRows(rows, pgx.RowToMap)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		key, _ := m["__pk"].(string)
		delete(m, "__pk") // 内部对齐键，不外泄
		out[key] = m
	}
	return out, nil
}

// ident 转义并双引号包裹标识符。
func ident(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
