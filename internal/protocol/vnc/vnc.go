// Package vnc implements an RFB (VNC) client handshake and tunnels the
// resulting framebuffer/control byte stream through the protocol.Conn
// interface. Authentication supports both the "None" and "VNC Auth" (DES
// challenge-response) security types.
package vnc

import (
	"bufio"
	"crypto/des"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/kaelwang/go-Term/internal/protocol"
)

// VNC is the protocol.Protocol implementation for VNC/RFB.
type VNC struct{}

// Dial satisfies protocol.Protocol.
func (VNC) Dial(conn *protocol.Connection) (protocol.Conn, error) {
	return Dial(conn)
}

type vncConn struct {
	conn   net.Conn
	reader *bufio.Reader
}

// Dial performs the RFB handshake and returns a streaming Conn.
func Dial(conn *protocol.Connection) (protocol.Conn, error) {
	addr := net.JoinHostPort(conn.Host, strconv.Itoa(conn.Port))
	c, err := net.DialTimeout("tcp", addr, dialTimeout(conn.Timeout))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", protocol.ErrConnectionFailed, err)
	}
	r := bufio.NewReader(c)

	if err := handshake(c, r, conn); err != nil {
		_ = c.Close()
		return nil, err
	}
	return &vncConn{conn: c, reader: r}, nil
}

func dialTimeout(d time.Duration) time.Duration {
	if d <= 0 {
		return 15 * time.Second
	}
	return d
}

// handshake runs the RFB 3.3/3.7/3.8 version + security negotiation.
func handshake(c net.Conn, r *bufio.Reader, conn *protocol.Connection) error {
	// 1. Server sends "RFB 003.00X\n".
	ver, err := r.ReadString('\n')
	if err != nil {
		return fmt.Errorf("%w: read version: %v", protocol.ErrConnectionFailed, err)
	}
	// 2. Reply with our supported version (3.8 when server supports it).
	reply := "RFB 003.008\n"
	if len(ver) >= 11 && ver[10] == '7' {
		reply = "RFB 003.007\n"
	} else if len(ver) >= 11 && ver[10] == '3' {
		reply = "RFB 003.003\n"
	}
	if _, err := c.Write([]byte(reply)); err != nil {
		return err
	}

	// 3. Security types: 1 byte count + N bytes.
	countByte, err := r.ReadByte()
	if err != nil {
		return err
	}
	count := int(countByte)
	if count == 0 {
		// Server sent a failure reason instead.
		reason, _ := io.ReadAll(io.LimitReader(r, 4096))
		return fmt.Errorf("%w: %s", protocol.ErrConnectionFailed, string(reason))
	}
	types := make([]byte, count)
	if _, err := io.ReadFull(r, types); err != nil {
		return err
	}

	// Choose "None" (1) if available, else "VNC Auth" (2).
	chosen := byte(0)
	for _, t := range types {
		if t == 1 {
			chosen = 1
			break
		}
	}
	if chosen == 0 {
		for _, t := range types {
			if t == 2 {
				chosen = 2
				break
			}
		}
	}
	if chosen == 0 {
		return fmt.Errorf("%w: no supported security type", protocol.ErrAuthFailed)
	}
	if _, err := c.Write([]byte{chosen}); err != nil {
		return err
	}

	// 4. Perform the chosen security handshake.
	if chosen == 1 {
		res := make([]byte, 4)
		if _, err := io.ReadFull(r, res); err != nil {
			return err
		}
		if binary.BigEndian.Uint32(res) != 0 {
			return fmt.Errorf("%w: vnc auth rejected", protocol.ErrAuthFailed)
		}
	} else {
		challenge := make([]byte, 16)
		if _, err := io.ReadFull(r, challenge); err != nil {
			return err
		}
		password := ""
		if conn.Credential != nil {
			password = conn.Credential.Password
		}
		resp, err := vncAuth(challenge, password)
		if err != nil {
			return err
		}
		if _, err := c.Write(resp); err != nil {
			return err
		}
		res := make([]byte, 4)
		if _, err := io.ReadFull(r, res); err != nil {
			return err
		}
		if binary.BigEndian.Uint32(res) != 0 {
			return fmt.Errorf("%w: vnc auth rejected", protocol.ErrAuthFailed)
		}
	}

	// 5. ClientInit (shared flag) + ServerInit.
	if _, err := c.Write([]byte{1}); err != nil {
		return err
	}
	// ServerInit: 2 bytes width, 2 bytes height, 16 bytes pixel format,
	// 4 bytes name length, then name. We consume it so the stream is clean.
	hdr := make([]byte, 24)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return err
	}
	nameLen := binary.BigEndian.Uint32(hdr[20:24])
	if nameLen > 0 {
		name := make([]byte, nameLen)
		if _, err := io.ReadFull(r, name); err != nil {
			return err
		}
	}
	return nil
}

// vncAuth computes the VNC DES challenge-response.
func vncAuth(challenge []byte, password string) ([]byte, error) {
	key := make([]byte, 8)
	pw := []byte(password)
	for i := 0; i < 8; i++ {
		if i < len(pw) {
			key[i] = reverseBits(pw[i])
		} else {
			key[i] = 0
		}
	}
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 16)
	block.Encrypt(out[0:8], challenge[0:8])
	block.Encrypt(out[8:16], challenge[8:16])
	return out, nil
}

// reverseBits reverses the bit order of a byte (per the VNC key schedule).
func reverseBits(b byte) byte {
	var r byte
	for i := 0; i < 8; i++ {
		r = (r << 1) | (b & 1)
		b >>= 1
	}
	return r
}

func (c *vncConn) Read(p []byte) (int, error)  { return c.reader.Read(p) }
func (c *vncConn) Write(p []byte) (int, error) { return c.conn.Write(p) }
func (c *vncConn) Close() error                { return c.conn.Close() }
func (c *vncConn) Resize(cols, rows int) error {
	return nil // RFB has its own framebuffer resize messages
}
func (c *vncConn) WindowChangeSupported() bool { return false }
