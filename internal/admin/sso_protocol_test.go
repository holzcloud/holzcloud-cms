package admin

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// Taking a website away at the identity provider is written to the protocol,
// naming what the assignment was and what it became.
//
// The row is written today. What was missing is anything holding it: the
// criterion 1 re-check made the row conditional on websites being added and
// every single sign-on test stayed green. SSO-06 asks for every change of
// rights in the protocol, and a removal is the change an operator most needs to
// be able to find afterwards.
func TestLosingAWebsiteGroupIsWrittenToTheProtocol(t *testing.T) {
	h, sm, database, siteA, siteB := newRightsSyncAdmin(t)
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
	assign(t, database, id, siteA, siteB)

	serveForwardAuth(t, h, sm, true, fwdGroupRequest("ada", "ada@example.com", fwdGroupA, nil))

	if n := countAction(t, database, activity.ActionUserUpdate, id); n != 1 {
		t.Fatalf("wrote %d %q rows for losing a website; want exactly 1", n, activity.ActionUserUpdate)
	}
	var meta string
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT COALESCE(metadata, '') FROM activity_log WHERE action = $1 AND entity_id = $2`,
		activity.ActionUserUpdate, id).Scan(&meta); err != nil {
		t.Fatalf("read the row: %v", err)
	}
	for _, want := range []string{
		`"field":"websites"`,
		fmt.Sprintf(`"from":[%d,%d]`, siteA, siteB),
		fmt.Sprintf(`"to":[%d]`, siteA),
		`"via":"sso"`,
	} {
		if !strings.Contains(meta, want) {
			t.Errorf("the protocol row for losing a website is %s and does not carry %s", meta, want)
		}
	}
}

// Window 26: a refused identity is refused on every request, and the protocol
// must not grow a row for each one.
//
// Nothing about the request changes — the proxy asserts the same person, the
// same rule turns them away, the next page they click does it again — so a
// proxy stuck on a denied identity could grow activity_log without bound. The
// sign-in brake is deliberately not fed here (T-10-20): a proxy asserting a
// wrong identity would then lock out the person whose address it names. So the
// brake is on the writing, and every request is still refused.
func TestARefusedIdentityWritesOneProtocolRowAndNotOnePerRequest(t *testing.T) {
	h, sm, database := newForwardAuthAdmin(t, true)

	for i := 0; i < 25; i++ {
		_, res := serveForwardAuth(t, h, sm, true, fwdRequest("stranger", "stranger@example.com", nil))
		if res.userID != 0 {
			t.Fatalf("request %d signed the refused identity in; the brake is on the writing only", i)
		}
	}

	rows := activityRows(t, database)
	if len(rows) != 1 {
		t.Fatalf("25 refused requests wrote %d protocol rows; want 1", len(rows))
	}

	// A different reason from the same identity is its own line: an operator
	// following a misconfiguration needs to see it change.
	serveForwardAuth(t, h, sm, true, fwdRequest("stranger", "", nil))
	if rows = activityRows(t, database); len(rows) != 2 {
		t.Errorf("a refusal for another reason was swallowed: %d rows, want 2", len(rows))
	}
}

// Window 28, first half: the row says WHY.
//
// Until now the reason went to the server log alone, so an operator reading
// /admin/protokoll saw that somebody had been turned away and nothing about
// why — and the answer was in a file on a machine they may not have.
func TestARefusalSaysWhyInTheProtocol(t *testing.T) {
	for _, tc := range []struct {
		name, username, email, want string
	}{
		{"no account carries the address", "stranger", "stranger@example.com", ssoRefuseNoAccount},
		{"no address at all", "ada", "", ssoRefuseNoEmail},
		{"an address above 0x7F", "mueller", "Müller@example.com", ssoRefuseNonASCII},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, sm, database := newForwardAuthAdmin(t, true)
			serveForwardAuth(t, h, sm, true, fwdRequest(tc.username, tc.email, nil))

			var meta string
			if err := database.Read.QueryRowContext(context.Background(),
				`SELECT COALESCE(metadata, '') FROM activity_log WHERE action = $1`,
				activity.ActionAuthLoginFail).Scan(&meta); err != nil {
				t.Fatalf("read the row: %v", err)
			}
			for _, want := range []string{`"via":"sso"`, `"reason":"` + tc.want + `"`} {
				if !strings.Contains(meta, want) {
					t.Errorf("metadata %s does not carry %s", meta, want)
				}
			}
		})
	}
}

// Window 22: the row an automatic sign-in writes before its session exists
// still names the account.
//
// LogActivity fills user_id from the session. At provisioning there is none, so
// the row went in NULL while every other row of the same sign-in — the rights
// rows a moment later — carried the account. One kind of row, findable through
// the protocol's user filter sometimes and not others.
func TestTheProvisioningRowIsFindableUnderTheAccountItCreated(t *testing.T) {
	h, sm, database, _, _ := newProvisioningAdmin(t, true)

	serveForwardAuth(t, h, sm, true, fwdRequest("ada", "ada@example.com", nil))

	var userID *int64
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT user_id FROM activity_log WHERE action = $1`,
		activity.ActionUserCreate).Scan(&userID); err != nil {
		t.Fatalf("read the row: %v", err)
	}
	if userID == nil {
		t.Fatal("the provisioning row carries user_id NULL, so the protocol's user filter cannot find it")
	}
	var want int64
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT id FROM users WHERE email = 'ada@example.com'`).Scan(&want); err != nil {
		t.Fatalf("read the account: %v", err)
	}
	if *userID != want {
		t.Errorf("user_id = %d; want the account it created, %d", *userID, want)
	}
}
