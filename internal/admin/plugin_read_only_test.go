package admin

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// A plugin screen is the one place where the host cannot tell a public write
// from a private one: the farm shop's settings shape the public order form,
// ticking off an order changes nothing anybody outside sees, and both arrive
// as a POST to the same address. So the plugin decides (D-13). A plugin that
// declares respects_read_only is told on every call whether the person may
// publish, and refuses its public writes itself; for one that does not, the
// host refuses every POST of such a person before the plugin is called.

// installShippedPlugin installs a plugin this repository ships, as
// installFormPlugin does, switched on and assigned to sites. mutate, when not
// nil, changes the parsed manifest before it is installed — the way a test
// makes a shipped module stand in for a third-party plugin that never heard
// of the read-only rule.
func installShippedPlugin(t *testing.T, database *db.DB, dir, id string, sites []int64, mutate func(*plugin.Manifest)) *plugin.Manager {
	t.Helper()
	ctx := context.Background()
	src := filepath.Join("..", "..", "plugins", id)

	raw, err := os.ReadFile(filepath.Join(src, "plugin.json"))
	if err != nil {
		t.Fatalf("read manifest of %s: %v", id, err)
	}
	manifest, err := plugin.ParseManifest(raw)
	if err != nil {
		t.Fatalf("parse manifest of %s: %v", id, err)
	}
	if mutate != nil {
		mutate(manifest)
	}
	module, err := os.ReadFile(filepath.Join(src, plugin.ModuleName))
	if err != nil {
		t.Fatalf("read module of %s: %v", id, err)
	}

	store := plugin.NewStore(database)
	if err := store.Install(ctx, &plugin.Package{Manifest: manifest, Module: module, SHA256: "c0ffee"}); err != nil {
		t.Fatalf("Install %s: %v", id, err)
	}
	if err := store.SetEnabled(ctx, id, true); err != nil {
		t.Fatalf("SetEnabled %s: %v", id, err)
	}
	if err := store.SetWebsites(ctx, id, sites); err != nil {
		t.Fatalf("SetWebsites %s: %v", id, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "plugins", id), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins", id, plugin.ModuleName), module, 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := plugin.NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	t.Cleanup(func() { rt.Close(context.Background()) })
	mgr, err := plugin.NewManager(ctx, store, rt, dir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return mgr
}

// pluginFlashAs runs a handler for this person — the user id alone in the
// session, no role, so the rights decide — and hands back both flashes.
func pluginFlashAs(t *testing.T, h *Handler, sm *scs.SessionManager, userID int64,
	fn func(http.ResponseWriter, *http.Request) error, req *http.Request,
) (rec *httptest.ResponseRecorder, bad, good string) {
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

func pluginScreenAddress(websiteID int64, id string) string {
	return fmt.Sprintf("/admin/websites/%d/plugins/%s", websiteID, id)
}

func pluginRoute(websiteID int64, id string) map[string]string {
	return map[string]string{"websiteID": strconv.FormatInt(websiteID, 10), "id": id}
}

// farmShopSettings is what the farm shop stored as its settings on this
// website, and whether it stored any.
func farmShopSettings(t *testing.T, database *db.DB, websiteID int64) (string, bool) {
	t.Helper()
	raw, ok, err := plugin.NewStore(database).StoreGet(context.Background(), "bestellung", websiteID, "einstellungen")
	if err != nil {
		t.Fatalf("StoreGet: %v", err)
	}
	return raw, ok
}

// lockedScreen reports whether the plugin's HTML stands inside a disabled
// fieldset: the fieldset comes first, and mark — a piece of the plugin's own
// screen — after it.
func lockedScreen(body, mark string) bool {
	loc := disabledFieldset.FindStringIndex(body)
	if loc == nil {
		return false
	}
	return strings.Contains(body[loc[1]:], mark)
}

func TestAPluginScreenThatDoesNotKnowTheRightIsReadOnly(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	// The shipped farm shop, with its declaration taken away: a module that
	// has never heard of the rule.
	h.SetPlugins(installShippedPlugin(t, database, t.TempDir(), "bestellung", []int64{ws.ID},
		func(m *plugin.Manifest) { m.Admin.RespectsReadOnly = false }))
	editor := editorWithoutPublishing(t, database, ws.ID)
	publisher := editorWhoMayPublish(t, database, ws.ID)
	address := pluginScreenAddress(ws.ID, "bestellung")
	route := pluginRoute(ws.ID, "bestellung")
	settings := url.Values{"aktion": {"einstellungen"}, "preis_feld": {"Preis"}}

	rec, bad, good := pluginFlashAs(t, h, sm, editor, h.HandlePluginScreen, postForm(address, settings, route))
	wantRefused(t, "undeclared plugin POST", rec, bad, address)
	if good != "" {
		t.Errorf("the plugin was called and answered %q", good)
	}
	if raw, ok := farmShopSettings(t, database, ws.ID); ok {
		t.Fatalf("the settings were stored by somebody without the publishing right: %s", raw)
	}

	rec, bad, _ = pluginFlashAs(t, h, sm, editor, h.HandlePluginScreen, htmx(postForm(address, settings, route)))
	wantRefusedHtmx(t, "undeclared plugin POST", rec, bad, address)
	if raw, ok := farmShopSettings(t, database, ws.ID); ok {
		t.Fatalf("the settings were stored through htmx: %s", raw)
	}

	// The screen itself opens: the sentence above it, and every control of the
	// plugin inside a disabled fieldset.
	body := getAs(t, sm, putUser(sm, editor), h.HandlePluginScreen, address+"?ansicht=einstellungen", route)
	if n := strings.Count(body, readOnlyText); n != 1 {
		t.Errorf("the sentence stands %d times on the screen, want once", n)
	}
	if !lockedScreen(body, `name="preis_feld"`) {
		t.Error("the plugin's form is not inside a disabled fieldset")
	}

	// The control: somebody who may publish sees the screen as before and
	// saves the settings.
	body = getAs(t, sm, putUser(sm, publisher), h.HandlePluginScreen, address+"?ansicht=einstellungen", route)
	if strings.Contains(body, readOnlyText) || disabledFieldset.MatchString(body) {
		t.Error("a person who may publish was shown the read-only screen")
	}
	rec, bad, good = pluginFlashAs(t, h, sm, publisher, h.HandlePluginScreen, postForm(address, settings, route))
	if rec.Code != http.StatusSeeOther || bad != "" || good == "" {
		t.Errorf("publisher: status %d, bad %q, good %q", rec.Code, bad, good)
	}
	if raw, ok := farmShopSettings(t, database, ws.ID); !ok || !strings.Contains(raw, `"Preis"`) {
		t.Errorf("a person who may publish could not save the settings: %q (%v)", raw, ok)
	}
}
