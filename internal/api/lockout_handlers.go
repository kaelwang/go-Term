package api

import (
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kaelwang/go-Term/internal/store"
)

// LockoutsListHandler lists every client IP currently tracked by the login
// brute-force guard (admin only). It exposes the failure count, lock/banned
// state and remaining seconds so an admin can decide what to unlock.
func LockoutsListHandler(c *gin.Context) {
	list, err := store.ListLockouts()
	if err != nil {
		respond(c, CodeTransferFail, err.Error(), nil)
		return
	}
	now := time.Now()
	out := make([]gin.H, 0, len(list))
	for _, l := range list {
		locked := false
		retryAfter := 0
		if !l.Banned && l.LockUntil > 0 {
			if until := time.Unix(l.LockUntil, 0); now.Before(until) {
				locked = true
				retryAfter = int(time.Until(until).Seconds())
				if retryAfter < 0 {
					retryAfter = 0
				}
			}
		}
		out = append(out, gin.H{
			"ip":           l.IP,
			"fail_count":   l.FailCount,
			"locked":       locked,
			"banned":       l.Banned,
			"retry_after":  retryAfter,
			"last_username": l.LastUser,
			"updated_at":   l.UpdatedAt,
		})
	}
	respond(c, 0, "ok", out)
}

// LockoutUnlockHandler clears the brute-force lockout for a specific IP so its
// owner can attempt login again (admin only). The IP is URL-encoded in the path
// (IPv6 addresses contain colons) and decoded here.
func LockoutUnlockHandler(c *gin.Context) {
	ip := c.Param("ip")
	if dec, err := url.QueryUnescape(ip); err == nil {
		ip = dec
	}
	if ip == "" {
		respond(c, CodeBadParam, "ip required", nil)
		return
	}
	if err := loginGuardUnlock(ip); err != nil {
		respond(c, CodeTransferFail, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", gin.H{"ip": ip})
}
