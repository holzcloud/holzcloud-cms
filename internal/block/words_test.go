package block

import (
	"strings"
	"testing"
	"testing/quick"
)

// Every marker this package can write has a word behind it.
//
// The one failure mode of a lookup table: a key is added to the renderer and not
// to the map, and the page loses a control's name with nothing reported. The
// keys are constants, so this cannot drift silently — but the map is what makes
// them mean anything, and a constant with no entry is a constant that resolves
// to the empty string on a live page.
func TestEveryWordMarkerHasACatalogueEntry(t *testing.T) {
	for _, key := range []string{wordPrevious, wordNext, wordClose, wordNoVideo, wordGallery} {
		word, ok := words[key]
		if !ok {
			t.Errorf("the key %q has no word; every control it names loses its name", key)
			continue
		}
		if strings.TrimSpace(word) == "" {
			t.Errorf("the key %q resolves to nothing", key)
		}
	}
	if len(words) != 5 {
		t.Errorf("the map holds %d words; the renderer writes five. A sixth needs "+
			"a catalogue entry, four translations and a line in TEMPLATE-SPEC, "+
			"not just a constant", len(words))
	}
}

// A document with no marker comes back the same string, untouched.
//
// This is the whole economy of the mechanism and the reason the guard is not
// tidiness: 16-MEASUREMENT.md measured the alternative at 37 µs and a 49 KB copy
// of every page into the garbage collector, on every view of every site,
// including the overwhelming majority that have no gallery at all.
func TestADocumentWithNoMarkerIsReturnedUntouched(t *testing.T) {
	for _, doc := range []string{
		"",
		"<p>Nothing to do here.</p>",
		// The neighbouring marker families must not be mistaken for this one.
		"<p>[[album:moebel:0]]</p>",
		"<p>[[snippet:footer]]</p>",
		// Nearly the prefix, and deliberately not it.
		"<p>[[w]]</p><p>[[ w:next]]</p><p>[[W:next]]</p>",
	} {
		if got := ResolveWords(doc, strings.ToUpper); got != doc {
			t.Errorf("ResolveWords rewrote a document with no marker:\n  in  %q\n  out %q", doc, got)
		}
	}
}

// The allocation, not just the identity: returning an equal string that was
// built fresh would pass the test above and still cost the copy it exists to
// avoid.
func TestResolvingADocumentWithNoMarkerAllocatesNothing(t *testing.T) {
	doc := strings.Repeat("<p>Die Werkstatt steht seit 1954 am selben Platz.</p>", 400)
	allocs := testing.AllocsPerRun(100, func() {
		if out := ResolveWords(doc, strings.ToUpper); len(out) != len(doc) {
			t.Fatal("the document changed length")
		}
	})
	if allocs != 0 {
		t.Errorf("resolving a marker-free document allocated %.0f times; the guard "+
			"is not in front of the work", allocs)
	}
}

// An unknown key costs its own control and never the page.
//
// It cannot arrive from this package — the keys are constants — but it can
// arrive from a stored body, which is a file on somebody's disk and a row in a
// database. Printing it would put [[w:…]] in front of a visitor, which is the
// one outcome internal/album/expand.go argues at length is worse than showing
// nothing.
func TestAnUnknownKeyResolvesToNothingRatherThanToSyntax(t *testing.T) {
	out := ResolveWords(`<p>[[w:nosuchword]] and `+Word(wordNext)+`</p>`, nil)
	if HasWordMarker(out) {
		t.Errorf("an unknown key was printed as syntax:\n%s", out)
	}
	if !strings.Contains(out, textNext) {
		t.Errorf("the unknown key took a known one down with it:\n%s", out)
	}
}

// Whatever a catalogue holds is escaped where it lands.
//
// The obligation moved with the word: the renderer used to escape what it wrote
// at save, and what it writes now needs no escaping. A translation carrying a
// quotation mark reaches an aria-label attribute, so the escaping has to happen
// at the substitution or it happens nowhere.
func TestAResolvedWordIsAlwaysEscaped(t *testing.T) {
	if err := quick.Check(func(raw string) bool {
		out := ResolveWords(`<div aria-label="`+Word(wordGallery)+`">x</div>`,
			func(string) string { return raw })
		// Whatever came in, the attribute must still be one attribute: nothing
		// between the opening quote and the closing one may be a quote itself.
		inner := out[strings.Index(out, `aria-label="`)+len(`aria-label="`):]
		inner = inner[:strings.Index(inner, `"`)]
		return !strings.ContainsAny(inner, `"<>`)
	}, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}

// Round trip: what the renderer writes is what the resolver reads.
//
// Written as a pair rather than as two assertions about a literal, because the
// marker's spelling is a stored format — a page saved today is read by a build
// from next year, and the two halves have to be changed together or not at all.
func TestWhatIsWrittenIsWhatIsRead(t *testing.T) {
	for key, word := range words {
		out := ResolveWords("<p>"+Word(key)+"</p>", nil)
		if out != "<p>"+word+"</p>" {
			t.Errorf("the round trip for %q gave %q, want %q", key, out, "<p>"+word+"</p>")
		}
	}
}
