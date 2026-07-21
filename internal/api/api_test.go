package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/security"
	"github.com/kaelwang/go-Term/internal/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupTestStore initializes an isolated SQLite store for a test and returns a
// config wired to it. The store is closed automatically when the test ends.
func setupTestStore(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		DBPath:           filepath.Join(dir, "test.db"),
		JWTSecret:        "secret",
		JWTExpireMinutes: 60,
	}
	config.Global = cfg
	if err := store.Init(cfg); err != nil {
		t.Fatalf("store init: %v", err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatalf("store migrate: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return cfg
}

// TestRespondEnvelope verifies the unified REST envelope shape
// {"code","message","data"} and that HTTP status is always 200.
func TestRespondEnvelope(t *testing.T) {
	cases := []struct {
		code    int
		message string
		data    interface{}
	}{
		{0, "ok", gin.H{"token": "abc"}},
		{4001, "bad params", nil},
		{1001, "missing token", nil},
		{2004, "unsupported protocol", nil},
		{3001, "transfer failed", gin.H{"detail": "x"}},
	}
	for _, tc := range cases {
		t.Run(strings.ReplaceAll(http.StatusText(0), " ", "")+itoa(tc.code), func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			respond(c, tc.code, tc.message, tc.data)

			if w.Code != http.StatusOK {
				t.Fatalf("HTTP status = %d want 200", w.Code)
			}
			var body map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not JSON: %v (%s)", err, w.Body.String())
			}
			if int(body["code"].(float64)) != tc.code {
				t.Errorf("code = %v want %d", body["code"], tc.code)
			}
			if body["message"] != tc.message {
				t.Errorf("message = %v want %q", body["message"], tc.message)
			}
			if _, ok := body["data"]; !ok {
				t.Error("envelope missing 'data' field")
			}
		})
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// TestLoginBadParams verifies malformed JSON yields code 4001.
func TestLoginBadParams(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/login", strings.NewReader("not-json"))
	c.Request.Header.Set("Content-Type", "application/json")
	LoginHandler(c)

	assertCode(t, w, 4001)
}

// TestLoginSuccess verifies a valid login returns code 0, a parseable JWT, and
// the user's role (B2 / C4 — login is now backed by the users table + bcrypt).
func TestLoginSuccess(t *testing.T) {
	setupTestStore(t)
	if _, err := store.CreateUser("alice", "pw", "user"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/login", strings.NewReader(`{"user":"alice","password":"pw"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	LoginHandler(c)

	assertCode(t, w, 0)
	var body struct {
		Data struct {
			Token string `json:"token"`
			User  string `json:"user"`
			Role  string `json:"role"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.User != "alice" || body.Data.Role != "user" {
		t.Errorf("user/role = %q/%q want alice/user", body.Data.User, body.Data.Role)
	}
	claims, err := security.ParseToken(body.Data.Token, "secret")
	if err != nil {
		t.Fatalf("issued token unparseable: %v", err)
	}
	if claims.User != "alice" {
		t.Errorf("token user = %q want alice", claims.User)
	}
}

// TestLoginWrongPassword verifies both a wrong password and an unknown user are
// rejected with 1001 (fail-closed, C2 / C4 — no whitelist short-circuit).
func TestLoginWrongPassword(t *testing.T) {
	setupTestStore(t)
	if _, err := store.CreateUser("alice", "pw", "user"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/login", strings.NewReader(`{"user":"alice","password":"wrong"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	LoginHandler(c)
	assertCode(t, w, 1001)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request, _ = http.NewRequest("POST", "/api/login", strings.NewReader(`{"user":"nobody","password":"x"}`))
	c2.Request.Header.Set("Content-Type", "application/json")
	LoginHandler(c2)
	assertCode(t, w2, 1001)
}

// TestJWTAuthMiddleware verifies the middleware behavior under ENABLE_AUTH.
func TestJWTAuthMiddleware(t *testing.T) {
	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user": c.GetString("user")})
	}

	t.Run("auth-disabled-passthrough", func(t *testing.T) {
		config.Global = &config.Config{AuthEnabled: false, JWTSecret: "sec"}
		r := gin.New()
		r.Use(JWTAuth())
		r.GET("/x", handler)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/x", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("auth-disabled should pass through, got %d", w.Code)
		}
	})

	t.Run("auth-enabled-no-token", func(t *testing.T) {
		config.Global = &config.Config{AuthEnabled: true, JWTSecret: "sec"}
		r := gin.New()
		r.Use(JWTAuth())
		r.GET("/x", handler)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/x", nil)
		r.ServeHTTP(w, req)
		assertCode(t, w, 1001)
	})

	t.Run("auth-enabled-bad-token", func(t *testing.T) {
		config.Global = &config.Config{AuthEnabled: true, JWTSecret: "sec"}
		r := gin.New()
		r.Use(JWTAuth())
		r.GET("/x", handler)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/x", nil)
		req.Header.Set("Authorization", "Bearer not-a-real-token")
		r.ServeHTTP(w, req)
		assertCode(t, w, 1001)
	})

	t.Run("auth-enabled-valid-token", func(t *testing.T) {
		config.Global = &config.Config{AuthEnabled: true, JWTSecret: "sec"}
		r := gin.New()
		r.Use(JWTAuth())
		r.GET("/x", handler)

		tok, err := security.GenerateToken("u", "sec", 60)
		if err != nil {
			t.Fatalf("gen token: %v", err)
		}
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/x", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("valid token should pass, got %d (body %s)", w.Code, w.Body.String())
		}
		var body struct {
			Code int    `json:"code"`
			User string `json:"user"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v (raw %s)", err, w.Body.String())
		}
		if body.Code != 0 {
			t.Fatalf("middleware aborted valid token with code %d (body %s)", body.Code, w.Body.String())
		}
		if body.User != "u" {
			t.Errorf("middleware did not set user, got %q (raw %s)", body.User, w.Body.String())
		}
	})
}

// assertCode decodes the unified envelope and checks the numeric code.
func assertCode(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, w.Body.String())
	}
	if body.Code != want {
		t.Errorf("envelope code = %d want %d (message=%q)", body.Code, want, body.Message)
	}
}
