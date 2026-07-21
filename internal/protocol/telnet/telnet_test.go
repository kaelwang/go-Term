package telnet

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// fakeNetConn is a no-op net.Conn that records everything written to it.
type fakeNetConn struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (f *fakeNetConn) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.Write(p)
}
func (f *fakeNetConn) Bytes() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]byte(nil), f.buf.Bytes()...)
}
func (f *fakeNetConn) Read(p []byte) (int, error)    { return 0, io.EOF }
func (f *fakeNetConn) Close() error                  { return nil }
func (f *fakeNetConn) LocalAddr() net.Addr           { return nil }
func (f *fakeNetConn) RemoteAddr() net.Addr          { return nil }
func (f *fakeNetConn) SetDeadline(time.Time) error   { return nil }
func (f *fakeNetConn) SetReadDeadline(time.Time) error   { return nil }
func (f *fakeNetConn) SetWriteDeadline(time.Time) error { return nil }

var _ net.Conn = (*fakeNetConn)(nil)

// TestReadStripsIAC verifies Telnet IAC negotiation sequences are removed from
// the application data stream, while an escaped IAC (255 255) becomes a single
// literal 255.
func TestReadStripsIAC(t *testing.T) {
	input := []byte{
		'A',
		iac, will, naws, // negotiate NAWS -> stripped
		'B',
		iac, iac, // escaped IAC -> literal 255
		'C',
	}
	c := &telnetConn{
		reader: bufio.NewReader(bytes.NewReader(input)),
		conn:   &fakeNetConn{},
	}
	out := make([]byte, 16)
	n, err := c.Read(out)
	if err != nil && err != io.EOF {
		t.Fatalf("Read error: %v", err)
	}
	got := out[:n]
	want := []byte{'A', 'B', iac, 'C'}
	if !bytes.Equal(got, want) {
		t.Fatalf("stripped output = %v want %v", got, want)
	}
}

// TestReadPlainData verifies ordinary data passes through unchanged.
func TestReadPlainData(t *testing.T) {
	c := &telnetConn{reader: bufio.NewReader(bytes.NewReader([]byte("hello"))), conn: &fakeNetConn{}}
	out := make([]byte, 16)
	n, _ := c.Read(out)
	if string(out[:n]) != "hello" {
		t.Fatalf("plain read = %q want hello", out[:n])
	}
}

// TestRespondRefusesOptions verifies the client refuses WILL/DO options with
// the appropriate DONT/WONT response (RFC 854).
func TestRespondRefusesOptions(t *testing.T) {
	conn := &fakeNetConn{}
	c := &telnetConn{conn: conn}

	c.respond(will, naws)
	if got := conn.Bytes(); !bytes.Equal(got, []byte{iac, dont, naws}) {
		t.Errorf("respond(will,naws) = %v want %v", got, []byte{iac, dont, naws})
	}

	conn2 := &fakeNetConn{}
	c2 := &telnetConn{conn: conn2}
	c2.respond(do, naws)
	if got := conn2.Bytes(); !bytes.Equal(got, []byte{iac, wont, naws}) {
		t.Errorf("respond(do,naws) = %v want %v", got, []byte{iac, wont, naws})
	}
}

// TestResizeNAWS verifies the NAWS sub-negotiation is encoded correctly.
func TestResizeNAWS(t *testing.T) {
	conn := &fakeNetConn{}
	c := &telnetConn{conn: conn}
	c.Resize(80, 24)
	want := []byte{iac, sb, naws, 0x00, 0x50, 0x00, 0x18, iac, se}
	if got := conn.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("Resize(80,24) = %v want %v", got, want)
	}
}

// TestWindowChangeSupported verifies VNC-like capability reporting.
func TestWindowChangeSupported(t *testing.T) {
	c := &telnetConn{conn: &fakeNetConn{}}
	if !c.WindowChangeSupported() {
		t.Error("telnet should report WindowChangeSupported=true")
	}
}
