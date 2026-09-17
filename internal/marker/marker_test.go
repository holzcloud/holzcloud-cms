package marker

import (
	"regexp"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

var album = regexp.MustCompile(`\[\[album:([a-z0-9][a-z0-9-]*):(\d+)(?::(\d+):([a-z0-9-]*))?\]\]`)

// The property the whole package rests on: the raw bytes of the tokens are the
// document. If that ever stopped holding, OnlyInText would silently rewrite
// pages it was only supposed to read.
func TestTheTokensAreTheDocument(t *testing.T) {
	for _, doc := range []string{
		`<p>Ein Absatz mit [[album:sommer-2025:1]] mittendrin.</p>`,
		`<a href="/x?q=1" rel="nofollow">klick</a>`,
		`<p>unclosed`,
		`<!-- ein Kommentar --><div class='einfach'>&amp; &#228; <br/></div>`,
		`<p>ä ö ü — 日本語</p>`,
		`< nicht wirklich ein tag >`,
		`<style>p { content: "x" }</style>`,
		``,
	} {
		z := html.NewTokenizer(strings.NewReader(doc))
		var b strings.Builder
		for z.Next() != html.ErrorToken {
			b.Write(z.Raw())
		}
		if b.String() != doc {
			t.Errorf("the tokens are not the document:\n in: %q\nout: %q", doc, b.String())
		}
	}
}

func TestAMarkerInTextIsLeftAlone(t *testing.T) {
	doc := `<p>Ein Absatz mit [[album:sommer-2025:1]] mittendrin.</p>`
	if got := OnlyInText(doc, album); got != doc {
		t.Errorf("a marker in text was touched:\n in: %q\nout: %q", doc, got)
	}
}

// The case of UF-1: the marker is in an href, where expanding it splices markup
// into the middle of an attribute value.
func TestAMarkerInAnAttributeIsRemoved(t *testing.T) {
	doc := `<p><a href="/x?q=[[album:sommer-2025:1]]" rel="nofollow">klick</a></p>`
	want := `<p><a href="/x?q=" rel="nofollow">klick</a></p>`
	if got := OnlyInText(doc, album); got != want {
		t.Errorf("OnlyInText = %q, want %q", got, want)
	}
}

// Both at once, in one document: the one in the attribute goes, the one in the
// text stays. A version that gave up and cleaned the whole document would fail
// here, and so would one that only looked at the first match.
func TestOneOfEachInOneDocument(t *testing.T) {
	doc := `<div title="[[album:a:1]]">vorher [[album:b:2]] nachher</div>` +
		`<img alt="[[album:c:3]]"><p>[[album:d:4]]</p>`
	want := `<div title="">vorher [[album:b:2]] nachher</div>` +
		`<img alt=""><p>[[album:d:4]]</p>`
	if got := OnlyInText(doc, album); got != want {
		t.Errorf("OnlyInText = %q, want %q", got, want)
	}
}

// Inside <style> the browser is not reading character data, so an expansion
// there would end the element with its own closing tag.
func TestAMarkerInRawTextIsRemoved(t *testing.T) {
	doc := `<style>p::after { content: "[[album:a:1]]" }</style><p>[[album:a:1]]</p>`
	want := `<style>p::after { content: "" }</style><p>[[album:a:1]]</p>`
	if got := OnlyInText(doc, album); got != want {
		t.Errorf("OnlyInText = %q, want %q", got, want)
	}
	// And the element ending puts text back on: a marker after </style> is text
	// again and must survive.
	doc = `<style>x</style>[[album:a:1]]`
	if got := OnlyInText(doc, album); got != doc {
		t.Errorf("the marker after </style> was removed: %q", got)
	}
}

// Byte for byte, and the promise the doc comment makes: a document this program
// wrote itself carries no marker outside text, so it comes back unchanged.
func TestADocumentWithNothingToRemoveComesBackItself(t *testing.T) {
	for _, doc := range []string{
		`<p>ohne Marker</p>`,
		`<p>[[album:a:1]]</p>`,
		`<p>[[album:a:1:3:hc-strom]]</p>`,
		`<div class='einfach'>&auml; <br> [[album:a:1]]</div>`,
	} {
		if got := OnlyInText(doc, album); got != doc {
			t.Errorf("a document was rewritten:\n in: %q\nout: %q", doc, got)
		}
	}
}

// What is not a match is not removed. `[[album:x]]` has no position number, so
// no expansion would ever have touched it — and neither does this.
func TestOnlyWhatWouldHaveBeenExpandedIsRemoved(t *testing.T) {
	doc := `<a title="[[album:x]] [[album:y:7]]">k</a>`
	want := `<a title="[[album:x]] ">k</a>`
	if got := OnlyInText(doc, album); got != want {
		t.Errorf("OnlyInText = %q, want %q", got, want)
	}
}

func TestNoPatternsAndNoMatchesAreBothTheDocument(t *testing.T) {
	doc := `<a title="[[album:y:7]]">k</a>`
	if got := OnlyInText(doc); got != doc {
		t.Errorf("without a pattern the document changed: %q", got)
	}
	if got := OnlyInText(doc, nil); got != doc {
		t.Errorf("a nil pattern changed the document: %q", got)
	}
	other := regexp.MustCompile(`\[\[snippet:[a-z]+\]\]`)
	if got := OnlyInText(doc, other); got != doc {
		t.Errorf("a pattern that does not match changed the document: %q", got)
	}
}
