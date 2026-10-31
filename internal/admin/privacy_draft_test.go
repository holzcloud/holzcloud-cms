package admin

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
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

	got := pagesWithSlugPrefix(t, h, ws.ID, privacy.DraftSlug)
	if len(got) != 1 {
		t.Fatalf("pages with the slug %q: %d, want 1", privacy.DraftSlug, len(got))
	}
	p := got[0]
	if p.Status != "draft" {
		t.Errorf("status %q, want draft", p.Status)
	}
	if p.Title != privacy.DraftTitle {
		t.Errorf("title %q, want %q", p.Title, privacy.DraftTitle)
	}
	if p.WebsiteID != ws.ID {
		t.Errorf("website %d, want %d", p.WebsiteID, ws.ID)
	}

	features, err := h.privacyFeatures(ctx, readWebsite(t, h, ws.ID))
	if err != nil {
		t.Fatalf("privacyFeatures: %v", err)
	}
	want := privacy.Compose(features)
	if p.ContentMarkdown != want {
		t.Errorf("the draft is not privacy.Compose of the website's functions")
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
	first := pagesWithSlugPrefix(t, h, ws.ID, privacy.DraftSlug)
	if len(first) != 1 {
		t.Fatalf("after one POST: %d drafts, want 1", len(first))
	}
	serve(t, h, sm, h.HandlePrivacyDraft, draftRequest(idOf(ws.ID)))

	slugs := map[string]bool{}
	for _, p := range pagesWithSlugPrefix(t, h, ws.ID, privacy.DraftSlug) {
		slugs[p.Slug] = true
		if p.Status != "draft" {
			t.Errorf("%s is %q, want draft", p.Slug, p.Status)
		}
	}
	if len(slugs) != 2 || !slugs[privacy.DraftSlug] || !slugs[privacy.DraftSlug+"-2"] {
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

	if got := pagesWithSlugPrefix(t, h, b.ID, privacy.DraftSlug); len(got) != 1 {
		t.Errorf("website B has %d drafts, want 1", len(got))
	}
	if got := pagesWithSlugPrefix(t, h, a.ID, privacy.DraftSlug); len(got) != 0 {
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
