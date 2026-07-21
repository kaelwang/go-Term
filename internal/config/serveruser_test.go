package config

import "testing"

// TestServerUserAllowedWhitelist verifies the single source of truth for the
// SERVER_USER gate (F3 / A8):
//   - empty whitelist permits everyone
//   - a nil Config also permits everyone
//   - a non-empty whitelist permits matching users, denies the rest.
func TestServerUserAllowedWhitelist(t *testing.T) {
	// Empty whitelist -> permit everyone.
	c := &Config{}
	if !c.ServerUserAllowed("anyone") {
		t.Error("empty whitelist should permit everyone")
	}

	// Nil *Config -> permit everyone (method must not deref nil).
	var n *Config
	if !n.ServerUserAllowed("anyone") {
		t.Error("nil Config should permit everyone")
	}

	// Non-empty whitelist -> match / no-match.
	c2 := &Config{ServerUserWhitelist: []string{"alice", "bob"}}
	if !c2.ServerUserAllowed("alice") {
		t.Error("alice should be allowed")
	}
	if !c2.ServerUserAllowed("bob") {
		t.Error("bob should be allowed")
	}
	if c2.ServerUserAllowed("carol") {
		t.Error("carol should be denied (not in whitelist)")
	}
}
