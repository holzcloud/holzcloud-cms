package tmplmgr

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

// ScriptRef is one piece of JavaScript found in a template.
type ScriptRef struct {
	File string
	What string
}

func (s ScriptRef) String() string {
	return fmt.Sprintf("%s: %s", s.File, s.What)
}

func summarizeScripts(refs []ScriptRef) string {
	const max = 3
	parts := make([]string, 0, max+1)
	for i, ref := range refs {
		if i == max {
			parts = append(parts, fmt.Sprintf("and %d more", len(refs)-max))
			break
		}
		parts = append(parts, ref.String())
	}
	return strings.Join(parts, "; ")
}

// dataBlockTypes are the <script type> values the browser never executes.
//
// A script element with an unknown type is a data block: the HTML parser hands
// its content to the page as text and never prepares a script from it. That is
// how the shipped themes carry their schema.org description, and an uploaded
// template is entitled to the same.
var dataBlockTypes = map[string]bool{
	"application/ld+json": true,
	"application/json":    true,
	"importmap":           false, // an import map is not data; it drives loading
}

// CheckNoScripts walks an extracted template and reports every piece of
// JavaScript in it.
//
// Templates carry no JavaScript at all. None of the five shipped themes needs
// any — the menu is a checkbox — and the rule is worth stating without
// exceptions: a page that needs a script to be usable is a page that is broken
// for the visitor whose script did not arrive, and "no JavaScript" is a rule an
// author, human or otherwise, can follow without judgement calls.
//
// The extension allow-list already keeps .js files out of an archive. This
// catches the three ways script gets into a page without one: an inline
// <script> block, an event-handler attribute, and a javascript: URL.
func CheckNoScripts(dir string) []ScriptRef {
	var refs []ScriptRef

	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".html", ".htm", ".svg":
		default:
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			rel = d.Name()
		}
		refs = append(refs, scanScripts(filepath.ToSlash(rel), string(content))...)
		return nil
	})

	return refs
}

// scanScripts finds executable script in one markup document.
func scanScripts(file, content string) []ScriptRef {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil
	}

	var refs []ScriptRef
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			refs = append(refs, scanScriptElement(file, n)...)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return refs
}

func scanScriptElement(file string, n *html.Node) []ScriptRef {
	var refs []ScriptRef

	if n.Data == "script" {
		kind := strings.ToLower(strings.TrimSpace(attrValue(n, "type")))
		if !dataBlockTypes[kind] {
			what := "<script> element"
			if kind != "" {
				what = fmt.Sprintf("<script type=%q> element", kind)
			}
			refs = append(refs, ScriptRef{File: file, What: what})
		}
	}

	for _, attr := range n.Attr {
		key := strings.ToLower(attr.Key)

		// on… handlers: onclick, onload, and the several dozen others. Matching
		// the prefix rather than a list means a new one cannot slip through.
		if strings.HasPrefix(key, "on") && len(key) > 2 {
			refs = append(refs, ScriptRef{File: file, What: key + "=\"…\" handler on <" + n.Data + ">"})
			continue
		}

		if isJavaScriptURL(attr.Val) {
			refs = append(refs, ScriptRef{File: file, What: "javascript: URL in " + n.Data + " " + key})
		}
	}
	return refs
}

// isJavaScriptURL reports whether a value is a javascript: URL.
//
// The scheme is compared after stripping whitespace and control characters,
// which browsers ignore: "java\tscript:" and " javascript:" both run.
func isJavaScriptURL(raw string) bool {
	var b strings.Builder
	for _, r := range raw {
		if r > ' ' {
			b.WriteRune(r)
		}
	}
	return strings.HasPrefix(strings.ToLower(b.String()), "javascript:")
}
