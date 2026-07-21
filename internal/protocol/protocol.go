// Package protocol defines the core abstractions shared by every supported
// remote-access protocol (SSH, Telnet, VNC, LocalShell) and the connection
// specification used to dial them.
package protocol

import (
	"errors"
	"fmt"
)

// Conn is a bidirectional, resizable byte stream that behaves like a terminal.
// Every protocol implementation (SSH session, raw Telnet socket, VNC RFB
// tunnel, local pty) must satisfy this interface so the gateway can treat them
// uniformly.
type Conn interface {
	// Read reads terminal/remote output into p.
	Read(p []byte) (int, error)
	// Write writes user input to the remote endpoint.
	Write(p []byte) (int, error)
	// Close terminates the underlying connection.
	Close() error
	// Resize changes the pseudo-terminal window size.
	Resize(cols, rows int) error
	// WindowChangeSupported reports whether Resize is meaningful.
	WindowChangeSupported() bool
}

// Protocol abstracts "how to establish a Conn for a given Connection spec".
type Protocol interface {
	Dial(conn *Connection) (Conn, error)
}

// registry maps a protocol type to its implementation.
var registry = map[ProtocolType]Protocol{}

// Register makes a Protocol implementation available under the given type.
func Register(t ProtocolType, p Protocol) {
	registry[t] = p
}

// GetProtocol returns the registered Protocol for t.
func GetProtocol(t ProtocolType) (Protocol, error) {
	if p, ok := registry[t]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedProtocol, t)
}

// Sentinel errors used across the codebase and surfaced to clients.
var (
	// ErrUnsupportedProtocol is returned when no implementation is registered.
	ErrUnsupportedProtocol = errors.New("unsupported protocol")
	// ErrAuthFailed is returned when the remote rejects credentials.
	ErrAuthFailed = errors.New("authentication failed")
	// ErrHostKeyMismatch is returned when a known host's key changed.
	ErrHostKeyMismatch = errors.New("host key mismatch")
	// ErrUnknownHostKey is returned when host key checking is strict and the
	// host is not yet trusted.
	ErrUnknownHostKey = errors.New("unknown host key")
	// ErrConnectionFailed is returned for low-level dial failures.
	ErrConnectionFailed = errors.New("connection failed")
)
