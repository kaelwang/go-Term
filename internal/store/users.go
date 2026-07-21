package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/kaelwang/go-Term/internal/config"
	"golang.org/x/crypto/bcrypt"
	"go.uber.org/zap"
)

// CreateUser inserts a new user with a bcrypt-hashed password.
func CreateUser(username, password, role string) (*User, error) {
	if role != "admin" && role != "user" {
		role = "user"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	res, err := db.Exec(
		`INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`,
		username, string(hash), role,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{
		ID:        int(id),
		Username:  username,
		Role:      role,
		CreatedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// GetUserByUsername looks up a user by login name, returning nil when absent.
func GetUserByUsername(username string) (*User, error) {
	u := &User{}
	var hash string
	err := db.QueryRow(
		`SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &hash, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// CheckPassword verifies a plaintext password against the stored hash. It
// returns (false, nil) when the user does not exist (fail-safe auth denial).
func CheckPassword(username, password string) (bool, error) {
	var hash string
	err := db.QueryRow(`SELECT password_hash FROM users WHERE username = ?`, username).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return false, err
	}
	return true, nil
}

// GetUserRole returns the role for a username (empty string on lookup failure).
func GetUserRole(username string) (string, error) {
	var role string
	err := db.QueryRow(`SELECT role FROM users WHERE username = ?`, username).Scan(&role)
	if err != nil {
		return "", err
	}
	return role, nil
}

// GetUserID returns the numeric id for a username.
func GetUserID(username string) (int, error) {
	var id int
	err := db.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ListUsers returns all users ordered by id.
func ListUsers() ([]User, error) {
	rows, err := db.Query(`SELECT id, username, role, created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// DeleteUser removes a user (cascades connections / credentials / settings).
func DeleteUser(id int) error {
	_, err := db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

// UpdateUser patches a user's profile. password == "" leaves the stored
// password unchanged; otherwise it is bcrypt-rehashed. Username and role are
// always applied (role is normalized to "user" when invalid).
func UpdateUser(id int, username, role, password string) error {
	if username == "" {
		return errors.New("username required")
	}
	if role != "admin" && role != "user" {
		role = "user"
	}
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = db.Exec(
			`UPDATE users SET username = ?, role = ?, password_hash = ? WHERE id = ?`,
			username, role, string(hash), id,
		)
		if err != nil {
			return err
		}
		return nil
	}
	_, err := db.Exec(
		`UPDATE users SET username = ?, role = ? WHERE id = ?`,
		username, role, id,
	)
	return err
}

// ResetPassword changes a user's password.
func ResetPassword(id int, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	res, err := db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CountUsers returns the total number of users.
func CountUsers() (int, error) {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// BootstrapAdmin inserts the first admin when the DB is empty and bootstrap
// credentials are provided; otherwise it logs a fail-closed warning (C2).
func BootstrapAdmin(cfg *config.Config) error {
	if db == nil {
		return nil
	}
	n, err := CountUsers()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if cfg.BootstrapAdminUser == "" || cfg.BootstrapAdminPass == "" {
		zap.L().Warn("no users found and GOTERM_BOOTSTRAP_ADMIN_* not set; login is disabled (fail-closed)")
		return nil
	}
	if _, err := CreateUser(cfg.BootstrapAdminUser, cfg.BootstrapAdminPass, "admin"); err != nil {
		return err
	}
	zap.L().Info("bootstrapped initial admin user", zap.String("username", cfg.BootstrapAdminUser))
	return nil
}
