// Package source 是 source-db（沙箱）只读访问层：按主键取源行用于标注展示。
package source

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
// 供审核台/导出一次性批量拉取源行，避免逐条往返 source-db。无匹配的 pk 不出现在结果中。
//
// 整数主键走快路径（`pkCol = ANY($1::bigint[])`）以命中主键索引——大表(百万行)批量取行的关键；
// 否则回退到文本比较（`CAST(pkCol AS text) = ANY(...)`，非整数主键）。两种路径返回的键一致。
func (r *Reader) GetRows(ctx context.Context, schema, table, pkCol string, pks []string) (map[string]map[string]any, error) {
	out := make(map[string]map[string]any, len(pks))
	if len(pks) == 0 {
		return out, nil
	}
	if ints, ok := allInts(pks); ok {
		q := fmt.Sprintf(`SELECT *, CAST(%s AS text) AS __pk FROM %s.%s WHERE %s = ANY($1::bigint[])`,
			ident(pkCol), ident(schema), ident(table), ident(pkCol))
		return r.collectByPK(ctx, q, ints, out)
	}
	q := fmt.Sprintf(`SELECT *, CAST(%s AS text) AS __pk FROM %s.%s WHERE CAST(%s AS text) = ANY($1::text[])`,
		ident(pkCol), ident(schema), ident(table), ident(pkCol))
	return r.collectByPK(ctx, q, pks, out)
}

// collectByPK 执行批量取行查询，按内部 __pk 文本键填充 out。
func (r *Reader) collectByPK(ctx context.Context, q string, arg any, out map[string]map[string]any) (map[string]map[string]any, error) {
	rows, err := r.pool.Query(ctx, q, arg)
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

// allInts 当所有 pk 都是合法 int64 时返回其切片，供整数主键快路径用。
func allInts(pks []string) ([]int64, bool) {
	out := make([]int64, len(pks))
	for i, s := range pks {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, false
		}
		out[i] = n
	}
	return out, true
}

// ident 转义并双引号包裹标识符。
func ident(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
