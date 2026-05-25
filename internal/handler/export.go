package handler

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/source"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

// exportChunk 每攒够这么多行就批量取一次源行并 flush，约束内存（支撑百万行流式）。
const exportChunk = 500

type ExportHandler struct {
	store  *store.Store
	source *source.Reader
}

func NewExportHandler(s *store.Store, src *source.Reader) *ExportHandler {
	return &ExportHandler{store: s, source: src}
}

// Export 流式导出「补全后的表」= 源行 + 标注 fills 叠加（C5.3）。
// 查询参数：format=jsonl|csv（默认 jsonl）、only_approved=true（仅导审核通过的）。
// 仅含 COMPLETED + 有效标注的行，隐藏列不导出。
func (h *ExportHandler) Export(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	format := c.DefaultQuery("format", "jsonl")
	if format != "jsonl" && format != "csv" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format 仅支持 jsonl|csv"})
		return
	}
	onlyApproved := c.Query("only_approved") == "true"

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

	// 导出列 = 非 hidden 的 schema 列（id + context + fill），按 schema 顺序。
	var outCols []domain.ColumnSpec
	fillSet := map[string]bool{}
	for _, col := range fs.Columns {
		if col.Role == domain.ColHidden {
			continue
		}
		outCols = append(outCols, col)
		if col.Role == domain.ColFill {
			fillSet[col.Code] = true
		}
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="dataset_%d_export.%s"`, id, format))
	if format == "csv" {
		h.streamCSV(c, ds, outCols, fillSet, onlyApproved)
	} else {
		h.streamJSONL(c, ds, outCols, fillSet, onlyApproved)
	}
}

func (h *ExportHandler) streamJSONL(c *gin.Context, ds *domain.Dataset, outCols []domain.ColumnSpec, fillSet map[string]bool, onlyApproved bool) {
	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Status(http.StatusOK)
	w := c.Writer
	enc := json.NewEncoder(w)

	buf := make([]*store.ExportRow, 0, exportChunk)
	flush := func() {
		if len(buf) == 0 {
			return
		}
		src := h.batchSource(c, ds, buf)
		for _, r := range buf {
			rec := mergeRow(outCols, fillSet, src[r.PK], decodeFills(r.Fills))
			rec["_pk"] = r.PK
			rec["_round"] = r.Round
			rec["_form_schema_version"] = r.Version
			rec["_annotator"] = r.Annotator
			if r.ReviewStatus != nil {
				rec["_review_status"] = *r.ReviewStatus
			}
			if r.Source != nil {
				rec["_source"] = *r.Source
			}
			_ = enc.Encode(rec) // Encode 自带换行
		}
		w.Flush()
		buf = buf[:0]
	}

	err := h.store.StreamExport(c.Request.Context(), ds.ID, onlyApproved, func(r *store.ExportRow) error {
		buf = append(buf, r)
		if len(buf) >= exportChunk {
			flush()
		}
		return nil
	})
	flush()
	if err != nil {
		slog.Error("导出遍历失败", "dataset", ds.ID, "error", err) // 已开始流式输出，无法改状态码
		_ = enc.Encode(map[string]any{"_error": "导出中断: " + err.Error()})
	}
}

func (h *ExportHandler) streamCSV(c *gin.Context, ds *domain.Dataset, outCols []domain.ColumnSpec, fillSet map[string]bool, onlyApproved bool) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Status(http.StatusOK)
	w := c.Writer
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM，让 Excel 正确识别中文
	cw := csv.NewWriter(w)

	header := make([]string, 0, len(outCols)+5)
	header = append(header, "_pk")
	for _, col := range outCols {
		header = append(header, col.Code)
	}
	header = append(header, "_round", "_form_schema_version", "_review_status", "_annotator")
	_ = cw.Write(header)

	buf := make([]*store.ExportRow, 0, exportChunk)
	flush := func() {
		if len(buf) == 0 {
			return
		}
		src := h.batchSource(c, ds, buf)
		for _, r := range buf {
			merged := mergeRow(outCols, fillSet, src[r.PK], decodeFills(r.Fills))
			rec := make([]string, 0, len(header))
			rec = append(rec, r.PK)
			for _, col := range outCols {
				rec = append(rec, csvVal(merged[col.Code]))
			}
			rec = append(rec, strconv.Itoa(r.Round), strconv.Itoa(r.Version), strDeref(r.ReviewStatus), r.Annotator)
			_ = cw.Write(rec)
		}
		cw.Flush()
		buf = buf[:0]
	}

	err := h.store.StreamExport(c.Request.Context(), ds.ID, onlyApproved, func(r *store.ExportRow) error {
		buf = append(buf, r)
		if len(buf) >= exportChunk {
			flush()
		}
		return nil
	})
	flush()
	if err != nil {
		slog.Error("导出遍历失败", "dataset", ds.ID, "error", err)
	}
}

// batchSource 一次取回一批行对应的源行（按 pk 文本键）。失败降级为空（导出仍含 fills）。
func (h *ExportHandler) batchSource(c *gin.Context, ds *domain.Dataset, buf []*store.ExportRow) map[string]map[string]any {
	pks := make([]string, 0, len(buf))
	for _, r := range buf {
		pks = append(pks, r.PK)
	}
	src, err := h.source.GetRows(c.Request.Context(), ds.SourceSchema, ds.SourceTable, ds.SourcePKColumn, pks)
	if err != nil {
		slog.Warn("导出批量取源行失败", "dataset", ds.ID, "error", err)
		return map[string]map[string]any{}
	}
	return src
}

// mergeRow 把源行与标注 fills 叠加成「补全后」的一行（按导出列投影；fill 列取标注值，其余取源值）。
func mergeRow(outCols []domain.ColumnSpec, fillSet map[string]bool, src, fills map[string]any) map[string]any {
	row := make(map[string]any, len(outCols))
	for _, col := range outCols {
		if fillSet[col.Code] {
			row[col.Code] = fills[col.Code]
		} else {
			row[col.Code] = src[col.Code]
		}
	}
	return row
}

func decodeFills(raw json.RawMessage) map[string]any {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}

// csvVal 把任意值序列化为 CSV 单元格（多选/对象 → JSON 字符串）。
func csvVal(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
