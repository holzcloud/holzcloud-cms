package page

import (
	"regexp"
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
)

// Excerpt derives a plain-text summary from Markdown.
//
// It exists so a page always has something for a listing and for
// <meta name="description">; an editor who wants control fills the field in and
// this is never consulted.
func Excerpt(markdown string) string {
	if strings.TrimSpace(markdown) == "" {
		return ""
	}

	// Fenced code goes first: otherwise its contents get mistaken for prose and
	// the summary is made of source code.
	text := cmsMarker.ReplaceAllString(markdown, " ")
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
	return truncateWords(text, MaxExcerptLength)
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
