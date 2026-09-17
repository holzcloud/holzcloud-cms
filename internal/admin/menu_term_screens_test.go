package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/term"
)

// The menu and label screens. Nine handlers between them and no test on any of
// them, although the menu is the one thing on a website every visitor uses and
// a label is how an editor finds their own work again.
//
// Driven red by thirteen mutations, all thirteen caught. Two of the tests were
// red against the code as it stood, and both were real:
//
//   - The location-key rule (T-04-09) was applied when a menu was created and
//     not when it was renamed, so "haupt" could become "Haupt Menü", be //nolint:german — the example key
//     answered "Menu saved", and take the navigation off the site with nothing
//     anywhere to say why. Fixed in menu.go.
//   - lookupTerm's doc comment claimed it checked that a label belongs to the
//     website. It did not; the check is in the store. The comment is corrected
//     and the behaviour is held below.
//
// One test had to be sharpened afterwards, and the shape is worth knowing
// because it recurs: an empty label name is refused twice, by the screen and by
// the store, so asserting "an error came back" passed with the screen's guard
// removed. Two guards are right; tested only through each other, only one of
// them is really held. The assertion is on the sentence.

func websiteRoute(websiteID int64, extra ...string) map[string]string {
	v := map[string]string{"id": strconv.FormatInt(websiteID, 10)}
	for i := 0; i+1 < len(extra); i += 2 {
		v[extra[i]] = extra[i+1]
	}
	return v
}

func TestAMenuIsCreatedNamedAndDeleted(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	route := websiteRoute(ws.ID)

	rec, bad, _ := albumFlash(t, h, sm, h.HandleMenuCreate, postForm("/admin/websites/1/menus",
		url.Values{"name": {"  Hauptmenü  "}, "location_key": {"haupt"}}, route))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}
	menus, err := menu.NewStore(database).ListMenus(ctx, ws.ID)
	if err != nil || len(menus) != 1 {
		t.Fatalf("ListMenus: %v (%d)", err, len(menus))
	}
	if menus[0].Name != "Hauptmenü" {
		t.Errorf("the name was not trimmed: %q", menus[0].Name)
	}

	// The same key twice in the same language is refused with a sentence, not
	// with a 500 out of the unique index.
	rec, bad, _ = albumFlash(t, h, sm, h.HandleMenuCreate, postForm("/admin/websites/1/menus",
		url.Values{"name": {"Noch eins"}, "location_key": {"haupt"}}, route))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("the duplicate key produced status %d", rec.Code)
	}
	if bad == "" {
		t.Error("the duplicate key was accepted in silence")
	}
	if menus, _ := menu.NewStore(database).ListMenus(ctx, ws.ID); len(menus) != 1 {
		t.Errorf("%d menus after the refused duplicate", len(menus))
	}

	// And it is deleted through the screen.
	id := strconv.FormatInt(menus[0].ID, 10)
	rec = serve(t, h, sm, h.HandleMenuDelete, postForm("/admin/websites/1/menus/1/delete",
		nil, websiteRoute(ws.ID, "menuID", id)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: status %d", rec.Code)
	}
	if menus, _ := menu.NewStore(database).ListMenus(ctx, ws.ID); len(menus) != 0 {
		t.Error("the menu is still there")
	}
}

// A location key is what a theme looks the menu up by. A key outside the
// alphabet is a menu no template can reach — so the create path refuses one,
// with the rule written next to T-04-09.
//
// The update path did not, and that is what this test found: an editor could
// rename "haupt" to "Haupt Menü" in the edit form, be told "Menu saved", and //nolint:german — the example key
// watch the navigation disappear from the site with nothing anywhere to say
// why. Silent loss, which is the one thing this project spends its comments
// avoiding.
func TestTheKeyRuleHoldsOnBothPathsAndNotOnlyOnCreate(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	for _, bad := range []string{"Haupt Menü", "HAUPT", "haupt_menu", "haupt menu", ""} {
		rec, flash, _ := albumFlash(t, h, sm, h.HandleMenuCreate,
			postForm("/admin/websites/1/menus",
				url.Values{"name": {"Menü"}, "location_key": {bad}}, websiteRoute(ws.ID)))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("create %q: status %d", bad, rec.Code)
		}
		if flash == "" {
			t.Errorf("create %q: accepted in silence", bad)
		}
	}
	if menus, _ := menu.NewStore(database).ListMenus(ctx, ws.ID); len(menus) != 0 {
		t.Fatalf("%d menus were created from keys no theme can reach", len(menus))
	}

	good, err := menu.NewStore(database).CreateMenu(ctx, ws.ID, "Hauptmenü", "haupt", "")
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}
	route := websiteRoute(ws.ID, "menuID", strconv.FormatInt(good.ID, 10))

	for _, bad := range []string{"Haupt Menü", "HAUPT", "haupt_menu"} {
		rec, flash, _ := albumFlash(t, h, sm, h.HandleMenuUpdate,
			postForm("/admin/websites/1/menus/1", url.Values{
				"name": {"Hauptmenü"}, "location_key": {bad},
			}, route))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("update %q: status %d", bad, rec.Code)
		}
		if flash == "" {
			t.Errorf("update %q: accepted in silence", bad)
		}
		stored, err := menu.NewStore(database).GetMenu(ctx, good.ID)
		if err != nil || stored == nil {
			t.Fatalf("GetMenu: %v", err)
		}
		if stored.LocationKey != "haupt" {
			t.Errorf("update %q: the key became %q, and the menu is now unreachable",
				bad, stored.LocationKey)
		}
	}

	// A key that keeps the rule still goes through, or the check would be a
	// broken form rather than a check.
	rec := serve(t, h, sm, h.HandleMenuUpdate, postForm("/admin/websites/1/menus/1",
		url.Values{"name": {"Fusszeile"}, "location_key": {"footer-2"}}, route))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("a valid update: status %d", rec.Code)
	}
	stored, _ := menu.NewStore(database).GetMenu(ctx, good.ID)
	if stored == nil || stored.LocationKey != "footer-2" || stored.Name != "Fusszeile" {
		t.Errorf("the valid update did not arrive: %+v", stored)
	}
}

// One website's menu route must not reach another website's menu, on any of the
// three verbs that take a menu id.
func TestOneSitesMenuRouteCannotReachAnothers(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	other, err := domain.NewStore(database).CreateWebsite(ctx, "Die andere", "")
	if err != nil {
		t.Fatal(err)
	}
	fremd, err := menu.NewStore(database).CreateMenu(ctx, other.ID, "Fremd", "haupt", "")
	if err != nil {
		t.Fatal(err)
	}
	route := websiteRoute(ws.ID, "menuID", strconv.FormatInt(fremd.ID, 10))

	for _, c := range []struct {
		what string
		fn   func(http.ResponseWriter, *http.Request) error
	}{
		{"the editor", h.HandleMenuEdit},
		{"the save", h.HandleMenuUpdate},
		{"the delete", h.HandleMenuDelete},
	} {
		rec := serve(t, h, sm, c.fn, postForm("/admin/websites/1/menus/1",
			url.Values{"name": {"Gekapert"}, "location_key": {"gekapert"}}, route))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s reached the other website's menu: status %d", c.what, rec.Code)
		}
	}

	still, err := menu.NewStore(database).GetMenu(ctx, fremd.ID)
	if err != nil || still == nil {
		t.Fatalf("the other website's menu is gone: %v", err)
	}
	if still.Name != "Fremd" || still.LocationKey != "haupt" {
		t.Errorf("the other website's menu was changed: %+v", still)
	}
}

// A menu in a language the website does not serve would be invisible and
// unexplainable, so the language is only taken when the website has it.
func TestAMenuOnlyGetsALanguageTheWebsiteServes(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	if err := domain.NewStore(database).UpdateSettings(ctx, ws.ID, domain.Settings{
		Locale: "de", ExtraLocales: "fr", TimeZone: "Europe/Zurich", OfflineMode: "notfound",
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	for _, c := range []struct{ sent, want string }{
		{"fr", "fr"},
		{"", ""},
		// Not served here: the menu is made in the main language rather than in
		// a language nobody would ever see.
		{"it", ""},
		{"klingon", ""},
	} {
		key := "m" + strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' {
				return r
			}
			return -1
		}, c.sent)
		rec := serve(t, h, sm, h.HandleMenuCreate, postForm("/admin/websites/1/menus",
			url.Values{"name": {"Menü"}, "location_key": {key}, "sprache": {c.sent}},
			websiteRoute(ws.ID)))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("sprache=%q: status %d", c.sent, rec.Code)
		}
		menus, _ := menu.NewStore(database).ListMenus(ctx, ws.ID)
		var made *menu.Menu
		for i := range menus {
			if menus[i].LocationKey == key {
				made = &menus[i]
			}
		}
		if made == nil {
			t.Fatalf("sprache=%q: no menu with key %q", c.sent, key)
		}
		if made.Locale != c.want {
			t.Errorf("sprache=%q produced a menu in %q, want %q", c.sent, made.Locale, c.want)
		}
	}
}

// A label is renamed without moving its address, and that is the whole point of
// the screen: an editor expects the URL to follow and it deliberately does not,
// so every link that already exists keeps working.
func TestRenamingALabelKeepsItsAddress(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	terms := term.NewStore(database)

	p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "x", "published")
	if err := terms.SetForPage(ctx, ws.ID, p.ID, []string{"Möbel"}); err != nil {
		t.Fatalf("SetForPage: %v", err)
	}
	list, err := terms.ListAll(ctx, ws.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListAll: %v (%d)", err, len(list))
	}
	before := list[0]

	route := websiteRoute(ws.ID, "termID", strconv.FormatInt(before.ID, 10))
	rec, bad, good := albumFlash(t, h, sm, h.HandleTermRename,
		postForm("/admin/websites/1/tags/1/rename", url.Values{"name": {"  Möbelbau  "}}, route))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}
	if !strings.Contains(good, "address") && !strings.Contains(good, "Adresse") {
		t.Errorf("the message does not warn that the address stays: %q", good)
	}

	after, err := terms.ListAll(ctx, ws.ID)
	if err != nil || len(after) != 1 {
		t.Fatalf("ListAll: %v", err)
	}
	if after[0].Name != "Möbelbau" {
		t.Errorf("the name is %q", after[0].Name)
	}
	if after[0].Slug != before.Slug {
		t.Errorf("the address moved from %q to %q — every link to it is broken",
			before.Slug, after[0].Slug)
	}

	// An empty name is refused rather than stored — and refused HERE, by the
	// screen, with the screen's own sentence. The store refuses it too, so
	// asserting only "some error came back" would pass with the screen's guard
	// removed and leave the operator reading "Renaming failed: a label needs a
	// name" where they should read a plain instruction. The same two-guard
	// shape as internal/branding's clean, caught the same way.
	rec, bad, _ = albumFlash(t, h, sm, h.HandleTermRename,
		postForm("/admin/websites/1/tags/1/rename", url.Values{"name": {"   "}}, route))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("an empty name: status %d", rec.Code)
	}
	if bad != "A term needs a name" {
		t.Errorf("an empty name was answered with %q", bad)
	}
	after, _ = terms.ListAll(ctx, ws.ID)
	if after[0].Name != "Möbelbau" {
		t.Errorf("the empty name was stored: %q", after[0].Name)
	}
}

// Deleting a label takes the label off the pages and leaves the pages.
func TestDeletingALabelLeavesTheContent(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	terms := term.NewStore(database)

	p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "Der Text bleibt.", "published")
	if err := terms.SetForPage(ctx, ws.ID, p.ID, []string{"Möbel", "Eiche"}); err != nil {
		t.Fatalf("SetForPage: %v", err)
	}
	list, _ := terms.ListAll(ctx, ws.ID)
	if len(list) != 2 {
		t.Fatalf("%d labels", len(list))
	}

	rec := serve(t, h, sm, h.HandleTermDelete, postForm("/admin/websites/1/tags/1/delete",
		nil, websiteRoute(ws.ID, "termID", strconv.FormatInt(list[0].ID, 10))))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}

	after, _ := terms.ListAll(ctx, ws.ID)
	if len(after) != 1 {
		t.Errorf("%d labels left, want 1", len(after))
	}
	still, err := h.pages.GetPage(ctx, p.ID)
	if err != nil || still == nil {
		t.Fatalf("the page went with the label: %v", err)
	}
	if still.ContentMarkdown != "Der Text bleibt." {
		t.Errorf("the page's text changed: %q", still.ContentMarkdown)
	}
}

// The scope of the label screens.
//
// lookupTerm does NOT check that the label belongs to the website — its doc
// comment said it did and that was wrong. The check is real and it is in the
// store: every statement in internal/term carries website_id. This test holds
// the behaviour rather than the mechanism, so moving the check from one to the
// other stays possible and losing it does not.
func TestOneSitesLabelRouteChangesNothingOfAnothers(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	terms := term.NewStore(database)

	other, err := domain.NewStore(database).CreateWebsite(ctx, "Die andere", "")
	if err != nil {
		t.Fatal(err)
	}
	p := seedPage(t, database, other.ID, "Fremd", "fremd", "x", "published")
	if err := terms.SetForPage(ctx, other.ID, p.ID, []string{"Möbel"}); err != nil {
		t.Fatal(err)
	}
	list, _ := terms.ListAll(ctx, other.ID)
	if len(list) != 1 {
		t.Fatalf("%d labels on the other website", len(list))
	}
	fremd := list[0]
	route := websiteRoute(ws.ID, "termID", strconv.FormatInt(fremd.ID, 10))

	serve(t, h, sm, h.HandleTermRename,
		postForm("/admin/websites/1/tags/1/rename", url.Values{"name": {"Gekapert"}}, route))
	serve(t, h, sm, h.HandleTermDelete,
		postForm("/admin/websites/1/tags/1/delete", nil, route))

	after, err := terms.ListAll(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 {
		t.Fatalf("the other website's label was deleted from this website's route")
	}
	if after[0].Name != fremd.Name {
		t.Errorf("the other website's label was renamed to %q", after[0].Name)
	}
}

// The two list screens render, with what is on them.
func TestTheMenuAndLabelListsShowWhatIsThere(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	if _, err := menu.NewStore(database).CreateMenu(ctx, ws.ID, "Hauptmenü", "haupt", ""); err != nil {
		t.Fatal(err)
	}
	p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "x", "published")
	if err := term.NewStore(database).SetForPage(ctx, ws.ID, p.ID, []string{"Möbel"}); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		what string
		fn   func(http.ResponseWriter, *http.Request) error
		want string
	}{
		{"the menu list", h.HandleMenuList, "Hauptmenü"},
		{"the label list", h.HandleTermList, "Möbel"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/x", nil)
		req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
		rec := serve(t, h, sm, c.fn, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", c.what, rec.Code)
			continue
		}
		if !strings.Contains(rec.Body.String(), c.want) {
			t.Errorf("%s does not show %q", c.what, c.want)
		}
	}

	// And an unknown website is a 404 rather than a 500 — the reason
	// lookupWebsite exists.
	for _, fn := range []func(http.ResponseWriter, *http.Request) error{
		h.HandleMenuList, h.HandleTermList,
	} {
		req := httptest.NewRequest(http.MethodGet, "/admin/websites/999/x", nil)
		req.SetPathValue("id", "999")
		if rec := serve(t, h, sm, fn, req); rec.Code != http.StatusNotFound {
			t.Errorf("an unknown website: status %d, want 404", rec.Code)
		}
	}
	_ = fmt.Sprint()
}
