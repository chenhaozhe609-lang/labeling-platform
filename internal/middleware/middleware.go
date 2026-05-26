// Package middleware 提供 Gin 中间件链：日志 / 恢复 / CORS / 限流 / 鉴权 / RBAC。
package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/platform/jwt"
)

// Gin context 键。
const (
	CtxUserID       = "uid"
	CtxRole         = "role"
	CtxOrgID        = "org_id"        // *int64：业务用户的组织；超管为 nil
	CtxIsSuperadmin = "is_superadmin" // bool：平台超管（org_id 为空）
)

// Logger 用 slog 输出结构化访问日志。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"dur_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		)
	}
}

// Recover 捕获 panic，返回 500。
func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered", "error", r, "path", c.Request.URL.Path)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			}
		}()
		c.Next()
	}
}

// CORS 开发期跨域（前端 dev server）。
func CORS(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// SecurityHeaders 设置基础安全响应头（防 MIME 嗅探 / 点击劫持 / Referrer 泄漏）。
// prod 下追加 HSTS——仅在经 HTTPS 暴露时生效，HTTP 下浏览器忽略，故无副作用。
// 前端静态资源的安全头由 nginx 负责（web/nginx.conf），此处覆盖后端 API 响应（纵深防御）。
func SecurityHeaders(prod bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		if prod {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// RateLimit 按 IP 的令牌桶限流（内存版；多实例/生产建议换 redis）。
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	var mu sync.Mutex
	limiters := make(map[string]*rate.Limiter)
	get := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		l, ok := limiters[ip]
		if !ok {
			l = rate.NewLimiter(rate.Limit(rps), burst)
			limiters[ip] = l
		}
		return l
	}
	return func(c *gin.Context) {
		if !get(c.ClientIP()).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limited"})
			return
		}
		c.Next()
	}
}

// RequireAuth 解析 Bearer access token，写入 uid/role。
func RequireAuth(jm *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(auth, "Bearer ")
		if !ok || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		claims, err := jm.Parse(token)
		if err != nil || claims.Type != jwt.TypeAccess {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxRole, claims.Role)
		c.Set(CtxOrgID, claims.OrgID)
		// 不变量（DB CHECK users_org_or_super）：org_id 为空 ⟺ 平台超管。
		c.Set(CtxIsSuperadmin, claims.OrgID == nil)
		c.Next()
	}
}

// RequireRole 按角色拦截（需在 RequireAuth 之后）。
func RequireRole(roles ...domain.Role) gin.HandlerFunc {
	allowed := make(map[domain.Role]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		role, _ := c.Get(CtxRole)
		rs, _ := role.(string)
		if !allowed[domain.Role(rs)] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

// RequireSuperadmin 仅放行平台超管（需在 RequireAuth 之后），供 /platform/* 端点。
func RequireSuperadmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsSuperadmin(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

// OrgID 取当前请求的组织 id（超管为 nil）。handler 传给 store 做 org 过滤。
func OrgID(c *gin.Context) *int64 {
	v, _ := c.Get(CtxOrgID)
	id, _ := v.(*int64)
	return id
}

// IsSuperadmin 当前请求是否平台超管。
func IsSuperadmin(c *gin.Context) bool {
	v, _ := c.Get(CtxIsSuperadmin)
	ok, _ := v.(bool)
	return ok
}
