package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

// ReviewItem 审核队列中的一条：某 COMPLETED 任务上的有效（未废弃）、未审、且非本人提交的标注（C5.1）。
type ReviewItem struct {
	AnnotationID int64           `json:"annotation_id"`
	TaskID       int64           `json:"task_id"`
	SourceRowPK  string          `json:"source_row_pk"`
	Round        int             `json:"round"`
	Annotator    string          `json:"annotator"`
	Data         json.RawMessage `json:"data"`
	PrevData     json.RawMessage `json:"-"` // 同任务上一版（已废弃）标注的 data，供「旧↔新」对比（B4.2）；可空
	CreatedAt    time.Time       `json:"created_at"`
}

// ReviewQueue 随机抽检（C5.1）：某数据集下「COMPLETED 任务 + 有效未审 + 非本人」的标注，
// ORDER BY random() 取样 limit 条。reviewer 每次刷新得到新的随机批次，即为抽检。
func (s *Store) ReviewQueue(ctx context.Context, datasetID, reviewerID int64, limit int) ([]ReviewItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.task_id, t.source_row_pk, t.round, u.username, a.data, a.created_at,
		       (SELECT pa.data FROM annotations pa
		        WHERE pa.task_id = a.task_id AND pa.superseded_at IS NOT NULL
		        ORDER BY pa.round DESC, pa.created_at DESC LIMIT 1) AS prev_data
		FROM annotations a
		JOIN tasks t ON t.id = a.task_id
		JOIN users u ON u.id = a.user_id
		WHERE a.dataset_id = $1
		  AND a.superseded_at IS NULL
		  AND a.reviewed_at IS NULL
		  AND a.user_id <> $2
		  AND t.status = 'COMPLETED'
		ORDER BY random()
		LIMIT $3`, datasetID, reviewerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ReviewItem, 0, limit)
	for rows.Next() {
		var it ReviewItem
		var prev []byte // 可空：上一版标注 data
		if err := rows.Scan(&it.AnnotationID, &it.TaskID, &it.SourceRowPK, &it.Round,
			&it.Annotator, &it.Data, &it.CreatedAt, &prev); err != nil {
			return nil, err
		}
		it.PrevData = prev
		out = append(out, it)
	}
	return out, rows.Err()
}

// CountReviewPending 待抽检池总量（与 ReviewQueue 同口径，去掉随机与 limit）。
func (s *Store) CountReviewPending(ctx context.Context, datasetID, reviewerID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM annotations a JOIN tasks t ON t.id = a.task_id
		WHERE a.dataset_id = $1 AND a.superseded_at IS NULL AND a.reviewed_at IS NULL
		  AND a.user_id <> $2 AND t.status = 'COMPLETED'`, datasetID, reviewerID).Scan(&n)
	return n, err
}

// SubmitReview 裁决一条标注（C5.2），单事务：
//   - approved：标 reviewed_at/by/status，task 保持 COMPLETED。
//   - needs_redo：标审核结果 + 废弃该标注 + 任务回 PENDING 且 round+1（与增量重标 SyncTasks 一致），
//     重新进入领取池由标注员重做。
//
// 返回 ErrConflict（已被审/已废弃/任务非 COMPLETED），ErrForbidden（审本人标注）。
func (s *Store) SubmitReview(ctx context.Context, annotationID, reviewerID int64, status, note string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var authorID, taskID int64
	var taskStatus string
	err = tx.QueryRow(ctx, `
		SELECT a.user_id, a.task_id, t.status
		FROM annotations a JOIN tasks t ON t.id = a.task_id
		WHERE a.id = $1 AND a.superseded_at IS NULL AND a.reviewed_at IS NULL
		FOR UPDATE OF a`, annotationID).Scan(&authorID, &taskID, &taskStatus)
	if err == pgx.ErrNoRows {
		return ErrConflict // 已被他人审/已废弃/不存在
	}
	if err != nil {
		return err
	}
	if authorID == reviewerID {
		return ErrForbidden // 不能审本人提交（队列已过滤，双保险）
	}
	if taskStatus != "COMPLETED" {
		return ErrConflict // 任务状态已变（如已被重标回 PENDING）
	}

	if _, err = tx.Exec(ctx, `
		UPDATE annotations
		SET reviewed_at = now(), reviewed_by = $2, review_status = $3, review_note = NULLIF($4, '')
		WHERE id = $1`, annotationID, reviewerID, status, note); err != nil {
		return err
	}

	if status == "needs_redo" {
		if _, err = tx.Exec(ctx,
			`UPDATE annotations SET superseded_at = now() WHERE id = $1`, annotationID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `
			UPDATE tasks
			SET status = 'PENDING', assigned_to = NULL, claimed_at = NULL, lease_expires_at = NULL,
			    completed_at = NULL, round = round + 1, updated_at = now()
			WHERE id = $1`, taskID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// EditReview 审核改写并通过（B4.4）：reviewer 微调 fills → 原标注 superseded + 标 approved，
// 新插一条 reviewer 署名、有效、已审通过的修正标注；task 保持 COMPLETED，沿用原 round / form_schema_version。
// 返回 ErrConflict（已被审/已废弃/任务非 COMPLETED），ErrForbidden（改本人标注）。
func (s *Store) EditReview(ctx context.Context, annotationID, reviewerID int64, data json.RawMessage, note string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var authorID, taskID, datasetID int64
	var round, fsv int
	var taskStatus string
	err = tx.QueryRow(ctx, `
		SELECT a.user_id, a.task_id, a.dataset_id, a.round, a.form_schema_version, t.status
		FROM annotations a JOIN tasks t ON t.id = a.task_id
		WHERE a.id = $1 AND a.superseded_at IS NULL AND a.reviewed_at IS NULL
		FOR UPDATE OF a`, annotationID).Scan(&authorID, &taskID, &datasetID, &round, &fsv, &taskStatus)
	if err == pgx.ErrNoRows {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if authorID == reviewerID {
		return ErrForbidden
	}
	if taskStatus != "COMPLETED" {
		return ErrConflict
	}

	// 原标注：reviewer 已处理（approved）且被修正版取代（superseded）。
	if _, err = tx.Exec(ctx, `
		UPDATE annotations
		SET reviewed_at = now(), reviewed_by = $2, review_status = 'approved',
		    review_note = NULLIF($3, ''), superseded_at = now()
		WHERE id = $1`, annotationID, reviewerID, note); err != nil {
		return err
	}
	// 修正版：reviewer 署名、有效(未废弃)、已审通过。
	if _, err = tx.Exec(ctx, `
		INSERT INTO annotations (task_id, dataset_id, user_id, data, form_schema_version, round,
		                         reviewed_at, reviewed_by, review_status)
		VALUES ($1, $2, $3, $4, $5, $6, now(), $3, 'approved')`,
		taskID, datasetID, reviewerID, data, fsv, round); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
