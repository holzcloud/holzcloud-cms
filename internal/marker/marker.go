// Package marker is the one place that knows what it costs to expand a marker
// by replacing a string.
//
// Four mechanisms in this program write a marker into stored HTML and resolve
// it at delivery: the album gallery (internal/block), the text snippet
// (internal/snippet), the late-resolved word (internal/block/words.go) and the
// website's value (internal/value). The first three replace with a regular
// expression over the whole document, which is fast, simple, and blind to where
// in the document the match sits. The value tokenizes, because it may stand in
// a link address and has to be written differently there; this package's part
// in it is RestoreEncodedValueMarkers, which the save path calls.
//
// # Why that blindness is a hole
//
// The Phase 11 security audit measured it (UF-1, 2026-09-08). Nothing stops an
// editor typing a marker into a text block, and both halves of the Markdown
// pipeline pass it through untouched:
//
//	in : <a href="/x?q=[[album:sommer-2025:1]]">klick</a>
//	out: <p><a href="/x?q=[[album:sommer-2025:1]]" rel="nofollow">klick</a></p>
//
// The marker is now inside an ATTRIBUTE VALUE. Expanding it there splices a
// gallery's markup into the middle of an href: the expansion's first quotation
// mark ends the attribute, and what follows is markup nobody wrote. The audit
// tried to make that reach further and could not — everything after the break
// is renderer-generated, alt text and captions go through html.EscapeString,
// bluemonday drops an href containing a space, and script-src 'self' forbids
// inline script regardless. Broken markup on one page, done by an authenticated
// editor to their own website. Not a way in; still not something to leave
// standing, because a page that renders wrongly is a page the operator has to
// debug.
//
// A marker whose expansion is plain TEXT is not affected, because the text is
// escaped on the way in and an attribute value is a perfectly good place for
// escaped text. That is why ResolveWords does not call this and does not need
// to: it escapes.
//
// # What this package does about it
//
// OnlyInText deletes the matches that are not in text, and nothing else. The
// substitution that follows is then blind in a document where blindness cannot
// hurt: every match left standing is a text node.
//
// It tokenizes rather than parses. html.Parse would build a tree and re-render
// it, which is a second pass over a body that media.MakeResponsive already
// parses once on the same route, and a re-render rewrites bytes this function
// has no business touching. A tokenizer hands back the raw bytes of each token,
// their concatenation is the input exactly (TestTheTokensAreTheDocument), and
// so the output differs from the input only where a match was removed.
package marker

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// encodedValueMarker is what goldmark writes for [[wert:key]] inside a link
// destination: the brackets percent-encoded (upper-case hex from goldmark; the
// lower-case spelling is accepted too, because an editor's raw HTML may carry
// it). The key alphabet is internal/value's.
var encodedValueMarker = regexp.MustCompile(`%5[Bb]%5[Bb]wert:([a-z0-9][a-z0-9_-]*)%5[Dd]%5[Dd]`)

// RestoreEncodedValueMarkers writes every percent-encoded value marker back in
// its literal spelling.
//
// goldmark percent-encodes the brackets of a marker it finds in a Markdown link
// destination or a bare autolink, and bluemonday keeps them, so a value marker
// in a link is stored as %5B%5Bwert:key%5D%5D. Delivery could look for both
// spellings, and that costs three and a half times the single guard on every
// page view (33-RESEARCH.md F2). Restoring the literal spelling once, here, on
// the save path, keeps delivery on the one prefix internal/value.Has asks for.
//
// A literal [[wert:key]] inside a double-quoted href is valid HTML: brackets
// need no escaping in an attribute value, and the resolver percent-encodes what
// it puts in their place.
//
// Nothing else is touched — a snippet or album marker in a link is not a value
// and stays as goldmark wrote it, and a document without "wert:" comes back as
// it came.
func RestoreEncodedValueMarkers(html string) string {
	if !strings.Contains(html, "wert:") {
		return html
	}
	return encodedValueMarker.ReplaceAllString(html, "[[wert:$1]]")
}

// rawTextElements are the elements whose content the browser does not read as
// character data, so a marker inside one is not in text however much it looks
// like it.
var rawTextElements = map[string]bool{
	"script": true, "style": true, "textarea": true, "title": true,
	"xmp": true, "iframe": true, "noembed": true, "noframes": true,
	"noscript": true, "plaintext": true,
}

// OnlyInText removes every match of every pattern that does not sit in text.
//
// A match inside a tag, a comment, a doctype — anywhere the browser is not
// reading character data — is deleted. What is left is exactly the set of
// matches a context-free replacement may safely expand.
//
// Deleting is the right outcome and not a compromise: a marker in an attribute
// value never rendered a gallery and never rendered a snippet. Before this it
// produced broken markup; after it, nothing. Nothing is what the editor had.
//
// The document comes back byte for byte when no pattern matches outside text,
// which is every page this program itself writes — so the common case pays one
// tokenize and keeps its bytes.
func OnlyInText(doc string, patterns ...*regexp.Regexp) string {
	relevant := patterns[:0:0]
	for _, p := range patterns {
		if p != nil && p.MatchString(doc) {
			relevant = append(relevant, p)
		}
	}
	if len(relevant) == 0 {
		return doc
	}

	z := html.NewTokenizer(strings.NewReader(doc))
	var out strings.Builder
	out.Grow(len(doc))
	changed := false
	inRawText := false
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			// The only error a strings.Reader produces is io.EOF, and the
			// tokenizer itself does not refuse input — it is written to make
			// sense of whatever it is handed. So this is the end of the
			// document and not a failure to read it.
			break
		}
		raw := string(z.Raw())
		switch tt {
		case html.StartTagToken:
			name, _ := z.TagName()
			inRawText = rawTextElements[string(name)]
		case html.EndTagToken:
			// Whatever ends, ends it: the tokenizer gives a raw-text element's
			// content as one token, so the next end tag is that element's own.
			inRawText = false
		}
		// A text token is character data and safe — UNLESS it is the content of
		// an element that holds raw text. Inside <style> or <script> the
		// browser is not reading character data, and an expansion there would
		// end the element early with its own closing tag. bluemonday drops both
		// elements out of editor content, so this cannot arise today from the
		// path that motivated the package; it is guarded rather than argued
		// away, because the guard is one map lookup and the argument depends on
		// a sanitizer's configuration staying as it is.
		if tt == html.TextToken && !inRawText {
			out.WriteString(raw)
			continue
		}
		cleaned := raw
		for _, p := range relevant {
			cleaned = p.ReplaceAllString(cleaned, "")
		}
		if cleaned != raw {
			changed = true
		}
		out.WriteString(cleaned)
	}
	if !changed {
		return doc
	}
	return out.String()
}
