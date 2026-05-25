package domain

import "encoding/json"

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
