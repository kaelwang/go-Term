package store

import (
	"database/sql"
	"errors"
	"time"
)

// Lockout records brute-force login-protection state for a client IP address.
// A row exists only while the IP has accumulated failures; a successful login
// or an admin unlock deletes it. State is persisted in SQLite so it survives
// process restarts (caveat: each process must share the same DB file to stay
// consistent across replicas).
type Lockout struct {
	IP        string
	FailCount int
	LockUntil int64 // unix seconds; 0 means not currently locked
	Banned    bool
	LastUser  string
	UpdatedAt time.Time
}

// GetLockout returns the lockout for an IP. A missing row is reported as an
// empty (non-banned, not-locked) Lockout rather than an error so callers can
// treat absence as "allowed".
func GetLockout(ip string) (*Lockout, error) {
	if db == nil {
		return &Lockout{IP: ip}, nil
	}
	l := &Lockout{IP: ip}
	var banned int
	var lockUntil sql.NullInt64
	var updated sql.NullTime
	err := db.QueryRow(
		`SELECT fail_count, lock_until, banned, last_username, updated_at
		 FROM login_lockouts WHERE ip = ?`, ip,
	).Scan(&l.FailCount, &lockUntil, &banned, &l.LastUser, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	if lockUntil.Valid {
		l.LockUntil = lockUntil.Int64
	}
	l.Banned = banned != 0
	if updated.Valid {
		l.UpdatedAt = updated.Time
	}
	return l, nil
}

// SaveLockout upserts the lockout row for an IP.
func SaveLockout(l *Lockout) error {
	if db == nil {
		return nil
	}
	banned := 0
	if l.Banned {
		banned = 1
	}
	_, err := db.Exec(
		`INSERT INTO login_lockouts (ip, fail_count, lock_until, banned, last_username, updated_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(ip) DO UPDATE SET
		   fail_count=excluded.fail_count,
		   lock_until=excluded.lock_until,
		   banned=excluded.banned,
		   last_username=excluded.last_username,
		   updated_at=CURRENT_TIMESTAMP`,
		l.IP, l.FailCount, nullInt64(l.LockUntil), banned, l.LastUser,
	)
	return err
}

// DeleteLockout removes the lockout for an IP (used on success and on admin
// unlock). A missing row is not an error.
func DeleteLockout(ip string) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(`DELETE FROM login_lockouts WHERE ip = ?`, ip)
	return err
}

// ListLockouts returns every tracked IP with accumulated failures, ordered by
// most recent activity first. It is used by the admin unlock UI.
func ListLockouts() ([]Lockout, error) {
	if db == nil {
		return nil, nil
	}
	rows, err := db.Query(
		`SELECT ip, fail_count, lock_until, banned, last_username, updated_at
		 FROM login_lockouts ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Lockout{}
	for rows.Next() {
		var l Lockout
		var banned int
		var lockUntil sql.NullInt64
		var updated sql.NullTime
		if err := rows.Scan(&l.IP, &l.FailCount, &lockUntil, &banned, &l.LastUser, &updated); err != nil {
			return nil, err
		}
		if lockUntil.Valid {
			l.LockUntil = lockUntil.Int64
		}
		l.Banned = banned != 0
		if updated.Valid {
			l.UpdatedAt = updated.Time
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func nullInt64(v int64) interface{} {
	if v == 0 {
		return nil
	}
	return v
}
