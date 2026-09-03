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
	if got := T("en", "Seite gespeichert"); got != "Page saved" {
		t.Errorf(`T("en", "Seite gespeichert") = %q; want "Page saved"`, got)
	}
}

// A catalogue entry left empty is not a translation. It has to fall through to
// the German rather than blanking the screen.
func TestEmptyTranslationFallsThrough(t *testing.T) {
	ensure()
	mu.Lock()
	catalogs["testleer"] = map[string]string{"Speichern": ""}
	mu.Unlock()
	defer func() {
		mu.Lock()
		delete(catalogs, "testleer")
		mu.Unlock()
	}()

	if got := T("testleer", "Speichern"); got != "Speichern" {
		t.Errorf("got %q; want the German back for an empty translation", got)
	}
}

func TestTfTranslatesTheFrameBeforeFillingItIn(t *testing.T) {
	got := Tf("en", "Seiten – %s", "Velowerkstatt")
	if got != "Pages – Velowerkstatt" {
		t.Errorf("Tf = %q; want %q", got, "Pages – Velowerkstatt")
	}
	// Unknown language: the German frame, still filled in.
	if got := Tf("xx", "Seiten – %s", "Velowerkstatt"); got != "Seiten – Velowerkstatt" {
		t.Errorf("Tf = %q; want the German frame filled in", got)
	}
}

func TestNIsAMarkerAndNothingElse(t *testing.T) {
	if got := N("Ein Kasten mit Knopf."); got != "Ein Kasten mit Knopf." {
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
		// Not among the five that ship, and no file on disk in a test: German.
		"nl-NL,nl;q=0.9":     "de",
		"":                   "de",
		"de;q=0.2, en;q=0.9": "en", // the higher wish wins, not the first
		"en;q=0":             "de", // q=0 means "not this one"
		"klingon":            "de",
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
	if got := T("de-CH", "Datei zu groß oder Upload fehlerhaft"); got != "Datei zu gross oder Upload fehlerhaft" {
		t.Errorf(`T("de-CH", …) = %q; want the Swiss spelling`, got)
	}
	// Not its own, and its base is German: the German comes back untouched.
	if got := T("de-CH", "Seite gespeichert"); got != "Seite gespeichert" {
		t.Errorf(`T("de-CH", "Seite gespeichert") = %q; want the German`, got)
	}
	// Its own: a Swiss Italian says natel.
	if got := T("it-CH", "Je näher, desto weniger Pixel bleiben übrig. Bei einem Handyfoto ist ab etwa 2× nicht mehr viel zu holen."); !strings.Contains(got, "natel") {
		t.Errorf(`T("it-CH", …) = %q; want the Swiss word`, got)
	}
	// Not its own: Italian answers, not German.
	if got := T("it-CH", "Seite gespeichert"); got != T("it", "Seite gespeichert") {
		t.Errorf(`T("it-CH", "Seite gespeichert") = %q; want what Italian says: %q`, got, T("it", "Seite gespeichert"))
	}
	if T("it", "Seite gespeichert") == "Seite gespeichert" {
		t.Fatal("the Italian catalogue is empty; the test above proves nothing")
	}
}

// A fassung of a language this build does not have falls through to German
// rather than to an empty screen.
func TestUnknownRegionIsHarmless(t *testing.T) {
	if got := T("nl-BE", "Seite gespeichert"); got != "Seite gespeichert" {
		t.Errorf("T(%q) = %q; want the German", "nl-BE", got)
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
