package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
)

func id(n int64) string { return strconv.FormatInt(n, 10) }

// Editing an entry: every field, the refusals, the tree rules and the log.
func TestEditingAMenuEntry(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	h.SetActivityStore(activity.NewStore(database))
	ctx := context.Background()
	store := menu.NewStore(database)

	m, err := store.CreateMenu(ctx, ws.ID, "Hauptmenü", "haupt", "")
	if err != nil {
		t.Fatal(err)
	}
	top, _ := store.CreateItem(ctx, m.ID, nil, "Oben", "url", "https://example.org/", nil, 0)
	child, _ := store.CreateItem(ctx, m.ID, &top.ID, "Kind", "custom", "", nil, 0)
	grand, _ := store.CreateItem(ctx, m.ID, &child.ID, "Enkel", "custom", "", nil, 0)
	other, _ := store.CreateItem(ctx, m.ID, nil, "Anderer", "custom", "", nil, 1)
	pg := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "x", "published")
	draft := seedPage(t, database, ws.ID, "Entwurf", "entwurf", "x", "draft")

	siteB, err := domain.NewStore(database).CreateWebsite(ctx, "Fremd", "")
	if err != nil {
		t.Fatal(err)
	}
	foreignPage := seedPage(t, database, siteB.ID, "Fremde Seite", "fremd", "x", "published")
	foreignMenu, _ := store.CreateMenu(ctx, siteB.ID, "Fremdmenü", "haupt", "")
	foreignItem, _ := store.CreateItem(ctx, foreignMenu.ID, nil, "Fremd", "custom", "", nil, 0)

	editURL := "/admin/websites/" + id(ws.ID) + "/menus/" + id(m.ID) + "/items/"
	post := func(itemID int64, v url.Values) (*httptest.ResponseRecorder, string, string) {
		route := websiteRoute(ws.ID, "menuID", id(m.ID), "itemID", id(itemID))
		return albumFlash(t, h, sm, h.HandleMenuItemUpdate, postForm("/x", v, route))
	}
	logged := func() int { return len(menuActivity(t, database, activity.ActionMenuItemUpdate)) }
	get := func(itemID int64) *menu.MenuItem {
		it, err := store.GetItem(ctx, itemID)
		if err != nil || it == nil {
			t.Fatalf("GetItem %d: %v", itemID, err)
		}
		return it
	}

	// Refusals: nothing changes, nothing is logged, the person lands on the form.
	refused := []struct {
		name string
		v    url.Values
	}{
		{"empty title", url.Values{"title": {"  "}, "item_type": {"custom"}}},
		{"bad kind", url.Values{"title": {"X"}, "item_type": {"evil"}}},
		{"page without page", url.Values{"title": {"X"}, "item_type": {"page"}}},
		{"url without url", url.Values{"title": {"X"}, "item_type": {"url"}, "url": {" "}}},
		{"draft page", url.Values{"title": {"X"}, "item_type": {"page"}, "page_id": {id(draft.ID)}}},
		{"foreign page", url.Values{"title": {"X"}, "item_type": {"page"}, "page_id": {id(foreignPage.ID)}}},
		{"own parent", url.Values{"title": {"X"}, "item_type": {"custom"}, "parent_id": {id(other.ID)}}},
		{"grandchild as parent", url.Values{"title": {"X"}, "item_type": {"custom"}, "parent_id": {id(grand.ID)}}},
		{"parent in another menu", url.Values{"title": {"X"}, "item_type": {"custom"}, "parent_id": {id(foreignItem.ID)}}},
	}
	for _, c := range refused {
		target := other.ID
		if c.name == "grandchild as parent" {
			target = top.ID
		}
		before := get(target)
		rec, bad, good := post(target, c.v)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != editURL+id(target)+"/edit" {
			t.Errorf("%s: status %d, location %q", c.name, rec.Code, rec.Header().Get("Location"))
		}
		if bad == "" || good != "" {
			t.Errorf("%s: flash error %q success %q", c.name, bad, good)
		}
		after := get(target)
		if after.Title != before.Title || after.ItemType != before.ItemType || after.URL != before.URL ||
			(after.ParentID == nil) != (before.ParentID == nil) {
			t.Errorf("%s: the entry changed: %+v -> %+v", c.name, before, after)
		}
		if n := logged(); n != 0 {
			t.Errorf("%s: %d log entries", c.name, n)
		}
	}
	if got := get(top.ID); got.ParentID != nil {
		t.Errorf("a cycle was stored: %+v", got)
	}

	// Success: custom -> page below Oben, goes to the end of Oben's children.
	rec, bad, good := post(other.ID, url.Values{
		"title": {"  Neu "}, "item_type": {"page"}, "page_id": {id(pg.ID)}, "url": {"ignored"}, "parent_id": {id(top.ID)},
	})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/websites/"+id(ws.ID)+"/menus/"+id(m.ID) {
		t.Fatalf("status %d, location %q", rec.Code, rec.Header().Get("Location"))
	}
	if bad != "" || good == "" {
		t.Errorf("flash error %q success %q", bad, good)
	}
	got := get(other.ID)
	if got.Title != "Neu" || got.ItemType != "page" || got.URL != "" || got.PageID == nil || *got.PageID != pg.ID ||
		got.ParentID == nil || *got.ParentID != top.ID || got.SortOrder != 1 {
		t.Errorf("stored %+v", got)
	}
	if n := logged(); n != 1 {
		t.Errorf("%d log entries after the edit, want 1", n)
	}

	// Kind url clears the page; an unchanged parent keeps the place.
	post(other.ID, url.Values{"title": {"Neu"}, "item_type": {"url"}, "url": {"https://example.org/x"}, "page_id": {id(pg.ID)}, "parent_id": {id(top.ID)}})
	got = get(other.ID)
	if got.PageID != nil || got.URL != "https://example.org/x" || got.SortOrder != 1 {
		t.Errorf("url kind stored %+v", got)
	}
	// Kind custom clears the address; empty parent moves to the top level.
	post(other.ID, url.Values{"title": {"Neu"}, "item_type": {"custom"}, "url": {"https://x"}})
	got = get(other.ID)
	if got.URL != "" || got.PageID != nil || got.ParentID != nil || got.SortOrder != 1 {
		t.Errorf("custom kind stored %+v", got)
	}

	// An htmx post answers with HX-Redirect.
	req := postForm("/x", url.Values{"title": {"Neu"}, "item_type": {"custom"}}, websiteRoute(ws.ID, "menuID", id(m.ID), "itemID", id(other.ID)))
	req.Header.Set("HX-Request", "true")
	rec = serve(t, h, sm, h.HandleMenuItemUpdate, req)
	if rec.Header().Get("HX-Redirect") == "" {
		t.Error("no HX-Redirect on an htmx post")
	}

	// Foreign menu, foreign item: 404, nothing touched.
	before := len(menuActivity(t, database, activity.ActionMenuItemUpdate))
	for _, route := range []map[string]string{
		websiteRoute(ws.ID, "menuID", id(foreignMenu.ID), "itemID", id(foreignItem.ID)),
		websiteRoute(ws.ID, "menuID", id(m.ID), "itemID", id(foreignItem.ID)),
	} {
		rec := serve(t, h, sm, h.HandleMenuItemUpdate, postForm("/x", url.Values{"title": {"Gehackt"}, "item_type": {"custom"}}, route))
		if rec.Code != http.StatusNotFound {
			t.Errorf("foreign update: status %d", rec.Code)
		}
	}
	if fi := get(foreignItem.ID); fi.Title != "Fremd" {
		t.Errorf("foreign entry changed: %+v", fi)
	}
	if n := len(menuActivity(t, database, activity.ActionMenuItemUpdate)); n != before {
		t.Errorf("foreign update logged")
	}
}

func TestMenuEntryCycleHelpers(t *testing.T) {
	one, two, three := int64(1), int64(2), int64(3)
	items := []menu.MenuItem{
		{ID: 1}, {ID: 2, ParentID: &one}, {ID: 3, ParentID: &two}, {ID: 4},
	}
	if !parentCreatesCycle(items, 1, 1) || !parentCreatesCycle(items, 1, 3) || !parentCreatesCycle(items, 2, 3) {
		t.Error("a cycle was not detected")
	}
	if parentCreatesCycle(items, 3, 1) || parentCreatesCycle(items, 1, 4) {
		t.Error("a valid parent was refused")
	}
	// A stored loop must not hang the walk.
	loop := []menu.MenuItem{{ID: 1, ParentID: &three}, {ID: 2, ParentID: &one}, {ID: 3, ParentID: &two}}
	if !parentCreatesCycle(loop, 1, 2) {
		t.Error("loop not handled")
	}
}

func TestTheMenuEntryFormListsOnlyValidParents(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	store := menu.NewStore(database)
	m, _ := store.CreateMenu(ctx, ws.ID, "Hauptmenü", "haupt", "")
	top, _ := store.CreateItem(ctx, m.ID, nil, "Oben", "custom", "", nil, 0)
	child, _ := store.CreateItem(ctx, m.ID, &top.ID, "Kindeintrag", "custom", "", nil, 0)
	store.CreateItem(ctx, m.ID, &child.ID, "Enkeleintrag", "custom", "", nil, 0)
	store.CreateItem(ctx, m.ID, nil, "Daneben", "custom", "", nil, 1)

	do := func(itemID int64) string {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		for k, v := range websiteRoute(ws.ID, "menuID", id(m.ID), "itemID", id(itemID)) {
			req.SetPathValue(k, v)
		}
		rec := serve(t, h, sm, h.HandleMenuItemEditForm, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		return rec.Body.String()
	}
	body := do(child.ID)
	action := `action="/admin/websites/` + id(ws.ID) + `/menus/` + id(m.ID) + `/items/` + id(child.ID) + `/update"`
	if !strings.Contains(body, action) {
		t.Error("no form posting to the update address")
	}
	if !strings.Contains(body, `value="Kindeintrag"`) {
		t.Error("the title is not prefilled")
	}
	parents := body[strings.Index(body, `id="item-parent"`):]
	for _, want := range []string{">Oben<", ">Daneben<"} {
		if !strings.Contains(parents, want) {
			t.Errorf("parent option %s missing", want)
		}
	}
	for _, not := range []string{"Kindeintrag<", "Enkeleintrag<"} {
		if strings.Contains(parents, not) {
			t.Errorf("parent options offer %s", not)
		}
	}
	if !strings.Contains(parents, `value="`+id(top.ID)+`" selected`) {
		t.Error("the current parent is not selected")
	}

	// Foreign ids are 404.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	for k, v := range websiteRoute(ws.ID+99, "menuID", id(m.ID), "itemID", id(child.ID)) {
		req.SetPathValue(k, v)
	}
	if rec := serve(t, h, sm, h.HandleMenuItemEditForm, req); rec.Code != http.StatusNotFound {
		t.Errorf("foreign website: status %d", rec.Code)
	}
}
