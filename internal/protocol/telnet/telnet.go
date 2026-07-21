// Package telnet implements a self-contained Telnet client (RFC 854) with
// minimal IAC negotiation and NAWS window-size support. It satisfies the
// protocol.Conn interface so the gateway can treat it like any terminal.
package telnet

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/kaelwang/go-Term/internal/protocol"
)

// IAC command/option constants (RFC 854 / 857 / 1073).
const (
	iac  = 255
	sb   = 250
	se   = 240
	will = 251
	wont = 252
	do   = 253
	dont = 254
	naws = 31
)

// Telnet is the protocol.Protocol implementation for Telnet.
type Telnet struct{}

// Dial satisfies protocol.Protocol.
func (Telnet) Dial(conn *protocol.Connection) (protocol.Conn, error) {
	return Dial(conn)
}

type telnetConn struct {
	conn   net.Conn
	reader *bufio.Reader
}

// Dial opens a Telnet connection and performs initial IAC negotiation.
func Dial(conn *protocol.Connection) (protocol.Conn, error) {
	addr := net.JoinHostPort(conn.Host, strconv.Itoa(conn.Port))
	c, err := net.DialTimeout("tcp", addr, dialTimeout(conn.Timeout))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", protocol.ErrConnectionFailed, err)
	}
	tc := &telnetConn{conn: c, reader: bufio.NewReader(c)}
	// Politely advertise NAWS support and refuse everything else.
	if conn.InitialCols > 0 {
		_ = tc.Resize(conn.InitialCols, conn.InitialRows)
	}
	return tc, nil
}

func dialTimeout(d time.Duration) time.Duration {
	if d <= 0 {
		return 15 * time.Second
	}
	return d
}

// Read strips IAC control sequences and returns application data.
func (c *telnetConn) Read(p []byte) (int, error) {
	out := make([]byte, 0, len(p))
	for len(out) < len(p) {
		b, err := c.reader.ReadByte()
		if err != nil {
			if len(out) > 0 {
				return copy(p, out), nil
			}
			return 0, err
		}
		if b != iac {
			out = append(out, b)
			continue
		}
		cmd, err := c.reader.ReadByte()
		if err != nil {
			if len(out) > 0 {
				return copy(p, out), nil
			}
			return 0, err
		}
		switch cmd {
		case iac:
			out = append(out, iac)
		case sb:
			c.skipSub()
		case will, wont, do, dont:
			opt, err := c.reader.ReadByte()
			if err != nil {
				if len(out) > 0 {
					return copy(p, out), nil
				}
				return 0, err
			}
			c.respond(cmd, opt)
		default:
			// IP, DM, BRK, AYT, EC, EL, GA - ignore.
		}
	}
	return copy(p, out), nil
}

func (c *telnetConn) respond(cmd, opt byte) {
	var resp byte
	switch cmd {
	case will:
		resp = dont
	case do:
		resp = wont
	default:
		return
	}
	_, _ = c.conn.Write([]byte{iac, resp, opt})
}

func (c *telnetConn) skipSub() {
	for {
		b, err := c.reader.ReadByte()
		if err != nil {
			return
		}
		if b == iac {
			n, err := c.reader.ReadByte()
			if err != nil {
				return
			}
			if n == se {
				return
			}
		}
	}
}

func (c *telnetConn) Write(p []byte) (int, error) {
	return c.conn.Write(p)
}

func (c *telnetConn) Close() error {
	return c.conn.Close()
}

// Resize sends a NAWS sub-negotiation reporting the new window size.
func (c *telnetConn) Resize(cols, rows int) error {
	if cols < 0 {
		cols = 0
	}
	if rows < 0 {
		rows = 0
	}
	buf := []byte{
		iac, sb, naws,
		byte(cols >> 8), byte(cols & 0xff),
		byte(rows >> 8), byte(rows & 0xff),
		iac, se,
	}
	_, err := c.conn.Write(buf)
	return err
}

func (c *telnetConn) WindowChangeSupported() bool { return true }
