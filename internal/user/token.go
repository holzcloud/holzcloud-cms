package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Token purposes.
const (
	PurposeInvite = "invite"
	PurposeReset  = "reset"
)

// Lifetimes. An invitation has to survive a weekend; a reset is answered within
// the hour or it was not urgent.
const (
	InviteLifetime = 72 * time.Hour
	ResetLifetime  = time.Hour
)

// ErrTokenInvalid covers every reason a link does not work: unknown, expired or
// already used. They are deliberately not distinguished — telling a stranger
// which of the three applies tells them whether the token existed.
var ErrTokenInvalid = errors.New("this link is no longer valid")

const tokenTimeLayout = "2006-01-02T15:04:05Z"

// IssueToken creates a one-time link for a user and returns the secret.
//
// The secret is returned exactly once and never stored: only its SHA-256 goes
// into the database, so a stolen copy of the file yields no working links. The
// caller shows the full URL once and cannot show it again — the same discipline
// the CSRF key already follows.
func (s *Store) IssueToken(ctx context.Context, userID int64, purpose string) (string, time.Time, error) {
	lifetime := ResetLifetime
	if purpose == PurposeInvite {
		lifetime = InviteLifetime
	} else if purpose != PurposeReset {
		return "", time.Time{}, fmt.Errorf("unknown token purpose %q", purpose)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("generate token: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)
	expires := time.Now().UTC().Add(lifetime)

	// An older link for the same purpose stops working the moment a new one is
	// issued, so a superseded invitation cannot be used later.
	if _, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM user_tokens WHERE user_id = $1 AND purpose = $2`, userID, purpose); err != nil {
		return "", time.Time{}, fmt.Errorf("clear old tokens: %w", err)
	}
	if _, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO user_tokens (user_id, token_hash, purpose, expires_at) VALUES ($1, $2, $3, $4)`,
		userID, hashToken(secret), purpose, expires.Format(tokenTimeLayout)); err != nil {
		return "", time.Time{}, fmt.Errorf("store token: %w", err)
	}
	return secret, expires, nil
}

// RedeemToken checks a link and returns the user it belongs to.
//
// It does not consume the token: the caller redeems it only after the new
// password has actually been stored, so a failed save leaves the link usable.
func (s *Store) RedeemToken(ctx context.Context, secret, purpose string) (*User, error) {
	var id, userID int64
	var expiresAt string
	var usedAt sql.NullString
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, user_id, expires_at, used_at FROM user_tokens
		 WHERE token_hash = $1 AND purpose = $2`, hashToken(secret), purpose).
		Scan(&id, &userID, &expiresAt, &usedAt)
	if err == sql.ErrNoRows {
		return nil, ErrTokenInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("look up token: %w", err)
	}
	if usedAt.Valid {
		return nil, ErrTokenInvalid
	}
	exp, err := time.Parse(tokenTimeLayout, expiresAt)
	if err != nil || time.Now().UTC().After(exp) {
		return nil, ErrTokenInvalid
	}

	return s.GetByID(ctx, userID)
}

// ConsumeToken marks a link as used, so it works exactly once.
func (s *Store) ConsumeToken(ctx context.Context, secret, purpose string) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE user_tokens SET used_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		 WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL`,
		hashToken(secret), purpose)
	if err != nil {
		return fmt.Errorf("consume token: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrTokenInvalid
	}
	return nil
}

// PurgeExpiredTokens removes links that can no longer be used.
func (s *Store) PurgeExpiredTokens(ctx context.Context) (int64, error) {
	res, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM user_tokens
		 WHERE used_at IS NOT NULL OR expires_at < strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`)
	if err != nil {
		return 0, fmt.Errorf("purge expired tokens: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// SetMustChangePassword flags an account so the next login has to set a new one.
func (s *Store) SetMustChangePassword(ctx context.Context, userID int64, must bool) error {
	value := 0
	if must {
		value = 1
	}
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE users SET must_change_password = $1 WHERE id = $2`, value, userID)
	if err != nil {
		return fmt.Errorf("set must change password: %w", err)
	}
	return nil
}

// RecordLogin stores when an account was last used, which is what makes a
// forgotten account visible in the user list.
func (s *Store) RecordLogin(ctx context.Context, userID int64) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE users SET last_login_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = $1`,
		userID)
	if err != nil {
		return fmt.Errorf("record login: %w", err)
	}
	return nil
}

// hashToken is what gets stored. Comparison happens on the hash, so the raw
// secret never sits in the database.
func hashToken(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// SameToken compares two secrets in constant time. Used where a token is
// checked against a value the caller already holds.
func SameToken(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
