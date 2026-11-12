package admin

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
)

// Creating an entry refuses what editing one refuses and stores what editing
// one stores. The create path used to run checks of its own, which took any
// page id and any parent id — a page of another website, a draft, an entry
// of another website's menu — and hung them in this menu.
func TestCreatingAMenuEntryIsCheckedLikeEditingOne(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	store := menu.NewStore(database)

	m, err := store.CreateMenu(ctx, ws.ID, "Hauptmenü", "haupt", "")
	if err != nil {
		t.Fatal(err)
	}
	top, err := store.CreateItem(ctx, m.ID, nil, "Oben", "custom", "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	pg := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "x", "published")
	draft := seedPage(t, database, ws.ID, "Entwurf", "entwurf", "x", "draft")

	siteB, err := domain.NewStore(database).CreateWebsite(ctx, "Fremd", "")
	if err != nil {
		t.Fatal(err)
	}
	foreignPage := seedPage(t, database, siteB.ID, "Fremde Seite", "fremd", "x", "published")
	foreignMenu, _ := store.CreateMenu(ctx, siteB.ID, "Fremdmenü", "haupt", "")
	foreignItem, _ := store.CreateItem(ctx, foreignMenu.ID, nil, "Fremd", "custom", "", nil, 0)

	menuURL := "/admin/websites/" + id(ws.ID) + "/menus/" + id(m.ID)
	post := func(v url.Values) (int, string, string, string) {
		rec, bad, good := albumFlash(t, h, sm, h.HandleMenuItemCreate,
			postForm("/x", v, websiteRoute(ws.ID, "menuID", id(m.ID))))
		return rec.Code, rec.Header().Get("Location"), bad, good
	}
	items := func() []menu.MenuItem {
		list, err := store.ListItems(ctx, m.ID)
		if err != nil {
			t.Fatalf("ListItems: %v", err)
		}
		return list
	}
	newest := func(before []menu.MenuItem) menu.MenuItem {
		after := items()
		if len(after) != len(before)+1 {
			t.Fatalf("%d entries after the create, want %d", len(after), len(before)+1)
		}
		known := map[int64]bool{}
		for _, it := range before {
			known[it.ID] = true
		}
		for _, it := range after {
			if !known[it.ID] {
				return it
			}
		}
		t.Fatal("no new entry found")
		return menu.MenuItem{}
	}

	refused := []struct {
		name string
		v    url.Values
	}{
		{"blank title", url.Values{"title": {"  "}, "item_type": {"custom"}}},
		{"unknown kind", url.Values{"title": {"X"}, "item_type": {"evil"}}},
		{"page without a page", url.Values{"title": {"X"}, "item_type": {"page"}}},
		{"draft page", url.Values{"title": {"X"}, "item_type": {"page"}, "page_id": {id(draft.ID)}}},
		{"page of another website", url.Values{"title": {"X"}, "item_type": {"page"}, "page_id": {id(foreignPage.ID)}}},
		{"url without an address", url.Values{"title": {"X"}, "item_type": {"url"}, "url": {" "}}},
		{"parent in another website's menu", url.Values{"title": {"X"}, "item_type": {"custom"}, "parent_id": {id(foreignItem.ID)}}},
		{"parent that is not a number", url.Values{"title": {"X"}, "item_type": {"custom"}, "parent_id": {"oben"}}},
	}
	for _, c := range refused {
		t.Run("refused/"+c.name, func(t *testing.T) {
			before := len(items())
			code, loc, bad, good := post(c.v)
			if code != http.StatusSeeOther || loc != menuURL {
				t.Errorf("status %d to %q, want 303 to %q", code, loc, menuURL)
			}
			if bad == "" || good != "" {
				t.Errorf("flash error %q, success %q", bad, good)
			}
			if n := len(items()); n != before {
				t.Errorf("%d entries after a refused create, want %d", n, before)
			}
		})
	}

	t.Run("url is stored verbatim", func(t *testing.T) {
		before := items()
		if code, _, bad, _ := post(url.Values{"title": {"Extern"}, "item_type": {"url"}, "url": {"https://example.org/"}}); code != http.StatusSeeOther || bad != "" {
			t.Fatalf("status %d, error %q", code, bad)
		}
		got := newest(before)
		if got.URL != "https://example.org/" || got.PageID != nil {
			t.Errorf("stored %+v", got)
		}
	})

	t.Run("page drops a posted address", func(t *testing.T) {
		before := items()
		post(url.Values{"title": {"Über"}, "item_type": {"page"}, "page_id": {id(pg.ID)}, "url": {"https://ignored"}})
		got := newest(before)
		if got.PageID == nil || *got.PageID != pg.ID || got.URL != "" {
			t.Errorf("stored %+v", got)
		}
	})

	t.Run("custom drops a posted address", func(t *testing.T) {
		before := items()
		post(url.Values{"title": {"Text"}, "item_type": {"custom"}, "url": {"https://ignored"}})
		got := newest(before)
		if got.URL != "" || got.PageID != nil {
			t.Errorf("stored %+v", got)
		}
	})

	t.Run("below a parent at the end of its group", func(t *testing.T) {
		if _, err := store.CreateItem(ctx, m.ID, &top.ID, "Erstes Kind", "custom", "", nil, 0); err != nil {
			t.Fatal(err)
		}
		before := items()
		post(url.Values{"title": {"Zweites Kind"}, "item_type": {"custom"}, "parent_id": {id(top.ID)}})
		got := newest(before)
		if got.ParentID == nil || *got.ParentID != top.ID || got.SortOrder != 1 {
			t.Errorf("stored %+v, want parent %d and sort order 1", got, top.ID)
		}
	})

	// The two paths treat an address without a scheme the same way. This
	// asserts equality, not a policy: what that address should be is open.
	t.Run("an address without a scheme is treated alike", func(t *testing.T) {
		v := url.Values{"title": {"Verband"}, "item_type": {"url"}, "url": {"holzverband.example"}}
		before := items()
		post(v)
		created := newest(before)

		existing, err := store.CreateItem(ctx, m.ID, nil, "Alt", "url", "https://example.org/alt", nil, 9)
		if err != nil {
			t.Fatal(err)
		}
		albumFlash(t, h, sm, h.HandleMenuItemUpdate,
			postForm("/x", v, websiteRoute(ws.ID, "menuID", id(m.ID), "itemID", id(existing.ID))))
		updated, err := store.GetItem(ctx, existing.ID)
		if err != nil || updated == nil {
			t.Fatalf("GetItem: %v", err)
		}
		if created.URL != updated.URL {
			t.Errorf("create stored %q, update stored %q", created.URL, updated.URL)
		}
	})
}
