package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

// reaper 跨实例互斥用的 advisory lock key。
const reaperLockKey int64 = 911001

const taskCols = `id, dataset_id, source_row_pk, content_hash, status, assigned_to, claimed_at, lease_expires_at, completed_at, round`

func scanTask(row pgx.Row) (*domain.Task, error) {
	var t domain.Task
	err := row.Scan(&t.ID, &t.DatasetID, &t.SourceRowPK, &t.ContentHash, &t.Status,
		&t.AssignedTo, &t.ClaimedAt, &t.LeaseExpiresAt, &t.CompletedAt, &t.Round)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &t, nil
}

// ClaimTask 用 FOR UPDATE SKIP LOCKED 抢一个 PENDING 任务并置 lease（PRD §11.2）。
// 池中无可领任务时返回 ErrNoTask。
func (s *Store) ClaimTask(ctx context.Context, datasetID, userID int64, leaseMin int) (*domain.Task, error) {
	row := s.pool.QueryRow(ctx, `
		WITH next AS (
			SELECT id FROM tasks
			WHERE dataset_id = $1 AND status = 'PENDING'
			  AND EXISTS (SELECT 1 FROM datasets d WHERE d.id = $1 AND d.status = 'READY') -- 暂停/未就绪不放任务
			ORDER BY id
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE tasks t
		SET status = 'CLAIMED', assigned_to = $2, claimed_at = now(),
		    lease_expires_at = now() + ($3::int * interval '1 minute'), updated_at = now()
		FROM next n
		WHERE t.id = n.id
		RETURNING t.id, t.dataset_id, t.source_row_pk, t.content_hash, t.status,
		          t.assigned_to, t.claimed_at, t.lease_expires_at, t.completed_at, t.round`,
		datasetID, userID, leaseMin)

	t, err := scanTask(row)
	if err == ErrNotFound {
		return nil, ErrNoTask
	}
	return t, err
}

func (s *Store) GetTask(ctx context.Context, id int64) (*domain.Task, error) {
	return scanTask(s.pool.QueryRow(ctx, `SELECT `+taskCols+` FROM tasks WHERE id = $1`, id))
}

// Heartbeat 续约，返回新的 lease 到期时间。任务非本人 CLAIMED 时返回 ErrConflict。
func (s *Store) Heartbeat(ctx context.Context, taskID, userID int64, leaseMin int) (time.Time, error) {
	var lease time.Time
	err := s.pool.QueryRow(ctx, `
		UPDATE tasks
		SET lease_expires_at = now() + ($3::int * interval '1 minute'), updated_at = now()
		WHERE id = $1 AND assigned_to = $2 AND status = 'CLAIMED'
		RETURNING lease_expires_at`, taskID, userID, leaseMin).Scan(&lease)
	if err == pgx.ErrNoRows {
		return time.Time{}, ErrConflict
	}
	return lease, err
}

// ReleaseTask 主动放回池。
func (s *Store) ReleaseTask(ctx context.Context, taskID, userID int64) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE tasks
		SET status = 'PENDING', assigned_to = NULL, claimed_at = NULL,
		    lease_expires_at = NULL, updated_at = now()
		WHERE id = $1 AND assigned_to = $2 AND status = 'CLAIMED'`, taskID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

// SubmitAnnotation 在同一事务内：置 task 为 COMPLETED（幂等校验 assigned_to + CLAIMED）+ 插 annotation。
// 任务已被回收/他人提交时返回 ErrConflict（PRD §11.4）。
func (s *Store) SubmitAnnotation(ctx context.Context, taskID, userID int64, data json.RawMessage, formSchemaVersion int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var datasetID int64
	var round int
	err = tx.QueryRow(ctx, `
		UPDATE tasks
		SET status = 'COMPLETED', completed_at = now(), updated_at = now()
		WHERE id = $1 AND assigned_to = $2 AND status = 'CLAIMED'
		RETURNING dataset_id, round`, taskID, userID).Scan(&datasetID, &round)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrConflict
		}
		return err
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO annotations (task_id, dataset_id, user_id, data, form_schema_version, round)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		taskID, datasetID, userID, data, formSchemaVersion, round); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ReapExpiredLeases 回收超时 lease。advisory lock 保证多实例只有一个在跑（PRD §11.5）。
// 锁的获取/UPDATE/释放在同一连接上完成（session 级 advisory lock 的正确用法）。
func (s *Store) ReapExpiredLeases(ctx context.Context) (int64, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()

	var got bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, reaperLockKey).Scan(&got); err != nil {
		return 0, err
	}
	if !got {
		return 0, nil
	}
	defer conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, reaperLockKey)

	tag, err := conn.Exec(ctx, `
		UPDATE tasks
		SET status = 'PENDING', assigned_to = NULL, claimed_at = NULL,
		    lease_expires_at = NULL, updated_at = now()
		WHERE status = 'CLAIMED' AND lease_expires_at < now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// CreateTasks 批量为某数据集生成 PENDING 任务（供 seed / 后续导入使用）。
func (s *Store) CreateTasks(ctx context.Context, datasetID int64, pks []string) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO tasks (dataset_id, source_row_pk, status)
		SELECT $1, unnest($2::text[]), 'PENDING'
		ON CONFLICT (dataset_id, source_row_pk) DO NOTHING`, datasetID, pks)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
