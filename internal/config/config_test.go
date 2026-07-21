package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain removes the ./data tree that config.Load() creates under the test
// working directory, keeping the repo tidy after the run.
func TestMain(m *testing.M) {
	code := m.Run()
	_ = os.RemoveAll("./data")
	os.Exit(code)
}

// clearEnv unsets every environment variable that config.Load binds so the
// tests are deterministic regardless of the host environment.
func clearEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"GOTERM_LISTEN", "GOTERM_MAX_CONCURRENCY", "ENABLE_AUTH",
		"DISABLE_LOCAL_TERMINAL", "SERVER_USER", "APP_SECRET", "JWT_SECRET",
		"JWT_EXPIRE_MINUTES", "GOTERM_KNOWN_HOSTS", "GOTERM_LOG_LEVEL",
	}
	for _, k := range keys {
		os.Unsetenv(k)
	}
}

func setEnv(t *testing.T, k, v string) {
	t.Helper()
	os.Setenv(k, v)
	t.Cleanup(func() { os.Unsetenv(k) })
}

// TestLoadDefaults verifies the built-in defaults are applied when no config
// file value and no environment variable override them.
func TestLoadDefaults(t *testing.T) {
	clearEnv(t)

	cfg := Load()

	if cfg.Listen != ":8080" {
		t.Errorf("Listen default: got %q want %q", cfg.Listen, ":8080")
	}
	if cfg.MaxConcurrency != 64 {
		t.Errorf("MaxConcurrency default: got %d want 64", cfg.MaxConcurrency)
	}
	if cfg.AuthEnabled {
		t.Error("AuthEnabled default should be false")
	}
	if cfg.DisableLocalTerm {
		t.Error("DisableLocalTerm default should be false")
	}
	if cfg.JWTExpireMinutes != 1440 {
		t.Errorf("JWTExpireMinutes default: got %d want 1440", cfg.JWTExpireMinutes)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel default: got %q want info", cfg.LogLevel)
	}
	// JWTSecret must fall back to AppSecret when empty.
	if cfg.AppSecret != "change-me-insecure-app-secret" {
		t.Errorf("AppSecret default mismatch: %q", cfg.AppSecret)
	}
	if cfg.JWTSecret != cfg.AppSecret {
		t.Errorf("JWTSecret should fall back to AppSecret, got %q", cfg.JWTSecret)
	}
	if cfg.ServerUserWhitelist != nil {
		t.Errorf("ServerUserWhitelist should be nil when SERVER_USER empty, got %v", cfg.ServerUserWhitelist)
	}
	if cfg.KnownHostsPath == "" {
		t.Error("KnownHostsPath should be expanded to a non-empty path")
	}
}

// TestLoadEnvOverrides verifies each bound environment variable overrides the
// default / file value.
func TestLoadEnvOverrides(t *testing.T) {
	clearEnv(t)
	setEnv(t, "GOTERM_LISTEN", ":9000")
	setEnv(t, "GOTERM_MAX_CONCURRENCY", "32")
	setEnv(t, "ENABLE_AUTH", "1")
	setEnv(t, "DISABLE_LOCAL_TERMINAL", "true")
	setEnv(t, "SERVER_USER", "alice,bob")
	setEnv(t, "APP_SECRET", "env-app-secret")
	setEnv(t, "JWT_SECRET", "env-jwt-secret")
	setEnv(t, "JWT_EXPIRE_MINUTES", "120")
	setEnv(t, "GOTERM_LOG_LEVEL", "debug")

	cfg := Load()

	if cfg.Listen != ":9000" {
		t.Errorf("GOTERM_LISTEN override: got %q want :9000", cfg.Listen)
	}
	if cfg.MaxConcurrency != 32 {
		t.Errorf("GOTERM_MAX_CONCURRENCY override: got %d want 32", cfg.MaxConcurrency)
	}
	if !cfg.AuthEnabled {
		t.Error("ENABLE_AUTH=1 should enable auth")
	}
	if !cfg.DisableLocalTerm {
		t.Error("DISABLE_LOCAL_TERMINAL=true should disable local terminal")
	}
	if len(cfg.ServerUserWhitelist) != 2 || cfg.ServerUserWhitelist[0] != "alice" || cfg.ServerUserWhitelist[1] != "bob" {
		t.Errorf("SERVER_USER override mismatch: %v", cfg.ServerUserWhitelist)
	}
	if cfg.AppSecret != "env-app-secret" {
		t.Errorf("APP_SECRET override: got %q", cfg.AppSecret)
	}
	// Explicit JWT_SECRET must win over APP_SECRET fallback.
	if cfg.JWTSecret != "env-jwt-secret" {
		t.Errorf("JWT_SECRET override: got %q want env-jwt-secret", cfg.JWTSecret)
	}
	if cfg.JWTExpireMinutes != 120 {
		t.Errorf("JWT_EXPIRE_MINUTES override: got %d want 120", cfg.JWTExpireMinutes)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("GOTERM_LOG_LEVEL override: got %q want debug", cfg.LogLevel)
	}
}

// TestParseBoolEnvVariants verifies the various truthy/falsy spellings the
// loader accepts.
func TestParseBoolEnvVariants(t *testing.T) {
	truthy := []string{"1", "t", "T", "true", "TRUE", "yes", "Y", "on", "On"}
	for _, v := range truthy {
		if !parseBoolEnv(v) {
			t.Errorf("parseBoolEnv(%q) should be true", v)
		}
	}
	falsy := []string{"0", "f", "false", "no", "n", "off", "", "garbage"}
	for _, v := range falsy {
		if parseBoolEnv(v) {
			t.Errorf("parseBoolEnv(%q) should be false", v)
		}
	}
}

// TestMaxConcurrencyClamp verifies a non-positive concurrency is clamped to 64.
func TestMaxConcurrencyClamp(t *testing.T) {
	clearEnv(t)
	setEnv(t, "GOTERM_MAX_CONCURRENCY", "0")
	cfg := Load()
	if cfg.MaxConcurrency != 64 {
		t.Errorf("zero concurrency should clamp to 64, got %d", cfg.MaxConcurrency)
	}

	clearEnv(t)
	setEnv(t, "GOTERM_MAX_CONCURRENCY", "-5")
	cfg = Load()
	if cfg.MaxConcurrency != 64 {
		t.Errorf("negative concurrency should clamp to 64, got %d", cfg.MaxConcurrency)
	}
}

// TestSplitList verifies comma-separated whitelist parsing.
func TestSplitList(t *testing.T) {
	if got := splitList(""); got != nil {
		t.Errorf("empty input should yield nil, got %v", got)
	}
	got := splitList("  alice , bob , , carol ")
	want := []string{"alice", "bob", "carol"}
	if len(got) != len(want) {
		t.Fatalf("splitList length mismatch: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitList[%d] = %q want %q", i, got[i], want[i])
		}
	}
}

// TestDialTimeout verifies the constant dial timeout.
func TestDialTimeout(t *testing.T) {
	cfg := &Config{}
	if cfg.DialTimeout().String() != "15s" {
		t.Errorf("DialTimeout should be 15s, got %v", cfg.DialTimeout())
	}
}

// TestUploadDownloadDirsCreated verifies the loader creates the upload/download
// directories (and cleans them up afterwards to keep the tree tidy).
func TestUploadDownloadDirsCreated(t *testing.T) {
	clearEnv(t)
	cfg := Load()
	for _, d := range []string{cfg.UploadDir, cfg.DownloadDir} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("expected dir %q to be created: %v", d, err)
		}
	}
	// Best-effort cleanup of the ./data tree created under cwd.
	_ = os.RemoveAll(filepath.Join(".", "data"))
}
