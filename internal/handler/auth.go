// Package handler 是 HTTP 处理层。
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/middleware"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/platform/jwt"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

type AuthHandler struct {
	store *store.Store
	jm    *jwt.Manager
}

func NewAuthHandler(s *store.Store, jm *jwt.Manager) *AuthHandler {
	return &AuthHandler{store: s, jm: jm}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         any    `json:"user,omitempty"`
}

// Login 校验用户名密码，签发 token。
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码必填"})
		return
	}

	user, err := h.store.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	access, refresh, err := h.jm.Generate(user.ID, string(user.Role))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token 签发失败"})
		return
	}
	c.JSON(http.StatusOK, tokenResponse{AccessToken: access, RefreshToken: refresh, User: user})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh 用 refresh token 换新 token（角色以库内当前值为准）。
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
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

	access, refresh, err := h.jm.Generate(user.ID, string(user.Role))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token 签发失败"})
		return
	}
	c.JSON(http.StatusOK, tokenResponse{AccessToken: access, RefreshToken: refresh})
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
