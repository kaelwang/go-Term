package api

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/security"
	"github.com/kaelwang/go-Term/internal/store"
)

// LoginHandler authenticates against the users table with bcrypt and returns a
// JWT. It is a public endpoint and deliberately does NOT apply the SERVER_USER
// whitelist gate (C4 — Web login is orthogonal to the server-user whitelist).
//
// Before checking credentials it enforces the brute-force guard: a locked or
// banned account is rejected up front, and every failed attempt advances the
// consecutive-failure counter that escalates the lockout.
func LoginHandler(c *gin.Context) {
	var req struct {
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.User == "" {
		respond(c, CodeBadParam, "user and password required", nil)
		return
	}
	username := strings.TrimSpace(req.User)
	ip := clientIP(c)

	// Reject locked / banned IPs before even inspecting the password. Lockout
	// is keyed by client IP (not username) so rotating usernames cannot dodge
	// the consecutive-failure counter.
	if info := loginGuardCheck(ip); !info.Allowed {
		respond(c, CodeAuthFail, info.Message, gin.H{
			"locked":      info.Locked,
			"banned":      info.Banned,
			"retry_after": info.RetryAfter,
			"fail_count":  info.FailCount,
			"warn":        info.Warn,
		})
		return
	}

	ok, _ := store.CheckPassword(username, req.Password)
	if !ok {
		info := loginGuardFail(ip, username)
		respond(c, CodeAuthFail, "用户名或密码错误", gin.H{
			"locked":      info.Locked,
			"banned":      info.Banned,
			"retry_after": info.RetryAfter,
			"fail_count":  info.FailCount,
			"remaining":   info.Remaining,
			"warn":        info.Warn,
		})
		return
	}

	// Success resets the consecutive-failure counter (and any lockout).
	loginGuardSucceed(ip)

	role, _ := store.GetUserRole(username)
	cfg := config.Global
	token, err := security.GenerateToken(username, cfg.JWTSecret, cfg.JWTExpireMinutes)
	if err != nil {
		respond(c, CodeAuthFail, "token error", nil)
		return
	}
	respond(c, 0, "ok", gin.H{"token": token, "user": username, "role": role})
}

// MeHandler returns the identity of the authenticated caller.
func MeHandler(c *gin.Context) {
	username := currentUser(c)
	role, _ := store.GetUserRole(username)
	respond(c, 0, "ok", gin.H{"user": username, "role": role})
}

// MeUpdateHandler lets the authenticated user update their own username and/or
// password. Role is never changeable here (admins use /users/:id). When the
// username changes a fresh JWT is returned so the client can re-identify.
func MeUpdateHandler(c *gin.Context) {
	username := currentUser(c)
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" {
		respond(c, CodeBadParam, "username required", nil)
		return
	}
	id, err := store.GetUserID(username)
	if err != nil {
		respond(c, CodeBadParam, "user not found", nil)
		return
	}
	role, _ := store.GetUserRole(username)
	if err := store.UpdateUser(id, req.Username, role, req.Password); err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	out := gin.H{"user": req.Username, "role": role}
	// Re-issue a token when the caller renames themselves so the client can
	// keep calling authenticated endpoints under the new identity.
	if req.Username != username {
		cfg := config.Global
		if newToken, terr := security.GenerateToken(req.Username, cfg.JWTSecret, cfg.JWTExpireMinutes); terr == nil {
			out["token"] = newToken
		}
	}
	respond(c, 0, "ok", out)
}

// UsersUpdateHandler updates any user's username, role, and optionally password
// (admin only). An empty password leaves the existing one intact. If the admin
// edits their own account and renames it, a fresh token is returned.
func UsersUpdateHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respond(c, CodeBadParam, "bad id", nil)
		return
	}
	caller := currentUser(c)
	var req struct {
		Username string `json:"username"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" {
		respond(c, CodeBadParam, "username required", nil)
		return
	}
	if req.Role != "admin" {
		req.Role = "user"
	}
	if err := store.UpdateUser(id, req.Username, req.Role, req.Password); err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	out := gin.H{}
	if callerID, e := store.GetUserID(caller); e == nil && callerID == id && req.Username != caller {
		cfg := config.Global
		if t, terr := security.GenerateToken(req.Username, cfg.JWTSecret, cfg.JWTExpireMinutes); terr == nil {
			out["token"] = t
		}
	}
	respond(c, 0, "ok", out)
}

// UsersListHandler lists all users (admin only).
func UsersListHandler(c *gin.Context) {
	users, err := store.ListUsers()
	if err != nil {
		respond(c, CodeTransferFail, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", users)
}

// UsersCreateHandler creates a new user (admin only).
func UsersCreateHandler(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		respond(c, CodeBadParam, "username and password required", nil)
		return
	}
	if req.Role != "admin" {
		req.Role = "user"
	}
	u, err := store.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", gin.H{"id": u.ID})
}

// UsersDeleteHandler deletes a user (admin only).
func UsersDeleteHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respond(c, CodeBadParam, "bad id", nil)
		return
	}
	// Prevent an admin from deleting the account they are currently logged in
	// with — that would lock out the only remaining admin and brick the UI.
	if callerID, e := store.GetUserID(currentUser(c)); e == nil && callerID == id {
		respond(c, CodeBadParam, "不能删除当前登录的账户", nil)
		return
	}
	if err := store.DeleteUser(id); err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", nil)
}

// UsersResetPasswordHandler changes a user's password (admin only).
func UsersResetPasswordHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respond(c, CodeBadParam, "bad id", nil)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		respond(c, CodeBadParam, "password required", nil)
		return
	}
	if err := store.ResetPassword(id, req.Password); err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", nil)
}
