package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/snippet"
)

// The rule of read_only_test.go, applied to what the library holds.
//
// A text snippet is expanded into every page that carries its marker at the
// moment the page is delivered, and an album is shown by its gallery as soon
// as it is stored. Both are therefore changes to the live site, and an editor
// without the publishing right may open them and not change them. As in the
// first file, every door is driven with a hand-built request: what the screen
// leaves out is a courtesy, and the handler refusing is the border.

// anyDelete is a form whose action deletes something.
var anyDelete = regexp.MustCompile(`action="[^"]*/delete"`)

func snippetListAddress(websiteID int64) string {
	return fmt.Sprintf("/admin/websites/%d/snippets", websiteID)
}

func seedSnippet(t *testing.T, store *snippet.Store, websiteID int64, key, name, markdown string) *snippet.Snippet {
	t.Helper()
	sn, err := store.Create(context.Background(), websiteID, key, name, markdown, "<p>"+markdown+"</p>")
	if err != nil {
		t.Fatalf("create snippet %q: %v", key, err)
	}
	return sn
}

func snippetsOf(t *testing.T, store *snippet.Store, websiteID int64) []snippet.Snippet {
	t.Helper()
	list, err := store.List(context.Background(), websiteID)
	if err != nil {
		t.Fatalf("List snippets: %v", err)
	}
	return list
}

func TestAnEditorWithoutPublishingCannotChangeSnippets(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	editor := editorWithoutPublishing(t, database, ws.ID)
	sn := seedSnippet(t, h.snippets, ws.ID, "kontakt", "Kontakt", "Bei uns im Laden")
	list := snippetListAddress(ws.ID)
	route := websiteRoute(ws.ID)

	t.Run("create", func(t *testing.T) {
		rec, flash := serveAs(t, h, sm, editor, h.HandleSnippetList, postForm(list, url.Values{
			"key": {"neu"}, "name": {"Neu"}, "content_markdown": {"Neuer Text"},
		}, route))
		wantRefused(t, "snippet create", rec, flash, list)
		if got := snippetsOf(t, h.snippets, ws.ID); len(got) != 1 {
			t.Errorf("%d snippets stored, want the one seeded", len(got))
		}
	})

	t.Run("change", func(t *testing.T) {
		rec, flash := serveAs(t, h, sm, editor, h.HandleSnippetList, postForm(list, url.Values{
			"id": {strconv.FormatInt(sn.ID, 10)}, "key": {"kontakt"}, "name": {"Kontakt"},
			"content_markdown": {"Umgeschrieben"},
		}, route))
		wantRefused(t, "snippet change", rec, flash, list)
		got, err := h.snippets.Get(ctx, ws.ID, sn.ID)
		if err != nil || got == nil {
			t.Fatalf("Get: %v %v", got, err)
		}
		if got.ContentMarkdown != "Bei uns im Laden" {
			t.Errorf("the snippet was changed to %q", got.ContentMarkdown)
		}
	})

	// A read-only person has no form to add a row to, so a group action is a
	// hand-built request like any other and is refused before it re-renders.
	t.Run("group action", func(t *testing.T) {
		rec, flash := serveAs(t, h, sm, editor, h.HandleSnippetList, postForm(list, url.Values{
			"id": {strconv.FormatInt(sn.ID, 10)}, "key": {"kontakt"}, "name": {"Kontakt"},
			GroupAction: {"neu:zeiten"},
		}, route))
		wantRefused(t, "snippet group action", rec, flash, list)
	})

	t.Run("delete", func(t *testing.T) {
		target := fmt.Sprintf("%s/%d/delete", list, sn.ID)
		delRoute := websiteRoute(ws.ID, "snippetID", strconv.FormatInt(sn.ID, 10))
		rec, flash := serveAs(t, h, sm, editor, h.HandleSnippetDelete, postForm(target, nil, delRoute))
		wantRefused(t, "snippet delete", rec, flash, list)
		rec, flash = serveAs(t, h, sm, editor, h.HandleSnippetDelete, htmx(postForm(target, nil, delRoute)))
		wantRefusedHtmx(t, "snippet delete", rec, flash, list)
		if got, _ := h.snippets.Get(ctx, ws.ID, sn.ID); got == nil {
			t.Error("the snippet was deleted")
		}
	})

	// Another website's snippet is not this person's business: the scope
	// answers first, and it answers 404, not the sentence.
	t.Run("foreign", func(t *testing.T) {
		other, err := h.domains.CreateWebsite(ctx, "Andere", "")
		if err != nil {
			t.Fatalf("CreateWebsite: %v", err)
		}
		foreign := seedSnippet(t, h.snippets, other.ID, "fremd", "Fremd", "Nicht deins")
		rec, flash := serveAs(t, h, sm, editor, h.HandleSnippetDelete, postForm(
			fmt.Sprintf("%s/%d/delete", list, foreign.ID), nil,
			websiteRoute(ws.ID, "snippetID", strconv.FormatInt(foreign.ID, 10))))
		if rec.Code != http.StatusNotFound {
			t.Errorf("foreign snippet: status %d, want 404", rec.Code)
		}
		if flash == readOnlyText {
			t.Error("foreign snippet: answered with the read-only sentence instead of 404")
		}
	})

	// The control: somebody who may publish changes it.
	t.Run("publisher", func(t *testing.T) {
		publisher := editorWhoMayPublish(t, database, ws.ID)
		rec, flash := serveAs(t, h, sm, publisher, h.HandleSnippetList, postForm(list, url.Values{
			"id": {strconv.FormatInt(sn.ID, 10)}, "key": {"kontakt"}, "name": {"Kontakt"},
			"content_markdown": {"Umgeschrieben"},
		}, route))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("publisher: status %d (%q), want 303", rec.Code, flash)
		}
		got, _ := h.snippets.Get(ctx, ws.ID, sn.ID)
		if got == nil || got.ContentMarkdown != "Umgeschrieben" {
			t.Error("a person who may publish could not change the snippet")
		}
	})
}

func TestTheSnippetScreenOffersNoChangeWithoutPublishing(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	editor := editorWithoutPublishing(t, database, ws.ID)
	publisher := editorWhoMayPublish(t, database, ws.ID)
	sn := seedSnippet(t, h.snippets, ws.ID, "kontakt", "Kontakt", "Bei uns im Laden")
	list := snippetListAddress(ws.ID)
	route := websiteRoute(ws.ID)
	form := fmt.Sprintf(`action="%s"`, list)
	opened := fmt.Sprintf(`%s?edit=%d`, list, sn.ID)
	saveButton := `hx-disabled-elt="this">Save</button>`

	body := getAs(t, sm, putUser(sm, editor), h.HandleSnippetList, list, route)
	if n := strings.Count(body, readOnlyText); n != 1 {
		t.Errorf("snippet list: the sentence stands %d times, want once", n)
	}
	if anyDelete.MatchString(body) {
		t.Error("snippet list offers delete")
	}
	if strings.Contains(body, form) {
		t.Error("snippet list offers the form to create a snippet")
	}
	if !strings.Contains(body, opened+`" class="btn btn--sm">View</a>`) {
		t.Error("snippet list: the link to the snippet does not read View")
	}

	body = getAs(t, sm, putUser(sm, editor), h.HandleSnippetList, opened, route)
	if n := strings.Count(body, readOnlyText); n != 1 {
		t.Errorf("opened snippet: the sentence stands %d times, want once", n)
	}
	if !disabledFieldset.MatchString(body) {
		t.Error("opened snippet: the fields are not in a disabled fieldset")
	}
	if !strings.Contains(body, "Bei uns im Laden") {
		t.Error("opened snippet: its content is not shown")
	}
	if strings.Contains(body, saveButton) {
		t.Error("opened snippet: a save button is offered")
	}

	body = getAs(t, sm, putUser(sm, publisher), h.HandleSnippetList, list, route)
	if strings.Contains(body, readOnlyText) {
		t.Error("a person who may publish reads the read-only sentence")
	}
	if !strings.Contains(body, form) || !anyDelete.MatchString(body) {
		t.Error("a person who may publish is not offered create and delete")
	}
	if !strings.Contains(body, opened+`" class="btn btn--sm">Edit</a>`) {
		t.Error("a person who may publish is not offered Edit")
	}
	body = getAs(t, sm, putUser(sm, publisher), h.HandleSnippetList, opened, route)
	if !strings.Contains(body, saveButton) || disabledFieldset.MatchString(body) {
		t.Error("a person who may publish cannot change the opened snippet")
	}
}

func albumListAddress(websiteID int64) string {
	return fmt.Sprintf("/admin/websites/%d/albums", websiteID)
}

func albumAddress(websiteID, albumID int64) string {
	return fmt.Sprintf("/admin/websites/%d/albums/%d", websiteID, albumID)
}

func TestAnEditorWithoutPublishingCannotChangeAlbums(t *testing.T) {
	f := newAlbumFixture(t)
	ctx := context.Background()
	h, sm, ws, a := f.h, f.sm, f.siteA, f.albumA
	editor := editorWithoutPublishing(t, f.db, ws.ID)
	first := f.mustAdd(t, a.ID, f.mediaA, "Werkbank")
	second := f.mustAdd(t, a.ID, f.seedMedia(t, ws.ID, "a2.jpg"), "Hobel")
	list := albumListAddress(ws.ID)
	here := albumAddress(ws.ID, a.ID)
	albumRoute := websiteRoute(ws.ID, "albumID", id64(a.ID))
	itemRoute := func(item int64) map[string]string {
		return websiteRoute(ws.ID, "albumID", id64(a.ID), "itemID", id64(item))
	}
	order := func() []int64 {
		var ids []int64
		for _, p := range f.pictures(t, a.ID) {
			ids = append(ids, p.ID)
		}
		return ids
	}
	before := order()

	t.Run("create", func(t *testing.T) {
		rec, flash := serveAs(t, h, sm, editor, h.HandleAlbumCreate,
			postForm(list, url.Values{"name": {"Neues Album"}}, websiteRoute(ws.ID)))
		wantRefused(t, "album create", rec, flash, list)
		albums, err := f.store.List(ctx, ws.ID)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(albums) != 1 {
			t.Errorf("%d albums stored, want the one seeded", len(albums))
		}
	})

	t.Run("rename", func(t *testing.T) {
		rec, flash := serveAs(t, h, sm, editor, h.HandleAlbumUpdate,
			postForm(here+"/update", url.Values{"name": {"Umbenannt"}}, albumRoute))
		wantRefused(t, "album rename", rec, flash, list)
		got, _ := f.store.Get(ctx, ws.ID, a.ID)
		if got == nil || got.Name != a.Name {
			t.Errorf("the album was renamed: %+v", got)
		}
	})

	t.Run("delete", func(t *testing.T) {
		rec, flash := serveAs(t, h, sm, editor, h.HandleAlbumDelete, postForm(here+"/delete", nil, albumRoute))
		wantRefused(t, "album delete", rec, flash, list)
		rec, flash = serveAs(t, h, sm, editor, h.HandleAlbumDelete, htmx(postForm(here+"/delete", nil, albumRoute)))
		wantRefusedHtmx(t, "album delete", rec, flash, list)
		if got, _ := f.store.Get(ctx, ws.ID, a.ID); got == nil {
			t.Fatal("the album was deleted")
		}
	})

	t.Run("picture add", func(t *testing.T) {
		values := url.Values{"bild.medium": {id64(f.mediaA)}, "bild.alt": {"Noch einmal"}}
		rec, flash := serveAs(t, h, sm, editor, h.HandleAlbumItemCreate, postForm(here+"/pictures", values, albumRoute))
		wantRefused(t, "picture add", rec, flash, here)
		rec, flash = serveAs(t, h, sm, editor, h.HandleAlbumItemCreate, htmx(postForm(here+"/pictures", values, albumRoute)))
		wantRefusedHtmx(t, "picture add", rec, flash, here)
		if n := len(f.pictures(t, a.ID)); n != 2 {
			t.Errorf("%d pictures, want the two seeded", n)
		}
	})

	t.Run("picture update", func(t *testing.T) {
		target := fmt.Sprintf("%s/pictures/%d/update", here, first)
		rec, flash := serveAs(t, h, sm, editor, h.HandleAlbumItemUpdate, postForm(target, url.Values{
			"bild.medium": {id64(f.mediaA)}, "bild.alt": {"Umgeschrieben"},
		}, itemRoute(first)))
		wantRefused(t, "picture update", rec, flash, here)
		for _, p := range f.pictures(t, a.ID) {
			if p.ID == first && p.Item.Alt != "Werkbank" {
				t.Errorf("the description was changed to %q", p.Item.Alt)
			}
		}
	})

	t.Run("picture delete", func(t *testing.T) {
		target := fmt.Sprintf("%s/pictures/%d/delete", here, first)
		rec, flash := serveAs(t, h, sm, editor, h.HandleAlbumItemDelete, postForm(target, nil, itemRoute(first)))
		wantRefused(t, "picture delete", rec, flash, here)
		if n := len(f.pictures(t, a.ID)); n != 2 {
			t.Errorf("%d pictures, want the two seeded", n)
		}
	})

	t.Run("picture reorder", func(t *testing.T) {
		target := fmt.Sprintf("%s/pictures/%d/reorder?direction=up", here, second)
		rec, flash := serveAs(t, h, sm, editor, h.HandleAlbumItemReorder, postForm(target, nil, itemRoute(second)))
		wantRefused(t, "picture reorder", rec, flash, here)
		if got := order(); fmt.Sprint(got) != fmt.Sprint(before) {
			t.Errorf("the order changed from %v to %v", before, got)
		}
	})

	// Another website's album is not this person's business: the scope
	// answers first, and it answers 404, not the sentence.
	t.Run("foreign", func(t *testing.T) {
		rec, flash := serveAs(t, h, sm, editor, h.HandleAlbumDelete, postForm(
			albumAddress(ws.ID, f.albumB.ID)+"/delete", nil, websiteRoute(ws.ID, "albumID", id64(f.albumB.ID))))
		if rec.Code != http.StatusNotFound {
			t.Errorf("foreign album: status %d, want 404", rec.Code)
		}
		if flash == readOnlyText {
			t.Error("foreign album: answered with the read-only sentence instead of 404")
		}
	})

	// The control: somebody who may publish renames it.
	t.Run("publisher", func(t *testing.T) {
		publisher := editorWhoMayPublish(t, f.db, ws.ID)
		rec, flash := serveAs(t, h, sm, publisher, h.HandleAlbumUpdate,
			postForm(here+"/update", url.Values{"name": {"Umbenannt"}}, albumRoute))
		if rec.Code != http.StatusSeeOther || flash != "" {
			t.Errorf("publisher: status %d (%q), want 303", rec.Code, flash)
		}
		if got, _ := f.store.Get(ctx, ws.ID, a.ID); got == nil || got.Name != "Umbenannt" {
			t.Error("a person who may publish could not rename the album")
		}
	})
}

func TestTheAlbumScreensOfferNoChangeWithoutPublishing(t *testing.T) {
	f := newAlbumFixture(t)
	h, sm, ws, a := f.h, f.sm, f.siteA, f.albumA
	editor := editorWithoutPublishing(t, f.db, ws.ID)
	publisher := editorWhoMayPublish(t, f.db, ws.ID)
	f.mustAdd(t, a.ID, f.mediaA, "Werkbank")
	f.mustAdd(t, a.ID, f.seedMedia(t, ws.ID, "a2.jpg"), "Hobel")
	list := albumListAddress(ws.ID)
	here := albumAddress(ws.ID, a.ID)
	listRoute := websiteRoute(ws.ID)
	albumRoute := websiteRoute(ws.ID, "albumID", id64(a.ID))

	createForm := fmt.Sprintf(`action="%s"`, list)
	albumControls := map[string]*regexp.Regexp{
		"save":     regexp.MustCompile(`hx-disabled-elt="this">Save</button>`),
		"save one": regexp.MustCompile(`>Save image</button>`),
		"add":      regexp.MustCompile(`action="[^"]*/pictures"`),
		"reorder":  regexp.MustCompile(`action="[^"]*/reorder\?`),
		"delete":   anyDelete,
	}

	body := getAs(t, sm, putUser(sm, editor), h.HandleAlbumList, list, listRoute)
	if n := strings.Count(body, readOnlyText); n != 1 {
		t.Errorf("album list: the sentence stands %d times, want once", n)
	}
	if strings.Contains(body, createForm) {
		t.Error("album list offers the form to create an album")
	}
	if anyDelete.MatchString(body) {
		t.Error("album list offers delete")
	}

	body = getAs(t, sm, putUser(sm, editor), h.HandleAlbumEdit, here, albumRoute)
	if n := strings.Count(body, readOnlyText); n != 1 {
		t.Errorf("album: the sentence stands %d times, want once", n)
	}
	for name, re := range albumControls {
		if re.MatchString(body) {
			t.Errorf("album offers %s", name)
		}
	}
	// The name and both pictures, each in a disabled fieldset.
	if n := len(disabledFieldset.FindAllString(body, -1)); n < 3 {
		t.Errorf("album: %d disabled fieldsets, want the name and both pictures", n)
	}
	if !strings.Contains(body, `value="Werkbank"`) {
		t.Error("album: the description of a picture is not shown")
	}

	body = getAs(t, sm, putUser(sm, publisher), h.HandleAlbumList, list, listRoute)
	if strings.Contains(body, readOnlyText) {
		t.Error("album list: a person who may publish reads the read-only sentence")
	}
	if !strings.Contains(body, createForm) || !anyDelete.MatchString(body) {
		t.Error("album list: a person who may publish is not offered create and delete")
	}
	body = getAs(t, sm, putUser(sm, publisher), h.HandleAlbumEdit, here, albumRoute)
	if strings.Contains(body, readOnlyText) {
		t.Error("album: a person who may publish reads the read-only sentence")
	}
	for name, re := range albumControls {
		if !re.MatchString(body) {
			t.Errorf("album: a person who may publish is not offered %s", name)
		}
	}
	if disabledFieldset.MatchString(body) {
		t.Error("album: a person who may publish finds the fields disabled")
	}
}
