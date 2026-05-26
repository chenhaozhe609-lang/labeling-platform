package handler

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/middleware"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

// InviteHandler 管理者管理本组织的邀请（RequireRole admin，按 org 限定）。
type InviteHandler struct {
	store *store.Store
	ttl   time.Duration
}

func NewInviteHandler(s *store.Store, ttl time.Duration) *InviteHandler {
	return &InviteHandler{store: s, ttl: ttl}
}

// Create 生成邀请。POST /admin/invites {role, email?}。返回 invite（含 token）+ accept_path。
func (h *InviteHandler) Create(c *gin.Context) {
	orgID := middleware.OrgID(c)
	if orgID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "超管不属于任何组织，无法发邀请"})
		return
	}
	var req struct {
		Role  string `json:"role"`
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	role := domain.Role(req.Role)
	if !role.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色非法（annotator|reviewer|admin）"})
		return
	}
	var email string // 可选限定受邀邮箱
	if strings.TrimSpace(req.Email) != "" {
		e, ok := normalizeEmail(req.Email)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱格式不合法"})
			return
		}
		email = e
	}
	token, err := randomToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成邀请失败"})
		return
	}

	uid := userID(c)
	inv, err := h.store.CreateInvite(c.Request.Context(), &domain.Invite{
		OrgID:     *orgID,
		Role:      role,
		Token:     token,
		Email:     email,
		CreatedBy: &uid,
		ExpiresAt: time.Now().Add(h.ttl),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成邀请失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invite": inv, "accept_path": "/accept-invite?token=" + token})
}

// List 本组织邀请。GET /admin/invites。
func (h *InviteHandler) List(c *gin.Context) {
	orgID := middleware.OrgID(c)
	if orgID == nil {
		c.JSON(http.StatusOK, gin.H{"items": []domain.Invite{}})
		return
	}
	items, err := h.store.ListInvites(c.Request.Context(), *orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询邀请失败"})
		return
	}
	if items == nil {
		items = []domain.Invite{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// Delete 撤销邀请。DELETE /admin/invites/:id（限本组织）。
func (h *InviteHandler) Delete(c *gin.Context) {
	orgID := middleware.OrgID(c)
	if orgID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "超管不属于任何组织"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	err = h.store.DeleteInvite(c.Request.Context(), *orgID, id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "邀请不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "撤销失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// randomToken 生成 32 字节随机、URL 安全的不可猜 token。
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
