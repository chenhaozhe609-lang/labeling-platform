package store

import (
	"context"
	"encoding/json"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

func (s *Store) CreateImportBatch(ctx context.Context, datasetID int64, fileName string, fileSize int64) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO import_batches (dataset_id, file_name, file_size_bytes) VALUES ($1,$2,$3) RETURNING id`,
		datasetID, fileName, fileSize).Scan(&id)
	return id, err
}

func (s *Store) FinishImportBatch(ctx context.Context, batchID int64, newCount, updatedCount int, errMsg string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE import_batches SET new_task_count=$2, updated_task_count=$3, error=NULLIF($4,'') WHERE id=$1`,
		batchID, newCount, updatedCount, errMsg)
	return err
}

func (s *Store) SetDatasetStatus(ctx context.Context, id int64, status domain.DatasetStatus) error {
	_, err := s.pool.Exec(ctx, `UPDATE datasets SET status=$2, updated_at=now() WHERE id=$1`, id, status)
	return err
}

// UpdateDatasetReflected 反射完成后写回源表坐标 + form_schema + 总行数，并置 READY。
func (s *Store) UpdateDatasetReflected(ctx context.Context, id int64, schema, table, pk string,
	hashCols []string, formSchema json.RawMessage, totalRows int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE datasets
		SET source_schema=$2, source_table=$3, source_pk_column=$4, hash_columns=$5,
		    form_schema=$6, total_rows=$7, status='READY', updated_at=now()
		WHERE id=$1`, id, schema, table, pk, hashCols, formSchema, totalRows)
	return err
}

// UpdateFormSchema 更新 form_schema，版本号 +1，返回新版本。
func (s *Store) UpdateFormSchema(ctx context.Context, id int64, formSchema json.RawMessage) (int, error) {
	var v int
	err := s.pool.QueryRow(ctx, `
		UPDATE datasets SET form_schema=$2, form_schema_version=form_schema_version+1, updated_at=now()
		WHERE id=$1 RETURNING form_schema_version`, id, formSchema).Scan(&v)
	if err != nil {
		return 0, mapNoRows(err)
	}
	return v, nil
}

// InsertTasksWithHash 批量插入带 content_hash 的 PENDING 任务（分块 + ON CONFLICT DO NOTHING）。
func (s *Store) InsertTasksWithHash(ctx context.Context, datasetID, batchID int64, pks, hashes []string) (int64, error) {
	const chunk = 5000
	var total int64
	for i := 0; i < len(pks); i += chunk {
		end := min(i+chunk, len(pks))
		tag, err := s.pool.Exec(ctx, `
			INSERT INTO tasks (dataset_id, source_row_pk, content_hash, status, import_batch_id)
			SELECT $1, p.pk, p.h, 'PENDING', $2
			FROM unnest($3::text[], $4::text[]) AS p(pk, h)
			ON CONFLICT (dataset_id, source_row_pk) DO NOTHING`,
			datasetID, batchID, pks[i:end], hashes[i:end])
		if err != nil {
			return total, err
		}
		total += tag.RowsAffected()
	}
	return total, nil
}

func (s *Store) GetDatasetProgress(ctx context.Context, id int64) (pending, claimed, completed int, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE status='PENDING'),
		  COUNT(*) FILTER (WHERE status='CLAIMED'),
		  COUNT(*) FILTER (WHERE status='COMPLETED')
		FROM tasks WHERE dataset_id=$1`, id).Scan(&pending, &claimed, &completed)
	return
}

func (s *Store) ListBatches(ctx context.Context, datasetID int64) ([]domain.ImportBatch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, dataset_id, file_name, file_size_bytes, new_task_count, updated_task_count, error, created_at
		FROM import_batches WHERE dataset_id=$1 ORDER BY created_at DESC`, datasetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ImportBatch
	for rows.Next() {
		var b domain.ImportBatch
		if err := rows.Scan(&b.ID, &b.DatasetID, &b.FileName, &b.FileSizeBytes,
			&b.NewTaskCount, &b.UpdatedTaskCount, &b.Error, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
