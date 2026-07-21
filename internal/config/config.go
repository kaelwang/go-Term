// Package config loads go-Term runtime configuration from a YAML file,
// environment variables, and sensible defaults. Environment variables take
// precedence over the file (see BindEnv calls).
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config is the resolved runtime configuration.
type Config struct {
	Listen              string        `json:"listen"`
	MaxConcurrency      int           `json:"max_concurrency"`
	AuthEnabled         bool          `json:"auth_enabled"`
	DisableLocalTerm    bool          `json:"disable_local_terminal"`
	ServerUserWhitelist []string      `json:"server_user_whitelist"`
	AppSecret           string        `json:"-"`
	JWTSecret           string        `json:"-"`
	JWTExpireMinutes    int           `json:"jwt_expire_minutes"`
	KnownHostsPath      string        `json:"known_hosts_path"`
	UploadDir           string        `json:"upload_dir"`
	DownloadDir         string        `json:"download_dir"`
	LogLevel            string        `json:"log_level"`
	// DBPath is the SQLite database file path (T-V1).
	DBPath string `json:"db_path"`
	// VaultKeyRaw is the raw encryption key for the credential vault (C3).
	// Prefer VaultKey() which falls back to AppSecret when this is empty.
	VaultKeyRaw string `json:"-"`
	// BootstrapAdminUser / BootstrapAdminPass seed the first admin account on an
	// empty database (C2). Both must be set; otherwise login fails closed.
	BootstrapAdminUser string `json:"-"`
	BootstrapAdminPass string `json:"-"`
}

// Global holds the loaded configuration for the running process.
var Global *Config

// expandUser expands a leading "~" to the current user's home directory.
func expandUser(p string) string {
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// Load reads configuration with the following precedence (high to low):
//   CLI flags (Options.Set)  >  environment variables  >  config.yaml  >  built-in defaults
//
// The optional Options (produced by ParseFlags) overlays explicitly-provided
// CLI flags on top of the other sources and may select a single config file
// via -c/--config. The variadic signature keeps existing no-argument Load()
// callers (and config_test.go) compiling unchanged.
func Load(opts ...*Options) *Config {
	var o *Options
	if len(opts) > 0 {
		o = opts[0]
	}
	return loadWith(o)
}

// loadWith contains the real loading logic. See Load for the public entry point.
func loadWith(opts *Options) *Config {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	if opts != nil && opts.ConfigFile != "" {
		// -c/--config selects the single config file; skip default search paths.
		v.SetConfigFile(opts.ConfigFile)
	} else {
		v.AddConfigPath("./internal/config/")
		v.AddConfigPath(".")
	}

	v.SetDefault("listen", ":8080")
	v.SetDefault("max_concurrency", 64)
	v.SetDefault("auth_enabled", false)
	v.SetDefault("disable_local_terminal", false)
	v.SetDefault("server_user", "")
	v.SetDefault("app_secret", "change-me-insecure-app-secret")
	v.SetDefault("jwt_secret", "")
	v.SetDefault("jwt_expire_minutes", 60*24)
	v.SetDefault("known_hosts", "~/.ssh/known_hosts")
	v.SetDefault("upload_dir", "./data/uploads")
	v.SetDefault("download_dir", "./data/downloads")
	v.SetDefault("log_level", "info")
	v.SetDefault("db_path", "./go-Term.db")
	v.SetDefault("vault_key", "")
	v.SetDefault("bootstrap_admin_user", "")
	v.SetDefault("bootstrap_admin_pass", "")

	// Bind environment variables (GOTERM_*, ENABLE_AUTH, ...).
	_ = v.BindEnv("listen", "GOTERM_LISTEN")
	_ = v.BindEnv("max_concurrency", "GOTERM_MAX_CONCURRENCY")
	_ = v.BindEnv("auth_enabled", "ENABLE_AUTH")
	_ = v.BindEnv("disable_local_terminal", "DISABLE_LOCAL_TERMINAL")
	_ = v.BindEnv("server_user", "SERVER_USER")
	_ = v.BindEnv("app_secret", "APP_SECRET")
	_ = v.BindEnv("jwt_secret", "JWT_SECRET")
	_ = v.BindEnv("jwt_expire_minutes", "JWT_EXPIRE_MINUTES")
	_ = v.BindEnv("known_hosts", "GOTERM_KNOWN_HOSTS")
	_ = v.BindEnv("log_level", "GOTERM_LOG_LEVEL")
	_ = v.BindEnv("db_path", "GOTERM_DB_PATH")
	_ = v.BindEnv("vault_key", "GOTERM_VAULT_KEY")
	_ = v.BindEnv("bootstrap_admin_user", "GOTERM_BOOTSTRAP_ADMIN_USER")
	_ = v.BindEnv("bootstrap_admin_pass", "GOTERM_BOOTSTRAP_ADMIN_PASS")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file present but unreadable: log and continue with defaults.
			zap.L().Warn("failed to read config file, using defaults", zap.Error(err))
		}
	}

	// Apply CLI flag overrides (highest precedence). Must run after ReadInConfig
	// so v.Set wins over environment variables, the config file, and defaults.
	if opts != nil && len(opts.Set) > 0 {
		applyCLIOptions(v, opts)
	}

	cfg := &Config{
		Listen:              v.GetString("listen"),
		MaxConcurrency:      v.GetInt("max_concurrency"),
		AuthEnabled:         parseBoolEnv(v.GetString("auth_enabled")),
		DisableLocalTerm:    parseBoolEnv(v.GetString("disable_local_terminal")),
		ServerUserWhitelist: splitList(v.GetString("server_user")),
		AppSecret:           v.GetString("app_secret"),
		JWTSecret:           v.GetString("jwt_secret"),
		JWTExpireMinutes:    v.GetInt("jwt_expire_minutes"),
		KnownHostsPath:      expandUser(v.GetString("known_hosts")),
		UploadDir:           expandUser(v.GetString("upload_dir")),
		DownloadDir:          expandUser(v.GetString("download_dir")),
		LogLevel:            v.GetString("log_level"),
		DBPath:              v.GetString("db_path"),
		VaultKeyRaw:         v.GetString("vault_key"),
		BootstrapAdminUser:  v.GetString("bootstrap_admin_user"),
		BootstrapAdminPass:  v.GetString("bootstrap_admin_pass"),
	}

	if cfg.JWTSecret == "" {
		cfg.JWTSecret = cfg.AppSecret
	}
	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = 64
	}
	for _, d := range []string{cfg.UploadDir, cfg.DownloadDir} {
		_ = os.MkdirAll(d, 0o755)
	}

	Global = cfg
	return cfg
}

// VaultKey returns the encryption key for the credential vault. It falls back
// to AppSecret when GOTERM_VAULT_KEY is not configured (C3).
func (c *Config) VaultKey() string {
	if c == nil {
		return ""
	}
	if c.VaultKeyRaw != "" {
		return c.VaultKeyRaw
	}
	return c.AppSecret
}

// parseBoolEnv interprets "1", "t", "true", "yes" (case-insensitive) as true.
func parseBoolEnv(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "t", "true", "yes", "y", "on":
		return true
	default:
		// viper may already coerce booleans; fall back to strconv.
		b, err := strconv.ParseBool(s)
		return err == nil && b
	}
}

// splitList splits a comma-separated string into trimmed, non-empty tokens.
func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// DialTimeout returns the default dial timeout used by protocol dialers.
func (c *Config) DialTimeout() time.Duration {
	return 15 * time.Second
}

// ServerUserAllowed reports whether user is permitted by the SERVER_USER
// whitelist. An empty whitelist permits everyone. This is the single source of
// truth shared by the RequireServerUser middleware and the WebSocket gateway's
// localshell gate (F3).
func (c *Config) ServerUserAllowed(user string) bool {
	if c == nil || len(c.ServerUserWhitelist) == 0 {
		return true
	}
	for _, w := range c.ServerUserWhitelist {
		if w == user {
			return true
		}
	}
	return false
}
