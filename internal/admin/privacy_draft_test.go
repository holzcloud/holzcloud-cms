package admin

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/privacy"
)

// draftRequest is the POST the settings screen sends.
func draftRequest(websiteID string) *http.Request {
	return postForm("/admin/websites/"+websiteID+"/datenschutz-vorlage", url.Values{}, map[string]string{"id": websiteID})
}

func idOf(id int64) string { return strconv.FormatInt(id, 10) }

// draftSlug and draftTitle are those of a German website's privacy draft.
func draftSlug() string  { return privacy.DraftSlug(privacy.KindPrivacy, "de") }
func draftTitle() string { return privacy.DraftTitle(privacy.KindPrivacy, "de") }

// pagesWithSlugPrefix lists the pages of a website whose slug starts with the
// prefix, in every status.
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

func TestPrivacyDraftCreatesTheCurrentTemplate(t *testing.T) {
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

	rec := serve(t, h, sm, h.HandlePrivacyDraft, draftRequest(idOf(ws.ID)))

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

	stored := readWebsite(t, h, ws.ID)
	req, err := h.legalRequest(ctx, stored, stored.Locale)
	if err != nil {
		t.Fatalf("legalRequest: %v", err)
	}
	want := privacy.ComposePrivacy(req)
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

func TestPrivacyDraftLeavesAnExistingPageAlone(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	before := seedPage(t, database, ws.ID, "Datenschutz", "datenschutz", "Unser **eigener** Text.", "published")

	serve(t, h, sm, h.HandlePrivacyDraft, draftRequest(idOf(ws.ID)))

	after, err := h.pages.GetPage(ctx, before.ID)
	if err != nil || after == nil {
		t.Fatalf("GetPage: %v", err)
	}
	if after.ID != before.ID ||
		after.ContentMarkdown != before.ContentMarkdown ||
		after.ContentHTML != before.ContentHTML ||
		after.Status != before.Status ||
		!after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("the existing privacy page changed:\nbefore %+v\nafter  %+v", before, after)
	}
	if got := pagesWithSlugPrefix(t, h, ws.ID, "datenschutz"); len(got) != 2 {
		t.Errorf("pages with a slug beginning datenschutz: %d, want 2", len(got))
	}
}

func TestPrivacyDraftTwiceMakesTwoDrafts(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)

	serve(t, h, sm, h.HandlePrivacyDraft, draftRequest(idOf(ws.ID)))
	first := pagesWithSlugPrefix(t, h, ws.ID, draftSlug())
	if len(first) != 1 {
		t.Fatalf("after one POST: %d drafts, want 1", len(first))
	}
	serve(t, h, sm, h.HandlePrivacyDraft, draftRequest(idOf(ws.ID)))

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

func TestPrivacyDraftBelongsToTheWebsiteOfThePath(t *testing.T) {
	h, sm, _, a := newTestAdmin(t)
	b, err := h.domains.CreateWebsite(context.Background(), "Zweite", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	serve(t, h, sm, h.HandlePrivacyDraft, draftRequest(idOf(b.ID)))

	if got := pagesWithSlugPrefix(t, h, b.ID, draftSlug()); len(got) != 1 {
		t.Errorf("website B has %d drafts, want 1", len(got))
	}
	if got := pagesWithSlugPrefix(t, h, a.ID, draftSlug()); len(got) != 0 {
		t.Errorf("website A has %d drafts, want 0", len(got))
	}
}

func TestPrivacyDraftUnknownWebsite(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	for _, id := range []string{"9999", "abc"} {
		rec := serve(t, h, sm, h.HandlePrivacyDraft, draftRequest(id))
		if rec.Code != http.StatusNotFound {
			t.Errorf("id %q: status %d, want 404", id, rec.Code)
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

// The handler is allowed to create a page and to look one up, and nothing else.
// A later edit that updates, publishes or deletes through the page store fails
// here, which is the promise of D-06 held at the place it could be broken.
func TestPrivacyDraftOnlyCreatesAPage(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "privacy_draft.go", nil, 0)
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
	want := []string{"CreatePage", "GetPageBySlug"}
	if !slices.Equal(got, want) {
		t.Errorf("privacy_draft.go calls %v on the page store, want exactly %v", got, want)
	}
}

func TestWebsiteSettingsShowThePrivacyCard(t *testing.T) {
	const (
		noPage    = "This website has no privacy page yet."
		published = "Your privacy page is published and is left exactly as it is."
		isDraft   = "Your privacy page is a draft and is left exactly as it is."
		notice    = "That page still carries the notice that it is an unfilled template."
		offer     = "Create it as a new draft to compare it with your page."
	)

	tests := []struct {
		name    string
		slug    string // empty: no privacy page
		body    string
		status  string
		want    []string
		wantNot []string
	}{
		{
			name:    "no privacy page",
			want:    []string{noPage},
			wantNot: []string{published, isDraft, notice, offer},
		},
		{
			name: "published page", slug: "datenschutz", body: "BODY-MARKER", status: "published",
			want:    []string{published, offer},
			wantNot: []string{noPage, isDraft, notice},
		},
		{
			name: "draft page", slug: "datenschutz", body: "BODY-MARKER", status: "draft",
			want:    []string{isDraft, offer},
			wantNot: []string{noPage, published, notice},
		},
		{
			name: "unfilled template", slug: "datenschutz", body: "BODY-MARKER\n\n" + privacy.TemplateNotice("de"), status: "draft",
			want:    []string{isDraft, notice, offer},
			wantNot: []string{noPage, published},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, sm, database, ws := newTestAdmin(t)
			var seeded *page.Page
			if tc.slug != "" {
				seeded = seedPage(t, database, ws.ID, "TITLE-MARKER", tc.slug, tc.body, tc.status)
			}

			rec := websiteGet(t, h, sm, h.HandleWebsiteEdit, ws.ID)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d, want 200", rec.Code)
			}
			body := rec.Body.String()

			action := `action="/admin/websites/` + idOf(ws.ID) + `/datenschutz-vorlage"`
			i := strings.Index(body, action)
			if i < 0 {
				t.Fatalf("no form with %s", action)
			}
			end := strings.Index(body[i:], "</form>")
			if end < 0 || !strings.Contains(body[i:i+end], `name="gorilla.csrf.Token"`) {
				t.Errorf("the draft form has no CSRF field")
			}
			if strings.Contains(body, "hx-post=\"/admin/websites/"+idOf(ws.ID)+"/datenschutz-vorlage\"") {
				t.Errorf("the draft form posts through hx-post; it must work without script")
			}
			for _, s := range tc.want {
				if !strings.Contains(body, s) {
					t.Errorf("missing %q", s)
				}
			}
			for _, s := range tc.wantNot {
				if strings.Contains(body, s) {
					t.Errorf("unexpected %q", s)
				}
			}
			// The stored title and text belong to the editors; the card never
			// prints them.
			for _, marker := range []string{"TITLE-MARKER", "BODY-MARKER"} {
				if strings.Contains(body, marker) {
					t.Errorf("the settings screen prints the stored %s", marker)
				}
			}
			edit := `href="/admin/websites/` + idOf(ws.ID) + `/pages/`
			if seeded != nil {
				if !strings.Contains(body, edit+idOf(seeded.ID)+`/edit"`) {
					t.Errorf("no link to the edit page of the privacy page")
				}
			}
		})
	}

	t.Run("new website screen has no card", func(t *testing.T) {
		h, sm, _, _ := newTestAdmin(t)
		req := httptest.NewRequest(http.MethodGet, "/admin/websites/new", nil)
		rec := serve(t, h, sm, h.HandleWebsiteCreate, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, want 200", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "datenschutz-vorlage") {
			t.Errorf("the new-website screen shows the privacy card")
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
			if _, err := database.Write.Exec(`UPDATE websites SET legal_frameworks = ? WHERE id = ?`, tc.set, ws.ID); err != nil {
				t.Fatal(err)
			}
			serve(t, h, sm, h.HandlePrivacyDraft, draftRequest(idOf(ws.ID)))
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
			serve(t, h, sm, h.HandlePrivacyDraft, draftRequest(idOf(ws.ID)))
			got := pagesWithSlugPrefix(t, h, ws.ID, draftSlug())
			if len(got) != 1 {
				t.Fatalf("%d drafts, want 1", len(got))
			}
			text := got[0].ContentMarkdown
			if hasSharp := strings.Contains(text, "\u00df"); hasSharp == tc.swiss {
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
