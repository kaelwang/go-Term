package ssh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaelwang/go-Term/internal/protocol"
)

// withSSHConfig writes a temporary ~/.ssh/config rooted at a temp dir and
// points os.UserHomeDir() at that temp dir (via USERPROFILE on Windows, the
// variable os.UserHomeDir consults first). This lets applySSHConfig /
// ResolveSSHConfig / ListSSHConfigHosts be exercised deterministically without
// touching the developer's real home directory.
func withSSHConfig(t *testing.T, content string) string {
	t.Helper()
	tmp := t.TempDir()
	sshDir := filepath.Join(tmp, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	// os.UserHomeDir() on Windows consults USERPROFILE (after HOMEDRIVE/
	// HOMEPATH). Clear those so USERPROFILE is authoritative.
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	return tmp
}

// TestParseSSHConfig verifies every directive we care about is parsed into the
// structured entry (F2 / A6).
func TestParseSSHConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg")
	cfg := `
Host web1
  HostName web1.example.com
  Port 2201
  User webuser
  IdentityFile /etc/keys/web1
  ProxyJump jump.web
  StrictHostKeyChecking ask

Host web2
  HostName web2.example.com
`
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	entries := ParseSSHConfig(path)
	if len(entries) != 2 {
		t.Fatalf("parsed %d hosts want 2 (%v)", len(entries), entries)
	}
	e1, ok := entries["web1"]
	if !ok || e1 == nil {
		t.Fatal("missing host web1")
	}
	if e1.HostName != "web1.example.com" {
		t.Errorf("HostName = %q want web1.example.com", e1.HostName)
	}
	if e1.Port != 2201 {
		t.Errorf("Port = %d want 2201", e1.Port)
	}
	if e1.User != "webuser" {
		t.Errorf("User = %q want webuser", e1.User)
	}
	if e1.IdentityFile != "/etc/keys/web1" {
		t.Errorf("IdentityFile = %q want /etc/keys/web1", e1.IdentityFile)
	}
	if e1.ProxyJump != "jump.web" {
		t.Errorf("ProxyJump = %q want jump.web", e1.ProxyJump)
	}
	if e1.StrictHostKeyChecking != "ask" {
		t.Errorf("StrictHostKeyChecking = %q want ask", e1.StrictHostKeyChecking)
	}
	if e2 := entries["web2"]; e2 == nil || e2.HostName != "web2.example.com" {
		t.Errorf("web2 mismatch: %+v", e2)
	}
}

// TestListSSHConfigHosts verifies the endpoint data source returns every
// declared alias, sorted (F2).
func TestListSSHConfigHosts(t *testing.T) {
	withSSHConfig(t, `
Host zeta
  HostName zeta.example.com
Host alpha
  HostName alpha.example.com
Host mike
  HostName mike.example.com
`)
	hosts := ListSSHConfigHosts()
	want := []string{"alpha", "mike", "zeta"}
	if len(hosts) != len(want) {
		t.Fatalf("hosts = %v want %v", hosts, want)
	}
	for i := range want {
		if hosts[i] != want[i] {
			t.Errorf("hosts[%d] = %q want %q", i, hosts[i], want[i])
		}
	}
}

// TestResolveSSHConfigGlob verifies wildcard Host patterns are matched, not
// just exact names (F2 / A6).
func TestResolveSSHConfigGlob(t *testing.T) {
	withSSHConfig(t, "Host *.internal\n  HostName resolved.internal\n  Port 9999\n")
	e := ResolveSSHConfig("host1.internal")
	if e == nil {
		t.Fatal("glob *.internal should match host1.internal")
	}
	if e.HostName != "resolved.internal" || e.Port != 9999 {
		t.Errorf("resolved entry = %+v", e)
	}
	if got := ResolveSSHConfig("no.match"); got != nil {
		t.Errorf("non-matching host should resolve nil, got %+v", got)
	}
}

// TestApplySSHConfigFillsFromAlias verifies that when the connection leaves
// fields empty, the alias values are applied (HostName/Port/User/ProxyJump/
// StrictHostKeyChecking) (F2 / A6).
func TestApplySSHConfigFillsFromAlias(t *testing.T) {
	withSSHConfig(t, `
Host alias
  HostName alias.host
  Port 2222
  User aliased
  ProxyJump jump.example.com
  StrictHostKeyChecking yes
`)
	conn := &protocol.Connection{SSHConfigHost: "alias"}
	applySSHConfig(conn)

	if conn.Host != "alias.host" {
		t.Errorf("Host = %q want alias.host", conn.Host)
	}
	if conn.Port != 2222 {
		t.Errorf("Port = %d want 2222", conn.Port)
	}
	if conn.Credential == nil || conn.Credential.Username != "aliased" {
		t.Errorf("Credential.Username = %v want aliased", conn.Credential)
	}
	if conn.Proxy == nil || conn.Proxy.Host != "jump.example.com" {
		t.Errorf("Proxy = %+v want host jump.example.com", conn.Proxy)
	}
	if !conn.StrictHostKeyChecking {
		t.Error("StrictHostKeyChecking should be true when config says yes")
	}
}

// TestApplySSHConfigExplicitPrecedence verifies that explicitly supplied
// connection fields always win over the alias values (F2 / A6).
func TestApplySSHConfigExplicitPrecedence(t *testing.T) {
	withSSHConfig(t, `
Host alias
  HostName alias.host
  Port 2222
  User aliased
  ProxyJump jump.example.com
  StrictHostKeyChecking yes
`)
	conn := &protocol.Connection{
		Host:                  "explicit.host",
		Port:                  22,
		Credential:            &protocol.Credential{Username: "explicituser"},
		SSHConfigHost:         "alias",
		StrictHostKeyChecking: true, // user already opted in
	}
	applySSHConfig(conn)

	if conn.Host != "explicit.host" {
		t.Errorf("explicit Host overwritten: %q", conn.Host)
	}
	if conn.Port != 22 {
		t.Errorf("explicit Port overwritten: %d", conn.Port)
	}
	if conn.Credential.Username != "explicituser" {
		t.Errorf("explicit User overwritten: %q", conn.Credential.Username)
	}
	// ProxyJump still applies (no explicit Proxy set).
	if conn.Proxy == nil || conn.Proxy.Host != "jump.example.com" {
		t.Errorf("Proxy = %+v want host jump.example.com", conn.Proxy)
	}
	// Explicit StrictHostKeyChecking=true must be preserved, not flipped by
	// the alias value.
	if !conn.StrictHostKeyChecking {
		t.Error("explicit StrictHostKeyChecking=true was clobbered by alias")
	}
}

// TestApplySSHConfigProxyJumpSingleVsHops verifies single vs comma-chained
// ProxyJump -> conn.Proxy vs conn.Hops (F2 / A6).
func TestApplySSHConfigProxyJumpSingleVsHops(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		withSSHConfig(t, "Host j1\n  ProxyJump jump.example.com\n")
		conn := &protocol.Connection{SSHConfigHost: "j1"}
		applySSHConfig(conn)
		if conn.Proxy == nil || conn.Proxy.Host != "jump.example.com" {
			t.Fatalf("Proxy = %+v want host jump.example.com", conn.Proxy)
		}
		if conn.Proxy.Port != 22 {
			t.Errorf("Proxy.Port = %d want 22 (default)", conn.Proxy.Port)
		}
		if len(conn.Hops) != 0 {
			t.Errorf("single jump must not populate Hops, got %v", conn.Hops)
		}
	})
	t.Run("chained", func(t *testing.T) {
		withSSHConfig(t, "Host j2\n  ProxyJump a.example.com,b.example.com\n")
		conn := &protocol.Connection{SSHConfigHost: "j2"}
		applySSHConfig(conn)
		if conn.Proxy != nil {
			t.Errorf("chained jump must not populate single Proxy, got %+v", conn.Proxy)
		}
		if len(conn.Hops) != 2 {
			t.Fatalf("Hops = %v want 2 entries", conn.Hops)
		}
		if conn.Hops[0].Host != "a.example.com" || conn.Hops[1].Host != "b.example.com" {
			t.Errorf("Hops hosts = %v want [a.example.com b.example.com]", conn.Hops)
		}
	})
}

// TestApplySSHConfigStrictHostKeyChecking verifies the directive is parsed and
// applied, including the off/no/false spellings, and that an explicit
// user choice is never downgraded (F2 / A6).
func TestApplySSHConfigStrictHostKeyChecking(t *testing.T) {
	cases := []struct {
		directive string
		initial   bool
		want      bool
	}{
		{"no", false, false},
		{"off", false, false},
		{"false", false, false},
		{"yes", false, true},
		{"ask", false, true},
		{"accept-new", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.directive, func(t *testing.T) {
			withSSHConfig(t, "Host sc\n  StrictHostKeyChecking "+tc.directive+"\n")
			conn := &protocol.Connection{SSHConfigHost: "sc", StrictHostKeyChecking: tc.initial}
			applySSHConfig(conn)
			if conn.StrictHostKeyChecking != tc.want {
				t.Errorf("directive %q: StrictHostKeyChecking = %v want %v", tc.directive, conn.StrictHostKeyChecking, tc.want)
			}
		})
	}
}

// TestApplySSHConfigIdentityFile verifies an alias IdentityFile is read from
// disk into the credential (keys never leave the server) (F2 / A5).
func TestApplySSHConfigIdentityFile(t *testing.T) {
	base := withSSHConfig(t, "Host idhost\n  IdentityFile ~/.ssh/testkey\n")
	keyPath := filepath.Join(base, ".ssh", "testkey")
	if err := os.WriteFile(keyPath, []byte("KEY-MATERIAL"), 0o600); err != nil {
		t.Fatal(err)
	}
	conn := &protocol.Connection{SSHConfigHost: "idhost"}
	applySSHConfig(conn)
	if conn.Credential == nil || conn.Credential.PrivateKey != "KEY-MATERIAL" {
		t.Errorf("PrivateKey = %q want KEY-MATERIAL", conn.Credential.PrivateKey)
	}
	if conn.Credential.Type != "publickey" {
		t.Errorf("Type = %q want publickey", conn.Credential.Type)
	}
}

// TestApplySSHConfigNilSafe verifies applySSHConfig never panics on nil or
// on a connection with no SSHConfigHost.
func TestApplySSHConfigNilSafe(t *testing.T) {
	applySSHConfig(nil) // must not panic
	applySSHConfig(&protocol.Connection{})
}

// TestParseProxyJumpToken verifies [user@]host[:port] parsing.
func TestParseProxyJumpToken(t *testing.T) {
	cases := []struct {
		in       string
		host     string
		port     int
		user     string
	}{
		{"host", "host", 22, ""},
		{"host:2022", "host", 2022, ""},
		{"user@host", "host", 22, "user"},
		{"user@host:2022", "host", 2022, "user"},
		{"", "", 22, ""},
	}
	for _, tc := range cases {
		h, p, u := parseProxyJumpToken(tc.in)
		if h != tc.host || p != tc.port || u != tc.user {
			t.Errorf("parseProxyJumpToken(%q) = (%q,%d,%q) want (%q,%d,%q)",
				tc.in, h, p, u, tc.host, tc.port, tc.user)
		}
	}
}

// TestBuildHopsFromProxyJump verifies comma chains become ordered hops.
func TestBuildHopsFromProxyJump(t *testing.T) {
	hops := buildHopsFromProxyJump("a:22,b:22,c:22")
	if len(hops) != 3 {
		t.Fatalf("hops = %v want 3", hops)
	}
	for i, want := range []string{"a", "b", "c"} {
		if hops[i].Host != want {
			t.Errorf("hops[%d].Host = %q want %q", i, hops[i].Host, want)
		}
		if hops[i].Port != 22 {
			t.Errorf("hops[%d].Port = %d want 22", i, hops[i].Port)
		}
	}
}

// TestIsStrictHostKeyCheckingOff verifies the off-spellings.
func TestIsStrictHostKeyCheckingOff(t *testing.T) {
	off := []string{"no", "off", "false", "NO", " Off ", "FALSE"}
	for _, v := range off {
		if !isStrictHostKeyCheckingOff(v) {
			t.Errorf("isStrictHostKeyCheckingOff(%q) should be true", v)
		}
	}
	on := []string{"yes", "ask", "accept-new", ""}
	for _, v := range on {
		if isStrictHostKeyCheckingOff(v) {
			t.Errorf("isStrictHostKeyCheckingOff(%q) should be false", v)
		}
	}
}

// TestMatchGlob exercises the wildcard matcher used by ResolveSSHConfig.
func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, s string
		want        bool
	}{
		{"exact", "exact", true},
		{"*", "anything", true},
		{"web*", "web1", true},
		{"web*", "other", false},
		{"a?c", "abc", true},
		{"a?c", "ac", false},
		{"prod-*", "prod-1", true},
		{"prod-*", "stage-1", false},
	}
	for _, tc := range cases {
		if got := matchGlob(tc.pattern, tc.s); got != tc.want {
			t.Errorf("matchGlob(%q,%q) = %v want %v", tc.pattern, tc.s, got, tc.want)
		}
	}
}
