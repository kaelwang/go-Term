// Package internal wires the configuration, protocols, gateway and REST API
// into a single runnable server.
package internal

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kaelwang/go-Term/internal/api"
	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/gateway"
	"github.com/kaelwang/go-Term/internal/protocol"
	"github.com/kaelwang/go-Term/internal/protocol/localshell"
	"github.com/kaelwang/go-Term/internal/protocol/ssh"
	"github.com/kaelwang/go-Term/internal/protocol/telnet"
	"github.com/kaelwang/go-Term/internal/protocol/vnc"
	"github.com/kaelwang/go-Term/internal/store"
	"go.uber.org/zap"
)

// registerProtocols wires the protocol implementations into the registry.
func registerProtocols() {
	protocol.Register(protocol.ProtocolSSH, ssh.SSH{})
	protocol.Register(protocol.ProtocolTelnet, telnet.Telnet{})
	protocol.Register(protocol.ProtocolVNC, vnc.VNC{})
	protocol.Register(protocol.ProtocolLocalShell, localshell.LocalShell{})
}

// Server is the top-level go-Term HTTP/WS server.
type Server struct {
	cfg      *config.Config
	registry *gateway.SessionRegistry
	engine   *gin.Engine
	http     *http.Server
}

// New constructs a server from the loaded configuration.
func New(cfg *config.Config) *Server {
	registerProtocols()

	// Open the SQLite store, migrate the schema, and bootstrap the first
	// admin when the DB is empty and bootstrap credentials are provided
	// (fail-closed otherwise, C2).
	if err := store.Init(cfg); err != nil {
		zap.L().Error("store init failed", zap.Error(err))
	} else {
		if err := store.Migrate(); err != nil {
			zap.L().Error("store migrate failed", zap.Error(err))
		}
		if err := store.BootstrapAdmin(cfg); err != nil {
			zap.L().Error("store bootstrap failed", zap.Error(err))
		}
	}

	// Wire the gateway credential resolver to the SQLite store (T-V3).
	gateway.CredentialResolver = func(user, id string) (*protocol.Credential, error) {
		return store.GetCredentialDecrypted(user, id)
	}

	registry := gateway.NewSessionRegistry(30 * time.Second)
	registry.StartKeepalive()
	engine := api.NewRouter(registry, Version)
	return &Server{cfg: cfg, registry: registry, engine: engine}
}

// Run starts listening and blocks until the server stops.
func (s *Server) Run() error {
	s.http = &http.Server{
		Addr:    s.cfg.Listen,
		Handler: s.engine,
	}
	zap.L().Info("go-Term listening", zap.String("addr", s.cfg.Listen))
	return s.http.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() {
	if s.http == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.http.Shutdown(ctx)
	_ = store.Close()
}
