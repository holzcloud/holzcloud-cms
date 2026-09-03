package web

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	formTagPattern   = regexp.MustCompile(`(?is)<form\b[^>]*>`)
	buttonTagPattern = regexp.MustCompile(`(?is)<(button|a)\b[^>]*>`)
	attrPattern      = regexp.MustCompile(`(?is)\b([a-z-]+)\s*=\s*"([^"]*)"`)
)

func attrs(tag string) map[string]string {
	out := map[string]string{}
	for _, m := range attrPattern.FindAllStringSubmatch(tag, -1) {
		out[strings.ToLower(m[1])] = m[2]
	}
	return out
}

func adminTemplateFiles(t *testing.T) map[string]string {
	t.Helper()
	dir := filepath.Join("..", "..", "cmd", "holzcloud", "templates", "admin")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read admin templates: %v", err)
	}
	files := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		files[e.Name()] = string(content)
	}
	return files
}

// Every state-changing action must work without JavaScript.
//
// htmx is progressive enhancement, not a dependency: a form carrying only
// hx-post silently does nothing when the script is unavailable — which is
// exactly how page delete, publish toggle and domain removal became inert.
// A plain method/action pair keeps them working either way.
func TestEveryHxPostFormAlsoSubmitsWithoutJS(t *testing.T) {
	for name, content := range adminTemplateFiles(t) {
		for _, tag := range formTagPattern.FindAllString(content, -1) {
			a := attrs(tag)
			if _, ok := a["hx-post"]; !ok {
				continue
			}
			if !strings.EqualFold(a["method"], "post") || a["action"] == "" {
				t.Errorf("%s: form has hx-post but no method/action fallback:\n  %s", name, strings.TrimSpace(tag))
			}
		}
	}
}

// A control that changes state must still work with the script gone.
//
// There are exactly two shapes that satisfy that. A <button type="submit">
// submits its enclosing form on a plain click, whatever htmx does or does not
// do — that is the shape a fragment uses, because the form is in the template
// that includes it and not in the fragment's own file. Anything else — a bare
// <a hx-post>, a <button type="button"> — has no path at all without the
// script, so it has to sit in a file that visibly contains the POST form.
func TestNoStateChangingControlOutsideAForm(t *testing.T) {
	for name, content := range adminTemplateFiles(t) {
		for _, tag := range buttonTagPattern.FindAllString(content, -1) {
			a := attrs(tag)
			for _, verb := range []string{"hx-post", "hx-put", "hx-delete", "hx-patch"} {
				if _, ok := a[verb]; !ok {
					continue
				}
				if strings.EqualFold(a["type"], "submit") {
					// Submits the enclosing form on a plain click. A
					// formaction would send it somewhere else, so that has to
					// be a POST target too — but no template uses one, and if
					// one ever does this check should be revisited.
					if _, ok := a["formaction"]; !ok {
						continue
					}
				}
				if !strings.Contains(content, `method="POST"`) {
					t.Errorf("%s: <%s> with %s and no non-JS path:\n  %s",
						name, strings.Split(tag, " ")[0][1:], verb, strings.TrimSpace(tag))
				}
			}
		}
	}
}

// The vendored htmx asset must be the real library.
//
// It was a 47-byte placeholder comment from the commit that introduced it,
// which made every hx-* attribute in the admin inert in a browser while the
// source looked perfectly correct. Nothing in the UI surfaced that, so only an
// assertion catches it coming back.
//
// See cmd/holzcloud/assets/VENDOR.md for provenance and the update procedure.
func TestVendoredHtmxIsTheRealLibrary(t *testing.T) {
	path := filepath.Join("..", "..", "cmd", "holzcloud", "assets", "htmx.min.js")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("htmx asset missing entirely: %v", err)
	}

	// The minified library is around 50 kB; anything tiny is a stub.
	const minPlausibleSize = 20000
	if len(content) < minPlausibleSize {
		t.Fatalf("htmx.min.js is %d bytes, which cannot be the library — every hx-* "+
			"attribute in the admin is inert. Content: %q",
			len(content), strings.TrimSpace(string(content)))
	}

	// Spot-check that it is htmx and not some other minified bundle.
	head := string(content[:min(len(content), 200)])
	if !strings.Contains(head, "htmx") {
		t.Errorf("htmx.min.js does not look like htmx: %q", head)
	}
	for _, marker := range []string{"hx-swap", "hx-target", "hx-confirm", "hx-headers"} {
		if !strings.Contains(string(content), marker) {
			t.Errorf("htmx.min.js does not handle %s, which the admin templates use", marker)
		}
	}
}

// Jedes vollständige Dokument im Admin-Bereich muss ein Zeichen nennen.
//
// Tut es das nicht, fordert der Browser von sich aus /favicon.ico an und
// bekommt auf jedem einzelnen Seitenaufruf einen 404 — im Protokoll des
// Servers und in der Konsole der Betreiberin. Das war lange so und ist beim
// Durchsehen der Vorlagen aufgefallen, nicht beim Benutzen: ein 404 auf ein
// Bild sieht man der Seite nicht an.
func TestEveryAdminDocumentNamesAFavicon(t *testing.T) {
	dir := filepath.Join("..", "..", "cmd", "holzcloud", "templates", "admin")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read admin templates: %v", err)
	}

	var geprueft int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		// Nur vollständige Dokumente. Ein Seitenfragment hat keinen Kopf, in
		// den ein Zeichen gehörte.
		if !strings.Contains(strings.ToLower(string(body)), "<!doctype") {
			continue
		}
		geprueft++
		if !strings.Contains(string(body), `rel="icon"`) {
			t.Errorf("%s ist ein vollständiges Dokument ohne <link rel=\"icon\"> — "+
				"jeder Aufruf erzeugt damit einen 404 auf /favicon.ico", e.Name())
		}
	}

	// Ohne diese Zeile wäre der Test grün, wenn die Vorlagen umbenannt oder
	// verschoben würden und die Schleife gar nichts mehr fände.
	if geprueft < 5 {
		t.Fatalf("nur %d vollständige Dokumente gefunden; die Suche greift nicht mehr", geprueft)
	}
}
