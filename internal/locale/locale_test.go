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

// The list comes out of a form: commas, spaces, lines, nonsense.
func TestListeLesen(t *testing.T) {
	got := ParseList("fr, it\nrm  ; de, fr, unsinn!, en", "de")
	want := []string{"fr", "it", "rm", "en"}
	if !slices.Equal(got, want) {
		t.Errorf("ParseList = %v, want %v", got, want)
	}
	// The main language does not belong in the list of the further ones.
	if got := ParseList("de", "de"); len(got) != 0 {
		t.Errorf("the main language is in the list: %v", got)
	}
	if got := ParseList("fr fr fr fr fr fr fr fr fr fr", "de"); len(got) != 1 {
		t.Errorf("Doppelte nicht entfernt: %v", got)
	}
}

// The main language has no prefix. On that hangs the fact that no existing
// address changes when somebody switches on a second language.
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

// Only a language the website really has is recognised as a prefix. Otherwise
// /it/kontakt would silently deliver the German page under an invented address
// — and a search engine would take in both.
func TestPraefixAbtrennen(t *testing.T) {
	extras := []string{"fr", "it"}
	for _, f := range []struct{ path, tag, rest string }{
		{"/fr/kontakt", "fr", "/kontakt"},
		{"/fr", "fr", "/"},
		{"/fr/", "fr", "/"},
		{"/it/tag/wolle", "it", "/tag/wolle"},
		{"/kontakt", "", "/kontakt"},
		{"/", "", "/"},
		// No prefix: the website has no Spanish.
		{"/es/kontakt", "", "/es/kontakt"},
		// A page that happens to start like a language stays a page.
		{"/franzoesisch", "", "/franzoesisch"},
	} {
		tag, rest := Split(f.path, extras)
		if tag != f.tag || rest != f.rest {
			t.Errorf("Split(%q) = %q, %q; want %q, %q", f.path, tag, rest, f.tag, f.rest)
		}
	}
}

// A page with the address "fr" would never be reachable on a website that has
// French.
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
