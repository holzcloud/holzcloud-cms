package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The menu item handlers take two ids that can disagree: the website in the
// address and the menu in the rest of the path. RequireWebsiteAccess only ever
// reads the first one — it says so itself: "everything under
// /admin/websites/<number> belongs to that website, whatever the rest of the
// path turns out to be". So a person who may enter website A passes the guard
// with A in the address and can name website B's menu behind it.
//
// The menu handlers one level up close that gap themselves by comparing
// m.WebsiteID against the website in the address. These tests hold the item
// handlers to the same promise.

// cheapHashing keeps the test user's password affordable. The real parameters
// are sized to cost 64 MB per hash, which is the point of them and the wrong
// price for a fixture.
var cheapHashing = auth.Argon2Params{Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

// menuScopeFixture is two websites that must not be able to reach each other,
// and one person who may only enter the first.
type menuScopeFixture struct {
	handler *Handler
	sm      *scs.SessionManager
	db      *db.DB
	menus   *menu.Store

	siteA, siteB  *domain.Website
	menuA, menuB  *menu.Menu
	itemA, itemB  *menu.MenuItem
	secondA       *menu.MenuItem
	secondB       *menu.MenuItem
	editorOnlyOnA int64
}

func newMenuScopeFixture(t *testing.T) *menuScopeFixture {
	t.Helper()
	ctx := context.Background()

	h, sm, database, siteA := newTestAdmin(t)

	domains := domain.NewStore(database)
	siteB, err := domains.CreateWebsite(ctx, "Fremde Seite", "")
	if err != nil {
		t.Fatalf("CreateWebsite B: %v", err)
	}

	menus := menu.NewStore(database)
	f := &menuScopeFixture{handler: h, sm: sm, db: database, menus: menus, siteA: siteA, siteB: siteB}
	f.menuA, f.itemA, f.secondA = seedMenu(t, menus, siteA.ID, "Hauptmenü A", "main")
	f.menuB, f.itemB, f.secondB = seedMenu(t, menus, siteB.ID, "Hauptmenü B", "main")

	// The person under test. An assignment must exist, because no assignment at
	// all means every website — see NewWebsiteAccessLookup.
	users := user.NewStore(database, cheapHashing)
	editorID, err := users.Create(ctx, "Editor A", "editor-a@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}
	if err := users.SetRights(ctx, editorID, user.Rights{MayPublish: true, Websites: []int64{siteA.ID}}); err != nil {
		t.Fatalf("SetRights: %v", err)
	}
	f.editorOnlyOnA = editorID

	// The premise of the whole attack: the guard admits this person on A and
	// refuses them on B. If this ever stops holding, the tests below are
	// measuring something other than what they claim.
	allowed := NewWebsiteAccessLookup(database)
	if !allowed(ctx, editorID, siteA.ID) {
		t.Fatal("the editor should be allowed on website A")
	}
	if allowed(ctx, editorID, siteB.ID) {
		t.Fatal("the editor should not be allowed on website B")
	}
	return f
}

// seedMenu creates a menu with two items, because reordering needs a neighbour.
func seedMenu(t *testing.T, menus *menu.Store, websiteID int64, name, locationKey string) (*menu.Menu, *menu.MenuItem, *menu.MenuItem) {
	t.Helper()
	ctx := context.Background()
	m, err := menus.CreateMenu(ctx, websiteID, name, locationKey, "")
	if err != nil {
		t.Fatalf("CreateMenu %s: %v", name, err)
	}
	first, err := menus.CreateItem(ctx, m.ID, nil, "Erster Eintrag", "url", "https://example.org/1", nil, 0)
	if err != nil {
		t.Fatalf("CreateItem 1: %v", err)
	}
	second, err := menus.CreateItem(ctx, m.ID, nil, "Zweiter Eintrag", "url", "https://example.org/2", nil, 1)
	if err != nil {
		t.Fatalf("CreateItem 2: %v", err)
	}
	return m, first, second
}

// post drives one request through the same chain main.go builds: the session,
// then RequireWebsiteAccess, then the routed mux. Going through the real
// middleware is the point — a handler-only test could not show that the guard
// lets the request past.
func (f *menuScopeFixture) post(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/websites/{id}/menus/{menuID}/items", f.handler.ErrHandler(f.handler.HandleMenuItemCreate))
	mux.HandleFunc("POST /admin/websites/{id}/menus/{menuID}/items/{itemID}/update", f.handler.ErrHandler(f.handler.HandleMenuItemUpdate))
	mux.HandleFunc("POST /admin/websites/{id}/menus/{menuID}/items/{itemID}/delete", f.handler.ErrHandler(f.handler.HandleMenuItemDelete))
	mux.HandleFunc("POST /admin/websites/{id}/menus/{menuID}/items/{itemID}/reorder", f.handler.ErrHandler(f.handler.HandleMenuItemReorder))

	guarded := auth.RequireWebsiteAccess(f.sm, NewWebsiteAccessLookup(f.db))(mux)

	form := url.Values{
		"title":     {"Eingeschleust"},
		"item_type": {"url"},
		"url":       {"https://attacker.example/"},
	}
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	f.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.sm.Put(r.Context(), auth.SessionKeyUserID, f.editorOnlyOnA)
		guarded.ServeHTTP(w, r)
	})).ServeHTTP(rec, req)
	return rec
}

func (f *menuScopeFixture) itemTitles(t *testing.T, menuID int64) []string {
	t.Helper()
	items, err := f.menus.ListItems(context.Background(), menuID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	titles := make([]string, 0, len(items))
	for _, it := range items {
		titles = append(titles, it.Title)
	}
	return titles
}

// refused is what any of these requests must be. A 404 is the shape the menu
// handlers one level up already use for the same disagreement.
func refused(t *testing.T, rec *httptest.ResponseRecorder, what string) {
	t.Helper()
	if rec.Code == http.StatusNotFound || rec.Code == http.StatusForbidden {
		return
	}
	t.Errorf("%s: status %d, want 404 or 403 — the write reached a foreign website", what, rec.Code)
}

// TestMenuItemCreateRefusesAForeignWebsitesMenu proves an editor confined to
// website A cannot add an entry to website B's menu by naming A in the address.
func TestMenuItemCreateRefusesAForeignWebsitesMenu(t *testing.T) {
	f := newMenuScopeFixture(t)
	before := f.itemTitles(t, f.menuB.ID)

	rec := f.post(t, fmt.Sprintf("/admin/websites/%d/menus/%d/items", f.siteA.ID, f.menuB.ID))
	refused(t, rec, "create")

	after := f.itemTitles(t, f.menuB.ID)
	if len(after) != len(before) {
		t.Errorf("website B's menu grew from %v to %v — a foreign entry was created", before, after)
	}
}

// TestMenuItemUpdateRefusesAForeignWebsitesMenu proves the same for editing.
func TestMenuItemUpdateRefusesAForeignWebsitesMenu(t *testing.T) {
	f := newMenuScopeFixture(t)

	target := fmt.Sprintf("/admin/websites/%d/menus/%d/items/%d/update", f.siteA.ID, f.menuB.ID, f.itemB.ID)
	rec := f.post(t, target)
	refused(t, rec, "update")

	after, err := f.menus.GetItem(context.Background(), f.itemB.ID)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if after.Title != f.itemB.Title {
		t.Errorf("website B's entry was retitled from %q to %q", f.itemB.Title, after.Title)
	}
	if after.URL != f.itemB.URL {
		t.Errorf("website B's entry was repointed from %q to %q", f.itemB.URL, after.URL)
	}
}

// TestMenuItemDeleteRefusesAForeignWebsitesMenu proves the same for deletion,
// which is the one that cannot be undone.
func TestMenuItemDeleteRefusesAForeignWebsitesMenu(t *testing.T) {
	f := newMenuScopeFixture(t)

	target := fmt.Sprintf("/admin/websites/%d/menus/%d/items/%d/delete", f.siteA.ID, f.menuB.ID, f.itemB.ID)
	rec := f.post(t, target)
	refused(t, rec, "delete")

	after, err := f.menus.GetItem(context.Background(), f.itemB.ID)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if after == nil {
		t.Error("website B's entry was deleted through website A's address")
	}
}

// TestMenuItemReorderRefusesAForeignWebsitesMenu covers the fourth item
// handler. It writes too — it swaps sort_order — and carries the same defect,
// so leaving it out would fix three quarters of the hole.
func TestMenuItemReorderRefusesAForeignWebsitesMenu(t *testing.T) {
	f := newMenuScopeFixture(t)
	before := f.itemTitles(t, f.menuB.ID)

	target := fmt.Sprintf("/admin/websites/%d/menus/%d/items/%d/reorder?direction=down", f.siteA.ID, f.menuB.ID, f.itemB.ID)
	rec := f.post(t, target)
	refused(t, rec, "reorder")

	after := f.itemTitles(t, f.menuB.ID)
	if strings.Join(before, "|") != strings.Join(after, "|") {
		t.Errorf("website B's menu was reordered from %v to %v", before, after)
	}
}

// TestMenuItemHandlersStillServeTheirOwnWebsite is the other half of the
// guarantee. A fix that refuses everything would pass every test above and
// break the feature, so the same person doing the same four things on their own
// website must still succeed.
func TestMenuItemHandlersStillServeTheirOwnWebsite(t *testing.T) {
	f := newMenuScopeFixture(t)
	ctx := context.Background()
	// Create.
	rec := f.post(t, fmt.Sprintf("/admin/websites/%d/menus/%d/items", f.siteA.ID, f.menuA.ID))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create on the own website: status %d, want 303", rec.Code)
	}
	titles := f.itemTitles(t, f.menuA.ID)
	if len(titles) != 3 {
		t.Fatalf("create on the own website did not add an entry: %v", titles)
	}

	// Update.
	rec = f.post(t, fmt.Sprintf("/admin/websites/%d/menus/%d/items/%d/update", f.siteA.ID, f.menuA.ID, f.itemA.ID))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update on the own website: status %d, want 303", rec.Code)
	}
	updated, err := f.menus.GetItem(ctx, f.itemA.ID)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if updated.Title != "Eingeschleust" {
		t.Errorf("update on the own website did not take: title %q", updated.Title)
	}

	// Reorder.
	rec = f.post(t, fmt.Sprintf("/admin/websites/%d/menus/%d/items/%d/reorder?direction=down", f.siteA.ID, f.menuA.ID, f.itemA.ID))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("reorder on the own website: status %d, want 303", rec.Code)
	}
	if got := f.itemTitles(t, f.menuA.ID); got[0] != f.secondA.Title {
		t.Errorf("reorder on the own website did not take: %v", got)
	}

	// Delete.
	rec = f.post(t, fmt.Sprintf("/admin/websites/%d/menus/%d/items/%d/delete", f.siteA.ID, f.menuA.ID, f.itemA.ID))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete on the own website: status %d, want 303", rec.Code)
	}
	gone, err := f.menus.GetItem(ctx, f.itemA.ID)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if gone != nil {
		t.Error("delete on the own website did not take")
	}
}

// TestMenuItemHandlersRefuseAMenuIdFromAnotherWebsiteEvenForAnAdmin pins the
// second half of the rule. An administrator passes RequireWebsiteAccess on
// every website, so the guard cannot help here at all; the address and the menu
// must still agree, or the admin UI would silently edit the wrong site's menu
// after a mistyped link.
func TestMenuItemHandlersRefuseAMenuIdFromAnotherWebsiteEvenForAnAdmin(t *testing.T) {
	f := newMenuScopeFixture(t)
	ctx := context.Background()

	users := user.NewStore(f.db, cheapHashing)
	adminID, err := users.Create(ctx, "Chefin", "admin@example.com", "passwort123", user.RoleAdmin)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	f.editorOnlyOnA = adminID

	target := fmt.Sprintf("/admin/websites/%d/menus/%d/items/%d/delete", f.siteA.ID, f.menuB.ID, f.itemB.ID)
	rec := f.post(t, target)
	if rec.Code != http.StatusNotFound {
		t.Errorf("delete with a mismatched menu id: status %d, want 404", rec.Code)
	}

	still, err := f.menus.GetItem(ctx, f.itemB.ID)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if still == nil {
		t.Error("website B's entry was deleted through website A's address")
	}
}
