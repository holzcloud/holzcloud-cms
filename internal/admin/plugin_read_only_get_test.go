package admin

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// installEchoPlugin installs the test plugin of internal/plugin/testdata with
// an admin screen and without respects_read_only: a module that has never
// heard of the rule. Its screen writes on a GET with ?tue=schreiben, the way a
// simple screen's "delete" link does.
func installEchoPlugin(t *testing.T, database *db.DB, dir string, sites []int64) *plugin.Manager {
	t.Helper()
	ctx := context.Background()
	manifest, err := plugin.ParseManifest([]byte(`{
		"id": "echo", "abi": 1, "name": "Echo", "version": "1.0.0",
		"hooks": ["admin"], "permissions": ["store"],
		"admin": {"label": "Echo", "per_website": true}
	}`))
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	module, err := os.ReadFile(filepath.Join("..", "plugin", "testdata", "echo.wasm"))
	if err != nil {
		t.Fatalf("read echo.wasm: %v", err)
	}
	store := plugin.NewStore(database)
	if err := store.Install(ctx, &plugin.Package{Manifest: manifest, Module: module, SHA256: "c0ffee"}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.SetEnabled(ctx, "echo", true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if err := store.SetWebsites(ctx, "echo", sites); err != nil {
		t.Fatalf("SetWebsites: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "plugins", "echo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins", "echo", plugin.ModuleName), module, 0o644); err != nil {
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

// getRequest is a GET of target with the route's path values set.
func getRequest(target string, pathValues map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	return req
}

// The host refuses an undeclared plugin's POST for somebody without the
// publishing right, and a GET reaches the plugin as before — reading stays
// open (D-05). But a plugin may write on a GET: its "delete" link is a GET.
// So the GET runs against a host that refuses the plugin's writes, and an
// action refused that way answers with the read-only sentence (D-02, D-13).
func TestAnUndeclaredPluginCannotWriteOnAGetWithoutPublishing(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	h.SetPlugins(installEchoPlugin(t, database, t.TempDir(), []int64{ws.ID}))
	editor := editorWithoutPublishing(t, database, ws.ID)
	publisher := editorWhoMayPublish(t, database, ws.ID)
	address := pluginScreenAddress(ws.ID, "echo")
	route := pluginRoute(ws.ID, "echo")

	written := func() bool {
		t.Helper()
		_, ok := pluginStored(t, database, "echo", ws.ID, "admin")
		return ok
	}

	req := getRequest(address+"?tue=schreiben", route)
	rec, bad, _ := pluginFlashAs(t, h, sm, editor, h.HandlePluginScreen, req)
	if written() {
		t.Fatal("an undeclared plugin wrote on a GET of somebody without the publishing right")
	}
	wantRefused(t, "undeclared plugin GET that writes", rec, bad, address)

	// Reading stays open: the plain screen still shows, locked.
	body := getAs(t, sm, putUser(sm, editor), h.HandlePluginScreen, address, route)
	if !lockedScreen(body, "<p>echo</p>") {
		t.Error("the plain screen of an undeclared plugin no longer opens")
	}

	// A plugin's HTML that closes a fieldset it never opened must not close
	// the host's disabled one: its later controls would stand enabled on a
	// screen whose every POST is refused (IN-03).
	body = getAs(t, sm, putUser(sm, editor), h.HandlePluginScreen, address+"?tue=fieldset", route)
	if !insideDisabledFieldset(body, `name="los"`) {
		t.Error("a stray </fieldset> in the plugin's HTML closed the host's disabled fieldset")
	}

	// The control: somebody who may publish triggers the write.
	rec, bad, _ = pluginFlashAs(t, h, sm, publisher, h.HandlePluginScreen, getRequest(address+"?tue=schreiben", route))
	if bad != "" || !written() {
		t.Errorf("publisher: status %d, flash %q, written %v", rec.Code, bad, written())
	}
	if strings.Contains(rec.Body.String(), "verweigert") {
		t.Error("the host refused a publisher's write")
	}
}
