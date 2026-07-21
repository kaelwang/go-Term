package api

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kaelwang/go-Term/internal/store"
)

// loginGuard enforces brute-force protection on the login endpoint. Consecutive
// password failures from a given client IP escalate:
//
//	3  failures -> locked for 60 seconds
//	5  failures -> locked for 1 hour
//	10 failures -> permanently banned (until an admin unlocks, see
//	               LockoutUnlockHandler)
//
// Lockout is keyed by client IP (not username) so an attacker cannot rotate
// usernames to reset the counter, and the state is persisted in SQLite
// (store.Lockout) so it survives process restarts. A successful login deletes
// the row so only *consecutive* failures accumulate.
type loginGuardInfo struct {
	Allowed    bool
	Locked     bool
	Banned     bool
	RetryAfter int // seconds remaining until a lock expires
	FailCount  int
	Remaining  int // attempts left until the next penalty tier (when allowed)
	Message    string
	Warn       string
}

const (
	lockTier1 = 3
	lockTier2 = 5
	lockTier3 = 10
	lock1     = 60 * time.Second
	lock2     = time.Hour
)

// loginGuardMu serializes the read-modify-write of a lockout record so two
// concurrent failures cannot both observe the same count and under-increment.
var loginGuardMu sync.Mutex

// clientIP returns the originating client IP. It prefers proxy headers
// (X-Forwarded-For / X-Real-IP) so the guard works correctly behind a reverse
// proxy, falling back to the direct remote address otherwise.
func clientIP(c *gin.Context) string {
	if v := strings.TrimSpace(c.GetHeader("X-Forwarded-For")); v != "" {
		if idx := strings.IndexByte(v, ','); idx >= 0 {
			v = v[:idx]
		}
		if ip := strings.TrimSpace(v); ip != "" {
			return ip
		}
	}
	if v := strings.TrimSpace(c.GetHeader("X-Real-IP")); v != "" {
		return v
	}
	if host, _, err := net.SplitHostPort(c.Request.RemoteAddr); err == nil {
		return host
	}
	return c.Request.RemoteAddr
}

// nextPenalty reports the failure count at which the next penalty kicks in and a
// human-readable description of that penalty.
func nextPenalty(fc int) (threshold int, text string) {
	switch {
	case fc < lockTier1:
		return lockTier1, "锁定 60 秒"
	case fc < lockTier2:
		return lockTier2, "锁定 1 小时"
	default:
		return lockTier3, "永久禁止登录"
	}
}

func penaltyWarn(fc int) string {
	t, text := nextPenalty(fc)
	rem := t - fc
	if rem <= 0 {
		return ""
	}
	if rem == 1 {
		return fmt.Sprintf("再错误 1 次将%s", text)
	}
	return fmt.Sprintf("再错误 %d 次将%s", rem, text)
}

func lockWarn(fc int) string {
	switch {
	case fc >= lockTier3:
		return "该 IP 已永久禁止登录，请联系管理员解锁"
	case fc >= lockTier2:
		return "该 IP 已被锁定 1 小时，期间无法登录；继续错误将永久禁止登录"
	case fc >= lockTier1:
		return "该 IP 已被锁定 60 秒，期间无法登录；连续错误过多将延长锁定甚至永久禁止登录"
	default:
		return ""
	}
}

// buildInfo derives the public info from an (already consistent) lockout record.
func buildInfo(l *store.Lockout) loginGuardInfo {
	info := loginGuardInfo{FailCount: l.FailCount}
	if l.Banned {
		info.Banned = true
		info.Allowed = false
		info.Message = "该 IP 因连续 10 次密码错误已被永久禁止登录，请联系管理员解锁"
		info.Warn = lockWarn(l.FailCount)
		return info
	}
	if l.LockUntil > 0 {
		until := time.Unix(l.LockUntil, 0)
		if time.Now().Before(until) {
			rem := int(time.Until(until).Seconds())
			if rem < 0 {
				rem = 0
			}
			info.Locked = true
			info.Allowed = false
			info.RetryAfter = rem
			info.Message = fmt.Sprintf("该 IP 已锁定，请 %d 秒后重试", rem)
			info.Warn = lockWarn(l.FailCount)
			return info
		}
	}
	info.Allowed = true
	t, _ := nextPenalty(l.FailCount)
	info.Remaining = t - l.FailCount
	info.Warn = penaltyWarn(l.FailCount)
	return info
}

// loginGuardCheck returns the current state for an IP without mutating it.
func loginGuardCheck(ip string) loginGuardInfo {
	loginGuardMu.Lock()
	defer loginGuardMu.Unlock()
	l, err := store.GetLockout(ip)
	if err != nil {
		// On store failure fail open (allow) rather than lock everyone out.
		return loginGuardInfo{Allowed: true}
	}
	return buildInfo(l)
}

// loginGuardFail records a failed attempt and returns the resulting state, which
// may now be locked or banned. username is recorded for admin visibility.
func loginGuardFail(ip, username string) loginGuardInfo {
	loginGuardMu.Lock()
	defer loginGuardMu.Unlock()
	l, _ := store.GetLockout(ip)
	if l == nil {
		l = &store.Lockout{IP: ip}
	}
	l.FailCount++
	l.LastUser = username
	switch {
	case l.FailCount >= lockTier3:
		l.Banned = true
	case l.FailCount >= lockTier2:
		l.LockUntil = time.Now().Add(lock2).Unix()
	case l.FailCount >= lockTier1:
		l.LockUntil = time.Now().Add(lock1).Unix()
	}
	_ = store.SaveLockout(l)
	return buildInfo(l)
}

// loginGuardSucceed clears all lockout state for an IP after a successful login
// so the consecutive-failure counter resets.
func loginGuardSucceed(ip string) {
	loginGuardMu.Lock()
	defer loginGuardMu.Unlock()
	_ = store.DeleteLockout(ip)
}

// loginGuardUnlock clears the lockout for an IP on an explicit admin action.
func loginGuardUnlock(ip string) error {
	loginGuardMu.Lock()
	defer loginGuardMu.Unlock()
	return store.DeleteLockout(ip)
}
