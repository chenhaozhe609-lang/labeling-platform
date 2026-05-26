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
	uid := userID(c)
	dsID, err := h.store.CreateDataset(ctx, &domain.Dataset{
		OrgID: ctxOrg(c), // 归调用者所在组织
		Name:  name, SourceSchema: "", SourceTable: "", SourcePKColumn: "",
		FormSchema: json.RawMessage("{}"), FormSchemaVersion: 1, Status: domain.StatusImporting,
		CreatedBy: &uid,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建数据集失败"})
		return
	}
	batchID, err := h.store.CreateImportBatch(ctx, dsID, fh.Filename, fh.Size, &uid)
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

// restoreIntoSchema 以受限角色恢复 dump（dump 自建其 schema），随后快照对比发现该 schema，
// 改名为隔离命名 schema（C6.1）。成功返回后 schema 已存在且为本次 dump 的唯一产物。
func (h *DatasetHandler) restoreIntoSchema(ctx context.Context, schema, dumpPath string, custom bool) error {
	before, err := service.ListSchemas(ctx, h.srcAdmin)
	if err != nil {
		return fmt.Errorf("快照 schema: %w", err)
	}
	if err := h.restorer.Restore(ctx, dumpPath, custom); err != nil {
		// 失败恢复可能已建出 schema（如恶意语句中途被拒）——清理掉，避免泄漏。
		if after, e := service.ListSchemas(ctx, h.srcAdmin); e == nil {
			for _, s := range service.NewSchemas(before, after) {
				_ = service.DropSchema(ctx, h.srcAdmin, s)
			}
		}
		return err
	}
	after, err := service.ListSchemas(ctx, h.srcAdmin)
	if err != nil {
		return fmt.Errorf("快照 schema: %w", err)
	}
	created := service.NewSchemas(before, after)
	if len(created) != 1 {
		for _, s := range created {
			_ = service.DropSchema(ctx, h.srcAdmin, s) // 清理本次恢复产物
		}
		if len(created) == 0 {
			return fmt.Errorf("dump 未创建独立 schema（请用 `pg_dump -n <schema>` 导出带 schema 的备份，暂不支持仅 public 的 dump）")
		}
		return fmt.Errorf("dump 含 %d 个 schema，暂仅支持单 schema 备份", len(created))
	}
	if err := service.RenameSchema(ctx, h.srcAdmin, created[0], schema); err != nil {
		_ = service.DropSchema(ctx, h.srcAdmin, created[0])
		return fmt.Errorf("规范化隔离 schema: %w", err)
	}
	return nil
}

func (h *DatasetHandler) runImport(ctx context.Context, dsID, batchID int64, schema, dumpPath string, custom bool) error {
	if err := h.restoreIntoSchema(ctx, schema, dumpPath, custom); err != nil {
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
	ds, err := h.store.GetDataset(ctx, id, ctxOrg(c))
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
	uid := userID(c)
	batchID, err := h.store.CreateImportBatch(ctx, id, "(generate-tasks)", 0, &uid)
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
	ds, err := h.store.GetDataset(ctx, id, ctxOrg(c))
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

	uid := userID(c)
	batchID, err := h.store.CreateImportBatch(ctx, id, fh.Filename, fh.Size, &uid)
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
	if err := h.restoreIntoSchema(ctx, newSchema, dumpPath, custom); err != nil {
		return err
	}
	// v2：同步只针对「任务行」（有空 fill 列）——与 generate-tasks 规则一致，
	// 避免把 fill 已填满、本不该进池的行也当成任务（PRD §24 任务规则）。
	fs, err := domain.ParseFormSchema(ds.FormSchema)
	if err != nil {
		return fmt.Errorf("form_schema 解析: %w", err)
	}
	fillCols := fs.FillColumns()
	if len(fillCols) == 0 {
		return fmt.Errorf("数据集尚未配置补全列，无法同步")
	}
	// 用数据集存储的 hash_columns/pk/table，保证哈希与上一版可比（PRD §12.2）
	rows, err := service.FetchTaskRows(ctx, h.srcAdmin, newSchema, ds.SourceTable, ds.SourcePKColumn, ds.HashColumns, fillCols)
	if err != nil {
		return fmt.Errorf("扫描待补全行: %w", err)
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

// Pause 暂停数据集（admin，C5.5）：READY → PAUSED，暂停后不再放任务。
func (h *DatasetHandler) Pause(c *gin.Context) {
	h.toggleStatus(c, true)
}

// Resume 恢复数据集（admin，C5.5）：PAUSED → READY。
func (h *DatasetHandler) Resume(c *gin.Context) {
	h.toggleStatus(c, false)
}

func (h *DatasetHandler) toggleStatus(c *gin.Context, pause bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	ctx := c.Request.Context()
	// 越权护栏：先确认数据集属于本组织，避免跨组织暂停/恢复。
	if _, err = h.store.GetDataset(ctx, id, ctxOrg(c)); errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据集不存在"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	if pause {
		err = h.store.PauseDataset(ctx, id)
	} else {
		err = h.store.ResumeDataset(ctx, id)
	}
	if errors.Is(err, store.ErrConflict) {
		msg := "仅 PAUSED 数据集可恢复"
		if pause {
			msg = "仅 READY 数据集可暂停"
		}
		c.JSON(http.StatusConflict, gin.H{"error": msg})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}
	h.respondDetail(c, id)
}

// Dashboard 本组织看板聚合（admin）；超管跨组织。
func (h *DatasetHandler) Dashboard(c *gin.Context) {
	d, err := h.store.GetDashboard(c.Request.Context(), ctxOrg(c))
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
	ds, err := h.store.GetDataset(ctx, id, ctxOrg(c))
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
	ctx := c.Request.Context()
	// 越权护栏：先确认数据集属于本组织。
	if _, err = h.store.GetDataset(ctx, id, ctxOrg(c)); errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据集不存在"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	v, err := h.store.UpdateFormSchema(ctx, id, json.RawMessage(body))
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
