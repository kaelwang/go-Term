package config

import (
	"os"
	"path/filepath"
	"testing"
)

// parseAndLoad is a small helper used by the CLI tests: it parses the given
// args (without exiting) and loads config on top of a clean environment.
func parseAndLoad(t *testing.T, args ...string) *Config {
	t.Helper()
	clearEnv(t)
	opts := ParseFlagsForTest(args, "1.0.0")
	return Load(opts)
}

// TestCLIPriorityOverEnv proves a CLI flag beats an environment variable:
// GOTERM_LISTEN=:9999 but --port 5517 must yield :5517.
func TestCLIPriorityOverEnv(t *testing.T) {
	clearEnv(t)
	setEnv(t, "GOTERM_LISTEN", ":9999")

	opts := ParseFlagsForTest([]string{"--port", "5517"}, "1.0.0")
	cfg := Load(opts)

	if cfg.Listen != ":5517" {
		t.Errorf("CLI --port should override GOTERM_LISTEN: got %q want %q", cfg.Listen, ":5517")
	}
}

// TestHostPortMerge proves --host + --port merge into host:port.
func TestHostPortMerge(t *testing.T) {
	cfg := parseAndLoad(t, "--host", "0.0.0.0", "--port", "5517")
	if cfg.Listen != "0.0.0.0:5517" {
		t.Errorf("--host/--port merge: got %q want %q", cfg.Listen, "0.0.0.0:5517")
	}
}

// TestHostOnlyMerge proves --host alone falls back to the 8080 port constant
// (the constant must NOT appear in the flag default, only in the merge logic).
func TestHostOnlyMerge(t *testing.T) {
	cfg := parseAndLoad(t, "--host", "127.0.0.1")
	if cfg.Listen != "127.0.0.1:8080" {
		t.Errorf("--host only merge: got %q want %q", cfg.Listen, "127.0.0.1:8080")
	}
}

// TestPortOnlyMerge proves --port alone yields :port.
func TestPortOnlyMerge(t *testing.T) {
	cfg := parseAndLoad(t, "--port", "5517")
	if cfg.Listen != ":5517" {
		t.Errorf("--port only merge: got %q want %q", cfg.Listen, ":5517")
	}
}

// TestListenPrecedence proves --listen wins over --host/--port.
func TestListenPrecedence(t *testing.T) {
	cfg := parseAndLoad(t, "--listen", "1.2.3.4:1234", "--host", "x", "--port", "y")
	if cfg.Listen != "1.2.3.4:1234" {
		t.Errorf("--listen should take precedence: got %q want %q", cfg.Listen, "1.2.3.4:1234")
	}
}

// TestDefaultPreservedNoCLI proves that with no CLI flags the listen address
// keeps the viper result (env or built-in default :8080) untouched.
func TestDefaultPreservedNoCLI(t *testing.T) {
	// No env, no CLI -> built-in default.
	cfg := parseAndLoad(t)
	if cfg.Listen != ":8080" {
		t.Errorf("default listen: got %q want %q", cfg.Listen, ":8080")
	}

	// Env, no CLI -> env wins over default (and CLI leaves it alone).
	clearEnv(t)
	setEnv(t, "GOTERM_LISTEN", ":7777")
	opts := ParseFlagsForTest([]string{}, "1.0.0")
	cfg = Load(opts)
	if cfg.Listen != ":7777" {
		t.Errorf("env listen without CLI: got %q want %q", cfg.Listen, ":7777")
	}
}

// TestConfigFileFlag proves -c/--config selects a single config file.
func TestConfigFileFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cli-config.yaml")
	if err := os.WriteFile(path, []byte("listen: :1234\nlog_level: warn\n"), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	clearEnv(t)
	opts := ParseFlagsForTest([]string{"-c", path}, "1.0.0")
	if opts.ConfigFile != path {
		t.Fatalf("Options.ConfigFile: got %q want %q", opts.ConfigFile, path)
	}
	cfg := Load(opts)
	if cfg.Listen != ":1234" {
		t.Errorf("config file listen: got %q want %q", cfg.Listen, ":1234")
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("config file log_level: got %q want warn", cfg.LogLevel)
	}
}

// TestAuthFlag proves --auth enables auth (mapped to auth_enabled).
func TestAuthFlag(t *testing.T) {
	cfg := parseAndLoad(t, "--auth")
	if !cfg.AuthEnabled {
		t.Error("--auth should enable auth")
	}
}

// TestOtherFlagsMapped proves additional flags land in the right config fields.
func TestOtherFlagsMapped(t *testing.T) {
	cfg := parseAndLoad(t,
		"--server-user", "alice,bob",
		"--db-path", "/tmp/cli.db",
		"--max-concurrency", "16",
		"--log-level", "debug",
	)
	if len(cfg.ServerUserWhitelist) != 2 || cfg.ServerUserWhitelist[0] != "alice" {
		t.Errorf("server_user: %v", cfg.ServerUserWhitelist)
	}
	if cfg.DBPath != "/tmp/cli.db" {
		t.Errorf("db_path: got %q", cfg.DBPath)
	}
	if cfg.MaxConcurrency != 16 {
		t.Errorf("max_concurrency: got %d want 16", cfg.MaxConcurrency)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("log_level: got %q want debug", cfg.LogLevel)
	}
}

// TestEmptySetNotRecorded proves that default-valued flags do not appear in
// Options.Set (so they never clobber env/yaml values).
func TestEmptySetNotRecorded(t *testing.T) {
	opts := ParseFlagsForTest([]string{}, "1.0.0")
	if len(opts.Set) != 0 {
		t.Errorf("no flags given: Set should be empty, got %v", opts.Set)
	}
}
