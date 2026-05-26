package store

import (
	"context"
	"time"
)

type LeaderRow struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Count    int    `json:"count"`
}

type ActivityRow struct {
	Username  string    `json:"username"`
	TaskID    int64     `json:"task_id"`
	DatasetID int64     `json:"dataset_id"`
	At        time.Time `json:"at"`
}

type DashboardData struct {
	Datasets       int           `json:"datasets"`
	Pending        int           `json:"pending"`
	Claimed        int           `json:"claimed"`
	Completed      int           `json:"completed"`
	Approved       int           `json:"approved"`   // 审核通过的标注数（累计）
	NeedsRedo      int           `json:"needs_redo"` // 被审核打回的标注数（累计）；非 task 状态
	TodaySubmitted int           `json:"today_submitted"`
	ActiveToday    int           `json:"active_today"`
	Leaderboard    []LeaderRow   `json:"leaderboard"`
	Activity       []ActivityRow `json:"activity"`
}

// GetDashboard 聚合看板数据，按组织隔离：orgID 非 nil 时仅统计该组织数据集下的任务/标注；
// orgID 为 nil（超管）统计全局。所有涉及 tasks/annotations 的查询都 JOIN datasets 过滤 org。
func (s *Store) GetDashboard(ctx context.Context, orgID *int64) (*DashboardData, error) {
	d := &DashboardData{Leaderboard: []LeaderRow{}, Activity: []ActivityRow{}}

	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM datasets WHERE ($1::bigint IS NULL OR org_id = $1)`, orgID).Scan(&d.Datasets); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT t.status, count(*)
		FROM tasks t JOIN datasets d ON d.id = t.dataset_id
		WHERE ($1::bigint IS NULL OR d.org_id = $1)
		GROUP BY t.status`, orgID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			rows.Close()
			return nil, err
		}
		switch status {
		case "PENDING":
			d.Pending = n
		case "CLAIMED":
			d.Claimed = n
		case "COMPLETED":
			d.Completed = n
			// NEEDS_REDO 不作为 task 状态出现（打回走「回 PENDING」）；打回量改由 annotations.review_status 统计，见下。
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	_ = s.pool.QueryRow(ctx, `
		SELECT count(*), count(DISTINCT a.user_id)
		FROM annotations a JOIN datasets d ON d.id = a.dataset_id
		WHERE a.created_at >= current_date AND ($1::bigint IS NULL OR d.org_id = $1)`,
		orgID).Scan(&d.TodaySubmitted, &d.ActiveToday)

	// 审核情况（累计）：approved / needs_redo 来自 annotations.review_status，而非 task 状态。
	_ = s.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE a.review_status = 'approved'),
		       count(*) FILTER (WHERE a.review_status = 'needs_redo')
		FROM annotations a JOIN datasets d ON d.id = a.dataset_id
		WHERE ($1::bigint IS NULL OR d.org_id = $1)`, orgID).Scan(&d.Approved, &d.NeedsRedo)

	lb, err := s.pool.Query(ctx, `
		SELECT a.user_id, u.username, count(*) AS n
		FROM annotations a JOIN users u ON u.id = a.user_id JOIN datasets d ON d.id = a.dataset_id
		WHERE a.superseded_at IS NULL AND ($1::bigint IS NULL OR d.org_id = $1)
		GROUP BY a.user_id, u.username
		ORDER BY n DESC LIMIT 8`, orgID)
	if err == nil {
		for lb.Next() {
			var r LeaderRow
			if lb.Scan(&r.UserID, &r.Username, &r.Count) == nil {
				d.Leaderboard = append(d.Leaderboard, r)
			}
		}
		lb.Close()
	}

	act, err := s.pool.Query(ctx, `
		SELECT u.username, a.task_id, a.dataset_id, a.created_at
		FROM annotations a JOIN users u ON u.id = a.user_id JOIN datasets d ON d.id = a.dataset_id
		WHERE ($1::bigint IS NULL OR d.org_id = $1)
		ORDER BY a.created_at DESC LIMIT 10`, orgID)
	if err == nil {
		for act.Next() {
			var r ActivityRow
			if act.Scan(&r.Username, &r.TaskID, &r.DatasetID, &r.At) == nil {
				d.Activity = append(d.Activity, r)
			}
		}
		act.Close()
	}

	return d, nil
}
