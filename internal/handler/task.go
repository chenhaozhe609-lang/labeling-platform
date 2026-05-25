package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/middleware"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/source"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/service"
)

type TaskHandler struct {
	store     *store.Store
	source    *source.Reader
	prefiller service.Prefiller
	leaseMin  int
}

func NewTaskHandler(s *store.Store, src *source.Reader, pf service.Prefiller, leaseMin int) *TaskHandler {
	return &TaskHandler{store: s, source: src, prefiller: pf, leaseMin: leaseMin}
}

func userID(c *gin.Context) int64 {
	v, _ := c.Get(middleware.CtxUserID)
	id, _ := v.(int64)
	return id
}

// bundle 组装 task + source_row + form_schema（GET/claim 复用）。
func (h *TaskHandler) bundle(ctx context.Context, t *domain.Task) (gin.H, error) {
	ds, err := h.store.GetDataset(ctx, t.DatasetID)
	if err != nil {
		return nil, err
	}
	srcRow := map[string]any{}
	if row, err := h.source.GetRow(ctx, ds.SourceSchema, ds.SourceTable, ds.SourcePKColumn, t.SourceRowPK); err != nil {
		slog.Warn("读取源行失败", "task", t.ID, "error", err) // 源数据缺失不阻断标注
	} else if row != nil {
		srcRow = row
	}

	out := gin.H{"task": t, "source_row": srcRow, "form_schema": json.RawMessage(ds.FormSchema)}

	// LLM 预填：context 列 → 预测 fill 列（PRD §24.5）。失败不阻断。
	if fs, err := domain.ParseFormSchema(ds.FormSchema); err == nil && h.prefiller != nil {
		if fills, err := h.prefiller.Prefill(ctx, fs, srcRow); err == nil && len(fills) > 0 {
			out["ai_suggestion"] = gin.H{"fills": fills, "_source": "ai"}
		}
	}
	return out, nil
}

// Claim 抢一个 PENDING 任务，直接返回完整 bundle；池空返回 {"task": null}。
func (h *TaskHandler) Claim(c *gin.Context) {
	var req struct {
		DatasetID int64 `json:"dataset_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dataset_id 必填"})
		return
	}
	t, err := h.store.ClaimTask(c.Request.Context(), req.DatasetID, userID(c), h.leaseMin)
	if errors.Is(err, store.ErrNoTask) {
		// 区分「暂停」与「真没任务」：暂停时给前端明确标志。
		if ds, e := h.store.GetDataset(c.Request.Context(), req.DatasetID); e == nil && ds.Status == domain.StatusPaused {
			c.JSON(http.StatusOK, gin.H{"task": nil, "paused": true})
			return
		}
		c.JSON(http.StatusOK, gin.H{"task": nil})
		return
	}
	if err != nil {
		slog.Error("claim 失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "claim 失败"})
		return
	}
	b, err := h.bundle(c.Request.Context(), t)
	if err != nil {
		slog.Error("组装任务失败", "task", t.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "组装任务失败"})
		return
	}
	c.JSON(http.StatusOK, b)
}

func (h *TaskHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	t, err := h.store.GetTask(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	b, err := h.bundle(c.Request.Context(), t)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "组装任务失败"})
		return
	}
	c.JSON(http.StatusOK, b)
}

func (h *TaskHandler) Heartbeat(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	lease, err := h.store.Heartbeat(c.Request.Context(), id, userID(c), h.leaseMin)
	if errors.Is(err, store.ErrConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": "任务已超时或被回收"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "续约失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"lease_expires_at": lease})
}

func (h *TaskHandler) Submit(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Data              json.RawMessage `json:"data" binding:"required"`
		FormSchemaVersion int             `json:"form_schema_version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data 必填"})
		return
	}
	if msg := h.validateFills(c.Request.Context(), id, req.Data); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	err := h.store.SubmitAnnotation(c.Request.Context(), id, userID(c), req.Data, req.FormSchemaVersion)
	if errors.Is(err, store.ErrConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": "任务已超时或被回收，请重新领取"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提交失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// validateFills 校验提交的 data.fills 覆盖了所有 fill 列且非空；返回错误文案（""=通过）。
func (h *TaskHandler) validateFills(ctx context.Context, taskID int64, data json.RawMessage) string {
	var payload struct {
		Fills map[string]any `json:"fills"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "data.fills 格式错误"
	}
	t, err := h.store.GetTask(ctx, taskID)
	if err != nil {
		return "" // 任务查不到：交给后续幂等校验处理
	}
	ds, err := h.store.GetDataset(ctx, t.DatasetID)
	if err != nil {
		return ""
	}
	fs, err := domain.ParseFormSchema(ds.FormSchema)
	if err != nil {
		return ""
	}
	for _, code := range fs.FillColumns() {
		if isEmptyVal(payload.Fills[code]) {
			return "请补全所有「补全列」后再提交"
		}
	}
	return ""
}

func isEmptyVal(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	default:
		return false
	}
}

func (h *TaskHandler) Release(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	err := h.store.ReleaseTask(c.Request.Context(), id, userID(c))
	if err != nil && !errors.Is(err, store.ErrConflict) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "释放失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true}) // 幂等：即便已非 CLAIMED 也返回成功
}

func (h *TaskHandler) ListDatasets(c *gin.Context) {
	items, err := h.store.ListDatasets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	if items == nil {
		items = []domain.DatasetListItem{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
