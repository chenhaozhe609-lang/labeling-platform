//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

// annIDForTask 取某任务的（唯一/有效）annotation id。
func annIDForTask(t *testing.T, taskID int64) int64 {
	t.Helper()
	var id int64
	if err := testPool.QueryRow(context.Background(),
		`SELECT id FROM annotations WHERE task_id=$1 AND superseded_at IS NULL ORDER BY id DESC LIMIT 1`,
		taskID).Scan(&id); err != nil {
		t.Fatalf("ann id for task %d: %v", taskID, err)
	}
	return id
}

// B4.4：reviewer 改写并通过 —— 原标注 superseded+approved，新插 reviewer 署名的有效 approved 标注，task 仍 COMPLETED。
func TestReview_EditAndApprove(t *testing.T) {
	ctx := context.Background()
	author := newUser(t)
	reviewer := newUser(t)
	dsID := newSyncDataset(t, nil)
	testStore.InsertTasksWithHash(ctx, dsID, newBatch(t, dsID), []string{"1"}, []string{"h"})
	taskID, _, _, _ := taskByPK(t, dsID, "1")
	completeWithAnnotation(t, taskID, dsID, author)
	annID := annIDForTask(t, taskID)

	newData := json.RawMessage(`{"fills":{"x":"corrected"},"_source":"reviewer-edited"}`)
	if err := testStore.EditReview(ctx, annID, reviewer, newData, "fix typo"); err != nil {
		t.Fatalf("EditReview: %v", err)
	}

	// 原标注：superseded + approved + reviewed_by=reviewer
	var sup bool
	var rs *string
	var rb *int64
	testPool.QueryRow(ctx,
		`SELECT superseded_at IS NOT NULL, review_status, reviewed_by FROM annotations WHERE id=$1`,
		annID).Scan(&sup, &rs, &rb)
	if !sup || rs == nil || *rs != "approved" || rb == nil || *rb != reviewer {
		t.Errorf("原标注状态错：superseded=%v review_status=%v reviewed_by=%v", sup, rs, rb)
	}

	// 应共 2 条；有效那条为 reviewer 署名、approved、data 为修正值
	var total int
	testPool.QueryRow(ctx, `SELECT count(*) FROM annotations WHERE task_id=$1`, taskID).Scan(&total)
	if total != 2 {
		t.Errorf("annotation 数=%d，期望 2（原 superseded + 修正版）", total)
	}
	var newUserID int64
	var newRS *string
	var data string
	if err := testPool.QueryRow(ctx,
		`SELECT user_id, review_status, data::text FROM annotations WHERE task_id=$1 AND superseded_at IS NULL`,
		taskID).Scan(&newUserID, &newRS, &data); err != nil {
		t.Fatalf("查修正版: %v", err)
	}
	if newUserID != reviewer || newRS == nil || *newRS != "approved" || !strings.Contains(data, "corrected") {
		t.Errorf("修正版状态错：user=%d review_status=%v data=%s", newUserID, newRS, data)
	}

	// task 仍 COMPLETED
	if _, st, _, _ := taskByPK(t, dsID, "1"); st != "COMPLETED" {
		t.Errorf("task status=%s，期望 COMPLETED", st)
	}
	// 已审 → 不再出现在该 reviewer 的队列
	items, _ := testStore.ReviewQueue(ctx, dsID, reviewer, 10)
	for _, it := range items {
		if it.TaskID == taskID {
			t.Errorf("已改写通过的任务仍在审核队列")
		}
	}
}

// 不能改写本人提交的标注。
func TestReview_EditSelfForbidden(t *testing.T) {
	ctx := context.Background()
	author := newUser(t)
	dsID := newSyncDataset(t, nil)
	testStore.InsertTasksWithHash(ctx, dsID, newBatch(t, dsID), []string{"1"}, []string{"h"})
	taskID, _, _, _ := taskByPK(t, dsID, "1")
	completeWithAnnotation(t, taskID, dsID, author)
	annID := annIDForTask(t, taskID)

	err := testStore.EditReview(ctx, annID, author, json.RawMessage(`{"fills":{"x":"y"}}`), "")
	if !errors.Is(err, store.ErrForbidden) {
		t.Errorf("自改 err=%v，期望 ErrForbidden", err)
	}
}

// B4.2：队列带上一版（已废弃）标注，供「旧↔新」对比。
func TestReview_QueueReturnsPrevious(t *testing.T) {
	ctx := context.Background()
	author := newUser(t)
	reviewer := newUser(t)
	dsID := newSyncDataset(t, nil)
	testStore.InsertTasksWithHash(ctx, dsID, newBatch(t, dsID), []string{"1"}, []string{"h"})
	taskID, _, _, _ := taskByPK(t, dsID, "1")

	// round1 标注被废弃（模拟曾被打回重标），round2 为当前有效、任务 COMPLETED
	testPool.Exec(ctx, `INSERT INTO annotations (task_id,dataset_id,user_id,data,form_schema_version,round,superseded_at)
		VALUES ($1,$2,$3,'{"fills":{"x":"OLD"}}',1,1, now())`, taskID, dsID, author)
	testPool.Exec(ctx, `UPDATE tasks SET status='COMPLETED', round=2, completed_at=now() WHERE id=$1`, taskID)
	testPool.Exec(ctx, `INSERT INTO annotations (task_id,dataset_id,user_id,data,form_schema_version,round)
		VALUES ($1,$2,$3,'{"fills":{"x":"NEW"}}',1,2)`, taskID, dsID, author)

	items, err := testStore.ReviewQueue(ctx, dsID, reviewer, 10)
	if err != nil {
		t.Fatalf("ReviewQueue: %v", err)
	}
	var found *store.ReviewItem
	for i := range items {
		if items[i].TaskID == taskID {
			found = &items[i]
		}
	}
	if found == nil {
		t.Fatal("队列未含该任务")
	}
	if !strings.Contains(string(found.Data), "NEW") {
		t.Errorf("current data=%s，期望含 NEW", found.Data)
	}
	if len(found.PrevData) == 0 || !strings.Contains(string(found.PrevData), "OLD") {
		t.Errorf("previous=%s，期望含 OLD（旧↔新对比）", found.PrevData)
	}
}
