//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

// AC-2：领取后租约过期 → reaper 把任务回收为 PENDING、清空 assigned_to/lease。
func TestReaper_ReclaimsExpiredLease(t *testing.T) {
	ctx := context.Background()
	uid, dsID := seed(t, 2)

	tk, err := testStore.ClaimTask(ctx, dsID, uid, 30)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	// 强制把 lease 设到过去
	if _, err := testPool.Exec(ctx,
		`UPDATE tasks SET lease_expires_at = now() - interval '1 minute' WHERE id = $1`, tk.ID); err != nil {
		t.Fatalf("force expire: %v", err)
	}

	n, err := testStore.ReapExpiredLeases(ctx)
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if n < 1 {
		t.Fatalf("回收数=%d，期望 >=1", n)
	}

	got, err := testStore.GetTask(ctx, tk.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != domain.TaskPending {
		t.Errorf("status=%s，期望 PENDING", got.Status)
	}
	if got.AssignedTo != nil {
		t.Errorf("assigned_to 未清空: %v", *got.AssignedTo)
	}
	if got.LeaseExpiresAt != nil {
		t.Errorf("lease_expires_at 未清空")
	}
}

// 未过期的任务不应被回收。
func TestReaper_KeepsActiveLease(t *testing.T) {
	ctx := context.Background()
	uid, dsID := seed(t, 1)
	tk, err := testStore.ClaimTask(ctx, dsID, uid, 30)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := testStore.ReapExpiredLeases(ctx); err != nil {
		t.Fatalf("reap: %v", err)
	}
	got, _ := testStore.GetTask(ctx, tk.ID)
	if got.Status != domain.TaskClaimed {
		t.Errorf("活跃任务被误回收，status=%s", got.Status)
	}
}
