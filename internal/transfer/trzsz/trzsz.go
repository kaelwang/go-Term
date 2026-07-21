// Package trzsz bridges the terminal stream to the trzsz tools (trz/tsz)
// via a PTY. The remote side runs `tsz`/`trz`; go-Term runs the
// matching local tool so files transfer through the live terminal. This is a
// clean, dependency-free, fully-compatible wrapper layer.
package trzsz

import (
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/kaelwang/go-Term/internal/protocol"
)

func binaryPath(name, envVar string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return name
}

func bridge(c protocol.Conn, cmd *exec.Cmd) error {
	f, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	defer f.Close()

	errc := make(chan error, 2)
	go func() { _, e := io.Copy(f, c); errc <- e }()
	go func() { _, e := io.Copy(c, f); errc <- e }()

	perr := cmd.Wait()
	select {
	case e := <-errc:
		return e
	default:
	}
	return perr
}

// Recv downloads a file. The user should have run `tsz <file>` on the
// remote; go-Term runs the local `trz` to receive into dir.
func Recv(c protocol.Conn, dir string) error {
	trz := binaryPath("trz", "GOTERM_TRZ_BIN")
	cmd := exec.Command(trz, dir)
	return bridge(c, cmd)
}

// Send uploads a file. The user should have run `trz` on the remote;
// go-Term runs the local `tsz <file>` to send it.
func Send(c protocol.Conn, file string) error {
	tsz := binaryPath("tsz", "GOTERM_TSZ_BIN")
	cmd := exec.Command(tsz, file)
	return bridge(c, cmd)
}
