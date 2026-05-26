package middleware

import (
	"sync"
	"time"
)

// LoginLimiter 是按「邮箱+IP」的登录失败计数 + 锁定（公网防爆破）。
// 单实例内存版，仿 RateLimit；多实例后续换 redis。
//
// 策略：连续失败达 threshold 次后锁定 base；其后每多失败一次，锁定时长翻倍（指数退避），
// 封顶 maxLock。登录成功即清零。
type LoginLimiter struct {
	mu        sync.Mutex
	attempts  map[string]*loginAttempt
	threshold int
	base      time.Duration
	maxLock   time.Duration
}

type loginAttempt struct {
	fails       int
	lockedUntil time.Time
}

// NewLoginLimiter 默认：5 次失败锁 5 分钟起、指数退避封顶 1 小时。
func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{
		attempts:  make(map[string]*loginAttempt),
		threshold: 5,
		base:      5 * time.Minute,
		maxLock:   time.Hour,
	}
}

// Locked 报告 key 当前是否在锁定中，及剩余时长。
func (l *LoginLimiter) Locked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.attempts[key]
	if a == nil {
		return false, 0
	}
	if d := time.Until(a.lockedUntil); d > 0 {
		return true, d
	}
	return false, 0
}

// Fail 记一次失败；达阈值则（重新）计算锁定截止时间。返回是否已进入锁定。
func (l *LoginLimiter) Fail(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.attempts[key]
	if a == nil {
		a = &loginAttempt{}
		l.attempts[key] = a
	}
	a.fails++
	if a.fails < l.threshold {
		return false
	}
	// 第 threshold 次失败锁 base，其后每次翻倍。
	lock := l.base << (a.fails - l.threshold)
	if lock <= 0 || lock > l.maxLock { // 移位溢出或超上限
		lock = l.maxLock
	}
	a.lockedUntil = time.Now().Add(lock)
	return true
}

// Reset 登录成功后清零该 key。
func (l *LoginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}
