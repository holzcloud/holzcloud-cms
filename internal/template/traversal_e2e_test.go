package template_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

type resolver struct{ slug string }

func (r resolver) ActiveTemplateSlug(context.Context, int64) (string, error) { return r.slug, nil }

// Reproduces the original attack end to end: a percent-encoded traversal that
// survives ServeMux path cleaning, routed through the real preview asset path.
func TestPreviewAssetTraversalBlocked(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "csrf.key"), []byte("TOP-SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "templates", "default"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "templates", "default", "style.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := tmpl.NewLoader(dataDir, fstest.MapFS{}, nil, resolver{slug: "default"})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/websites/{id}/preview/t/{path...}", func(w http.ResponseWriter, r *http.Request) {
		p, ok := tmpl.SafeAssetPath(r.PathValue("path"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		content, err := loader.Asset(r.Context(), 1, p)
		if err != nil || content == nil {
			http.NotFound(w, r)
			return
		}
		w.Write(content)
	})

	attacks := []string{
		"/admin/websites/1/preview/t/%2e%2e/%2e%2e/csrf.key",
		"/admin/websites/1/preview/t/..%2f..%2fcsrf.key",
		"/admin/websites/1/preview/t/%2e%2e%2f%2e%2e%2fcsrf.key",
		"/admin/websites/1/preview/t/%2e%2e/%2e%2e/%2e%2e/etc/passwd",
	}
	for _, url := range attacks {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, body %q — expected 404", url, rec.Code, rec.Body.String())
		}
		if rec.Body.Len() > 0 && rec.Body.String() != "404 page not found\n" {
			t.Errorf("%s leaked: %q", url, rec.Body.String())
		}
	}

	// The legitimate asset still resolves.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/admin/websites/1/preview/t/style.css", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "body{}" {
		t.Errorf("legitimate asset broken: status %d body %q", rec.Code, rec.Body.String())
	}
}
