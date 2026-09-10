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
