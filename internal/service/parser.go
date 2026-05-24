// Package service 含导入/反射等业务编排。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ReflectResult 是对 source-db 某表反射出的导入元信息。
type ReflectResult struct {
	Table       string
	PKColumn    string
	HashColumns []string
	TotalRows   int
	FormSchema  json.RawMessage // 含 source_fields（只读）；annotation_fields 由 admin 后续编辑
}

type sourceField struct {
	Code    string `json:"code"`
	Type    string `json:"type"`
	Widget  string `json:"widget"`
	Label   string `json:"label"`
	Primary bool   `json:"primary,omitempty"`
}

type formSchema struct {
	Version          int           `json:"version"`
	SourceFields     []sourceField `json:"source_fields"`
	AnnotationFields []any         `json:"annotation_fields"`
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

// Reflect 扫 information_schema 生成 form_schema 雏形 + 主键 + hash 列建议。
func Reflect(ctx context.Context, pool *pgxpool.Pool, schema, table string) (*ReflectResult, error) {
	rows, err := pool.Query(ctx, `
		SELECT column_name, data_type, character_maximum_length
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fields []sourceField
	var hashCols []string
	for rows.Next() {
		var name, dataType string
		var maxLen *int
		if err := rows.Scan(&name, &dataType, &maxLen); err != nil {
			return nil, err
		}
		widget, primary := mapWidget(dataType, maxLen)
		fields = append(fields, sourceField{Code: name, Type: dataType, Widget: widget, Label: name, Primary: primary})
		if !isTimestamp(dataType) && name != "created_at" && name != "updated_at" {
			hashCols = append(hashCols, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("表 %q 无列", table)
	}

	pk, err := detectPK(ctx, pool, schema, table)
	if err != nil {
		return nil, err
	}

	var total int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.%s`, ident(schema), ident(table))).Scan(&total); err != nil {
		return nil, err
	}

	fs, err := json.Marshal(formSchema{Version: 1, SourceFields: fields, AnnotationFields: []any{}})
	if err != nil {
		return nil, err
	}

	return &ReflectResult{
		Table:       table,
		PKColumn:    pk,
		HashColumns: hashCols,
		TotalRows:   total,
		FormSchema:  fs,
	}, nil
}

// detectPK 取单列主键；复合主键取第一列；无主键报错（v1 要求源表有主键）。
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
		return "", fmt.Errorf("源表 %q 缺主键（v1 要求有主键）: %w", table, err)
	}
	return pk, nil
}

func mapWidget(dataType string, maxLen *int) (widget string, primary bool) {
	dt := strings.ToLower(dataType)
	switch {
	case dt == "text":
		return "TextArea", true
	case strings.HasPrefix(dt, "character"): // character varying / character
		if maxLen == nil || *maxLen > 40 {
			return "Input", true
		}
		return "Input", false
	case dt == "boolean":
		return "Switch", false
	case dt == "date" || strings.HasPrefix(dt, "timestamp"):
		return "DatePicker", false
	case dt == "integer" || dt == "bigint" || dt == "smallint" || dt == "numeric" ||
		dt == "real" || dt == "double precision":
		return "InputNumber", false
	case dt == "jsonb" || dt == "json":
		return "TextArea", false
	default:
		return "Input", false
	}
}

func isTimestamp(dataType string) bool {
	return strings.HasPrefix(strings.ToLower(dataType), "timestamp") || strings.ToLower(dataType) == "date"
}

func ident(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
