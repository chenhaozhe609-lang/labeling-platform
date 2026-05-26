package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ColumnRole 列角色（PRD §24）。
type ColumnRole string

const (
	ColContext ColumnRole = "context" // 只读背景；兼作 LLM 预填输入
	ColFill    ColumnRole = "fill"    // 待补全的标注目标
	ColHidden  ColumnRole = "hidden"  // 不展示
	ColID      ColumnRole = "id"      // 主键
)

// FieldOption single/multi 选项。
type FieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Key   string `json:"key,omitempty"` // 快捷键
}

// FieldConfig fill 列的输入控件配置。
type FieldConfig struct {
	Kind        string        `json:"kind"` // text|single|multi|number|bool|date
	Options     []FieldOption `json:"options,omitempty"`
	Regex       string        `json:"regex,omitempty"`
	Placeholder string        `json:"placeholder,omitempty"`
	Hint        string        `json:"hint,omitempty"`
	Min         *float64      `json:"min,omitempty"`
	Max         *float64      `json:"max,omitempty"`
	Step        *float64      `json:"step,omitempty"`
}

// ColumnSpec 每个源列的角色 + 控件配置。
type ColumnSpec struct {
	Code  string       `json:"code"`
	Type  string       `json:"type"`
	Role  ColumnRole   `json:"role"`
	Label string       `json:"label,omitempty"`
	PK    bool         `json:"pk,omitempty"`
	Field *FieldConfig `json:"field,omitempty"`
}

// FormSchema v2（PRD §24）：每列带角色 + 控件配置。
type FormSchema struct {
	Version     int          `json:"version"`
	PrimaryCols []string     `json:"primary_cols"`
	Columns     []ColumnSpec `json:"columns"`
}

// FillColumns 返回所有 fill 列的列名。
func (fs *FormSchema) FillColumns() []string {
	var out []string
	for _, c := range fs.Columns {
		if c.Role == ColFill {
			out = append(out, c.Code)
		}
	}
	return out
}

// ContextColumns 返回所有 context 列的列名（LLM 预填输入）。
func (fs *FormSchema) ContextColumns() []string {
	var out []string
	for _, c := range fs.Columns {
		if c.Role == ColContext {
			out = append(out, c.Code)
		}
	}
	return out
}

// FillSpecs 返回 fill 列的完整规格（含控件）。
func (fs *FormSchema) FillSpecs() []ColumnSpec {
	var out []ColumnSpec
	for _, c := range fs.Columns {
		if c.Role == ColFill {
			out = append(out, c)
		}
	}
	return out
}

// ParseFormSchema 从 JSONB 解析。
func ParseFormSchema(raw json.RawMessage) (*FormSchema, error) {
	var fs FormSchema
	if err := json.Unmarshal(raw, &fs); err != nil {
		return nil, err
	}
	return &fs, nil
}

func fieldKind(c ColumnSpec) string {
	if c.Field != nil && c.Field.Kind != "" {
		return c.Field.Kind
	}
	return "text"
}

func roleLabel(r ColumnRole) string {
	switch r {
	case ColContext:
		return "上下文"
	case ColHidden:
		return "隐藏"
	case ColID:
		return "主键"
	default:
		return string(r)
	}
}

func removedOptions(old, neu *FieldConfig) []string {
	if old == nil {
		return nil
	}
	have := map[string]bool{}
	if neu != nil {
		for _, o := range neu.Options {
			have[o.Value] = true
		}
	}
	var removed []string
	for _, o := range old.Options {
		if !have[o.Value] {
			label := o.Label
			if label == "" {
				label = o.Value
			}
			removed = append(removed, label)
		}
	}
	return removed
}

// DestructiveChanges 比对新旧 form_schema，返回会让已有标注失配的破坏性变更（人类可读）。
// 只看 fill 列（仅 fill 列被标注）：删除 / 改为非 fill / 改控件类型 / 删除单多选选项。
// 新增列、改 label、加选项、动 context/hidden 列等均非破坏性，不在此列。
func DestructiveChanges(old, neu *FormSchema) []string {
	if old == nil || neu == nil {
		return nil
	}
	newByCode := make(map[string]ColumnSpec, len(neu.Columns))
	for _, c := range neu.Columns {
		newByCode[c.Code] = c
	}
	var out []string
	for _, o := range old.Columns {
		if o.Role != ColFill {
			continue
		}
		label := o.Label
		if label == "" {
			label = o.Code
		}
		n, ok := newByCode[o.Code]
		if !ok {
			out = append(out, fmt.Sprintf("删除补全列「%s」", label))
			continue
		}
		if n.Role != ColFill {
			out = append(out, fmt.Sprintf("补全列「%s」改为「%s」（不再标注）", label, roleLabel(n.Role)))
			continue
		}
		oldKind, newKind := fieldKind(o), fieldKind(n)
		if oldKind != newKind {
			out = append(out, fmt.Sprintf("补全列「%s」控件类型 %s → %s", label, oldKind, newKind))
		}
		if newKind == "single" || newKind == "multi" {
			if removed := removedOptions(o.Field, n.Field); len(removed) > 0 {
				out = append(out, fmt.Sprintf("补全列「%s」删除选项：%s", label, strings.Join(removed, "、")))
			}
		}
	}
	return out
}
