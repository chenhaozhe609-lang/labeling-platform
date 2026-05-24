package domain

import (
	"encoding/json"
	"time"
)

type DatasetStatus string

const (
	StatusImporting DatasetStatus = "IMPORTING"
	StatusReady     DatasetStatus = "READY"
	StatusPaused    DatasetStatus = "PAUSED"
	StatusDone      DatasetStatus = "DONE"
	StatusFailed    DatasetStatus = "FAILED"
)

type Dataset struct {
	ID                int64           `json:"id"`
	Name              string          `json:"name"`
	SourceSchema      string          `json:"source_schema"`
	SourceTable       string          `json:"source_table"`
	SourcePKColumn    string          `json:"source_pk_column"`
	FormSchema        json.RawMessage `json:"form_schema"`
	FormSchemaVersion int             `json:"form_schema_version"`
	Status            DatasetStatus   `json:"status"`
	TotalRows         int             `json:"total_rows"`
	CreatedAt         time.Time       `json:"created_at"`
}

// DatasetListItem 是列表/看板用的轻量投影（含进度计数）。
type DatasetListItem struct {
	ID                int64         `json:"id"`
	Name              string        `json:"name"`
	Status            DatasetStatus `json:"status"`
	TotalRows         int           `json:"total_rows"`
	Completed         int           `json:"completed"`
	Pending           int           `json:"pending"`
	Claimed           int           `json:"claimed"`
	FormSchemaVersion int           `json:"form_schema_version"`
}
