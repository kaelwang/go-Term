package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/protocol"
	"github.com/kaelwang/go-Term/internal/security"
	"go.uber.org/zap"
)

// upgrader upgrades HTTP connections to WebSockets, accepting all origins.
var upgrader = websocket.Upgrader{
	CheckOrigin:  func(r *http.Request) bool { return true },
	Subprotocols: []string{"webssh"},
}

// Message is the JSON envelope exchanged on the WebSocket.
type Message struct {
	Type    string          `json:"type"`
	Session string          `json:"session"`
	Payload json.RawMessage `json:"payload"`
}

// errorMessage builds an error envelope.
func errorMessage(text string) Message {
	return Message{
		Type:    "error",
		Payload: json.RawMessage(`"` + jsonEscape(text) + `"`),
	}
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

// connectRequest is the initial envelope sent by the client to open a session.
type connectRequest struct {
	Type    string `json:"type"`
	Session string `json:"session"`
	Payload struct {
		Connection *protocol.Connection `json:"connection"`
	} `json:"payload"`
}

// Connect upgrades the request, reads the connection spec, dials the target
// protocol, and wires up the I/O bridge.
func Connect(registry *SessionRegistry, w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		zap.L().Error("ws upgrade failed", zap.Error(err))
		return
	}

	_, data, err := ws.ReadMessage()
	if err != nil {
		_ = ws.Close()
		return
	}
	var req connectRequest
	if err := json.Unmarshal(data, &req); err != nil {
		_ = ws.WriteJSON(errorMessage("bad request"))
		_ = ws.Close()
		return
	}
	conn := req.Payload.Connection
	if conn == nil {
		_ = ws.WriteJSON(errorMessage("missing connection spec"))
		_ = ws.Close()
		return
	}

	// 前端不一定会传 known_hosts_path，缺失时用全局配置兜底，
	// 避免 SSH 握手因空路径报 "empty known_hosts path"。
	applyConnDefaults(conn)

	if conn.Protocol == protocol.ProtocolLocalShell && config.Global.DisableLocalTerm {
		_ = ws.WriteJSON(errorMessage("local terminal is disabled"))
		_ = ws.Close()
		return
	}

	// Localshell gate (F3 / A7): when auth is enabled and a SERVER_USER
	// whitelist is configured, only whitelisted users may open a local shell.
	// SSH/Telnet/VNC sessions are not subject to this gate.
	if conn.Protocol == protocol.ProtocolLocalShell &&
		config.Global.AuthEnabled &&
		len(config.Global.ServerUserWhitelist) > 0 {
		if err := checkServerUserWhitelist(r); err != nil {
			_ = ws.WriteJSON(errorMessage("permission denied"))
			_ = ws.Close()
			return
		}
	}

	// Resolve a saved vault credential by id when the connection references
	// one and auth is enabled (T-V3). On any failure we skip resolution and
	// proceed with whatever inline credential (if any) was supplied.
	if conn.CredentialID != "" && config.Global.AuthEnabled && CredentialResolver != nil {
		if u, uerr := wsUser(r); uerr == nil && u != "" {
			if cred, rerr := CredentialResolver(u, conn.CredentialID); rerr == nil && cred != nil {
				conn.Credential = cred
			} else if rerr != nil {
				zap.L().Warn("credential resolution failed",
					zap.String("id", conn.CredentialID), zap.Error(rerr))
			}
		}
	}

	proto, err := protocol.GetProtocol(conn.Protocol)
	if err != nil {
		_ = ws.WriteJSON(errorMessage("unsupported protocol"))
		_ = ws.Close()
		return
	}

	remote, err := proto.Dial(conn)
	if err != nil {
		_ = ws.WriteJSON(errorMessage(err.Error()))
		_ = ws.Close()
		return
	}

	sess := &Session{
		ID:       conn.ID,
		Conn:     remote,
		ws:       ws,
		registry: registry,
	}
	registry.Register(sess)
	zap.L().Info("session opened", zap.String("id", sess.ID), zap.String("protocol", string(conn.Protocol)))

	go WritePump(sess)
	go ReadPump(sess)
}

// applyConnDefaults fills optional connection fields from the global config
// when the client (frontend) does not supply them. It backs the known_hosts
// path with config.Global.KnownHostsPath so the SSH host-key callback never
// receives an empty path — an empty path is what surfaced as
// "empty known_hosts path" during the handshake before this fallback existed.
//
// The same *protocol.Connection is passed down to the protocol dialers, so
// proxies and jump-host hops (which read conn.KnownHostsPath via
// makeHostKeyCallback(conn)) inherit the fallback automatically.
func applyConnDefaults(conn *protocol.Connection) {
	if conn == nil {
		return
	}
	if conn.KnownHostsPath == "" && config.Global.KnownHostsPath != "" {
		conn.KnownHostsPath = config.Global.KnownHostsPath
	}
}

// checkServerUserWhitelist validates the JWT carried in the WebSocket query
// string against the SERVER_USER whitelist (F3 / A7).
func checkServerUserWhitelist(r *http.Request) error {
	token := r.URL.Query().Get("token")
	if token == "" {
		return fmt.Errorf("missing token")
	}
	claims, err := security.ParseToken(token, config.Global.JWTSecret)
	if err != nil {
		return err
	}
	if !config.Global.ServerUserAllowed(claims.User) {
		return fmt.Errorf("permission denied")
	}
	return nil
}
