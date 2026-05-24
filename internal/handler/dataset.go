package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/platform/pgrestore"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/service"
)

type DatasetHandler struct {
	store      *store.Store
	srcAdmin   *pgxpool.Pool // postgres@source-db：建 schema / 反射 / 授权 / 读取
	restorer   *pgrestore.Restorer
	uploadDir  string
	maxBytes   int64
	readerRole string
}

func NewDatasetHandler(s *store.Store, srcAdmin *pgxpool.Pool, r *pgrestore.Restorer,
	uploadDir string, maxBytes int64, readerRole string) *DatasetHandler {
	return &DatasetHandler{store: s, srcAdmin: srcAdmin, restorer: r,
		uploadDir: uploadDir, maxBytes: maxBytes, readerRole: readerRole}
}

// Upload 接收 dump → 沙箱恢复 → 反射 form_schema → 生成任务 → READY（同步，适合中小 dump）。
func (h *DatasetHandler) Upload(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name 必填"})
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file 必填"})
		return
	}
	if fh.Size > h.maxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "文件超过大小上限"})
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	custom := ext == ".backup" || ext == ".dump"
	if ext != ".sql" && !custom {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 .sql / .backup / .dump"})
		return
	}

	ctx := c.Request.Context()
	dsID, err := h.store.CreateDataset(ctx, &domain.Dataset{
		Name: name, SourceSchema: "", SourceTable: "", SourcePKColumn: "",
		FormSchema: json.RawMessage("{}"), FormSchemaVersion: 1, Status: domain.StatusImporting,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建数据集失败"})
		return
	}
	batchID, err := h.store.CreateImportBatch(ctx, dsID, fh.Filename, fh.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建批次失败"})
		return
	}
	schema := fmt.Sprintf("ds_%d_v%d", dsID, batchID)

	if err := os.MkdirAll(h.uploadDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建上传目录失败"})
		return
	}
	dst := filepath.Join(h.uploadDir, fmt.Sprintf("%d_%d%s", dsID, batchID, ext))
	if err := c.SaveUploadedFile(fh, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	defer os.Remove(dst) // dump 不留盘（PRD §20.7）

	if err := h.runImport(ctx, dsID, batchID, schema, dst, custom); err != nil {
		slog.Error("导入失败", "dataset", dsID, "error", err)
		_ = h.store.SetDatasetStatus(ctx, dsID, domain.StatusFailed)
		_ = h.store.FinishImportBatch(ctx, batchID, 0, 0, err.Error())
		_ = service.DropSchema(ctx, h.srcAdmin, schema)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导入失败: " + err.Error()})
		return
	}
	h.respondDetail(c, dsID)
}

func (h *DatasetHandler) runImport(ctx context.Context, dsID, batchID int64, schema, dumpPath string, custom bool) error {
	if err := service.CreateSchema(ctx, h.srcAdmin, schema); err != nil {
		return fmt.Errorf("建 schema: %w", err)
	}
	if err := h.restorer.Restore(ctx, schema, dumpPath, custom); err != nil {
		return err
	}
	table, err := service.DetectTable(ctx, h.srcAdmin, schema)
	if err != nil {
		return err
	}
	ref, err := service.Reflect(ctx, h.srcAdmin, schema, table)
	if err != nil {
		return err
	}
	if err := service.GrantReader(ctx, h.srcAdmin, schema, h.readerRole); err != nil {
		return fmt.Errorf("授权 reader: %w", err)
	}

	// v2：导入只反射 + 置 READY；任务在 admin 配置完「补全列」后由 GenerateTasks 生成（PRD §24.7）。
	if err := h.store.UpdateDatasetReflected(ctx, dsID, schema, table, ref.PKColumn, ref.HashColumns, ref.FormSchema, ref.TotalRows); err != nil {
		return err
	}
	return h.store.FinishImportBatch(ctx, batchID, 0, 0, "")
}

// GenerateTasks 按当前 form_schema 的 fill 列，把「有空 fill 列」的源行物化为任务（PRD §24.7）。
func (h *DatasetHandler) GenerateTasks(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	ctx := c.Request.Context()
	ds, err := h.store.GetDataset(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据集不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	fs, err := domain.ParseFormSchema(ds.FormSchema)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "form_schema 解析失败"})
		return
	}
	fillCols := fs.FillColumns()
	if len(fillCols) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先在「列与字段」配置至少一个补全列"})
		return
	}

	rows, err := service.FetchTaskRows(ctx, h.srcAdmin, ds.SourceSchema, ds.SourceTable, ds.SourcePKColumn, ds.HashColumns, fillCols)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "扫描待补全行失败: " + err.Error()})
		return
	}
	batchID, err := h.store.CreateImportBatch(ctx, id, "(generate-tasks)", 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建批次失败"})
		return
	}
	pks := make([]string, len(rows))
	hs := make([]string, len(rows))
	for i, r := range rows {
		pks[i], hs[i] = r.PK, r.Hash
	}
	n, err := h.store.InsertTasksWithHash(ctx, id, batchID, pks, hs)
	if err != nil {
		_ = h.store.FinishImportBatch(ctx, batchID, 0, 0, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成任务失败"})
		return
	}
	_ = h.store.FinishImportBatch(ctx, batchID, int(n), 0, "")
	h.respondDetail(c, id)
}

// Sync 对已有数据集重传新版本 dump：仅新增 + content_hash 差异重标（PRD §12）。
func (h *DatasetHandler) Sync(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	ctx := c.Request.Context()
	ds, err := h.store.GetDataset(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据集不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file 必填"})
		return
	}
	if fh.Size > h.maxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "文件超过大小上限"})
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	custom := ext == ".backup" || ext == ".dump"
	if ext != ".sql" && !custom {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 .sql / .backup / .dump"})
		return
	}

	batchID, err := h.store.CreateImportBatch(ctx, id, fh.Filename, fh.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建批次失败"})
		return
	}
	newSchema := fmt.Sprintf("ds_%d_v%d", id, batchID)

	if err := os.MkdirAll(h.uploadDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建上传目录失败"})
		return
	}
	dst := filepath.Join(h.uploadDir, fmt.Sprintf("%d_%d%s", id, batchID, ext))
	if err := c.SaveUploadedFile(fh, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	defer os.Remove(dst)

	if err := h.runSync(ctx, ds, batchID, newSchema, dst, custom); err != nil {
		slog.Error("同步失败", "dataset", id, "error", err)
		_ = h.store.FinishImportBatch(ctx, batchID, 0, 0, err.Error())
		_ = service.DropSchema(ctx, h.srcAdmin, newSchema)
		// 同步失败不动旧数据：数据集保持原 READY + 旧 source_schema
		c.JSON(http.StatusInternalServerError, gin.H{"error": "同步失败: " + err.Error()})
		return
	}
	h.respondDetail(c, id)
}

func (h *DatasetHandler) runSync(ctx context.Context, ds *domain.Dataset, batchID int64, newSchema, dumpPath string, custom bool) error {
	if err := service.CreateSchema(ctx, h.srcAdmin, newSchema); err != nil {
		return fmt.Errorf("建 schema: %w", err)
	}
	if err := h.restorer.Restore(ctx, newSchema, dumpPath, custom); err != nil {
		return err
	}
	// 用数据集存储的 hash_columns/pk/table，保证哈希与上一版可比（PRD §12.2）
	rows, err := service.FetchHashedRows(ctx, h.srcAdmin, newSchema, ds.SourceTable, ds.SourcePKColumn, ds.HashColumns)
	if err != nil {
		return fmt.Errorf("计算 content_hash: %w", err)
	}
	if err := service.GrantReader(ctx, h.srcAdmin, newSchema, h.readerRole); err != nil {
		return fmt.Errorf("授权 reader: %w", err)
	}
	pks := make([]string, len(rows))
	hs := make([]string, len(rows))
	for i, r := range rows {
		pks[i], hs[i] = r.PK, r.Hash
	}
	newN, updN, err := h.store.SyncTasks(ctx, ds.ID, batchID, pks, hs)
	if err != nil {
		return fmt.Errorf("同步任务: %w", err)
	}
	if err := h.store.FinishImportBatch(ctx, batchID, newN, updN, ""); err != nil {
		return err
	}
	old := ds.SourceSchema
	if err := h.store.UpdateDatasetSourceSchema(ctx, ds.ID, newSchema, len(rows)); err != nil {
		return err
	}
	if old != "" && old != newSchema {
		_ = service.DropSchema(ctx, h.srcAdmin, old) // 切换后清旧版本，失败不致命
	}
	return nil
}

// Dashboard 全局看板聚合（admin）。
func (h *DatasetHandler) Dashboard(c *gin.Context) {
	d, err := h.store.GetDashboard(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询看板失败"})
		return
	}
	c.JSON(http.StatusOK, d)
}

func (h *DatasetHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	h.respondDetail(c, id)
}

func (h *DatasetHandler) respondDetail(c *gin.Context, id int64) {
	ctx := c.Request.Context()
	ds, err := h.store.GetDataset(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据集不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	pending, claimed, completed, _ := h.store.GetDatasetProgress(ctx, id)
	batches, _ := h.store.ListBatches(ctx, id)
	if batches == nil {
		batches = []domain.ImportBatch{}
	}
	c.JSON(http.StatusOK, gin.H{
		"dataset":  ds,
		"progress": gin.H{"pending": pending, "claimed": claimed, "completed": completed},
		"batches":  batches,
	})
}

// UpdateFormSchema 整体替换 form_schema（版本 +1）。body 为 form_schema 的 JSON。
func (h *DatasetHandler) UpdateFormSchema(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil || !json.Valid(body) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "form_schema 不是合法 JSON"})
		return
	}
	v, err := h.store.UpdateFormSchema(c.Request.Context(), id, json.RawMessage(body))
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据集不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"form_schema_version": v})
}
