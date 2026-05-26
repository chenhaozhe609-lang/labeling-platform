package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/middleware"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

// sameOrg 报告目标用户是否在调用者所属组织内（超管恒为 true）——用户管理的越权护栏。
func sameOrg(c *gin.Context, u *domain.User) bool {
	if middleware.IsSuperadmin(c) {
		return true
	}
	org := middleware.OrgID(c)
	return org != nil && u.OrgID != nil && *org == *u.OrgID
}

type UserHandler struct {
	store *store.Store
}

func NewUserHandler(s *store.Store) *UserHandler {
	return &UserHandler{store: s}
}

// List 本组织全部用户（admin）。
func (h *UserHandler) List(c *gin.Context) {
	users, err := h.store.ListUsers(c.Request.Context(), middleware.OrgID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败"})
		return
	}
	if users == nil {
		users = []domain.User{}
	}
	c.JSON(http.StatusOK, gin.H{"items": users})
}

// Create 在本组织新建用户（admin）。新用户归调用者所在组织。
func (h *UserHandler) Create(c *gin.Context) {
	orgID := middleware.OrgID(c)
	if orgID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请在组织上下文中创建用户"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	email, okEmail := normalizeEmail(req.Email)
	role := domain.Role(req.Role)
	if req.Username == "" || !okEmail || len(req.Password) < minPasswordLen || !role.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名、合法邮箱必填、密码≥8位、角色合法（annotator|reviewer|admin）"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}
	u, err := h.store.CreateUser(c.Request.Context(), store.NewUser{
		Username: req.Username, Email: email, PasswordHash: string(hash), Role: role, OrgID: orgID,
	})
	if errors.Is(err, store.ErrConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已注册"})
		return
	}
	if err != nil {
		slog.Error("创建用户失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		return
	}
	c.JSON(http.StatusOK, u)
}

// Update 改角色 / 重置密码（admin）。body：{role?, password?}。
func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	var req struct {
		Role     *string `json:"role"`
		Password *string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	ctx := c.Request.Context()
	target, err := h.store.GetUserByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	if !sameOrg(c, target) { // 越权护栏：不暴露跨组织用户存在
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if req.Role != nil {
		role := domain.Role(*req.Role)
		if !role.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "角色非法"})
			return
		}
		// 守卫：不能把本组织最后一个 admin 降级（含降自己）。
		if target.Role == domain.RoleAdmin && role != domain.RoleAdmin {
			if n, _ := h.store.CountAdmins(ctx, middleware.OrgID(c)); n <= 1 {
				c.JSON(http.StatusConflict, gin.H{"error": "至少保留一个管理员"})
				return
			}
		}
		if err := h.store.UpdateUserRole(ctx, id, role); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "改角色失败"})
			return
		}
	}

	if req.Password != nil {
		if len(*req.Password) < minPasswordLen {
			c.JSON(http.StatusBadRequest, gin.H{"error": "密码≥8位"})
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
			return
		}
		if err := h.store.UpdateUserPassword(ctx, id, string(hash)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "重置密码失败"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Delete 删除用户（admin）。守卫：不能删自己 / 不能删最后一个 admin；有关联数据时 409。
func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	if id == userID(c) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除自己"})
		return
	}
	ctx := c.Request.Context()
	target, err := h.store.GetUserByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	if !sameOrg(c, target) { // 越权护栏：不暴露跨组织用户存在
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if target.Role == domain.RoleAdmin {
		if n, _ := h.store.CountAdmins(ctx, middleware.OrgID(c)); n <= 1 {
			c.JSON(http.StatusConflict, gin.H{"error": "至少保留一个管理员"})
			return
		}
	}
	err = h.store.DeleteUser(ctx, id)
	if errors.Is(err, store.ErrConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": "该用户已有标注/创建过数据集，无法删除"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
