package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The SEO tab of the editor and the report of a website. What is wrong with a
// page is decided in internal/seo and tested there; these tests are about the
// two screens: that they exist, that they say what the analyzer said, that the
// filter and the pager work without a script, and that an editor of another
// website is turned away.

// seoBody is a text long enough, with a link to the website itself.
var seoBody = "## Über uns\n\n" + strings.Repeat("Wir bauen Möbel aus Holz. ", 40) + "\n\n[Kontakt](/kontakt)"

func seedSEOPage(t *testing.T, database *db.DB, websiteID int64, title, slug, description, status string) *page.Page {
	t.Helper()
	html, err := page.RenderMarkdown(seoBody)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	p, err := page.NewStore(database).CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID, Title: title, Slug: slug, Markdown: seoBody, HTML: html, Status: status,
		Meta: page.PageMeta{MetaDescription: description},
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	return p
}

func seoEditRequest(websiteID, pageID int64) *http.Request {
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/admin/websites/%d/pages/%d/edit", websiteID, pageID), nil)
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	req.SetPathValue("pageID", strconv.FormatInt(pageID, 10))
	return req
}

func seoReportRequest(websiteID int64, query string) *http.Request {
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/admin/websites/%d/seo%s", websiteID, query), nil)
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	return req
}

const (
	goodTitle = "Handgemachte Möbel aus Holz"                                                                  // 27 runes
	goodDesc  = "Wir bauen Tische, Stühle und Regale aus heimischem Holz, nach Mass und mit Liebe zum Detail." // 94 runes
)

func TestSEOTabShowsFindingsOfTheSavedPage(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	bad := seedSEOPage(t, database, ws.ID, "Hi", "hi", "", "published")

	body := serve(t, h, sm, h.HandlePageEdit, seoEditRequest(ws.ID, bad.ID)).Body.String()
	for _, want := range []string{
		`id="panel-seo"`, `editor-panel--seo`,
		"seo-status--warn",
		"Title too short", "No description",
		`href="/admin/websites/` + strconv.FormatInt(ws.ID, 10) + `/seo"`,
		"saved version",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the SEO tab of a bad page lacks %q", want)
		}
	}
	// The tab is a radio like the other three: no script, nothing posted.
	if tab := body[strings.Index(body, "editor-panel--seo"):]; strings.Contains(tab[:strings.Index(tab, "</section>")], "hx-") {
		t.Error("the SEO tab carries an htmx attribute")
	}
}

func TestSEOTabIsGreenForAGoodPage(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	good := seedSEOPage(t, database, ws.ID, goodTitle, "moebel", goodDesc, "published")

	body := serve(t, h, sm, h.HandlePageEdit, seoEditRequest(ws.ID, good.ID)).Body.String()
	if !strings.Contains(body, "seo-status--ok") {
		t.Error("a good page is not green")
	}
	for _, bad := range []string{"seo-status--warn", "seo-status--error", "Title too short", "No title", "Description too short"} {
		if strings.Contains(body, bad) {
			t.Errorf("a good page shows %q", bad)
		}
	}
}

func TestSEOTabIsRedWithoutATitleOnAStoredPage(t *testing.T) {
	// A title cannot be saved empty through the form; the store can still hold
	// one (import, a plugin), and the tab has to say so.
	h, sm, database, ws := newTestAdmin(t)
	p := seedSEOPage(t, database, ws.ID, goodTitle, "leer", goodDesc, "published")
	if _, err := database.Write.Exec(`UPDATE pages SET title = '' WHERE id = $1`, p.ID); err != nil {
		t.Fatalf("blank the title: %v", err)
	}
	body := serve(t, h, sm, h.HandlePageEdit, seoEditRequest(ws.ID, p.ID)).Body.String()
	if !strings.Contains(body, "seo-status--error") || !strings.Contains(body, "No title") {
		t.Error("a page without a title is not red")
	}
}

func TestSEOTabIsAbsentOnANewPage(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/websites/%d/pages/new", ws.ID), nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	body := serve(t, h, sm, h.HandlePageCreate, req).Body.String()
	if strings.Contains(body, "panel-seo") {
		t.Error("there is nothing saved to check on a new page")
	}
}

func TestSEOTabDuplicatesIgnoreDraftsAndTrash(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	me := seedSEOPage(t, database, ws.ID, goodTitle, "eins", goodDesc, "published")
	draft := seedSEOPage(t, database, ws.ID, goodTitle, "zwei", goodDesc, "draft")
	trashed := seedSEOPage(t, database, ws.ID, goodTitle, "drei", goodDesc, "published")
	if err := page.NewStore(database).TrashPage(context.Background(), trashed.ID); err != nil {
		t.Fatalf("TrashPage: %v", err)
	}
	_ = draft

	body := serve(t, h, sm, h.HandlePageEdit, seoEditRequest(ws.ID, me.ID)).Body.String()
	if strings.Contains(body, "Title used twice") || strings.Contains(body, "Description used twice") {
		t.Error("a draft and a trashed page count as duplicates")
	}

	seedSEOPage(t, database, ws.ID, strings.ToLower(goodTitle), "vier", goodDesc, "published")
	body = serve(t, h, sm, h.HandlePageEdit, seoEditRequest(ws.ID, me.ID)).Body.String()
	if !strings.Contains(body, "Title used twice") || !strings.Contains(body, "Description used twice") {
		t.Error("a published page with the same title in another case is not found")
	}
}

func TestSEOReportListsPagesAndLinksToTheEditor(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	bad := seedSEOPage(t, database, ws.ID, "Hi", "hi", "", "published")
	seedSEOPage(t, database, ws.ID, goodTitle, "moebel", goodDesc, "published")

	body := serve(t, h, sm, h.HandleSEOReport, seoReportRequest(ws.ID, "")).Body.String()
	for _, want := range []string{
		fmt.Sprintf(`href="/admin/websites/%d/pages/%d/edit"`, ws.ID, bad.ID),
		"Title too short", "Pages checked", "Pages with hints",
		`<form method="get" action="/admin/websites/` + strconv.FormatInt(ws.ID, 10) + `/seo"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the report lacks %q", want)
		}
	}
	if strings.Contains(body, "hx-post") {
		t.Error("the report posts nothing")
	}
	// The good page has only the notes every page gets; its worst is a note, and
	// it is listed — a note is a finding.
	if !strings.Contains(body, ">Hi<") {
		t.Error("the bad page is not listed by its title")
	}
}

func TestSEOReportFilterNarrowsAndIgnoresMadeUpCodes(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	seedSEOPage(t, database, ws.ID, "Hi", "hi", goodDesc, "published")
	seedSEOPage(t, database, ws.ID, goodTitle, "moebel", "", "published")

	rows := func(query string) string {
		return serve(t, h, sm, h.HandleSEOReport, seoReportRequest(ws.ID, query)).Body.String()
	}
	body := rows("?code=title-short")
	if !strings.Contains(body, ">Hi<") || strings.Contains(body, ">"+goodTitle+"<") {
		t.Error("the filter by title-short does not narrow the list to that page")
	}
	if !strings.Contains(body, `value="title-short" selected`) {
		t.Error("the chosen code is not selected in the filter")
	}
	body = rows("?code=description-missing")
	if strings.Contains(body, ">Hi<") || !strings.Contains(body, ">"+goodTitle+"<") {
		t.Error("the filter by description-missing does not narrow the list to that page")
	}
	// A code that does not exist is dropped, not turned into an empty table.
	body = rows("?code=nonsense%22%3E%3Cscript%3E")
	if !strings.Contains(body, ">Hi<") || !strings.Contains(body, ">"+goodTitle+"<") || strings.Contains(body, "<script>") {
		t.Error("an unknown code must show the whole list and echo nothing")
	}
}

func TestSEOReportPaginates(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	for i := 0; i < seoPerPage+5; i++ {
		seedSEOPage(t, database, ws.ID, fmt.Sprintf("S%02d", i), fmt.Sprintf("s%02d", i), "", "published")
	}
	count := func(body string) int { return strings.Count(body, "/pages/") - strings.Count(body, `/pages"`) }
	first := serve(t, h, sm, h.HandleSEOReport, seoReportRequest(ws.ID, "")).Body.String()
	if !strings.Contains(first, "?page=2") {
		t.Error("no link to the second page")
	}
	second := serve(t, h, sm, h.HandleSEOReport, seoReportRequest(ws.ID, "?page=2")).Body.String()
	if !strings.Contains(second, "?page=1") || strings.Contains(second, "?page=3") {
		t.Error("the second page links wrongly")
	}
	if n1, n2 := count(first), count(second); n1 <= n2 || n2 == 0 {
		t.Errorf("page one lists %d edit links, page two %d", n1, n2)
	}
	// Beyond the last page shows the last page, not an empty list.
	far := serve(t, h, sm, h.HandleSEOReport, seoReportRequest(ws.ID, "?page=999")).Body.String()
	if !strings.Contains(far, "/pages/") {
		t.Error("a page number beyond the end shows nothing")
	}
}

func TestSEOReportOfAnUnknownWebsiteIsNotFound(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	if rec := serve(t, h, sm, h.HandleSEOReport, seoReportRequest(999, "")); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// An editor assigned to one website reaches the report of that one and is
// turned away from another's — by the middleware that guards everything under
// /admin/websites/<id>, which is the point: the report did not have to ask.
func TestSEOReportRefusesAnEditorOfAnotherWebsite(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	other, err := h.domains.CreateWebsite(ctx, "Andere", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	seedSEOPage(t, database, other.ID, "Geheim", "geheim", "", "draft")

	users := user.NewStore(database, cheapHashing)
	id, err := users.Create(ctx, "Redakteur", "r@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}
	if err := users.SetRights(ctx, id, user.Rights{MayPublish: true, Websites: []int64{ws.ID}}); err != nil {
		t.Fatalf("SetRights: %v", err)
	}

	guarded := auth.RequireWebsiteAccess(sm, NewWebsiteAccessLookup(database))(
		http.HandlerFunc(h.ErrHandler(h.HandleSEOReport)))
	run := func(websiteID int64) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sm.Put(r.Context(), auth.SessionKeyUserID, id)
			guarded.ServeHTTP(w, r)
		})).ServeHTTP(rec, seoReportRequest(websiteID, ""))
		return rec
	}
	if rec := run(ws.ID); rec.Code != http.StatusOK {
		t.Errorf("own website: status %d, want 200", rec.Code)
	}
	rec := run(other.ID)
	if rec.Code != http.StatusForbidden {
		t.Errorf("another website: status %d, want 403", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Geheim") {
		t.Error("the refusal leaks a title of the other website")
	}
}

// A picture in a block is described by the block or by the library. The tab
// counts the ones that have neither.
func TestSEOTabCountsBlockImagesWithoutDescription(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	store := media.NewStore(database)
	bare, err := store.Create(ctx, ws.ID, "a.jpg", "a.jpg", "image/jpeg", 10, "h1")
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	described, err := store.Create(ctx, ws.ID, "b.jpg", "b.jpg", "image/jpeg", 10, "h2")
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	if err := store.UpdateMeta(ctx, described.ID, "Ein Tisch", ""); err != nil {
		t.Fatalf("UpdateMeta: %v", err)
	}
	blocks := fmt.Sprintf(`[{"typ":"text","markdown":%q},{"typ":"bild","medium":%d},{"typ":"bild","medium":%d}]`,
		seoBody, bare.ID, described.ID)
	p, err := page.NewStore(database).CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: goodTitle, Slug: "bloecke", Status: "published", Blocks: blocks,
		Meta: page.PageMeta{MetaDescription: goodDesc},
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	body := serve(t, h, sm, h.HandlePageEdit, seoEditRequest(ws.ID, p.ID)).Body.String()
	if !strings.Contains(body, "1 images have no description") {
		t.Errorf("one bare picture and one described one: the tab should count exactly one\n%s",
			body[strings.Index(body, "editor-panel--seo"):][:1500])
	}
}
