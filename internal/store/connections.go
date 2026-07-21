package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// scanConnection reads a connection row, converting empty JSON columns to nil
// so they serialize as JSON null rather than an empty string.
func scanConnection(rows *sql.Rows) (Connection, error) {
	var c Connection
	var proxy, hops, options []byte
	err := rows.Scan(
		&c.ID, &c.UserID, &c.GroupID, &c.Name, &c.Protocol, &c.Host, &c.Port,
		&c.Username, &c.AuthType, &c.CredentialID, &c.SSHConfigHost,
		&proxy, &hops, &options, &c.CreatedAt, &c.UpdatedAt,
	)
	if len(proxy) > 0 {
		c.Proxy = proxy
	}
	if len(hops) > 0 {
		c.Hops = hops
	}
	if len(options) > 0 {
		c.Options = options
	}
	return c, err
}

// ListConnections returns all connections for a user, including those without
// a group (group_id NULL maps to the virtual "未分组" group on the client).
func ListConnections(username string) ([]Connection, error) {
	uid, err := GetUserID(username)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(
		`SELECT id, user_id, group_id, name, protocol, host, port, username,
		        auth_type, credential_id, ssh_config_host, proxy, hops, options,
		        created_at, updated_at
		 FROM connections WHERE user_id = ? ORDER BY id`,
		uid,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Connection{}
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateConnection persists a new connection for a user.
func CreateConnection(username string, in *Connection) (*Connection, error) {
	uid, err := GetUserID(username)
	if err != nil {
		return nil, err
	}
	now := time.Now().Format(time.RFC3339)
	res, err := db.Exec(
		`INSERT INTO connections
		   (user_id, group_id, name, protocol, host, port, username, auth_type,
		    credential_id, ssh_config_host, proxy, hops, options)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uid, in.GroupID, in.Name, in.Protocol, in.Host, in.Port, in.Username,
		in.AuthType, in.CredentialID, in.SSHConfigHost,
		string(in.Proxy), string(in.Hops), string(in.Options),
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Connection{
		ID: int(id), UserID: uid, GroupID: in.GroupID, Name: in.Name,
		Protocol: in.Protocol, Host: in.Host, Port: in.Port, Username: in.Username,
		AuthType: in.AuthType, CredentialID: in.CredentialID, SSHConfigHost: in.SSHConfigHost,
		Proxy: in.Proxy, Hops: in.Hops, Options: in.Options,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// GetConnection fetches a single connection by id, enforcing ownership.
func GetConnection(username string, id int) (*Connection, error) {
	uid, err := GetUserID(username)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(
		`SELECT id, user_id, group_id, name, protocol, host, port, username,
		        auth_type, credential_id, ssh_config_host, proxy, hops, options,
		        created_at, updated_at
		 FROM connections WHERE id = ? AND user_id = ?`,
		id, uid,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("connection not found")
	}
	c, err := scanConnection(rows)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateConnection applies a partial update to an existing connection using a
// read-modify-write strategy: the stored record is loaded first, then only the
// fields present in `fields` overwrite the existing values before writing back.
//
// This is the fix for bug B4 (batch move to group). Previously the handler
// bound the request into a brand-new Connection struct and the store performed
// a full 12-column overwrite, so a payload of {"group_id": N} silently cleared
// name/host/protocol/port/username/auth_type/credential_id/ssh_config_host.
// Now an absent field keeps its stored value.
//
// `id` is taken from the URL path, not from the body, so it cannot be spoofed.
// `UserID` and `CreatedAt` are always preserved; `UpdatedAt` is bumped.
func UpdateConnection(username string, id int, fields map[string]interface{}) error {
	uid, err := GetUserID(username)
	if err != nil {
		return err
	}
	existing, err := GetConnection(username, id)
	if err != nil {
		return err
	}
	// Defense in depth: ownership is already enforced by GetConnection.
	if existing.UserID != uid {
		return fmt.Errorf("connection not found")
	}

	merged := *existing
	if v, ok := fields["group_id"]; ok {
		merged.GroupID = toIntPtr(v)
	}
	if v, ok := fields["name"]; ok {
		merged.Name = toString(v)
	}
	if v, ok := fields["protocol"]; ok {
		merged.Protocol = toString(v)
	}
	if v, ok := fields["host"]; ok {
		merged.Host = toString(v)
	}
	if v, ok := fields["port"]; ok {
		merged.Port = toInt(v)
	}
	if v, ok := fields["username"]; ok {
		merged.Username = toString(v)
	}
	if v, ok := fields["auth_type"]; ok {
		merged.AuthType = toString(v)
	}
	if v, ok := fields["credential_id"]; ok {
		merged.CredentialID = toIntPtr(v)
	}
	if v, ok := fields["ssh_config_host"]; ok {
		merged.SSHConfigHost = toString(v)
	}
	// proxy/hops/options are json.RawMessage columns; only overwrite when the
	// request actually carries them, otherwise the stored JSON is preserved.
	if v, ok := fields["proxy"]; ok {
		if raw := toRawMessage(v); len(raw) > 0 {
			merged.Proxy = raw
		}
	}
	if v, ok := fields["hops"]; ok {
		if raw := toRawMessage(v); len(raw) > 0 {
			merged.Hops = raw
		}
	}
	if v, ok := fields["options"]; ok {
		if raw := toRawMessage(v); len(raw) > 0 {
			merged.Options = raw
		}
	}

	res, err := db.Exec(
		`UPDATE connections SET
		   group_id = ?, name = ?, protocol = ?, host = ?, port = ?, username = ?,
		   auth_type = ?, credential_id = ?, ssh_config_host = ?, proxy = ?, hops = ?,
		   options = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND user_id = ?`,
		merged.GroupID, merged.Name, merged.Protocol, merged.Host, merged.Port, merged.Username,
		merged.AuthType, merged.CredentialID, merged.SSHConfigHost,
		string(merged.Proxy), string(merged.Hops), string(merged.Options),
		id, uid,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("connection not found")
	}
	return nil
}

// toInt extracts an int from a value decoded by json.Unmarshal (numbers arrive
// as float64). Unknown types fall back to 0.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}

// toIntPtr extracts an *int, mapping a JSON null (decoded as nil) to a real nil
// pointer so the column is written as SQL NULL. A non-nil number becomes &n.
func toIntPtr(v interface{}) *int {
	if v == nil {
		return nil
	}
	n := toInt(v)
	return &n
}

// toString extracts a string field. nil becomes ""; structured values are
// best-effort serialized (used defensively, should not happen for scalars).
func toString(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case nil:
		return ""
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// toRawMessage normalizes a request value into json.RawMessage so that
// proxy/hops/options can be stored verbatim. nil / invalid values yield an
// empty slice, signalling "keep the original column" to the caller.
func toRawMessage(v interface{}) json.RawMessage {
	if v == nil {
		return nil
	}
	if b, ok := v.(json.RawMessage); ok {
		return b
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

// DeleteConnection removes a connection.
func DeleteConnection(username string, id int) error {
	uid, err := GetUserID(username)
	if err != nil {
		return err
	}
	res, err := db.Exec(`DELETE FROM connections WHERE id = ? AND user_id = ?`, id, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("connection not found")
	}
	return nil
}
