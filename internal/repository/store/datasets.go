package store

import (
	"context"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

// GetDataset 取数据集，并按 org 隔离——这是数据集相关操作的统一越权护栏：
// orgID 非 nil 时，跨组织的数据集返回 ErrNotFound（不暴露存在）；orgID 为 nil（超管）旁路过滤。
func (s *Store) GetDataset(ctx context.Context, id int64, orgID *int64) (*domain.Dataset, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, source_schema, source_table, source_pk_column, hash_columns,
		       form_schema, form_schema_version, status, total_rows, created_by, org_id, created_at
		FROM datasets WHERE id = $1 AND ($2::bigint IS NULL OR org_id = $2)`, id, orgID)

	var d domain.Dataset
	err := row.Scan(&d.ID, &d.Name, &d.SourceSchema, &d.SourceTable, &d.SourcePKColumn, &d.HashColumns,
		&d.FormSchema, &d.FormSchemaVersion, &d.Status, &d.TotalRows, &d.CreatedBy, &d.OrgID, &d.CreatedAt)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &d, nil
}

// ListDatasets 列出组织内数据集（含进度计数）。orgID 为 nil（超管）时跨组织返回全部。
func (s *Store) ListDatasets(ctx context.Context, orgID *int64) ([]domain.DatasetListItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id, d.name, d.status, d.total_rows, d.form_schema_version,
		       COUNT(t.id) FILTER (WHERE t.status = 'COMPLETED') AS completed,
		       COUNT(t.id) FILTER (WHERE t.status = 'PENDING')   AS pending,
		       COUNT(t.id) FILTER (WHERE t.status = 'CLAIMED')   AS claimed
		FROM datasets d
		LEFT JOIN tasks t ON t.dataset_id = d.id
		WHERE ($1::bigint IS NULL OR d.org_id = $1)
		GROUP BY d.id
		ORDER BY d.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.DatasetListItem
	for rows.Next() {
		var it domain.DatasetListItem
		if err := rows.Scan(&it.ID, &it.Name, &it.Status, &it.TotalRows, &it.FormSchemaVersion,
			&it.Completed, &it.Pending, &it.Claimed); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// PauseDataset READY → PAUSED（C5.5）。非 READY 返回 ErrConflict。暂停后无法领取新任务。
func (s *Store) PauseDataset(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE datasets SET status = 'PAUSED', updated_at = now() WHERE id = $1 AND status = 'READY'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict // 不存在或非 READY
	}
	return nil
}

// ResumeDataset PAUSED → READY（C5.5）。非 PAUSED 返回 ErrConflict。
func (s *Store) ResumeDataset(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE datasets SET status = 'READY', updated_at = now() WHERE id = $1 AND status = 'PAUSED'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict // 不存在或非 PAUSED
	}
	return nil
}

// CreateDataset 供 seed / 后续导入使用。
func (s *Store) CreateDataset(ctx context.Context, d *domain.Dataset) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO datasets (name, source_schema, source_table, source_pk_column,
		                      form_schema, form_schema_version, status, total_rows, created_by, org_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		d.Name, d.SourceSchema, d.SourceTable, d.SourcePKColumn,
		d.FormSchema, d.FormSchemaVersion, d.Status, d.TotalRows, d.CreatedBy, d.OrgID).Scan(&id)
	return id, err
}
