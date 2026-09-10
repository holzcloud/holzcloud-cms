package admin

import (
	"context"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// An administrator whose every group was removed at the identity provider must
// reach no website here — and must certainly not become an editor of all of
// them.
//
// The sync writes the role first and the websites second. An administrator has
// no rows in user_websites, correctly, because user.Store.Rights answers
// Everything() for the role before it reads them. Take the administration group
// AND every website group away and the role half demotes the account to editor,
// then the website half refuses for want of a website group and writes nothing:
// an editor with no rows, which NewWebsiteAccessLookup reads as every website.
// The refusal stops this sign-in; the account stays in exactly the state D-01
// exists to prevent.
//
// TestSyncRightsAppliesADemotionAtTheNextSignIn keeps group A and so never
// reaches this road. Criterion 1 asks that a demotion at the identity provider
// takes effect here, so a fix that keeps the account an administrator is not
// one either: whatever the account is afterwards, it reaches neither website.
func TestAnAdministratorWhoLosesEveryGroupReachesNoWebsite(t *testing.T) {
	h, sm, database, siteA, siteB := newRightsSyncAdmin(t)
	ctx := context.Background()
	// A second administrator, so the last-administrator guard is not what
	// decides this test.
	seedAccount(t, database, "root@example.com", user.RoleAdmin)
	id := seedAccount(t, database, "ada@example.com", user.RoleAdmin)

	_, res := serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", "irgendwas-anderes", nil))

	if res.userID != 0 {
		t.Errorf("an account with no group this installation knows was signed in (as %d)", res.userID)
	}
	lookup := NewWebsiteAccessLookup(database)
	for _, site := range []struct {
		name string
		id   int64
	}{{"Seite A", siteA}, {"Seite B", siteB}} {
		if lookup(ctx, id, site.id) {
			t.Errorf("after the identity provider took every group away, the account (users.role = %q, "+
				"user_websites = %v) still reaches %q", storedRole(t, database, id),
				assignedWebsites(t, database, id), site.name)
		}
	}
}

// The last administrator who loses every group is left exactly as they were.
//
// user.Store.Update refuses to demote the last administrator, and a lagging
// role is the lesser harm than an installation nobody can administer. The sync
// recognises that case before it writes anything, because by the time Update
// refuses, the website half would already have limited an account that stays
// an administrator: a limit nobody can see while the role lasts, and a protocol
// row describing a change that never happened. Update's own refusal still holds
// the role without the early check — which is why a mutation removing that
// check left every other test green, and why this one exists.
func TestTheLastAdministratorWhoLosesEveryGroupIsLeftAsTheyWere(t *testing.T) {
	h, sm, database, _, _ := newRightsSyncAdmin(t)
	id := seedAccount(t, database, "ada@example.com", user.RoleAdmin)

	_, res := serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", "irgendwas-anderes", nil))

	if got := storedRole(t, database, id); got != user.RoleAdmin {
		t.Fatalf("users.role = %q; the last administrator must not be demoted by a group change "+
			"on somebody else's server", got)
	}
	if res.userID != id {
		t.Errorf("the last administrator was turned away (session user_id = %d); the role lags one "+
			"sign-in behind and the sign-in continues with it", res.userID)
	}
	var limited int
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT websites_limited FROM users WHERE id = $1`, id).Scan(&limited); err != nil {
		t.Fatalf("read websites_limited: %v", err)
	}
	if limited != 0 {
		t.Errorf("a website limit was written on the last administrator, who stays an administrator — " +
			"invisible while the role lasts, and waiting for whoever demotes them by hand")
	}
	if n := countAction(t, database, activity.ActionUserUpdate, id); n != 0 {
		t.Errorf("wrote %d %q rows for a change that did not happen", n, activity.ActionUserUpdate)
	}
}
