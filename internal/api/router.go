package api

import (
	"github.com/gin-gonic/gin"
	"github.com/kaelwang/go-Term/internal/gateway"
	static "github.com/kaelwang/go-Term/internal/static"
)

// NewRouter builds the gin engine with all routes wired. The version string is
// surfaced by /api/public/config.
func NewRouter(registry *gateway.SessionRegistry, version string) *gin.Engine {
	SetRegistry(registry)
	apiVersion = version

	r := gin.New()
	r.Use(gin.Recovery())

	// Public endpoints (no JWT required, C7).
	r.POST("/api/login", LoginHandler)
	r.GET("/api/public/config", PublicConfigHandler)

	// JWT-protected REST API. When ENABLE_AUTH=1 every route below (except
	// /login and /public/config) requires a valid token.
	apiG := r.Group("/api")
	apiG.Use(JWTAuth())
	{
		apiG.POST("/test-terminal", TestTerminalHandler)
		apiG.GET("/download", DownloadHandler)
		apiG.POST("/upload", UploadHandler)
		apiG.GET("/hostkey", HostKeyHandler)
		apiG.POST("/hostkey", HostKeyHandler).Use(RequireServerUser())
		apiG.GET("/local-shell-enabled", LocalShellEnabledHandler)
		apiG.GET("/ssh-config-hosts", SSHConfigHostsHandler)
		apiG.GET("/sessions", SessionCountHandler)
		// file transfer (trzsz)
		apiG.POST("/transfer-upload", TransferUploadHandler)
		apiG.GET("/transfer-file", TransferFileHandler)
		apiG.GET("/transfer-bins", TransferBinsHandler)
		// file manager
		apiG.POST("/list", FileListHandler)
		apiG.GET("/file", FileDownloadHandler)
		apiG.POST("/file", FileUploadHandler)
		apiG.POST("/mkdir", FileMkdirHandler)
		apiG.POST("/rename", FileRenameHandler)
		apiG.POST("/remove", FileRemoveHandler)

		// Auth / identity.
		apiG.GET("/me", MeHandler)
		apiG.PUT("/me", MeUpdateHandler)

		// User management (admin only). A dedicated sub-group scoped to
		// /users applies RoleRequired("admin") to these routes only.
		usersG := apiG.Group("/users")
		usersG.Use(RoleRequired("admin"))
		{
			usersG.GET("", UsersListHandler)
			usersG.POST("", UsersCreateHandler)
			usersG.PUT("/:id", UsersUpdateHandler)
			usersG.DELETE("/:id", UsersDeleteHandler)
			usersG.POST("/:id/reset-password", UsersResetPasswordHandler)
			// Brute-force login lockouts (keyed by client IP).
			usersG.GET("/lockouts", LockoutsListHandler)
			usersG.POST("/lockouts/:ip/unlock", LockoutUnlockHandler)
		}

		// Credential vault (per-user isolation; no RequireServerUser, C4).
		apiG.GET("/credentials", CredentialsList)
		apiG.POST("/credentials", CredentialsCreate)
		apiG.GET("/credentials/:id/secret", CredentialsGetSecret)
		apiG.PUT("/credentials/:id", CredentialsUpdate)
		apiG.DELETE("/credentials/:id", CredentialsDelete)

		// Saved connections & groups (per-user isolation).
		apiG.GET("/connections", ConnectionsList)
		apiG.POST("/connections", ConnectionsCreate)
		apiG.PUT("/connections/:id", ConnectionsUpdate)
		apiG.DELETE("/connections/:id", ConnectionsDelete)
		apiG.GET("/connection-groups", GroupsList)
		apiG.POST("/connection-groups", GroupsCreate)
		apiG.PUT("/connection-groups/:id", GroupsUpdate)
		apiG.DELETE("/connection-groups/:id", GroupsDelete)

		// Per-user settings.
		apiG.GET("/settings", SettingsGet)
		apiG.PUT("/settings", SettingsPut)
	}

	// WebSocket terminal (JWT protected when auth is enabled).
	r.GET("/ws", JWTAuth(), func(c *gin.Context) {
		gateway.Connect(registry, c.Writer, c.Request)
	})

	// Serve the embedded SPA build (production). No-op in dev builds.
	static.Register(r)

	return r
}

// SessionCountHandler reports the number of active sessions.
func SessionCountHandler(c *gin.Context) {
	if apiRegistry == nil {
		respond(c, 0, "ok", gin.H{"count": 0})
		return
	}
	respond(c, 0, "ok", gin.H{"count": apiRegistry.Count()})
}
