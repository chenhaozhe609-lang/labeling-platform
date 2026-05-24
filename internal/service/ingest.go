package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HashedRow 是源表某行的主键 + 稳定内容哈希。
type HashedRow struct {
	PK   string
	Hash string
}

// FetchHashedRows 在 source-db 计算每行的稳定 content_hash（PRD §12.2：
// 显式列 concat_ws，列顺序固定，缺失以空串占位——避免源 schema 加列时全表误判）。
func FetchHashedRows(
	ctx context.Context,
	pool *pgxpool.Pool,
	schema, table, pkCol string,
	hashCols []string,
) ([]HashedRow, error) {
	hashExpr := "md5('')"
	if len(hashCols) > 0 {
		parts := make([]string, len(hashCols))
		for i, c := range hashCols {
			parts[i] = fmt.Sprintf("COALESCE(%s::text, '')", ident(c))
		}
		hashExpr = fmt.Sprintf("md5(concat_ws('|', %s))", strings.Join(parts, ", "))
	}

	q := fmt.Sprintf(`SELECT %s::text AS pk, %s AS h FROM %s.%s`,
		ident(pkCol), hashExpr, ident(schema), ident(table))

	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HashedRow
	for rows.Next() {
		var r HashedRow
		if err := rows.Scan(&r.PK, &r.Hash); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
