package value

import (
	"context"
	"fmt"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

const timeLayout = "2006-01-02T15:04:05Z"

// Value is one named text of one website.
type Value struct {
	ID        int64
	WebsiteID int64
	// Key is what a page writes between [[wert: and ]], e.g.
	// "version-holzcloud-cms". It cannot be changed after creation: renaming a
	// key in use would silently empty every sentence that names it.
	Key string
	// Text is the one line that stands in the key's place.
	Text      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store handles SQL operations for values. Every statement carries the website
// id, so no method can read or write another website's value (D-03).
type Store struct {
	DB *db.DB
}

// NewStore creates a value store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

const columns = `id, website_id, key, value, created_at, updated_at`

func scan(row interface{ Scan(...any) error }) (*Value, error) {
	var v Value
	var createdAt, updatedAt string
	if err := row.Scan(&v.ID, &v.WebsiteID, &v.Key, &v.Text, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	v.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	v.UpdatedAt, _ = time.Parse(timeLayout, updatedAt)
	return &v, nil
}

// Set creates the value of a key or changes its text, and reports whether it
// created it. This is what a release step calls: it does not care whether the
// value existed yet.
//
// updated_at moves only when the text differs. Setting the version a release
// already set is no change, and the public side's caching validator should not
// say there was one.
func (s *Store) Set(ctx context.Context, websiteID int64, key, text string) (*Value, bool, error) {
	if err := CheckKey(key); err != nil {
		return nil, false, err
	}
	text, err := CleanText(text)
	if err != nil {
		return nil, false, err
	}
	// Whether the row existed is asked inside the same transaction as the
	// upsert, on the one write connection, so the answer belongs to this write
	// and not to one that slipped in between.
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("set value: %w", err)
	}
	defer tx.Rollback()
	var existing int64
	err = tx.QueryRowContext(ctx,
		`SELECT count(*) FROM site_values WHERE website_id = $1 AND key = $2`, websiteID, key).Scan(&existing)
	if err != nil {
		return nil, false, fmt.Errorf("set value: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO site_values (website_id, key, value) VALUES ($1, $2, $3)
		 ON CONFLICT (website_id, key) DO UPDATE SET
		   value = excluded.value,
		   updated_at = CASE WHEN site_values.value = excluded.value THEN site_values.updated_at
		                     ELSE strftime('%Y-%m-%dT%H:%M:%SZ', 'now') END`,
		websiteID, key, text); err != nil {
		return nil, false, fmt.Errorf("set value: %w", err)
	}
	v, err := scan(tx.QueryRowContext(ctx,
		`SELECT `+columns+` FROM site_values WHERE website_id = $1 AND key = $2`, websiteID, key))
	if err != nil {
		return nil, false, fmt.Errorf("set value: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("set value: %w", err)
	}
	return v, existing == 0, nil
}

// Texts returns every text of one website by key, and the newest updated_at
// among them — the public side's caching validator for a page that names a
// value. The zero time means the website has none.
func (s *Store) Texts(ctx context.Context, websiteID int64) (map[string]string, time.Time, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT key, value, updated_at FROM site_values WHERE website_id = $1`, websiteID)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("load values: %w", err)
	}
	defer rows.Close()
	texts := map[string]string{}
	var newest time.Time
	for rows.Next() {
		var key, text, updatedAt string
		if err := rows.Scan(&key, &text, &updatedAt); err != nil {
			return nil, time.Time{}, fmt.Errorf("scan value: %w", err)
		}
		texts[key] = text
		if at, _ := time.Parse(timeLayout, updatedAt); at.After(newest) {
			newest = at
		}
	}
	return texts, newest, rows.Err()
}
