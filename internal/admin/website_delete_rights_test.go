package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// Deleting a website must never widen what somebody else may reach.
//
// user_websites.website_id is ON DELETE CASCADE, and NewWebsiteAccessLookup
// reads an editor with no rows as an editor of every website. An editor
// assigned to exactly the website being deleted therefore loses their one row,
// and with it every limit — although nothing about that editor was touched and
// the administrator only removed a site.
//
// The Phase 10 audit found this three times independently (criterion 3 and the
// threat batches 10-03/04 and 10-05/06) and two counter-checks confirmed it
// through this handler with a live session: 403 on the foreign website before
// the deletion, 200 after it, same cookie. Phase 10 made the road automatic,
// because every provisioned account holds exactly one row, the default website;
// the road itself is older than Phase 10 — migration 00033.
func TestDeletingAWebsiteDoesNotWidenAnEditorsAccess(t *testing.T) {
	h, sm, database, siteA := newTestAdmin(t)
	ctx := context.Background()
	domains := domain.NewStore(database)
	siteB, err := domains.CreateWebsite(ctx, "Seite B", "")
	if err != nil {
		t.Fatalf("create Seite B: %v", err)
	}

	limited := seedAccount(t, database, "begrenzt@example.com", user.RoleEditor)
	if err := h.users.SetRights(ctx, limited,
		user.Rights{MayPublish: true, Websites: []int64{siteA.ID}}); err != nil {
		t.Fatalf("limit the editor to %q: %v", siteA.Name, err)
	}
	// The control: an editor nobody ever limited. A fix that makes the cascade
	// fail closed by reading "no rows" as "nothing" would lock this editor out
	// of every website — the lock-out the roadmap names as the reason the
	// lookup was left alone. Both halves have to hold.
	unlimited := seedAccount(t, database, "offen@example.com", user.RoleEditor)

	lookup := NewWebsiteAccessLookup(database)
	if !lookup(ctx, limited, siteA.ID) || lookup(ctx, limited, siteB.ID) {
		t.Fatalf("before the deletion the limited editor reaches A=%v, B=%v; want true, false — "+
			"the rest of this test could not tell a regression from the starting state",
			lookup(ctx, limited, siteA.ID), lookup(ctx, limited, siteB.ID))
	}
	if !lookup(ctx, unlimited, siteB.ID) {
		t.Fatal("before the deletion an editor with no limits cannot reach Seite B")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/websites/{id}/delete", h.ErrHandler(h.HandleWebsiteDelete))
	req := httptest.NewRequest(http.MethodPost,
		"/admin/websites/"+strconv.FormatInt(siteA.ID, 10)+"/delete", nil)
	rec := httptest.NewRecorder()
	sm.LoadAndSave(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("deleting %q answered %d; want 303", siteA.Name, rec.Code)
	}
	if ws, err := domains.GetWebsite(ctx, siteA.ID); err != nil || ws != nil {
		t.Fatalf("after the delete handler ran, %q is still there (err %v); "+
			"the assertions below would be measuring nothing", siteA.Name, err)
	}

	if lookup(ctx, limited, siteB.ID) {
		t.Errorf("an editor limited to %q reaches %q after an administrator deleted %q — "+
			"the cascade removed their one assignment and \"no rows\" was read as \"every website\"",
			siteA.Name, siteB.Name, siteA.Name)
	}
	if !lookup(ctx, unlimited, siteB.ID) {
		t.Errorf("an editor nobody ever limited lost %q when an unrelated website was deleted — "+
			"that is the lock-out, not the fix", siteB.Name)
	}
}
