package vnc

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/kaelwang/go-Term/internal/protocol"
)

// scriptedConn is a net.Conn that serves a fixed byte script on Read and
// records everything written to it, allowing the RFB handshake to be exercised
// purely (no real VNC server).
type scriptedConn struct {
	mu  sync.Mutex
	r   *bytes.Reader
	w   bytes.Buffer
}

func newScripted(b []byte) *scriptedConn {
	return &scriptedConn{r: bytes.NewReader(b)}
}
func (s *scriptedConn) Read(p []byte) (int, error)    { return s.r.Read(p) }
func (s *scriptedConn) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}
func (s *scriptedConn) Written() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.w.Bytes()...)
}
func (s *scriptedConn) Close() error                 { return nil }
func (s *scriptedConn) LocalAddr() net.Addr          { return nil }
func (s *scriptedConn) RemoteAddr() net.Addr         { return nil }
func (s *scriptedConn) SetDeadline(time.Time) error  { return nil }
func (s *scriptedConn) SetReadDeadline(time.Time) error  { return nil }
func (s *scriptedConn) SetWriteDeadline(time.Time) error { return nil }

var _ net.Conn = (*scriptedConn)(nil)

// serverInit builds the 24-byte ServerInit block (name length = 0).
func serverInit() []byte {
	hdr := make([]byte, 24)
	binary.BigEndian.PutUint32(hdr[20:24], 0) // name length 0
	return hdr
}

// runHandshake feeds a scripted server response through handshake and returns
// the bytes the client wrote plus any error.
func runHandshake(t *testing.T, script []byte, conn *protocol.Connection) ([]byte, error) {
	t.Helper()
	sc := newScripted(script)
	err := handshake(sc, bufio.NewReader(sc), conn)
	return sc.Written(), err
}

// TestHandshakeVersionNegotiation verifies the RFB version string parsing and
// the correct reply version for 3.8 / 3.7 / 3.3.
func TestHandshakeVersionNegotiation(t *testing.T) {
	cases := []struct {
		serverVer string
		wantReply string
	}{
		{"RFB 003.008\n", "RFB 003.008\n"},
		{"RFB 003.007\n", "RFB 003.007\n"},
		{"RFB 003.003\n", "RFB 003.003\n"},
	}
	for _, tc := range cases {
		t.Run(tc.serverVer, func(t *testing.T) {
			script := append([]byte(tc.serverVer), 0x01, 0x01) // count=1, type=None
			script = append(script, 0, 0, 0, 0)                // auth result OK
			script = append(script, serverInit()...)           // serverinit (no name)
			written, err := runHandshake(t, script, &protocol.Connection{})
			if err != nil {
				t.Fatalf("handshake error: %v", err)
			}
			if !bytes.HasPrefix(written, []byte(tc.wantReply)) {
				t.Errorf("reply = %q want prefix %q", written[:len(tc.wantReply)], tc.wantReply)
			}
		})
	}
}

// TestHandshakeNoneSecurity verifies the "None" security type is selected and
// the full handshake completes successfully.
func TestHandshakeNoneSecurity(t *testing.T) {
	script := append([]byte("RFB 003.008\n"), 0x01, 0x01) // count=1, type=None(1)
	script = append(script, 0, 0, 0, 0)                  // auth result OK
	script = append(script, serverInit()...)
	written, err := runHandshake(t, script, &protocol.Connection{})
	if err != nil {
		t.Fatalf("handshake error: %v", err)
	}
	// Written bytes: version reply (12) + chosen type (1) + clientinit (1).
	if len(written) != 14 {
		t.Fatalf("written length = %d want 14 (reply+chosen+clientinit)", len(written))
	}
	if written[12] != 0x01 {
		t.Errorf("chosen security type = %#x want 0x01 (None)", written[12])
	}
	if written[13] != 0x01 {
		t.Errorf("clientinit shared flag = %#x want 0x01", written[13])
	}
}

// TestHandshakeVNCAuth verifies that when only "VNC Auth" (2) is offered, it is
// selected, the challenge-response is computed, and the handshake completes.
func TestHandshakeVNCAuth(t *testing.T) {
	challenge := make([]byte, 16) // all zeros
	script := append([]byte("RFB 003.008\n"), 0x01, 0x02) // count=1, type VNC Auth(2)
	script = append(script, challenge...)                 // 16-byte challenge
	script = append(script, 0, 0, 0, 0)                  // auth result OK
	script = append(script, serverInit()...)

	conn := &protocol.Connection{Credential: &protocol.Credential{Password: "pw"}}
	written, err := runHandshake(t, script, conn)
	if err != nil {
		t.Fatalf("handshake error: %v", err)
	}
	// Written: reply(12) + chosen(1) + 16-byte response + clientinit(1) = 30.
	if len(written) != 30 {
		t.Fatalf("written length = %d want 30", len(written))
	}
	if written[12] != 0x02 {
		t.Errorf("chosen security type = %#x want 0x02 (VNC Auth)", written[12])
	}
	wantResp, rerr := vncAuth(challenge, "pw")
	if rerr != nil {
		t.Fatalf("vncAuth: %v", rerr)
	}
	if !bytes.Equal(written[13:29], wantResp) {
		t.Errorf("VNC auth response mismatch:\n got %v\nwant %v", written[13:29], wantResp)
	}
}

// TestHandshakeServerRejects verifies a server failure (count byte 0 + reason)
// surfaces as an error carrying the reason text.
func TestHandshakeServerRejects(t *testing.T) {
	script := append([]byte("RFB 003.008\n"), 0x00) // count 0 => failure
	script = append(script, []byte("access denied")...)
	_, err := runHandshake(t, script, &protocol.Connection{})
	if err == nil {
		t.Fatal("expected handshake error on server rejection, got nil")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("access denied")) {
		t.Errorf("error should contain server reason, got %v", err)
	}
}

// TestReverseBits is a sanity check on the VNC key schedule helper.
func TestReverseBits(t *testing.T) {
	if reverseBits(0x00) != 0x00 {
		t.Error("reverseBits(0) != 0")
	}
	if reverseBits(0xFF) != 0xFF {
		t.Error("reverseBits(0xFF) != 0xFF")
	}
	if reverseBits(0x01) != 0x80 {
		t.Errorf("reverseBits(0x01) = %#x want 0x80", reverseBits(0x01))
	}
	if reverseBits(0x80) != 0x01 {
		t.Errorf("reverseBits(0x80) = %#x want 0x01", reverseBits(0x80))
	}
}

// ensure io is referenced (kept for clarity of error handling imports).
var _ = io.EOF
