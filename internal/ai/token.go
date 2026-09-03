// Package ai lets an operator connect their own AI assistant to the running
// CMS and write content with it.
//
// The protocol is MCP — the one Claude, ChatGPT and the editors speak — over
// plain HTTP: JSON-RPC 2.0 at one address, a bearer token in the header. No
// SDK, no dependency; it is a few hundred lines of request handling, and
// keeping it that way is what keeps it auditable.
//
// The direction matters. This server does not call an AI, and no key to any
// provider is stored here. The assistant runs wherever the operator runs it and
// connects inwards, which is why this does not touch the rule that nothing is
// fetched from a third party at runtime.
//
// Everything an assistant can do, it does through the same stores the admin
// uses. Same validation, same revisions, same slug rules. A page written by an
// assistant is a page like any other, and it can be looked at, edited and
// reverted by a person afterwards — which is the whole point of putting it
// behind the same door instead of a second one.
package ai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

const timeLayout = "2006-01-02T15:04:05Z"

// TokenPrefix marks a Holzcloud key so it is recognisable in a config file, in
// a screenshot, and in a secret scanner.
const TokenPrefix = "hc_"

// ErrNoToken and friends are the reasons a request is refused.
var (
	ErrNoToken   = errors.New("kein Zugangsschlüssel")
	ErrBadToken  = errors.New("der Zugangsschlüssel stimmt nicht")
	ErrExpired   = errors.New("der Zugangsschlüssel ist abgelaufen")
	ErrReadOnly  = errors.New("dieser Zugangsschlüssel darf nur lesen")
	ErrNotForYou = errors.New("dieser Zugangsschlüssel gilt für eine andere Website")
)

// Token is a key as the admin shows it. The secret is not part of it: it exists
// once, in the response to the request that created it, and never again.
type Token struct {
	ID   int64
	Name string
	// WebsiteID is 0 for a key that may reach every website.
	WebsiteID  int64
	CanWrite   bool
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	CreatedAt  time.Time
}

// Scope is what a verified key is allowed to do. It travels with a request
// instead of being looked up again in each tool, so there is one place that
// decides and no tool that can forget to ask.
type Scope struct {
	TokenID   int64
	Name      string
	WebsiteID int64
	CanWrite  bool
}

// MayWrite returns nil when this key may change something.
func (s Scope) MayWrite() error {
	if !s.CanWrite {
		return ErrReadOnly
	}
	return nil
}

// MaySee returns nil when this key may touch a given website.
func (s Scope) MaySee(websiteID int64) error {
	if s.WebsiteID != 0 && s.WebsiteID != websiteID {
		return ErrNotForYou
	}
	return nil
}

// Store keeps the keys.
type Store struct{ DB *db.DB }

// NewStore creates the store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

// Issue creates a key and returns the secret, once.
//
// A lifetime of zero means it does not expire. That is a deliberate option and
// not an oversight: a key that stops working on a Tuesday morning, in a config
// file on somebody else's machine, is a key whose failure nobody can explain.
func (s *Store) Issue(ctx context.Context, name string, websiteID int64, canWrite bool, lifetime time.Duration) (string, *Token, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil, errors.New("der Schlüssel braucht einen Namen")
	}
	if len(name) > 80 {
		name = name[:80]
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("schlüssel erzeugen: %w", err)
	}
	secret := TokenPrefix + base64.RawURLEncoding.EncodeToString(raw)

	var site any
	if websiteID > 0 {
		site = websiteID
	}
	var expires any
	if lifetime > 0 {
		expires = time.Now().UTC().Add(lifetime).Format(timeLayout)
	}

	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO ai_tokens (name, token_hash, website_id, can_write, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		name, hash(secret), site, boolToInt(canWrite), expires)
	if err != nil {
		return "", nil, fmt.Errorf("schlüssel sichern: %w", err)
	}
	id, _ := res.LastInsertId()

	t, err := s.Get(ctx, id)
	if err != nil {
		return "", nil, err
	}
	return secret, t, nil
}

// Verify turns a presented secret into a scope, or says why not.
//
// The lookup is by hash, so a wrong key is a miss in an index rather than a
// comparison this code could get wrong.
func (s *Store) Verify(ctx context.Context, secret string) (Scope, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return Scope{}, ErrNoToken
	}

	var (
		id        int64
		name      string
		websiteID *int64
		canWrite  int
		expires   *string
	)
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, name, website_id, can_write, expires_at FROM ai_tokens WHERE token_hash = $1`,
		hash(secret)).Scan(&id, &name, &websiteID, &canWrite, &expires)
	if err != nil {
		return Scope{}, ErrBadToken
	}
	if expires != nil {
		if t, perr := time.Parse(timeLayout, *expires); perr == nil && time.Now().UTC().After(t) {
			return Scope{}, ErrExpired
		}
	}

	scope := Scope{TokenID: id, Name: name, CanWrite: canWrite == 1}
	if websiteID != nil {
		scope.WebsiteID = *websiteID
	}
	return scope, nil
}

// Touch records that a key was used.
//
// Best effort and deliberately not part of the request's success: a write that
// fails here must not turn a working call into an error. What it buys is the
// one line on the admin screen that says whether a key is still in use.
func (s *Store) Touch(ctx context.Context, id int64) {
	_, _ = s.DB.Write.ExecContext(ctx,
		`UPDATE ai_tokens SET last_used_at = $1 WHERE id = $2`,
		time.Now().UTC().Format(timeLayout), id)
}

// List returns the keys, newest first.
func (s *Store) List(ctx context.Context) ([]Token, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, name, website_id, can_write, last_used_at, expires_at, created_at
		 FROM ai_tokens ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("schlüssel lesen: %w", err)
	}
	defer rows.Close()

	var out []Token
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get returns one key.
func (s *Store) Get(ctx context.Context, id int64) (*Token, error) {
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, name, website_id, can_write, last_used_at, expires_at, created_at
		 FROM ai_tokens WHERE id = $1`, id)
	t, err := scanToken(row)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Revoke deletes a key. It stops working with the next request.
func (s *Store) Revoke(ctx context.Context, id int64) error {
	_, err := s.DB.Write.ExecContext(ctx, `DELETE FROM ai_tokens WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("schlüssel zurückziehen: %w", err)
	}
	return nil
}

func scanToken(row interface{ Scan(...any) error }) (Token, error) {
	var (
		t         Token
		websiteID *int64
		canWrite  int
		lastUsed  *string
		expires   *string
		created   string
	)
	if err := row.Scan(&t.ID, &t.Name, &websiteID, &canWrite, &lastUsed, &expires, &created); err != nil {
		return Token{}, err
	}
	if websiteID != nil {
		t.WebsiteID = *websiteID
	}
	t.CanWrite = canWrite == 1
	t.LastUsedAt = parseTime(lastUsed)
	t.ExpiresAt = parseTime(expires)
	t.CreatedAt, _ = time.Parse(timeLayout, created)
	return t, nil
}

func parseTime(raw *string) *time.Time {
	if raw == nil {
		return nil
	}
	t, err := time.Parse(timeLayout, *raw)
	if err != nil {
		return nil
	}
	return &t
}

// hash is what is stored. SHA-256 and not a password hash on purpose: the
// secret is 32 random bytes, so there is no dictionary to run against it, and
// a slow hash here would only slow down every legitimate request.
func hash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
