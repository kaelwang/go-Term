package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kaelwang/go-Term/internal/gateway"
	"github.com/kaelwang/go-Term/internal/security"
	"github.com/kaelwang/go-Term/internal/store"
)

// TestVaultConnectionsSettingsFlow exercises the credential vault, saved
// connections, per-user settings, /me, and admin-gated user management over the
// real router with JWT auth enabled (T-V2 / T-V3 / T-V4 / T-V5).
func TestVaultConnectionsSettingsFlow(t *testing.T) {
	cfg := setupTestStore(t)
	cfg.AuthEnabled = true
	if _, err := store.CreateUser("alice", "pw", "user"); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if _, err := store.CreateUser("admin", "pw", "admin"); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	reg := gateway.NewSessionRegistry(0)
	r := NewRouter(reg, "test")
	aliceTok, _ := security.GenerateToken("alice", "secret", 60)
	adminTok, _ := security.GenerateToken("admin", "secret", 60)

	do := func(method, path, tok, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		var rdr *strings.Reader
		if body != "" {
			rdr = strings.NewReader(body)
		} else {
			rdr = strings.NewReader("")
		}
		req, _ := http.NewRequest(method, path, rdr)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		r.ServeHTTP(w, req)
		return w
	}

	// /api/me
	w := do("GET", "/api/me", aliceTok, "")
	assertCode(t, w, 0)
	var me struct {
		Data struct {
			User string `json:"user"`
			Role string `json:"role"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &me)
	if me.Data.User != "alice" || me.Data.Role != "user" {
		t.Fatalf("me = %+v", me.Data)
	}

	// Credential vault CRUD.
	w = do("POST", "/api/credentials", aliceTok,
		`{"name":"c1","type":"password","username":"root","password":"p"}`)
	assertCode(t, w, 0)
	var created struct {
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	credID := created.Data.ID
	if credID == 0 {
		t.Fatal("expected credential id")
	}

	w = do("GET", "/api/credentials", aliceTok, "")
	assertCode(t, w, 0)
	var list struct {
		Data []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Data) != 1 || list.Data[0].Name != "c1" {
		t.Fatalf("cred list = %+v", list.Data)
	}

	w = do("GET", "/api/credentials/"+itoa(credID)+"/secret", aliceTok, "")
	assertCode(t, w, 0)
	var secret struct {
		Data struct {
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &secret)
	if secret.Data.Username != "root" || secret.Data.Password != "p" {
		t.Fatalf("secret = %+v", secret.Data)
	}

	// Saved connections CRUD.
	w = do("POST", "/api/connections", aliceTok,
		`{"name":"srv","protocol":"ssh","host":"1.2.3.4","port":22,"username":"root","auth_type":"password"}`)
	assertCode(t, w, 0)
	var connCreated struct {
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &connCreated)
	connID := connCreated.Data.ID
	if connID == 0 {
		t.Fatal("expected connection id")
	}

	w = do("GET", "/api/connections", aliceTok, "")
	assertCode(t, w, 0)
	var connList struct {
		Data []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &connList)
	if len(connList.Data) != 1 || connList.Data[0].Name != "srv" {
		t.Fatalf("conn list = %+v", connList.Data)
	}

	// Per-user settings.
	w = do("PUT", "/api/settings", aliceTok, `{"theme":"light","fontSize":20}`)
	assertCode(t, w, 0)
	w = do("GET", "/api/settings", aliceTok, "")
	assertCode(t, w, 0)
	var settings struct {
		Data struct {
			Theme    string `json:"theme"`
			FontSize int    `json:"fontSize"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &settings)
	if settings.Data.Theme != "light" || settings.Data.FontSize != 20 {
		t.Fatalf("settings = %+v", settings.Data)
	}

	// Admin gating on user management.
	w = do("GET", "/api/users", aliceTok, "")
	assertCode(t, w, 1002) // non-admin denied
	w = do("GET", "/api/users", adminTok, "")
	assertCode(t, w, 0) // admin allowed

	w = do("POST", "/api/users", adminTok,
		`{"username":"carol","password":"pw","role":"user"}`)
	assertCode(t, w, 0)

	// Cleanup.
	do("DELETE", "/api/credentials/"+itoa(credID), aliceTok, "")
	do("DELETE", "/api/connections/"+itoa(connID), aliceTok, "")
}
