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

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/value"
)

// The Values screen (phase 33, plan 02). Every helper in this file starts with
// screenValue: plans 03 and 05 add test files to this package in the same wave.

func screenValueList(websiteID int64) string {
	return fmt.Sprintf("/admin/websites/%d/werte", websiteID)
}

func screenValueDelete(websiteID, valueID int64) string {
	return fmt.Sprintf("/admin/websites/%d/werte/%d/delete", websiteID, valueID)
}

func screenValueDeleteRoute(websiteID, valueID int64) map[string]string {
	return websiteRoute(websiteID, "valueID", strconv.FormatInt(valueID, 10))
}

// screenValueSeed stores a value or fails the test.
func screenValueSeed(t *testing.T, store *value.Store, websiteID int64, key, text string) *value.Value {
	t.Helper()
	v, err := store.Create(context.Background(), websiteID, key, text)
	if err != nil {
		t.Fatalf("Create(%q): %v", key, err)
	}
	return v
}

// screenValueServe runs one handler as a user and hands back both flashes:
// the refusal and the success, because a delete answers with either.
func screenValueServe(t *testing.T, h *Handler, sm *scs.SessionManager, userID int64,
	fn func(http.ResponseWriter, *http.Request) error, req *http.Request) (rec *httptest.ResponseRecorder, bad, good string) {
	t.Helper()
	rec = httptest.NewRecorder()
	var handlerErr error
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, userID)
		handlerErr = fn(w, r)
		bad = sm.GetString(r.Context(), auth.SessionKeyFlashError)
		good = sm.GetString(r.Context(), auth.SessionKeyFlashSuccess)
	})).ServeHTTP(rec, req)
	if handlerErr != nil {
		t.Fatalf("handler: %v", handlerErr)
	}
	return rec, bad, good
}

// screenValueSeedPage stores a page with a language and a status.
func screenValueSeedPage(t *testing.T, h *Handler, websiteID int64, title, slug, locale, status, markdown string) *page.Page {
	t.Helper()
	html, err := page.RenderMarkdown(markdown)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	p, err := h.pages.CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID, Title: title, Slug: slug, Locale: locale,
		Markdown: markdown, HTML: html, Status: status,
	})
	if err != nil {
		t.Fatalf("CreatePage(%q): %v", title, err)
	}
	return p
}

func TestAnEditorWithoutPublishingCannotChangeValues(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	editor := editorWithoutPublishing(t, database, ws.ID)
	v := screenValueSeed(t, h.valueStore, ws.ID, "version", "0.1.5")
	list := screenValueList(ws.ID)
	route := websiteRoute(ws.ID)

	stored := func(t *testing.T) []value.Value {
		t.Helper()
		got, err := h.valueStore.List(ctx, ws.ID)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		return got
	}

	t.Run("create", func(t *testing.T) {
		form := url.Values{"key": {"neu"}, "text": {"Neu"}}
		rec, flash := serveAs(t, h, sm, editor, h.HandleValueList, postForm(list, form, route))
		wantRefused(t, "value create", rec, flash, list)
		rec, flash = serveAs(t, h, sm, editor, h.HandleValueList, htmx(postForm(list, form, route)))
		wantRefusedHtmx(t, "value create", rec, flash, list)
		if got := stored(t); len(got) != 1 {
			t.Errorf("%d values stored, want the one seeded", len(got))
		}
	})

	t.Run("change", func(t *testing.T) {
		form := url.Values{"id": {strconv.FormatInt(v.ID, 10)}, "text": {"0.2.0"}}
		rec, flash := serveAs(t, h, sm, editor, h.HandleValueList, postForm(list, form, route))
		wantRefused(t, "value change", rec, flash, list)
		rec, flash = serveAs(t, h, sm, editor, h.HandleValueList, htmx(postForm(list, form, route)))
		wantRefusedHtmx(t, "value change", rec, flash, list)
		if got, _ := h.valueStore.Get(ctx, ws.ID, v.ID); got == nil || got.Text != "0.1.5" {
			t.Errorf("the value was changed: %+v", got)
		}
	})

	t.Run("delete", func(t *testing.T) {
		target := screenValueDelete(ws.ID, v.ID)
		delRoute := screenValueDeleteRoute(ws.ID, v.ID)
		rec, flash := serveAs(t, h, sm, editor, h.HandleValueDelete, postForm(target, nil, delRoute))
		wantRefused(t, "value delete", rec, flash, list)
		rec, flash = serveAs(t, h, sm, editor, h.HandleValueDelete, htmx(postForm(target, nil, delRoute)))
		wantRefusedHtmx(t, "value delete", rec, flash, list)
		if got, _ := h.valueStore.Get(ctx, ws.ID, v.ID); got == nil {
			t.Error("the value was deleted")
		}
	})

	t.Run("screen", func(t *testing.T) {
		form := fmt.Sprintf(`action="%s"`, list)
		opened := fmt.Sprintf(`%s?edit=%d`, list, v.ID)
		saveButton := `hx-disabled-elt="this">Save</button>`

		body := getAs(t, sm, putUser(sm, editor), h.HandleValueList, list, route)
		if n := strings.Count(body, readOnlyText); n != 1 {
			t.Errorf("value list: the sentence stands %d times, want once", n)
		}
		if strings.Contains(body, form) {
			t.Error("value list offers the form to create a value")
		}
		if anyDelete.MatchString(body) {
			t.Error("value list offers delete")
		}
		if !strings.Contains(body, opened+`" class="btn btn--sm">View</a>`) {
			t.Error("value list: the link to the value does not read View")
		}

		body = getAs(t, sm, putUser(sm, editor), h.HandleValueList, opened, route)
		if n := strings.Count(body, readOnlyText); n != 1 {
			t.Errorf("opened value: the sentence stands %d times, want once", n)
		}
		if !disabledFieldset.MatchString(body) {
			t.Error("opened value: the fields are not in a disabled fieldset")
		}
		if !strings.Contains(body, `value="0.1.5"`) {
			t.Error("opened value: its text is not shown")
		}
		if strings.Contains(body, saveButton) {
			t.Error("opened value: a save button is offered")
		}

		publisher := editorWhoMayPublish(t, database, ws.ID)
		body = getAs(t, sm, putUser(sm, publisher), h.HandleValueList, list, route)
		if strings.Contains(body, readOnlyText) {
			t.Error("a person who may publish reads the read-only sentence")
		}
		if !strings.Contains(body, form) || !anyDelete.MatchString(body) {
			t.Error("a person who may publish is not offered create and delete")
		}
		if !strings.Contains(body, opened+`" class="btn btn--sm">Edit</a>`) {
			t.Error("a person who may publish is not offered Edit")
		}
	})
}

func TestDeletingAValueInUseIsRefusedAndNamesThePages(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	publisher := editorWhoMayPublish(t, database, ws.ID)
	used := screenValueSeed(t, h.valueStore, ws.ID, "v", "0.1.5")
	free := screenValueSeed(t, h.valueStore, ws.ID, "frei", "niemand")
	screenValueSeedPage(t, h, ws.ID, "Page B", "page-b", "fr", "draft", "Version [[wert:v]]")
	seedSnippet(t, h.snippets, ws.ID, "fuss", "Fuss", "Stand [[wert:v]]")
	list := screenValueList(ws.ID)

	rec, bad, _ := screenValueServe(t, h, sm, publisher, h.HandleValueDelete,
		postForm(screenValueDelete(ws.ID, used.ID), nil, screenValueDeleteRoute(ws.ID, used.ID)))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != list {
		t.Errorf("in use: status %d to %q, want 303 to the list", rec.Code, rec.Header().Get("Location"))
	}
	want := "The value is still used on: Page B (fr), Fuss [[snippet:fuss]]. Remove the placeholders from those pages first."
	if bad != want {
		t.Errorf("in use: flash %q, want %q", bad, want)
	}
	if got, _ := h.valueStore.Get(ctx, ws.ID, used.ID); got == nil {
		t.Error("a value in use was deleted")
	}

	rec, bad, good := screenValueServe(t, h, sm, publisher, h.HandleValueDelete,
		postForm(screenValueDelete(ws.ID, free.ID), nil, screenValueDeleteRoute(ws.ID, free.ID)))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != list {
		t.Errorf("unused: status %d to %q, want 303 to the list", rec.Code, rec.Header().Get("Location"))
	}
	if bad != "" || good != "Value deleted" {
		t.Errorf("unused: flashes %q / %q, want only Value deleted", bad, good)
	}
	if got, _ := h.valueStore.Get(ctx, ws.ID, free.ID); got != nil {
		t.Error("an unused value was not deleted")
	}
}

func TestAValueOfAnotherWebsiteCannotBeOpenedOrChanged(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	other, err := h.domains.CreateWebsite(ctx, "Andere", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	foreign := screenValueSeed(t, h.valueStore, other.ID, "fremd", "Nicht deins")
	list := screenValueList(ws.ID)
	route := websiteRoute(ws.ID)

	for _, who := range []struct {
		name string
		id   int64
	}{
		{"publisher", editorWhoMayPublish(t, database, ws.ID)},
		{"read-only", editorWithoutPublishing(t, database, ws.ID)},
	} {
		t.Run(who.name, func(t *testing.T) {
			body := getAs(t, sm, putUser(sm, who.id), h.HandleValueList,
				fmt.Sprintf("%s?edit=%d", list, foreign.ID), route)
			if strings.Contains(body, "Nicht deins") || strings.Contains(body, "fremd") {
				t.Error("?edit= with another website's id shows that value")
			}

			rec, flash := serveAs(t, h, sm, who.id, h.HandleValueList, postForm(list, url.Values{
				"id": {strconv.FormatInt(foreign.ID, 10)}, "text": {"Gekapert"},
			}, route))
			if rec.Code != http.StatusNotFound {
				t.Errorf("edit of a foreign id: status %d, want 404", rec.Code)
			}
			if flash == readOnlyText {
				t.Error("edit of a foreign id answered with the read-only sentence instead of 404")
			}

			rec, flash = serveAs(t, h, sm, who.id, h.HandleValueDelete, postForm(
				screenValueDelete(ws.ID, foreign.ID), nil, screenValueDeleteRoute(ws.ID, foreign.ID)))
			if rec.Code != http.StatusNotFound {
				t.Errorf("delete of a foreign id: status %d, want 404", rec.Code)
			}
			if flash == readOnlyText {
				t.Error("delete of a foreign id answered with the read-only sentence instead of 404")
			}

			got, _ := h.valueStore.Get(ctx, other.ID, foreign.ID)
			if got == nil || got.Text != "Nicht deins" {
				t.Errorf("the other website's value was touched: %+v", got)
			}
		})
	}
}

func TestAKeyCannotBeRenamed(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	publisher := editorWhoMayPublish(t, database, ws.ID)
	v := screenValueSeed(t, h.valueStore, ws.ID, "version", "0.1.5")

	rec, bad, _ := screenValueServe(t, h, sm, publisher, h.HandleValueList, postForm(screenValueList(ws.ID), url.Values{
		"id": {strconv.FormatInt(v.ID, 10)}, "key": {"anders"}, "text": {"0.1.6"},
	}, websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d (%q), want 303", rec.Code, bad)
	}
	got, _ := h.valueStore.Get(ctx, ws.ID, v.ID)
	if got == nil || got.Key != "version" || got.Text != "0.1.6" {
		t.Errorf("after an edit naming another key: %+v, want key version and text 0.1.6", got)
	}
	if renamed, _ := h.valueStore.GetByKey(ctx, ws.ID, "anders"); renamed != nil {
		t.Error("a value with the posted key exists")
	}
}

func TestAnInvalidValueShowsTheErrorAtTheField(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	publisher := editorWhoMayPublish(t, database, ws.ID)
	screenValueSeed(t, h.valueStore, ws.ID, "vergeben", "schon da")

	for _, c := range []struct {
		name, key, text, field, sentence string
	}{
		{"bad key", "Version", "0.1.5", "key", "Lower-case letters, digits, hyphen and underscore only."},
		{"taken key", "vergeben", "0.1.5", "key", "That key is already used by another value."},
		{"empty text", "leer", "   ", "text", "Please enter a text."},
		{"too long", "lang", strings.Repeat("a", value.MaxTextBytes+1), "text", "The text is longer than 400 bytes."},
		{"line break", "zeile", "eins\nzwei", "text", "The text must be one line, without line breaks or control characters."},
		{"marker", "marke", "a [[b", "text", "A value cannot contain [ or ]."},
		{"one bracket", "klammer", "wert:x]]", "text", "A value cannot contain [ or ]."},
	} {
		t.Run(c.name, func(t *testing.T) {
			rec, bad, _ := screenValueServe(t, h, sm, publisher, h.HandleValueList, postForm(screenValueList(ws.ID),
				url.Values{"key": {c.key}, "text": {c.text}}, websiteRoute(ws.ID)))
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status %d (%q), want 422", rec.Code, bad)
			}
			want := fmt.Sprintf(`<p class="form-error" id="%s-error">%s</p>`, c.field, c.sentence)
			if !strings.Contains(rec.Body.String(), want) {
				t.Errorf("the form does not show %q at the field %s", c.sentence, c.field)
			}
			got, _ := h.valueStore.List(ctx, ws.ID)
			if len(got) != 1 {
				t.Errorf("%d values stored, want the one seeded", len(got))
			}
		})
	}
}

func TestTheScreenShowsUseUndefinedAndNotReplaced(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	publisher := editorWhoMayPublish(t, database, ws.ID)
	list := screenValueList(ws.ID)
	route := websiteRoute(ws.ID)

	body := getAs(t, sm, putUser(sm, publisher), h.HandleValueList, list, route)
	for _, card := range []string{"Used but not defined", "Not replaced here"} {
		if strings.Contains(body, card) {
			t.Errorf("an empty website shows the card %q", card)
		}
	}

	screenValueSeed(t, h.valueStore, ws.ID, "v", "0.1.5")
	screenValueSeedPage(t, h, ws.ID, "Stand", "stand", "", "published", "Alpha [[wert:v]]: neu")
	gap := screenValueSeedPage(t, h, ws.ID, "Lücke", "luecke", "", "draft", "Telefon [[wert:fehlt]]")
	titled := screenValueSeedPage(t, h, ws.ID, "Titel [[wert:v]]", "titel", "", "draft", "Nichts")
	menus := menu.NewStore(database)
	m, err := menus.CreateMenu(ctx, ws.ID, "Haupt", "haupt", "")
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}
	if _, err := menus.CreateItem(ctx, m.ID, nil, "Version [[wert:v]]", "url", "/x", nil, 0); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	body = getAs(t, sm, putUser(sm, publisher), h.HandleValueList, list, route)

	undefined := screenValueCard(t, body, "Used but not defined")
	if !strings.Contains(undefined, "<code>[[wert:fehlt]]</code>") {
		t.Error("the undefined key is not listed as its marker")
	}
	if !strings.Contains(undefined, fmt.Sprintf(`href="/admin/websites/%d/pages/%d/edit">Lücke</a>`, ws.ID, gap.ID)) {
		t.Error("the page naming the undefined key is not linked to its editor")
	}
	if strings.Contains(undefined, "[[wert:v]]") {
		t.Error("a defined key is listed as undefined")
	}

	outside := screenValueCard(t, body, "Not replaced here")
	if !strings.Contains(outside, "Page title") ||
		!strings.Contains(outside, fmt.Sprintf(`href="/admin/websites/%d/pages/%d/edit">Titel [[wert:v]]</a>`, ws.ID, titled.ID)) {
		t.Error("the marker in a page title is not reported with its page")
	}
	if !strings.Contains(outside, "Menu entry") || !strings.Contains(outside, "Version [[wert:v]]") {
		t.Error("the marker in a menu entry is not reported")
	}

	row := screenValueRowOf(t, body, "v")
	if !strings.Contains(row, "on one page") {
		t.Errorf("the in-use column of v does not read on one page: %s", row)
	}
}

// screenValueCard cuts the card whose heading is title out of a body.
func screenValueCard(t *testing.T, body, title string) string {
	t.Helper()
	i := strings.Index(body, "<h2>"+title+"</h2>")
	if i < 0 {
		t.Fatalf("no card %q on the screen", title)
	}
	rest := body[i:]
	if j := strings.Index(rest, `<div class="card"`); j > 0 {
		rest = rest[:j]
	}
	return rest
}

// screenValueRowOf cuts the table row of one key out of a body.
func screenValueRowOf(t *testing.T, body, key string) string {
	t.Helper()
	i := strings.Index(body, "<td><code>[[wert:"+key+"]]</code></td>")
	if i < 0 {
		t.Fatalf("no row for %q", key)
	}
	rest := body[i:]
	if j := strings.Index(rest, "</tr>"); j > 0 {
		rest = rest[:j]
	}
	return rest
}

func TestThePageEditorListsTheValues(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	ctx := context.Background()
	other, err := h.domains.CreateWebsite(ctx, "Andere", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	screenValueSeed(t, h.valueStore, ws.ID, "version-holzcloud-cms", "0.1.5")
	screenValueSeed(t, h.valueStore, other.ID, "fremd", "Nicht deins")
	p := screenValueSeedPage(t, h, ws.ID, "Stand", "stand", "", "draft", "Alpha [[wert:version-holzcloud-cms]]: neu")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/websites/%d/pages/%d/edit", ws.ID, p.ID), nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	req.SetPathValue("pageID", strconv.FormatInt(p.ID, 10))
	body := serve(t, h, sm, h.HandlePageEdit, req).Body.String()

	i := strings.Index(body, "<summary>Insert a value</summary>")
	if i < 0 {
		t.Fatal("the page editor offers no Insert a value")
	}
	help := body[i:]
	if j := strings.Index(help, "</details>"); j > 0 {
		help = help[:j]
	}
	if !strings.Contains(help, "<code>[[wert:version-holzcloud-cms]]</code> — 0.1.5") {
		t.Error("Insert a value does not list the marker with its text")
	}
	if !strings.Contains(help, fmt.Sprintf(`href="/admin/websites/%d/werte"`, ws.ID)) {
		t.Error("Insert a value does not link to the Values screen")
	}
	if strings.Contains(body, "fremd") || strings.Contains(body, "Nicht deins") {
		t.Error("another website's value is listed in the page editor")
	}
	if !strings.Contains(body, "Alpha 0.1.5: neu") {
		t.Error("the preview pane of the opened page does not show the value")
	}
}

func TestTheLivePreviewShowsTheValue(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	screenValueSeed(t, h.valueStore, ws.ID, "v", "0.1.5")
	target := fmt.Sprintf("/admin/websites/%d/pages/preview", ws.ID)

	for _, c := range []struct{ markdown, want string }{
		{"Alpha [[wert:v]]: neu", "Alpha 0.1.5: neu"},
		{"Alpha [[wert:fehlt]]: neu", "Alpha : neu"},
	} {
		rec := serve(t, h, sm, h.HandlePagePreview, postForm(target,
			url.Values{"content_markdown": {c.markdown}}, websiteRoute(ws.ID)))
		body := rec.Body.String()
		if !strings.Contains(body, c.want) {
			t.Errorf("preview of %q lacks %q: %s", c.markdown, c.want, body)
		}
		if strings.Contains(body, "[[wert:") {
			t.Errorf("preview of %q shows the raw marker", c.markdown)
		}
	}
}
