package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

// LLMConfig 配置 OpenAI 兼容的 Chat Completions 端点（默认 DeepSeek）。
type LLMConfig struct {
	BaseURL string // https://api.deepseek.com（或 .../v1）
	APIKey  string
	Model   string
	Timeout time.Duration
}

// LLMPrefiller 用 context 列拼 prompt 调 LLM 预测 fill 列（PRD §24.5）。
// 仅依赖标准库手写 HTTP（OpenAI 兼容协议），不引第三方 SDK。
type LLMPrefiller struct {
	cfg    LLMConfig
	client *http.Client
}

func NewLLMPrefiller(cfg LLMConfig) *LLMPrefiller {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.Model == "" {
		cfg.Model = "deepseek-chat"
	}
	return &LLMPrefiller{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

const llmMaxContextChars = 2000 // 单个上下文字段送入 prompt 的字符上限，控 token

// Prefill 调 LLM 给 fill 列预测值；返回经 schema 校正后的合法值（非法/缺失项丢弃）。
// 无 fill 列或无源行时返回空（不浪费调用）。任何错误向上抛，由调用方降级（不阻断标注）。
func (p *LLMPrefiller) Prefill(ctx context.Context, fs *domain.FormSchema, sourceRow map[string]any) (map[string]any, error) {
	fills := fs.FillSpecs()
	if len(fills) == 0 || len(sourceRow) == 0 {
		return map[string]any{}, nil
	}

	sys, user := buildPrompt(fs, sourceRow)
	body, err := json.Marshal(chatReq{
		Model:          p.cfg.Model,
		Temperature:    0,
		MaxTokens:      512,
		Stream:         false,
		ResponseFormat: &respFormat{Type: "json_object"},
		Messages: []chatMsg{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var cr chatResp
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("解析 LLM 响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := "LLM 调用失败"
		if cr.Error != nil {
			msg = cr.Error.Message
		}
		return nil, fmt.Errorf("%s (HTTP %d)", msg, resp.StatusCode)
	}
	if len(cr.Choices) == 0 {
		return nil, fmt.Errorf("LLM 无返回")
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(cr.Choices[0].Message.Content), &raw); err != nil {
		return nil, fmt.Errorf("LLM 输出非合法 JSON: %w", err)
	}

	out := map[string]any{}
	for _, spec := range fills {
		if v, ok := coerceFill(spec, raw[spec.Code]); ok {
			out[spec.Code] = v
		}
	}
	return out, nil
}

// buildPrompt 拼系统/用户提示：上下文字段值 + 各目标字段的取值约束，要求只返回 JSON。
func buildPrompt(fs *domain.FormSchema, sourceRow map[string]any) (system, user string) {
	system = "你是严谨的数据标注助手。下面给出一条记录的若干「上下文字段」，以及需要你判断的「目标字段」及其取值约束。" +
		"请仅依据上下文，为每个目标字段给出最合适的值，并只返回一个 JSON 对象（键为字段 code，值为字段取值），" +
		"不要输出任何解释、注释或 markdown 代码块。"

	var b strings.Builder
	b.WriteString("上下文字段：\n")
	for _, c := range fs.Columns {
		if c.Role != domain.ColContext {
			continue
		}
		b.WriteString(fmt.Sprintf("- %s（%s）：%s\n", labelOf(c), c.Code, truncate(fmt.Sprint(sourceRow[c.Code]), llmMaxContextChars)))
	}

	b.WriteString("\n目标字段（为每个给出值）：\n")
	for _, c := range fs.FillSpecs() {
		kind := "text"
		if c.Field != nil && c.Field.Kind != "" {
			kind = c.Field.Kind
		}
		switch kind {
		case "single":
			b.WriteString(fmt.Sprintf("- %s：单选，必须从以下取一个并返回其 value：%s\n", fieldHead(c), optionsHint(c)))
		case "multi":
			b.WriteString(fmt.Sprintf("- %s：多选，返回 value 组成的数组（可空）：%s\n", fieldHead(c), optionsHint(c)))
		case "bool":
			b.WriteString(fmt.Sprintf("- %s：布尔，返回 true 或 false\n", fieldHead(c)))
		case "number":
			b.WriteString(fmt.Sprintf("- %s：数字%s，返回数值\n", fieldHead(c), numRangeHint(c)))
		case "date":
			b.WriteString(fmt.Sprintf("- %s：日期，返回 YYYY-MM-DD 字符串\n", fieldHead(c)))
		default:
			b.WriteString(fmt.Sprintf("- %s：文本，返回简短字符串\n", fieldHead(c)))
		}
	}
	b.WriteString("\n只返回 JSON 对象，键用上面每个目标字段的 code。")
	return system, b.String()
}

func labelOf(c domain.ColumnSpec) string {
	if c.Label != "" {
		return c.Label
	}
	return c.Code
}

func fieldHead(c domain.ColumnSpec) string { return fmt.Sprintf("%s（code=%s）", labelOf(c), c.Code) }

func optionsHint(c domain.ColumnSpec) string {
	if c.Field == nil {
		return "[]"
	}
	parts := make([]string, 0, len(c.Field.Options))
	for _, o := range c.Field.Options {
		parts = append(parts, fmt.Sprintf("%q(%s)", o.Value, o.Label))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func numRangeHint(c domain.ColumnSpec) string {
	if c.Field == nil || (c.Field.Min == nil && c.Field.Max == nil) {
		return ""
	}
	lo, hi := "", ""
	if c.Field.Min != nil {
		lo = strconv.FormatFloat(*c.Field.Min, 'f', -1, 64)
	}
	if c.Field.Max != nil {
		hi = strconv.FormatFloat(*c.Field.Max, 'f', -1, 64)
	}
	return fmt.Sprintf("（范围 %s~%s）", lo, hi)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// coerceFill 把 LLM 返回的原始值按字段约束校正为合法值；不合法返回 (nil,false) 丢弃该项。
func coerceFill(c domain.ColumnSpec, v any) (any, bool) {
	if v == nil {
		return nil, false
	}
	kind := "text"
	if c.Field != nil && c.Field.Kind != "" {
		kind = c.Field.Kind
	}
	switch kind {
	case "single":
		if val, ok := matchOption(c, v); ok {
			return val, true
		}
		return nil, false
	case "multi":
		arr, ok := v.([]any)
		if !ok {
			return nil, false
		}
		out := []string{}
		for _, e := range arr {
			if val, ok := matchOption(c, e); ok {
				out = append(out, val)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	case "bool":
		switch x := v.(type) {
		case bool:
			return x, true
		case string:
			s := strings.ToLower(strings.TrimSpace(x))
			if s == "true" || s == "是" || s == "1" || s == "yes" {
				return true, true
			}
			if s == "false" || s == "否" || s == "0" || s == "no" {
				return false, true
			}
		}
		return nil, false
	case "number":
		f, ok := toFloat(v)
		if !ok {
			return nil, false
		}
		if c.Field != nil {
			if c.Field.Min != nil && f < *c.Field.Min {
				f = *c.Field.Min
			}
			if c.Field.Max != nil && f > *c.Field.Max {
				f = *c.Field.Max
			}
		}
		return f, true
	default: // text / date
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, false
		}
		return s, true
	}
}

// matchOption 把模型返回的字符串对到选项的 value（先比 value 再比 label，忽略大小写）。
func matchOption(c domain.ColumnSpec, v any) (string, bool) {
	s, ok := v.(string)
	if !ok || c.Field == nil {
		return "", false
	}
	s = strings.TrimSpace(s)
	for _, o := range c.Field.Options {
		if o.Value == s {
			return o.Value, true
		}
	}
	for _, o := range c.Field.Options {
		if strings.EqualFold(o.Value, s) || strings.EqualFold(o.Label, s) {
			return o.Value, true
		}
	}
	return "", false
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// ---- OpenAI 兼容 Chat Completions 请求/响应 ----

type chatReq struct {
	Model          string      `json:"model"`
	Messages       []chatMsg   `json:"messages"`
	Temperature    float64     `json:"temperature"`
	MaxTokens      int         `json:"max_tokens,omitempty"`
	Stream         bool        `json:"stream"`
	ResponseFormat *respFormat `json:"response_format,omitempty"`
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type respFormat struct {
	Type string `json:"type"` // "json_object"
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}
