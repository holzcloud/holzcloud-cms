package auth

import (
	"context"
	"database/sql"
	"time"
)

// SQLiteStore implements scs.Store for modernc.org/sqlite (no CGO).
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a session store backed by the given *sql.DB.
// Pass the write pool (DB.Write) — sessions do writes.
// The caller schedules DeleteExpired through internal/jobs rather than the
// store spawning its own goroutine, so cleanup stops with the rest of the
// process on SIGTERM.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

// DeleteExpired removes sessions whose expiry has passed.
func (s *SQLiteStore) DeleteExpired(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expiry < julianday('now')")
	return err
}

// Find returns the session data for the given token, if it exists and has not expired.
func (s *SQLiteStore) Find(token string) ([]byte, bool, error) {
	row := s.db.QueryRow(
		"SELECT data FROM sessions WHERE token = $1 AND julianday('now') < expiry", token)
	var data []byte
	err := row.Scan(&data)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// Commit inserts or replaces a session.
func (s *SQLiteStore) Commit(token string, data []byte, expiry time.Time) error {
	_, err := s.db.Exec(
		"REPLACE INTO sessions (token, data, expiry) VALUES ($1, $2, julianday($3))",
		token, data, expiry.UTC().Format("2006-01-02T15:04:05.999"))
	return err
}

// Delete removes a session by token.
func (s *SQLiteStore) Delete(token string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token = $1", token)
	return err
}

// All returns all non-expired sessions. Required by the scs.IterableStore interface.
func (s *SQLiteStore) All() (map[string][]byte, error) {
	rows, err := s.db.Query(
		"SELECT token, data FROM sessions WHERE julianday('now') < expiry")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := make(map[string][]byte)
	for rows.Next() {
		var token string
		var data []byte
		if err := rows.Scan(&token, &data); err != nil {
			return nil, err
		}
		sessions[token] = data
	}
	return sessions, rows.Err()
}
