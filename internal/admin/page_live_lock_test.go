package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// Without the right to publish, nobody changes what the public sees through a
// page.
//
// The user form promised it — "Without this right one writes and submits for
// review; somebody else puts it online" — and only the status toggle and the
// two bulk status actions kept the promise. Everything else that writes a page
// changed a live one for anybody: the save, the inline rename, the
// wastebasket (single and bulk), restoring out of the wastebasket, restoring
// an older version and inserting a picture. Audit of 2026-11-10, Part 1,
// item 7.
//
// Each door is tested with a hand-built request and not through the screen,
// because the screen hiding a button is a courtesy and the handler refusing is
// the border. Each asserts the stored row afterwards and that the refusal was
// not silent.

// liveLockSentence is the start of the refusal the editor reads. Tests run in
// English, so the rendered sentence is the catalogue key.
const liveLockSentence = "This page is online"

func putForm(target string, values url.Values, pathValues map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodPut, target, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	return req
}

func TestAnEditorWithoutPublishingCannotChangeAPublishedPage(t *testing.T) {
	t.Run("the save", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		ctx := context.Background()
		editor := editorWithoutPublishing(t, database, ws.ID)
		p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "der alte text", "published")

		const typed = "Ein neuer Absatz, den die Redakteurin getippt hat."
		rec, _ := serveAs(t, h, sm, editor, h.HandlePageEdit,
			postForm("/admin/websites/1/pages/1/edit", url.Values{
				"title":            {"Neuer Titel"},
				"slug":             {"neue-adresse"},
				"content_markdown": {typed},
				"status":           {"published"},
				"publish_at":       {"2031-05-06T07:08"},
				"version":          {strconv.FormatInt(p.Version, 10)},
			}, pageRoute(ws.ID, p.ID)))

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status %d, want 422", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, typed) {
			t.Error("the typed text is not handed back")
		}
		if !strings.Contains(body, liveLockSentence) {
			t.Error("the refusal does not say why")
		}
		after, err := h.pages.GetPage(ctx, p.ID)
		if err != nil || after == nil {
			t.Fatalf("GetPage: %v", err)
		}
		if after.Title != p.Title || after.ContentMarkdown != p.ContentMarkdown || after.Slug != p.Slug {
			t.Errorf("the live page was changed: %q %q %q", after.Title, after.Slug, after.ContentMarkdown)
		}
		if after.PublishAt != nil || after.UnpublishAt != nil {
			t.Errorf("the schedule was changed: %v %v", after.PublishAt, after.UnpublishAt)
		}
		if after.Version != p.Version {
			t.Errorf("the version moved from %d to %d", p.Version, after.Version)
		}
	})

	t.Run("the inline rename", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		ctx := context.Background()
		editor := editorWithoutPublishing(t, database, ws.ID)
		p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "text", "published")

		_, flash := serveAs(t, h, sm, editor, h.HandlePageInlineEditSave,
			putForm("/admin/websites/1/pages/1/edit-title", url.Values{"title": {"Umbenannt"}},
				pageRoute(ws.ID, p.ID)))
		if flash == "" {
			t.Error("the refusal was silent")
		}
		after, _ := h.pages.GetPage(ctx, p.ID)
		if after == nil || after.Title != p.Title || after.Slug != p.Slug {
			t.Errorf("the live page was renamed: %+v", after)
		}
	})

	t.Run("the wastebasket", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		ctx := context.Background()
		editor := editorWithoutPublishing(t, database, ws.ID)
		p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "text", "published")

		_, flash := serveAs(t, h, sm, editor, h.HandlePageDelete,
			postForm("/admin/websites/1/pages/1/delete", nil, pageRoute(ws.ID, p.ID)))
		if flash == "" {
			t.Error("the refusal was silent")
		}
		after, _ := h.pages.GetPage(ctx, p.ID)
		if after == nil || after.InTrash() {
			t.Error("the live page was moved to the wastebasket")
		}
	})

	t.Run("the bulk wastebasket", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		ctx := context.Background()
		editor := editorWithoutPublishing(t, database, ws.ID)
		live := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "text", "published")
		draft := seedPage(t, database, ws.ID, "Entwurf", "entwurf", "text", "draft")

		_, flash := serveAs(t, h, sm, editor, h.HandlePageBulk,
			postForm("/admin/websites/1/pages/bulk", url.Values{
				"action":   {"trash"},
				"page_ids": {strconv.FormatInt(live.ID, 10), strconv.FormatInt(draft.ID, 10)},
			}, websiteRoute(ws.ID)))
		if flash == "" {
			t.Error("the refusal was silent")
		}
		if after, _ := h.pages.GetPage(ctx, live.ID); after == nil || after.InTrash() {
			t.Error("the live page was moved to the wastebasket")
		}
		if after, _ := h.pages.GetPage(ctx, draft.ID); after == nil || !after.InTrash() {
			t.Error("the draft beside it was not moved — the guard refuses too much")
		}
	})

	t.Run("restoring out of the wastebasket", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		ctx := context.Background()
		editor := editorWithoutPublishing(t, database, ws.ID)
		p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "text", "published")
		if err := h.pages.TrashPage(ctx, p.ID); err != nil {
			t.Fatalf("TrashPage: %v", err)
		}

		_, flash := serveAs(t, h, sm, editor, h.HandleTrashRestore,
			postForm("/admin/websites/1/trash/1/restore", nil, pageRoute(ws.ID, p.ID)))
		if flash == "" {
			t.Error("the refusal was silent")
		}
		if after, _ := h.pages.GetPage(ctx, p.ID); after == nil || !after.InTrash() {
			t.Error("a page that was online came back online")
		}
	})

	t.Run("restoring an older version", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		ctx := context.Background()
		editor := editorWithoutPublishing(t, database, ws.ID)
		p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "die alte fassung", "draft")
		if err := h.pages.UpdatePage(ctx, p.ID, page.PageUpdate{
			Title: p.Title, Slug: p.Slug, Markdown: "die neue fassung", HTML: "<p>neu</p>",
			Status: "draft", ExpectedVersion: p.Version,
		}); err != nil {
			t.Fatalf("UpdatePage: %v", err)
		}
		if err := h.pages.SetPageStatus(ctx, p.ID, "published", nil); err != nil {
			t.Fatalf("SetPageStatus: %v", err)
		}
		revs, err := h.pages.ListRevisions(ctx, p.ID)
		if err != nil || len(revs) == 0 {
			t.Fatalf("ListRevisions: %v (%d)", err, len(revs))
		}

		_, flash := serveAs(t, h, sm, editor, h.HandlePageRevisionRestore,
			postForm("/admin/websites/1/pages/1/revisions/1/restore", nil, websiteRoute(ws.ID,
				"pageID", strconv.FormatInt(p.ID, 10),
				"revID", strconv.FormatInt(revs[len(revs)-1].ID, 10))))
		if flash == "" {
			t.Error("the refusal was silent")
		}
		if after, _ := h.pages.GetPage(ctx, p.ID); after == nil || after.ContentMarkdown != "die neue fassung" {
			t.Errorf("an older version was put online: %+v", after)
		}
	})

	t.Run("inserting a picture", func(t *testing.T) {
		h, sm, database, ws := mediaAdmin(t)
		ctx := context.Background()
		editor := editorWithoutPublishing(t, database, ws.ID)
		serve(t, h, sm, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "werkstatt.png", onePixelPNG))
		items, _, err := media.NewStore(database).List(ctx, ws.ID, media.Filter{}, 1, 10)
		if err != nil || len(items) != 1 {
			t.Fatalf("%d files: %v", len(items), err)
		}
		p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "der text", "published")

		_, flash := serveAs(t, h, sm, editor, h.HandlePageInsertMedia,
			postForm("/admin/websites/1/pages/1/bild",
				url.Values{"media_id": {strconv.FormatInt(items[0].ID, 10)}}, pageRoute(ws.ID, p.ID)))
		if flash == "" {
			t.Error("the refusal was silent")
		}
		if after, _ := h.pages.GetPage(ctx, p.ID); after == nil || after.ContentMarkdown != "der text" {
			t.Errorf("a picture was put into the live page: %+v", after)
		}
	})
}

// When a page appears is a publishing decision. On a draft it would otherwise
// take effect unseen the moment a publisher uses the list toggle.
func TestAnEditorWithoutPublishingCannotSetASchedule(t *testing.T) {
	t.Run("a draft takes no schedule", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		ctx := context.Background()
		editor := editorWithoutPublishing(t, database, ws.ID)
		p := seedPage(t, database, ws.ID, "Entwurf", "entwurf", "alt", "draft")

		rec, _ := serveAs(t, h, sm, editor, h.HandlePageEdit,
			postForm("/admin/websites/1/pages/1/edit", url.Values{
				"title":            {"Entwurf"},
				"slug":             {"entwurf"},
				"content_markdown": {"neu"},
				"status":           {"draft"},
				"publish_at":       {"2031-05-06T07:08"},
				"unpublish_at":     {"2032-05-06T07:08"},
				"version":          {strconv.FormatInt(p.Version, 10)},
			}, pageRoute(ws.ID, p.ID)))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status %d, want 303 — the save itself must work", rec.Code)
		}
		after, _ := h.pages.GetPage(ctx, p.ID)
		if after == nil || after.ContentMarkdown != "neu" {
			t.Fatalf("the text was not stored: %+v", after)
		}
		if after.PublishAt != nil || after.UnpublishAt != nil {
			t.Errorf("a schedule was stored: %v %v", after.PublishAt, after.UnpublishAt)
		}
	})

	t.Run("a publisher's schedule stays", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		ctx := context.Background()
		editor := editorWithoutPublishing(t, database, ws.ID)
		p := seedPage(t, database, ws.ID, "Entwurf", "entwurf", "alt", "draft")
		// Seconds on purpose: a round trip through the form's text would lose
		// them.
		from := time.Date(2031, 5, 6, 7, 8, 9, 0, time.UTC)
		until := time.Date(2032, 5, 6, 7, 8, 9, 0, time.UTC)
		if err := h.pages.UpdatePage(ctx, p.ID, page.PageUpdate{
			Title: p.Title, Slug: p.Slug, Markdown: "alt", HTML: "<p>alt</p>", Status: "draft",
			Schedule:        page.PageSchedule{PublishAt: &from, UnpublishAt: &until},
			ExpectedVersion: p.Version,
		}); err != nil {
			t.Fatalf("UpdatePage: %v", err)
		}
		scheduled, _ := h.pages.GetPage(ctx, p.ID)

		rec, _ := serveAs(t, h, sm, editor, h.HandlePageEdit,
			postForm("/admin/websites/1/pages/1/edit", url.Values{
				"title":            {"Entwurf"},
				"slug":             {"entwurf"},
				"content_markdown": {"neu"},
				"status":           {"draft"},
				"publish_at":       {""},
				"unpublish_at":     {""},
				"version":          {strconv.FormatInt(scheduled.Version, 10)},
			}, pageRoute(ws.ID, p.ID)))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status %d, want 303", rec.Code)
		}
		after, _ := h.pages.GetPage(ctx, p.ID)
		if after == nil || after.ContentMarkdown != "neu" {
			t.Fatalf("the text was not stored: %+v", after)
		}
		if after.PublishAt == nil || !after.PublishAt.Equal(from) ||
			after.UnpublishAt == nil || !after.UnpublishAt.Equal(until) {
			t.Errorf("the stored schedule moved: %v %v", after.PublishAt, after.UnpublishAt)
		}
	})

	t.Run("a new page takes no schedule", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		ctx := context.Background()
		editor := editorWithoutPublishing(t, database, ws.ID)

		rec, _ := serveAs(t, h, sm, editor, h.HandlePageCreate,
			postForm("/admin/websites/1/pages/new", url.Values{
				"title":            {"Neu"},
				"slug":             {"neu"},
				"content_markdown": {"text"},
				"status":           {"published"},
				"publish_at":       {"2031-05-06T07:08"},
				"unpublish_at":     {"2032-05-06T07:08"},
			}, websiteRoute(ws.ID)))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status %d, want 303", rec.Code)
		}
		created, err := h.pages.GetPageBySlug(ctx, ws.ID, "neu")
		if err != nil || created == nil {
			t.Fatalf("the page was not created: %v", err)
		}
		if created.Status != "draft" {
			t.Errorf("created as %q", created.Status)
		}
		if created.PublishAt != nil || created.UnpublishAt != nil {
			t.Errorf("a schedule was stored: %v %v", created.PublishAt, created.UnpublishAt)
		}
	})
}

// Without this the tests above would pass against a guard that refuses the
// whole screen.
func TestAnEditorWithoutPublishingStillWorksOnDrafts(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	editor := editorWithoutPublishing(t, database, ws.ID)
	p := seedPage(t, database, ws.ID, "Entwurf", "entwurf", "alt", "draft")

	rec, _ := serveAs(t, h, sm, editor, h.HandlePageEdit,
		postForm("/admin/websites/1/pages/1/edit", url.Values{
			"title":            {"Entwurf"},
			"slug":             {"entwurf"},
			"content_markdown": {"neu"},
			"status":           {"draft"},
			"version":          {strconv.FormatInt(p.Version, 10)},
		}, pageRoute(ws.ID, p.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save: status %d, want 303", rec.Code)
	}
	if after, _ := h.pages.GetPage(ctx, p.ID); after == nil || after.ContentMarkdown != "neu" {
		t.Fatalf("save: the text was not stored: %+v", after)
	}

	_, flash := serveAs(t, h, sm, editor, h.HandlePageInlineEditSave,
		putForm("/admin/websites/1/pages/1/edit-title", url.Values{"title": {"Umbenannt"}},
			pageRoute(ws.ID, p.ID)))
	if flash != "" {
		t.Errorf("rename: refused with %q", flash)
	}
	if after, _ := h.pages.GetPage(ctx, p.ID); after == nil || after.Title != "Umbenannt" {
		t.Errorf("rename: the draft was not renamed: %+v", after)
	}

	_, flash = serveAs(t, h, sm, editor, h.HandlePageDelete,
		postForm("/admin/websites/1/pages/1/delete", nil, pageRoute(ws.ID, p.ID)))
	if flash != "" {
		t.Errorf("trash: refused with %q", flash)
	}
	if after, _ := h.pages.GetPage(ctx, p.ID); after == nil || !after.InTrash() {
		t.Fatal("trash: the draft was not moved to the wastebasket")
	}

	_, flash = serveAs(t, h, sm, editor, h.HandleTrashRestore,
		postForm("/admin/websites/1/trash/1/restore", nil, pageRoute(ws.ID, p.ID)))
	if flash != "" {
		t.Errorf("restore: refused with %q", flash)
	}
	if after, _ := h.pages.GetPage(ctx, p.ID); after == nil || after.InTrash() {
		t.Error("restore: the draft is still in the wastebasket")
	}
}

// primarySubmit is the visible save button's own tag. The schedule inputs are
// disabled for this person on every page, so the bare word "disabled" proves
// nothing about the button.
var primarySubmit = regexp.MustCompile(`<button type="submit" class="btn btn--primary"[^>]*>`)

// disabledAttr is the attribute itself and not a word inside another one:
// every submit button carries hx-disabled-elt.
var disabledAttr = regexp.MustCompile(`\sdisabled[\s>=]`)

func TestTheScreensDoNotOfferAnEditorWhatTheLockRefuses(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	editor := editorWithoutPublishing(t, database, ws.ID)
	live := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "text", "published")
	draft := seedPage(t, database, ws.ID, "Entwurf", "entwurf", "text", "draft")

	form := func(p *page.Page) string {
		req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/edit", nil)
		for k, v := range pageRoute(ws.ID, p.ID) {
			req.SetPathValue(k, v)
		}
		rec, _ := serveAs(t, h, sm, editor, h.HandlePageEdit, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET edit form: status %d", rec.Code)
		}
		return rec.Body.String()
	}

	body := form(live)
	if !strings.Contains(body, liveLockSentence) {
		t.Error("the edit form of a published page does not say it is locked")
	}
	button := primarySubmit.FindString(body)
	if button == "" {
		t.Fatal("no primary submit button on the form")
	}
	if !disabledAttr.MatchString(button) {
		t.Errorf("the save button of a locked page is offered: %s", button)
	}

	body = form(draft)
	if strings.Contains(body, liveLockSentence) {
		t.Error("the edit form of a draft says it is locked")
	}
	button = primarySubmit.FindString(body)
	if button == "" {
		t.Fatal("no primary submit button on the draft's form")
	}
	if disabledAttr.MatchString(button) {
		t.Errorf("the save button of a draft is disabled: %s", button)
	}

	rec, _ := serveAs(t, h, sm, editor, h.HandlePageList, pageListRequest(ws.ID, ""))
	list := rec.Body.String()
	deleteOf := func(p *page.Page) string {
		return "/admin/websites/" + strconv.FormatInt(ws.ID, 10) + "/pages/" + strconv.FormatInt(p.ID, 10) + "/delete"
	}
	if !strings.Contains(list, deleteOf(draft)) {
		t.Error("the draft's row offers no wastebasket")
	}
	if strings.Contains(list, deleteOf(live)) {
		t.Error("the published page's row offers the wastebasket")
	}
}
