package store

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/security"
)

// testInit spins up an isolated SQLite database for a single test.
func testInit(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(dir, "test.db"), VaultKeyRaw: "test-key"}
	if err := Init(cfg); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = Close() })
	config.Global = cfg
}

func TestUserLifecycle(t *testing.T) {
	testInit(t)

	if n, _ := CountUsers(); n != 0 {
		t.Fatalf("initial count = %d want 0", n)
	}
	u, err := CreateUser("alice", "pw", "user")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if n, _ := CountUsers(); n != 1 {
		t.Fatalf("count = %d want 1", n)
	}
	// wrong password
	if ok, _ := CheckPassword("alice", "bad"); ok {
		t.Error("wrong password should fail")
	}
	if ok, _ := CheckPassword("alice", "pw"); !ok {
		t.Error("correct password should succeed")
	}
	if role, _ := GetUserRole("alice"); role != "user" {
		t.Errorf("role = %q want user", role)
	}
	if err := ResetPassword(u.ID, "newpw"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if ok, _ := CheckPassword("alice", "newpw"); !ok {
		t.Error("new password should succeed")
	}
	if err := DeleteUser(u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n, _ := CountUsers(); n != 0 {
		t.Fatalf("count after delete = %d want 0", n)
	}
}

func TestCredentialEncryptRoundTrip(t *testing.T) {
	testInit(t)
	if _, err := CreateUser("alice", "pw", "user"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	plain := `{"username":"root","password":"s3cret"}`
	enc, err := security.Encrypt(plain, "test-key")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	id, err := CreateCredential("alice", "c1", "password", enc, `{"note":"x"}`)
	if err != nil {
		t.Fatalf("create cred: %v", err)
	}
	list, err := ListCredentials("alice")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "c1" || list[0].Value != "" {
		t.Fatalf("list mismatch: %+v", list)
	}
	pc, err := GetCredentialDecrypted("alice", strconv.Itoa(id))
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if pc.Username != "root" || pc.Password != "s3cret" || pc.Type != "password" {
		t.Errorf("decrypted = %+v", pc)
	}
	// wrong user cannot decrypt
	if _, err := GetCredentialDecrypted("bob", strconv.Itoa(id)); err == nil {
		t.Error("bob should not be able to read alice's credential")
	}
}

func TestConnectionCRUD(t *testing.T) {
	testInit(t)
	if _, err := CreateUser("alice", "pw", "user"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	g, err := CreateGroup("alice", "Default", nil, 0)
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	conn, err := CreateConnection("alice", &Connection{
		GroupID: &g.ID, Name: "srv1", Protocol: "ssh", Host: "10.0.0.1", Port: 22,
		Username: "root", AuthType: "password", CredentialID: nil,
	})
	if err != nil {
		t.Fatalf("create conn: %v", err)
	}
	list, err := ListConnections("alice")
	if err != nil {
		t.Fatalf("list conn: %v", err)
	}
	if len(list) != 1 || list[0].Name != "srv1" || list[0].GroupID == nil || *list[0].GroupID != g.ID {
		t.Fatalf("list conn mismatch: %+v", list)
	}
	conn.Host = "10.0.0.2"
	if err := UpdateConnection("alice", conn.ID, map[string]interface{}{"host": "10.0.0.2"}); err != nil {
		t.Fatalf("update conn: %v", err)
	}
	list, _ = ListConnections("alice")
	if list[0].Host != "10.0.0.2" {
		t.Errorf("update not applied: %+v", list[0])
	}
	if err := DeleteConnection("alice", conn.ID); err != nil {
		t.Fatalf("delete conn: %v", err)
	}
	list, _ = ListConnections("alice")
	if len(list) != 0 {
		t.Errorf("expected empty after delete, got %d", len(list))
	}
}

func TestGroupCRUDAndReorder(t *testing.T) {
	testInit(t)
	if _, err := CreateUser("alice", "pw", "user"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	a, _ := CreateGroup("alice", "A", nil, 0)
	b, _ := CreateGroup("alice", "B", nil, 1)
	list, err := ListGroups("alice")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(list))
	}
	// swap order: move B above A by giving it a smaller sort_order (C6).
	if err := UpdateGroup("alice", b.ID, map[string]interface{}{"name": "B", "sort_order": -1}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	list, _ = ListGroups("alice")
	if list[0].ID != b.ID {
		t.Errorf("expected B first after reorder, got %+v", list)
	}
	_ = a
	if err := DeleteGroup("alice", b.ID); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	list, _ = ListGroups("alice")
	if len(list) != 1 {
		t.Errorf("expected 1 group after delete, got %d", len(list))
	}
}

// Bug B4 regression: moving a connection to a group via a partial payload
// {"group_id": N} must not clear the connection's other columns.
func TestConnectionPartialUpdatePreservesFields(t *testing.T) {
	testInit(t)
	if _, err := CreateUser("alice", "pw", "user"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	g, err := CreateGroup("alice", "Target", nil, 0)
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	credID := 42
	conn, err := CreateConnection("alice", &Connection{
		Name: "srv1", Protocol: "ssh", Host: "10.0.0.1", Port: 22,
		Username: "root", AuthType: "password", CredentialID: &credID,
	})
	if err != nil {
		t.Fatalf("create conn: %v", err)
	}

	// Only the group is moved; everything else must survive.
	if err := UpdateConnection("alice", conn.ID, map[string]interface{}{"group_id": g.ID}); err != nil {
		t.Fatalf("partial update: %v", err)
	}
	got, err := GetConnection("alice", conn.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "srv1" {
		t.Errorf("name cleared: got %q", got.Name)
	}
	if got.Host != "10.0.0.1" {
		t.Errorf("host cleared: got %q", got.Host)
	}
	if got.Protocol != "ssh" {
		t.Errorf("protocol cleared: got %q", got.Protocol)
	}
	if got.Port != 22 {
		t.Errorf("port cleared: got %d", got.Port)
	}
	if got.Username != "root" {
		t.Errorf("username cleared: got %q", got.Username)
	}
	if got.AuthType != "password" {
		t.Errorf("auth_type cleared: got %q", got.AuthType)
	}
	if got.CredentialID == nil || *got.CredentialID != credID {
		t.Errorf("credential_id cleared: got %v", got.CredentialID)
	}
	if got.SSHConfigHost != "" {
		t.Errorf("ssh_config_host changed: got %q", got.SSHConfigHost)
	}
	if got.GroupID == nil || *got.GroupID != g.ID {
		t.Errorf("group_id not applied: got %v", got.GroupID)
	}
}

// Bug B3 regression: renaming a group via {"name":"x"} must not reset its
// sort_order to 0.
func TestGroupRenamePreservesSortOrder(t *testing.T) {
	testInit(t)
	if _, err := CreateUser("alice", "pw", "user"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	g, err := CreateGroup("alice", "Old", nil, 5)
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	if err := UpdateGroup("alice", g.ID, map[string]interface{}{"name": "New"}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	got, err := GetGroup("alice", g.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "New" {
		t.Errorf("name not updated: got %q", got.Name)
	}
	if got.SortOrder != 5 {
		t.Errorf("sort_order reset to %d, want 5", got.SortOrder)
	}
}

func TestSettingsMerge(t *testing.T) {
	testInit(t)
	if _, err := CreateUser("alice", "pw", "user"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	data, err := GetSettings("alice")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !contains(data, `"theme":"dark"`) {
		t.Errorf("defaults missing theme: %s", data)
	}
	if err := SetSettings("alice", `{"theme":"light","fontSize":20}`); err != nil {
		t.Fatalf("set: %v", err)
	}
	data, _ = GetSettings("alice")
	if !contains(data, `"theme":"light"`) || !contains(data, `"fontSize":20`) {
		t.Errorf("user settings not applied: %s", data)
	}
	// unspecified fields fall back to defaults
	if !contains(data, `"scrollback":10000`) {
		t.Errorf("missing default scrollback: %s", data)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
