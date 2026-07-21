package gateway

import (
	"strings"
	"testing"

	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/protocol"
	"github.com/kaelwang/go-Term/internal/security"
)

// withGlobalKnownHosts installs a temporary global config with the given
// known_hosts path so applyConnDefaults can be exercised deterministically,
// then restores the previous global on cleanup.
func withGlobalKnownHosts(t *testing.T, path string) {
	t.Helper()
	prev := config.Global
	config.Global = &config.Config{KnownHostsPath: path}
	t.Cleanup(func() { config.Global = prev })
}

// TestApplyConnDefaultsFallsBackToGlobal verifies the core fix: when the
// frontend sends an empty known_hosts_path, it is replaced by the global
// default so the SSH handshake never sees an empty path.
func TestApplyConnDefaultsFallsBackToGlobal(t *testing.T) {
	withGlobalKnownHosts(t, "/home/user/.ssh/known_hosts")

	conn := &protocol.Connection{ID: "c1", Protocol: protocol.ProtocolSSH, KnownHostsPath: ""}
	applyConnDefaults(conn)

	if conn.KnownHostsPath != "/home/user/.ssh/known_hosts" {
		t.Fatalf("KnownHostsPath fallback: got %q want %q", conn.KnownHostsPath, "/home/user/.ssh/known_hosts")
	}
}

// TestApplyConnDefaultsKeepsClientValue verifies an explicitly supplied
// known_hosts_path is left untouched (no clobbering of a valid client value).
func TestApplyConnDefaultsKeepsClientValue(t *testing.T) {
	withGlobalKnownHosts(t, "/home/user/.ssh/known_hosts")

	conn := &protocol.Connection{ID: "c2", Protocol: protocol.ProtocolSSH, KnownHostsPath: "/custom/path"}
	applyConnDefaults(conn)

	if conn.KnownHostsPath != "/custom/path" {
		t.Fatalf("client KnownHostsPath should be preserved: got %q", conn.KnownHostsPath)
	}
}

// TestApplyConnDefaultsEmptyGlobalNoop verifies that when the global config
// has no known_hosts path either, the connection path stays empty (the
// fallback is conservative and does not invent a value).
func TestApplyConnDefaultsEmptyGlobalNoop(t *testing.T) {
	withGlobalKnownHosts(t, "")

	conn := &protocol.Connection{ID: "c3", Protocol: protocol.ProtocolSSH, KnownHostsPath: ""}
	applyConnDefaults(conn)

	if conn.KnownHostsPath != "" {
		t.Fatalf("with empty global, KnownHostsPath should stay empty: got %q", conn.KnownHostsPath)
	}
}

// TestApplyConnDefaultsNilSafe verifies a nil connection does not panic.
func TestApplyConnDefaultsNilSafe(t *testing.T) {
	withGlobalKnownHosts(t, "/home/user/.ssh/known_hosts")
	applyConnDefaults(nil) // must not panic
}

// TestFallbackEliminatesEmptyKnownHostsError ties the unit fix to the original
// symptom: an empty known_hosts_path is exactly what made security.CheckHostKey
// return "empty known_hosts path" during the handshake. After applyConnDefaults
// fills the path, that error can no longer be produced.
func TestFallbackEliminatesEmptyKnownHostsError(t *testing.T) {
	// The bug: empty path -> "empty known_hosts path" error.
	if _, err := security.CheckHostKey("", "127.0.0.1:22", nil); err == nil ||
		!strings.Contains(err.Error(), "empty known_hosts path") {
		t.Fatalf("empty path should produce 'empty known_hosts path' error, got %v", err)
	}

	// The fix: fallback fills the path, so the error disappears.
	withGlobalKnownHosts(t, "/tmp/webssh_qa_known_hosts")
	conn := &protocol.Connection{ID: "c4", Protocol: protocol.ProtocolSSH, KnownHostsPath: ""}
	applyConnDefaults(conn)

	if conn.KnownHostsPath == "" {
		t.Fatal("precondition: fallback should have filled KnownHostsPath")
	}
	if _, err := security.CheckHostKey(conn.KnownHostsPath, "127.0.0.1:22", nil); err != nil {
		t.Fatalf("after fallback, CheckHostKey must not error: %v", err)
	}
}
