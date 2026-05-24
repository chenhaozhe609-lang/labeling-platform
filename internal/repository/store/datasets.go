package store

import (
	"context"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

func (s *Store) GetDataset(ctx context.Context, id int64) (*domain.Dataset, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, source_schema, source_table, source_pk_column, hash_columns,
		       form_schema, form_schema_version, status, total_rows, created_at
		FROM datasets WHERE id = $1`, id)

	var d domain.Dataset
	err := row.Scan(&d.ID, &d.Name, &d.SourceSchema, &d.SourceTable, &d.SourcePKColumn, &d.HashColumns,
		&d.FormSchema, &d.FormSchemaVersion, &d.Status, &d.TotalRows, &d.CreatedAt)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &d, nil
}

func (s *Store) ListDatasets(ctx context.Context) ([]domain.DatasetListItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id, d.name, d.status, d.total_rows, d.form_schema_version,
		       COUNT(t.id) FILTER (WHERE t.status = 'COMPLETED') AS completed,
		       COUNT(t.id) FILTER (WHERE t.status = 'PENDING')   AS pending,
		       COUNT(t.id) FILTER (WHERE t.status = 'CLAIMED')   AS claimed
		FROM datasets d
		LEFT JOIN tasks t ON t.dataset_id = d.id
		GROUP BY d.id
		ORDER BY d.created_at DESC`)
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

// CreateDataset 供 seed / 后续导入使用。
func (s *Store) CreateDataset(ctx context.Context, d *domain.Dataset) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO datasets (name, source_schema, source_table, source_pk_column,
		                      form_schema, form_schema_version, status, total_rows)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		d.Name, d.SourceSchema, d.SourceTable, d.SourcePKColumn,
		d.FormSchema, d.FormSchemaVersion, d.Status, d.TotalRows).Scan(&id)
	return id, err
}
