package admin

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
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
