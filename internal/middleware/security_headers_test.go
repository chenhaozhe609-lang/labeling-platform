package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func runWithHeaders(prod bool) http.Header {
	r := gin.New()
	r.Use(SecurityHeaders(prod))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	return w.Header()
}

func TestSecurityHeaders_AlwaysSet(t *testing.T) {
	h := runWithHeaders(false)
	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	}
	for k, v := range want {
		if got := h.Get(k); got != v {
			t.Errorf("%s=%q，期望 %q", k, got, v)
		}
	}
	if h.Get("Strict-Transport-Security") != "" {
		t.Errorf("非 prod 不应设置 HSTS，got %q", h.Get("Strict-Transport-Security"))
	}
}

func TestSecurityHeaders_HSTSInProd(t *testing.T) {
	h := runWithHeaders(true)
	if h.Get("Strict-Transport-Security") == "" {
		t.Error("prod 应设置 HSTS")
	}
}
