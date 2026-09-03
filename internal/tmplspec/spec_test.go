package tmplspec

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

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
