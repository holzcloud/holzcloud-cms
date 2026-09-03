package user

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
)

func newTestStore(t *testing.T) (*Store, int64) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	// Cheap Argon2 parameters: these tests are about tokens, not hashing costs.
	s := NewStore(database, auth.Argon2Params{
		Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	id, err := s.Create(context.Background(), "Max", "max@example.de", "passwort123", RoleEditor)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return s, id
}

// A link has to work exactly once: an invitation forwarded to a group chat must
// not still work after the intended person used it.
func TestTokenWorksOnlyOnce(t *testing.T) {
	s, id := newTestStore(t)
	ctx := context.Background()

	secret, _, err := s.IssueToken(ctx, id, PurposeInvite)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	u, err := s.RedeemToken(ctx, secret, PurposeInvite)
	if err != nil {
		t.Fatalf("RedeemToken: %v", err)
	}
	if u.ID != id {
		t.Fatalf("redeemed the wrong user: %+v", u)
	}
	if err := s.ConsumeToken(ctx, secret, PurposeInvite); err != nil {
		t.Fatalf("ConsumeToken: %v", err)
	}

	if _, err := s.RedeemToken(ctx, secret, PurposeInvite); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("a used token still works: %v", err)
	}
	if err := s.ConsumeToken(ctx, secret, PurposeInvite); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("a used token could be consumed twice: %v", err)
	}
}

// The raw secret must never be in the database: a stolen copy of the file must
// not yield working links.
func TestOnlyTheHashIsStored(t *testing.T) {
	s, id := newTestStore(t)
	ctx := context.Background()

	secret, _, err := s.IssueToken(ctx, id, PurposeReset)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	var stored string
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT token_hash FROM user_tokens WHERE user_id = $1`, id).Scan(&stored); err != nil {
		t.Fatalf("read token: %v", err)
	}
	if stored == secret {
		t.Fatal("the raw secret is in the database")
	}
	if stored != hashToken(secret) {
		t.Errorf("stored value is neither the secret nor its hash: %q", stored)
	}
}

func TestExpiredTokenIsRefused(t *testing.T) {
	s, id := newTestStore(t)
	ctx := context.Background()

	secret, _, err := s.IssueToken(ctx, id, PurposeReset)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	past := time.Now().UTC().Add(-time.Minute).Format(tokenTimeLayout)
	if _, err := s.DB.Write.ExecContext(ctx,
		`UPDATE user_tokens SET expires_at = $1 WHERE user_id = $2`, past, id); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	if _, err := s.RedeemToken(ctx, secret, PurposeReset); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("an expired token still works: %v", err)
	}
}

// An invitation link must not double as a reset link, and vice versa.
func TestTokenIsBoundToItsPurpose(t *testing.T) {
	s, id := newTestStore(t)
	ctx := context.Background()

	secret, _, err := s.IssueToken(ctx, id, PurposeInvite)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := s.RedeemToken(ctx, secret, PurposeReset); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("an invitation worked as a reset: %v", err)
	}
}

// Issuing a new link has to retire the old one, or a superseded invitation
// stays usable indefinitely.
func TestIssuingRetiresThePreviousLink(t *testing.T) {
	s, id := newTestStore(t)
	ctx := context.Background()

	first, _, _ := s.IssueToken(ctx, id, PurposeReset)
	second, _, err := s.IssueToken(ctx, id, PurposeReset)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	if _, err := s.RedeemToken(ctx, first, PurposeReset); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("the superseded link still works: %v", err)
	}
	if _, err := s.RedeemToken(ctx, second, PurposeReset); err != nil {
		t.Errorf("the new link does not work: %v", err)
	}
}

func TestTokensAreUnpredictable(t *testing.T) {
	s, id := newTestStore(t)
	ctx := context.Background()

	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		secret, _, err := s.IssueToken(ctx, id, PurposeReset)
		if err != nil {
			t.Fatalf("IssueToken: %v", err)
		}
		if len(secret) < 40 {
			t.Fatalf("token %q is only %d characters", secret, len(secret))
		}
		if seen[secret] {
			t.Fatal("the same token was issued twice")
		}
		seen[secret] = true
	}
}

func TestPurgeRemovesUsedAndExpiredTokens(t *testing.T) {
	s, id := newTestStore(t)
	ctx := context.Background()

	used, _, _ := s.IssueToken(ctx, id, PurposeInvite)
	s.ConsumeToken(ctx, used, PurposeInvite)
	live, _, _ := s.IssueToken(ctx, id, PurposeReset)

	if _, err := s.PurgeExpiredTokens(ctx); err != nil {
		t.Fatalf("PurgeExpiredTokens: %v", err)
	}
	if _, err := s.RedeemToken(ctx, live, PurposeReset); err != nil {
		t.Errorf("the purge removed a live token: %v", err)
	}

	var n int
	s.DB.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_tokens`).Scan(&n)
	if n != 1 {
		t.Errorf("%d tokens survived the purge, want 1", n)
	}
}

func TestRecordLoginStoresTheTimestamp(t *testing.T) {
	s, id := newTestStore(t)
	ctx := context.Background()

	if err := s.RecordLogin(ctx, id); err != nil {
		t.Fatalf("RecordLogin: %v", err)
	}
	var last string
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COALESCE(last_login_at, '') FROM users WHERE id = $1`, id).Scan(&last); err != nil {
		t.Fatalf("read last_login_at: %v", err)
	}
	if last == "" {
		t.Error("last_login_at was not recorded")
	}
}
