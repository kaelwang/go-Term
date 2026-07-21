package gateway

import (
	"fmt"
	"net/http"

	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/protocol"
	"github.com/kaelwang/go-Term/internal/security"
)

// CredentialResolver decrypts a saved vault credential on demand. It is wired
// to the SQLite store by internal.New; when nil, the gateway skips credential
// resolution (backward compatible, T-V3).
var CredentialResolver func(user, id string) (*protocol.Credential, error)

// wsUser extracts the JWT username carried in the WebSocket query string. It
// mirrors checkServerUserWhitelist's token-parsing approach (F3 / A7).
func wsUser(r *http.Request) (string, error) {
	token := r.URL.Query().Get("token")
	if token == "" {
		return "", fmt.Errorf("missing token")
	}
	claims, err := security.ParseToken(token, config.Global.JWTSecret)
	if err != nil {
		return "", err
	}
	return claims.User, nil
}
