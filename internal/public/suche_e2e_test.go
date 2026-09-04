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

// Die Suche als Plugin, durch die ganze Kette: echtes Modul, echte Laufzeit,
// echte Middleware, echte HTTP-Antwort.
//
// Der Punkt ist nicht, dass die Suche funktioniert — das prüft der Seitenspeicher
// selbst. Der Punkt ist, dass eine Funktion, die aus dem Kern ausgezogen ist,
// von aussen ununterscheidbar bleibt: dieselbe Adresse, dieselbe Ansicht des
// Themes, dieselben Kopfzeilen.
func TestSuchePluginBeantwortetSuche(t *testing.T) {
	modul := wasmtest.Modul(t, "../../plugins/suche/plugin.wasm")
	roh, err := os.ReadFile("../../plugins/suche/plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := plugin.ParseManifest(roh)
	if err != nil {
		t.Fatalf("das mitgelieferte Manifest ist ungültig: %v", err)
	}

	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Velowerkstatt")
	seedPage(t, database, ws.ID, "Wolle vom Hof", "wolle",
		"Wir verkaufen Wolle von unseren Schafen.", "published")
	seedPage(t, database, ws.ID, "Noch nicht fertig", "entwurf",
		"Auch hier steht Wolle, aber die Seite ist ein Entwurf.", "draft")

	h.SetPlugins(loadPlugin(t, h, database, manifest, modul, ws.ID))

	// Die Anfrage geht durch dieselbe Middleware wie im Server: der Auflöser
	// hat die Website schon gesetzt, der Mux käme erst danach.
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

	// Die Ansicht des Themes, nicht eine nackte Seite des Plugins.
	if !strings.Contains(body, "<html>") || !strings.Contains(body, "Suche: Wolle") {
		t.Errorf("die Antwort ist keine Seite des Themes:\n%s", body)
	}
	if !strings.Contains(body, "Wolle vom Hof") {
		t.Errorf("der Treffer fehlt:\n%s", body)
	}
	// Und der Entwurf ist auch über das Plugin nicht zu bekommen.
	if strings.Contains(body, "Noch nicht fertig") {
		t.Errorf("der Entwurf steht in den Treffern:\n%s", body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
	if rec.Header().Get("X-Robots-Tag") != "noindex" {
		t.Errorf("X-Robots-Tag = %q", rec.Header().Get("X-Robots-Tag"))
	}
}

// Ohne Plugin gibt es keine Suche — und ein Theme darf dann auch nicht darauf
// verlinken, sonst zeigt die Website selbst auf eine Adresse, die es nicht gibt.
func TestOhneSuchePluginKeineSuche(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Velowerkstatt")

	if h.hasSearch(ws.ID) {
		t.Error("ohne Plugin meldet die Website eine Suche")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://velowerkstatt.test/suche?q=Wolle", nil)
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	reached := false
	h.PluginMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	})).ServeHTTP(rec, req)

	if !reached {
		t.Error("die Anfrage kam nicht beim Kern an")
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

	// Der Manager liest das Modul von der Platte, so wie im Server.
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
	// Dieselben Host-Funktionen wie im Server: Seiten lesen und im Theme
	// ausgeben. Ohne sie stünde das Plugin vor verschlossenen Türen.
	rt.WithPages(h.PagesForPlugin)
	rt.WithRender(h.RenderForPlugin)
	rt.WithNotify(h.NotifyForPlugin)

	manager, err := plugin.NewManager(ctx, store, rt, dir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return manager
}
