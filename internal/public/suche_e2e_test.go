package public

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
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/plugin/wasmtest"
)

// The search as a plugin, through the whole chain: real module, real runtime,
// real middleware, real HTTP answer.
//
// The point is not that the search works — the page store checks that itself.
// The point is that a function that has moved out of the core stays
// indistinguishable from outside: the same address, the same view of the theme,
// the same headers.
func TestSuchePluginBeantwortetSuche(t *testing.T) {
	module := wasmtest.Module(t, "../../plugins/suche/plugin.wasm")
	raw, err := os.ReadFile("../../plugins/suche/plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := plugin.ParseManifest(raw)
	if err != nil {
		t.Fatalf("the shipped manifest is invalid: %v", err)
	}

	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Velowerkstatt")
	seedPage(t, database, ws.ID, "Wolle vom Hof", "wolle",
		"Wir verkaufen Wolle von unseren Schafen.", "published")
	seedPage(t, database, ws.ID, "Noch nicht fertig", "entwurf",
		"Auch hier steht Wolle, aber die Seite ist ein Entwurf.", "draft")

	h.SetPlugins(loadPlugin(t, h, database, manifest, module, ws.ID))

	// The request goes through the same middleware as in the server: the
	// resolver has already set the website, the mux would come only afterwards.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://velowerkstatt.test/suche?q=Wolle", nil)
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	h.PluginMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("die Anfrage lief am Plugin vorbei bis zum Kern")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status %d, Rumpf: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	// The theme's view, not a bare page of the plugin.
	if !strings.Contains(body, "<html>") || !strings.Contains(body, "Suche: Wolle") {
		t.Errorf("the answer is not a page of the theme:\n%s", body)
	}
	if !strings.Contains(body, "Wolle vom Hof") {
		t.Errorf("der Treffer fehlt:\n%s", body)
	}
	// And the draft is not to be had through the plugin either.
	if strings.Contains(body, "Noch nicht fertig") {
		t.Errorf("the draft is among the hits:\n%s", body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
	if rec.Header().Get("X-Robots-Tag") != "noindex" {
		t.Errorf("X-Robots-Tag = %q", rec.Header().Get("X-Robots-Tag"))
	}
}

// Without the plugin there is no search — and a theme must then not link to it
// either, or the website points at an address of its own that does not exist.
func TestOhneSuchePluginKeineSuche(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Velowerkstatt")

	if h.hasSearch(ws.ID) {
		t.Error("without the plugin the website reports a search")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://velowerkstatt.test/suche?q=Wolle", nil)
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	reached := false
	h.PluginMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	})).ServeHTTP(rec, req)

	if !reached {
		t.Error("the request did not reach the core")
	}
}

// loadPlugin installs a module, switches it on for one website and returns the
// manager, wired to the handler's host operations.
func loadPlugin(t *testing.T, h *Handler, database *db.DB, m *plugin.Manifest, module []byte, websiteID int64) *plugin.Manager {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()

	store := plugin.NewStore(database)
	if err := store.Install(ctx, &plugin.Package{
		Manifest: m, Module: module, SHA256: strings.Repeat("c", 64),
	}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.SetEnabled(ctx, m.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if err := store.SetWebsites(ctx, m.ID, []int64{websiteID}); err != nil {
		t.Fatalf("SetWebsites: %v", err)
	}

	// The manager reads the module from disk, just as in the server.
	if err := os.MkdirAll(filepath.Join(dir, "plugins", m.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins", m.ID, plugin.ModuleName), module, 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := plugin.NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	t.Cleanup(func() { rt.Close(context.Background()) })
	// The same host functions as in the server: read pages and render in the
	// theme. Without them the plugin would stand before locked doors.
	rt.WithPages(h.PagesForPlugin)
	rt.WithRender(h.RenderForPlugin)
	rt.WithNotify(h.NotifyForPlugin)

	manager, err := plugin.NewManager(ctx, store, rt, dir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return manager
}
