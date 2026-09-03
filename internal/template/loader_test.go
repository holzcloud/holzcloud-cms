package template

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestSafeAssetPathRejectsTraversal(t *testing.T) {
	// http.ServeMux cleans the *escaped* URL, so a percent-encoded "%2e%2e"
	// reaches the handler already decoded to "..". These are the values a
	// handler can actually receive from r.PathValue.
	rejected := []string{
		"",
		"..",
		"../style.css",
		"../../etc/passwd",
		"../../holzcloud.sqlite",
		"a/../../b",
		"/etc/passwd",
		"/style.css",
		`..\windows`,
		"foo\x00bar",
	}
	for _, p := range rejected {
		if got, ok := SafeAssetPath(p); ok {
			t.Errorf("SafeAssetPath(%q) = %q, true; want not ok", p, got)
		}
	}

	accepted := map[string]string{
		"style.css":            "style.css",
		"img/logo.png":         "img/logo.png",
		"./style.css":          "style.css",
		"fonts/a/../b.woff2":   "fonts/b.woff2",
		"deeply/nested/x.webp": "deeply/nested/x.webp",
	}
	for in, want := range accepted {
		got, ok := SafeAssetPath(in)
		if !ok {
			t.Errorf("SafeAssetPath(%q) rejected; want accepted", in)
			continue
		}
		if got != want {
			t.Errorf("SafeAssetPath(%q) = %q; want %q", in, got, want)
		}
	}
}

type stubResolver struct{ slug string }

func (s stubResolver) ActiveTemplateSlug(context.Context, int64) (string, error) {
	return s.slug, nil
}

// Asset must never read outside the template directory, even though the
// requested path is joined onto a filesystem path.
func TestAssetDoesNotEscapeDataDir(t *testing.T) {
	dataDir := t.TempDir()

	secret := filepath.Join(dataDir, "csrf.key")
	if err := os.WriteFile(secret, []byte("TOP-SECRET-KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	tmplDir := filepath.Join(dataDir, "templates", "mytheme")
	if err := os.MkdirAll(tmplDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "style.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	defaultFS := fstest.MapFS{"style.css": &fstest.MapFile{Data: []byte("default")}}
	loader := NewLoader(dataDir, defaultFS, nil, stubResolver{slug: "mytheme"})

	// The real asset still resolves.
	got, err := loader.Asset(context.Background(), 1, "style.css")
	if err != nil {
		t.Fatalf("Asset(style.css): %v", err)
	}
	if string(got) != "body{}" {
		t.Errorf("Asset(style.css) = %q; want the on-disk template asset", got)
	}

	// Traversal out of the template directory must not.
	for _, p := range []string{"../../csrf.key", "../csrf.key", "../../../etc/passwd"} {
		content, err := loader.Asset(context.Background(), 1, p)
		if err != nil {
			t.Errorf("Asset(%q) returned error %v; want nil content", p, err)
		}
		if content != nil {
			t.Errorf("Asset(%q) leaked %d bytes: %q", p, len(content), content)
		}
	}
}

// A slug that is not a plain path segment must never be joined onto a path.
func TestAssetRejectsUnsafeSlug(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "secret.css"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	defaultFS := fstest.MapFS{}
	loader := NewLoader(dataDir, defaultFS, nil, stubResolver{slug: "../.."})

	content, err := loader.Asset(context.Background(), 1, "secret.css")
	if err != nil {
		t.Fatalf("Asset: %v", err)
	}
	if content != nil {
		t.Errorf("unsafe slug resolved to %q; want nil", content)
	}
}
