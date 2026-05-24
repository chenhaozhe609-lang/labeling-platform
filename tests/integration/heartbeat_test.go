//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

// AC-3：heartbeat 续约延长 lease；非持有者续约返回冲突。
func TestHeartbeat_ExtendsLease(t *testing.T) {
	ctx := context.Background()
	uid, dsID := seed(t, 1)

	tk, err := testStore.ClaimTask(ctx, dsID, uid, 1) // 1 分钟租约
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if tk.LeaseExpiresAt == nil {
		t.Fatalf("claim 未设置 lease")
	}

	newLease, err := testStore.Heartbeat(ctx, tk.ID, uid, 30) // 续到 30 分钟
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if !newLease.After(*tk.LeaseExpiresAt) {
		t.Errorf("续约后 lease=%v 未晚于原 lease=%v", newLease, *tk.LeaseExpiresAt)
	}
}

func TestHeartbeat_WrongUserConflict(t *testing.T) {
	ctx := context.Background()
	uid, dsID := seed(t, 1)
	tk, err := testStore.ClaimTask(ctx, dsID, uid, 30)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	other := newUser(t)
	if _, err := testStore.Heartbeat(ctx, tk.ID, other, 30); !errors.Is(err, store.ErrConflict) {
		t.Errorf("非持有者续约 err=%v，期望 ErrConflict", err)
	}
}
