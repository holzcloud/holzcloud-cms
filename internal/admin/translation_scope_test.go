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
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// A translation group is named by a number that arrives in a hidden form field.
// Nothing has ever checked that the number belongs to the website the editor is
// on: SetTranslation writes `WHERE id = $3` and both readers ask
// `WHERE (id = $1 OR translation_of = $1)` with no website predicate.
//
// So the same shape as the menu items, the product save and the outbox retry:
// a second id, taken from the form, that RequireWebsiteAccess never sees —
// except that here the second id is not even a resource the editor names on
// purpose. It is a link, and a link reads both ways.
//
// These tests assert the effect and not the status code. A refusal that still
// filed the page under a foreign group would be no refusal at all, and the
// damage here is a read, not a write: the group is printed back on the editor's
// own screen, title and draft badge included.

// multilingual switches a test website to two languages. setLanguage returns
// without doing anything on a single-language site, so nothing below would run.
func multilingual(t *testing.T, h *Handler, ws *domain.Website, extra string) *domain.Website {
	t.Helper()
	if _, err := h.db.Write.ExecContext(context.Background(),
		`UPDATE websites SET extra_locales = $1 WHERE id = $2`, extra, ws.ID); err != nil {
		t.Fatalf("set extra_locales: %v", err)
	}
	fresh, err := domain.NewStore(h.db).GetWebsite(context.Background(), ws.ID)
	if err != nil || fresh == nil {
		t.Fatalf("reload website: %v", err)
	}
	if !fresh.Multilingual() {
		t.Fatal("the fixture website is not multilingual — the test would prove nothing")
	}
	return fresh
}

// TestPageSaveRefusesAForeignWebsitesTranslationGroup is the proof. An editor
// on website A must not be able to file A's page under a page of website B.
func TestPageSaveRefusesAForeignWebsitesTranslationGroup(t *testing.T) {
	h, sm, database, siteA := newTestAdmin(t)
	siteA = multilingual(t, h, siteA, "fr")

	siteB, err := domain.NewStore(database).CreateWebsite(context.Background(), "Fremde Seite", "")
	if err != nil {
		t.Fatalf("CreateWebsite B: %v", err)
	}
	// A draft, because a draft is the thing the editor has least business
	// knowing about: the panel prints its title and marks it "Entwurf".
	fremde := seedPage(t, database, siteB.ID, "Geheimer Entwurf", "geheim", "noch nicht fertig", "draft")
	eigene := seedPage(t, database, siteA.ID, "Kontakt", "kontakt", "text", "published")

	req := postForm(fmt.Sprintf("/admin/websites/%d/pages/%d/edit", siteA.ID, eigene.ID), url.Values{
		"title":            {"Kontakt"},
		"slug":             {"kontakt"},
		"content_markdown": {"text"},
		"status":           {"published"},
		"version":          {strconv.FormatInt(eigene.Version, 10)},
		"sprache":          {"fr"},
		"uebersetzung_von": {strconv.FormatInt(fremde.ID, 10)},
	}, map[string]string{
		"id":     strconv.FormatInt(siteA.ID, 10),
		"pageID": strconv.FormatInt(eigene.ID, 10),
	})
	serve(t, h, sm, h.HandlePageEdit, req)

	after, err := page.NewStore(database).GetPage(context.Background(), eigene.ID)
	if err != nil || after == nil {
		t.Fatalf("reread page: %v", err)
	}
	if after.TranslationOf == fremde.ID {
		t.Errorf("website A's page was filed under website B's page %d — the group crosses the websites",
			fremde.ID)
	}
}

// TestTheTranslationPanelNeverShowsAForeignPage is the read half. Even with the
// link already in the database — set by an earlier version, by an import, or by
// the save above before it was closed — neither reader may hand the foreign
// page back.
func TestTheTranslationPanelNeverShowsAForeignPage(t *testing.T) {
	h, sm, database, siteA := newTestAdmin(t)
	siteA = multilingual(t, h, siteA, "fr")

	siteB, err := domain.NewStore(database).CreateWebsite(context.Background(), "Fremde Seite", "")
	if err != nil {
		t.Fatalf("CreateWebsite B: %v", err)
	}
	fremde := seedPage(t, database, siteB.ID, "Geheimer Entwurf", "geheim", "noch nicht fertig", "draft")
	eigene := seedPage(t, database, siteA.ID, "Kontakt", "kontakt", "text", "published")

	// Straight into the column, past every handler: the question is what the
	// readers do with a row that is already wrong.
	if _, err := database.Write.ExecContext(context.Background(),
		`UPDATE pages SET locale = 'fr', translation_of = $1 WHERE id = $2`, fremde.ID, eigene.ID); err != nil {
		t.Fatalf("plant the link: %v", err)
	}

	fresh, err := page.NewStore(database).GetPage(context.Background(), eigene.ID)
	if err != nil || fresh == nil {
		t.Fatalf("reread page: %v", err)
	}

	group, err := page.NewStore(database).TranslationsForEditor(context.Background(), siteA.ID, fresh)
	if err != nil {
		t.Fatalf("TranslationsForEditor: %v", err)
	}
	for _, g := range group {
		if g.WebsiteID != siteA.ID {
			t.Errorf("TranslationsForEditor returned page %d of website %d — the editor reads across websites",
				g.ID, g.WebsiteID)
		}
	}

	// The public reader needs its own fixture. PublicPredicate already hides a
	// draft, so the group above would come back short for the honest reason and
	// prove nothing about the website. A published foreign page is the case
	// that reaches the language switcher on both domains.
	sichtbare := seedPage(t, database, siteB.ID, "Fremde Seite B", "fremd", "text", "published")
	zweite := seedPage(t, database, siteA.ID, "Impressum", "impressum", "text", "published")
	if _, err := database.Write.ExecContext(context.Background(),
		`UPDATE pages SET locale = 'fr', translation_of = $1 WHERE id = $2`, sichtbare.ID, zweite.ID); err != nil {
		t.Fatalf("plant the published link: %v", err)
	}
	zweiteFrisch, err := page.NewStore(database).GetPage(context.Background(), zweite.ID)
	if err != nil || zweiteFrisch == nil {
		t.Fatalf("reread page: %v", err)
	}
	public, err := page.NewStore(database).Translations(context.Background(), siteA.ID, zweiteFrisch)
	if err != nil {
		t.Fatalf("Translations: %v", err)
	}
	for _, g := range public {
		if g.WebsiteID != siteA.ID {
			t.Errorf("Translations returned page %d of website %d — the language switcher crosses the websites",
				g.ID, g.WebsiteID)
		}
	}

	// And the screen itself, which is where an operator would actually read the
	// foreign title.
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/admin/websites/%d/pages/%d/edit", siteA.ID, eigene.ID), nil)
	req.SetPathValue("id", strconv.FormatInt(siteA.ID, 10))
	req.SetPathValue("pageID", strconv.FormatInt(eigene.ID, 10))
	rec := serve(t, h, sm, h.HandlePageEdit, req)
	if strings.Contains(rec.Body.String(), "Geheimer Entwurf") {
		t.Error("the edit screen printed website B's draft title")
	}
}
