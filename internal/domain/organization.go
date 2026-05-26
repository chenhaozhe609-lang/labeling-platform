package domain

import "time"

// Organization 是多租户的隔离单位：每个 manager 注册即建一个组织，其用户/数据集归该组织。
type Organization struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	OwnerID   *int64    `json:"owner_id,omitempty"` // 该组织 owner（admin）用户 id；建组织事务内回填
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Invite 邀请加入既有组织的一次性凭证（token 进邀请链接）。
type Invite struct {
	ID         int64      `json:"id"`
	OrgID      int64      `json:"org_id"`
	Role       Role       `json:"role"`
	Token      string     `json:"token"`
	Email      string     `json:"email,omitempty"` // 限定受邀邮箱；空=不限
	CreatedBy  *int64     `json:"created_by,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	AcceptedBy *int64     `json:"accepted_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
