package domain

import (
	"encoding/json"
	"time"
)

type TaskStatus string

const (
	TaskPending   TaskStatus = "PENDING"
	TaskClaimed   TaskStatus = "CLAIMED"
	TaskCompleted TaskStatus = "COMPLETED"
	TaskNeedsRedo TaskStatus = "NEEDS_REDO"
)

type Task struct {
	ID             int64      `json:"id"`
	DatasetID      int64      `json:"dataset_id"`
	SourceRowPK    string     `json:"source_row_pk"`
	ContentHash    *string    `json:"content_hash,omitempty"`
	Status         TaskStatus `json:"status"`
	AssignedTo     *int64     `json:"assigned_to,omitempty"`
	ClaimedAt      *time.Time `json:"claimed_at,omitempty"`
	LeaseExpiresAt *time.Time `json:"lease_expires_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Round          int        `json:"round"`
}

// Annotation 一次标注提交。
type Annotation struct {
	ID                int64           `json:"id"`
	TaskID            int64           `json:"task_id"`
	DatasetID         int64           `json:"dataset_id"`
	UserID            int64           `json:"user_id"`
	Data              json.RawMessage `json:"data"`
	FormSchemaVersion int             `json:"form_schema_version"`
	Round             int             `json:"round"`
	CreatedAt         time.Time       `json:"created_at"`
}
