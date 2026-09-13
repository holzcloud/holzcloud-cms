// Package wording is the operator's own words for what a theme calls things.
//
// A theme ships its own catalogue (see internal/template/lang.go). That settles
// the language and leaves the choice of word to the theme author, which is one
// person too far away: the author wrote "Warenkorb", the farm shop says "Korb",
// the club says "Merkliste". Until now the only ways to change one word were to
// fork the theme or to edit a file on the server — and both mean the next theme
// update either overwrites the change or is never applied.
//
// The key is the theme author's own sentence, the same string that stands in
// lang/<tag>.json and in the template. Anything else would need a second name
// for every word and a way to keep the two in step.
//
// The wording belongs to the WEBSITE and not to the theme it uses today, so an
// operator who tries another theme and comes back finds their words still
// there. The price is rows for words the current theme does not ask for, and
// the screen says so out loud rather than tidying them away.
package wording

import (
	"context"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// MaxKeys bounds one website's overrides.
//
// Not against abuse — it is the operator's own screen — but against a store
// that is read in full on every render. Two hundred is more words than any of
// the eight shipped themes mints (measured 2026-09-13: 150 across all eight
// together), so reaching it means something other than wording is being kept
// here.
const MaxKeys = 200

// MaxValue bounds one word. A button label that does not fit on a button is
// not the problem this solves.
const MaxValue = 500

// Store reads and writes a website's own wording.
type Store struct {
	DB *db.DB
}

// NewStore creates a wording store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

// Entry is one overridden word.
type Entry struct {
	Locale string
	Key    string
	Value  string
}

// ThemeWords returns one website's overrides for one language.
//
// This is on the render path, so it answers with a map and an error and does
// nothing else. A language with no overrides returns an empty map rather than
// nil-and-an-error: having changed no words is the ordinary case.
//
// It does NOT fall back from "de-CH" to "de". The theme's own catalogue does,
// because a theme author writes one German catalogue and expects Swiss German
// to use it; an operator who overrides a word under "de" and publishes under
// "de-CH" has said which language they meant. Falling back here would apply a
// word to a language they did not choose.
func (s *Store) ThemeWords(ctx context.Context, websiteID int64, locale string) (map[string]string, error) {
	out := map[string]string{}
	if locale == "" {
		return out, nil
	}
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT key, value FROM theme_wording WHERE website_id = ? AND locale = ?`,
		websiteID, locale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, rows.Err()
}

// All returns every override a website has, in every language, ordered so the
// screen can print them without sorting again.
func (s *Store) All(ctx context.Context, websiteID int64) ([]Entry, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT locale, key, value FROM theme_wording
		 WHERE website_id = ? ORDER BY locale, key`, websiteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Locale, &e.Key, &e.Value); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Count is how many words a website has overridden.
func (s *Store) Count(ctx context.Context, websiteID int64) (int, error) {
	var n int
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM theme_wording WHERE website_id = ?`, websiteID).Scan(&n)
	return n, err
}

// Set writes one word, or removes it when the value is empty.
//
// An empty value is a removal and not a stored empty string, because the screen
// offers one text box per word and clearing it is how a person says "use the
// theme's word again". Storing "" instead would put a blank label on the page
// and give them no way back except deleting a row they cannot see.
func (s *Store) Set(ctx context.Context, websiteID int64, locale, key, value string) error {
	locale, key = strings.TrimSpace(locale), strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if locale == "" || key == "" {
		return nil
	}
	if value == "" {
		return s.Delete(ctx, websiteID, locale, key)
	}
	if len([]rune(value)) > MaxValue {
		value = string([]rune(value)[:MaxValue])
	}

	// The bound is checked against what is already there, and only for a key
	// that is not yet stored: rewording an existing entry must keep working
	// even at the limit, or an operator who reaches it can no longer correct
	// their own typo.
	n, err := s.Count(ctx, websiteID)
	if err != nil {
		return err
	}
	if n >= MaxKeys {
		var exists int
		if err := s.DB.Read.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM theme_wording WHERE website_id = ? AND locale = ? AND key = ?`,
			websiteID, locale, key).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrTooMany
		}
	}

	_, err = s.DB.Write.ExecContext(ctx,
		`INSERT INTO theme_wording (website_id, locale, key, value, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(website_id, locale, key) DO UPDATE SET value = excluded.value,
		     updated_at = excluded.updated_at`,
		websiteID, locale, key, value, time.Now().UTC().Format(time.RFC3339))
	return err
}

// Delete removes one word, so the theme's own comes back.
func (s *Store) Delete(ctx context.Context, websiteID int64, locale, key string) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM theme_wording WHERE website_id = ? AND locale = ? AND key = ?`,
		websiteID, locale, key)
	return err
}

// ErrTooMany says the website has as many overridden words as it may have.
var ErrTooMany = errTooMany{}

type errTooMany struct{}

func (errTooMany) Error() string { return "wording: too many overridden words" }
