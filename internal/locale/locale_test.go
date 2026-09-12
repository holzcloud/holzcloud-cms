package locale

import (
	"slices"
	"testing"
)

func TestValidLanguageTags(t *testing.T) {
	for _, gut := range []string{"de", "fr", "en", "rm", "de-CH", "fr-CH", "pt-BR"} {
		if !Valid(gut) {
			t.Errorf("%q wurde abgelehnt", gut)
		}
	}
	for _, schlecht := range []string{
		"", "d", "deutsch", "DE", "de-ch", "de-CHE", "de-", "-CH", "de/CH",
		"../etc", "de CH", "de.CH",
	} {
		if Valid(schlecht) {
			t.Errorf("%q wurde angenommen", schlecht)
		}
	}
}

func TestNormalisieren(t *testing.T) {
	for raw, will := range map[string]string{
		"DE": "de", "de_ch": "de-CH", " fr ": "fr", "FR-ch": "fr-CH", "de-CH": "de-CH",
	} {
		if got := Normalise(raw); got != will {
			t.Errorf("Normalise(%q) = %q, want %q", raw, got, will)
		}
	}
}

// Die Liste kommt aus einem Formular: Kommas, Leerzeichen, Zeilen, Unsinn.
func TestListeLesen(t *testing.T) {
	got := ParseList("fr, it\nrm  ; de, fr, unsinn!, en", "de")
	want := []string{"fr", "it", "rm", "en"}
	if !slices.Equal(got, want) {
		t.Errorf("ParseList = %v, want %v", got, want)
	}
	// Die Hauptsprache gehört nicht in die Liste der weiteren.
	if got := ParseList("de", "de"); len(got) != 0 {
		t.Errorf("the main language is in the list: %v", got)
	}
	if got := ParseList("fr fr fr fr fr fr fr fr fr fr", "de"); len(got) != 1 {
		t.Errorf("Doppelte nicht entfernt: %v", got)
	}
}

// Die Hauptsprache hat kein Präfix. Daran hängt, dass keine bestehende Adresse
// sich ändert, wenn jemand eine zweite Sprache einschaltet.
func TestTheMainLanguageHasNoPrefix(t *testing.T) {
	if p := Prefix("de", "de"); p != "" {
		t.Errorf("Prefix(de, de) = %q, want leer", p)
	}
	if p := Prefix("", "de"); p != "" {
		t.Errorf("Prefix(leer, de) = %q, want leer", p)
	}
	if p := Prefix("fr", "de"); p != "/fr" {
		t.Errorf("Prefix(fr, de) = %q, want /fr", p)
	}

	for _, f := range []struct{ tag, path, will string }{
		{"de", "/kontakt", "/kontakt"},
		{"fr", "/kontakt", "/fr/kontakt"},
		{"fr", "/", "/fr"},
		{"de", "/", "/"},
		{"fr", "", "/fr"},
	} {
		if got := Path(f.tag, "de", f.path); got != f.will {
			t.Errorf("Path(%q, de, %q) = %q, want %q", f.tag, f.path, got, f.will)
		}
	}
}

// Nur eine Sprache, die die Website wirklich hat, wird als Präfix erkannt.
// Sonst lieferte /it/kontakt still die deutsche Seite unter einer erfundenen
// Adresse — und eine Suchmaschine nähme beide auf.
func TestPraefixAbtrennen(t *testing.T) {
	extras := []string{"fr", "it"}
	for _, f := range []struct{ path, tag, rest string }{
		{"/fr/kontakt", "fr", "/kontakt"},
		{"/fr", "fr", "/"},
		{"/fr/", "fr", "/"},
		{"/it/tag/wolle", "it", "/tag/wolle"},
		{"/kontakt", "", "/kontakt"},
		{"/", "", "/"},
		// Kein Präfix: die Website hat kein Spanisch.
		{"/es/kontakt", "", "/es/kontakt"},
		// Eine Seite, die zufällig wie eine Sprache anfängt, bleibt eine Seite.
		{"/franzoesisch", "", "/franzoesisch"},
	} {
		tag, rest := Split(f.path, extras)
		if tag != f.tag || rest != f.rest {
			t.Errorf("Split(%q) = %q, %q; want %q, %q", f.path, tag, rest, f.tag, f.rest)
		}
	}
}

// Eine Seite mit der Adresse „fr" wäre auf einer Website mit Französisch nie
// erreichbar.
func TestTakenAddresses(t *testing.T) {
	if !Reserved("fr", []string{"fr", "it"}) {
		t.Error("fr was not recognised as taken")
	}
	if Reserved("frankreich", []string{"fr"}) {
		t.Error("frankreich was wrongly recognised as taken")
	}
}

func TestNamen(t *testing.T) {
	// Name is what the ADMIN calls a language, so it is in the admin's source
	// language — English since v2.0, German before it. Native, below, is what
	// the language calls itself and does not move.
	for tag, will := range map[string]string{
		"de": "German", "fr": "French", "fr-CH": "French (CH)",
		"xx": "xx", "rm": "Romansh",
	} {
		if got := Name(tag); got != will {
			t.Errorf("Name(%q) = %q, want %q", tag, got, will)
		}
	}
}

func TestPickOnlyAcceptsALanguageTheWebsiteHas(t *testing.T) {
	extras := []string{"fr", "it"}
	cases := map[string]string{
		"fr":     "fr",
		" FR ":   "fr",
		"it":     "it",
		"es":     "", // not a language of this website
		"":       "",
		"../etc": "",
	}
	for in, want := range cases {
		if got := Pick(in, extras); got != want {
			t.Errorf("Pick(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestNativeNamesTheLanguageInItself(t *testing.T) {
	// A switcher is read by somebody who does not speak the page they are on.
	if got := Native("fr"); got != "Français" {
		t.Errorf(`Native("fr") = %q; want "Français"`, got)
	}
	if got := Name("fr"); got != "French" {
		t.Errorf(`Name("fr") = %q; want "French" — Name is in the admin's own language`, got)
	}
	// An unknown tag falls back rather than inventing a name.
	if got := Native("xx"); got != "xx" {
		t.Errorf(`Native("xx") = %q; want "xx"`, got)
	}
}
