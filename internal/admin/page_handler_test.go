package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/config"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// newTestAdmin builds a handler over a real migrated database and the real
// on-disk admin templates, so a template that no longer matches its data struct
// fails here rather than in the browser.
func newTestAdmin(t *testing.T) (*Handler, *scs.SessionManager, *db.DB, *domain.Website) {
	t.Helper()

	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	templates, err := web.ParseAdminTemplates(os.DirFS("../../cmd/holzcloud/templates/admin"))
	if err != nil {
		t.Fatalf("ParseAdminTemplates: %v", err)
	}

	sm := scs.New()
	sm.Store = memstore.New()

	domains := domain.NewStore(database)
	ws, err := domains.CreateWebsite(context.Background(), "Testseite", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	cfg := &config.Config{DataDir: dir}
	h := NewHandler(database, sm, templates, auth.Argon2Params{}, domains,
		domain.NewResolver(domains), page.NewStore(database), tmplmgr.NewStore(database, dir),
		menu.NewStore(database), media.NewStore(database), snippet.NewStore(database),
		term.NewStore(database),
		sharelink.New([]byte("test")),
		tmpl.NewLoader(dir, os.DirFS("../../cmd/holzcloud/templates/public/default"), nil, nil),
		cfg, nil, nil)

	return h, sm, database, ws
}

// serve runs one handler with a live session, the way the middleware chain
// would. Handlers read and write the session, so it cannot be skipped.
func serve(t *testing.T, h *Handler, sm *scs.SessionManager, fn func(http.ResponseWriter, *http.Request) error, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	var handlerErr error
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerErr = fn(w, r)
	})).ServeHTTP(rec, req)
	if handlerErr != nil {
		t.Fatalf("handler: %v", handlerErr)
	}
	return rec
}

// postForm builds a POST request with the route values the mux would set.
func postForm(target string, values url.Values, pathValues map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	return req
}

func seedPage(t *testing.T, database *db.DB, websiteID int64, title, slug, body, status string) *page.Page {
	t.Helper()
	html, err := page.RenderMarkdown(body)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	p, err := page.NewStore(database).CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID,
		Title:     title,
		Slug:      slug,
		Markdown:  body,
		HTML:      html,
		Status:    status,
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	return p
}

// The whole point of the 422 rewrite: a rejected save must hand the text back.
// The previous flash-and-redirect discarded it, so a long article vanished
// because the title field was empty.
func TestRejectedEditKeepsTheSubmittedText(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	p := seedPage(t, database, ws.ID, "Titel", "titel", "urspruenglich", "draft")

	const typed = "Ein langer Absatz, den niemand zweimal schreiben moechte."
	req := postForm("/admin/websites/1/pages/1/edit", url.Values{
		"title":            {""}, // the rejection
		"slug":             {"titel"},
		"content_markdown": {typed},
		"status":           {"draft"},
		"version":          {strconv.FormatInt(p.Version, 10)},
	}, map[string]string{"id": strconv.FormatInt(ws.ID, 10), "pageID": strconv.FormatInt(p.ID, 10)})

	rec := serve(t, h, sm, h.HandlePageEdit, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, typed) {
		t.Error("the submitted text is not in the re-rendered form — it was thrown away")
	}
	if !strings.Contains(body, "Titel angeben") {
		t.Error("no field error next to the title")
	}

	stored, err := page.NewStore(database).GetPage(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if stored.ContentMarkdown != "urspruenglich" {
		t.Errorf("a rejected submit changed the stored page to %q", stored.ContentMarkdown)
	}
}

// Two editors, one page. The loser must get their own text back plus an
// explanation, not a 500 and not a silent overwrite.
func TestConflictingSaveShowsTheBannerAndKeepsTheText(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	store := page.NewStore(database)
	p := seedPage(t, database, ws.ID, "Titel", "titel", "start", "draft")

	// Someone else saves first.
	if err := store.UpdatePage(context.Background(), p.ID, page.PageUpdate{
		Title: "Titel", Slug: "titel", Markdown: "fassung von A", HTML: "<p>A</p>",
		Status: "draft", ExpectedVersion: p.Version,
	}); err != nil {
		t.Fatalf("first save: %v", err)
	}

	const typed = "fassung von B, die nicht verloren gehen darf"
	req := postForm("/admin/websites/1/pages/1/edit", url.Values{
		"title":            {"Titel"},
		"slug":             {"titel"},
		"content_markdown": {typed},
		"status":           {"draft"},
		"version":          {strconv.FormatInt(p.Version, 10)}, // stale
	}, map[string]string{"id": strconv.FormatInt(ws.ID, 10), "pageID": strconv.FormatInt(p.ID, 10)})

	rec := serve(t, h, sm, h.HandlePageEdit, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, typed) {
		t.Error("the second editor's text is gone from the response")
	}
	if !strings.Contains(body, "jemand anderem gespeichert") {
		t.Error("no conflict banner")
	}

	stored, _ := store.GetPage(context.Background(), p.ID)
	if stored.ContentMarkdown != "fassung von A" {
		t.Errorf("the first editor's save was overwritten: %q", stored.ContentMarkdown)
	}
}

func TestSuccessfulEditRedirectsAndStores(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	p := seedPage(t, database, ws.ID, "Titel", "titel", "alt", "draft")

	req := postForm("/admin/websites/1/pages/1/edit", url.Values{
		"title":            {"Neuer Titel"},
		"slug":             {"titel"},
		"content_markdown": {"neuer text"},
		"status":           {"published"},
		"version":          {strconv.FormatInt(p.Version, 10)},
	}, map[string]string{"id": strconv.FormatInt(ws.ID, 10), "pageID": strconv.FormatInt(p.ID, 10)})

	rec := serve(t, h, sm, h.HandlePageEdit, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}

	stored, _ := page.NewStore(database).GetPage(context.Background(), p.ID)
	if stored.Title != "Neuer Titel" || stored.ContentMarkdown != "neuer text" {
		t.Errorf("stored page is %+v", stored)
	}
	if stored.PublishedAt == nil {
		t.Error("publishing through the form did not set published_at")
	}
	if stored.Version == p.Version {
		t.Error("version was not bumped, so a concurrent edit could not be detected")
	}
}

// Delete is now a soft delete: the page leaves the list but stays recoverable.
func TestDeleteMovesThePageToTheTrash(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	p := seedPage(t, database, ws.ID, "Weg", "weg", "text", "published")

	req := postForm("/admin/websites/1/pages/1/delete", nil,
		map[string]string{"id": strconv.FormatInt(ws.ID, 10), "pageID": strconv.FormatInt(p.ID, 10)})
	rec := serve(t, h, sm, h.HandlePageDelete, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}

	store := page.NewStore(database)
	trash, err := store.ListTrash(context.Background(), ws.ID)
	if err != nil {
		t.Fatalf("ListTrash: %v", err)
	}
	if len(trash) != 1 || trash[0].ID != p.ID {
		t.Fatalf("trash holds %+v, want the deleted page", trash)
	}

	// And it must be recoverable through the UI.
	restore := postForm("/admin/websites/1/trash/1/restore", nil,
		map[string]string{"id": strconv.FormatInt(ws.ID, 10), "pageID": strconv.FormatInt(p.ID, 10)})
	serve(t, h, sm, h.HandleTrashRestore, restore)

	back, _ := store.GetPage(context.Background(), p.ID)
	if back == nil || back.InTrash() || back.Slug != "weg" {
		t.Errorf("restore did not bring the page back: %+v", back)
	}
}

// The preview pane renders Markdown without storing anything, and it is a place
// where unsanitised HTML would run in the admin's own browser.
func TestPreviewRendersMarkdownAndStripsScripts(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)

	req := postForm("/admin/websites/1/pages/preview", url.Values{
		"content_markdown": {"# Titel\n\n<script>alert(1)</script>\n\nText"},
	}, map[string]string{"id": strconv.FormatInt(ws.ID, 10)})

	rec := serve(t, h, sm, h.HandlePagePreview, req)
	body := rec.Body.String()

	if !strings.Contains(body, "<h1") {
		t.Error("the heading was not rendered")
	}
	if strings.Contains(body, "<script") {
		t.Errorf("script survived into the preview: %s", body)
	}
}

func TestRevisionRestorePutsTheOlderTextBack(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	store := page.NewStore(database)
	p := seedPage(t, database, ws.ID, "Titel", "titel", "die alte fassung", "draft")

	if err := store.UpdatePage(context.Background(), p.ID, page.PageUpdate{
		Title: "Titel", Slug: "titel", Markdown: "die neue fassung", HTML: "<p>neu</p>",
		Status: "draft", ExpectedVersion: p.Version,
	}); err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}

	revs, err := store.ListRevisions(context.Background(), p.ID)
	if err != nil || len(revs) != 1 {
		t.Fatalf("ListRevisions: %v (%d)", err, len(revs))
	}

	req := postForm("/admin/websites/1/pages/1/revisions/1/restore", nil, map[string]string{
		"id":     strconv.FormatInt(ws.ID, 10),
		"pageID": strconv.FormatInt(p.ID, 10),
		"revID":  strconv.FormatInt(revs[0].ID, 10),
	})
	serve(t, h, sm, h.HandlePageRevisionRestore, req)

	after, _ := store.GetPage(context.Background(), p.ID)
	if after.ContentMarkdown != "die alte fassung" {
		t.Errorf("page holds %q after restore", after.ContentMarkdown)
	}

	// The restore is itself an edit, so the state it replaced is recoverable.
	revs, _ = store.ListRevisions(context.Background(), p.ID)
	if len(revs) != 2 {
		t.Errorf("got %d revisions after a restore, want 2", len(revs))
	}
}

// The revisions and trash pages are rendered here so a template that drifts
// away from its data struct fails the build rather than 500ing in production.
func TestHistoryAndTrashPagesRender(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	p := seedPage(t, database, ws.ID, "Titel", "titel", "text", "draft")
	id := strconv.FormatInt(ws.ID, 10)
	pid := strconv.FormatInt(p.ID, 10)

	revReq := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/revisions", nil)
	revReq.SetPathValue("id", id)
	revReq.SetPathValue("pageID", pid)
	if rec := serve(t, h, sm, h.HandlePageRevisions, revReq); rec.Code != http.StatusOK {
		t.Errorf("revisions page: status %d", rec.Code)
	}

	trashReq := httptest.NewRequest(http.MethodGet, "/admin/websites/1/trash", nil)
	trashReq.SetPathValue("id", id)
	rec := serve(t, h, sm, h.HandleTrash, trashReq)
	if rec.Code != http.StatusOK {
		t.Errorf("trash page: status %d", rec.Code)
	}
	if want := strconv.Itoa(int(page.TrashRetention / (24 * time.Hour))); !strings.Contains(rec.Body.String(), want) {
		t.Errorf("trash page does not name the %s-day retention", want)
	}
}

// A new website that answers "Seite nicht gefunden" on its own domain is the
// first thing its owner sees after pointing DNS at it.
func TestNewWebsiteGetsStarterContent(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)

	req := postForm("/admin/websites/new", url.Values{
		"name": {"Schreinerei"}, "description": {"Möbel nach Maß"},
	}, nil)
	rec := serve(t, h, sm, h.HandleWebsiteCreate, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}

	// Website 1 is the fixture's; the new one is 2.
	store := page.NewStore(database)
	home, err := store.GetHomePage(context.Background(), 2)
	if err != nil {
		t.Fatalf("GetHomePage: %v", err)
	}
	if home == nil {
		t.Fatal("the new website has no published start page")
	}

	// The legal pages are scaffolding, so they must stay drafts: a published
	// but empty imprint is worse than none.
	for _, slug := range []string{"impressum", "datenschutz"} {
		p, err := store.GetPageBySlug(context.Background(), 2, slug)
		if err != nil || p == nil {
			t.Fatalf("%s missing: %v", slug, err)
		}
		if p.Status != "draft" {
			t.Errorf("%s was created as %q, want draft", slug, p.Status)
		}
	}

	// § 5 DDG wants the imprint reachable from every page, which is what the
	// footer menu is for. The entry is created up front; its link only becomes
	// live once the operator publishes the page, because GetMenuTree
	// deliberately blanks the slug of an unpublished target.
	menus := menu.NewStore(database)
	list, err := menus.ListMenus(context.Background(), 2)
	if err != nil {
		t.Fatalf("ListMenus: %v", err)
	}
	var footerID int64
	for _, m := range list {
		if m.LocationKey == "footer" {
			footerID = m.ID
		}
	}
	if footerID == 0 {
		t.Fatalf("no footer menu was created: %+v", list)
	}
	items, err := menus.ListItems(context.Background(), footerID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("footer menu has %d entries, want the two legal pages: %+v", len(items), items)
	}

	// While the imprint is a draft the readiness check has to say so, and it
	// has to flip once the page is published.
	ws2, _ := domain.NewStore(database).GetWebsite(context.Background(), 2)
	if imprintCheck(h.siteChecks(context.Background(), ws2, nil)) {
		t.Error("the imprint check passes while the page is still a draft")
	}
	if err := store.SetPageStatus(context.Background(), mustPageID(t, store, 2, "impressum"), "published", nil); err != nil {
		t.Fatalf("publish imprint: %v", err)
	}
	if !imprintCheck(h.siteChecks(context.Background(), ws2, nil)) {
		t.Error("the imprint check still fails after publishing the page")
	}
}

func imprintCheck(checks []SiteCheck) bool {
	for _, c := range checks {
		if strings.Contains(c.Label, "Impressum") {
			return c.OK
		}
	}
	return false
}

func mustPageID(t *testing.T, store *page.Store, websiteID int64, slug string) int64 {
	t.Helper()
	p, err := store.GetPageBySlug(context.Background(), websiteID, slug)
	if err != nil || p == nil {
		t.Fatalf("page %q: %v", slug, err)
	}
	return p.ID
}

// An import or a clone brings its own pages and would collide on "home".
func TestStarterContentCanBeSwitchedOff(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)

	req := postForm("/admin/websites/new", url.Values{
		"name": {"Leer"}, "starter_content": {"off"},
	}, nil)
	serve(t, h, sm, h.HandleWebsiteCreate, req)

	pages, total, err := page.NewStore(database).ListPages(context.Background(), 2, page.ListFilter{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	if total != 0 {
		t.Errorf("got %d pages (%+v), want none", total, pages)
	}
}

// The settings screen is where an operator finds out what is still missing.
func TestSiteChecksReportWhatIsMissing(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	ctx := context.Background()

	byLabel := func(checks []SiteCheck) map[string]bool {
		m := map[string]bool{}
		for _, c := range checks {
			m[c.Label] = c.OK
		}
		return m
	}

	got := byLabel(h.siteChecks(ctx, ws, nil))
	if got["Mindestens eine Domain"] {
		t.Error("a website with no domains passed the domain check")
	}
	if got["Veröffentlichte Startseite"] {
		t.Error("a website with no pages passed the start page check")
	}

	// Now satisfy two of them and check they flip.
	domains := domain.NewStore(database)
	if _, err := domains.AddDomain(ctx, ws.ID, "demo.test", true); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	seedPage(t, database, ws.ID, "Startseite", "home", "text", "published")

	list, _ := domains.ListDomains(ctx, ws.ID)
	got = byLabel(h.siteChecks(ctx, ws, list))
	if !got["Mindestens eine Domain"] || !got["Eine Hauptdomain festgelegt"] {
		t.Error("domain checks did not pass after adding a primary domain")
	}
	if !got["Veröffentlichte Startseite"] {
		t.Error("start page check did not pass after publishing one")
	}
}
