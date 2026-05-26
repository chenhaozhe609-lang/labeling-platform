package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/source"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

const (
	reviewQueueDefault = 20
	reviewQueueMax     = 50
)

type ReviewHandler struct {
	store  *store.Store
	source *source.Reader
}

func NewReviewHandler(s *store.Store, src *source.Reader) *ReviewHandler {
	return &ReviewHandler{store: s, source: src}
}

// Queue 随机抽检队列（C5.1）：某数据集下待审标注 + 各自源行 + 数据集 form_schema。
// 源行一次性批量取回，前端 J/K 切换无需再往返。
func (h *ReviewHandler) Queue(c *gin.Context) {
	datasetID, err := strconv.ParseInt(c.Query("dataset_id"), 10, 64)
	if err != nil || datasetID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dataset_id 必填"})
		return
	}
	limit := reviewQueueDefault
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = min(v, reviewQueueMax)
	}

	ctx := c.Request.Context()
	ds, err := h.store.GetDataset(ctx, datasetID)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据集不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	items, err := h.store.ReviewQueue(ctx, datasetID, userID(c), limit)
	if err != nil {
		slog.Error("审核队列查询失败", "dataset", datasetID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询审核队列失败"})
		return
	}
	total, _ := h.store.CountReviewPending(ctx, datasetID, userID(c))

	pks := make([]string, 0, len(items))
	for _, it := range items {
		pks = append(pks, it.SourceRowPK)
	}
	srcRows, err := h.source.GetRows(ctx, ds.SourceSchema, ds.SourceTable, ds.SourcePKColumn, pks)
	if err != nil {
		slog.Warn("批量读取源行失败", "dataset", datasetID, "error", err) // 源缺失不阻断审核
		srcRows = map[string]map[string]any{}
	}

	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		row := srcRows[it.SourceRowPK]
		if row == nil {
			row = map[string]any{}
		}
		item := gin.H{
			"annotation_id": it.AnnotationID,
			"task_id":       it.TaskID,
			"source_row_pk": it.SourceRowPK,
			"round":         it.Round,
			"annotator":     it.Annotator,
			"created_at":    it.CreatedAt,
			"data":          it.Data,
			"source_row":    row,
		}
		if len(it.PrevData) > 0 { // 旧↔新对比：上一版（已废弃）标注（B4.2）
			item["previous"] = json.RawMessage(it.PrevData)
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{
		"dataset_name":  ds.Name,
		"form_schema":   json.RawMessage(ds.FormSchema),
		"pending_total": total,
		"items":         out,
	})
}

// Decision 裁决一条标注（C5.2）：approved / needs_redo。
func (h *ReviewHandler) Decision(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status 必填"})
		return
	}
	if req.Status != "approved" && req.Status != "needs_redo" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status 取值非法（approved|needs_redo）"})
		return
	}

	err = h.store.SubmitReview(c.Request.Context(), id, userID(c), req.Status, req.Note)
	if errors.Is(err, store.ErrForbidden) {
		c.JSON(http.StatusForbidden, gin.H{"error": "不能审核本人提交的标注"})
		return
	}
	if errors.Is(err, store.ErrConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": "该标注已被审核或已失效"})
		return
	}
	if err != nil {
		slog.Error("裁决失败", "annotation", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "裁决失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Edit 审核改写并通过（B4.4）：reviewer 微调 fills 后直接通过。
func (h *ReviewHandler) Edit(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	var req struct {
		Data json.RawMessage `json:"data" binding:"required"`
		Note string          `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data 必填"})
		return
	}
	var payload struct {
		Fills map[string]any `json:"fills"`
	}
	if json.Unmarshal(req.Data, &payload) != nil || len(payload.Fills) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data.fills 不能为空"})
		return
	}

	err = h.store.EditReview(c.Request.Context(), id, userID(c), req.Data, req.Note)
	if errors.Is(err, store.ErrForbidden) {
		c.JSON(http.StatusForbidden, gin.H{"error": "不能改写本人提交的标注"})
		return
	}
	if errors.Is(err, store.ErrConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": "该标注已被审核或已失效"})
		return
	}
	if err != nil {
		slog.Error("改写并通过失败", "annotation", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
