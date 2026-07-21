// Command server is the go-Term entry point: it loads configuration,
// initializes the structured logger, builds the server and serves until exit.
package main

import (
	"os"

	"github.com/kaelwang/go-Term/internal"
	"github.com/kaelwang/go-Term/internal/config"
	"go.uber.org/zap"
)

func main() {
	opts := config.ParseFlags(os.Args[1:], internal.Version)
	cfg := config.Load(opts)
	buildLogger(cfg.LogLevel)

	server := internal.New(cfg)
	if err := server.Run(); err != nil {
		zap.L().Fatal("server exited", zap.Error(err))
		os.Exit(1)
	}
}

// buildLogger configures the global zap logger from a level string.
func buildLogger(level string) {
	atomicLevel := zap.NewAtomicLevel()
	if err := atomicLevel.UnmarshalText([]byte(level)); err != nil {
		atomicLevel.SetLevel(zap.InfoLevel)
	}
	cfg := zap.NewProductionConfig()
	cfg.Level = atomicLevel
	cfg.Encoding = "json"
	logger, err := cfg.Build()
	if err != nil {
		logger = zap.NewNop()
	}
	zap.ReplaceGlobals(logger)
}
