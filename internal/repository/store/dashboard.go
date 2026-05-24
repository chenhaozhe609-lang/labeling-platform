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
	NeedsRedo      int           `json:"needs_redo"`
	TodaySubmitted int           `json:"today_submitted"`
	ActiveToday    int           `json:"active_today"`
	Leaderboard    []LeaderRow   `json:"leaderboard"`
	Activity       []ActivityRow `json:"activity"`
}

// GetDashboard 从 meta-db 聚合全局看板数据。
func (s *Store) GetDashboard(ctx context.Context) (*DashboardData, error) {
	d := &DashboardData{Leaderboard: []LeaderRow{}, Activity: []ActivityRow{}}

	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM datasets`).Scan(&d.Datasets); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `SELECT status, count(*) FROM tasks GROUP BY status`)
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
		case "NEEDS_REDO":
			d.NeedsRedo = n
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	_ = s.pool.QueryRow(ctx,
		`SELECT count(*) FROM annotations WHERE created_at >= current_date`).Scan(&d.TodaySubmitted)
	_ = s.pool.QueryRow(ctx,
		`SELECT count(DISTINCT user_id) FROM annotations WHERE created_at >= current_date`).Scan(&d.ActiveToday)

	lb, err := s.pool.Query(ctx, `
		SELECT a.user_id, u.username, count(*) AS n
		FROM annotations a JOIN users u ON u.id = a.user_id
		WHERE a.superseded_at IS NULL
		GROUP BY a.user_id, u.username
		ORDER BY n DESC LIMIT 8`)
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
		FROM annotations a JOIN users u ON u.id = a.user_id
		ORDER BY a.created_at DESC LIMIT 10`)
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
