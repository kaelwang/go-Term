package protocol

import "time"

// ProtocolType enumerates the supported remote-access protocols.
type ProtocolType string

const (
	// ProtocolSSH is the Secure Shell protocol.
	ProtocolSSH ProtocolType = "ssh"
	// ProtocolTelnet is the legacy Telnet protocol.
	ProtocolTelnet ProtocolType = "telnet"
	// ProtocolVNC is the RFB/VNC protocol.
	ProtocolVNC ProtocolType = "vnc"
	// ProtocolLocalShell is a local pseudo-terminal on the server host.
	ProtocolLocalShell ProtocolType = "localshell"
)

// Credential holds the authentication material for a connection.
type Credential struct {
	// Type selects the authentication method:
	//   password | publickey | keyboard-interactive | agent
	Type string `json:"type"`
	// Username is the remote login name.
	Username string `json:"username"`
	// Password is the cleartext password (or keyboard-interactive answer).
	Password string `json:"password,omitempty"`
	// PrivateKey is a PEM-encoded private key (may be passphrase-encrypted).
	PrivateKey string `json:"private_key,omitempty"`
	// Passphrase decrypts PrivateKey when required.
	Passphrase string `json:"passphrase,omitempty"`
	// Answers are extra prompts answers, e.g. 2FA tokens / OTP.
	Answers []string `json:"answers,omitempty"`
}

// HopConfig describes a single jump host in a multi-hop connection chain.
type HopConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
	UseAgent   bool   `json:"use_agent"`
}

// ProxyConfig is a single SSH proxy / jump host used before the final target.
type ProxyConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
	UseAgent   bool   `json:"use_agent"`
}

// TunnelConfig describes a port-forwarding rule attached to an SSH session.
type TunnelConfig struct {
	// Type is one of: local | remote | dynamic.
	Type string `json:"type"`
	// LocalAddr is the bind address for local/dynamic forwarding,
	// e.g. 127.0.0.1:8080.
	LocalAddr string `json:"local_addr"`
	// RemoteAddr is the target for local forwarding, e.g. 127.0.0.1:80.
	RemoteAddr string `json:"remote_addr"`
}

// Connection is the full specification required to dial a remote endpoint.
type Connection struct {
	// ID is an opaque session identifier assigned by the gateway.
	ID string `json:"id"`
	// Protocol selects which Protocol implementation to use.
	Protocol ProtocolType `json:"protocol"`
	// Credential carries authentication data.
	Credential *Credential `json:"credential,omitempty"`
	// CredentialID references a saved vault credential by id. When set, the
	// gateway resolves and fills Credential at dial time (T-V3). It is a
	// string so it can be carried directly over the WS connect envelope.
	CredentialID string `json:"credential_id,omitempty"`
	// Host is the target hostname or IP (or empty for local shell).
	Host string `json:"host"`
	// Port is the target TCP port.
	Port int `json:"port"`
	// Timeout bounds the dial attempt.
	Timeout time.Duration `json:"timeout"`
	// KnownHostsPath points to the user's known_hosts file.
	KnownHostsPath string `json:"known_hosts_path"`
	// StrictHostKeyChecking enforces known_hosts verification.
	StrictHostKeyChecking bool `json:"strict_host_key_checking"`
	// SSHConfigHost, when set, refers to a Host alias defined in the user's
	// ~/.ssh/config. When dialing, the matching entry's HostName/Port/User/
	// IdentityFile/ProxyJump/StrictHostKeyChecking are applied, with any
	// explicitly supplied connection fields taking precedence.
	SSHConfigHost string `json:"ssh_config_host,omitempty"`
	// Proxy is a single jump host used before the target.
	Proxy *ProxyConfig `json:"proxy,omitempty"`
	// Hops is an ordered chain of jump hosts (connection hopping).
	Hops []*HopConfig `json:"hops,omitempty"`
	// Tunnel optionally requests a port-forwarding rule.
	Tunnel *TunnelConfig `json:"tunnel,omitempty"`
	// InitialCols / InitialRows are the requested PTY size.
	InitialCols int `json:"initial_cols"`
	InitialRows int `json:"initial_rows"`
	// Command, when set, is executed instead of launching an interactive shell.
	Command string `json:"command,omitempty"`
	// Env carries extra environment variables.
	Env map[string]string `json:"env,omitempty"`
}

// Clone returns a shallow copy of the connection with a new ID.
func (c *Connection) Clone() *Connection {
	cp := *c
	return &cp
}
