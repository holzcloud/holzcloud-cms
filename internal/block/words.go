package block

import (
	"html"
	"regexp"
	"strings"
)

// The five words this package writes into a page, and the markers that stand
// for them until somebody is reading.
//
// # Why they are not translated at save
//
// block.Render runs once, when an editor presses Save, and what it writes goes
// into pages.content_html. A word translated there is translated in whatever
// language the WEBSITE had at that moment, and it stays in it: a site changed
// from German to Spanish answers "Imagen siguiente" beside
// aria-label="Galerie" until every page carrying a gallery is saved again. That
// is window 8, and only a screen reader ever said it out loud.
//
// v2.2 fixed half of it — an album gallery's wrapper became late along with its
// tiles, so the two halves of one region agree. It could not fix the other half,
// because an inline gallery on the same page was still frozen and two galleries
// in two languages is worse than two galleries in the wrong one. Window 34 is
// what was left: the album half answers in the WEBSITE's language, and a page
// has a language of its own.
//
// So nothing is translated at save any more. The renderer writes a marker, one
// pass at delivery replaces every marker with the word of the page's language,
// and the question "which language is this?" is asked once, in the place that
// knows — which is the shape PUB-01 established for the public side and the
// reason public.LocaleMiddleware exists.
//
// # Why a marker and not a second render
//
// Measured, in 16-MEASUREMENT.md. Rendering a page's blocks again on every
// request costs 124 µs and 215 KB on a 23 KB page and cannot be skipped
// cheaply, because the render is the question. A guarded substitution costs a
// page with no marker 311 ns and no allocation at all — the same
// strings.Contains the album marker already asks. Searching a body is free;
// building a second one is what costs.
const wordMarkerPrefix = "[[w:"

// The keys. Short, because they are written into every gallery on every page,
// and stable, because they are stored: renaming one would strand every page
// saved before the rename.
const (
	wordPrevious = "previous"
	wordNext     = "next"
	wordClose    = "close"
	wordNoVideo  = "novideo"
	wordGallery  = "gallery"
)

// words maps a stored key to the catalogue entry it stands for.
//
// The values are the i18n.N literals declared in render.go, referenced rather
// than repeated: a key whose sentence is spelled out a second time here is a key
// that silently stops being translated when somebody corrects the first one.
var words = map[string]string{
	wordPrevious: textPrevious,
	wordNext:     textNext,
	wordClose:    textClose,
	wordNoVideo:  textNoVideo,
	wordGallery:  textGallery,
}

// wordMarkerPattern matches what Word writes.
//
// The key alphabet is deliberately narrow — lowercase letters only — so that a
// marker cannot be confused with an album marker and so that nothing an editor
// types can widen it.
var wordMarkerPattern = regexp.MustCompile(`\[\[w:([a-z]+)\]\]`)

// Word is what the renderer writes where a word used to stand.
//
// Built from the prefix rather than spelled out, so the writer and the reader
// cannot disagree — the same rule AlbumMarker follows.
func Word(key string) string { return wordMarkerPrefix + key + "]]" }

// HasWordMarker reports whether a document carries any of them.
//
// The cheap question, asked before the expensive one. Exported, unlike
// hasAlbumMarker, because the answer is worth having outside this package: the
// test that proves no public route leaks a marker asks it, and so does anything
// that wants to know whether a stored body is a new one or an old one.
func HasWordMarker(s string) bool { return strings.Contains(s, wordMarkerPrefix) }

// ResolveWords replaces every marker with the word t gives for it.
//
// t is the language of whoever is reading: the PAGE's language on a public
// route, the operator's in the admin. That is the whole point of doing this
// late, and the two are different questions — getting them the wrong way round
// is how v2.0 came to answer German visitors in English.
//
// A document with no marker is returned unchanged and untouched, which is what
// keeps this free for the pages that do not use it — and for every page written
// before markers existed, whose words are already in the stored HTML. Such a
// page renders exactly as it did and heals on its next save: no migration, no
// broken page, the rule window 8 followed (WORD-03).
//
// The word is escaped here rather than at save. That is not a new obligation,
// it is the same one moved: the renderer escaped what it wrote, and what it
// writes now is a marker that needs no escaping. A marker stands in text
// content and inside an attribute alike, and html.EscapeString is correct in
// both.
//
// An unknown key resolves to nothing. It cannot happen — the keys are constants
// and TestEveryWordMarkerHasACatalogueEntry holds them against this map — and if
// it ever does, a missing accessible name costs its own control while a printed
// [[w:…]] would put this program's internal syntax in front of a visitor, which
// internal/album/expand.go states as the rule from the other side.
func ResolveWords(doc string, t func(string) string) string {
	if !HasWordMarker(doc) {
		return doc
	}
	if t == nil {
		t = func(word string) string { return word }
	}
	return wordMarkerPattern.ReplaceAllStringFunc(doc, func(match string) string {
		key := match[len(wordMarkerPrefix) : len(match)-2]
		word, ok := words[key]
		if !ok {
			return ""
		}
		return html.EscapeString(t(word))
	})
}
