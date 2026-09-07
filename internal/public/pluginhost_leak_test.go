package public

import (
	"context"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// PagesForPlugin says of itself: "Published pages only, in every case ... the
// rule that an unfinished page is not public has to hold in the host: a module
// is written by someone else, and a check inside it is a promise, not a
// guarantee."
//
// TestPluginsNeverSeeADraft holds it to exactly one of the five states a page
// can be in and still not be public: status='draft'. The other four —  a
// publication date in the future, an unpublication date in the past, and a
// password on either the listing or the body — were never seeded, which is why
// the gap read green.
//
// The distinction the store already draws is the whole fix: PublicPredicate is
// "may be served on its own route", ListablePredicate is that plus "may appear
// in a listing". Every other public listing in the tree uses the second one —
// the sitemap, the feed, the search, both archives. The plugin host used
// neither: it built an admin ListFilter{Status: "published"} instead.

// hide puts a page into one of the states that are not public, straight in the
// column: no handler offers publish_at in the past tense, and the question is
// what the reader does with the row, not how it got there.
func hide(t *testing.T, database *db.DB, slug, set string) {
	t.Helper()
	res, err := database.Write.ExecContext(context.Background(),
		`UPDATE pages SET `+set+` WHERE slug = $1`, slug)
	if err != nil {
		t.Fatalf("hide %s: %v", slug, err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatalf("hide %s changed %d rows", slug, n)
	}
}

func TestPluginsNeverSeeAPageThatIsNotYetOrNoLongerPublic(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Testseite")

	seedPage(t, database, ws.ID, "Fertig", "fertig", "Sichtbar.", "published")
	seedPage(t, database, ws.ID, "Nächste Saison", "kuenftig", "Preis 49.-", "published")
	seedPage(t, database, ws.ID, "Abgelaufen", "abgelaufen", "Alte Aktion.", "published")
	seedPage(t, database, ws.ID, "Händlerpreise", "preise", "Einkaufspreis 12.-", "published")

	// Published, but its window has not opened yet.
	hide(t, database, "kuenftig", `publish_at = '2099-12-01T00:00:00Z'`)
	// Published, and its window has closed.
	hide(t, database, "abgelaufen", `unpublish_at = '2000-01-01T00:00:00Z'`)
	// Published and served — behind a password. The gate is on the page route
	// and in a cookie; the plugin path consults neither.
	hide(t, database, "preise", `access = 'password', access_password = 'geheim'`)

	ctx := context.Background()
	list, _, err := listPages(t, h, ctx, ws.ID)
	if err != nil {
		t.Fatalf("pages.list: %v", err)
	}
	for _, p := range list {
		switch p.Slug {
		case "kuenftig":
			t.Error("pages.list handed out a page whose publication date has not arrived")
		case "abgelaufen":
			t.Error("pages.list handed out a page whose publication window has closed")
		case "preise":
			t.Error("pages.list handed out a password-protected page's title")
		}
	}
	if len(list) != 1 || list[0].Slug != "fertig" {
		t.Errorf("pages.list returned %d pages, want only \"fertig\": %+v", len(list), list)
	}

	// The single-page read. GetPublishedPage applies PublicPredicate, which is
	// right for the page's own route and wrong here: on that route the gate
	// stands in front of the body, and on this one there is no gate at all.
	got, err := h.PagesForPlugin(ctx, ws.ID, plugin.PagesQuery{
		Op: plugin.OpPagesGet, Slug: "preise", Limit: 10,
	})
	if err != nil {
		t.Fatalf("pages.get: %v", err)
	}
	if len(got.Pages) != 0 {
		t.Errorf("pages.get handed out the protected page: %+v", got.Pages)
	}
	for _, p := range got.Pages {
		if strings.Contains(p.HTML, "Einkaufspreis") {
			t.Error("pages.get handed out the protected page's body, password never asked for")
		}
	}
	for _, slug := range []string{"kuenftig", "abgelaufen"} {
		res, err := h.PagesForPlugin(ctx, ws.ID, plugin.PagesQuery{
			Op: plugin.OpPagesGet, Slug: slug, Limit: 10,
		})
		if err != nil {
			t.Fatalf("pages.get %s: %v", slug, err)
		}
		if len(res.Pages) != 0 {
			t.Errorf("pages.get handed out %q, which is outside its publication window", slug)
		}
	}
}
