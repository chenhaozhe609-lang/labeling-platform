// Package domain 领域模型。
package domain

import "time"

type Role string

const (
	RoleAnnotator Role = "annotator"
	RoleReviewer  Role = "reviewer"
	RoleAdmin     Role = "admin"
)

// Valid 校验 role 取值合法。
func (r Role) Valid() bool {
	switch r {
	case RoleAnnotator, RoleReviewer, RoleAdmin:
		return true
	default:
		return false
	}
}

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
