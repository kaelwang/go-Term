package api

import (
	"github.com/gin-gonic/gin"
	"github.com/kaelwang/go-Term/internal/config"
)

// PublicConfigHandler returns non-sensitive deployment info used by the
// frontend login gate. It is a public endpoint (no JWT required, C7).
func PublicConfigHandler(c *gin.Context) {
	authEnabled := config.Global != nil && config.Global.AuthEnabled
	respond(c, 0, "ok", gin.H{
		"auth_enabled": authEnabled,
		"version":      apiVersion,
	})
}
