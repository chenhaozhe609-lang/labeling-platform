// Package handler 是 HTTP 处理层。
package handler

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/middleware"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/platform/jwt"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

// minPasswordLen 注册/改密的密码下限（登录硬化，§4）。
const minPasswordLen = 8

type AuthHandler struct {
	store   *store.Store
	jm      *jwt.Manager
	limiter *middleware.LoginLimiter
}

func NewAuthHandler(s *store.Store, jm *jwt.Manager, limiter *middleware.LoginLimiter) *AuthHandler {
	return &AuthHandler{store: s, jm: jm, limiter: limiter}
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         any    `json:"user,omitempty"`
	Org          any    `json:"org,omitempty"`
}

// issue 按用户当前 org/token_version 签发 access+refresh，并组织成响应。
func (h *AuthHandler) issue(c *gin.Context, u *domain.User, org any) {
	access, refresh, err := h.jm.Generate(u.ID, string(u.Role), u.OrgID, u.TokenVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token 签发失败"})
		return
	}
	c.JSON(http.StatusOK, tokenResponse{AccessToken: access, RefreshToken: refresh, User: u, Org: org})
}

// Signup 开放注册：建组织 + owner（该组织 admin），单事务。POST /auth/signup。
func (h *AuthHandler) Signup(c *gin.Context) {
	var req struct {
		OrgName  string `json:"org_name"`
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	req.OrgName = strings.TrimSpace(req.OrgName)
	req.Username = strings.TrimSpace(req.Username)
	email, ok := normalizeEmail(req.Email)
	if req.OrgName == "" || req.Username == "" || !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "组织名、用户名、合法邮箱必填"})
		return
	}
	if len(req.Password) < minPasswordLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码至少 8 位"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	org, user, err := h.store.CreateOrgWithOwner(c.Request.Context(), req.OrgName,
		store.NewUser{Username: req.Username, Email: email, PasswordHash: string(hash)})
	if errors.Is(err, store.ErrConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已注册"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
		return
	}
	h.issue(c, user, org)
}

// Login 按邮箱校验密码、签发 token；按「邮箱+IP」失败限流锁定。POST /auth/login。
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱和密码必填"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱和密码必填"})
		return
	}

	key := email + "|" + c.ClientIP()
	if locked, _ := h.limiter.Locked(key); locked {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "登录尝试过多，请稍后再试"})
		return
	}

	user, err := h.store.GetUserByEmail(c.Request.Context(), email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.limiter.Fail(key)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		h.limiter.Fail(key)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	h.limiter.Reset(key)
	h.issue(c, user, nil)
}

// Refresh 用 refresh token 换新 token：校验 token_version（吊销）与库内当前角色/组织。POST /auth/refresh。
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token 必填"})
		return
	}

	claims, err := h.jm.Parse(req.RefreshToken)
	if err != nil || claims.Type != jwt.TypeRefresh {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token 无效"})
		return
	}

	user, err := h.store.GetUserByID(c.Request.Context(), claims.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}
	// 吊销校验：token 签发时的 tv 必须等于库内当前 tv，否则该 token 已被 logout-all/改密作废。
	if claims.TokenVersion != user.TokenVersion {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "会话已失效，请重新登录"})
		return
	}

	h.issue(c, user, nil)
}

// LogoutAll 使该用户所有已签发 token 失效（bump token_version）。POST /auth/logout-all（需登录）。
func (h *AuthHandler) LogoutAll(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxUserID)
	id, _ := uid.(int64)
	if err := h.store.BumpTokenVersion(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "登出失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// AcceptInvite 凭邀请 token 在对应组织建用户（role 取自邀请），并发 token。POST /auth/accept-invite。
// 请求带 email（受邀人自选登录邮箱）；若邀请限定了邮箱，须与之一致。
func (h *AuthHandler) AcceptInvite(c *gin.Context) {
	var req struct {
		Token    string `json:"token"`
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	req.Username = strings.TrimSpace(req.Username)
	email, ok := normalizeEmail(req.Email)
	if req.Token == "" || req.Username == "" || !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邀请、用户名、合法邮箱必填"})
		return
	}
	if len(req.Password) < minPasswordLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码至少 8 位"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	user, err := h.store.AcceptInvite(c.Request.Context(), req.Token, req.Username, email, string(hash))
	switch {
	case errors.Is(err, store.ErrNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": "邀请无效或已过期"})
		return
	case errors.Is(err, store.ErrInviteEmailMismatch):
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱与邀请不符"})
		return
	case errors.Is(err, store.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已注册"})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加入失败"})
		return
	}
	h.issue(c, user, nil)
}

// Me 返回当前登录用户。
func (h *AuthHandler) Me(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxUserID)
	id, _ := uid.(int64)
	user, err := h.store.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// normalizeEmail 去空格转小写并做基本格式校验；返回归一后的邮箱与是否合法。
func normalizeEmail(raw string) (string, bool) {
	e := strings.ToLower(strings.TrimSpace(raw))
	if e == "" {
		return "", false
	}
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Address != e {
		return "", false
	}
	return e, true
}
