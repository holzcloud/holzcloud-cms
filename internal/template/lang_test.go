package template

import (
	"strings"
	"testing"
	"testing/fstest"
)

// A theme translates against its own catalogue and against nothing else.
//
// This is the sentence that answers TEMPLATE-SPEC §2.5's objection. A t that
// reached into this program's catalogue would work for the eight themes we
// ship and silently return the key for a stranger's, which §2.5 rightly called
// worse than none.
func TestAThemeTranslatesFromItsOwnCatalogue(t *testing.T) {
	theme := fstest.MapFS{
		"lang/de.json": &fstest.MapFile{Data: []byte(`{"Cart":"Warenkorb","Search":"Suche"}`)},
		"lang/fr.json": &fstest.MapFile{Data: []byte(`{"Cart":"Panier","Search":"Recherche"}`)},
	}
	read := func(name string) ([]byte, error) { return theme.ReadFile(name) }

	for _, c := range []struct{ locale, key, want string }{
		{"de", "Cart", "Warenkorb"},
		{"fr", "Cart", "Panier"},
		{"fr", "Search", "Recherche"},
		// A language the theme does not carry falls back to the key, which is
		// the author's own word — never to another language.
		{"it", "Cart", "Cart"},
		// A key the catalogue does not carry, likewise.
		{"de", "Checkout", "Checkout"},
	} {
		if got := loadCatalog(read, c.locale).T(c.key); got != c.want {
			t.Errorf("locale %q, key %q = %q, want %q", c.locale, c.key, got, c.want)
		}
	}
}

// A regional tag falls back to its base language and stops there.
func TestARegionalTagFallsBackToItsBaseAndNoFurther(t *testing.T) {
	theme := fstest.MapFS{
		"lang/de.json":    &fstest.MapFile{Data: []byte(`{"Cart":"Warenkorb"}`)},
		"lang/fr-CH.json": &fstest.MapFile{Data: []byte(`{"Cart":"Panier (CH)"}`)},
	}
	read := func(name string) ([]byte, error) { return theme.ReadFile(name) }

	if got := loadCatalog(read, "de-CH").T("Cart"); got != "Warenkorb" {
		t.Errorf("de-CH without its own file = %q, want the German one", got)
	}
	if got := loadCatalog(read, "fr-CH").T("Cart"); got != "Panier (CH)" {
		t.Errorf("fr-CH with its own file = %q, want it preferred", got)
	}
	// fr.json does not exist. It must NOT reach for de.json: a French page in
	// German chrome is the defect this exists to remove.
	if got := loadCatalog(read, "fr").T("Cart"); got != "Cart" {
		t.Errorf("fr with no catalogue = %q, want the key — never another language", got)
	}
}

// An empty translation is a missing one.
//
// A catalogue generated from a spreadsheet is full of "": treating it as a
// translation would put blank buttons on the page, which is the one outcome
// worse than the wrong language.
func TestAnEmptyTranslationCountsAsMissing(t *testing.T) {
	theme := fstest.MapFS{
		"lang/de.json": &fstest.MapFile{Data: []byte(`{"Cart":"","Search":"Suche"}`)},
	}
	cat := loadCatalog(func(n string) ([]byte, error) { return theme.ReadFile(n) }, "de")
	if got := cat.T("Cart"); got != "Cart" {
		t.Errorf("empty translation = %q, want the key", got)
	}
	if cat.Has("Cart") {
		t.Error("an empty translation reports as present, so Check will not name it")
	}
}

// A catalogue that does not parse does not take the site down.
func TestABrokenCatalogueRendersTheKeysRatherThanFailing(t *testing.T) {
	theme := fstest.MapFS{
		"lang/de.json": &fstest.MapFile{Data: []byte(`{"Cart": "Warenkorb",}`)}, // trailing comma
	}
	cat := loadCatalog(func(n string) ([]byte, error) { return theme.ReadFile(n) }, "de")
	if got := cat.T("Cart"); got != "Cart" {
		t.Errorf("= %q, want the key: a comma must not be able to take a site down", got)
	}
}

// Check names the words a theme mints and does not translate.
//
// Without this the whole mechanism is silent in exactly the way §2.5 warned
// about: four English words in the middle of a French page, every gate green.
func TestCheckNamesTheUntranslatedWords(t *testing.T) {
	theme := fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte(
			`<html><body><a>{{t "Cart"}}</a>{{block "content" .}}{{end}}</body></html>`)},
		"page.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<p>{{t "Search"}} {{tf "Page %d" 1}}</p>{{end}}`)},
		"lang/fr.json": &fstest.MapFile{Data: []byte(`{"Cart":"Panier"}`)},
	}

	problems := CheckCatalogs(theme)
	if len(problems) != 1 {
		t.Fatalf("%d problems, want 1:\n%v", len(problems), problems)
	}
	p := problems[0].String()
	for _, want := range []string{"lang/fr.json", "2 of 3", "Page %d", "Search"} {
		if !strings.Contains(p, want) {
			t.Errorf("the report does not carry %q:\n%s", want, p)
		}
	}
	if strings.Contains(p, `"Cart"`) {
		t.Errorf("a translated word is named as missing:\n%s", p)
	}
}

// A theme that asks for words and carries no catalogue at all is told so once,
// not once per word.
func TestAThemeWithNoCatalogueIsToldOnce(t *testing.T) {
	theme := fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte(`{{t "Cart"}}{{t "Search"}}{{t "Menu"}}`)},
	}
	problems := CheckCatalogs(theme)
	if len(problems) != 1 {
		t.Fatalf("%d problems, want exactly 1:\n%v", len(problems), problems)
	}
	if !strings.Contains(problems[0].String(), "3 translatable words") {
		t.Errorf("the count is not in the message:\n%s", problems[0])
	}
}

// A theme that mints no words at all is not nagged about catalogues. Most
// themes written before this existed are in exactly that state and they still
// work.
func TestAThemeThatMintsNoWordsIsLeftAlone(t *testing.T) {
	theme := fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte(`<html><body>{{.Page.Title}}</body></html>`)},
	}
	if problems := CheckCatalogs(theme); len(problems) != 0 {
		t.Fatalf("%d problems, want none:\n%v", len(problems), problems)
	}
}

// The keys are read out of the source, in every spelling of the call.
func TestEverySpellingOfTheCallIsFound(t *testing.T) {
	theme := fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte(`
			{{t "plain"}}
			{{- t "trimmed" }}
			{{th "with markup"}}
			{{tf "with %d values" 2}}
			<a title="{{t "in an attribute"}}">x</a>
			{{if .X}}{{t "inside a branch"}}{{end}}
		`)},
	}
	got := KeysUsed(theme)
	want := []string{"in an attribute", "inside a branch", "plain", "trimmed",
		"with %d values", "with markup"}
	if len(got) != len(want) {
		t.Fatalf("found %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("found %v, want %v", got, want)
		}
	}
}

// A leftover entry is worth saying and never worth refusing.
func TestALeftoverEntryIsReportedButDoesNotRefuse(t *testing.T) {
	theme := fstest.MapFS{
		"layout.html":  &fstest.MapFile{Data: []byte(`{{t "Cart"}}`)},
		"lang/de.json": &fstest.MapFile{Data: []byte(`{"Cart":"Warenkorb","Basket":"Korb"}`)},
	}
	problems := CheckCatalogs(theme)
	if len(problems) != 1 {
		t.Fatalf("%d problems, want 1:\n%v", len(problems), problems)
	}
	if !strings.Contains(problems[0].String(), "Basket") {
		t.Errorf("the leftover entry is not named:\n%s", problems[0])
	}
}

// A word the program mints is not left over, and is not demanded either.
//
// Render404 fills "Page not found" into {{.Page.Title}}, so no template
// contains it — yet a catalogue that carries it is right to. A theme that
// leaves it out is not nagged, because the fallback is the key, which is
// English and readable; a theme that carries it is not told it has a spare
// entry, because it does not.
func TestAWordTheProgramMintsIsNeitherLeftOverNorDemanded(t *testing.T) {
	carries := fstest.MapFS{
		"layout.html":  &fstest.MapFile{Data: []byte(`{{t "Cart"}}`)},
		"lang/de.json": &fstest.MapFile{Data: []byte(`{"Cart":"Warenkorb","Page not found":"Seite nicht gefunden"}`)},
	}
	if problems := CheckCatalogs(carries); len(problems) != 0 {
		t.Errorf("a catalogue carrying a program-minted word was reported:\n%v", problems)
	}

	leaves := fstest.MapFS{
		"layout.html":  &fstest.MapFile{Data: []byte(`{{t "Cart"}}`)},
		"lang/de.json": &fstest.MapFile{Data: []byte(`{"Cart":"Warenkorb"}`)},
	}
	if problems := CheckCatalogs(leaves); len(problems) != 0 {
		t.Errorf("a theme was nagged about a word no template of its own asks for:\n%v", problems)
	}
}
