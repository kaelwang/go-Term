// Package localshell provides a local pseudo-terminal backed by the server
// host's default shell (bash/zsh on Unix, cmd.exe on Windows) via creack/pty.
// It satisfies protocol.Conn and is disabled when DISABLE_LOCAL_TERMINAL=1
// (enforced by the API middleware, not here).
package localshell

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/creack/pty"
	"github.com/kaelwang/go-Term/internal/protocol"
)

// LocalShell is the protocol.Protocol implementation for local terminals.
type LocalShell struct{}

// Dial satisfies protocol.Protocol.
func (LocalShell) Dial(conn *protocol.Connection) (protocol.Conn, error) {
	return Dial(conn)
}

type localConn struct {
	cmd *exec.Cmd
	f   *os.File
}

// Dial spawns a local PTY shell.
func Dial(conn *protocol.Connection) (protocol.Conn, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		if runtime.GOOS == "windows" {
			shell = "cmd.exe"
		} else {
			shell = "/bin/bash"
		}
	}
	c := exec.Command(shell)
	c.Env = append(os.Environ(), "TERM=xterm-256color")

	cols, rows := conn.InitialCols, conn.InitialRows
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	f, err := pty.StartWithSize(c, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", protocol.ErrConnectionFailed, err)
	}
	return &localConn{cmd: c, f: f}, nil
}

func (c *localConn) Read(p []byte) (int, error)  { return c.f.Read(p) }
func (c *localConn) Write(p []byte) (int, error) { return c.f.Write(p) }

func (c *localConn) Close() error {
	_ = c.f.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_, _ = c.cmd.Process.Wait()
	return nil
}

func (c *localConn) Resize(cols, rows int) error {
	return pty.Setsize(c.f, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
}

func (c *localConn) WindowChangeSupported() bool { return true }
