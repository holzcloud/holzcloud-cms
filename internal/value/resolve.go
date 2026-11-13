package value

import (
	"html"
	"strings"

	xhtml "golang.org/x/net/html"
)

// rawTextElements are the elements whose content the browser does not read as
// character data, so a marker inside one is not in text however much it looks
// like it. The list is internal/marker's; it is repeated rather than exported
// from there because that package is a leaf that answers one question.
var rawTextElements = map[string]bool{
	"script": true, "style": true, "textarea": true, "title": true,
	"xmp": true, "iframe": true, "noembed": true, "noframes": true,
	"noscript": true, "plaintext": true,
}

// Resolve replaces every value marker in a stored document with its text,
// written for the place the marker stands.
//
// texts is ONE website's values, key to text — the request's website and no
// other (D-03). A nil or empty map makes every marker empty.
//
// # Why a tokenizer and not a regular expression over the body
//
// block.ResolveWords replaces over the whole body because its words are
// constants and html.EscapeString is right for them in text and in an
// attribute alike. A value is written by an editor and may stand inside a link
// address, where escaping makes markup inert but does not make an address safe:
// a regular expression cannot tell an href from a sentence, and the two need
// different treatment. So the document is tokenized the way marker.OnlyInText
// does it: every token without a marker is written back as its raw bytes, and
// the output differs from the input only where a marker stood.
//
// # Where a marker is replaced and where it is removed
//
// In text — a paragraph, a heading, a list item, code — the marker becomes
// html.EscapeString of its text. Anywhere the browser is not reading character
// data — a comment, a doctype, the content of <script>, <style> or <title> — it
// is removed and never expanded.
//
// # Why an unknown key is nothing
//
// A printed [[wert:…]] would put this program's internal syntax in front of a
// visitor, which is the rule internal/album/expand.go states for a gallery
// whose album is gone. The key is reported to the editor elsewhere (VAL-03); the
// visitor reads the sentence without it.
//
// A document without the prefix is returned as it came, with no tokenizer and
// no allocation: that is what a page without a value costs.
func Resolve(doc string, texts map[string]string) string {
	if !Has(doc) {
		return doc
	}
	z := xhtml.NewTokenizer(strings.NewReader(doc))
	var out strings.Builder
	out.Grow(len(doc))
	inRawText := false
	for {
		tt := z.Next()
		if tt == xhtml.ErrorToken {
			// A strings.Reader ends with io.EOF and the tokenizer refuses no
			// input, so this is the end of the document (marker.OnlyInText).
			break
		}
		raw := string(z.Raw())
		switch tt {
		case xhtml.StartTagToken:
			name, _ := z.TagName()
			inRawText = rawTextElements[string(name)]
		case xhtml.EndTagToken:
			inRawText = false
		}
		if !Has(raw) {
			out.WriteString(raw)
			continue
		}
		if tt == xhtml.TextToken && !inRawText {
			out.WriteString(pattern.ReplaceAllStringFunc(raw, func(m string) string {
				return html.EscapeString(texts[keyOf(m)])
			}))
			continue
		}
		out.WriteString(pattern.ReplaceAllString(raw, ""))
	}
	return out.String()
}

// keyOf takes the key out of a complete marker.
func keyOf(marker string) string { return marker[len(Prefix) : len(marker)-2] }
