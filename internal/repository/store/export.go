package store

import (
	"context"
	"encoding/json"
)

// ExportRow 导出时的一条「已完成标注」元信息（源行另从 source-db 取，由调用方叠加）。
type ExportRow struct {
	PK           string
	Round        int
	Version      int     // 该标注采用的 form_schema 版本
	ReviewStatus *string // approved | needs_redo | nil（未审）
	Annotator    string
	Source       *string         // _source: human | ai | ai-edited
	Fills        json.RawMessage // 标注填入值 {fill列code: value}
}

// StreamExport 流式遍历某数据集「COMPLETED 任务 + 有效(未废弃)标注」的行，
// 按 (form_schema_version, pk) 排序逐行回调——版本聚在一起便于按版本分桶（C5.3）。
// onlyApproved=true 时只导审核通过的。回调返回错误即中止遍历。
func (s *Store) StreamExport(ctx context.Context, datasetID int64, onlyApproved bool, fn func(*ExportRow) error) error {
	rows, err := s.pool.Query(ctx, `
		SELECT t.source_row_pk, t.round, a.form_schema_version, a.review_status,
		       u.username, a.data->>'_source', a.data->'fills'
		FROM annotations a
		JOIN tasks t ON t.id = a.task_id
		JOIN users u ON u.id = a.user_id
		WHERE a.dataset_id = $1 AND a.superseded_at IS NULL AND t.status = 'COMPLETED'
		  AND ($2 = false OR a.review_status = 'approved')
		ORDER BY a.form_schema_version, t.source_row_pk`, datasetID, onlyApproved)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		r := &ExportRow{}
		var fills []byte
		if err := rows.Scan(&r.PK, &r.Round, &r.Version, &r.ReviewStatus,
			&r.Annotator, &r.Source, &fills); err != nil {
			return err
		}
		r.Fills = append(json.RawMessage(nil), fills...) // 复制，避免跨 Next() 复用底层缓冲
		if err := fn(r); err != nil {
			return err
		}
	}
	return rows.Err()
}
