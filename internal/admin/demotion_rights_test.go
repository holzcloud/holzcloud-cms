package admin

import (
	"context"
	"testing"

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
