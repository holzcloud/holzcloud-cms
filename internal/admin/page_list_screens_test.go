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

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The page list and what can be done from it: the screen an editor lives in.
//
// Four of the six handlers here change a page's status, and the rule they share
// is the one the editor above them already obeys — somebody who may not publish
// does not publish through the selection list either. That rule had a comment
// and no test.
//
// Driven red by eighteen mutations, all eighteen caught. Two had to be chased
// down afterwards, and both are the same lesson in a different disguise: an
// assertion that a version doing LESS also satisfies.
//
//   - Turning the search branch off entirely left the unfiltered listing, which
//     showed the page the search was supposed to find. The assertion is on the
//     TITLE that must not come back, because titles are what a listing shows
//     and body text is what only a search snippet shows.
//   - The duplicate's website check has two halves, and only `src == nil` was
//     covered. The other half needed another website's page.
//
// Writing them also found `src.Title + " (Kopie)"` — a German suffix glued on
// in Go, so every installation in every language titled its copies that way.
// A concatenation is invisible to the collector: the gate reported neither open
// nor orphaned about it.

// editorWithoutPublishing creates a person the rights say may not publish, and
// returns their id for serveAs.
func editorWithoutPublishing(t *testing.T, database *db.DB, websiteID int64) int64 {
	t.Helper()
	ctx := context.Background()
	users := user.NewStore(database, cheapHashing)
	id, err := users.Create(ctx, "Redakteurin", "red@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}
	if err := users.SetRights(ctx, id, user.Rights{MayPublish: false, Websites: []int64{websiteID}}); err != nil {
		t.Fatalf("SetRights: %v", err)
	}
	// The premise: without this the tests below would measure a guard that
	// refuses everybody, or one that refuses nobody.
	rights, err := users.Rights(ctx, id)
	if err != nil || rights.MayPublish {
		t.Fatalf("the editor may publish after all: %+v (%v)", rights, err)
	}
	return id
}

func pageListRequest(websiteID int64, query string) *http.Request {
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/admin/websites/%d/pages%s", websiteID, query), nil)
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	return req
}

func TestThePageListShowsFiltersAndSearches(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "Wir bauen Möbel.", "published")
	seedPage(t, database, ws.ID, "Entwurf", "entwurf", "Noch nicht fertig.", "draft")

	// Everything, unfiltered.
	rec := serve(t, h, sm, h.HandlePageList, pageListRequest(ws.ID, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Über uns") || !strings.Contains(body, "Entwurf") {
		t.Error("the list does not show both pages")
	}

	// Filtered by status.
	rec = serve(t, h, sm, h.HandlePageList, pageListRequest(ws.ID, "?status=draft"))
	body = rec.Body.String()
	if !strings.Contains(body, "Entwurf") {
		t.Error("the draft filter left out the draft")
	}
	if strings.Contains(body, "Über uns") {
		t.Error("the draft filter kept the published page")
	}

	// A search REPLACES the listing rather than filtering it, which is why it
	// is a different branch: ranking by relevance and paginating by date at the
	// same time would be a lie about the order.
	//
	// Asserted on the TITLE of the page that must not come back, not on its
	// body text: the unfiltered listing shows titles and no bodies, so a
	// version that skipped the search branch entirely and listed everything
	// would satisfy a body-text assertion without searching at all.
	rec = serve(t, h, sm, h.HandlePageList, pageListRequest(ws.ID, "?q=M%C3%B6bel"))
	body = rec.Body.String()
	if !strings.Contains(body, "Über uns") {
		t.Error("the search did not find the page whose text it names")
	}
	if strings.Contains(body, "Entwurf") {
		t.Error("the search returned a page it does not match — or did not search at all")
	}

	// An unknown website is a 404 and not a 500.
	if rec := serve(t, h, sm, h.HandlePageList, pageListRequest(999, "")); rec.Code != http.StatusNotFound {
		t.Errorf("an unknown website: status %d, want 404", rec.Code)
	}
}

// Every language unless one is asked for. Defaulting to the main language would
// hide every translation from the list that is supposed to be the place you
// find your pages.
func TestThePageListShowsEveryLanguageUnlessAskedOtherwise(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	if err := domain.NewStore(database).UpdateSettings(ctx, ws.ID, domain.Settings{
		Locale: "de", ExtraLocales: "fr", TimeZone: "Europe/Zurich", OfflineMode: "notfound",
	}); err != nil {
		t.Fatal(err)
	}
	seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "x", "published")
	if _, err := h.pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "À propos", Slug: "a-propos",
		Markdown: "y", Status: "published", Locale: "fr",
	}); err != nil {
		t.Fatal(err)
	}

	body := serve(t, h, sm, h.HandlePageList, pageListRequest(ws.ID, "")).Body.String()
	if !strings.Contains(body, "Über uns") || !strings.Contains(body, "À propos") {
		t.Error("the unfiltered list hides a translation")
	}

	body = serve(t, h, sm, h.HandlePageList, pageListRequest(ws.ID, "?sprache=fr")).Body.String()
	if !strings.Contains(body, "À propos") {
		t.Error("the French filter left out the French page")
	}
	if strings.Contains(body, "Über uns") {
		t.Error("the French filter kept the German page")
	}
}

func TestTheStatusToggleSwitchesBothWays(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "x", "draft")
	route := websiteRoute(ws.ID, "pageID", strconv.FormatInt(p.ID, 10))

	rec := serve(t, h, sm, h.HandlePageStatusToggle,
		postForm("/admin/websites/1/pages/1/status", nil, route))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	after, _ := h.pages.GetPage(ctx, p.ID)
	if after == nil || after.Status != "published" {
		t.Fatalf("the draft was not published: %+v", after)
	}

	// And back.
	serve(t, h, sm, h.HandlePageStatusToggle, postForm("/admin/websites/1/pages/1/status", nil, route))
	back, _ := h.pages.GetPage(ctx, p.ID)
	if back == nil || back.Status != "draft" {
		t.Errorf("the published page was not withdrawn: %+v", back)
	}

	// Another website's page is a 404 and stays as it was.
	other, err := domain.NewStore(database).CreateWebsite(ctx, "Die andere", "")
	if err != nil {
		t.Fatal(err)
	}
	fremd := seedPage(t, database, other.ID, "Fremd", "fremd", "x", "draft")
	rec = serve(t, h, sm, h.HandlePageStatusToggle, postForm("/admin/websites/1/pages/1/status",
		nil, websiteRoute(ws.ID, "pageID", strconv.FormatInt(fremd.ID, 10))))
	if rec.Code != http.StatusNotFound {
		t.Errorf("another website's page: status %d, want 404", rec.Code)
	}
	still, _ := h.pages.GetPage(ctx, fremd.ID)
	if still == nil || still.Status != "draft" {
		t.Errorf("another website's page was published from here: %+v", still)
	}
}

// The rule the selection list must not be a back door to.
func TestSomebodyWhoMayNotPublishPublishesNowhere(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	editor := editorWithoutPublishing(t, database, ws.ID)
	p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "x", "draft")

	// Not through the single toggle.
	rec, flash := serveAs(t, h, sm, editor, h.HandlePageStatusToggle,
		postForm("/admin/websites/1/pages/1/status", nil,
			websiteRoute(ws.ID, "pageID", strconv.FormatInt(p.ID, 10))))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status %d", rec.Code)
	}
	if flash == "" {
		t.Error("the refusal was silent")
	}
	if after, _ := h.pages.GetPage(ctx, p.ID); after == nil || after.Status != "draft" {
		t.Fatalf("the page was published by somebody who may not: %+v", after)
	}

	// And not through the selection list either, which is the case the comment
	// above the guard names.
	for _, action := range []string{"publish", "unpublish"} {
		rec, flash := serveAs(t, h, sm, editor, h.HandlePageBulk,
			postForm("/admin/websites/1/pages/bulk", url.Values{
				"action": {action}, "page_ids": {strconv.FormatInt(p.ID, 10)},
			}, websiteRoute(ws.ID)))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status %d", action, rec.Code)
		}
		if flash == "" {
			t.Errorf("%s: the refusal was silent", action)
		}
		if after, _ := h.pages.GetPage(ctx, p.ID); after == nil || after.Status != "draft" {
			t.Errorf("%s changed the status to %q", action, after.Status)
		}
	}

	// The trash is not a publishing right, so it still works — otherwise this
	// test would pass against a guard that refuses the whole screen.
	rec, _ = serveAs(t, h, sm, editor, h.HandlePageBulk,
		postForm("/admin/websites/1/pages/bulk", url.Values{
			"action": {"trash"}, "page_ids": {strconv.FormatInt(p.ID, 10)},
		}, websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("trash: status %d", rec.Code)
	}
	if after, _ := h.pages.GetPage(ctx, p.ID); after == nil || !after.InTrash() {
		t.Error("the trash action did not work for somebody who may not publish")
	}
}

// A bulk action over a selection that is partly somebody else's: the foreign
// pages are left alone AND counted, because a bare "done" would hide it.
func TestABulkActionSkipsWhatIsNotThisWebsitesAndSaysSo(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	mine := seedPage(t, database, ws.ID, "Meins", "meins", "x", "draft")
	other, err := domain.NewStore(database).CreateWebsite(ctx, "Die andere", "")
	if err != nil {
		t.Fatal(err)
	}
	fremd := seedPage(t, database, other.ID, "Fremd", "fremd", "x", "draft")

	_, _, good := albumFlash(t, h, sm, h.HandlePageBulk,
		postForm("/admin/websites/1/pages/bulk", url.Values{
			"action": {"publish"},
			"page_ids": {
				strconv.FormatInt(mine.ID, 10),
				strconv.FormatInt(fremd.ID, 10),
				"999", "keine-zahl",
			},
		}, websiteRoute(ws.ID)))

	if after, _ := h.pages.GetPage(ctx, mine.ID); after == nil || after.Status != "published" {
		t.Errorf("this website's page was not published: %+v", after)
	}
	if after, _ := h.pages.GetPage(ctx, fremd.ID); after == nil || after.Status != "draft" {
		t.Fatalf("another website's page was published through the selection list: %+v", after)
	}
	// One done, two skipped — "keine-zahl" never becomes an id at all.
	if !strings.Contains(good, "1") {
		t.Errorf("the message does not say how many were done: %q", good)
	}
	if !strings.Contains(good, "2") {
		t.Errorf("the message does not say how many were skipped: %q", good)
	}
}

func TestABulkActionWithoutASelectionOrWithoutAnActionIsRefused(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "x", "draft")

	for _, c := range []struct {
		what string
		form url.Values
	}{
		{"no selection", url.Values{"action": {"publish"}}},
		{"only unusable ids", url.Values{"action": {"publish"}, "page_ids": {"keine-zahl"}}},
		{"an action nobody offers", url.Values{
			"action": {"loeschen-fuer-immer"}, "page_ids": {strconv.FormatInt(p.ID, 10)}}},
	} {
		rec, bad, _ := albumFlash(t, h, sm, h.HandlePageBulk,
			postForm("/admin/websites/1/pages/bulk", c.form, websiteRoute(ws.ID)))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status %d", c.what, rec.Code)
		}
		if bad == "" {
			t.Errorf("%s: accepted in silence", c.what)
		}
	}

	if after, _ := h.pages.GetPage(ctx, p.ID); after == nil || after.Status != "draft" || after.InTrash() {
		t.Errorf("a refused bulk action changed the page: %+v", after)
	}
}

// A copy is always a draft, whatever the original was: duplicating a live page
// and instantly publishing an unedited copy of it is never what anyone meant.
func TestADuplicateIsADraftAndKeepsItsKind(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	src, err := h.pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "Neue Werkbank", Slug: "neue-werkbank",
		Markdown: "Endlich fertig.", HTML: "<p>x</p>", Status: "published",
		Kind: page.KindPost,
	})
	if err != nil {
		t.Fatal(err)
	}

	rec := serve(t, h, sm, h.HandlePageDuplicate, postForm("/admin/websites/1/pages/1/duplicate",
		nil, websiteRoute(ws.ID, "pageID", strconv.FormatInt(src.ID, 10))))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}

	all, _, err := h.pages.ListPages(ctx, ws.ID, page.ListFilter{Locale: "*", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatal(err)
	}
	var copied *page.Page
	for i := range all {
		if all[i].ID != src.ID {
			copied = &all[i]
		}
	}
	if copied == nil {
		t.Fatal("no copy was made")
	}
	if copied.Status != "draft" {
		t.Errorf("the copy is %q — a copy of a live page must not go live unedited", copied.Status)
	}
	if copied.Slug == src.Slug {
		t.Errorf("the copy took the original's address %q", copied.Slug)
	}
	// A copy of a post is a post: filing it under "pages" puts it where the
	// person who just duplicated it would not look.
	if !copied.IsPost() {
		t.Error("the copy is not a post although the original was")
	}
	if copied.ContentMarkdown != src.ContentMarkdown {
		t.Errorf("the copy's text is %q", copied.ContentMarkdown)
	}
	// The suffix is a sentence an operator reads, so it goes through the
	// catalogue rather than being glued on in Go.
	if !strings.HasPrefix(copied.Title, src.Title) || copied.Title == src.Title {
		t.Errorf("the copy is titled %q", copied.Title)
	}

	// And another website's page is not duplicated into this one, which is the
	// half of the guard that `src == nil` does not cover.
	other, err := domain.NewStore(database).CreateWebsite(ctx, "Die andere", "")
	if err != nil {
		t.Fatal(err)
	}
	fremd := seedPage(t, database, other.ID, "Fremd", "fremd", "x", "published")
	rec = serve(t, h, sm, h.HandlePageDuplicate, postForm("/admin/websites/1/pages/1/duplicate",
		nil, websiteRoute(ws.ID, "pageID", strconv.FormatInt(fremd.ID, 10))))
	if rec.Code != http.StatusNotFound {
		t.Errorf("another website's page: status %d, want 404", rec.Code)
	}
	after, _, err := h.pages.ListPages(ctx, ws.ID, page.ListFilter{Locale: "*", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(all) {
		t.Errorf("another website's page was copied into this one: %d pages, was %d",
			len(after), len(all))
	}
}

// Submitting for review and taking the note off again, which is the same button
// twice.
func TestAPageIsSubmittedForReviewAndTheNoteComesOffAgain(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "x", "draft")
	route := websiteRoute(ws.ID, "pageID", strconv.FormatInt(p.ID, 10))

	rec := serve(t, h, sm, h.HandlePageReview,
		postForm("/admin/websites/1/pages/1/review", nil, route))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	after, _ := h.pages.GetPage(ctx, p.ID)
	if after == nil || after.ReviewState != "pending" {
		t.Fatalf("the page was not submitted: %+v", after)
	}
	// The list counts it, which is what an editor with the right sees.
	body := serve(t, h, sm, h.HandlePageList, pageListRequest(ws.ID, "")).Body.String()
	if !strings.Contains(body, "Über uns") {
		t.Error("the submitted page left the list")
	}

	// The same button again takes it off.
	serve(t, h, sm, h.HandlePageReview, postForm("/admin/websites/1/pages/1/review", nil, route))
	back, _ := h.pages.GetPage(ctx, p.ID)
	if back == nil || back.ReviewState == "pending" {
		t.Errorf("the review note is still on: %+v", back)
	}
}
