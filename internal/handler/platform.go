package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

// PlatformHandler 平台超管端点（RequireSuperadmin）：跨组织运营/排障。
type PlatformHandler struct {
	store *store.Store
}

func NewPlatformHandler(s *store.Store) *PlatformHandler {
	return &PlatformHandler{store: s}
}

// ListOrgs 全部组织。GET /platform/orgs。
func (h *PlatformHandler) ListOrgs(c *gin.Context) {
	orgs, err := h.store.ListOrgs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询组织失败"})
		return
	}
	if orgs == nil {
		orgs = []domain.Organization{}
	}
	c.JSON(http.StatusOK, gin.H{"items": orgs})
}
