package page

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
)

// UnlockCookiePrefix names the cookie that remembers a page was unlocked.
//
// One cookie per page, scoped to that page's path, so unlocking the trade price
// list does not quietly open the other protected page next to it. The privacy
// text lists it under this prefix.
const UnlockCookiePrefix = "hc_frei_"

// UnlockLifetime is how long an unlocked page stays open.
//
// A session cookie would be cleared when the browser closes, which is the right
// default, but a customer comparing two price lists over an afternoon should
// not have to retype. Twelve hours splits the difference.
const UnlockLifetime = 12 * time.Hour

// AccessUpdate is what the editor's protection settings amount to.
type AccessUpdate struct {
	// Protected switches the gate on.
	Protected bool
	// Password is the new password, or empty to keep the existing one. An
	// editor changing a hint must not have to retype the password, and a form
	// that echoed the old one back would put it in the page source.
	Password string
	Hint     string
}

// SetAccess stores a page's protection.
//
// Hashing happens here with the same Argon2id parameters as an account
// password: a page password is typed by a person, so it is guessable, and the
// only thing standing between a copied database file and every protected page
// is the work factor.
//
// It is PrepareAccess followed by ApplyAccess. A caller that saves a page and
// its protection together calls the two halves itself, on either side of
// storing the page.
func (s *Store) SetAccess(ctx context.Context, id int64, u AccessUpdate, params auth.Argon2Params) error {
	prepared, err := PrepareAccess(u, params)
	if err != nil {
		return err
	}
	return s.ApplyAccess(ctx, id, prepared)
}

// PreparedAccess is an AccessUpdate whose new password, if it brings one, is
// already hashed, so storing it can no longer be refused by the Argon2 gate.
// Only PrepareAccess makes one; the zero value switches protection off, as the
// zero AccessUpdate does.
type PreparedAccess struct {
	update AccessUpdate
	hash   string
}

// PrepareAccess is the half of SetAccess that can be refused: it checks the
// length of a new password and hashes it.
//
// It writes nothing. The editor calls it BEFORE the page itself is stored,
// because hashing can find every slot of the Argon2 gate taken (auth.ErrBusy).
// Done after, as it was, a busy gate left a page that was meant to be
// protected stored, and published if that was asked for, without its password
// (code review WR-01). Done before, a busy gate refuses the whole save and
// nothing is stored.
func PrepareAccess(u AccessUpdate, params auth.Argon2Params) (PreparedAccess, error) {
	prepared := PreparedAccess{update: u}
	if !u.Protected || u.Password == "" {
		return prepared, nil
	}
	if len(u.Password) < MinPagePasswordLength {
		return PreparedAccess{}, ErrPagePasswordTooShort
	}
	hash, err := auth.HashPassword(u.Password, params)
	if err != nil {
		return PreparedAccess{}, fmt.Errorf("hash page password: %w", err)
	}
	prepared.hash = hash
	return prepared, nil
}

// ApplyAccess stores a page's protection as PrepareAccess prepared it.
func (s *Store) ApplyAccess(ctx context.Context, id int64, prepared PreparedAccess) error {
	u := prepared.update
	if !u.Protected {
		// Switching it off clears the hash rather than leaving it behind: a
		// password that no longer guards anything is a secret with no purpose
		// and a copy of it in every backup.
		_, err := s.DB.Write.ExecContext(ctx,
			`UPDATE pages SET access = 'public', access_password = '', access_hint = '' WHERE id = $1`, id)
		if err != nil {
			return fmt.Errorf("clear page access: %w", err)
		}
		return nil
	}

	if prepared.hash == "" {
		// No new password: keep the existing hash and update only the hint.
		res, err := s.DB.Write.ExecContext(ctx,
			`UPDATE pages SET access = 'password', access_hint = $1
			 WHERE id = $2 AND access_password <> ''`, u.Hint, id)
		if err != nil {
			return fmt.Errorf("update page access hint: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNoPagePassword
		}
		return nil
	}

	if _, err := s.DB.Write.ExecContext(ctx,
		`UPDATE pages SET access = 'password', access_password = $1, access_hint = $2 WHERE id = $3`,
		prepared.hash, u.Hint, id); err != nil {
		return fmt.Errorf("set page access: %w", err)
	}
	return nil
}

// MinPagePasswordLength is deliberately lower than an account's.
//
// A page password gets read out over the phone and typed by a customer; the
// account password protects the whole installation. Requiring the same length
// here would get "Holz2024" written on a sticky note either way.
const MinPagePasswordLength = 6

// ErrNoPagePassword means protection was asked for without ever setting one.
var ErrNoPagePassword = fmt.Errorf("this page has no password yet")

// ErrPagePasswordTooShort is returned for a password below the minimum.
var ErrPagePasswordTooShort = fmt.Errorf("a page password needs at least %d characters", MinPagePasswordLength)

// CheckPagePassword reports whether a submitted password opens a page.
//
// The error is only ever auth.ErrBusy: every Argon2 derivation waits for a slot
// and a server that has none free says so instead of guessing. A busy check is
// not a wrong password, so it is neither a "false, nil" nor counted against
// anyone; the caller answers "try again". Every other failure to verify (a
// stored hash that cannot be read) stays what it always was: this password
// does not open the page.
func CheckPagePassword(p *Page, submitted string) (bool, error) {
	if p == nil || !p.Protected() {
		return false, nil
	}
	match, err := auth.VerifyPassword(submitted, p.AccessPassword)
	if errors.Is(err, auth.ErrBusy) {
		return false, err
	}
	return err == nil && match, nil
}

// HasProtected reports whether a website has at least one live page behind a
// password.
//
// The status of the page is deliberately not part of the question. A protected
// draft sets the unlock cookie the day it is published, and the privacy text
// that is written before that day has to name the cookie already: over-
// disclosing in a draft is the safe error, a text that is wrong on the day the
// page goes live is not. A page in the wastebasket does not count.
func (s *Store) HasProtected(ctx context.Context, websiteID int64) (bool, error) {
	var found bool
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM pages WHERE website_id = $1 AND access = 'password'`+LivePredicate+`)`,
		websiteID).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("look for protected pages: %w", err)
	}
	return found, nil
}

// GetForPreview returns a page regardless of its status, for a share link.
//
// The public path never calls this: it is reached only after a signed token has
// been verified, and the token names the page id it was minted for.
func (s *Store) GetForPreview(ctx context.Context, websiteID, id int64) (*Page, error) {
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT `+pageColumns+` FROM pages WHERE id = $1 AND website_id = $2`+LivePredicate,
		id, websiteID)
	p, err := scanPage(row)
	if err != nil {
		// A missing page and a deleted one look the same to a share link, which
		// is correct: both mean the link leads nowhere any more.
		return nil, nil
	}
	return p, nil
}
