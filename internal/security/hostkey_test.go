package security

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckHostKeyEmptyPathReturnsError documents the original failure mode:
// when the connection spec carries an empty known_hosts path, CheckHostKey
// returns the "empty known_hosts path" error that surfaced during the SSH
// handshake. This is the exact error the gateway fallback is meant to prevent.
func TestCheckHostKeyEmptyPathReturnsError(t *testing.T) {
	res, err := CheckHostKey("", "127.0.0.1:22", nil)
	if err == nil {
		t.Fatal("empty path should return an error")
	}
	if !strings.Contains(err.Error(), "empty known_hosts path") {
		t.Fatalf("error should mention 'empty known_hosts path', got %q", err.Error())
	}
	// Result must be the zero value on error.
	if res.Known || res.Trusted {
		t.Errorf("on error result should be zero value, got %+v", res)
	}
}

// TestCheckHostKeyNonEmptyPathNoEmptyError verifies that a non-empty path never
// produces the "empty known_hosts path" error. A missing file is treated as
// "host unknown, not trusted" (nil error), which is the state the gateway
// fallback guarantees by supplying config.Global.KnownHostsPath.
func TestCheckHostKeyNonEmptyPathNoEmptyError(t *testing.T) {
	// t.TempDir() yields a valid, existing directory; the file itself is absent.
	missing := filepath.Join(t.TempDir(), "known_hosts")

	res, err := CheckHostKey(missing, "127.0.0.1:22", nil)
	if err != nil {
		t.Fatalf("non-empty path should not return 'empty known_hosts path' error, got %v", err)
	}
	if res.Known {
		t.Error("absent file should report host as unknown")
	}
	if res.Trusted {
		t.Error("absent file should report host as untrusted")
	}
}
