package store

import "encoding/json"

// User is a go-Term account. The password hash is never serialized to JSON.
type User struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

// ConnectionGroup is a single-level grouping of saved connections.
type ConnectionGroup struct {
	ID        int    `json:"id"`
	UserID    int    `json:"user_id"`
	Name      string `json:"name"`
	ParentID  *int   `json:"parent_id"`
	SortOrder int    `json:"sort_order"`
}

// Connection is a persisted connection owned by a user. Passwords / private
// keys are NOT stored here; they live in the credential vault and are
// referenced by credential_id.
type Connection struct {
	ID            int             `json:"id"`
	UserID        int             `json:"user_id"`
	GroupID       *int            `json:"group_id"`
	Name          string          `json:"name"`
	Protocol      string          `json:"protocol"`
	Host          string          `json:"host"`
	Port          int             `json:"port"`
	Username      string          `json:"username"`
	AuthType      string          `json:"auth_type"`
	CredentialID  *int            `json:"credential_id"`
	SSHConfigHost string          `json:"ssh_config_host"`
	Proxy         json.RawMessage `json:"proxy"`
	Hops          json.RawMessage `json:"hops"`
	Options       json.RawMessage `json:"options"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

// Credential is an encrypted vault entry. The ciphertext (Value) is never
// serialized in list responses.
type Credential struct {
	ID        int    `json:"id"`
	UserID    int    `json:"user_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Value     string `json:"-"`
	Meta      string `json:"meta"`
	CreatedAt string `json:"created_at"`
}

// CredentialSecret is the decrypted plaintext of a vault entry.
type CredentialSecret struct {
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

// UserSetting is the per-user settings row keyed by user_id.
type UserSetting struct {
	UserID int    `json:"user_id"`
	Data   string `json:"data"`
}
