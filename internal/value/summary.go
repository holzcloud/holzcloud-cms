package value

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ResolveSummary puts the website's values into a page's summary — the excerpt
// that becomes <meta name="description">, og:description, the JSON-LD
// description, an archive entry, the feed's <summary>.
//
// The excerpt is plain text, derived at save from the page's Markdown, and it
// keeps a value marker as it stands (page.Excerpt) so that the value goes in
// here, when the summary is handed out, and changing the value changes every
// summary without saving a page (VAL-04).
//
// The result is plain text and NOT escaped: every caller hands it to
// something that escapes for its place — html/template for an attribute or a
// paragraph, encoding/xml for the feed, encoding/json for the JSON-LD.
//
// # Why not ResolvePlain
//
// ResolvePlain is for a search passage: it knows the highlight bytes and a cut
// at either end. A summary has neither — page.Excerpt never cuts inside a
// marker, and an excerpt that starts with "x]]" is an editor's text that a
// closing-half rule would eat. What a summary needs that a passage does not is
// the space: a key nobody defined is nothing, and "Alpha [[wert:x]]: neu" must
// read "Alpha: neu" and not "Alpha : neu", which is the line this function was
// written against.
//
// A summary without the prefix is returned as it came and allocates nothing.
func ResolveSummary(s string, texts map[string]string) string {
	if !Has(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	last := 0
	for _, m := range pattern.FindAllStringSubmatchIndex(s, -1) {
		before := s[last:m[0]]
		text := texts[s[m[2]:m[3]]]
		if text == "" {
			// The marker goes without a trace: the space in front of it goes
			// too when what follows is a space, a punctuation mark or the end.
			after := s[m[1]:]
			r, _ := utf8.DecodeRuneInString(after)
			if after == "" || unicode.IsSpace(r) || unicode.IsPunct(r) {
				before = strings.TrimRightFunc(before, unicode.IsSpace)
			}
		}
		b.WriteString(before)
		b.WriteString(text)
		last = m[1]
	}
	b.WriteString(s[last:])
	// A half the summary ends in — an editor's own excerpt can be anything —
	// is not text either; then the spaces are made single again.
	return strings.Join(strings.Fields(cutOpeningHalf(b.String())), " ")
}
