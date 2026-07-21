// Package ssh implements the SSH protocol: dialing, PTY sessions, command
// execution, host-key handling, SSH-agent, public-key and keyboard-interactive
// (2FA) authentication, jump-host hopping and local/remote/dynamic tunneling.
package ssh

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kaelwang/go-Term/internal/protocol"
	"github.com/kaelwang/go-Term/internal/security"
	cryptossh "golang.org/x/crypto/ssh"
)

// SSH is the protocol.Protocol implementation for Secure Shell.
type SSH struct{}

// Dial satisfies protocol.Protocol.
func (SSH) Dial(conn *protocol.Connection) (protocol.Conn, error) {
	return Dial(conn)
}

// sshConn adapts an *ssh.Session (and its client) to the protocol.Conn
// interface used by the gateway.
type sshConn struct {
	client       *cryptossh.Client
	extraClients []*cryptossh.Client
	session      *cryptossh.Session
	stdin        io.WriteCloser
	reader       io.Reader
	pty          bool
}

func (c *sshConn) Read(p []byte) (int, error)  { return c.reader.Read(p) }
func (c *sshConn) Write(p []byte) (int, error) { return c.stdin.Write(p) }

func (c *sshConn) Close() error {
	_ = c.stdin.Close()
	_ = c.session.Close()
	_ = c.client.Close()
	for _, ec := range c.extraClients {
		_ = ec.Close()
	}
	return nil
}

func (c *sshConn) Resize(cols, rows int) error {
	if !c.pty {
		return nil
	}
	return c.session.WindowChange(rows, cols)
}

func (c *sshConn) WindowChangeSupported() bool { return c.pty }

// makeHostKeyCallback returns a crypto/ssh HostKeyCallback that consults the
// known_hosts file. When strict checking is disabled the host key is accepted
// on first use (TOFU) and persisted.
func makeHostKeyCallback(conn *protocol.Connection) cryptossh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key cryptossh.PublicKey) error {
		res, err := security.CheckHostKey(conn.KnownHostsPath, hostname, key)
		if err != nil {
			return err
		}
		if res.Trusted {
			return nil
		}
		if !conn.StrictHostKeyChecking {
			return security.AddHostKey(conn.KnownHostsPath, hostname, key)
		}
		if !res.Known {
			return fmt.Errorf("%w: %s", protocol.ErrUnknownHostKey, hostname)
		}
		return fmt.Errorf("%w: %s", protocol.ErrHostKeyMismatch, hostname)
	}
}

// baseConfig builds the shared *ssh.ClientConfig for the target connection.
func baseConfig(conn *protocol.Connection) (*cryptossh.ClientConfig, error) {
	if conn.Credential == nil {
		conn.Credential = &protocol.Credential{Username: "root"}
	}
	if conn.Credential.Username == "" {
		conn.Credential.Username = "root"
	}
	auth, err := authMethods(conn.Credential, conn.Proxy != nil && conn.Proxy.UseAgent)
	if err != nil {
		return nil, err
	}
	cfg := &cryptossh.ClientConfig{
		User:            conn.Credential.Username,
		Auth:            auth,
		HostKeyCallback: makeHostKeyCallback(conn),
		Timeout:         conn.Timeout,
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	return cfg, nil
}

// DialRaw establishes the underlying *ssh.Client (and any hidden hop clients)
// without opening a shell. Transfer protocols (SFTP) use this.
func DialRaw(conn *protocol.Connection) (*cryptossh.Client, []*cryptossh.Client, error) {
	cfg, err := baseConfig(conn)
	if err != nil {
		return nil, nil, err
	}

	var client *cryptossh.Client
	if len(conn.Hops) > 0 {
		client, err = dialHops(conn, cfg)
	} else if conn.Proxy != nil {
		client, err = dialProxy(conn, cfg)
	} else {
		addr := net.JoinHostPort(conn.Host, strconv.Itoa(conn.Port))
		client, err = cryptossh.Dial("tcp", addr, cfg)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", protocol.ErrConnectionFailed, err)
	}
	extra := consumeExtra(client)
	return client, extra, nil
}

// Dial establishes an SSH connection honoring proxies, hops and tunnels.
func Dial(conn *protocol.Connection) (protocol.Conn, error) {
	// Apply any ~/.ssh/config Host alias before resolving the final target.
	applySSHConfig(conn)

	client, extra, err := DialRaw(conn)
	if err != nil {
		return nil, err
	}

	if conn.Tunnel != nil {
		if e := startTunnel(client, conn.Tunnel); e != nil {
			_ = client.Close()
			for _, ec := range extra {
				_ = ec.Close()
			}
			return nil, fmt.Errorf("%w: tunnel: %v", protocol.ErrConnectionFailed, e)
		}
	}

	return newSession(client, conn, extra)
}

// applySSHConfig augments conn with values resolved from a ~/.ssh/config
// Host alias referenced by conn.SSHConfigHost. Explicitly supplied fields on
// conn always take precedence over the alias (F2 / A6): only empty fields are
// filled from the configuration entry.
func applySSHConfig(conn *protocol.Connection) {
	if conn == nil || conn.SSHConfigHost == "" {
		return
	}
	entry := ResolveSSHConfig(conn.SSHConfigHost)
	if entry == nil {
		return
	}

	// HostName.
	if conn.Host == "" && entry.HostName != "" {
		conn.Host = entry.HostName
	}
	// Port.
	if conn.Port == 0 && entry.Port != 0 {
		conn.Port = entry.Port
	}
	// User.
	if conn.Credential == nil {
		conn.Credential = &protocol.Credential{}
	}
	if conn.Credential.Username == "" && entry.User != "" {
		conn.Credential.Username = entry.User
	}
	// IdentityFile -> read the server-side private key into the credential
	// (A5: keys are never uploaded from the browser).
	if entry.IdentityFile != "" && conn.Credential.PrivateKey == "" {
		if data, err := os.ReadFile(entry.IdentityFile); err == nil {
			conn.Credential.PrivateKey = string(data)
			if conn.Credential.Type == "" {
				conn.Credential.Type = "publickey"
			}
		}
	}
	// StrictHostKeyChecking. Only enforce "yes/ask/accept-new" when the
	// connection has not already opted in, preserving an explicit user choice.
	if entry.StrictHostKeyChecking != "" && !conn.StrictHostKeyChecking {
		conn.StrictHostKeyChecking = !isStrictHostKeyCheckingOff(entry.StrictHostKeyChecking)
	}
	// ProxyJump -> single ProxyConfig (a,b) or ordered Hops (A6).
	if entry.ProxyJump != "" {
		if strings.Contains(entry.ProxyJump, ",") {
			if conn.Hops == nil {
				conn.Hops = buildHopsFromProxyJump(entry.ProxyJump)
			}
		} else if conn.Proxy == nil {
			h, port, user := parseProxyJumpToken(entry.ProxyJump)
			conn.Proxy = &protocol.ProxyConfig{Host: h, Port: port, Username: user}
		}
	}
}

// isStrictHostKeyCheckingOff reports whether an OpenSSH StrictHostKeyChecking
// directive means "do not enforce" (no/off/false).
func isStrictHostKeyCheckingOff(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "no", "off", "false":
		return true
	default:
		return false
	}
}

// parseProxyJumpToken parses a single ProxyJump token of the form
// [user@]host[:port] into its components.
func parseProxyJumpToken(tok string) (host string, port int, user string) {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return "", 22, ""
	}
	if at := strings.Index(tok, "@"); at >= 0 {
		user = tok[:at]
		tok = tok[at+1:]
	}
	if colon := strings.LastIndex(tok, ":"); colon >= 0 {
		if p, err := strconv.Atoi(tok[colon+1:]); err == nil {
			port = p
			tok = tok[:colon]
		}
	}
	host = tok
	if port == 0 {
		port = 22
	}
	return host, port, user
}

// buildHopsFromProxyJump splits a comma-separated ProxyJump chain into an
// ordered list of HopConfig entries (inner jump first, outer last).
func buildHopsFromProxyJump(jump string) []*protocol.HopConfig {
	parts := strings.Split(jump, ",")
	hops := make([]*protocol.HopConfig, 0, len(parts))
	for _, p := range parts {
		h, port, user := parseProxyJumpToken(p)
		hops = append(hops, &protocol.HopConfig{Host: h, Port: port, Username: user})
	}
	return hops
}

// newSession opens a shell (or runs a command) on an established client and
// adapts it to protocol.Conn.
func newSession(client *cryptossh.Client, conn *protocol.Connection, extra []*cryptossh.Client) (protocol.Conn, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", protocol.ErrConnectionFailed, err)
	}

	// Combine stdout and stderr into a single reader for terminal streaming.
	pr, pw := io.Pipe()
	session.Stdout = pw
	session.Stderr = pw

	c := &sshConn{
		client:       client,
		extraClients: extra,
		session:      session,
		reader:       pr,
		pty:          conn.InitialCols > 0,
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	c.stdin = stdin

	if c.pty {
		modes := cryptossh.TerminalModes{
			cryptossh.ECHO:          1,
			cryptossh.TTY_OP_ISPEED: 14400,
			cryptossh.TTY_OP_OSPEED: 14400,
		}
		cols, rows := conn.InitialCols, conn.InitialRows
		if cols <= 0 {
			cols = 80
		}
		if rows <= 0 {
			rows = 24
		}
		if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
			_ = session.Close()
			return nil, err
		}
	}

	if conn.Command != "" {
		if err := session.Start(conn.Command); err != nil {
			_ = session.Close()
			return nil, err
		}
	} else {
		if err := session.Shell(); err != nil {
			_ = session.Close()
			return nil, err
		}
	}

	// When the remote shell/command exits, close the pipe so the reader
	// receives EOF.
	go func() {
		_ = session.Wait()
		_ = pw.Close()
	}()

	return c, nil
}
