// Package job 后台周期任务。
package job

import (
	"context"
	"log/slog"
	"time"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

// Reaper 周期回收超时 lease（PRD §11.5）。
type Reaper struct {
	store    *store.Store
	interval time.Duration
}

func NewReaper(s *store.Store, interval time.Duration) *Reaper {
	return &Reaper{store: s, interval: interval}
}

// Run 阻塞运行直到 ctx 取消；启动立即跑一次。
func (r *Reaper) Run(ctx context.Context) {
	r.tick(ctx)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

func (r *Reaper) tick(ctx context.Context) {
	n, err := r.store.ReapExpiredLeases(ctx)
	if err != nil {
		slog.Error("reaper 执行失败", "error", err)
		return
	}
	if n > 0 {
		slog.Info("lease 已回收", "count", n)
	}
}
