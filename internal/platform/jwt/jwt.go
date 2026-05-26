// Package jwt 封装 access / refresh token 的签发与解析。
package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	TypeAccess  TokenType = "access"
	TypeRefresh TokenType = "refresh"
)

var ErrInvalidToken = errors.New("invalid token")

// Claims 自定义负载：user_id + role + org_id + token_version + token 类型。
// org_id 为空（nil）表示平台超管（不属于任何业务组织）。
type Claims struct {
	UserID       int64     `json:"uid"`
	Role         string    `json:"role"`
	OrgID        *int64    `json:"org_id,omitempty"`
	TokenVersion int       `json:"tv"`
	Type         TokenType `json:"typ"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewManager(secret string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

// Generate 同时签发 access 与 refresh token。orgID 为 nil 表示超管；tv 为签发时的 token_version。
func (m *Manager) Generate(userID int64, role string, orgID *int64, tv int) (access, refresh string, err error) {
	now := time.Now()
	access, err = m.sign(userID, role, orgID, tv, TypeAccess, now, m.accessTTL)
	if err != nil {
		return "", "", err
	}
	refresh, err = m.sign(userID, role, orgID, tv, TypeRefresh, now, m.refreshTTL)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func (m *Manager) sign(userID int64, role string, orgID *int64, tv int, typ TokenType, now time.Time, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:       userID,
		Role:         role,
		OrgID:        orgID,
		TokenVersion: tv,
		Type:         typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Parse 校验签名与过期，返回 Claims。
func (m *Manager) Parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (m *Manager) AccessTTL() time.Duration { return m.accessTTL }
