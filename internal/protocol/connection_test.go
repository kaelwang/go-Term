package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

// TestConnectionJSONAlignment verifies the Go Connection struct serializes to
// the exact snake_case field names expected by the frontend ConnectionSpec
// (frontend/src/types.ts), and that values round-trip without loss.
func TestConnectionJSONAlignment(t *testing.T) {
	conn := &Connection{
		ID:       "sess-1",
		Protocol: ProtocolSSH,
		Host:     "192.168.1.10",
		Port:     2222,
		Timeout:  30 * time.Second,
		Credential: &Credential{
			Type:       "password",
			Username:   "alice",
			Password:   "s3cr3t",
			PrivateKey: "PEM-KEY",
			Passphrase: "pp",
			Answers:    []string{"otp-123"},
		},
		KnownHostsPath:           "~/.ssh/known_hosts",
		StrictHostKeyChecking:    true,
		Proxy:                    &ProxyConfig{Host: "jump", Port: 22, Username: "proxyuser", UseAgent: true},
		Hops:                     []*HopConfig{{Host: "hop1", Port: 22, Username: "hopuser"}},
		Tunnel:                   &TunnelConfig{Type: "local", LocalAddr: "127.0.0.1:8080", RemoteAddr: "127.0.0.1:80"},
		InitialCols:              120,
		InitialRows:              40,
		Command:                  "uptime",
		Env:                      map[string]string{"TERM": "xterm"},
	}

	data, err := json.Marshal(conn)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Top-level fields must match the frontend ConnectionSpec names.
	wantTop := []string{
		"id", "protocol", "host", "port", "credential", "known_hosts_path",
		"strict_host_key_checking", "proxy", "hops", "tunnel",
		"initial_cols", "initial_rows", "command", "env",
	}
	for _, k := range wantTop {
		if _, ok := m[k]; !ok {
			t.Errorf("missing expected JSON field %q (frontend expects it)", k)
		}
	}
	if m["protocol"] != "ssh" {
		t.Errorf("protocol = %v want ssh", m["protocol"])
	}
	if int(m["port"].(float64)) != 2222 {
		t.Errorf("port = %v want 2222", m["port"])
	}
	if m["strict_host_key_checking"] != true {
		t.Errorf("strict_host_key_checking = %v want true", m["strict_host_key_checking"])
	}

	// Credential sub-fields must use the frontend names (private_key, etc.).
	cred := m["credential"].(map[string]interface{})
	for _, k := range []string{"type", "username", "password", "private_key", "passphrase", "answers"} {
		if _, ok := cred[k]; !ok {
			t.Errorf("credential missing field %q", k)
		}
	}
	if cred["username"] != "alice" || cred["private_key"] != "PEM-KEY" {
		t.Errorf("credential sub-fields wrong: %+v", cred)
	}

	// Proxy / Hop / Tunnel field names.
	proxy := m["proxy"].(map[string]interface{})
	if proxy["host"] != "jump" || proxy["use_agent"] != true {
		t.Errorf("proxy fields wrong: %+v", proxy)
	}
	hops := m["hops"].([]interface{})
	if len(hops) != 1 {
		t.Fatalf("hops length = %d want 1", len(hops))
	}
	tunnel := m["tunnel"].(map[string]interface{})
	if tunnel["type"] != "local" || tunnel["local_addr"] != "127.0.0.1:8080" {
		t.Errorf("tunnel fields wrong: %+v", tunnel)
	}

	// Round-trip: unmarshal back must preserve the values.
	var back Connection
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal back: %v", err)
	}
	if back.ID != conn.ID || back.Host != conn.Host || back.Port != conn.Port {
		t.Errorf("round-trip top-level mismatch: %+v", back)
	}
	if back.Credential == nil || back.Credential.Username != "alice" || back.Credential.PrivateKey != "PEM-KEY" {
		t.Errorf("round-trip credential mismatch: %+v", back.Credential)
	}
	if back.Proxy == nil || back.Proxy.Host != "jump" || !back.Proxy.UseAgent {
		t.Errorf("round-trip proxy mismatch: %+v", back.Proxy)
	}
	if len(back.Hops) != 1 || back.Hops[0].Host != "hop1" {
		t.Errorf("round-trip hops mismatch: %+v", back.Hops)
	}
	if back.Tunnel == nil || back.Tunnel.LocalAddr != "127.0.0.1:8080" {
		t.Errorf("round-trip tunnel mismatch: %+v", back.Tunnel)
	}
	if back.InitialCols != 120 || back.InitialRows != 40 || back.Command != "uptime" {
		t.Errorf("round-trip pty/command mismatch")
	}
}

// TestClone verifies Clone produces an independent copy.
func TestClone(t *testing.T) {
	c := &Connection{Host: "h", Port: 22}
	cp := c.Clone()
	cp.Host = "changed"
	if c.Host != "h" {
		t.Error("Clone did not produce an independent copy")
	}
}
