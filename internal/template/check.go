package template

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Problem is one reason a template will not work.
type Problem struct {
	// File is the template file, relative to the archive root.
	File string
	// Line is the line it was found on, or 0 when the failure has no line.
	Line int
	// Message says what is wrong, in the terms the author used.
	Message string
	// Hint is what to do about it, and is empty when nothing useful can be
	// said. For a misspelt field it lists the field names that do exist —
	// which is the whole difference between "wrong" and "fixable".
	Hint string
}

func (p Problem) String() string {
	loc := p.File
	if p.Line > 0 {
		loc = fmt.Sprintf("%s:%d", p.File, p.Line)
	}
	out := loc + ": " + p.Message
	if p.Hint != "" {
		out += "\n    " + p.Hint
	}
	return out
}

// Summarize renders problems for a one-line error message.
func Summarize(problems []Problem) string {
	const max = 3
	parts := make([]string, 0, max+1)
	for i, p := range problems {
		if i == max {
			parts = append(parts, fmt.Sprintf("and %d more", len(problems)-max))
			break
		}
		parts = append(parts, strings.ReplaceAll(p.String(), "\n    ", " — "))
	}
	return strings.Join(parts, "; ")
}

// Check renders a template the way the server will and reports what breaks.
//
// Until this existed the upload checked only that layout.html and page.html
// were present. A template whose expressions did not compile — or that named a
// field which does not exist — was accepted, and the failure surfaced as a 500
// on the next visitor's request, on the live site, with nothing said to the
// person who had just uploaded it. That is survivable for someone who can read
// the server log and hopeless for an agent whose only feedback is the upload's
// answer.
//
// theme is the archive being checked. fallback is the default theme, whose
// views fill in for the ones an archive leaves out; pass nil to skip that part.
func Check(theme fs.FS, fallback fs.FS) []Problem {
	var problems []Problem

	for _, name := range []string{layoutFile, "page.html"} {
		if _, err := fs.Stat(theme, name); err != nil {
			problems = append(problems, Problem{
				File:    name,
				Message: "required file is missing",
				Hint:    "an archive must contain at least layout.html and page.html",
			})
		}
	}
	// Without the layout there is nothing to render the views into, and every
	// further message would only repeat that.
	layout, err := fs.ReadFile(theme, layoutFile)
	if err != nil {
		return problems
	}

	problems = append(problems, checkLayoutStructure(string(layout))...)

	// The views the archive brings itself, plus — through the fallback — the
	// ones it leaves to the default theme. Both combinations really occur at
	// runtime, and both render through *this* layout.
	for _, view := range viewFiles {
		source, own := theme, true
		if _, err := fs.Stat(theme, view); err != nil {
			if fallback == nil {
				continue
			}
			source, own = fallback, false
		}
		body, err := fs.ReadFile(source, view)
		if err != nil {
			continue
		}
		problems = append(problems, checkView(view, string(layout), string(body), own)...)
		if own {
			problems = append(problems, checkFormContract(view, string(body))...)
		}
	}

	return dedupe(problems)
}

// dedupe collapses the same fault reported from several views.
//
// A mistake in layout.html breaks every view, and every view is rendered — so
// without this a single typo is reported seven times and the other six faults
// scroll off the end of the report. One fault, one line.
func dedupe(problems []Problem) []Problem {
	type key struct {
		file, message string
		line          int
	}
	seen := make(map[key]bool, len(problems))

	out := problems[:0]
	for _, p := range problems {
		k := key{p.File, p.Message, p.Line}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
	}
	return out
}

// formContracts are the fields the server reads out of a template's forms.
//
// Two views are not just presentation: they submit to a fixed endpoint under
// fixed field names, and the names are not guessable. A gate.html whose input
// is called "password" instead of "passwort" renders perfectly, looks right,
// and cannot unlock anything — the render check sees nothing wrong because
// nothing is wrong until someone types into it.
var formContracts = map[string][]struct{ needle, what string }{
	"gate.html": {
		{`/freischalten`, `a form posting to /freischalten`},
		{`name="passwort"`, `the password field, which must be named "passwort"`},
		{`name="seite"`, `the page field, which must be named "seite" and carry {{.Page.Slug}}`},
	},
	"search.html": {
		{`/suche`, `a form submitting to /suche`},
		{`name="q"`, `the query field, which must be named "q"`},
	},
}

// checkFormContract verifies a view still submits what the server reads.
func checkFormContract(view, body string) []Problem {
	// Single quotes are as valid as double ones in HTML, and an author who
	// writes name='q' has done nothing wrong.
	normalized := strings.ReplaceAll(body, "'", `"`)

	var problems []Problem
	for _, want := range formContracts[view] {
		if !strings.Contains(normalized, want.needle) {
			problems = append(problems, Problem{
				File:    view,
				Message: "is missing " + want.what,
				Hint:    "the server reads these names; a form that renames them submits nothing it can use",
			})
		}
	}
	return problems
}

// CheckDir is Check against a directory on disk.
func CheckDir(dir string, fallback fs.FS) []Problem {
	return Check(os.DirFS(dir), fallback)
}

// contentAction finds the layout's inclusion of the view's body.
//
// Both spellings count. {{block "content" .}}…{{end}} defines the block and
// executes it in one go, and the view's own definition replaces the default
// because the loader parses the view after the layout. Accepting only
// {{template}} would have rejected a layout that works.
var contentAction = regexp.MustCompile(`{{-?\s*(?:template|block)\s+"content"`)

// checkLayoutStructure catches the mistakes that produce a page which renders
// without any error and is useless.
func checkLayoutStructure(layout string) []Problem {
	var problems []Problem

	// A layout that never pulls in "content" renders every page as an empty
	// frame. Nothing errors: the views define a block the layout never asks
	// for, so the site comes up, looks plausible, and has no content on it.
	if !contentAction.MatchString(layout) {
		problems = append(problems, Problem{
			File:    layoutFile,
			Message: "never includes the page body",
			Hint:    `put {{template "content" .}} where the page content belongs`,
		})
	}

	if !strings.Contains(strings.ToLower(layout), "<html") {
		problems = append(problems, Problem{
			File:    layoutFile,
			Message: "is not a whole HTML document",
			Hint:    "it is the outermost file and must carry <html>, <head> and <body> itself",
		})
	}

	return problems
}

// checkView parses and renders one view inside the layout.
func checkView(view, layout, body string, own bool) []Problem {
	// The same construction the loader uses, so a template that passes here
	// cannot fail there for a reason this check never saw.
	sample := SampleData()
	set := template.New("").Funcs(funcMap(sample.Site.Locale, sample.Site.TimeZone))

	for _, f := range []struct{ name, content string }{
		{layoutFile, layout},
		{view, body},
	} {
		if _, err := set.New(f.name).Parse(f.content); err != nil {
			return []Problem{parseProblem(f.name, err)}
		}
	}

	if own && !definesContent(set) {
		return []Problem{{
			File:    view,
			Message: `does not define the "content" block`,
			Hint:    `wrap the whole file in {{define "content"}} … {{end}}`,
		}}
	}

	// Both fixtures, because they fail differently. The full one catches a
	// misspelt field; the minimal one catches the far more common mistake of
	// reaching through something that is allowed to be absent.
	full := renderProblem(set, SampleData())
	empty := renderProblem(set, MinimalData())

	var problems []Problem
	add := func(p *Problem, note string) {
		if p == nil {
			return
		}
		if note != "" {
			p.Message = note + ": " + p.Message
		}
		// Only worth saying when the failure is reported against a file the
		// author did not write. When it already points at layout.html, adding
		// "the fault is in layout.html" says nothing.
		if !own && p.File != layoutFile {
			p.Hint = strings.TrimSpace(p.Hint +
				" (this view came from the default theme, so the fault is in layout.html)")
		}
		problems = append(problems, *p)
	}

	switch {
	case full != nil && empty != nil && sameProblem(*full, *empty):
		// The template is wrong regardless of the data. Saying so twice, once
		// per fixture, only makes the real count harder to see.
		add(full, "")
	default:
		add(full, "on a page with everything filled in")
		add(empty, "on a page with every optional field empty")
	}
	return problems
}

// renderProblem executes the set and returns the failure, or nil.
func renderProblem(set *template.Template, data PageData) *Problem {
	err := set.ExecuteTemplate(io.Discard, layoutFile, data)
	if err == nil {
		return nil
	}
	p := executeProblem("", err)
	return &p
}

func sameProblem(a, b Problem) bool {
	return a.File == b.File && a.Line == b.Line && a.Message == b.Message
}

// definesContent reports whether the parsed set has a non-empty "content".
//
// Checking the parse tree rather than the file text: a {{define "content"}}
// inside an HTML comment does not count, and one written across several lines
// does.
func definesContent(set *template.Template) bool {
	t := set.Lookup("content")
	return t != nil && t.Tree != nil && t.Tree.Root != nil && len(t.Tree.Root.Nodes) > 0
}

var (
	// template: layout.html:12: function "foo" not defined
	parseErrPattern = regexp.MustCompile(`^template: ([^:]+):(\d+):\s*(.*)$`)
	// template: layout.html:5:12: executing "layout.html" at <.Page.Titel>: …
	execErrPattern = regexp.MustCompile(`^template: ([^:]+):(\d+)(?::\d+)?:\s*executing\s+\S+\s+at\s+<([^>]*)>:\s*(.*)$`)
	// … can't evaluate field Titel in type template.PageContent
	fieldErrPattern = regexp.MustCompile(`can't evaluate field (\w+) in type [\w.*]*?(\w+)$`)
	// … nil pointer evaluating *template.PageLink.Title
	nilErrPattern = regexp.MustCompile(`nil pointer evaluating [\w.*]*?(\w+)\.(\w+)$`)
)

func parseProblem(file string, err error) Problem {
	msg := err.Error()
	if m := parseErrPattern.FindStringSubmatch(msg); m != nil {
		line, _ := strconv.Atoi(m[2])
		p := Problem{File: m[1], Line: line, Message: m[3]}
		if strings.Contains(m[3], "not defined") {
			p.Hint = "available helpers: " + strings.Join(HelperNames(), ", ")
		}
		return p
	}
	return Problem{File: file, Message: msg}
}

func executeProblem(file string, err error) Problem {
	msg := err.Error()
	m := execErrPattern.FindStringSubmatch(msg)
	if m == nil {
		return Problem{File: file, Message: msg}
	}

	line, _ := strconv.Atoi(m[2])
	expr, detail := m[3], m[4]
	p := Problem{File: m[1], Line: line, Message: detail + " (at " + expr + ")"}

	if f := fieldErrPattern.FindStringSubmatch(detail); f != nil {
		if fields := FieldsOf(f[2]); len(fields) > 0 {
			p.Hint = f[2] + " has: " + strings.Join(fields, ", ")
		}
	}
	if n := nilErrPattern.FindStringSubmatch(detail); n != nil {
		p.Hint = n[1] + " is empty on this page — guard it with {{with " +
			guessPath(expr, n[2]) + "}} … {{end}}"
	}
	return p
}

// guessPath trims the final field off an expression so the hint names the thing
// to guard rather than the field that blew up: .Page.Prev, not .Page.Prev.Title.
func guessPath(expr, field string) string {
	if trimmed := strings.TrimSuffix(expr, "."+field); trimmed != expr && trimmed != "" {
		return trimmed
	}
	return expr
}

// FieldsOf returns the exported field names of a type in the data contract, or
// nil when no such type is reachable from PageData.
//
// This is the reply to a typo. "can't evaluate field Titel in type
// template.PageContent" tells an author that something is wrong; the list of
// names that do exist tells them what to write instead, without having to leave
// the error message and go read the source.
func FieldsOf(typeName string) []string {
	var found []string
	seen := map[reflect.Type]bool{}

	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || seen[t] || !ownType(t) || found != nil {
			return
		}
		seen[t] = true

		if t.Name() == typeName {
			for i := 0; i < t.NumField(); i++ {
				if f := t.Field(i); f.IsExported() {
					found = append(found, f.Name)
				}
			}
			return
		}
		for i := 0; i < t.NumField(); i++ {
			walk(t.Field(i).Type)
		}
	}
	walk(reflect.TypeOf(PageData{}))

	return found
}

// ownType reports whether a struct type belongs to this package's data
// contract. time.Time and menu.MenuNode are reachable from PageData but are not
// part of what this package documents or fills in.
func ownType(t reflect.Type) bool {
	return strings.HasSuffix(t.PkgPath(), "internal/template")
}

// HelperNames lists the template helper functions, sorted.
func HelperNames() []string {
	var names []string
	for name := range funcMap("de", "Europe/Berlin") {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
