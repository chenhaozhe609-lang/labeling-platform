// Package service 含导入/反射等业务编排。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

// ReflectResult 是对 source-db 某表反射出的导入元信息（form_schema v2）。
type ReflectResult struct {
	Table       string
	PKColumn    string
	HashColumns []string
	TotalRows   int
	FormSchema  json.RawMessage // v2：columns + role（默认 context，pk→id）+ field
}

// DetectTable 选 schema 中行数最多的基础表（v1 假设一个数据集对应一张源表）。
func DetectTable(ctx context.Context, pool *pgxpool.Pool, schema string) (string, error) {
	var table string
	err := pool.QueryRow(ctx, `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind = 'r'
		ORDER BY c.reltuples DESC, c.relname
		LIMIT 1`, schema).Scan(&table)
	if err != nil {
		return "", fmt.Errorf("schema %q 中未找到表: %w", schema, err)
	}
	return table, nil
}

// Reflect 扫 information_schema 生成 form_schema v2 雏形：
// 每列默认 role=context（主键→id），并按数据类型预置 field 控件配置，
// admin 后续在「列与字段」里把需要标注的列改为 fill / 隐藏的改为 hidden。
func Reflect(ctx context.Context, pool *pgxpool.Pool, schema, table string) (*ReflectResult, error) {
	pk, _ := detectPK(ctx, pool, schema, table) // 无主键时 pk="" ，下面用 __row 兜底由调用方处理

	rows, err := pool.Query(ctx, `
		SELECT column_name, data_type, character_maximum_length
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []domain.ColumnSpec
	var hashCols []string
	var primaryCols []string
	for rows.Next() {
		var name, dataType string
		var maxLen *int
		if err := rows.Scan(&name, &dataType, &maxLen); err != nil {
			return nil, err
		}
		spec := domain.ColumnSpec{Code: name, Type: dataType, Label: name, Field: defaultField(dataType, maxLen)}
		if name == pk {
			spec.Role = domain.ColID
			spec.PK = true
		} else {
			spec.Role = domain.ColContext // 默认全 context，admin 再提升为 fill
		}
		cols = append(cols, spec)

		if !isTimestamp(dataType) && name != "created_at" && name != "updated_at" {
			hashCols = append(hashCols, name)
		}
		if name != pk && isTextual(dataType) && len(primaryCols) < 2 {
			primaryCols = append(primaryCols, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("表 %q 无列", table)
	}
	if pk == "" {
		return nil, fmt.Errorf("源表 %q 缺主键（v1 要求有主键）", table)
	}

	var total int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.%s`, ident(schema), ident(table))).Scan(&total); err != nil {
		return nil, err
	}

	fs, err := json.Marshal(domain.FormSchema{Version: 1, PrimaryCols: primaryCols, Columns: cols})
	if err != nil {
		return nil, err
	}
	return &ReflectResult{Table: table, PKColumn: pk, HashColumns: hashCols, TotalRows: total, FormSchema: fs}, nil
}

func detectPK(ctx context.Context, pool *pgxpool.Pool, schema, table string) (string, error) {
	var pk string
	err := pool.QueryRow(ctx, `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		WHERE tc.table_schema = $1 AND tc.table_name = $2 AND tc.constraint_type = 'PRIMARY KEY'
		ORDER BY kcu.ordinal_position
		LIMIT 1`, schema, table).Scan(&pk)
	if err != nil {
		return "", err
	}
	return pk, nil
}

// defaultField 按 PG 类型给出默认控件配置（admin 改 role=fill 后即可用）。
func defaultField(dataType string, maxLen *int) *domain.FieldConfig {
	dt := strings.ToLower(dataType)
	switch {
	case strings.HasSuffix(dt, "[]") || dt == "array":
		return &domain.FieldConfig{Kind: "multi"}
	case dt == "boolean":
		return &domain.FieldConfig{Kind: "bool"}
	case dt == "date" || strings.HasPrefix(dt, "timestamp"):
		return &domain.FieldConfig{Kind: "date"}
	case dt == "integer" || dt == "bigint" || dt == "smallint" || dt == "numeric" ||
		dt == "real" || dt == "double precision":
		return &domain.FieldConfig{Kind: "number"}
	default: // text / character varying / char / json...
		f := &domain.FieldConfig{Kind: "text"}
		if maxLen != nil {
			f.Hint = fmt.Sprintf("最长 %d 字符", *maxLen)
		}
		return f
	}
}

func isTextual(dataType string) bool {
	dt := strings.ToLower(dataType)
	return dt == "text" || strings.HasPrefix(dt, "character")
}

func isTimestamp(dataType string) bool {
	dt := strings.ToLower(dataType)
	return strings.HasPrefix(dt, "timestamp") || dt == "date"
}

func ident(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
