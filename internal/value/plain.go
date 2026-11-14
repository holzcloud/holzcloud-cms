package value

import (
	"regexp"
	"strings"
)

// The two bytes internal/page puts around a search term inside a passage. They
// are named again here rather than imported: this package depends on nothing
// but the database, and the page store is a far heavier thing to pull in for
// two bytes. CleanText refuses every control character, so a value's own text
// can never carry one of them.
const (
	searchMarkStart = "\x02"
	searchMarkEnd   = "\x03"
)

// ellipsis is what FTS5 writes where it cut a passage.
const ellipsis = "…"

// bracketPair is one [[…]] with no bracket and no line break inside: the shape
// of every marker, with whatever a highlight put between its brackets.
var bracketPair = regexp.MustCompile(`\[\[[^\[\]\n]*\]\]`)

// ResolvePlain puts the website's values into a passage of plain text, and
// removes what a cut left of a marker.
//
// It is for the passages of a full-text search. Those come from the Markdown
// source, not from the rendered page, so a marker stands in them as the editor
// wrote it — and the passage is cut at a token boundary, which can fall inside
// a marker. Three cases, and each comes out as the value's text or as nothing:
//
//   - A complete marker becomes the raw text of its value, or nothing for a
//     key the website does not define. A highlight inside it — the visitor
//     searched for "version" and the index marked that word of the key — is
//     dropped with the marker.
//   - An opening half at the end of the passage ("…Alpha [[wert:version-holz…")
//     is removed, the ellipsis kept.
//   - A closing half at the start ("…cms]]: neu…") is removed the same way.
//
// A marker of another family ([[snippet:…]]) is left as it stands: this is
// the value's pass and no other.
//
// The result is NOT escaped, and must not be: the caller escapes after, with
// page.Highlight, which is also what turns the remaining sentinels into
// <mark>. Escaping here first would escape twice; inserting a text after
// Highlight would insert it unescaped into HTML.
//
// A passage without "[[" and without "]]" is returned as it is and allocates
// nothing, which is the path nearly every search hit takes.
func ResolvePlain(s string, texts map[string]string) string {
	if !strings.Contains(s, "[[") && !strings.Contains(s, "]]") {
		return s
	}
	s = bracketPair.ReplaceAllStringFunc(s, func(m string) string {
		bare := withoutSearchMarks(m)
		if !strings.HasPrefix(bare, Prefix) {
			return m
		}
		key := keyOf(bare)
		if key == "" || !isKeyStart(key[0]) || !allKeyBytes(key) {
			return m
		}
		return texts[key]
	})
	s = cutOpeningHalf(s)
	return cutClosingHalf(s)
}

// cutOpeningHalf removes a marker the passage ends inside of.
func cutOpeningHalf(s string) string {
	i := strings.LastIndex(s, "[[")
	if i < 0 {
		return s
	}
	rest := s[i+2:]
	if strings.Contains(rest, "]]") {
		// A complete pair of another family, left alone above.
		return s
	}
	tail := ""
	if strings.HasSuffix(rest, ellipsis) {
		tail = ellipsis
		rest = strings.TrimSuffix(rest, ellipsis)
	}
	if !isValueOpening(withoutSearchMarks(rest)) {
		return s
	}
	return s[:i] + tail
}

// cutClosingHalf removes a marker the passage starts inside of.
func cutClosingHalf(s string) string {
	j := strings.Index(s, "]]")
	if j < 0 {
		return s
	}
	head := s[:j]
	if strings.Contains(head, "[[") {
		return s
	}
	lead := ""
	if strings.HasPrefix(head, ellipsis) {
		lead = ellipsis
		head = strings.TrimPrefix(head, ellipsis)
	}
	if !isValueClosing(withoutSearchMarks(head)) {
		return s
	}
	return lead + s[j+2:]
}

// isValueOpening reports whether b, what followed "[[" to the end of the
// passage, can be the start of a value marker: nothing, part of "wert:", or
// "wert:" and part of a key.
func isValueOpening(b string) bool {
	name := strings.TrimPrefix(Prefix, "[[")
	if strings.HasPrefix(name, b) {
		return true
	}
	return strings.HasPrefix(b, name) && allKeyBytes(b[len(name):])
}

// isValueClosing reports whether b, what stood before "]]" from the start of
// the passage, can be the end of a value marker: part of a key, or the end of
// "[[wert" followed by ":" and a key.
func isValueClosing(b string) bool {
	if i := strings.LastIndex(b, ":"); i >= 0 {
		return strings.HasSuffix(strings.TrimSuffix(Prefix, ":"), b[:i]) && allKeyBytes(b[i+1:])
	}
	return allKeyBytes(b)
}

func withoutSearchMarks(s string) string {
	s = strings.ReplaceAll(s, searchMarkStart, "")
	return strings.ReplaceAll(s, searchMarkEnd, "")
}

func isKeyStart(c byte) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }

// allKeyBytes reports whether every byte of s is one a key may hold.
func allKeyBytes(s string) bool {
	for i := 0; i < len(s); i++ {
		if c := s[i]; !isKeyStart(c) && c != '-' && c != '_' {
			return false
		}
	}
	return true
}
