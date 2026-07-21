package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/gateway"
	"github.com/kaelwang/go-Term/internal/security"
	"github.com/kaelwang/go-Term/internal/store"
)

// TestServerUserAllowedFunc verifies the centralized whitelist helper used by
// both LoginHandler and the RequireServerUser middleware (F3 / A8).
func TestServerUserAllowedFunc(t *testing.T) {
	config.Global = &config.Config{ServerUserWhitelist: []string{"alice"}}
	if !serverUserAllowed("alice") {
		t.Error("alice should be allowed")
	}
	if serverUserAllowed("bob") {
		t.Error("bob should be denied")
	}

	// Empty whitelist -> permit everyone.
	config.Global = &config.Config{}
	if !serverUserAllowed("bob") {
		t.Error("empty whitelist should allow everyone")
	}
}

// TestRequireServerUser verifies the middleware behavior (F3 / A8):
//   - AuthEnabled=false            -> always pass (no user needed)
//   - AuthEnabled=true, empty wl  -> pass (gate only arms with a whitelist)
//   - AuthEnabled=true, non-wl    -> 1002 PermissionDenied
//   - AuthEnabled=true, in wl     -> pass
func TestRequireServerUser(t *testing.T) {
	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0})
	}

	t.Run("auth-disabled-passthrough", func(t *testing.T) {
		config.Global = &config.Config{AuthEnabled: false, JWTSecret: "sec"}
		r := gin.New()
		r.Use(RequireServerUser())
		r.GET("/x", handler)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/x", nil)
		r.ServeHTTP(w, req)
		assertCode(t, w, 0)
	})

	t.Run("auth-enabled-empty-whitelist", func(t *testing.T) {
		config.Global = &config.Config{AuthEnabled: true, JWTSecret: "sec"}
		r := gin.New()
		r.Use(RequireServerUser())
		r.GET("/x", handler)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/x", nil)
		r.ServeHTTP(w, req)
		assertCode(t, w, 0)
	})

	t.Run("auth-enabled-denied", func(t *testing.T) {
		config.Global = &config.Config{
			AuthEnabled: true, ServerUserWhitelist: []string{"alice"}, JWTSecret: "sec",
		}
		r := gin.New()
		r.Use(JWTAuth(), RequireServerUser())
		r.GET("/x", handler)
		tok, err := security.GenerateToken("bob", "sec", 60)
		if err != nil {
			t.Fatalf("gen token: %v", err)
		}
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/x", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		r.ServeHTTP(w, req)
		assertCode(t, w, 1002)
	})

	t.Run("auth-enabled-allowed", func(t *testing.T) {
		config.Global = &config.Config{
			AuthEnabled: true, ServerUserWhitelist: []string{"alice"}, JWTSecret: "sec",
		}
		r := gin.New()
		r.Use(JWTAuth(), RequireServerUser())
		r.GET("/x", handler)
		tok, err := security.GenerateToken("alice", "sec", 60)
		if err != nil {
			t.Fatalf("gen token: %v", err)
		}
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/x", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		r.ServeHTTP(w, req)
		assertCode(t, w, 0)
	})
}

// TestRouterRegistersNewEndpoints verifies the file-transfer endpoints, the
// public /api/public/config endpoint, and that /api/credentials is now under
// the JWT group WITHOUT RequireServerUser (C4 / C7).
func TestRouterRegistersNewEndpoints(t *testing.T) {
	reg := gateway.NewSessionRegistry(0)

	// --- Init an isolated store so per-user handlers can run. ---
	dir := t.TempDir()
	cfg := &config.Config{
		AuthEnabled: false, JWTSecret: "sec", DownloadDir: t.TempDir(),
		DBPath: filepath.Join(dir, "test.db"),
	}
	config.Global = cfg
	if err := store.Init(cfg); err != nil {
		t.Fatalf("store init: %v", err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatalf("store migrate: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// --- Auth disabled: endpoints are reachable. ---
	config.Global = &config.Config{AuthEnabled: false, JWTSecret: "sec", DownloadDir: t.TempDir(),
		DBPath: filepath.Join(dir, "test.db")}
	r := NewRouter(reg, "test")

	// GET /api/public/config -> {auth_enabled:false, version:"test"}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/public/config", nil)
	r.ServeHTTP(w, req)
	assertCode(t, w, 0)
	var pc struct {
		Data struct {
			AuthEnabled bool   `json:"auth_enabled"`
			Version     string `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &pc); err != nil {
		t.Fatalf("decode public/config: %v (%s)", err, w.Body.String())
	}
	if pc.Data.AuthEnabled {
		t.Error("auth_enabled should be false")
	}
	if pc.Data.Version != "test" {
		t.Errorf("version = %q want test", pc.Data.Version)
	}

	// GET /api/ssh-config-hosts
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/ssh-config-hosts", nil)
	r.ServeHTTP(w, req)
	assertCode(t, w, 0)

	// GET /api/transfer-bins
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/transfer-bins", nil)
	r.ServeHTTP(w, req)
	assertCode(t, w, 0)

	// POST /api/transfer-upload with no multipart file -> 4001 (route exists).
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/transfer-upload", nil)
	r.ServeHTTP(w, req)
	assertCode(t, w, 4001)

	// GET /api/transfer-file with no path -> 4001 (route exists).
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/transfer-file", nil)
	r.ServeHTTP(w, req)
	assertCode(t, w, 4001)

	// GET /api/credentials route exists (HTTP 200; handler runs without a
	// user context when auth is disabled and returns a non-404).
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/credentials", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/api/credentials route missing (status %d)", w.Code)
	}

	// --- Auth enabled: JWTAuth gates the protected group. ---
	config.Global = &config.Config{AuthEnabled: true, JWTSecret: "sec", DownloadDir: t.TempDir(),
		DBPath: filepath.Join(dir, "test.db")}
	rAuth := NewRouter(reg, "test")

	// No token -> JWTAuth aborts with 1001.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/ssh-config-hosts", nil)
	rAuth.ServeHTTP(w, req)
	assertCode(t, w, 1001)

	// POST /api/hostkey with no token -> 1001 (JWTAuth before RequireServerUser).
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/hostkey", nil)
	rAuth.ServeHTTP(w, req)
	assertCode(t, w, 1001)

	// GET /api/credentials with a valid token for a NON-whitelisted user must
	// NOT be blocked by RequireServerUser (no 1002). The handler runs to
	// completion, returning code 0 (empty list for the new user, C4).
	if _, err := store.CreateUser("alice", "pw", "user"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	tok, err := security.GenerateToken("alice", "sec", 60)
	if err != nil {
		t.Fatalf("gen token: %v", err)
	}
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/credentials", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rAuth.ServeHTTP(w, req)
	assertCode(t, w, 0)
}
