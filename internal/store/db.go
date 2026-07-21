// Package store provides a thin, self-contained persistence layer for
// go-Term backed by a pure-Go SQLite driver (modernc.org/sqlite, zero CGO).
// All tables are created idempotently on first start; no external migration
// framework is used.
package store

import (
	"database/sql"
	"fmt"

	"github.com/kaelwang/go-Term/internal/config"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (no CGO)
	"go.uber.org/zap"
)

// db is the process-wide SQLite connection shared by all store helpers.
var db *sql.DB

// migrateStatements are the idempotent DDL statements executed on startup.
var migrateStatements = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role          TEXT NOT NULL DEFAULT 'user',
		created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,

	`CREATE TABLE IF NOT EXISTS connection_groups (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id    INTEGER NOT NULL,
		name       TEXT NOT NULL,
		parent_id  INTEGER,
		sort_order INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY(user_id)   REFERENCES users(id)             ON DELETE CASCADE,
		FOREIGN KEY(parent_id) REFERENCES connection_groups(id) ON DELETE SET NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_groups_user ON connection_groups(user_id)`,

	`CREATE TABLE IF NOT EXISTS connections (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id         INTEGER NOT NULL,
		group_id        INTEGER,
		name            TEXT NOT NULL,
		protocol        TEXT NOT NULL,
		host            TEXT,
		port            INTEGER,
		username        TEXT,
		auth_type       TEXT,
		credential_id   INTEGER,
		ssh_config_host TEXT,
		proxy           TEXT,
		hops            TEXT,
		options         TEXT,
		created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id)      REFERENCES users(id)             ON DELETE CASCADE,
		FOREIGN KEY(group_id)     REFERENCES connection_groups(id) ON DELETE SET NULL,
		FOREIGN KEY(credential_id) REFERENCES credentials(id)      ON DELETE SET NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_conn_user  ON connections(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_conn_group ON connections(group_id)`,

	`CREATE TABLE IF NOT EXISTS credentials (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id    INTEGER NOT NULL,
		name       TEXT NOT NULL,
		type       TEXT NOT NULL,
		value      TEXT NOT NULL,
		meta       TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_cred_user ON credentials(user_id)`,

	`CREATE TABLE IF NOT EXISTS user_settings (
		user_id INTEGER PRIMARY KEY,
		data    TEXT NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	)`,

	// Brute-force login protection state, keyed by client IP (not username, so
	// an attacker cannot rotate usernames to dodge the counter). A row exists
	// only while an IP has accumulated failures; a successful login or an admin
	// unlock deletes it. Persistence survives process restarts.
	`CREATE TABLE IF NOT EXISTS login_lockouts (
		ip            TEXT PRIMARY KEY,
		fail_count    INTEGER NOT NULL DEFAULT 0,
		lock_until    INTEGER,            -- unix seconds; NULL/0 means not locked
		banned        INTEGER NOT NULL DEFAULT 0,
		last_username TEXT,
		updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_lockouts_updated ON login_lockouts(updated_at)`,
}

// Init opens the SQLite database, tunes the connection pool for SQLite's
// single-writer model (SetMaxOpenConns(1) avoids write-lock contention), and
// verifies connectivity. It does not create tables; call Migrate afterwards.
func Init(cfg *config.Config) error {
	path := cfg.DBPath
	if path == "" {
		path = "./go-Term.db"
	}
	var err error
	db, err = sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite serializes writes; a single connection prevents "database is
	// locked" errors under concurrent management-plane requests.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping sqlite: %w", err)
	}
	zap.L().Info("sqlite store initialized", zap.String("path", path))
	return nil
}

// Close releases the database connection.
func Close() error {
	if db == nil {
		return nil
	}
	err := db.Close()
	db = nil
	return err
}

// Migrate creates all tables idempotently.
func Migrate() error {
	if db == nil {
		return fmt.Errorf("store not initialized")
	}
	for _, ddl := range migrateStatements {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}
