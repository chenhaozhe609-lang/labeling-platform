//go:build integration

package integration

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

// AC-1：N 个任务、50 并发反复领取，要求总领取数 = N 且无任何任务被领取两次。
// 验证 FOR UPDATE SKIP LOCKED 的互斥正确性（配合 -race）。
func TestConcurrentClaim_NoDuplicates(t *testing.T) {
	ctx := context.Background()
	const nTasks = 300
	const workers = 50
	uid, dsID := seed(t, nTasks)

	var wg sync.WaitGroup
	var mu sync.Mutex
	claimed := make(map[int64]int, nTasks)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				tk, err := testStore.ClaimTask(ctx, dsID, uid, 30)
				if errors.Is(err, store.ErrNoTask) {
					return
				}
				if err != nil {
					t.Errorf("claim 出错: %v", err)
					return
				}
				mu.Lock()
				claimed[tk.ID]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(claimed) != nTasks {
		t.Fatalf("领到 %d 个不同任务，期望 %d", len(claimed), nTasks)
	}
	dups := 0
	for id, c := range claimed {
		if c > 1 {
			dups++
			t.Errorf("任务 %d 被领取 %d 次（重复）", id, c)
		}
	}
	if dups == 0 {
		t.Logf("OK: %d 任务被 %d 并发恰好各领取一次", nTasks, workers)
	}
}
