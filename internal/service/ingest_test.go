package service

import "testing"

// E11 / 工程优化 #1：content_hash 必须基于显式 hash_columns（concat_ws，固定顺序），
// 绝不能用 s::text 全行——否则源表加列/改列顺序会炸全表重标。
// buildHashExpr 是 §12.2 的 SQL 生成器，这里把它的输出钉死。

func TestBuildHashExpr_ExplicitColumns(t *testing.T) {
	got := buildHashExpr([]string{"title", "body"})
	want := `md5(concat_ws('|', COALESCE("title"::text, ''), COALESCE("body"::text, '')))`
	if got != want {
		t.Errorf("buildHashExpr=\n  %s\n期望\n  %s", got, want)
	}
}

func TestBuildHashExpr_Empty(t *testing.T) {
	if got := buildHashExpr(nil); got != "md5('')" {
		t.Errorf("空 hash_columns 期望 md5('')，得 %s", got)
	}
}

// 列顺序：表达式顺序 = hash_columns 切片顺序（由 admin 配置固定），
// 与源表物理列顺序无关——后者根本不是本函数的输入。
func TestBuildHashExpr_OrderFollowsSlice(t *testing.T) {
	ab := buildHashExpr([]string{"a", "b"})
	ba := buildHashExpr([]string{"b", "a"})
	if ab == ba {
		t.Errorf("切片顺序不同应产生不同表达式，但都为 %s", ab)
	}
	// 同一输入必须确定性可复现（增量同步两版用同 hash_columns → 同表达式 → 可比）
	if buildHashExpr([]string{"a", "b"}) != ab {
		t.Errorf("buildHashExpr 非确定性")
	}
}

// 安全护栏：绝不出现全行 hash（如 md5(t.*::text) / md5(s::text)）。
func TestBuildHashExpr_NeverWholeRow(t *testing.T) {
	expr := buildHashExpr([]string{"id", "title"})
	for _, bad := range []string{"::text)", "*::text", "s::text", "t::text"} {
		if containsSub(expr, bad) {
			t.Errorf("表达式含疑似全行 hash 片段 %q：%s", bad, expr)
		}
	}
}

// 标识符注入防护：列名里的双引号被转义。
func TestBuildHashExpr_QuotesEscaped(t *testing.T) {
	got := buildHashExpr([]string{`a"b`})
	want := `md5(concat_ws('|', COALESCE("a""b"::text, '')))`
	if got != want {
		t.Errorf("转义错误：\n  %s\n期望\n  %s", got, want)
	}
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
