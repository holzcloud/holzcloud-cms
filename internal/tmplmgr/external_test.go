package tmplmgr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemplateDir materialises a template directory from name -> content.
func writeTemplateDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// refsContain reports whether any finding mentions the given URL.
func refsContain(refs []ExternalRef, url string) bool {
	for _, ref := range refs {
		if ref.URL == url {
			return true
		}
	}
	return false
}

func TestCheckNoExternalRefsFindsRemoteSubresources(t *testing.T) {
	cases := map[string]struct {
		files map[string]string
		url   string
	}{
		"remote stylesheet": {
			map[string]string{"layout.html": `<link rel="stylesheet" href="https://cdn.example.com/a.css">`},
			"https://cdn.example.com/a.css",
		},
		"google font": {
			map[string]string{"layout.html": `<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter">`},
			"https://fonts.googleapis.com/css2?family=Inter",
		},
		"remote script": {
			map[string]string{"page.html": `<script src="https://cdn.example.com/x.js"></script>`},
			"https://cdn.example.com/x.js",
		},
		"protocol-relative": {
			map[string]string{"page.html": `<script src="//cdn.example.com/x.js"></script>`},
			"//cdn.example.com/x.js",
		},
		"remote image": {
			map[string]string{"home.html": `<img src="http://example.com/logo.png">`},
			"http://example.com/logo.png",
		},
		"iframe": {
			map[string]string{"home.html": `<iframe src="https://www.youtube.com/embed/x"></iframe>`},
			"https://www.youtube.com/embed/x",
		},
		"css font-face": {
			map[string]string{"style.css": `@font-face{src:url(https://fonts.gstatic.com/s/inter.woff2)}`},
			"https://fonts.gstatic.com/s/inter.woff2",
		},
		"css import": {
			map[string]string{"style.css": `@import "https://cdn.example.com/reset.css";`},
			"https://cdn.example.com/reset.css",
		},
		"inline style block": {
			map[string]string{"layout.html": `<style>body{background:url("https://example.com/bg.jpg")}</style>`},
			"https://example.com/bg.jpg",
		},
		"style attribute": {
			map[string]string{"page.html": `<div style="background:url(https://example.com/bg.jpg)"></div>`},
			"https://example.com/bg.jpg",
		},
		"srcset": {
			map[string]string{"home.html": `<img srcset="/a.png 1x, https://cdn.example.com/b.png 2x">`},
			"https://cdn.example.com/b.png",
		},
		"preconnect": {
			map[string]string{"layout.html": `<link rel="preconnect" href="https://fonts.gstatic.com">`},
			"https://fonts.gstatic.com",
		},
		"svg external image": {
			map[string]string{"icon.svg": `<svg><image href="https://example.com/x.png"/></svg>`},
			"https://example.com/x.png",
		},
	}

	for label, tc := range cases {
		refs := CheckNoExternalRefs(writeTemplateDir(t, tc.files))
		if len(refs) == 0 {
			t.Errorf("%s: no external reference detected", label)
			continue
		}
		if !refsContain(refs, tc.url) {
			t.Errorf("%s: expected %q among %v", label, tc.url, refs)
		}
	}
}

// Self-hosted assets and ordinary content must not be flagged; a false positive
// would block a perfectly valid template.
func TestCheckNoExternalRefsAllowsSelfHostedAndContent(t *testing.T) {
	files := map[string]string{
		"layout.html": `<!DOCTYPE html><html><head>
			<link rel="stylesheet" href="/t/style.css">
			<link rel="icon" href="favicon.ico">
			<link rel="canonical" href="https://example.com/page">
			<link rel="alternate" hreflang="en" href="https://example.com/en/">
			<style>@font-face{font-family:Inter;src:url("/t/fonts/inter.woff2") format("woff2")}</style>
			</head><body>
			<a href="https://github.com/holzcloud">Ein ganz normaler Link</a>
			<a href="mailto:info@example.com">Mail</a>
			<img src="/media/1/foto.png" srcset="/media/1/foto.png 1x, /media/1/foto@2x.png 2x">
			<img src="{{.Site.Logo}}">
			<img src="data:image/gif;base64,R0lGODlhAQABAAAAACw=">
			<script src="/assets/htmx.min.js"></script>
			<div style="background:url(/t/bg.png)"></div>
			<svg><use href="#icon"/></svg>
			</body></html>`,
		"page.html":  `{{define "content"}}<article>{{.Page.ContentHTML}}</article>{{end}}`,
		"style.css":  `body{background:url("../img/bg.png")} @import "reset.css";`,
		"reset.css":  `*{margin:0}`,
		"readme.txt": `https://example.com is mentioned in prose only`,
	}

	if refs := CheckNoExternalRefs(writeTemplateDir(t, files)); len(refs) != 0 {
		t.Errorf("false positives on a self-hosted template: %v", refs)
	}
}

// The shipped templates must satisfy the rule they enforce on uploads.
func TestBuiltinTemplatesLoadNothingExternal(t *testing.T) {
	root := filepath.Join("..", "..", "cmd", "holzcloud", "templates")
	for _, sub := range []string{"admin", "public"} {
		dir := filepath.Join(root, sub)
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("template directory %s missing: %v", dir, err)
		}
		if refs := CheckNoExternalRefs(dir); len(refs) != 0 {
			t.Errorf("%s templates load external resources: %v", sub, refs)
		}
	}
}

// The upload path must refuse such an archive outright.
func TestExtractTemplateRejectsExternalResources(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "theme")
	r := buildZip(t,
		zipEntry{name: "layout.html", body: []byte(
			`<html><head><link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter"></head><body>{{template "content" .}}</body></html>`)},
		zipEntry{name: "page.html", body: []byte(`{{define "content"}}x{{end}}`)},
	)

	err := ExtractTemplate(r, int64(r.Len()), dest, 1<<20, nil)
	if err == nil {
		t.Fatal("expected the archive to be rejected")
	}
	if !strings.Contains(err.Error(), "external") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "fonts.googleapis.com") {
		t.Errorf("error should name the offending URL: %v", err)
	}
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Error("the rejected template was installed anyway")
	}
}

// On a fresh install data/templates/ does not exist yet. Extraction places its
// temp directory next to the destination, so without creating that parent first
// the very first upload an operator ever makes fails.
func TestExtractTemplateCreatesMissingTemplatesDirectory(t *testing.T) {
	// Nothing below dataDir exists, exactly like a first run.
	dataDir := t.TempDir()
	dest := filepath.Join(dataDir, "templates", "theme")

	r := buildZip(t, validTemplate()...)
	if err := ExtractTemplate(r, int64(r.Len()), dest, 1<<20, nil); err != nil {
		t.Fatalf("first upload on a fresh install failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "layout.html")); err != nil {
		t.Errorf("template was not installed: %v", err)
	}
}

// A template that bundles its own font is exactly what the rule asks for.
func TestExtractTemplateAcceptsBundledFont(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "theme")
	r := buildZip(t,
		zipEntry{name: "layout.html", body: []byte(
			`<html><head><link rel="stylesheet" href="/t/style.css"></head><body>{{template "content" .}}</body></html>`)},
		zipEntry{name: "page.html", body: []byte(`{{define "content"}}x{{end}}`)},
		zipEntry{name: "style.css", body: []byte(
			`@font-face{font-family:Inter;src:url("/t/fonts/inter.woff2") format("woff2")}`)},
		zipEntry{name: "fonts/inter.woff2", body: []byte("wOF2fake")},
	)

	if err := ExtractTemplate(r, int64(r.Len()), dest, 1<<20, nil); err != nil {
		t.Fatalf("a template bundling its own font was rejected: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "fonts", "inter.woff2")); err != nil {
		t.Errorf("bundled font was not installed: %v", err)
	}
}

// The templates this binary ships have to pass the check an uploaded one has to
// pass. Nothing enforced that: the rule was applied at upload, and the built-in
// themes went in through the repository, past the gate rather than through it.
func TestBuiltinTemplatesHaveNoExternalRefs(t *testing.T) {
	const root = "../../cmd/holzcloud/templates/public"

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read shipped templates: %v", err)
	}

	var themes int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		themes++
		for _, ref := range CheckNoExternalRefs(filepath.Join(root, e.Name())) {
			t.Errorf("shipped template %q loads something from a third party: %s", e.Name(), ref)
		}
	}

	// Without this the test passes just as loudly when the path is wrong.
	if themes == 0 {
		t.Fatalf("no shipped templates found under %s", root)
	}
}
