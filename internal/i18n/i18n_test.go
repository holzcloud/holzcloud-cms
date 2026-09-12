package i18n

import (
	"strings"
	"testing"
)

// The rule everything else rests on: an unknown string comes back as it went
// in. A half-finished catalogue must produce German sentences, never keys.
func TestUnknownStringFallsBackToGerman(t *testing.T) {
	cases := []struct{ lang, in string }{
		{"en", "Eine Zeichenkette, die es nirgends gibt"},
		{"xx", "Seite gespeichert"},
		{"", "Seite gespeichert"},
		{Source, "Seite gespeichert"},
	}
	for _, c := range cases {
		if got := T(c.lang, c.in); got != c.in {
			t.Errorf("T(%q, %q) = %q; want the German back", c.lang, c.in, got)
		}
	}
}

func TestKnownStringIsTranslated(t *testing.T) {
	if got := T("en", "Page saved"); got != "Page saved" {
		t.Errorf(`T("en", "Seite gespeichert") = %q; want "Page saved"`, got)
	}
}

// A catalogue entry left empty is not a translation. It has to fall through to
// the source text rather than blanking the screen.
func TestEmptyTranslationFallsThrough(t *testing.T) {
	ensure()
	mu.Lock()
	catalogs["testempty"] = map[string]string{"Save": ""}
	mu.Unlock()
	defer func() {
		mu.Lock()
		delete(catalogs, "testempty")
		mu.Unlock()
	}()

	if got := T("testempty", "Save"); got != "Save" {
		t.Errorf("got %q; want the source text back for an empty translation", got)
	}
}

func TestTfTranslatesTheFrameBeforeFillingItIn(t *testing.T) {
	got := Tf("de", "Pages – %s", "Velowerkstatt")
	if got != "Seiten – Velowerkstatt" {
		t.Errorf("Tf = %q; want %q", got, "Seiten – Velowerkstatt")
	}
	// Unknown language: the source frame, still filled in.
	if got := Tf("xx", "Pages – %s", "Velowerkstatt"); got != "Pages – Velowerkstatt" {
		t.Errorf("Tf = %q; want the source frame filled in", got)
	}
}

func TestNIsAMarkerAndNothingElse(t *testing.T) {
	if got := N("A box with a button."); got != "A box with a button." {
		t.Errorf("N changed its argument to %q", got)
	}
}

func TestAcceptLanguage(t *testing.T) {
	cases := map[string]string{
		"en-GB,en;q=0.9,de;q=0.8": "en",
		// A Swiss browser asks for de-CH and gets the Swiss fassung, which is
		// the whole point of shipping one.
		"de-CH,de;q=0.9": "de-CH",
		"it-CH":          "it-CH",
		"fr-CH,fr;q=0.9": "fr-CH",
		// A region we have nothing for falls back on the language, not on
		// nothing: German for de-DE, French for fr-FR.
		"de-DE,de;q=0.9": "de",
		"fr-FR,fr;q=0.9": "fr",
		"es-419":         "es",
		// Not among the five that ship, and no file on disk in a test: the
		// source language, which is English since v2.0.
		"nl-NL,nl;q=0.9":     "en",
		"":                   "en",
		"de;q=0.2, en;q=0.9": "en", // the higher wish wins, not the first
		"de;q=0":             "en", // q=0 means "not this one"
		"klingon":            "en",
	}
	for header, want := range cases {
		if got := FromAcceptLanguage(header); got != want {
			t.Errorf("FromAcceptLanguage(%q) = %q; want %q", header, got, want)
		}
	}
}

// The five this build promises to speak out of the box, and the Swiss fassung
// of each national language.
func TestTheShippedLanguagesAreThere(t *testing.T) {
	for _, code := range []string{"de", "en", "fr", "it", "es", "de-CH", "fr-CH", "it-CH"} {
		if !Known(code) {
			t.Errorf("%s is missing from this build", code)
		}
	}
}

// What a regional fassung is for: it says the few sentences it says
// differently, and everything else comes from underneath it.
func TestRegionalFassungLeansOnItsBase(t *testing.T) {
	// Its own: Switzerland writes no ß and quotes with guillemets.
	if got := T("de-CH", "File too large or the upload went wrong"); got != "Datei zu gross oder Upload fehlerhaft" {
		t.Errorf(`T("de-CH", …) = %q; want the Swiss spelling`, got)
	}
	// Not its own, and its base is German: de.json answers.
	//
	// This is new since v2.0 and is the one thing the flip changed about how a
	// regional fassung works. While German was the SOURCE, chain() stopped at
	// de-CH — there was nothing under it to ask, because the key was already
	// German. Now German is a translation like any other, so de-CH leans on
	// de.json exactly the way fr-CH leans on fr.json.
	if got := T("de-CH", "Page saved"); got != "Seite gespeichert" {
		t.Errorf(`T("de-CH", "Page saved") = %q; want what de.json says`, got)
	}
	// Its own: a Swiss Italian says natel.
	if got := T("it-CH", "The closer in, the fewer pixels are left. On a phone photo there is not much to gain beyond about 2×."); !strings.Contains(got, "natel") {
		t.Errorf(`T("it-CH", …) = %q; want the Swiss word`, got)
	}
	// Not its own: Italian answers, not German.
	if got := T("it-CH", "Page saved"); got != T("it", "Page saved") {
		t.Errorf(`T("it-CH", "Seite gespeichert") = %q; want what Italian says: %q`, got, T("it", "Page saved"))
	}
	if T("it", "Page saved") == "Page saved" {
		t.Fatal("the Italian catalogue is empty; the test above proves nothing")
	}
}

// A fassung of a language this build does not have falls through to the source
// rather than to an empty screen.
func TestUnknownRegionIsHarmless(t *testing.T) {
	if got := T("nl-BE", "Page saved"); got != "Page saved" {
		t.Errorf("T(%q) = %q; want the source text", "nl-BE", got)
	}
}

func TestTagsAreNormalised(t *testing.T) {
	cases := map[string]string{
		"de_ch": "de-CH", "DE-CH": "de-CH", " fr-ch ": "fr-CH", "IT": "it", "rm": "rm",
	}
	for in, want := range cases {
		if got := Normalise(in); got != want {
			t.Errorf("Normalise(%q) = %q; want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "d", "deutsch", "de-CHE", "de-1", "d e"} {
		if ValidTag(bad) {
			t.Errorf("ValidTag(%q) is true; want false", bad)
		}
	}
	for _, good := range []string{"de", "rm", "de-CH", "fr-CH"} {
		if !ValidTag(good) {
			t.Errorf("ValidTag(%q) is false; want true", good)
		}
	}
	if Base("de-CH") != "de" || Base("de") != "" {
		t.Errorf("Base is wrong: %q %q", Base("de-CH"), Base("de"))
	}
}

// The picker is read by somebody looking for their own language, so a fassung
// has to be recognisable as one — and sort next to what it belongs to.
func TestRegionalFassungIsNamedAfterItsLanguage(t *testing.T) {
	if got := nameOf("de-CH"); got != "Deutsch (Schweiz)" {
		t.Errorf("nameOf(de-CH) = %q", got)
	}
	if got := nameOf("nl-BE"); got != "Nederlands (BE)" {
		t.Errorf("nameOf(nl-BE) = %q", got)
	}
}

func TestLanguagesAlwaysContainTheSource(t *testing.T) {
	found := false
	for _, l := range Languages() {
		if l.Code == Source {
			found = true
			if l.Name == "" {
				t.Error("the source language has no name")
			}
		}
	}
	if !found {
		t.Error("German is missing from the list of languages")
	}
	if !Known(Source) {
		t.Error("German is not Known, although it needs no catalogue")
	}
}
