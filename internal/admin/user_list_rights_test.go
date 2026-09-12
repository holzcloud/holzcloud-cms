package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The user list must not show an editor limited to no website as an editor of
// every website.
//
// The list printed "every website" whenever the number of assigned websites
// was zero — the same false distinction the form had, one screen earlier. Since
// migration 00052 zero assigned websites means one of two opposite things, and
// the screen an administrator reads to decide who can get in must say which.
func TestTheUserListShowsAnEditorLimitedToNothingAsLimited(t *testing.T) {
	h, sm, database, siteA := newTestAdmin(t)
	ctx := context.Background()
	domains := domain.NewStore(database)
	if _, err := domains.CreateWebsite(ctx, "Seite B", ""); err != nil {
		t.Fatalf("create Seite B: %v", err)
	}
	limited := seedAccount(t, database, "begrenzt@example.com", user.RoleEditor)
	if err := h.users.SetRights(ctx, limited,
		user.Rights{MayPublish: true, Websites: []int64{siteA.ID}}); err != nil {
		t.Fatalf("limit the editor: %v", err)
	}
	if err := domains.DeleteWebsite(ctx, siteA.ID); err != nil {
		t.Fatalf("delete %q: %v", siteA.Name, err)
	}
	seedAccount(t, database, "offen@example.com", user.RoleEditor)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/users", h.ErrHandler(h.HandleUserList))
	rec := httptest.NewRecorder()
	sm.LoadAndSave(mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/users", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/users answered %d", rec.Code)
	}

	row := func(email string) string {
		for _, chunk := range strings.Split(rec.Body.String(), "<tr") {
			if strings.Contains(chunk, email) {
				return chunk
			}
		}
		t.Fatalf("no row for %s in the user list", email)
		return ""
	}
	if strings.Contains(row("begrenzt@example.com"), "every website") {
		t.Error("the user list says \"every website\" for an editor who is limited to no website")
	}
	if !strings.Contains(row("offen@example.com"), "every website") {
		t.Error("the user list no longer says \"every website\" for an editor nobody limited — the control")
	}
}
