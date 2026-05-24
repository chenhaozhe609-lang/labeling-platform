package service

import (
	"context"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

// Prefiller 用 context 列预测 fill 列的值（PRD §24.5 LLM 自动预填）。
type Prefiller interface {
	Prefill(ctx context.Context, fs *domain.FormSchema, sourceRow map[string]any) (map[string]any, error)
}

// StubPrefiller 是占位实现：按控件类型给一个合理默认，标记来源 ai。
// 真 LLM 接入后替换此实现（输入 = context 列值，输出 = 各 fill 列预测值）。
type StubPrefiller struct{}

func (StubPrefiller) Prefill(_ context.Context, fs *domain.FormSchema, _ map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for _, c := range fs.FillSpecs() {
		if c.Field == nil {
			continue
		}
		switch c.Field.Kind {
		case "single":
			if len(c.Field.Options) > 0 {
				out[c.Code] = c.Field.Options[0].Value
			}
		case "bool":
			out[c.Code] = false
		case "number":
			if c.Field.Min != nil {
				out[c.Code] = *c.Field.Min
			}
		}
		// text / multi / date：桩不猜，留空待人工
	}
	return out, nil
}
