package gateway

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/protocol"
)

// transferStatusPayload mirrors the JSON shape emitted by emitTransferStatus.
type transferStatusPayload struct {
	Protocol  string `json:"protocol"`
	Direction string `json:"direction"`
	Status    string `json:"status"`
	Error     string `json:"error"`
	Path      string `json:"path"`
}

// readTransferStatus reads messages from the client until a transfer_status
// envelope with the given status is seen, or the deadline elapses.
func readTransferStatus(t *testing.T, cli interface {
	ReadMessage() (int, []byte, error)
	SetReadDeadline(time.Time) error
}, wantStatus string) *transferStatusPayload {
	t.Helper()
	cli.SetReadDeadline(time.Now().Add(2 * time.Second))
	for i := 0; i < 4; i++ {
		_, data, err := cli.ReadMessage()
		if err != nil {
			t.Fatalf("read msg %d: %v", i, err)
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal msg %d: %v", i, err)
		}
		if msg.Type != "transfer_status" {
			continue
		}
		var p transferStatusPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if p.Status == wantStatus {
			return &p
		}
	}
	return nil
}

// TestRunTransferEmitsStatusOnUnknownProtocol verifies requirement (a): an
// unknown transfer protocol is reported back to the client as a
// transfer_status{status:"error"} envelope carrying the dispatcher error
// (F1 / A4). The single-owner gateMu is exercised by RunExclusive and the
// message is delivered via WriteMessage.
func TestRunTransferEmitsStatusOnUnknownProtocol(t *testing.T) {
	srvConn, cli, cleanup := wsPair(t)
	defer cleanup()

	s := &Session{ID: "rt1", ws: srvConn, Conn: &mockConn{}}
	s.RunTransfer("bogus", "send", "/tmp/webssh-nonexistent")

	// A "running" envelope is emitted first, then "error".
	running := readTransferStatus(t, cli, "running")
	if running == nil {
		t.Fatal("expected a transfer_status 'running' envelope first")
	}
	errStatus := readTransferStatus(t, cli, "error")
	if errStatus == nil {
		t.Fatal("expected a transfer_status 'error' envelope")
	}
	if !strings.Contains(errStatus.Error, "unsupported transfer protocol") {
		t.Errorf("error status = %q want it to mention 'unsupported transfer protocol'", errStatus.Error)
	}
	if errStatus.Protocol != "bogus" || errStatus.Direction != "send" {
		t.Errorf("status envelope metadata = {%q,%q} want {bogus,send}",
			errStatus.Protocol, errStatus.Direction)
	}
}

// TestRunTransferDispatchUnknownDirection verifies the direction dispatcher
// rejects an unknown direction with a clear error (no ws needed here because
// runTransfer returns before touching WriteMessage).
func TestRunTransferDispatchUnknownDirection(t *testing.T) {
	s := &Session{ID: "rt2", Conn: &mockConn{}}
	_, err := s.runTransfer("trzsz", "frobnicate", "")
	if err == nil || !strings.Contains(err.Error(), "unknown transfer direction") {
		t.Fatalf("expected 'unknown transfer direction' error, got %v", err)
	}
}

// TestRunTransferSendUnsupportedProtocol verifies the protocol branch in
// runTransferSend returns "unsupported transfer protocol" for an unknown kind.
func TestRunTransferSendUnsupportedProtocol(t *testing.T) {
	s := &Session{ID: "rt3", Conn: &mockConn{}}
	_, err := s.runTransferSend("telnetx", "/tmp/x")
	if err == nil || !strings.Contains(err.Error(), "unsupported transfer protocol") {
		t.Fatalf("expected 'unsupported transfer protocol' error, got %v", err)
	}
}

// TestRunTransferRecvUnsupportedProtocol mirrors the recv-side dispatcher
// branch check. runTransferRecv reads config.Global.DownloadDir up front,
// so we mirror the production reality that config.Load() has run.
func TestRunTransferRecvUnsupportedProtocol(t *testing.T) {
	config.Global = &config.Config{DownloadDir: t.TempDir()}
	s := &Session{ID: "rt4", Conn: &mockConn{}}
	_, err := s.runTransferRecv("telnetx", "")
	if err == nil || !strings.Contains(err.Error(), "unsupported transfer protocol") {
		t.Fatalf("expected 'unsupported transfer protocol' error, got %v", err)
	}
}

// TestRunTransferRecvNilConfig verifies the nil-guard on config.Global: a nil
// config yields a clear error instead of panicking on DownloadDir (T-NIL / B6).
func TestRunTransferRecvNilConfig(t *testing.T) {
	orig := config.Global
	config.Global = nil
	defer func() { config.Global = orig }()
	s := &Session{ID: "rt-nil", Conn: &mockConn{}}
	_, err := s.runTransferRecv("trzsz", "")
	if err == nil {
		t.Fatal("expected error when config.Global is nil")
	}
}

// TestRunExclusiveSetsTransferringFlag documents the single-owner invariant:
// gateMu is held and transferring is true for the whole duration of the
// exclusive callback, then released (so the normal pump resumes) (F1 / A3).
func TestRunExclusiveSetsTransferringFlag(t *testing.T) {
	s := &Session{}
	var inside bool
	s.RunExclusive(func() {
		inside = s.transferring.Load()
	})
	if !inside {
		t.Error("transferring should be true while the exclusive fn runs")
	}
	if s.transferring.Load() {
		t.Error("transferring should be false after RunExclusive returns")
	}
	// The fakeConn must satisfy protocol.Conn (compile-time guard kept here
	// so a future signature change breaks the build loudly).
	var _ protocol.Conn = (*mockConn)(nil)
}
