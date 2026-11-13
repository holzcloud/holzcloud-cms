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
	if loc == nil || !hostLocked(body) {
		return false
	}
	return strings.Contains(body[loc[1]:], mark)
}

// hostLockMark is the host's own fieldset around a plugin's screen.
const hostLockMark = `<fieldset class="read-only-fields" disabled>`

// hostLocked reports whether the host itself disabled the screen — its own
// fieldset, not one the plugin drew around fields it leaves read-only.
func hostLocked(body string) bool { return strings.Contains(body, hostLockMark) }

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
	if strings.Contains(body, readOnlyText) || hostLocked(body) {
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

// shippedPluginAdmin is a handler with one shipped plugin installed exactly as
// it is shipped — manifest unchanged — and the two kinds of editor.
func shippedPluginAdmin(t *testing.T, id string) (h *Handler, sm *scs.SessionManager, database *db.DB, websiteID, editor, publisher int64) {
	t.Helper()
	h, sm, database, ws := newTestAdmin(t)
	h.SetPlugins(installShippedPlugin(t, database, t.TempDir(), id, []int64{ws.ID}, nil))
	return h, sm, database, ws.ID, editorWithoutPublishing(t, database, ws.ID), editorWhoMayPublish(t, database, ws.ID)
}

// pluginStored reads one key a plugin stored on this website.
func pluginStored(t *testing.T, database *db.DB, id string, websiteID int64, key string) (string, bool) {
	t.Helper()
	raw, ok, err := plugin.NewStore(database).StoreGet(context.Background(), id, websiteID, key)
	if err != nil {
		t.Fatalf("StoreGet %s %s: %v", id, key, err)
	}
	return raw, ok
}

// pluginStore puts a record into a plugin's storage, the way a visitor's
// submission would have.
func pluginStore(t *testing.T, database *db.DB, id string, websiteID int64, key, value string) {
	t.Helper()
	if err := plugin.NewStore(database).StoreSet(context.Background(), id, websiteID, key, value); err != nil {
		t.Fatalf("StoreSet %s %s: %v", id, key, err)
	}
}

// wantPluginRefused is the refusal of a plugin that declared the rule: its own
// answer, so the redirect goes back to the view the request came from, with
// the sentence as an error flash.
func wantPluginRefused(t *testing.T, what string, rec *httptest.ResponseRecorder, bad, good, to string) {
	t.Helper()
	wantRefused(t, what, rec, bad, to)
	if good != "" {
		t.Errorf("%s: a success was reported as well: %q", what, good)
	}
}

func TestTheShippedPluginsRefuseWhatChangesTheWebsite(t *testing.T) {
	t.Run("farm shop settings", func(t *testing.T) {
		h, sm, database, wsID, editor, publisher := shippedPluginAdmin(t, "bestellung")
		address := pluginScreenAddress(wsID, "bestellung")
		route := pluginRoute(wsID, "bestellung")

		rec, bad, good := pluginFlashAs(t, h, sm, editor, h.HandlePluginScreen,
			postForm(address, url.Values{"aktion": {"einstellungen"}, "preis_feld": {"Preis"}}, route))
		wantPluginRefused(t, "settings", rec, bad, good, address+"?ansicht=einstellungen")
		if raw, ok := pluginStored(t, database, "bestellung", wsID, "einstellungen"); ok {
			t.Fatalf("the settings were stored: %s", raw)
		}

		body := getAs(t, sm, putUser(sm, editor), h.HandlePluginScreen, address+"?ansicht=einstellungen", route)
		if n := strings.Count(body, readOnlyText); n != 1 {
			t.Errorf("settings: the sentence stands %d times, want once", n)
		}
		if strings.Contains(body, `value="einstellungen"`) {
			t.Error("settings: the screen still offers to save")
		}
		if hostLocked(body) {
			t.Error("settings: the host locked a screen whose plugin declared the rule")
		}

		// The publisher keeps the save button.
		body = getAs(t, sm, putUser(sm, publisher), h.HandlePluginScreen, address+"?ansicht=einstellungen", route)
		if !strings.Contains(body, `value="einstellungen"`) || strings.Contains(body, readOnlyText) {
			t.Error("settings: a person who may publish lost the save button or was shown the sentence")
		}
	})

	t.Run("contact form editor", func(t *testing.T) {
		h, sm, database, wsID, editor, publisher := shippedPluginAdmin(t, "kontaktformular")
		address := pluginScreenAddress(wsID, "kontaktformular")
		route := pluginRoute(wsID, "kontaktformular")
		forms := address + "?ansicht=formulare"

		rec, bad, good := pluginFlashAs(t, h, sm, editor, h.HandlePluginScreen,
			postForm(forms, url.Values{"neues_formular": {"Hoffest"}}, route))
		wantPluginRefused(t, "new form", rec, bad, good, forms)
		if raw, ok := pluginStored(t, database, "kontaktformular", wsID, "formular:hoffest"); ok {
			t.Fatalf("the form was created: %s", raw)
		}
		if body := getAs(t, sm, putUser(sm, publisher), h.HandlePluginScreen, forms, route); strings.Contains(body, "Hoffest") {
			t.Error("the form list shows the form the editor was refused")
		}

		// A form the publisher made: the editor may neither delete nor change it.
		_, bad, _ = pluginFlashAs(t, h, sm, publisher, h.HandlePluginScreen,
			postForm(forms, url.Values{"neues_formular": {"Sommerfest"}}, route))
		if bad != "" {
			t.Fatalf("publisher could not create a form: %q", bad)
		}
		before, ok := pluginStored(t, database, "kontaktformular", wsID, "formular:sommerfest")
		if !ok {
			t.Fatal("the publisher's form was not stored")
		}

		rec, bad, good = pluginFlashAs(t, h, sm, editor, h.HandlePluginScreen,
			postForm(forms, url.Values{"loeschen_formular": {"sommerfest"}}, route))
		wantPluginRefused(t, "delete form", rec, bad, good, forms)

		editorAddress := address + "?ansicht=formular&kennung=sommerfest"
		for name, values := range map[string]url.Values{
			"save form":    {"kennung": {"sommerfest"}, "name": {"Winterfest"}, "sichern": {"1"}},
			"field action": {"kennung": {"sommerfest"}, "name": {"Sommerfest"}, "feldaktion": {"neu"}},
		} {
			rec, bad, good = pluginFlashAs(t, h, sm, editor, h.HandlePluginScreen, postForm(editorAddress, values, route))
			wantPluginRefused(t, name, rec, bad, good, editorAddress)
		}
		if after, _ := pluginStored(t, database, "kontaktformular", wsID, "formular:sommerfest"); after != before {
			t.Errorf("the form was changed by somebody without the publishing right:\n%s\n%s", before, after)
		}

		body := getAs(t, sm, putUser(sm, editor), h.HandlePluginScreen, forms, route)
		if n := strings.Count(body, readOnlyText); n != 1 {
			t.Errorf("form list: the sentence stands %d times, want once", n)
		}
		for _, control := range []string{`name="neues_formular"`, `name="loeschen_formular"`} {
			if strings.Contains(body, control) {
				t.Errorf("form list offers %s", control)
			}
		}
		if !strings.Contains(body, "Sommerfest") {
			t.Error("form list: the editor no longer sees the forms")
		}
		body = getAs(t, sm, putUser(sm, editor), h.HandlePluginScreen, editorAddress, route)
		for _, control := range []string{`name="sichern"`, `name="feldaktion"`} {
			if strings.Contains(body, control) {
				t.Errorf("form editor offers %s", control)
			}
		}
		if strings.Count(body, readOnlyText) != 1 {
			t.Error("form editor: the sentence is missing")
		}

		// The publisher keeps every control.
		body = getAs(t, sm, putUser(sm, publisher), h.HandlePluginScreen, forms, route)
		for _, control := range []string{`name="neues_formular"`, `name="loeschen_formular"`} {
			if !strings.Contains(body, control) {
				t.Errorf("a person who may publish lost %s", control)
			}
		}
	})

	t.Run("contact form attachments and consent", func(t *testing.T) {
		h, sm, database, wsID, editor, publisher := shippedPluginAdmin(t, "kontaktformular")
		address := pluginScreenAddress(wsID, "kontaktformular")
		route := pluginRoute(wsID, "kontaktformular")
		consent := address + "?ansicht=einwilligung"

		for name, values := range map[string]url.Values{
			"attachments": {"anhang_sichern": {"1"}, "anhang": {"1"}},
			"consent":     {"einwilligung_sichern": {"1"}, "einwilligungstext": {"Ich bin einverstanden."}},
		} {
			rec, bad, good := pluginFlashAs(t, h, sm, editor, h.HandlePluginScreen, postForm(consent, values, route))
			wantPluginRefused(t, name, rec, bad, good, consent)
		}
		for _, key := range []string{"anhang", "einwilligungstext"} {
			if raw, ok := pluginStored(t, database, "kontaktformular", wsID, key); ok {
				t.Errorf("%s was stored by somebody without the publishing right: %q", key, raw)
			}
		}

		body := getAs(t, sm, putUser(sm, editor), h.HandlePluginScreen, consent, route)
		if strings.Count(body, readOnlyText) != 1 {
			t.Error("consent: the sentence is missing")
		}
		for _, control := range []string{`name="einwilligung_sichern"`, `name="anhang_sichern"`} {
			if strings.Contains(body, control) {
				t.Errorf("consent screen offers %s", control)
			}
		}
		body = getAs(t, sm, putUser(sm, publisher), h.HandlePluginScreen, consent, route)
		for _, control := range []string{`name="einwilligung_sichern"`, `name="anhang_sichern"`} {
			if !strings.Contains(body, control) {
				t.Errorf("a person who may publish lost %s", control)
			}
		}
	})
}

func TestTheShippedPluginsStillLetAnEditorHandleOrdersAndMessages(t *testing.T) {
	t.Run("farm shop orders", func(t *testing.T) {
		h, sm, database, wsID, editor, _ := shippedPluginAdmin(t, "bestellung")
		address := pluginScreenAddress(wsID, "bestellung")
		route := pluginRoute(wsID, "bestellung")
		pluginStore(t, database, "bestellung", wsID, "bestellung:b1",
			`{"id":"b1","eingegangen":"2026-11-12T10:00:00Z","name":"Anna","email":"anna@example.com","posten":[]}`)

		rec, bad, good := pluginFlashAs(t, h, sm, editor, h.HandlePluginScreen,
			postForm(address, url.Values{"aktion": {"erledigt"}, "id": {"b1"}}, route))
		if rec.Code != http.StatusSeeOther || bad != "" || good != "Order ticked off." {
			t.Errorf("tick off: status %d, bad %q, good %q", rec.Code, bad, good)
		}
		if raw, _ := pluginStored(t, database, "bestellung", wsID, "bestellung:b1"); !strings.Contains(raw, `"erledigt":true`) {
			t.Errorf("the order was not ticked off: %s", raw)
		}

		rec, bad, good = pluginFlashAs(t, h, sm, editor, h.HandlePluginScreen,
			postForm(address, url.Values{"aktion": {"loeschen"}, "id": {"b1"}}, route))
		if rec.Code != http.StatusSeeOther || bad != "" || good != "Order deleted." {
			t.Errorf("delete: status %d, bad %q, good %q", rec.Code, bad, good)
		}
		if _, ok := pluginStored(t, database, "bestellung", wsID, "bestellung:b1"); ok {
			t.Error("the order is still there")
		}
	})

	t.Run("contact form messages", func(t *testing.T) {
		h, sm, database, wsID, editor, _ := shippedPluginAdmin(t, "kontaktformular")
		address := pluginScreenAddress(wsID, "kontaktformular")
		route := pluginRoute(wsID, "kontaktformular")
		turnedAway := address + "?ansicht=abgewiesen"
		spam := func(key string) {
			pluginStore(t, database, "kontaktformular", wsID, "abgewiesen:"+key,
				`{"kennung":"`+key+`","name":"Bot","email":"bot@example.com","text":"Buy now","zeit":"2026-11-12T10:00:00Z"}`)
		}
		spam("q1")
		spam("q2")

		body := getAs(t, sm, putUser(sm, editor), h.HandlePluginScreen, turnedAway, route)
		if !strings.Contains(body, `name="verwerfen"`) || !strings.Contains(body, `name="freigeben"`) {
			t.Error("the turned-away screen offers the editor no way to discard or release")
		}
		if strings.Contains(body, readOnlyText) || hostLocked(body) {
			t.Error("the turned-away screen is read-only for the editor")
		}

		rec, bad, good := pluginFlashAs(t, h, sm, editor, h.HandlePluginScreen,
			postForm(turnedAway, url.Values{"verwerfen": {"q1"}}, route))
		if rec.Code != http.StatusSeeOther || bad != "" || good != "Discarded." {
			t.Errorf("discard: status %d, bad %q, good %q", rec.Code, bad, good)
		}
		if _, ok := pluginStored(t, database, "kontaktformular", wsID, "abgewiesen:q1"); ok {
			t.Error("the discarded submission is still there")
		}

		rec, bad, good = pluginFlashAs(t, h, sm, editor, h.HandlePluginScreen,
			postForm(turnedAway, url.Values{"freigeben": {"q2"}}, route))
		if rec.Code != http.StatusSeeOther || bad != "" || good != "Moved into the messages." {
			t.Errorf("release: status %d, bad %q, good %q", rec.Code, bad, good)
		}
	})

	t.Run("not-found log", func(t *testing.T) {
		h, sm, _, wsID, editor, _ := shippedPluginAdmin(t, "nicht-gefunden")
		address := pluginScreenAddress(wsID, "nicht-gefunden")
		route := pluginRoute(wsID, "nicht-gefunden")

		rec, bad, good := pluginFlashAs(t, h, sm, editor, h.HandlePluginScreen,
			postForm(address, url.Values{"alles_loeschen": {"1"}}, route))
		if rec.Code != http.StatusSeeOther || bad != "" || good != "List emptied." {
			t.Errorf("clear: status %d, bad %q, good %q", rec.Code, bad, good)
		}
	})
}
