package plugin

import (
	"strings"
	"testing"
)

func TestAssetTypeKnowsOnlyInertFiles(t *testing.T) {
	type want struct {
		ct       string
		download bool
	}
	known := map[string]want{
		"stil.css":      {"text/css; charset=utf-8", false},
		"a/STIL.CSS":    {"text/css; charset=utf-8", false},
		"bild.png":      {"image/png", false},
		"bild.jpg":      {"image/jpeg", false},
		"bild.jpeg":     {"image/jpeg", false},
		"bild.gif":      {"image/gif", false},
		"bild.webp":     {"image/webp", false},
		"bild.avif":     {"image/avif", false},
		"favicon.ico":   {"image/x-icon", false},
		"fonts/a.woff2": {"font/woff2", false},
		"fonts/a.woff":  {"font/woff", false},
		"daten.json":    {"application/json", false},
		"notiz.txt":     {"text/plain; charset=utf-8", false},
		"logo.svg":      {"image/svg+xml", true},
	}
	for name, w := range known {
		ct, download, ok := AssetType(name)
		if !ok || ct != w.ct || download != w.download {
			t.Errorf("AssetType(%q) = %q, %v, %v; want %q, %v, true", name, ct, download, ok, w.ct, w.download)
		}
	}

	for _, name := range []string{
		"app.js", "APP.JS", "x.mjs", "x.cjs", "x.wasm", "seite.html", "seite.htm",
		"x.xhtml", "feed.xml", "style.xsl", "style.xslt", "Makefile", "noext", "a/b.",
		"app.js.css.js", "", ".js",
	} {
		if ct, _, ok := AssetType(name); ok {
			t.Errorf("AssetType(%q) is known (%q), want unknown", name, ct)
		}
	}
}

func TestAnArchiveCarryingScriptIsRefused(t *testing.T) {
	for _, name := range []string{"app.js", "x.mjs", "seite.html", "feed.xml", "sub/dir/modul.wasm", "APP.JS"} {
		a := archiveFile(t, goodManifest(), map[string][]byte{
			ModuleName:      wasm,
			AssetDir + name: []byte("alert(1)"),
		})
		_, err := lies(t, a)
		if err == nil {
			t.Errorf("an archive with assets/%s was read", name)
			continue
		}
		if !strings.Contains(err.Error(), AssetDir+name) {
			t.Errorf("the error for assets/%s does not name it: %v", name, err)
		}
	}

	a := archiveFile(t, goodManifest(), map[string][]byte{
		ModuleName:                 wasm,
		AssetDir + "stil.css":      []byte("a{}"),
		AssetDir + "bild/logo.svg": []byte("<svg/>"),
		AssetDir + "fonts/a.woff2": []byte("wOF2"),
	})
	p, err := lies(t, a)
	if err != nil {
		t.Fatalf("a package of inert assets: %v", err)
	}
	if len(p.Assets) != 3 {
		t.Errorf("assets: %v", p.Assets)
	}
}
