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
	q := fmt.Sprintf(`SELECT %s::text AS pk, %s AS h FROM %s.%s`,
		ident(pkCol), buildHashExpr(hashCols), ident(schema), ident(table))
	return scanHashedRows(ctx, pool, q)
}

// FetchTaskRows 仅返回「行内任一 fill 列为空」的行（PRD §24 任务规则）。
// fillCols 为空时返回 nil（没有待补全列 → 无任务）。
func FetchTaskRows(
	ctx context.Context,
	pool *pgxpool.Pool,
	schema, table, pkCol string,
	hashCols, fillCols []string,
) ([]HashedRow, error) {
	if len(fillCols) == 0 {
		return nil, nil
	}
	hashExpr := buildHashExpr(hashCols)
	conds := make([]string, len(fillCols))
	for i, c := range fillCols {
		conds[i] = fmt.Sprintf("(CAST(%s AS text) IS NULL OR CAST(%s AS text) = '')", ident(c), ident(c))
	}
	q := fmt.Sprintf(`SELECT %s::text AS pk, %s AS h FROM %s.%s WHERE %s`,
		ident(pkCol), hashExpr, ident(schema), ident(table), strings.Join(conds, " OR "))
	return scanHashedRows(ctx, pool, q)
}

func buildHashExpr(hashCols []string) string {
	if len(hashCols) == 0 {
		return "md5('')"
	}
	parts := make([]string, len(hashCols))
	for i, c := range hashCols {
		parts[i] = fmt.Sprintf("COALESCE(%s::text, '')", ident(c))
	}
	return fmt.Sprintf("md5(concat_ws('|', %s))", strings.Join(parts, ", "))
}

func scanHashedRows(ctx context.Context, pool *pgxpool.Pool, q string) ([]HashedRow, error) {
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
