package api

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/kaelwang/go-Term/internal/store"
)

// SettingsGet returns the caller's settings merged over the code defaults.
func SettingsGet(c *gin.Context) {
	data, err := store.GetSettings(currentUser(c))
	if err != nil {
		respond(c, CodeTransferFail, err.Error(), nil)
		return
	}
	var obj interface{}
	_ = json.Unmarshal([]byte(data), &obj)
	respond(c, 0, "ok", obj)
}

// SettingsPut persists the caller's settings (full object replacement).
func SettingsPut(c *gin.Context) {
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		respond(c, CodeBadParam, "bad params", nil)
		return
	}
	if err := store.SetSettings(currentUser(c), string(raw)); err != nil {
		respond(c, CodeTransferFail, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", nil)
}
