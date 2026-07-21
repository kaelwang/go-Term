// Package gateway implements the WebSocket terminal gateway: session registry,
// WS upgrade + connection routing, and the bidirectional bridge between a
// remote protocol.Conn and the WebSocket stream.
package gateway

import (
	"encoding/base64"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kaelwang/go-Term/internal/protocol"
	"go.uber.org/zap"
)

// Session represents one live terminal session bound to a WebSocket.
type Session struct {
	ID       string
	Conn     protocol.Conn
	ws       *websocket.Conn
	registry *SessionRegistry

	lastActive atomic.Int64
	closed    atomic.Bool
	mu        sync.Mutex

	// gateMu serializes exclusive ownership of Conn. While a file transfer is
	// running it is held for the transfer's whole duration, pausing the normal
	// pump bridge so transfer bytes are not interleaved with terminal traffic
	// (F1 / A3).
	gateMu sync.Mutex
	// transferring is true while a file transfer owns the session Conn.
	transferring atomic.Bool
}

// SessionRegistry tracks all active sessions in a concurrency-safe map.
type SessionRegistry struct {
	sessions sync.Map // map[string]*Session
	keepAlive time.Duration
}

// NewSessionRegistry creates a registry with the given keepalive period.
func NewSessionRegistry(keepAlive time.Duration) *SessionRegistry {
	if keepAlive <= 0 {
		keepAlive = 30 * time.Second
	}
	return &SessionRegistry{keepAlive: keepAlive}
}

// Register stores a session.
func (r *SessionRegistry) Register(s *Session) {
	r.sessions.Store(s.ID, s)
}

// Get returns a session by ID.
func (r *SessionRegistry) Get(id string) (*Session, bool) {
	v, ok := r.sessions.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*Session), true
}

// Remove closes and deletes a session.
func (r *SessionRegistry) Remove(id string) {
	if v, ok := r.sessions.LoadAndDelete(id); ok {
		v.(*Session).Close()
	}
}

// Count returns the number of active sessions.
func (r *SessionRegistry) Count() int {
	n := 0
	r.sessions.Range(func(_, _ interface{}) bool {
		n++
		return true
	})
	return n
}

// StartKeepalive periodically pings every session.
func (r *SessionRegistry) StartKeepalive() {
	go func() {
		t := time.NewTicker(r.keepAlive)
		defer t.Stop()
		for range t.C {
			r.sessions.Range(func(_, v interface{}) bool {
				v.(*Session).SendKeepalive()
				return true
			})
		}
	}()
}

// markActive records the last activity timestamp.
func (s *Session) markActive() { s.lastActive.Store(time.Now().UnixNano()) }

// SendKeepalive emits a keepalive envelope to the client.
func (s *Session) SendKeepalive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return
	}
	_ = s.ws.WriteJSON(Message{Type: "keepalive", Session: s.ID})
}

// WriteOutput pushes remote output to the client as a base64 data message.
func (s *Session) WriteOutput(b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"data": base64.StdEncoding.EncodeToString(b),
	})
	_ = s.ws.WriteJSON(Message{Type: "data", Session: s.ID, Payload: payload})
}

// RunExclusive executes fn while holding exclusive ownership of the session
// Conn. It blocks the normal pump bridge (which also acquires gateMu around
// its Conn access) for the whole duration, guaranteeing a single owner of the
// byte stream — used by file transfers (F1 / A3).
func (s *Session) RunExclusive(fn func()) {
	s.gateMu.Lock()
	s.transferring.Store(true)
	defer func() {
		s.transferring.Store(false)
		s.gateMu.Unlock()
	}()
	fn()
}

// WriteMessage emits a JSON envelope to the client under the session mutex so
// it never races with the pumps' own writes.
func (s *Session) WriteMessage(m Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return
	}
	_ = s.ws.WriteJSON(m)
}

// Close tears down the session (idempotent).
func (s *Session) Close() {
	if s.closed.Swap(true) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Conn != nil {
		_ = s.Conn.Close()
	}
	_ = s.ws.Close()
	zap.L().Debug("session closed", zap.String("id", s.ID))
}
