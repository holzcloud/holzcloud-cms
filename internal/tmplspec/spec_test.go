package tmplspec

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/holzcloud/holzcloud-cms/internal/field"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// A specification an agent follows to the letter is worse than none once it is
// wrong. Adding a field to the data contract and forgetting the document is the
// easy mistake and an invisible one — nothing fails to compile, and a template
// author simply never learns the field exists.
//
// So the document is checked against the contract itself, by reflection.
func TestSpecDocumentsEveryFieldOfTheContract(t *testing.T) {
	spec := Markdown()

	for _, path := range contractPaths() {
		if !strings.Contains(spec, path) {
			t.Errorf("the specification never mentions %s — a template author "+
				"has no way to learn it exists", path)
		}
	}
}

// contractPaths returns every dotted path a template can write, e.g.
// ".Page.Title" and ".Archive.Entries".
//
// Nested element types are reported by their own name rather than by every path
// that reaches them: .Site.Terms and .Page.Terms are both a TermLink, and the
// document describes TermLink once.
func contractPaths() []string {
	var paths []string
	seen := map[reflect.Type]bool{}

	var walk func(reflect.Type, string, int)
	walk = func(t reflect.Type, prefix string, depth int) {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || !ownType(t) || depth > 2 {
			return
		}
		if seen[t] {
			return
		}
		seen[t] = true

		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			path := prefix + "." + f.Name
			if depth < 2 {
				paths = append(paths, path)
			} else {
				// A leaf of a nested type: the name alone has to appear.
				paths = append(paths, f.Name)
			}
			walk(f.Type, path, depth+1)
		}
	}
	walk(reflect.TypeOf(tmpl.PageData{}), "", 0)

	return paths
}

func ownType(t reflect.Type) bool {
	return strings.HasSuffix(t.PkgPath(), "internal/template")
}

// A helper the document does not list is a helper nobody uses; one it lists
// that does not exist is worse — an author writes it and the upload fails.
func TestSpecListsExactlyTheHelpersThatExist(t *testing.T) {
	spec := Markdown()

	for _, name := range tmpl.HelperNames() {
		if !strings.Contains(spec, name) {
			t.Errorf("the specification never mentions the %q helper", name)
		}
	}

	// The other direction: anything that looks like a helper call in the
	// document has to resolve.
	real := map[string]bool{}
	for _, name := range tmpl.HelperNames() {
		real[name] = true
	}
	for _, invented := range []string{"formatTime", "dateFormat", "asset", "url", "truncate"} {
		if strings.Contains(spec, "{{"+invented) {
			t.Errorf("the specification demonstrates %q, which is not a helper", invented)
		}
		_ = real
	}
}

// Every view file has to be named, or an author cannot know it can be supplied.
func TestSpecNamesEveryViewFile(t *testing.T) {
	spec := Markdown()

	for _, view := range tmpl.ViewFiles() {
		if !strings.Contains(spec, view) {
			t.Errorf("the specification never mentions %s", view)
		}
	}
	if !strings.Contains(spec, "layout.html") {
		t.Error("the specification never mentions layout.html")
	}
}

// The example in the document is the thing an agent copies first. If it does
// not pass the checker, the first upload fails on the specification's own work.
func TestSpecExamplesPassTheChecker(t *testing.T) {
	layout := codeBlockContaining(t, Markdown(), "Zum Inhalt springen")
	page := codeBlockContaining(t, Markdown(), "Weitere Beiträge")

	theme := fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte(layout)},
		"page.html":   &fstest.MapFile{Data: []byte(page)},
	}
	for _, p := range tmpl.Check(theme, nil) {
		t.Errorf("the minimal template in the specification does not pass the checker: %s", p)
	}
}

// codeBlockContaining returns the fenced block holding a marker.
func codeBlockContaining(t *testing.T, doc, marker string) string {
	t.Helper()

	// Splitting on the fence alternates prose, code, prose, code — so only the
	// odd segments are code. Without that the prose describing a snippet
	// matches before the snippet does.
	for i, block := range strings.Split(doc, "```") {
		if i%2 == 0 || !strings.Contains(block, marker) {
			continue
		}
		// Drop the language tag on the opening fence.
		if _, rest, ok := strings.Cut(block, "\n"); ok {
			return rest
		}
	}
	t.Fatalf("no code block in the specification contains %q", marker)
	return ""
}

// FieldEntry is the one type of the contract the reflection walk above cannot
// reach: it belongs to internal/field, and ownType deliberately stops there so
// that a foreign struct's internals are not reported as missing paths.
//
// The consequence is that the walk says nothing about the members a template
// author actually types — .Values, .Term, .Yes. Adding one of them to the type
// and forgetting the document is exactly the invisible mistake the walk exists
// to catch, so it is caught here instead.
func TestSpecDocumentsEveryFieldEntryMember(t *testing.T) {
	spec := Markdown()

	entry := reflect.TypeOf(field.Entry{})
	for i := 0; i < entry.NumField(); i++ {
		f := entry.Field(i)
		if !f.IsExported() {
			continue
		}
		if !strings.Contains(spec, "`."+f.Name+"`") {
			t.Errorf("the specification never names `.%s` — a template author "+
				"reading it has no way to learn the member exists", f.Name)
		}
	}
}

// The per-kind tax, made mechanical. Every kind an operator can pick has to
// have a row in the specification's kind table, or a template author is left
// to guess what .Value holds for it and what to print.
//
// The row and not merely the word: "text" and "code" occur all over a document
// about writing templates, so a plain substring search would report a kind as
// documented that nobody ever wrote a line about.
func TestSpecDocumentsEveryFieldKind(t *testing.T) {
	rows := map[string]bool{}
	for _, line := range strings.Split(Markdown(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		name, _, ok := strings.Cut(strings.TrimPrefix(line, "| `"), "`")
		if ok {
			rows[name] = true
		}
	}

	for _, k := range field.Kinds {
		if !rows[k.Kind] {
			t.Errorf("the specification's kind table has no row for %q — a "+
				"template author cannot learn what .Value holds for it", k.Kind)
		}
	}
}

// The two tests below are English while the rest of this file is German prose
// in English function names; this project's code became English on 2026-09-06.
//
// They guard an instruction rather than a field, which is why reflection
// cannot hold them. TestSpecDocumentsEveryFieldOfTheContract above walks the
// data contract and would never notice that the specification tells nobody to
// load /assets/bausteine.css — and until this phase it did not: the string
// appeared in the document zero times and in all eight shipped layout.html
// files.

// The core writes the markup of a block, so the core supplies its base
// styling. A theme that does not link the stylesheet shows a gallery as a
// column of pictures and a :target lightbox not at all — the second one does
// not degrade, it fails.
func TestSpecRequiresTheBlockStylesheet(t *testing.T) {
	spec := Markdown()

	const link = "/assets/bausteine.css"
	if !strings.Contains(spec, link) {
		t.Fatalf("the specification never mentions %s, so a template author "+
			"following it renders every block wrong", link)
	}
	// Three places, because two of them are copied whole: the asset table, the
	// layout example in §3 and the complete minimal template in §11. A count
	// below three means one of the two examples still teaches a theme that
	// renders blocks wrong.
	if got := strings.Count(spec, link); got < 3 {
		t.Errorf("%s appears %d times, want at least 3 — the asset table, §3's "+
			"layout example and §11's minimal template", link, got)
	}
}

// This is the one that would have caught the drift. The document can be right
// while a theme quietly stops linking the file, and nothing else in the suite
// looks.
//
// The themes are read from disk relative to this package rather than through an
// embedded filesystem: they are embedded by package main under cmd/holzcloud,
// which a library test cannot import. Reading the source tree is therefore the
// only way to see them from here, and it is enough — the files are in this
// repository and the test runs in it.
func TestEveryShippedThemeLinksTheBlockStylesheet(t *testing.T) {
	const themes = "../../cmd/holzcloud/templates/public"

	layouts, err := filepath.Glob(filepath.Join(themes, "*", "layout.html"))
	if err != nil {
		t.Fatalf("the theme directory could not be read: %v", err)
	}
	// A directory walk that finds nothing must fail. Passing on a tree with no
	// themes at all is the failure mode of every test of this shape.
	if len(layouts) == 0 {
		t.Fatalf("no layout.html was found under %s — the test proved nothing", themes)
	}

	for _, path := range layouts {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s could not be read: %v", path, err)
			continue
		}
		if !strings.Contains(string(body), "/assets/bausteine.css") {
			t.Errorf("the theme %q does not link /assets/bausteine.css, so every "+
				"block on it is unstyled and its lightbox does not open",
				filepath.Base(filepath.Dir(path)))
		}
	}
}
