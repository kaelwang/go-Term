package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/protocol"
	"github.com/kaelwang/go-Term/internal/security"
)

// ListCredentials returns the non-secret metadata for a user's credentials
// (the encrypted value is never returned, B4).
func ListCredentials(username string) ([]Credential, error) {
	uid, err := GetUserID(username)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(
		`SELECT id, user_id, name, type, meta, created_at
		 FROM credentials WHERE user_id = ? ORDER BY id`,
		uid,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Credential{}
	for rows.Next() {
		var c Credential
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Meta, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateCredential inserts an encrypted credential. The caller is responsible
// for encrypting `value` with config.VaultKey() before calling.
func CreateCredential(username, name, ctype, value, meta string) (int, error) {
	uid, err := GetUserID(username)
	if err != nil {
		return 0, err
	}
	res, err := db.Exec(
		`INSERT INTO credentials (user_id, name, type, value, meta) VALUES (?, ?, ?, ?, ?)`,
		uid, name, ctype, value, meta,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

// UpdateCredential overwrites an existing credential (re-encrypts value).
func UpdateCredential(username, name, ctype, value, meta string, id int) error {
	uid, err := GetUserID(username)
	if err != nil {
		return err
	}
	res, err := db.Exec(
		`UPDATE credentials SET name = ?, type = ?, value = ?, meta = ?
		 WHERE id = ? AND user_id = ?`,
		name, ctype, value, meta, id, uid,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("credential not found")
	}
	return nil
}

// DeleteCredential removes a credential.
func DeleteCredential(username string, id int) error {
	uid, err := GetUserID(username)
	if err != nil {
		return err
	}
	res, err := db.Exec(`DELETE FROM credentials WHERE id = ? AND user_id = ?`, id, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("credential not found")
	}
	return nil
}

// GetCredentialDecrypted loads a credential by id (owned by username) and
// returns the decrypted protocol.Credential for use by the gateway (C1/C3).
// The id argument is the credential id as a string (as carried over the WS).
func GetCredentialDecrypted(username, id string) (*protocol.Credential, error) {
	uid, err := GetUserID(username)
	if err != nil {
		return nil, err
	}
	credID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid credential id")
	}
	var (
		ctype, value, meta string
	)
	err = db.QueryRow(
		`SELECT type, value, meta FROM credentials WHERE id = ? AND user_id = ?`,
		credID, uid,
	).Scan(&ctype, &value, &meta)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("credential not found")
	}
	if err != nil {
		return nil, err
	}
	plain, err := security.Decrypt(value, config.Global.VaultKey())
	if err != nil {
		return nil, err
	}
	var secret CredentialSecret
	if err := json.Unmarshal([]byte(plain), &secret); err != nil {
		return nil, err
	}
	pc := &protocol.Credential{
		Username:   secret.Username,
		Password:   secret.Password,
		PrivateKey: secret.PrivateKey,
		Passphrase: secret.Passphrase,
	}
	if ctype == "private_key" {
		pc.Type = "publickey"
	} else {
		pc.Type = "password"
	}
	return pc, nil
}
