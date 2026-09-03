package media

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// DefaultSizes tells the browser how wide the image will actually be laid out.
//
// Without it a browser assumes the full viewport width and picks the largest
// candidate, which would make the whole srcset pointless. The shipped themes cap
// the text column at 70rem, so this matches them; a theme that wants something
// else can set its own sizes attribute and it will be kept.
const DefaultSizes = "(max-width: 70rem) 100vw, 70rem"

// MakeResponsive rewrites the <img> tags of a document to use the scaled copies.
//
// An editor writes `![Werkstatt](/media/1/foo.jpg)` and gets one file, whatever
// size it happened to be. This is where that single reference turns into a
// candidate list, explicit dimensions and lazy loading — without the editor
// having to know any of it, and without changing what is stored.
//
// A parser, not a regexp: an attribute can be quoted three ways or spread over
// lines, and rewriting the wrong thing here would corrupt a published page.
func MakeResponsive(pageHTML string, idx Index) string {
	if len(idx) == 0 || !strings.Contains(pageHTML, "<img") {
		return pageHTML
	}

	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(pageHTML), body)
	if err != nil {
		return pageHTML
	}

	changed := false
	for _, n := range nodes {
		if rewriteImages(n, idx) {
			changed = true
		}
	}
	if !changed {
		return pageHTML
	}

	var out strings.Builder
	for _, n := range nodes {
		if err := html.Render(&out, n); err != nil {
			return pageHTML
		}
	}
	return out.String()
}

// rewriteImages walks a subtree and reports whether anything was touched.
func rewriteImages(n *html.Node, idx Index) bool {
	changed := false
	if n.Type == html.ElementNode && n.Data == "img" {
		changed = rewriteImage(n, idx)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if rewriteImages(c, idx) {
			changed = true
		}
	}
	return changed
}

func rewriteImage(n *html.Node, idx Index) bool {
	have := map[string]bool{}
	src := ""
	for _, a := range n.Attr {
		have[a.Key] = true
		if a.Key == "src" {
			src = a.Val
		}
	}
	// An author who already wrote a srcset by hand knows better than this pass.
	if have["srcset"] || src == "" {
		return false
	}

	set, ok := idx[mediaPath(src)]
	if !ok || set.Width <= 0 {
		return false
	}

	// The address is brought up to date first. A page saved before a crop still
	// carries the old one, and that address is cached for a year as immutable —
	// so without this the crop appears to have done nothing, for everyone who
	// had already seen the picture.
	if versioned := set.VersionedPath(); versioned != src {
		for k := range n.Attr {
			if n.Attr[k].Key == "src" {
				n.Attr[k].Val = versioned
			}
		}
	}

	// Dimensions come first: they let the browser reserve the space before the
	// bytes arrive, which is what stops the text jumping while a page loads.
	// They are useful even when there is no variant to offer.
	if !have["width"] && !have["height"] && set.Height > 0 {
		n.Attr = append(n.Attr,
			html.Attribute{Key: "width", Val: strconv.Itoa(set.Width)},
			html.Attribute{Key: "height", Val: strconv.Itoa(set.Height)})
	}
	if !have["loading"] {
		n.Attr = append(n.Attr, html.Attribute{Key: "loading", Val: "lazy"})
	}
	if !have["decoding"] {
		n.Attr = append(n.Attr, html.Attribute{Key: "decoding", Val: "async"})
	}

	if srcset := set.SrcSet(); srcset != "" {
		n.Attr = append(n.Attr, html.Attribute{Key: "srcset", Val: srcset})
		if !have["sizes"] {
			n.Attr = append(n.Attr, html.Attribute{Key: "sizes", Val: DefaultSizes})
		}
	}
	// An image with no description is a hole for anyone using a screen reader.
	// The stored description is the best guess available at render time.
	if !have["alt"] && set.AltText != "" {
		n.Attr = append(n.Attr, html.Attribute{Key: "alt", Val: set.AltText})
	}
	return true
}

// mediaPath normalises a same-origin media reference to its index key.
func mediaPath(src string) string {
	if !strings.HasPrefix(src, "/media/") {
		return ""
	}
	if i := strings.IndexAny(src, "?#"); i >= 0 {
		src = src[:i]
	}
	return src
}
