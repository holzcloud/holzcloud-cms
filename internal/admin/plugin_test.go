package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// A plugin's files come from an archive and are served from the admin's own
// origin, so a page or an image with script in it must never open as a page.
func TestPluginAssetsAreNeverServedAsDocuments(t *testing.T) {
	h, _, database, _ := newTestAdmin(t)
	dir := t.TempDir()
	store := plugin.NewStore(database)
	rt, err := plugin.NewRuntime(context.Background(), store, nil)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	mgr, err := plugin.NewManager(context.Background(), store, rt, dir, nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	h.SetPlugins(mgr)

	assets := filepath.Join(dir, "plugins", "eins", plugin.AssetDir)
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{ // name -> forced download
		"seite.html": true, "seite.htm": true, "bild.svg": true, "daten.xml": true, "x.xhtml": true,
		"app.js": false, "stil.css": false,
	}
	for name := range files {
		if err := os.WriteFile(filepath.Join(assets, name), []byte("<script>alert(1)</script>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for name, download := range files {
		req := httptest.NewRequest(http.MethodGet, "/plugin-assets/eins/"+name, nil)
		req.SetPathValue("id", "eins")
		req.SetPathValue("path", name)
		rec := httptest.NewRecorder()
		if err := h.HandlePluginAsset(rec, req); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", name, rec.Code)
		}
		hd := rec.Header()
		if hd.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: no nosniff", name)
		}
		if csp := hd.Get("Content-Security-Policy"); csp != "sandbox; default-src 'none'" {
			t.Errorf("%s: CSP %q", name, csp)
		}
		if got := hd.Get("Content-Disposition") == "attachment"; got != download {
			t.Errorf("%s: attachment=%v, want %v", name, got, download)
		}
	}
}
