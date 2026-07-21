package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kaelwang/go-Term/internal/protocol"
)

// mockConn is a minimal protocol.Conn used as the remote side of a Session so
// Close() can be exercised without a real network connection.
type mockConn struct {
	closed bool
	mu     sync.Mutex
}

func (m *mockConn) Read(p []byte) (int, error)  { return 0, io.EOF }
func (m *mockConn) Write(p []byte) (int, error) { return len(p), nil }
func (m *mockConn) Close() error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	return nil
}
func (m *mockConn) Resize(cols, rows int) error { return nil }
func (m *mockConn) WindowChangeSupported() bool { return false }

var _ protocol.Conn = (*mockConn)(nil)

// wsPair spins up a throwaway httptest server, upgrades one WebSocket
// connection, and returns the server-side *websocket.Conn together with the
// client-side *websocket.Conn for assertions. Everything stays on loopback.
func wsPair(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	t.Helper()
	srvCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Upgrade(w, r, nil, 1024, 1024)
		if err != nil {
			t.Errorf("ws upgrade: %v", err)
			return
		}
		srvCh <- c
	}))
	cli, _, err := websocket.DefaultDialer.Dial("ws://"+srv.Listener.Addr().String()+"/", nil)
	if err != nil {
		srv.Close()
		t.Fatalf("ws dial: %v", err)
	}
	srvConn := <-srvCh
	cleanup := func() {
		_ = cli.Close()
		_ = srvConn.Close()
		srv.Close()
	}
	return srvConn, cli, cleanup
}

// TestSessionRegistryLifecycle verifies New/Register/Get/Remove/Count on the
// sync.Map-backed registry.
func TestSessionRegistryLifecycle(t *testing.T) {
	r := NewSessionRegistry(0) // <=0 should default to 30s
	if r.keepAlive != 30*time.Second {
		t.Errorf("keepAlive default: got %v want 30s", r.keepAlive)
	}
	if r.Count() != 0 {
		t.Errorf("new registry Count = %d want 0", r.Count())
	}

	srvConn, cli, cleanup := wsPair(t)
	defer cleanup()
	s := &Session{ID: "s1", ws: srvConn, Conn: &mockConn{}}
	r.Register(s)
	if r.Count() != 1 {
		t.Errorf("after register Count = %d want 1", r.Count())
	}
	got, ok := r.Get("s1")
	if !ok || got != s {
		t.Fatal("Get returned wrong session")
	}
	if _, ok := r.Get("nope"); ok {
		t.Error("Get of unknown id should return ok=false")
	}

	r.Remove("s1")
	if r.Count() != 0 {
		t.Errorf("after remove Count = %d want 0", r.Count())
	}
	_ = cli
}

// TestWriteOutputEnvelope verifies WriteOutput emits the unified data envelope
// with base64-encoded payload.
func TestWriteOutputEnvelope(t *testing.T) {
	srvConn, cli, cleanup := wsPair(t)
	defer cleanup()
	s := &Session{ID: "out1", ws: srvConn}

	s.WriteOutput([]byte("hi there"))

	cli.SetReadDeadline(time.Now().Add(2 * time.Second))
	mt, data, err := cli.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
		t.Fatalf("unexpected message type %d", mt)
	}
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "data" {
		t.Errorf("msg.Type = %q want data", msg.Type)
	}
	if msg.Session != "out1" {
		t.Errorf("msg.Session = %q want out1", msg.Session)
	}
	var payload struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Data == "" {
		t.Error("payload data is empty")
	}
}

// TestSendKeepaliveEnvelope verifies the keepalive envelope is written.
func TestSendKeepaliveEnvelope(t *testing.T) {
	srvConn, cli, cleanup := wsPair(t)
	defer cleanup()
	s := &Session{ID: "kalive", ws: srvConn}

	s.SendKeepalive()

	cli.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := cli.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "keepalive" {
		t.Errorf("msg.Type = %q want keepalive", msg.Type)
	}
	if msg.Session != "kalive" {
		t.Errorf("msg.Session = %q want kalive", msg.Session)
	}
}

// TestSessionCloseIdempotent verifies Close is idempotent and that a closed
// session no longer writes output/keepalive.
func TestSessionCloseIdempotent(t *testing.T) {
	srvConn, cli, cleanup := wsPair(t)
	defer cleanup()
	s := &Session{ID: "cl1", ws: srvConn, Conn: &mockConn{}}

	s.Close()
	s.Close() // second call must not panic

	if !s.closed.Load() {
		t.Error("session should be marked closed")
	}
	// Post-close writes must be no-ops (no message sent).
	s.WriteOutput([]byte("ignored"))
	s.SendKeepalive()

	cli.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, _, err := cli.ReadMessage(); err == nil {
		t.Error("expected no message after close, but one was received")
	}
}

// TestBroadcastToMultipleSessions simulates a broadcast by iterating the
// registry and writing to every session (there is no single Broadcast() API;
// the gateway composes broadcasts from registry.Range + WriteOutput).
func TestBroadcastToMultipleSessions(t *testing.T) {
	r := NewSessionRegistry(0)
	const n = 4
	clients := make([]*websocket.Conn, n)
	cleanups := make([]func(), 0, n)
	defer func() {
		for _, c := range cleanups {
			c()
		}
	}()
	for i := 0; i < n; i++ {
		srvConn, cli, cleanup := wsPair(t)
		cleanups = append(cleanups, cleanup)
		clients[i] = cli
		r.Register(&Session{ID: fmt.Sprintf("b%d", i), ws: srvConn})
	}

	// Broadcast a message to all sessions.
	r.sessions.Range(func(_, v interface{}) bool {
		v.(*Session).WriteOutput([]byte("broadcast"))
		return true
	})

	for i, cli := range clients {
		cli.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := cli.ReadMessage()
		if err != nil {
			t.Fatalf("client %d read: %v", i, err)
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("client %d unmarshal: %v", i, err)
		}
		if msg.Type != "data" {
			t.Errorf("client %d msg.Type = %q want data", i, msg.Type)
		}
	}
	if r.Count() != n {
		t.Errorf("registry Count = %d want %d", r.Count(), n)
	}
}

// TestStartKeepaliveDelivers verifies the registry's keepalive ticker pings
// every registered session.
func TestStartKeepaliveDelivers(t *testing.T) {
	r := NewSessionRegistry(20 * time.Millisecond)
	srvConn, cli, cleanup := wsPair(t)
	defer cleanup()
	r.Register(&Session{ID: "ka", ws: srvConn})

	r.StartKeepalive()

	received := make(chan struct{}, 1)
	go func() {
		cli.SetReadDeadline(time.Now().Add(2 * time.Second))
		for {
			_, data, err := cli.ReadMessage()
			if err != nil {
				return
			}
			var msg Message
			if json.Unmarshal(data, &msg) == nil && msg.Type == "keepalive" {
				select {
				case received <- struct{}{}:
				default:
				}
				return
			}
		}
	}()

	select {
	case <-received:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("no keepalive message received within 2s")
	}
}
