package admin

import (
	"context"
	"database/sql"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// An identity is who the identity provider signed in, and an address is only
// something that identity claims.
//
// ForwardAuthSignIn found the account with GetByEmail, and nothing tied the
// identity to the row it reached: no stored username, no marker that the
// account was ever linked to single sign-on. The doc comment on web.Identity
// says "the identity is pinned to the username", and DEPLOY.md warns that
// renaming a user at the identity provider creates a new account here. Both
// were false. Whoever can make the identity provider emit a chosen
// X-authentik-email became the account that carries it — an administrator
// included, and through via_sso without the second factor that account had set
// up.
//
// Found by the Phase 10 code review as CR-01 and measured there with exactly
// this request. The roadmap had asked for the opposite ("pin to username"); the
// implementation chose the address because the users schema was on the
// phase's deliberately-unchanged list and the address was its only unique key.
func TestAnotherUsernameWithTheSameAddressIsNotThatAccount(t *testing.T) {
	h, sm, database := newForwardAuthAdmin(t, true)
	boss := seedAccount(t, database, "boss@example.com", user.RoleAdmin)

	_, res := serveForwardAuth(t, h, sm, true, fwdRequest("mallory", "boss@example.com", nil))

	if res.userID != 0 {
		t.Errorf("the identity %q claimed the address of account %d and was signed in as it "+
			"(session user_id = %d, role %q, via_sso %v) — an address the identity provider "+
			"emits is not who the identity provider signed in", "mallory", boss, res.userID, res.role, res.viaSSO)
	}
}

// An account created by hand and never linked to an identity is not reachable
// through single sign-on at all, whatever the headers say.
//
// The alternative — link it to the first identity that arrives with its address
// — decides the link by who is quicker on the day the operator switches single
// sign-on on. An operator links such an account on purpose.
func TestAnAccountNobodyLinkedIsNotReachedBySSO(t *testing.T) {
	h, sm, database := newForwardAuthAdmin(t, true)
	// Written straight into the table, not with seedAccount: this account must
	// stay exactly as the password form would leave it.
	res0, err := database.Write.ExecContext(context.Background(),
		`INSERT INTO users (name, email, password, role) VALUES ('Boss', 'boss@example.com', 'x', 'admin')`)
	if err != nil {
		t.Fatalf("insert the hand-made account: %v", err)
	}
	id, _ := res0.LastInsertId()

	_, res := serveForwardAuth(t, h, sm, true, fwdRequest("boss", "boss@example.com", nil))

	if res.userID != 0 {
		t.Errorf("an account nobody linked to single sign-on (id %d) was signed in by an identity "+
			"that merely arrived with its address (session user_id = %d)", id, res.userID)
	}
}

// Provisioning neither hands out nor links an account that merely carries the
// address.
//
// With provisioning on, an identity nobody linked arrives with the address of
// an account made by hand. Step 5a refuses it before provisioning runs. Behind
// it the duplicate-address branch of provisionSSOUser refuses as well, because
// it returns only an account linked to this identity; before migration 00053
// that branch returned whichever account carried the address.
func TestProvisioningDoesNotHandOutAnAccountThatCarriesTheAddress(t *testing.T) {
	h, sm, database, _, _ := newProvisioningAdmin(t, true)
	ctx := context.Background()
	if _, err := database.Write.ExecContext(ctx,
		`INSERT INTO users (name, email, password, role) VALUES ('Ada', 'ada@example.com', 'x', 'admin')`); err != nil {
		t.Fatalf("insert the hand-made account: %v", err)
	}
	before := countUsers(t, database)

	_, res := serveForwardAuth(t, h, sm, true, fwdRequest("mallory", "ada@example.com", nil))

	if res.userID != 0 {
		t.Errorf("an identity nobody linked reached the account that carries its address "+
			"(session user_id = %d, role %q)", res.userID, res.role)
	}
	if n := countUsers(t, database); n != before {
		t.Errorf("provisioning created %d account(s) for an address that already has one", n-before)
	}
	var linked sql.NullString
	if err := database.Read.QueryRowContext(ctx,
		`SELECT sso_username FROM users WHERE email = 'ada@example.com'`).Scan(&linked); err != nil {
		t.Fatalf("read the link: %v", err)
	}
	if linked.Valid {
		t.Errorf("a sign-in linked the hand-made account to %q; only an operator links an account", linked.String)
	}
}

// A provisioned account is reached again by the identity it was created for,
// and by no other identity arriving with its address.
//
// The first sign-in gets its account straight back from provisionSSOUser and
// never looks it up, so it cannot tell whether provisioning linked the account.
// The second sign-in can: it finds the account only through the link.
func TestAProvisionedAccountIsReachedAgainOnlyByItsIdentity(t *testing.T) {
	h, sm, database, _, _ := newProvisioningAdmin(t, true)

	_, first := serveForwardAuth(t, h, sm, true, fwdRequest("ada", "ada@example.com", nil))
	id, _, _, _ := accountByEmail(t, database, "ada@example.com")
	if first.userID != id {
		t.Fatalf("the first sign-in did not provision and sign in (session user_id = %d, account %d)",
			first.userID, id)
	}

	_, again := serveForwardAuth(t, h, sm, true, fwdRequest("ada", "ada@example.com", nil))
	if again.userID != id {
		t.Errorf("the identity a provisioned account was created for could not sign in to it a second "+
			"time (session user_id = %d, account %d) — provisioning did not link it", again.userID, id)
	}

	_, other := serveForwardAuth(t, h, sm, true, fwdRequest("mallory", "ada@example.com", nil))
	if other.userID != 0 {
		t.Errorf("another identity with the same address reached the provisioned account (session user_id = %d)",
			other.userID)
	}
}
