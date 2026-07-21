package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/security"
	"github.com/kaelwang/go-Term/internal/store"
)

// JWTAuth enforces JWT authentication when ENABLE_AUTH=1. Otherwise it is a
// no-op pass-through.
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Global
		if !cfg.AuthEnabled {
			c.Next()
			return
		}
		token := extractToken(c)
		if token == "" {
			abortCode(c, CodeAuthFail, "missing token")
			return
		}
		claims, err := security.ParseToken(token, cfg.JWTSecret)
		if err != nil {
			abortCode(c, CodeAuthFail, "invalid token")
			return
		}
		c.Set("user", claims.User)
		c.Next()
	}
}

// RequireServerUser restricts an endpoint to the SERVER_USER whitelist.
// It only takes effect when authentication is enabled and a non-empty
// whitelist is configured; otherwise it is a pass-through (F3 / A8).
func RequireServerUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Global.AuthEnabled {
			c.Next()
			return
		}
		user, _ := c.Get("user")
		us, _ := user.(string)
		if !serverUserAllowed(us) {
			abortCode(c, CodePermissionDenied, "permission denied")
			return
		}
		c.Next()
	}
}

// serverUserAllowed centralizes the SERVER_USER whitelist check (F3 / A8).
func serverUserAllowed(user string) bool {
	return config.Global.ServerUserAllowed(user)
}

// RoleRequired restricts an endpoint to callers whose role matches. It is a
// pass-through when auth is disabled; otherwise it resolves the caller's role
// from the users table (the DB is the source of truth, C1) and compares.
func RoleRequired(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Global.AuthEnabled {
			c.Next()
			return
		}
		username, _ := c.Get("user")
		user, _ := username.(string)
		if user == "" {
			abortCode(c, CodePermissionDenied, "permission denied")
			return
		}
		r, err := store.GetUserRole(user)
		if err != nil || r != role {
			abortCode(c, CodePermissionDenied, "permission denied")
			return
		}
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if t := c.Query("token"); t != "" {
		return t
	}
	return ""
}

func abortCode(c *gin.Context, code int, msg string) {
	c.AbortWithStatusJSON(http.StatusOK, gin.H{"code": code, "message": msg, "data": nil})
}
