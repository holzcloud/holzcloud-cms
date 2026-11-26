package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// A plugin's files come from an archive and are served from the admin's own
// origin, so nothing a browser runs or opens as a page may come out of
// /plugin-assets: not a script, not a document, even when an installation from
// before the extension table still has one on disk.
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
	type want struct {
		status   int
		ct       string
		download bool
	}
	files := map[string]want{
		"app.js":     {http.StatusNotFound, "", false},
		"modul.mjs":  {http.StatusNotFound, "", false},
		"seite.html": {http.StatusNotFound, "", false},
		"seite.htm":  {http.StatusNotFound, "", false},
		"x.xhtml":    {http.StatusNotFound, "", false},
		"daten.xml":  {http.StatusNotFound, "", false},
		"ohne":       {http.StatusNotFound, "", false},
		"stil.css":   {http.StatusOK, "text/css; charset=utf-8", false},
		"bild.svg":   {http.StatusOK, "image/svg+xml", true},
	}
	for name := range files {
		if err := os.WriteFile(filepath.Join(assets, name), []byte("<script>alert(1)</script>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for name, w := range files {
		req := httptest.NewRequest(http.MethodGet, "/plugin-assets/eins/"+name, nil)
		req.SetPathValue("id", "eins")
		req.SetPathValue("path", name)
		rec := httptest.NewRecorder()
		if err := h.HandlePluginAsset(rec, req); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if rec.Code != w.status {
			t.Errorf("%s: status %d, want %d", name, rec.Code, w.status)
			continue
		}
		if w.status != http.StatusOK {
			if body := rec.Body.String(); strings.Contains(body, "alert(1)") {
				t.Errorf("%s: the file's content was served: %q", name, body)
			}
			continue
		}
		hd := rec.Header()
		if got := hd.Get("Content-Type"); got != w.ct {
			t.Errorf("%s: Content-Type %q, want %q", name, got, w.ct)
		}
		if hd.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: no nosniff", name)
		}
		if csp := hd.Get("Content-Security-Policy"); csp != "sandbox; default-src 'none'" {
			t.Errorf("%s: CSP %q", name, csp)
		}
		if got := hd.Get("Content-Disposition") == "attachment"; got != w.download {
			t.Errorf("%s: attachment=%v, want %v", name, got, w.download)
		}
	}
}
