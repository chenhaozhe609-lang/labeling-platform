package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

func f64(v float64) *float64 { return &v }

func demoSchema() *domain.FormSchema {
	return &domain.FormSchema{
		Version:     1,
		PrimaryCols: []string{"title"},
		Columns: []domain.ColumnSpec{
			{Code: "title", Type: "text", Role: domain.ColContext, Label: "专业名称"},
			{Code: "discipline", Type: "varchar", Role: domain.ColFill, Label: "学科归类",
				Field: &domain.FieldConfig{Kind: "single", Options: []domain.FieldOption{
					{Value: "theory", Label: "理论型"},
					{Value: "engineering", Label: "工程型"},
					{Value: "applied", Label: "应用型"},
				}}},
			{Code: "difficulty", Type: "smallint", Role: domain.ColFill, Label: "难度",
				Field: &domain.FieldConfig{Kind: "number", Min: f64(1), Max: f64(5)}},
			{Code: "needs_math", Type: "bool", Role: domain.ColFill, Label: "需数学",
				Field: &domain.FieldConfig{Kind: "bool"}},
		},
	}
}

// 用 mock OpenAI 兼容端点验证：请求格式正确 + 响应按 schema 校正（label→value、数值钳制、bool 容错）。
func TestLLMPrefiller_CoerceAndRequest(t *testing.T) {
	var gotAuth, gotModel string
	var gotRespFormat bool
	var gotUser string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		var req chatReq
		_ = json.Unmarshal(body, &req)
		gotModel = req.Model
		gotRespFormat = req.ResponseFormat != nil && req.ResponseFormat.Type == "json_object"
		for _, m := range req.Messages {
			if m.Role == "user" {
				gotUser = m.Content
			}
		}
		// 模型返回：discipline 用「标签」而非 value；difficulty 越界；bool 用中文「是」
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{\"discipline\":\"工程型\",\"difficulty\":9,\"needs_math\":\"是\"}"}}]}`)
	}))
	defer srv.Close()

	p := NewLLMPrefiller(LLMConfig{BaseURL: srv.URL, APIKey: "test-key", Model: "deepseek-chat"})
	out, err := p.Prefill(context.Background(), demoSchema(), map[string]any{"title": "计算机科学与技术"})
	if err != nil {
		t.Fatalf("Prefill 出错: %v", err)
	}

	// 请求侧
	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization = %q, 期望 Bearer test-key", gotAuth)
	}
	if gotModel != "deepseek-chat" {
		t.Errorf("model = %q", gotModel)
	}
	if !gotRespFormat {
		t.Errorf("response_format 未设为 json_object")
	}
	if !strings.Contains(gotUser, "计算机科学与技术") {
		t.Errorf("user prompt 未含上下文值, got: %s", gotUser)
	}

	// 响应校正侧
	if out["discipline"] != "engineering" {
		t.Errorf("discipline = %v, 期望 engineering（标签→value）", out["discipline"])
	}
	if out["difficulty"] != float64(5) {
		t.Errorf("difficulty = %v, 期望钳制为 5", out["difficulty"])
	}
	if out["needs_math"] != true {
		t.Errorf("needs_math = %v, 期望 true（中文「是」）", out["needs_math"])
	}
}

// 非法单选值应被丢弃，而非写入脏值。
func TestLLMPrefiller_DropInvalidSingle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{\"discipline\":\"不存在的类\"}"}}]}`)
	}))
	defer srv.Close()

	p := NewLLMPrefiller(LLMConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	out, err := p.Prefill(context.Background(), demoSchema(), map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("Prefill 出错: %v", err)
	}
	if _, ok := out["discipline"]; ok {
		t.Errorf("非法单选值应被丢弃，但 out 含 discipline=%v", out["discipline"])
	}
}

// 无源行时不调用 LLM，直接返回空。
func TestLLMPrefiller_SkipWhenNoSource(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{}"}}]}`)
	}))
	defer srv.Close()

	p := NewLLMPrefiller(LLMConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	out, err := p.Prefill(context.Background(), demoSchema(), map[string]any{})
	if err != nil {
		t.Fatalf("Prefill 出错: %v", err)
	}
	if called {
		t.Errorf("无源行时不应调用 LLM")
	}
	if len(out) != 0 {
		t.Errorf("期望空结果, got %v", out)
	}
}
