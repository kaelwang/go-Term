package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// DefaultSettingsJSON returns the canonical default settings as JSON. Per-user
// settings override these fields (C5): no new config keys are introduced.
func DefaultSettingsJSON() string {
	d := map[string]interface{}{
		"theme":                "dark",
		"fontSize":             14,
		"fontFamily":           "Menlo, Monaco, Consolas, 'Courier New', monospace",
		"encoding":             "utf-8",
		"cursorBlink":          true,
		"cursorStyle":          "block",
		"scrollback":           10000,
		"webgl":                false,
		"lineHeight":           1.0,
		"letterSpacing":        0,
		"defaultProtocol":      "ssh",
		"defaultAuthType":      "password",
		"defaultTransfer":      "sftp",
		"recvAutoDownload":     false,
		"strictHostKeyChecking": false,
		"connectTimeoutSec":    15,
	}
	b, _ := json.Marshal(d)
	return string(b)
}

// GetSettings returns the user's settings merged over the defaults.
func GetSettings(username string) (string, error) {
	uid, err := GetUserID(username)
	if err != nil {
		// User lookup failure (e.g. auth disabled) -> just return defaults.
		return DefaultSettingsJSON(), nil
	}
	var data string
	err = db.QueryRow(`SELECT data FROM user_settings WHERE user_id = ?`, uid).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultSettingsJSON(), nil
	}
	if err != nil {
		return DefaultSettingsJSON(), err
	}
	return mergeSettings(DefaultSettingsJSON(), data), nil
}

// SetSettings upserts the user's settings JSON.
func SetSettings(username, data string) error {
	uid, err := GetUserID(username)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT INTO user_settings (user_id, data) VALUES (?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET data = excluded.data`,
		uid, data,
	)
	return err
}

// mergeSettings overlays override fields on top of the base JSON object.
func mergeSettings(base, override string) string {
	var b, o map[string]interface{}
	if err := json.Unmarshal([]byte(base), &b); err != nil || b == nil {
		b = map[string]interface{}{}
	}
	if err := json.Unmarshal([]byte(override), &o); err != nil {
		return base
	}
	for k, v := range o {
		b[k] = v
	}
	out, err := json.Marshal(b)
	if err != nil {
		return base
	}
	return string(out)
}
