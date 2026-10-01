package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"regexp"
	"testing"
)

// The holzcloud theme links its stylesheet as /t/style.css?v=<hash>, and the
// hash must be the first twelve hex digits of the SHA-256 of style.css.
//
// A versioned address is answered with `max-age=31536000, immutable`, so a
// browser keeps it for a year. That is the point: before the version, the
// stylesheet was kept for an hour, and the release that turned the phone menu
// into a <details> reached visitors as new HTML on top of the old stylesheet —
// a "Menü" toggle and the whole wrapped menu below it at once. But a version
// that does not change with the file would be worse than none, because the old
// stylesheet would then stay for a year. So this test fails the moment
// style.css changes without the link: rebuild the value with
//
//	sha256sum cmd/holzcloud/templates/public/holzcloud/style.css | cut -c1-12
func TestHolzcloudStylesheetVersionMatchesContent(t *testing.T) {
	css, err := fs.ReadFile(staticFS, "templates/public/holzcloud/style.css")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := fs.ReadFile(staticFS, "templates/public/holzcloud/layout.html")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(css)
	want := hex.EncodeToString(sum[:])[:12]

	m := regexp.MustCompile(`href="/t/style\.css\?v=([0-9a-f]+)"`).FindSubmatch(layout)
	if m == nil {
		t.Fatalf("layout.html does not link /t/style.css?v=<hash>; want v=%s", want)
	}
	if got := string(m[1]); got != want {
		t.Errorf("layout.html links style.css?v=%s, but the file's hash is %s: "+
			"a browser would keep the old stylesheet for a year", got, want)
	}
}
