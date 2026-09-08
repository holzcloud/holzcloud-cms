package public

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// pages.get hands a plugin `p.ContentHTML` exactly as it stands in the column,
// and the column is not what a visitor is served. Two things are frozen into it
// as markers and expanded per request — a snippet since Phase 8, an album since
// Phase 11 — so a plugin receives the internal syntax where the text belongs.
//
// The album half is the fifth path on which GAL-03 was false, after the
// unconditional GET (never), the conditional request (CR-01), the Atom feed
// (CR-03) and the admin preview (WR-03). It is the same class as CR-03 and it
// reached neither the review nor the fix report.
//
// The snippet half is older and worse in one way: it predates this phase
// entirely, and it is the shape the feed's own comment already argued about —
// "so a subscriber sees the same text as a visitor rather than the raw marker".
// A plugin is a reader too.
func TestPagesGetHandsNoRawMarkerToAPlugin(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Testseite")

	seedPage(t, database, ws.ID, "Galerie", "galerie",
		"Vor dem Baustein.", "published")
	// Straight into the column, because that is where the renderer puts them:
	// a block page is rendered once on save and both markers survive into it.
	if _, err := database.Write.ExecContext(context.Background(),
		`UPDATE pages SET content_html = $1 WHERE slug = 'galerie'`,
		`<p>Vor dem Baustein.</p>`+"\n"+`<p>[[album:sommer:0]]</p>`+"\n"+`<p>[[snippet:footer-kontakt]]</p>`,
	); err != nil {
		t.Fatalf("plant the markers: %v", err)
	}

	r := httptest.NewRequest("GET", "http://example.test/galerie", nil)
	r = r.WithContext(domain.WebsiteToContext(r.Context(), ws))
	ctx := withRequest(r.Context(), r)

	got, err := h.PagesForPlugin(ctx, ws.ID, plugin.PagesQuery{
		Op: plugin.OpPagesGet, Slug: "galerie", Limit: 10,
	})
	if err != nil {
		t.Fatalf("pages.get: %v", err)
	}
	if len(got.Pages) != 1 {
		t.Fatalf("pages.get returned %d pages, want 1", len(got.Pages))
	}

	body := got.Pages[0].HTML
	if strings.Contains(body, "[[album:") {
		t.Errorf("the plugin was handed a raw album marker; a module that prints "+
			"this shows the internal syntax and leaks the album's address:\n  %s", body)
	}
	if strings.Contains(body, "[[snippet:") {
		t.Errorf("the plugin was handed a raw snippet marker:\n  %s", body)
	}
	// And the honest half: the text around them is still there, so a fix that
	// merely dropped the body would not pass.
	if !strings.Contains(body, "Vor dem Baustein") {
		t.Errorf("the page's own text did not survive the expansion:\n  %s", body)
	}
}
