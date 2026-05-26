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
	Username     string    `json:"username"` // 显示名，不再全局唯一
	Email        string    `json:"email"`    // 登录标识，全局唯一（超管亦用邮箱登录）
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	OrgID        *int64    `json:"org_id,omitempty"` // 业务用户必填；超管为 NULL
	TokenVersion int       `json:"-"`                // 吊销计数：+1 使旧 token 失效，不外泄
	IsSuperadmin bool      `json:"is_superadmin"`    // 平台超管（跨组织）
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
