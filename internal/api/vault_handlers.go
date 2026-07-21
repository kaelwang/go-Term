package api

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/security"
	"github.com/kaelwang/go-Term/internal/store"
)

// currentUser extracts the JWT username injected by JWTAuth().
func currentUser(c *gin.Context) string {
	u, _ := c.Get("user")
	s, _ := u.(string)
	return s
}

// CredentialsList returns the metadata of the caller's credentials (no secret).
func CredentialsList(c *gin.Context) {
	list, err := store.ListCredentials(currentUser(c))
	if err != nil {
		respond(c, CodeTransferFail, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", list)
}

// CredentialsCreate stores a new encrypted credential.
func CredentialsCreate(c *gin.Context) {
	var req struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		PrivateKey string `json:"private_key"`
		Passphrase string `json:"passphrase"`
		Meta       string `json:"meta"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" ||
		(req.Type != "password" && req.Type != "private_key") {
		respond(c, CodeBadParam, "name and valid type required", nil)
		return
	}
	secret := store.CredentialSecret{
		Username:   req.Username,
		Password:   req.Password,
		PrivateKey: req.PrivateKey,
		Passphrase: req.Passphrase,
	}
	plain, err := json.Marshal(secret)
	if err != nil {
		respond(c, CodeTransferFail, err.Error(), nil)
		return
	}
	enc, err := security.Encrypt(string(plain), config.Global.VaultKey())
	if err != nil {
		respond(c, CodeTransferFail, err.Error(), nil)
		return
	}
	if req.Meta == "" {
		req.Meta = "{}"
	}
	id, err := store.CreateCredential(currentUser(c), req.Name, req.Type, enc, req.Meta)
	if err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", gin.H{"id": id})
}

// CredentialsUpdate overwrites an existing credential (re-encrypts value).
func CredentialsUpdate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respond(c, CodeBadParam, "bad id", nil)
		return
	}
	var req struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		PrivateKey string `json:"private_key"`
		Passphrase string `json:"passphrase"`
		Meta       string `json:"meta"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" ||
		(req.Type != "password" && req.Type != "private_key") {
		respond(c, CodeBadParam, "name and valid type required", nil)
		return
	}
	secret := store.CredentialSecret{
		Username:   req.Username,
		Password:   req.Password,
		PrivateKey: req.PrivateKey,
		Passphrase: req.Passphrase,
	}
	plain, _ := json.Marshal(secret)
	enc, err := security.Encrypt(string(plain), config.Global.VaultKey())
	if err != nil {
		respond(c, CodeTransferFail, err.Error(), nil)
		return
	}
	if req.Meta == "" {
		req.Meta = "{}"
	}
	if err := store.UpdateCredential(currentUser(c), req.Name, req.Type, enc, req.Meta, id); err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", nil)
}

// CredentialsDelete removes a credential.
func CredentialsDelete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respond(c, CodeBadParam, "bad id", nil)
		return
	}
	if err := store.DeleteCredential(currentUser(c), id); err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", nil)
}

// CredentialsGetSecret decrypts and returns a credential's plaintext on demand.
func CredentialsGetSecret(c *gin.Context) {
	pc, err := store.GetCredentialDecrypted(currentUser(c), c.Param("id"))
	if err != nil {
		respond(c, CodePermissionDenied, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", pc)
}
