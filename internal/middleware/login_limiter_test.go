package middleware

import (
	"testing"
)

// 失败累计到阈值即锁定，成功（Reset）清零；指数退避使后续锁定时长递增。
func TestLoginLimiter_LockoutAndReset(t *testing.T) {
	l := NewLoginLimiter()
	key := "user@example.com|1.2.3.4"

	// 阈值（5）之前不锁定。
	for i := 0; i < l.threshold-1; i++ {
		if locked := l.Fail(key); locked {
			t.Fatalf("第 %d 次失败不应锁定", i+1)
		}
		if locked, _ := l.Locked(key); locked {
			t.Fatalf("第 %d 次失败后不应处于锁定", i+1)
		}
	}

	// 第 threshold 次失败进入锁定。
	if locked := l.Fail(key); !locked {
		t.Fatal("达到阈值应锁定")
	}
	locked, d1 := l.Locked(key)
	if !locked || d1 <= 0 {
		t.Fatalf("应处于锁定且有剩余时长，got locked=%v d=%v", locked, d1)
	}

	// 再失败一次，锁定时长应翻倍（指数退避）。
	l.Fail(key)
	_, d2 := l.Locked(key)
	if d2 <= d1 {
		t.Fatalf("指数退避应使锁定时长递增：d1=%v d2=%v", d1, d2)
	}

	// 成功后清零，立即解锁。
	l.Reset(key)
	if locked, _ := l.Locked(key); locked {
		t.Fatal("Reset 后应解锁")
	}
}

// 不同 key（不同邮箱/IP）互不影响。
func TestLoginLimiter_KeysIsolated(t *testing.T) {
	l := NewLoginLimiter()
	for i := 0; i < l.threshold; i++ {
		l.Fail("a@x.com|1.1.1.1")
	}
	if locked, _ := l.Locked("a@x.com|1.1.1.1"); !locked {
		t.Fatal("a 应被锁定")
	}
	if locked, _ := l.Locked("b@x.com|1.1.1.1"); locked {
		t.Fatal("b 不应受 a 影响")
	}
}
