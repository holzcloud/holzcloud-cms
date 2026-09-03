package tmplmgr

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// ExternalRef is one external subresource reference found in a template.
type ExternalRef struct {
	File string // path relative to the template root
	What string // where it was found, e.g. `img src` or `@import`
	URL  string
}

func (e ExternalRef) String() string {
	return fmt.Sprintf("%s: %s references %s", e.File, e.What, e.URL)
}

// summarizeRefs renders the first few references for an error message, so the
// admin sees what to fix without a wall of text.
func summarizeRefs(refs []ExternalRef) string {
	const max = 3
	parts := make([]string, 0, max)
	for i, ref := range refs {
		if i == max {
			parts = append(parts, fmt.Sprintf("and %d more", len(refs)-max))
			break
		}
		parts = append(parts, ref.String())
	}
	return strings.Join(parts, "; ")
}

// subresourceAttrs lists, per element, the attributes whose value makes the
// browser fetch something. A plain <a href> is missing on purpose: an outbound
// hyperlink is content, not a subresource, and stays allowed.
var subresourceAttrs = map[string][]string{
	"img":    {"src", "srcset"},
	"script": {"src"},
	"source": {"src", "srcset"},
	"iframe": {"src"},
	"frame":  {"src"},
	"embed":  {"src"},
	"object": {"data"},
	"video":  {"src", "poster"},
	"audio":  {"src"},
	"track":  {"src"},
	"input":  {"src"},
	"use":    {"href", "xlink:href"},
	"image":  {"href", "xlink:href"},
}

// fetchingLinkRels are the <link rel> values that cause a fetch. Other rel
// values — canonical, alternate, author, license, next, prev — are metadata and
// may legitimately point at another origin.
var fetchingLinkRels = map[string]bool{
	"stylesheet":       true,
	"preload":          true,
	"modulepreload":    true,
	"prefetch":         true,
	"preconnect":       true,
	"dns-prefetch":     true,
	"prerender":        true,
	"icon":             true,
	"shortcut icon":    true,
	"apple-touch-icon": true,
	"manifest":         true,
	"mask-icon":        true,
}

var (
	cssURLPattern    = regexp.MustCompile(`(?i)url\(\s*['"]?([^'")]+)`)
	cssImportPattern = regexp.MustCompile(`(?i)@import\s+(?:url\(\s*)?['"]([^'"]+)`)
)

// CheckNoExternalRefs walks an extracted template and reports every external
// subresource it references.
//
// Nothing may be loaded from a third party at runtime (see CLAUDE.md). The CSP
// already blocks such a request in the browser, but blocking it there only
// produces a silently broken page; refusing the upload tells the admin what is
// wrong while they can still fix it.
func CheckNoExternalRefs(dir string) []ExternalRef {
	var refs []ExternalRef

	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
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
		rel = filepath.ToSlash(rel)

		switch strings.ToLower(filepath.Ext(path)) {
		case ".html", ".htm":
			refs = append(refs, scanHTML(rel, string(content))...)
		case ".css":
			refs = append(refs, scanCSS(rel, "stylesheet", string(content))...)
		case ".svg":
			// An SVG can pull in an external image or stylesheet just like HTML.
			refs = append(refs, scanHTML(rel, string(content))...)
		}
		return nil
	})

	return refs
}

// CheckExternalRefs scans one markup document for external subresources.
//
// It exists so the media upload can apply the same rule to an SVG: an SVG is
// markup and can pull a stylesheet or an image from another server exactly like
// HTML. The scanner already handled .svg for template archives, but nothing
// outside the archive path ever called it.
func CheckExternalRefs(name, content string) []ExternalRef {
	refs := scanHTML(name, content)
	// A <style> block inside an SVG can carry a url() or an @import.
	return append(refs, scanCSS(name, "stylesheet", content)...)
}

// scanHTML parses a document and collects external subresource references.
// Template actions ({{...}}) survive as literal attribute text, which is fine:
// a value containing an action is not a fixed external URL.
func scanHTML(file, content string) []ExternalRef {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil
	}

	var refs []ExternalRef
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			refs = append(refs, scanElement(file, n)...)
		}
		if n.Type == html.ElementNode && n.Data == "style" && n.FirstChild != nil {
			refs = append(refs, scanCSS(file, "<style>", n.FirstChild.Data)...)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return refs
}

// scanElement checks one element's fetching attributes.
func scanElement(file string, n *html.Node) []ExternalRef {
	var refs []ExternalRef

	check := func(what, value string) {
		for _, candidate := range splitSrcset(value) {
			if isExternalURL(candidate) {
				refs = append(refs, ExternalRef{File: file, What: what, URL: candidate})
			}
		}
	}

	for _, attr := range n.Attr {
		key := strings.ToLower(attr.Key)

		// Inline style attributes can carry url(...).
		if key == "style" {
			refs = append(refs, scanCSS(file, `style=""`, attr.Val)...)
			continue
		}

		if n.Data == "link" && key == "href" {
			if fetchingLinkRels[strings.ToLower(strings.TrimSpace(attrValue(n, "rel")))] {
				check("link href", attr.Val)
			}
			continue
		}

		for _, want := range subresourceAttrs[n.Data] {
			if key == want {
				check(n.Data+" "+key, attr.Val)
			}
		}
	}
	return refs
}

// scanCSS collects url(...) and @import targets.
func scanCSS(file, what, content string) []ExternalRef {
	var refs []ExternalRef
	for _, pattern := range []*regexp.Regexp{cssURLPattern, cssImportPattern} {
		for _, m := range pattern.FindAllStringSubmatch(content, -1) {
			if candidate := strings.TrimSpace(m[1]); isExternalURL(candidate) {
				refs = append(refs, ExternalRef{File: file, What: what, URL: candidate})
			}
		}
	}
	return refs
}

// splitSrcset yields the URLs of a srcset-style value; a plain value yields itself.
func splitSrcset(value string) []string {
	if !strings.Contains(value, ",") {
		return []string{value}
	}
	var out []string
	for _, part := range strings.Split(value, ",") {
		if fields := strings.Fields(strings.TrimSpace(part)); len(fields) > 0 {
			out = append(out, fields[0])
		}
	}
	return out
}

// attrValue returns an element's attribute value, or "" when absent.
func attrValue(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}

// isExternalURL reports whether a subresource URL would leave this origin.
//
// Relative and root-relative paths are fine. So are data: URIs — they carry
// their bytes inline and cause no request. A scheme or a protocol-relative
// "//host" prefix means another origin.
func isExternalURL(raw string) bool {
	u := strings.TrimSpace(raw)
	if u == "" || strings.HasPrefix(u, "#") {
		return false
	}
	// A value built by the template is not a fixed external reference.
	if strings.Contains(u, "{{") {
		return false
	}
	if strings.HasPrefix(u, "//") {
		return true
	}
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "data:") {
		return false
	}
	// A scheme appears before any /, ? or #.
	for i := 0; i < len(lower); i++ {
		switch lower[i] {
		case ':':
			return true
		case '/', '?', '#':
			return false
		}
	}
	return false
}
