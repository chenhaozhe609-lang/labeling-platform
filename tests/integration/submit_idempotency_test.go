//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

var sampleData = json.RawMessage(`{"discipline":"engineering","difficulty":4}`)

func annotationCount(t *testing.T, taskID int64) int {
	t.Helper()
	var n int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM annotations WHERE task_id = $1`, taskID).Scan(&n); err != nil {
		t.Fatalf("count annotations: %v", err)
	}
	return n
}

// AC-4：提交幂等——重复提交不重复写 annotation、不改状态。
func TestSubmit_Idempotent(t *testing.T) {
	ctx := context.Background()
	uid, dsID := seed(t, 1)
	tk, err := testStore.ClaimTask(ctx, dsID, uid, 30, nil)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := testStore.SubmitAnnotation(ctx, tk.ID, uid, sampleData, 1); err != nil {
		t.Fatalf("首次提交失败: %v", err)
	}
	if err := testStore.SubmitAnnotation(ctx, tk.ID, uid, sampleData, 1); !errors.Is(err, store.ErrConflict) {
		t.Errorf("重复提交 err=%v，期望 ErrConflict", err)
	}
	if c := annotationCount(t, tk.ID); c != 1 {
		t.Errorf("annotation 数=%d，期望 1（幂等）", c)
	}

	got, _ := testStore.GetTask(ctx, tk.ID, nil)
	if got.Status != domain.TaskCompleted {
		t.Errorf("status=%s，期望 COMPLETED", got.Status)
	}
}

// 越权提交——非持有者提交应冲突，且不写 annotation、不改状态。
func TestSubmit_OwnershipEnforced(t *testing.T) {
	ctx := context.Background()
	uid, dsID := seed(t, 1)
	tk, err := testStore.ClaimTask(ctx, dsID, uid, 30, nil)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	other := newUser(t)
	if err := testStore.SubmitAnnotation(ctx, tk.ID, other, sampleData, 1); !errors.Is(err, store.ErrConflict) {
		t.Errorf("越权提交 err=%v，期望 ErrConflict", err)
	}
	if c := annotationCount(t, tk.ID); c != 0 {
		t.Errorf("越权后 annotation 数=%d，期望 0", c)
	}
	got, _ := testStore.GetTask(ctx, tk.ID, nil)
	if got.Status != domain.TaskClaimed {
		t.Errorf("越权后 status=%s，期望仍 CLAIMED", got.Status)
	}
}

// 释放后再提交应冲突（任务已回 PENDING）。
func TestSubmit_AfterReleaseConflict(t *testing.T) {
	ctx := context.Background()
	uid, dsID := seed(t, 1)
	tk, err := testStore.ClaimTask(ctx, dsID, uid, 30, nil)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := testStore.ReleaseTask(ctx, tk.ID, uid); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := testStore.SubmitAnnotation(ctx, tk.ID, uid, sampleData, 1); !errors.Is(err, store.ErrConflict) {
		t.Errorf("释放后提交 err=%v，期望 ErrConflict", err)
	}
}
