package page

import (
	"context"
	"fmt"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
)

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
func (s *Store) SetAccess(ctx context.Context, id int64, u AccessUpdate, params auth.Argon2Params) error {
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

	if u.Password == "" {
		// Keep the existing hash and update only the hint.
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

	if len(u.Password) < MinPagePasswordLength {
		return ErrPagePasswordTooShort
	}
	hash, err := auth.HashPassword(u.Password, params)
	if err != nil {
		return fmt.Errorf("hash page password: %w", err)
	}
	if _, err := s.DB.Write.ExecContext(ctx,
		`UPDATE pages SET access = 'password', access_password = $1, access_hint = $2 WHERE id = $3`,
		hash, u.Hint, id); err != nil {
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
func CheckPagePassword(p *Page, submitted string) bool {
	if p == nil || !p.Protected() {
		return false
	}
	match, err := auth.VerifyPassword(submitted, p.AccessPassword)
	return err == nil && match
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
