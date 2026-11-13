package admin

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// Without the right to publish, nobody changes what the public sees at once.
//
// 0.1.4 kept that promise for published pages and nowhere else: a menu, a
// label or a redirect is online the moment it is stored, and every one of
// those doors stood open to an editor the administrator had limited. Phase 31
// closes them with one rule — refuse, say whom to ask, never a 403 — and these
// tests hold each door with a hand-built request, because a screen that hides
// a button is a courtesy and the handler refusing is the border. Each asserts
// what is stored afterwards and that the refusal was not silent.

// readOnlyText is the refusal such an editor reads. Tests run in English, so
// the rendered sentence is the catalogue key.
const readOnlyText = "What is changed here is online at once. Changing it is not part of your account – ask somebody who may publish."

// htmx marks a request the way htmx sends it.
func htmx(req *http.Request) *http.Request {
	req.Header.Set("HX-Request", "true")
	return req
}

// wantRefused is the refusal of a plain form post: back to the area's own
// screen with the sentence, and never a 403.
func wantRefused(t *testing.T, what string, rec *httptest.ResponseRecorder, flash, to string) {
	t.Helper()
	if rec.Code == http.StatusForbidden {
		t.Errorf("%s: 403 — the route is open to editors and must answer with a sentence", what)
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("%s: status %d, want 303", what, rec.Code)
	}
	if got := rec.Header().Get("Location"); got != to {
		t.Errorf("%s: Location %q, want %q", what, got, to)
	}
	if flash != readOnlyText {
		t.Errorf("%s: flash %q, want the read-only sentence", what, flash)
	}
	if !strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "HX-Request") {
		t.Errorf("%s: Vary %q does not name HX-Request", what, rec.Header().Values("Vary"))
	}
}

// wantRefusedHtmx is the same refusal for htmx: a 200 that tells htmx where to
// go, because a 303 would be swapped into the page instead of followed.
func wantRefusedHtmx(t *testing.T, what string, rec *httptest.ResponseRecorder, flash, to string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Errorf("%s (htmx): status %d, want 200", what, rec.Code)
	}
	if got := rec.Header().Get("HX-Redirect"); got != to {
		t.Errorf("%s (htmx): HX-Redirect %q, want %q", what, got, to)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("%s (htmx): Location %q, want none", what, got)
	}
	if flash != readOnlyText {
		t.Errorf("%s (htmx): flash %q, want the read-only sentence", what, flash)
	}
	if !strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "HX-Request") {
		t.Errorf("%s (htmx): Vary %q does not name HX-Request", what, rec.Header().Values("Vary"))
	}
}

// editorWhoMayPublish is the control: the same kind of account with the right.
func editorWhoMayPublish(t *testing.T, database *db.DB, websiteID int64) int64 {
	t.Helper()
	ctx := context.Background()
	users := user.NewStore(database, cheapHashing)
	id, err := users.Create(ctx, "Verlegerin", "pub@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}
	if err := users.SetRights(ctx, id, user.Rights{MayPublish: true, Websites: []int64{websiteID}}); err != nil {
		t.Fatalf("SetRights: %v", err)
	}
	return id
}

// putUser puts only the user id in the session, the way serveAs does.
func putUser(sm *scs.SessionManager, id int64) func(r *http.Request) {
	return func(r *http.Request) { sm.Put(r.Context(), auth.SessionKeyUserID, id) }
}

// getAs renders a screen for this person and hands back the body.
func getAs(t *testing.T, sm *scs.SessionManager, put func(r *http.Request),
	fn func(http.ResponseWriter, *http.Request) error, target string, pathValues map[string]string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	rec := httptest.NewRecorder()
	var handlerErr error
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		put(r)
		handlerErr = fn(w, r)
	})).ServeHTTP(rec, req)
	if handlerErr != nil {
		t.Fatalf("handler: %v", handlerErr)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d", target, rec.Code)
	}
	return rec.Body.String()
}

func menuListAddress(websiteID int64) string {
	return fmt.Sprintf("/admin/websites/%d/menus", websiteID)
}

func menuStillThere(t *testing.T, menus *menu.Store, id int64) bool {
	t.Helper()
	m, err := menus.GetMenu(context.Background(), id)
	if err != nil {
		t.Fatalf("GetMenu: %v", err)
	}
	return m != nil
}

func TestAWriteWithoutThePublishingRightIsRefusedWithASentence(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	editor := editorWithoutPublishing(t, database, ws.ID)
	m, _, _ := seedMenu(t, h.menuStore, ws.ID, "Hauptmenü", "haupt")
	route := websiteRoute(ws.ID, "menuID", strconv.FormatInt(m.ID, 10))
	target := fmt.Sprintf("/admin/websites/%d/menus/%d/delete", ws.ID, m.ID)

	rec, flash := serveAs(t, h, sm, editor, h.HandleMenuDelete, postForm(target, nil, route))
	wantRefused(t, "menu delete", rec, flash, menuListAddress(ws.ID))
	if !menuStillThere(t, h.menuStore, m.ID) {
		t.Fatal("the menu was deleted")
	}

	rec, flash = serveAs(t, h, sm, editor, h.HandleMenuDelete, htmx(postForm(target, nil, route)))
	wantRefusedHtmx(t, "menu delete", rec, flash, menuListAddress(ws.ID))
	if !menuStillThere(t, h.menuStore, m.ID) {
		t.Fatal("the menu was deleted through htmx")
	}

	// Another website's menu is not this person's business at all: the scope
	// answers first, and it answers 404, not the sentence.
	other, err := h.domains.CreateWebsite(context.Background(), "Andere", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	foreign, _, _ := seedMenu(t, h.menuStore, other.ID, "Fremd", "fremd")
	rec, flash = serveAs(t, h, sm, editor, h.HandleMenuDelete,
		postForm(target, nil, websiteRoute(ws.ID, "menuID", strconv.FormatInt(foreign.ID, 10))))
	if rec.Code != http.StatusNotFound {
		t.Errorf("foreign menu: status %d, want 404", rec.Code)
	}
	if flash == readOnlyText {
		t.Error("foreign menu: answered with the read-only sentence instead of 404")
	}

	// The control: somebody who may publish deletes it.
	publisher := editorWhoMayPublish(t, database, ws.ID)
	rec, flash = serveAs(t, h, sm, publisher, h.HandleMenuDelete, postForm(target, nil, route))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("publisher: status %d (%q), want 303", rec.Code, flash)
	}
	if menuStillThere(t, h.menuStore, m.ID) {
		t.Error("a person who may publish could not delete the menu")
	}
}

// A rights lookup that fails used to count as every right there is: rightsOf
// answered Everything() on any error, which is right for deciding what a list
// shows and wrong for deciding whether a write happens (D-08).
func TestTheRightsLookupFailsClosed(t *testing.T) {
	const nobody = int64(424242)

	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	h, sm, database, ws := newTestAdmin(t)
	m, _, _ := seedMenu(t, h.menuStore, ws.ID, "Hauptmenü", "haupt")
	route := websiteRoute(ws.ID, "menuID", strconv.FormatInt(m.ID, 10))
	target := fmt.Sprintf("/admin/websites/%d/menus/%d/delete", ws.ID, m.ID)

	t.Run("an area guard refuses", func(t *testing.T) {
		rec, flash := serveAs(t, h, sm, nobody, h.HandleMenuDelete, postForm(target, nil, route))
		wantRefused(t, "menu delete with a failing lookup", rec, flash, menuListAddress(ws.ID))
		if !menuStillThere(t, h.menuStore, m.ID) {
			t.Fatal("the menu was deleted although the rights could not be read")
		}
		out := logged.String()
		if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "rights lookup failed") {
			t.Errorf("no ERROR line for the failed lookup; log:\n%s", out)
		}
	})

	t.Run("a published page stays locked", func(t *testing.T) {
		p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "der alte text", "published")
		rec, _ := serveAs(t, h, sm, nobody, h.HandlePageEdit,
			postForm("/admin/websites/1/pages/1/edit", url.Values{
				"title":            {"Neuer Titel"},
				"slug":             {"neue-adresse"},
				"content_markdown": {"Ein neuer Absatz."},
				"status":           {"published"},
				"version":          {strconv.FormatInt(p.Version, 10)},
			}, pageRoute(ws.ID, p.ID)))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status %d, want 422", rec.Code)
		}
		after, err := h.pages.GetPage(context.Background(), p.ID)
		if err != nil || after == nil {
			t.Fatalf("GetPage: %v", err)
		}
		if after.Title != p.Title {
			t.Errorf("the live page was changed with a failing lookup: title %q", after.Title)
		}
	})

	t.Run("an administrator is not affected", func(t *testing.T) {
		// A menu of its own, so this does not depend on what the first
		// subtest left behind.
		mine, _, _ := seedMenu(t, h.menuStore, ws.ID, "Fussmenü", "fuss")
		rec := serveWithRole(t, h, sm, nobody, user.RoleAdmin, h.HandleMenuDelete,
			postForm(fmt.Sprintf("/admin/websites/%d/menus/%d/delete", ws.ID, mine.ID), nil, nil),
			websiteRoute(ws.ID, "menuID", strconv.FormatInt(mine.ID, 10)))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("administrator: status %d, want 303", rec.Code)
		}
		if menuStillThere(t, h.menuStore, mine.ID) {
			t.Error("an administrator could not delete the menu when the lookup failed")
		}
	})
}

func TestTheMenuListOffersNoChangeWithoutPublishing(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	editor := editorWithoutPublishing(t, database, ws.ID)
	publisher := editorWhoMayPublish(t, database, ws.ID)
	m, _, _ := seedMenu(t, h.menuStore, ws.ID, "Hauptmenü", "haupt")
	route := websiteRoute(ws.ID)

	deleteAction := fmt.Sprintf("/menus/%d/delete", m.ID)
	createAction := fmt.Sprintf(`action="/admin/websites/%d/menus"`, ws.ID)

	body := getAs(t, sm, putUser(sm, editor), h.HandleMenuList, menuListAddress(ws.ID), route)
	if strings.Count(body, readOnlyText) != 1 {
		t.Errorf("the sentence stands %d times on the menu list, want once", strings.Count(body, readOnlyText))
	}
	if strings.Contains(body, deleteAction) {
		t.Error("the menu list offers delete")
	}
	if strings.Contains(body, createAction) {
		t.Error("the menu list offers create")
	}
	if !strings.Contains(body, "Hauptmenü") {
		t.Error("the menu's name is no longer shown")
	}

	body = getAs(t, sm, putUser(sm, publisher), h.HandleMenuList, menuListAddress(ws.ID), route)
	if strings.Contains(body, readOnlyText) {
		t.Error("a person who may publish reads the read-only sentence")
	}
	if !strings.Contains(body, deleteAction) || !strings.Contains(body, createAction) {
		t.Error("a person who may publish is not offered create and delete")
	}
}

func menuEditorAddress(websiteID, menuID int64) string {
	return fmt.Sprintf("/admin/websites/%d/menus/%d", websiteID, menuID)
}

func TestAnEditorWithoutPublishingCannotChangeMenus(t *testing.T) {
	type fixture struct {
		h      *Handler
		sm     *scs.SessionManager
		ws     int64
		editor int64
		m      *menu.Menu
		first  *menu.MenuItem
		second *menu.MenuItem
	}
	setup := func(t *testing.T) fixture {
		h, sm, database, ws := newTestAdmin(t)
		editor := editorWithoutPublishing(t, database, ws.ID)
		m, first, second := seedMenu(t, h.menuStore, ws.ID, "Hauptmenü", "haupt")
		return fixture{h, sm, ws.ID, editor, m, first, second}
	}
	itemRoute := func(f fixture, item *menu.MenuItem) map[string]string {
		return websiteRoute(f.ws, "menuID", strconv.FormatInt(f.m.ID, 10), "itemID", strconv.FormatInt(item.ID, 10))
	}
	items := func(t *testing.T, f fixture) []menu.MenuItem {
		t.Helper()
		got, err := f.h.menuStore.ListItems(context.Background(), f.m.ID)
		if err != nil {
			t.Fatalf("ListItems: %v", err)
		}
		return got
	}

	t.Run("create", func(t *testing.T) {
		f := setup(t)
		form := url.Values{"name": {"Fussmenü"}, "location_key": {"fuss"}}
		target := menuListAddress(f.ws)
		rec, flash := serveAs(t, f.h, f.sm, f.editor, f.h.HandleMenuCreate, postForm(target, form, websiteRoute(f.ws)))
		wantRefused(t, "menu create", rec, flash, menuListAddress(f.ws))
		rec, flash = serveAs(t, f.h, f.sm, f.editor, f.h.HandleMenuCreate, htmx(postForm(target, form, websiteRoute(f.ws))))
		wantRefusedHtmx(t, "menu create", rec, flash, menuListAddress(f.ws))
		menus, err := f.h.menuStore.ListMenus(context.Background(), f.ws)
		if err != nil {
			t.Fatalf("ListMenus: %v", err)
		}
		if len(menus) != 1 {
			t.Errorf("%d menus stored, want the one seeded", len(menus))
		}
	})

	t.Run("update", func(t *testing.T) {
		f := setup(t)
		rec, flash := serveAs(t, f.h, f.sm, f.editor, f.h.HandleMenuUpdate,
			postForm(menuEditorAddress(f.ws, f.m.ID)+"/update",
				url.Values{"name": {"Umbenannt"}, "location_key": {"anders"}},
				websiteRoute(f.ws, "menuID", strconv.FormatInt(f.m.ID, 10))))
		wantRefused(t, "menu update", rec, flash, menuListAddress(f.ws))
		after, err := f.h.menuStore.GetMenu(context.Background(), f.m.ID)
		if err != nil || after == nil {
			t.Fatalf("GetMenu: %v", err)
		}
		if after.Name != f.m.Name || after.LocationKey != f.m.LocationKey {
			t.Errorf("the menu was changed: %q %q", after.Name, after.LocationKey)
		}
	})

	t.Run("item create", func(t *testing.T) {
		f := setup(t)
		rec, flash := serveAs(t, f.h, f.sm, f.editor, f.h.HandleMenuItemCreate,
			postForm(menuEditorAddress(f.ws, f.m.ID)+"/items",
				url.Values{"title": {"Neu"}, "item_type": {"url"}, "url": {"https://example.org/neu"}},
				websiteRoute(f.ws, "menuID", strconv.FormatInt(f.m.ID, 10))))
		wantRefused(t, "entry create", rec, flash, menuEditorAddress(f.ws, f.m.ID))
		if n := len(items(t, f)); n != 2 {
			t.Errorf("%d entries, want the two seeded", n)
		}
	})

	t.Run("item update", func(t *testing.T) {
		f := setup(t)
		rec, flash := serveAs(t, f.h, f.sm, f.editor, f.h.HandleMenuItemUpdate,
			postForm(fmt.Sprintf("%s/items/%d/update", menuEditorAddress(f.ws, f.m.ID), f.first.ID),
				url.Values{"title": {"Umbenannt"}, "item_type": {"url"}, "url": {f.first.URL}},
				itemRoute(f, f.first)))
		wantRefused(t, "entry update", rec, flash, menuEditorAddress(f.ws, f.m.ID))
		after, err := f.h.menuStore.GetItem(context.Background(), f.first.ID)
		if err != nil || after == nil {
			t.Fatalf("GetItem: %v", err)
		}
		if after.Title != f.first.Title {
			t.Errorf("the entry was renamed to %q", after.Title)
		}
	})

	t.Run("item delete", func(t *testing.T) {
		f := setup(t)
		target := fmt.Sprintf("%s/items/%d/delete", menuEditorAddress(f.ws, f.m.ID), f.first.ID)
		rec, flash := serveAs(t, f.h, f.sm, f.editor, f.h.HandleMenuItemDelete, postForm(target, nil, itemRoute(f, f.first)))
		wantRefused(t, "entry delete", rec, flash, menuEditorAddress(f.ws, f.m.ID))
		rec, flash = serveAs(t, f.h, f.sm, f.editor, f.h.HandleMenuItemDelete, htmx(postForm(target, nil, itemRoute(f, f.first))))
		wantRefusedHtmx(t, "entry delete", rec, flash, menuEditorAddress(f.ws, f.m.ID))
		if n := len(items(t, f)); n != 2 {
			t.Errorf("%d entries, want the two seeded", n)
		}
	})

	t.Run("item reorder", func(t *testing.T) {
		f := setup(t)
		rec, flash := serveAs(t, f.h, f.sm, f.editor, f.h.HandleMenuItemReorder,
			postForm(fmt.Sprintf("%s/items/%d/reorder?direction=up", menuEditorAddress(f.ws, f.m.ID), f.second.ID),
				nil, itemRoute(f, f.second)))
		wantRefused(t, "entry reorder", rec, flash, menuEditorAddress(f.ws, f.m.ID))
		for _, it := range items(t, f) {
			want := f.first.SortOrder
			if it.ID == f.second.ID {
				want = f.second.SortOrder
			}
			if it.SortOrder != want {
				t.Errorf("entry %q moved to %d, want %d", it.Title, it.SortOrder, want)
			}
		}
	})
}

// disabledFieldset is a fieldset carrying the disabled attribute; a bare
// substring would match hx-disabled-elt on every button.
var disabledFieldset = regexp.MustCompile(`<fieldset[^>]*\sdisabled[\s>=]`)

func TestTheMenuEditorOffersNoChangeWithoutPublishing(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	editor := editorWithoutPublishing(t, database, ws.ID)
	publisher := editorWhoMayPublish(t, database, ws.ID)
	m, first, _ := seedMenu(t, h.menuStore, ws.ID, "Hauptmenü", "haupt")
	menuRoute := websiteRoute(ws.ID, "menuID", strconv.FormatInt(m.ID, 10))
	itemRoute := websiteRoute(ws.ID, "menuID", strconv.FormatInt(m.ID, 10), "itemID", strconv.FormatInt(first.ID, 10))
	itemAddress := fmt.Sprintf("%s/items/%d/edit", menuEditorAddress(ws.ID, m.ID), first.ID)
	saveButton := `hx-disabled-elt="this">Save</button>`

	controls := map[string]string{
		"delete":  `/delete"`,
		"add":     `/items"`,
		"reorder": `/reorder`,
		"save":    saveButton,
	}

	body := getAs(t, sm, putUser(sm, editor), h.HandleMenuEdit, menuEditorAddress(ws.ID, m.ID), menuRoute)
	if strings.Count(body, readOnlyText) != 1 {
		t.Errorf("menu editor: the sentence stands %d times, want once", strings.Count(body, readOnlyText))
	}
	for name, mark := range controls {
		if strings.Contains(body, mark) {
			t.Errorf("menu editor offers %s (%s)", name, mark)
		}
	}
	if !strings.Contains(body, itemAddress) {
		t.Error("menu editor: the entry no longer opens")
	}

	body = getAs(t, sm, putUser(sm, editor), h.HandleMenuItemEditForm, itemAddress, itemRoute)
	if strings.Count(body, readOnlyText) != 1 {
		t.Errorf("entry: the sentence stands %d times, want once", strings.Count(body, readOnlyText))
	}
	if !disabledFieldset.MatchString(body) {
		t.Error("entry: the fields are not in a disabled fieldset")
	}
	if strings.Contains(body, saveButton) {
		t.Error("entry: a save button is offered")
	}
	if !strings.Contains(body, first.Title) {
		t.Error("entry: its values are not shown")
	}

	body = getAs(t, sm, putUser(sm, publisher), h.HandleMenuEdit, menuEditorAddress(ws.ID, m.ID), menuRoute)
	if strings.Contains(body, readOnlyText) {
		t.Error("menu editor: a person who may publish reads the read-only sentence")
	}
	for name, mark := range controls {
		if !strings.Contains(body, mark) {
			t.Errorf("menu editor: a person who may publish is not offered %s", name)
		}
	}
	body = getAs(t, sm, putUser(sm, publisher), h.HandleMenuItemEditForm, itemAddress, itemRoute)
	if strings.Contains(body, readOnlyText) || disabledFieldset.MatchString(body) || !strings.Contains(body, saveButton) {
		t.Error("entry: a person who may publish cannot save it")
	}
}

// labelsAndRedirects is a website with one label in use, one redirect and one
// internal link that leads nowhere.
type labelsAndRedirects struct {
	h        *Handler
	sm       *scs.SessionManager
	database *db.DB
	ws       int64
	editor   int64
	label    int64
	redirect int64
}

func seedLabelsAndRedirects(t *testing.T) labelsAndRedirects {
	t.Helper()
	ctx := context.Background()
	h, sm, database, ws := newTestAdmin(t)
	editor := editorWithoutPublishing(t, database, ws.ID)
	p := seedPage(t, database, ws.ID, "Start", "start", "Siehe [alt](/alte-seite).", "published")
	if err := h.terms.SetForPage(ctx, ws.ID, p.ID, []string{"Möbel"}); err != nil {
		t.Fatalf("SetForPage: %v", err)
	}
	labels, err := h.terms.ListAll(ctx, ws.ID)
	if err != nil || len(labels) != 1 {
		t.Fatalf("ListAll: %v %v", labels, err)
	}
	if err := h.pages.AddRedirect(ctx, ws.ID, "/frueher", "/start", 301); err != nil {
		t.Fatalf("AddRedirect: %v", err)
	}
	redirects, err := h.pages.ListRedirects(ctx, ws.ID)
	if err != nil || len(redirects) != 1 {
		t.Fatalf("ListRedirects: %v %v", redirects, err)
	}
	return labelsAndRedirects{h, sm, database, ws.ID, editor, labels[0].ID, redirects[0].ID}
}

func (f labelsAndRedirects) labelNames(t *testing.T) []string {
	t.Helper()
	labels, err := f.h.terms.ListAll(context.Background(), f.ws)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	var names []string
	for _, l := range labels {
		names = append(names, l.Name)
	}
	return names
}

func (f labelsAndRedirects) redirectSources(t *testing.T) []string {
	t.Helper()
	list, err := f.h.pages.ListRedirects(context.Background(), f.ws)
	if err != nil {
		t.Fatalf("ListRedirects: %v", err)
	}
	var from []string
	for _, r := range list {
		from = append(from, r.FromPath)
	}
	return from
}

func TestAnEditorWithoutPublishingCannotChangeLabelsOrRedirects(t *testing.T) {
	labelList := func(ws int64) string { return fmt.Sprintf("/admin/websites/%d/tags", ws) }
	redirectList := func(ws int64) string { return fmt.Sprintf("/admin/websites/%d/redirects", ws) }

	t.Run("label rename", func(t *testing.T) {
		f := seedLabelsAndRedirects(t)
		rec, flash := serveAs(t, f.h, f.sm, f.editor, f.h.HandleTermRename,
			postForm(fmt.Sprintf("%s/%d/rename", labelList(f.ws), f.label), url.Values{"name": {"Möbelstücke"}},
				websiteRoute(f.ws, "termID", strconv.FormatInt(f.label, 10))))
		wantRefused(t, "label rename", rec, flash, labelList(f.ws))
		if got := f.labelNames(t); len(got) != 1 || got[0] != "Möbel" {
			t.Errorf("labels after the refusal: %q", got)
		}
	})

	t.Run("label delete", func(t *testing.T) {
		f := seedLabelsAndRedirects(t)
		rec, flash := serveAs(t, f.h, f.sm, f.editor, f.h.HandleTermDelete,
			postForm(fmt.Sprintf("%s/%d/delete", labelList(f.ws), f.label), nil,
				websiteRoute(f.ws, "termID", strconv.FormatInt(f.label, 10))))
		wantRefused(t, "label delete", rec, flash, labelList(f.ws))
		if got := f.labelNames(t); len(got) != 1 {
			t.Errorf("labels after the refusal: %q", got)
		}
	})

	t.Run("redirect add", func(t *testing.T) {
		f := seedLabelsAndRedirects(t)
		form := url.Values{"from_path": {"/alte-seite"}, "to_path": {"/start"}, "code": {"301"}}
		rec, flash := serveAs(t, f.h, f.sm, f.editor, f.h.HandleRedirectList,
			postForm(redirectList(f.ws), form, websiteRoute(f.ws)))
		wantRefused(t, "redirect add", rec, flash, redirectList(f.ws))
		rec, flash = serveAs(t, f.h, f.sm, f.editor, f.h.HandleRedirectList,
			htmx(postForm(redirectList(f.ws), form, websiteRoute(f.ws))))
		wantRefusedHtmx(t, "redirect add", rec, flash, redirectList(f.ws))
		if got := f.redirectSources(t); len(got) != 1 || got[0] != "/frueher" {
			t.Errorf("redirects after the refusal: %q", got)
		}
	})

	t.Run("redirect delete", func(t *testing.T) {
		f := seedLabelsAndRedirects(t)
		rec, flash := serveAs(t, f.h, f.sm, f.editor, f.h.HandleRedirectDelete,
			postForm(fmt.Sprintf("%s/%d/delete", redirectList(f.ws), f.redirect), nil,
				websiteRoute(f.ws, "redirectID", strconv.FormatInt(f.redirect, 10))))
		wantRefused(t, "redirect delete", rec, flash, redirectList(f.ws))
		if got := f.redirectSources(t); len(got) != 1 {
			t.Errorf("redirects after the refusal: %q", got)
		}
	})
}

func TestTheLabelAndRedirectScreensOfferNoChangeWithoutPublishing(t *testing.T) {
	f := seedLabelsAndRedirects(t)
	publisher := editorWhoMayPublish(t, f.database, f.ws)
	route := websiteRoute(f.ws)
	labels := fmt.Sprintf("/admin/websites/%d/tags", f.ws)
	redirects := fmt.Sprintf("/admin/websites/%d/redirects", f.ws)
	addAction := fmt.Sprintf(`action="%s"`, redirects)

	body := getAs(t, f.sm, putUser(f.sm, f.editor), f.h.HandleTermList, labels, route)
	if strings.Count(body, readOnlyText) != 1 {
		t.Errorf("labels: the sentence stands %d times, want once", strings.Count(body, readOnlyText))
	}
	for _, mark := range []string{`/rename"`, `/delete"`} {
		if strings.Contains(body, mark) {
			t.Errorf("labels: offers %s", mark)
		}
	}
	if !strings.Contains(body, "Möbel") {
		t.Error("labels: the label's name is no longer shown")
	}

	body = getAs(t, f.sm, putUser(f.sm, f.editor), f.h.HandleRedirectList, redirects, route)
	if strings.Count(body, readOnlyText) != 1 {
		t.Errorf("redirects: the sentence stands %d times, want once", strings.Count(body, readOnlyText))
	}
	for _, mark := range []string{addAction, `/delete"`, `redirects?from=`} {
		if strings.Contains(body, mark) {
			t.Errorf("redirects: offers %s", mark)
		}
	}
	if !strings.Contains(body, "/frueher") || !strings.Contains(body, "/alte-seite") {
		t.Error("redirects: the redirect or the broken link is no longer listed")
	}

	body = getAs(t, f.sm, putUser(f.sm, publisher), f.h.HandleTermList, labels, route)
	if strings.Contains(body, readOnlyText) || !strings.Contains(body, `/rename"`) || !strings.Contains(body, `/delete"`) {
		t.Error("labels: a person who may publish is not offered rename and delete, or reads the sentence")
	}
	body = getAs(t, f.sm, putUser(f.sm, publisher), f.h.HandleRedirectList, redirects, route)
	if strings.Contains(body, readOnlyText) {
		t.Error("redirects: a person who may publish reads the sentence")
	}
	for _, mark := range []string{addAction, `/delete"`, `redirects?from=`} {
		if !strings.Contains(body, mark) {
			t.Errorf("redirects: a person who may publish is not offered %s", mark)
		}
	}
}
