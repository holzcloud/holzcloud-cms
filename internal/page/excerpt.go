package page

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// MaxExcerptLength is roughly what a search result snippet shows. Longer text is
// cut at a word boundary rather than mid-word.
const MaxExcerptLength = 160

// The Markdown syntax that must not show up in a plain-text summary. These run
// against the Markdown source rather than the rendered HTML, which keeps the
// result independent of which goldmark extensions happen to be enabled.
var (
	fencedCode = regexp.MustCompile("(?s)```.*?```")
	image      = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	// A link keeps its visible text, so "[Impressum](/impressum)" reads as
	// "Impressum" rather than vanishing from the summary.
	link       = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	inlineHTML = regexp.MustCompile(`<[^>]+>`)
	heading    = regexp.MustCompile(`^\s{0,3}#{1,6}\s+`)
	blockquote = regexp.MustCompile(`^\s{0,3}>\s?`)
	listBullet = regexp.MustCompile(`^\s{0,3}([-*+]|\d+\.)\s+`)
	emphasis   = regexp.MustCompile(`[*_~` + "`" + `]+`)
	// CMS markers are page syntax, not prose. Left in, they end up in the
	// excerpt and from there in <meta name="description"> and og:description,
	// where a search result would read "Willkommen [[snippet:zeiten]] Ende."
	//
	// One pattern for every marker rather than one per kind: [[formular]] leaked
	// into a meta description exactly the way [[snippet:zeiten]] did before it,
	// and a pattern per marker means the next one added leaks too.
	//
	// The shape is duplicated from internal/snippet rather than imported: that
	// package reads the pages table, so depending on it from here would point
	// the layering the wrong way for one regexp.
	cmsMarker = regexp.MustCompile(`\[\[[a-z0-9][a-z0-9_:-]*\]\]`)
	// valueMarker is the one marker the excerpt keeps: [[wert:key]], a website's
	// value. Its shape is internal/value's, duplicated for the reason cmsMarker
	// gives.
	valueMarker = regexp.MustCompile(`\[\[wert:[a-z0-9][a-z0-9_-]*\]\]`)
	// heldValue is what a kept value marker stands as while the Markdown is
	// taken apart: a control byte no pass touches around the marker's index.
	// The marker itself cannot stand there — its brackets break the link and
	// image patterns, and the emphasis pass eats the underscore of a key.
	heldValue = regexp.MustCompile("\x01([0-9]+)\x01")
)

// Excerpt derives a plain-text summary from Markdown.
//
// It exists so a page always has something for a listing and for
// <meta name="description">; an editor who wants control fills the field in and
// this is never consulted.
//
// A value marker, [[wert:key]], is kept as it stands, and it is the only
// marker that is. The summary is the page's text in short, and in that text the
// value is part of the sentence: "Alpha [[wert:v]]: neu" reads "Alpha 0.1.5:
// neu" on the page, and a summary without the marker read "Alpha : neu" in
// every search result. The value is put in where the summary is handed out
// (value.ResolveSummary), not here: a text frozen at save would keep the old
// version after the value changed (VAL-04). Only a marker in prose survives —
// one in a link address or in a picture goes with the address or the picture.
//
// A product's description is summarised the same way: the shop resolves
// values in it like a page (WR-03 of the phase 33 review).
func Excerpt(markdown string) string {
	if strings.TrimSpace(markdown) == "" {
		return ""
	}

	var held []string
	text := valueMarker.ReplaceAllStringFunc(markdown, func(m string) string {
		held = append(held, m)
		return "\x01" + strconv.Itoa(len(held)-1) + "\x01"
	})
	// Fenced code goes first: otherwise its contents get mistaken for prose and
	// the summary is made of source code.
	text = cmsMarker.ReplaceAllString(text, " ")
	text = fencedCode.ReplaceAllString(text, " ")
	text = image.ReplaceAllString(text, " ")
	text = link.ReplaceAllString(text, "$1")
	text = inlineHTML.ReplaceAllString(text, "")

	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = heading.ReplaceAllString(line, "")
		line = blockquote.ReplaceAllString(line, "")
		line = listBullet.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		// A setext underline or a horizontal rule carries no words.
		if line == "" || strings.Trim(line, "-=*_ ") == "" {
			continue
		}
		lines = append(lines, line)
	}

	text = emphasis.ReplaceAllString(strings.Join(lines, " "), "")
	text = strings.Join(strings.Fields(text), " ")
	if len(held) > 0 {
		text = heldValue.ReplaceAllStringFunc(text, func(m string) string {
			i, err := strconv.Atoi(m[1 : len(m)-1])
			if err != nil || i >= len(held) {
				return ""
			}
			return held[i]
		})
	}
	return withoutCutMarker(truncateWords(text, MaxExcerptLength))
}

// withoutCutMarker removes what the cut left of a value marker at the end.
//
// truncateWords cuts at a space and a marker holds none, so a marker is either
// kept whole or left out — except when the first MaxExcerptLength characters
// hold no space at all and the cut falls mid-word. Half a marker is neither
// text nor anything delivery can resolve.
func withoutCutMarker(s string) string {
	i := strings.LastIndex(s, "[[")
	if i < 0 || !strings.HasSuffix(s, "…") {
		return s
	}
	rest := strings.TrimSuffix(s[i+2:], "…")
	// "wert:" cut anywhere, or "wert:" and part of a key. Anything else is
	// somebody's two brackets and stays.
	if !strings.HasPrefix("wert:", rest) &&
		!(strings.HasPrefix(rest, "wert:") && !strings.ContainsAny(rest, " ]")) {
		return s
	}
	return strings.TrimRight(s[:i], " ,;:.-") + "…"
}

// truncateWords cuts at the last word boundary that fits, so a summary never
// ends in the middle of a word.
func truncateWords(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	cut := string(runes[:limit])
	if i := strings.LastIndexFunc(cut, unicode.IsSpace); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:.-") + "…"
}
