package admin

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/privacy"
)

// draftRequest is the POST the legal card sends: without fields it asks for
// every missing page, with art and sprache for one compare draft.
func draftRequest(websiteID string, values url.Values) *http.Request {
	if values == nil {
		values = url.Values{}
	}
	return postForm("/admin/websites/"+websiteID+"/rechtstexte", values, map[string]string{"id": websiteID})
}

// legalPagesCompare asks for the current template of one kind and language
// next to whatever page exists.
func legalPagesCompare(websiteID string, k privacy.Kind, tag string) *http.Request {
	return draftRequest(websiteID, url.Values{"art": {string(k)}, "sprache": {tag}})
}

// legalPagesSetFrameworks stores the legal frameworks of a website directly.
func legalPagesSetFrameworks(t *testing.T, database *db.DB, websiteID int64, set string) {
	t.Helper()
	if _, err := database.Write.Exec(`UPDATE websites SET legal_frameworks = ? WHERE id = ?`, set, websiteID); err != nil {
		t.Fatalf("set the frameworks: %v", err)
	}
}

func idOf(id int64) string { return strconv.FormatInt(id, 10) }

// draftSlug and draftTitle are those of a German website's privacy draft.
func draftSlug() string  { return privacy.DraftSlug(privacy.KindPrivacy, "de") }
func draftTitle() string { return privacy.DraftTitle(privacy.KindPrivacy, "de") }

// pagesWithSlugPrefix lists the pages of a website whose slug starts with the
// prefix, in every status and every language.
func pagesWithSlugPrefix(t *testing.T, h *Handler, websiteID int64, prefix string) []page.Page {
	t.Helper()
	all, _, err := h.pages.ListPages(context.Background(), websiteID, page.ListFilter{PerPage: 200})
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	var out []page.Page
	for _, p := range all {
		if strings.HasPrefix(p.Slug, prefix) {
			out = append(out, p)
		}
	}
	return out
}

// legalPagesExact lists the pages of a website whose slug is exactly s.
func legalPagesExact(t *testing.T, h *Handler, websiteID int64, s string) []page.Page {
	t.Helper()
	var out []page.Page
	for _, p := range pagesWithSlugPrefix(t, h, websiteID, s) {
		if p.Slug == s {
			out = append(out, p)
		}
	}
	return out
}

// readWebsite reads the website back, as the handler does.
func readWebsite(t *testing.T, h *Handler, id int64) *domain.Website {
	t.Helper()
	ws, err := h.domains.GetWebsite(context.Background(), id)
	if err != nil || ws == nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	return ws
}

// legalPagesComposed is what the composer of kind k makes for the website in
// the language tag.
func legalPagesComposed(t *testing.T, h *Handler, websiteID int64, k privacy.Kind, tag string) string {
	t.Helper()
	ws := readWebsite(t, h, websiteID)
	req, err := h.legalRequest(context.Background(), ws, tag)
	if err != nil {
		t.Fatalf("legalRequest: %v", err)
	}
	switch k {
	case privacy.KindPrivacy:
		return privacy.ComposePrivacy(req)
	case privacy.KindImprint:
		return privacy.ComposeImprint(req)
	case privacy.KindDNS:
		return privacy.ComposeDNS(req)
	}
	t.Fatalf("unknown kind %q", k)
	return ""
}

func TestTheCompareDraftIsTheCurrentTemplate(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	h.SetActivityStore(activity.NewStore(database))

	if _, err := database.Write.Exec(
		`UPDATE websites SET shop_base = 'laden', price_display = 'both' WHERE id = ?`, ws.ID); err != nil {
		t.Fatalf("switch the shop on: %v", err)
	}
	protected := seedPage(t, database, ws.ID, "Preisliste", "preisliste", "x", "published")
	if err := h.pages.SetAccess(ctx, protected.ID, page.AccessUpdate{Protected: true, Password: "holz2026"}, h.argon2Params); err != nil {
		t.Fatalf("SetAccess: %v", err)
	}

	rec := serve(t, h, sm, h.HandleLegalDrafts, legalPagesCompare(idOf(ws.ID), privacy.KindPrivacy, "de"))

	got := pagesWithSlugPrefix(t, h, ws.ID, draftSlug())
	if len(got) != 1 {
		t.Fatalf("pages with the slug %q: %d, want 1", draftSlug(), len(got))
	}
	p := got[0]
	if p.Status != "draft" {
		t.Errorf("status %q, want draft", p.Status)
	}
	if p.Title != draftTitle() {
		t.Errorf("title %q, want %q", p.Title, draftTitle())
	}
	if draftSlug() != "datenschutz-vorlage" || draftTitle() != "Datenschutzerklärung (neue Vorlage)" {
		t.Errorf("the German draft is called %q at %q, want the names of Phase 30", draftTitle(), draftSlug())
	}
	if p.WebsiteID != ws.ID {
		t.Errorf("website %d, want %d", p.WebsiteID, ws.ID)
	}
	if p.Locale != "" {
		t.Errorf("locale %q, want the main language (empty)", p.Locale)
	}

	want := legalPagesComposed(t, h, ws.ID, privacy.KindPrivacy, "de")
	if p.ContentMarkdown != want {
		t.Errorf("the draft is not the composed template of the website's functions and frameworks")
	}
	for _, name := range []string{"hc_warenkorb", "hc_preise", "hc_frei_"} {
		if !strings.Contains(p.ContentMarkdown, name) {
			t.Errorf("the draft does not name the cookie %q", name)
		}
	}
	html, err := page.RenderMarkdown(want)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if p.ContentHTML != html {
		t.Errorf("the stored HTML is not the rendering of the Markdown")
	}

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}
	wantLoc := "/admin/websites/" + idOf(ws.ID) + "/pages/" + idOf(p.ID) + "/edit"
	if loc := rec.Header().Get("Location"); loc != wantLoc {
		t.Errorf("redirected to %q, want %q", loc, wantLoc)
	}

	entries, _, err := activity.NewStore(database).List(ctx, activity.Filter{Action: activity.ActionPageCreate}, 20, 0)
	if err != nil {
		t.Fatalf("activity List: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.EntityID == p.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("no page-create entry in the activity log for the new draft")
	}

	// The imprint has its own compare draft under its own address.
	serve(t, h, sm, h.HandleLegalDrafts, legalPagesCompare(idOf(ws.ID), privacy.KindImprint, "de"))
	imprint := legalPagesExact(t, h, ws.ID, privacy.DraftSlug(privacy.KindImprint, "de"))
	if len(imprint) != 1 {
		t.Fatalf("imprint compare drafts: %d, want 1", len(imprint))
	}
	if imprint[0].ContentMarkdown != legalPagesComposed(t, h, ws.ID, privacy.KindImprint, "de") {
		t.Errorf("the imprint compare draft is not the composed imprint")
	}
}

func TestLegalDraftsLeaveExistingPagesAlone(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	legalPagesSetFrameworks(t, database, ws.ID, "eu,ch")
	privacyPage := seedPage(t, database, ws.ID, "Datenschutz", "datenschutz", "Unser **eigener** Text.", "published")
	imprintPage := seedPage(t, database, ws.ID, "Impressum", "impressum", "Unser eigenes Impressum.", "draft")

	serve(t, h, sm, h.HandleLegalDrafts, draftRequest(idOf(ws.ID), nil))
	serve(t, h, sm, h.HandleLegalDrafts, legalPagesCompare(idOf(ws.ID), privacy.KindPrivacy, "de"))
	serve(t, h, sm, h.HandleLegalDrafts, legalPagesCompare(idOf(ws.ID), privacy.KindImprint, "de"))

	for _, before := range []*page.Page{privacyPage, imprintPage} {
		after, err := h.pages.GetPage(ctx, before.ID)
		if err != nil || after == nil {
			t.Fatalf("GetPage: %v", err)
		}
		if after.ID != before.ID ||
			after.Title != before.Title ||
			after.Slug != before.Slug ||
			after.ContentMarkdown != before.ContentMarkdown ||
			after.ContentHTML != before.ContentHTML ||
			after.Status != before.Status ||
			after.Locale != before.Locale ||
			after.TranslationOf != before.TranslationOf ||
			!after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Errorf("the existing page %q changed:\nbefore %+v\nafter  %+v", before.Slug, before, after)
		}
	}
	if got := pagesWithSlugPrefix(t, h, ws.ID, "datenschutz"); len(got) != 2 {
		t.Errorf("pages with a slug beginning datenschutz: %d, want 2 (the page and one compare draft)", len(got))
	}
	if got := pagesWithSlugPrefix(t, h, ws.ID, "impressum"); len(got) != 2 {
		t.Errorf("pages with a slug beginning impressum: %d, want 2 (the page and one compare draft)", len(got))
	}
}

func TestTheCompareDraftTwiceMakesTwo(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)

	serve(t, h, sm, h.HandleLegalDrafts, legalPagesCompare(idOf(ws.ID), privacy.KindPrivacy, "de"))
	first := pagesWithSlugPrefix(t, h, ws.ID, draftSlug())
	if len(first) != 1 {
		t.Fatalf("after one POST: %d drafts, want 1", len(first))
	}
	serve(t, h, sm, h.HandleLegalDrafts, legalPagesCompare(idOf(ws.ID), privacy.KindPrivacy, "de"))

	slugs := map[string]bool{}
	for _, p := range pagesWithSlugPrefix(t, h, ws.ID, draftSlug()) {
		slugs[p.Slug] = true
		if p.Status != "draft" {
			t.Errorf("%s is %q, want draft", p.Slug, p.Status)
		}
	}
	if len(slugs) != 2 || !slugs[draftSlug()] || !slugs[draftSlug()+"-2"] {
		t.Errorf("slugs after two POSTs: %v, want datenschutz-vorlage and datenschutz-vorlage-2", slugs)
	}
	again, err := h.pages.GetPage(context.Background(), first[0].ID)
	if err != nil || again == nil {
		t.Fatalf("GetPage: %v", err)
	}
	if again.ContentMarkdown != first[0].ContentMarkdown || !again.UpdatedAt.Equal(first[0].UpdatedAt) {
		t.Errorf("the first draft changed on the second POST")
	}
}

func TestLegalDraftsBelongToTheWebsiteOfThePath(t *testing.T) {
	h, sm, _, a := newTestAdmin(t)
	b, err := h.domains.CreateWebsite(context.Background(), "Zweite", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	serve(t, h, sm, h.HandleLegalDrafts, legalPagesCompare(idOf(b.ID), privacy.KindPrivacy, "de"))
	serve(t, h, sm, h.HandleLegalDrafts, draftRequest(idOf(b.ID), nil))

	if got := pagesWithSlugPrefix(t, h, b.ID, draftSlug()); len(got) != 1 {
		t.Errorf("website B has %d compare drafts, want 1", len(got))
	}
	if got := legalPagesExact(t, h, b.ID, privacy.Slug(privacy.KindImprint, "de")); len(got) != 1 {
		t.Errorf("website B has %d imprints, want 1", len(got))
	}
	all, _, err := h.pages.ListPages(context.Background(), a.ID, page.ListFilter{PerPage: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Errorf("website A has %d pages, want 0", len(all))
	}
}

func TestLegalDraftsUnknownWebsite(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	for _, id := range []string{"9999", "abc"} {
		for _, req := range []*http.Request{draftRequest(id, nil), legalPagesCompare(id, privacy.KindPrivacy, "de")} {
			rec := serve(t, h, sm, h.HandleLegalDrafts, req)
			if rec.Code != http.StatusNotFound {
				t.Errorf("id %q: status %d, want 404", id, rec.Code)
			}
		}
	}
	var n int
	if err := database.Read.QueryRow(`SELECT COUNT(*) FROM pages`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d pages exist after refused requests, want 0 (website %d)", n, ws.ID)
	}
}

func TestTheCompareDraftRefusesUnknownKindsAndLanguages(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	legalPagesSetFrameworks(t, database, ws.ID, "eu,ch")

	for name, values := range map[string]url.Values{
		"dns without US":     {"art": {"dns"}, "sprache": {"de"}},
		"unknown kind":       {"art": {"bogus"}, "sprache": {"de"}},
		"foreign language":   {"art": {"privacy"}, "sprache": {"fr"}},
		"no language":        {"art": {"privacy"}},
		"odd language":       {"art": {"privacy"}, "sprache": {"../de"}},
		"kind of other case": {"art": {"PRIVACY"}, "sprache": {"de"}},
	} {
		rec := serve(t, h, sm, h.HandleLegalDrafts, draftRequest(idOf(ws.ID), values))
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 404 or 400", name, rec.Code)
		}
	}
	var n int
	if err := database.Read.QueryRow(`SELECT COUNT(*) FROM pages`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d pages exist after refused requests, want 0", n)
	}
}

func TestMissingLegalPagesAreCreatedAsDrafts(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	legalPagesSetFrameworks(t, database, ws.ID, "eu,ch,us")

	rec, _, good := albumFlash(t, h, sm, h.HandleLegalDrafts, draftRequest(idOf(ws.ID), nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/admin/websites/"+idOf(ws.ID)) || strings.Contains(loc, "/pages/") {
		t.Errorf("redirected to %q, want the settings of the website", loc)
	}
	if good != "3 legal pages created as drafts" {
		t.Errorf("flash %q, want the count of three", good)
	}

	for _, k := range []privacy.Kind{privacy.KindPrivacy, privacy.KindImprint, privacy.KindDNS} {
		slug := privacy.Slug(k, "de")
		got := legalPagesExact(t, h, ws.ID, slug)
		if len(got) != 1 {
			t.Errorf("%s: %d pages at %q, want 1", k, len(got), slug)
			continue
		}
		p := got[0]
		if p.Status != "draft" {
			t.Errorf("%s: status %q, want draft", k, p.Status)
		}
		if p.Title != privacy.Title(k, "de") {
			t.Errorf("%s: title %q, want %q", k, p.Title, privacy.Title(k, "de"))
		}
		if p.Locale != "" {
			t.Errorf("%s: locale %q, want the main language", k, p.Locale)
		}
		if p.ContentMarkdown != legalPagesComposed(t, h, ws.ID, k, "de") {
			t.Errorf("%s: the draft is not the composed template", k)
		}
	}
	if slugs := []string{privacy.Slug(privacy.KindPrivacy, "de"), privacy.Slug(privacy.KindImprint, "de"), privacy.Slug(privacy.KindDNS, "de")}; !slices.Equal(slugs, []string{"datenschutz", "impressum", "do-not-sell-or-share"}) {
		t.Errorf("German slugs %v", slugs)
	}

	rec, _, good = albumFlash(t, h, sm, h.HandleLegalDrafts, draftRequest(idOf(ws.ID), nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("second POST: status %d, want 303", rec.Code)
	}
	if good != "No legal page was missing." {
		t.Errorf("second POST: flash %q, want that nothing was missing", good)
	}
	all, _, err := h.pages.ListPages(context.Background(), ws.ID, page.ListFilter{PerPage: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("after the second POST %d pages, want still 3", len(all))
	}

	// A privacy page under another usual slug is the privacy page.
	h2, sm2, database2, ws2 := newTestAdmin(t)
	legalPagesSetFrameworks(t, database2, ws2.ID, "eu,ch")
	seedPage(t, database2, ws2.ID, "Datenschutzerklärung", "datenschutzerklaerung", "Eigener Text.", "published")
	rec, _, good = albumFlash(t, h2, sm2, h2.HandleLegalDrafts, draftRequest(idOf(ws2.ID), nil))
	if got := legalPagesExact(t, h2, ws2.ID, "datenschutz"); len(got) != 0 {
		t.Errorf("a second privacy page was created beside datenschutzerklaerung")
	}
	if got := legalPagesExact(t, h2, ws2.ID, "impressum"); len(got) != 1 {
		t.Errorf("the missing imprint was not created")
	}
	if rec.Code != http.StatusSeeOther || good != "Legal template created as a draft" {
		t.Errorf("one page created: status %d, flash %q", rec.Code, good)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "/pages/") {
		t.Errorf("one page created: redirected to %q, want its editor", loc)
	}
}

// The handler is allowed to create a page and to look one up, and nothing else.
// A later edit that updates, publishes or deletes through the page store fails
// here, which is the promise of D-01 held at the place it could be broken.
//
// The list was CreatePage and GetPageBySlug while the card knew one page in one
// language. It is GetPageBySlugIn now because an address is unique only within
// a language: the card asks for the imprint of each language separately. The
// promise is the same — nothing that writes to an existing page.
func TestLegalPagesOnlyCreatePages(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "legal_pages.go", nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	called := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		recv, ok := sel.X.(*ast.SelectorExpr)
		if !ok || !strings.HasSuffix(strings.ToLower(recv.Sel.Name), "pages") {
			return true
		}
		called[sel.Sel.Name] = true
		return true
	})

	var got []string
	for name := range called {
		got = append(got, name)
	}
	slices.Sort(got)
	want := []string{"CreatePage", "GetPageBySlugIn"}
	if !slices.Equal(got, want) {
		t.Errorf("legal_pages.go calls %v on the page store, want exactly %v", got, want)
	}
}

// legalPagesCellRE finds one cell of the card: its language, its kind and its
// state are attributes, so the test does not depend on the words.
var legalPagesCellRE = regexp.MustCompile(`(?s)<td[^>]*data-legal-lang="([^"]*)"[^>]*data-legal-kind="([^"]*)"[^>]*data-legal-state="([^"]*)"[^>]*>(.*?)</td>`)

type legalPagesSeenCell struct{ State, Inner string }

// legalPagesCells reads the cells of the card, keyed "tag/kind".
func legalPagesCells(body string) map[string]legalPagesSeenCell {
	out := map[string]legalPagesSeenCell{}
	for _, m := range legalPagesCellRE.FindAllStringSubmatch(body, -1) {
		out[m[1]+"/"+m[2]] = legalPagesSeenCell{State: m[3], Inner: m[4]}
	}
	return out
}

func TestWebsiteSettingsShowTheLegalCard(t *testing.T) {
	words := map[string]string{
		"missing":   ">Missing<",
		"draft":     ">Draft<",
		"published": ">Published<",
		"template":  ">Unfilled template<",
	}

	tests := []struct {
		name    string
		set     string
		slug    string // empty: no page seeded
		body    string
		status  string
		want    map[string]string // "kind" → state
		wantDNS bool
	}{
		{
			name: "no legal page", set: "eu,ch",
			want: map[string]string{"privacy": "missing", "imprint": "missing"},
		},
		{
			name: "published privacy page", set: "eu,ch", slug: "datenschutz", body: "BODY-MARKER", status: "published",
			want: map[string]string{"privacy": "published", "imprint": "missing"},
		},
		{
			name: "draft imprint", set: "eu,ch", slug: "impressum", body: "BODY-MARKER", status: "draft",
			want: map[string]string{"privacy": "missing", "imprint": "draft"},
		},
		{
			name: "unfilled template", set: "eu,ch", slug: "datenschutz", body: "BODY-MARKER\n\n" + privacy.TemplateNotice("de"), status: "published",
			want: map[string]string{"privacy": "template", "imprint": "missing"},
		},
		{
			name: "united states", set: "eu,ch,us",
			want:    map[string]string{"privacy": "missing", "imprint": "missing", "dns": "missing"},
			wantDNS: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, sm, database, ws := newTestAdmin(t)
			legalPagesSetFrameworks(t, database, ws.ID, tc.set)
			var seeded *page.Page
			if tc.slug != "" {
				seeded = seedPage(t, database, ws.ID, "TITLE-MARKER", tc.slug, tc.body, tc.status)
			}

			rec := websiteGet(t, h, sm, h.HandleWebsiteEdit, ws.ID)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d, want 200", rec.Code)
			}
			body := rec.Body.String()

			cells := legalPagesCells(body)
			for kind, state := range tc.want {
				c, ok := cells["de/"+kind]
				if !ok {
					t.Errorf("no cell for %s in the main language", kind)
					continue
				}
				if c.State != state {
					t.Errorf("%s: state %q, want %q", kind, c.State, state)
				}
				if !strings.Contains(c.Inner, words[state]) {
					t.Errorf("%s: the cell does not say %s", kind, words[state])
				}
			}
			if _, has := cells["de/dns"]; has != tc.wantDNS {
				t.Errorf("Do-Not-Sell cell present = %v, want %v", has, tc.wantDNS)
			}
			if has := strings.Contains(body, `data-legal-column="dns"`); has != tc.wantDNS {
				t.Errorf("Do-Not-Sell column present = %v, want %v", has, tc.wantDNS)
			}
			if len(cells) != len(tc.want) {
				t.Errorf("%d cells, want %d", len(cells), len(tc.want))
			}

			action := `action="/admin/websites/` + idOf(ws.ID) + `/rechtstexte"`
			i := strings.Index(body, action)
			if i < 0 {
				t.Fatalf("no form with %s", action)
			}
			end := strings.Index(body[i:], "</form>")
			if end < 0 || !strings.Contains(body[i:i+end], `name="gorilla.csrf.Token"`) {
				t.Errorf("the draft form has no CSRF field")
			}
			if strings.Contains(body, `hx-post="/admin/websites/`+idOf(ws.ID)+`/rechtstexte"`) {
				t.Errorf("a draft form posts through hx-post; it must work without script")
			}
			if !strings.Contains(body, "Create the missing pages as drafts") {
				t.Errorf("a page is missing and the button to create it is not offered")
			}
			// The stored title and text belong to the editors; the card never
			// prints them.
			for _, marker := range []string{"TITLE-MARKER", "BODY-MARKER"} {
				if strings.Contains(body, marker) {
					t.Errorf("the settings screen prints the stored %s", marker)
				}
			}
			if seeded != nil {
				kind := "privacy"
				if tc.slug == "impressum" {
					kind = "imprint"
				}
				c := cells["de/"+kind]
				edit := `href="/admin/websites/` + idOf(ws.ID) + `/pages/` + idOf(seeded.ID) + `/edit"`
				if !strings.Contains(c.Inner, edit) {
					t.Errorf("no link to the editor of the existing page in its cell")
				}
				if !strings.Contains(c.Inner, `name="art" value="`+kind+`"`) || !strings.Contains(c.Inner, `name="sprache" value="de"`) ||
					!strings.Contains(c.Inner, "New draft to compare") {
					t.Errorf("no compare form for the existing page in its cell")
				}
			}
		})
	}

	t.Run("nothing missing offers no button", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		legalPagesSetFrameworks(t, database, ws.ID, "eu")
		seedPage(t, database, ws.ID, "x", "datenschutz", "x", "published")
		seedPage(t, database, ws.ID, "x", "impressum", "x", "published")
		body := websiteGet(t, h, sm, h.HandleWebsiteEdit, ws.ID).Body.String()
		if strings.Contains(body, "Create the missing pages as drafts") {
			t.Errorf("nothing is missing and the button is offered")
		}
	})

	t.Run("no framework chosen", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		legalPagesSetFrameworks(t, database, ws.ID, "")
		body := websiteGet(t, h, sm, h.HandleWebsiteEdit, ws.ID).Body.String()
		if !strings.Contains(body, "No legal framework is chosen for this website.") {
			t.Errorf("the card does not say that no framework is chosen")
		}
		if cells := legalPagesCells(body); len(cells) != 2 {
			t.Errorf("%d cells without a framework, want the privacy page and the imprint", len(cells))
		}
	})

	t.Run("new website screen has no card", func(t *testing.T) {
		h, sm, _, _ := newTestAdmin(t)
		req := httptest.NewRequest(http.MethodGet, "/admin/websites/new", nil)
		rec := serve(t, h, sm, h.HandleWebsiteCreate, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, want 200", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "/rechtstexte") {
			t.Errorf("the new-website screen shows the legal card")
		}
	})
}

// The draft follows the frameworks stored for the website: with EU it carries
// the EU section, with none it carries the core and the closing line and no
// framework section at all.
func TestTheDraftFollowsTheFrameworks(t *testing.T) {
	const euHeading = "\n## Europäische Union: Datenschutz-Grundverordnung (DSGVO)\n"
	frameworkPrefixes := []string{"\n## Europäische Union", "\n## Schweiz", "\n## Vereinigtes Königreich", "\n## USA"}

	for _, tc := range []struct {
		set    string
		wantEU bool
	}{{"eu", true}, {"", false}} {
		t.Run("set="+tc.set, func(t *testing.T) {
			h, sm, database, ws := newTestAdmin(t)
			legalPagesSetFrameworks(t, database, ws.ID, tc.set)
			serve(t, h, sm, h.HandleLegalDrafts, legalPagesCompare(idOf(ws.ID), privacy.KindPrivacy, "de"))
			got := pagesWithSlugPrefix(t, h, ws.ID, draftSlug())
			if len(got) != 1 {
				t.Fatalf("%d drafts, want 1", len(got))
			}
			text := got[0].ContentMarkdown
			if strings.Contains(text, euHeading) != tc.wantEU {
				t.Errorf("set %q: EU section present = %v, want %v", tc.set, !tc.wantEU, tc.wantEU)
			}
			if !tc.wantEU {
				for _, p := range frameworkPrefixes {
					if strings.Contains(text, p) {
						t.Errorf("set %q: the draft has a framework section %q", tc.set, p)
					}
				}
			}
			if !strings.Contains(text, privacy.TemplateNotice("de")) {
				t.Errorf("set %q: the draft lost the template notice", tc.set)
			}
			if !strings.HasSuffix(strings.TrimSpace(text), privacy.NoLegalAdvice("de")) {
				t.Errorf("set %q: the draft does not close with the no-legal-advice line", tc.set)
			}
		})
	}
}

// A website in Switzerland gets its German draft in Swiss spelling (D-15); a
// website in Germany keeps the sharp s.
func TestASwissWebsiteGetsSwissSpelling(t *testing.T) {
	for _, tc := range []struct {
		country string
		swiss   bool
	}{{"CH", true}, {"DE", false}} {
		t.Run(tc.country, func(t *testing.T) {
			h, sm, database, ws := newTestAdmin(t)
			ctx := context.Background()
			if _, err := database.Write.Exec(`UPDATE websites SET country = ?, legal_frameworks = 'eu,ch' WHERE id = ?`, tc.country, ws.ID); err != nil {
				t.Fatal(err)
			}
			serve(t, h, sm, h.HandleLegalDrafts, legalPagesCompare(idOf(ws.ID), privacy.KindPrivacy, "de"))
			got := pagesWithSlugPrefix(t, h, ws.ID, draftSlug())
			if len(got) != 1 {
				t.Fatalf("%d drafts, want 1", len(got))
			}
			text := got[0].ContentMarkdown
			if hasSharp := strings.Contains(text, "ß"); hasSharp == tc.swiss {
				t.Errorf("country %s: sharp s present = %v, want %v", tc.country, hasSharp, !tc.swiss)
			}
			stored := readWebsite(t, h, ws.ID)
			req, err := h.legalRequest(ctx, stored, stored.Locale)
			if err != nil {
				t.Fatal(err)
			}
			if req.Swiss != tc.swiss {
				t.Errorf("country %s: legalRequest.Swiss = %v, want %v", tc.country, req.Swiss, tc.swiss)
			}
			if text != privacy.ComposePrivacy(req) {
				t.Errorf("country %s: the draft is not the composed template of its request", tc.country)
			}
		})
	}

	// A de-CH page of a website elsewhere is Swiss too.
	h, _, _, ws := newTestAdmin(t)
	req, err := h.legalRequest(context.Background(), readWebsite(t, h, ws.ID), "de-CH")
	if err != nil {
		t.Fatal(err)
	}
	if !req.Swiss {
		t.Error("a de-CH page does not get the Swiss spelling")
	}
}
