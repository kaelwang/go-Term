// Package config loads go-Term runtime configuration from CLI flags, a YAML
// file, environment variables, and sensible defaults. CLI flags take the
// highest precedence (see ParseFlags and applyCLIOptions).
package config

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Options captures the result of parsing CLI flags.
//
// ConfigFile is set when the user explicitly passed -c/--config; it is consumed
// by Load (via viper.SetConfigFile) before ReadInConfig runs.
//
// Set records only the flags the user explicitly provided on the command line
// (flag.Visit skips defaults). Values are kept as their raw string form
// (f.Value.String()); Load maps each key to its viper key (dashes -> underscores)
// and applies it via v.Set. The names "host", "port", "listen", "help",
// "version" and "config" are handled specially and never appear 1:1 in Set.
type Options struct {
	ConfigFile string
	Set        map[string]string
}

// ParseFlags registers and parses the go-Term command-line flags. It uses
// flag.ExitOnError: unknown flags print usage and exit 2; -h/--help and
// -v/--version print their message and exit 0.
func ParseFlags(args []string, version string) *Options {
	return parseFlags(args, version, flag.ExitOnError)
}

// ParseFlagsForTest is identical to ParseFlags but uses flag.ContinueOnError so
// it can be exercised from Go tests without terminating the test process.
func ParseFlagsForTest(args []string, version string) *Options {
	return parseFlags(args, version, flag.ContinueOnError)
}

// parseFlags builds the flag set, parses args with the given error handling,
// and collects the explicitly-provided flags into an Options value.
func parseFlags(args []string, version string, eh flag.ErrorHandling) *Options {
	fs := flag.NewFlagSet("go-Term", eh)
	fs.SetOutput(os.Stdout)

	var (
		configFile         string
		host               string
		port               string
		listen             string
		logLevel           string
		auth               bool
		serverUser         string
		dbPath             string
		vaultKey           string
		knownHosts         string
		uploadDir          string
		downloadDir        string
		maxConcurrency     int
		bootstrapAdminUser string
		bootstrapAdminPass string
	)

	// -c / --config is special: it selects the single config file viper reads.
	fs.StringVar(&configFile, "config", "", "指定 YAML 配置文件路径（交给 viper 读取，覆盖默认搜索路径）")
	fs.StringVar(&configFile, "c", "", "指定 YAML 配置文件路径（config 的短名）")

	fs.StringVar(&host, "host", "", "监听主机(IP/域名)；与 --port 合并为 host:port（默认空 = 监听所有网卡）")
	fs.StringVar(&port, "port", "", "监听端口；与 --host 合并为 host:port（默认 8080，仅当指定 --host 时用于回退）")
	fs.StringVar(&listen, "listen", "", "完整监听地址 host:port，优先级高于 --host/--port")
	fs.StringVar(&logLevel, "log-level", "info", "日志级别: debug|info|warn|error (默认 \"info\")")
	fs.BoolVar(&auth, "auth", false, "启用 Web 登录与 JWT 鉴权（对应 ENABLE_AUTH）")
	fs.StringVar(&serverUser, "server-user", "", "本地终端/特权操作白名单，逗号分隔（对应 SERVER_USER）")
	fs.StringVar(&dbPath, "db-path", "./go-Term.db", "SQLite 数据库文件路径（对应 GOTERM_DB_PATH，默认 \"./go-Term.db\"）")
	fs.StringVar(&vaultKey, "vault-key", "", "凭证库 AES-256-GCM 密钥（对应 GOTERM_VAULT_KEY）⚠机密建议用环境变量")
	fs.StringVar(&knownHosts, "known-hosts", "~/.ssh/known_hosts", "known_hosts 文件路径（对应 GOTERM_KNOWN_HOSTS，默认 \"~/.ssh/known_hosts\"）")
	fs.StringVar(&uploadDir, "upload-dir", "./data/uploads", "HTTP 上传临时目录（默认 \"./data/uploads\"）")
	fs.StringVar(&downloadDir, "download-dir", "./data/downloads", "HTTP 下载落盘目录（默认 \"./data/downloads\"）")
	fs.IntVar(&maxConcurrency, "max-concurrency", 64, "最大并发会话/传输 worker 数（对应 GOTERM_MAX_CONCURRENCY，默认 64）")
	fs.StringVar(&bootstrapAdminUser, "bootstrap-admin-user", "", "首启引导管理员用户名（对应 GOTERM_BOOTSTRAP_ADMIN_USER）")
	fs.StringVar(&bootstrapAdminPass, "bootstrap-admin-pass", "", "首启引导管理员密码（对应 GOTERM_BOOTSTRAP_ADMIN_PASS）⚠机密")

	// Custom usage is shown on -h/--help and on parse errors.
	fs.Usage = func() { printUsage(fs, version) }

	// -h / --help and -v / --version print and exit 0 immediately.
	fs.BoolFunc("help", "显示帮助信息并退出", func(string) error {
		printUsage(fs, version)
		os.Exit(0)
		return nil
	})
	fs.BoolFunc("h", "显示帮助信息并退出（help 的短名）", func(string) error {
		printUsage(fs, version)
		os.Exit(0)
		return nil
	})
	fs.BoolFunc("version", "显示版本号并退出", func(string) error {
		fmt.Printf("go-Term %s\n", version)
		os.Exit(0)
		return nil
	})
	fs.BoolFunc("v", "显示版本号并退出（version 的短名）", func(string) error {
		fmt.Printf("go-Term %s\n", version)
		os.Exit(0)
		return nil
	})

	_ = fs.Parse(args)

	opts := &Options{ConfigFile: configFile, Set: make(map[string]string)}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "config", "c":
			// Special-cased: routed to Options.ConfigFile, not into Set.
			opts.ConfigFile = f.Value.String()
		case "help", "h", "version", "v":
			// Already handled by BoolFunc (exit 0); nothing to record.
		default:
			opts.Set[f.Name] = f.Value.String()
		}
	})
	return opts
}

// applyCLIOptions overlays the explicitly-provided CLI flags onto the viper
// instance. It is called after ReadInConfig so v.Set wins over env/yaml/default.
//
// Final precedence (high -> low): CLI flags (v.Set) > env (BindEnv) >
// config.yaml > built-in defaults.
func applyCLIOptions(v *viper.Viper, opts *Options) {
	const defaultPort = "8080"

	// host / port / listen merge (only when at least one was explicitly set).
	if _, ok := opts.Set["listen"]; ok {
		v.Set("listen", opts.Set["listen"])
	} else if _, ok := opts.Set["host"]; ok {
		host := opts.Set["host"]
		port := defaultPort
		if p, ok := opts.Set["port"]; ok {
			port = p
		}
		if host == "" {
			v.Set("listen", ":"+port)
		} else {
			v.Set("listen", host+":"+port)
		}
	} else if _, ok := opts.Set["port"]; ok {
		v.Set("listen", ":"+opts.Set["port"])
	}

	// Remaining flags: viper key = dash -> underscore; skip special-cased names.
	// A small rename map covers flags whose name does not match their viper key
	// (e.g. --auth maps to auth_enabled).
	skip := map[string]bool{
		"host": true, "port": true, "listen": true,
		"help": true, "version": true, "config": true,
	}
	rename := map[string]string{
		"auth": "auth_enabled",
	}
	for name, val := range opts.Set {
		if skip[name] {
			continue
		}
		key := name
		if r, ok := rename[name]; ok {
			key = r
		} else {
			key = strings.ReplaceAll(name, "-", "_")
		}
		v.Set(key, val)
	}
}

// printUsage renders the custom --help text.
func printUsage(fs *flag.FlagSet, version string) {
	_ = version // version is shown via -v/--version; usage body is static.
	text := `go-Term - Web SSH / 远程终端网关

Usage:
  go-Term [flags]

Flags:
  -h, --help                      显示帮助信息并退出
  -v, --version                   显示版本号并退出
  -c, --config string             指定 YAML 配置文件路径（交给 viper 读取，覆盖默认搜索路径）

  --host string                   监听主机(IP/域名)；与 --port 合并为 host:port（默认空 = 监听所有网卡）
  --port string                   监听端口；与 --host 合并为 host:port（默认 8080，仅当指定 --host 时用于回退）
  --listen string                 完整监听地址 host:port，优先级高于 --host/--port
  --log-level string              日志级别: debug|info|warn|error (默认 "info")
  --auth                         启用 Web 登录与 JWT 鉴权（对应 ENABLE_AUTH）
  --server-user string            本地终端/特权操作白名单，逗号分隔（对应 SERVER_USER）
  --db-path string               SQLite 数据库文件路径（对应 GOTERM_DB_PATH，默认 "./go-Term.db"）
  --vault-key string              凭证库 AES-256-GCM 密钥（对应 GOTERM_VAULT_KEY）⚠机密建议用环境变量
  --known-hosts string            known_hosts 文件路径（对应 GOTERM_KNOWN_HOSTS，默认 "~/.ssh/known_hosts"）
  --upload-dir string             HTTP 上传临时目录（默认 "./data/uploads"）
  --download-dir string           HTTP 下载落盘目录（默认 "./data/downloads"）
  --max-concurrency int           最大并发会话/传输 worker 数（对应 GOTERM_MAX_CONCURRENCY，默认 64）
  --bootstrap-admin-user string   首启引导管理员用户名（对应 GOTERM_BOOTSTRAP_ADMIN_USER）
  --bootstrap-admin-pass string   首启引导管理员密码（对应 GOTERM_BOOTSTRAP_ADMIN_PASS）⚠机密

配置优先级:  CLI 参数  >  环境变量  >  config.yaml  >  内置默认值
注意:  APP_SECRET / JWT_SECRET 不提供命令行参数，请通过环境变量或 config.yaml 设置。
`
	_, _ = fmt.Fprint(fs.Output(), text)
}
