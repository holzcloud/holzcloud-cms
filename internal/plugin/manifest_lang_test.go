package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// shippedLanguages are the four this repository translates every operator-facing
// string into. A shipped plugin that carries three of them is one that says its
// own name in English to a quarter of the people who installed it.
var shippedLanguages = []string{"de", "es", "fr", "it"}

// A manifest with no catalogue behaves exactly as it did before v2.3.
//
// This is the whole compatibility promise for a plugin somebody else wrote: not
// declaring a translation must cost nothing, and must never turn a name into an
// empty string in a language the author never heard of.
func TestAManifestWithoutACatalogueIsUnchanged(t *testing.T) {
	m := Manifest{
		Name:        "Guestbook",
		Description: "Takes entries and shows them.",
		Admin:       &AdminEntry{Label: "Entries"},
	}
	for _, lang := range []string{"", "de", "fr", "kl", "de-CH"} {
		if got := m.NameIn(lang); got != m.Name {
			t.Errorf("NameIn(%q) = %q, want the source %q", lang, got, m.Name)
		}
		if got := m.DescriptionIn(lang); got != m.Description {
			t.Errorf("DescriptionIn(%q) = %q, want the source", lang, got)
		}
		if got := m.AdminLabelIn(lang); got != "Entries" {
			t.Errorf("AdminLabelIn(%q) = %q, want the source", lang, got)
		}
	}
}

// Every way a lookup can miss ends at the source sentence and never at "".
func TestAMissingTranslationFallsBackToTheSource(t *testing.T) {
	m := Manifest{
		Name:        "Guestbook",
		Description: "Takes entries and shows them.",
		Admin:       &AdminEntry{Label: "Entries"},
		Lang: map[string]map[string]string{
			// German has the name and not the rest.
			"de": {"Guestbook": "Gästebuch"},
			// An empty value is a key somebody started and did not finish. It
			// must not reach a screen as an empty heading.
			"fr": {"Guestbook": ""},
		},
	}
	if got := m.NameIn("de"); got != "Gästebuch" {
		t.Errorf("NameIn(de) = %q", got)
	}
	if got := m.DescriptionIn("de"); got != m.Description {
		t.Errorf("a language that carries only the name lost the description: %q", got)
	}
	if got := m.AdminLabelIn("de"); got != "Entries" {
		t.Errorf("a language that carries only the name lost the label: %q", got)
	}
	if got := m.NameIn("fr"); got != "Guestbook" {
		t.Errorf("an empty translation reached the screen: %q", got)
	}
	if got := m.NameIn("kl"); got != "Guestbook" {
		t.Errorf("an unknown language did not fall back: %q", got)
	}
}

// A regional tag asks its base language first.
//
// de-CH is a real operator language in this program — internal/i18n generates it
// — and a Swiss operator reading everything else in German must not meet English
// plugin names in the sidebar.
func TestARegionalTagFallsBackToItsBaseLanguage(t *testing.T) {
	m := Manifest{
		Name: "Guestbook",
		Lang: map[string]map[string]string{"de": {"Guestbook": "Gästebuch"}},
	}
	if got := m.NameIn("de-CH"); got != "Gästebuch" {
		t.Errorf("NameIn(de-CH) = %q; a regional tag must ask its base language", got)
	}
	// And the regional catalogue still wins where it exists.
	m.Lang["de-CH"] = map[string]string{"Guestbook": "Gästebuech"}
	if got := m.NameIn("de-CH"); got != "Gästebuech" {
		t.Errorf("NameIn(de-CH) = %q; the regional catalogue must win", got)
	}
}

// A plugin with no admin screen has no label, in any language.
func TestAPluginWithoutAScreenHasNoLabel(t *testing.T) {
	m := Manifest{Name: "Search"}
	if got := m.AdminLabelIn("de"); got != "" {
		t.Errorf("AdminLabelIn = %q for a plugin with no screen", got)
	}
	if got := m.TranslatableStrings(); len(got) != 1 || got[0] != "Search" {
		t.Errorf("TranslatableStrings = %q, want only the name", got)
	}
}

// The shipped plugins carry all four languages for all of their own strings.
//
// Not a style rule: these five are what an operator meets on the day they
// install this program, and four of them said their name in German on an
// English installation from v2.0 until this was fixed (ledger WIN-08). A sixth
// plugin written by somebody else fails nothing here — this reads the packages
// in this repository and no others.
func TestTheShippedPluginsTranslateTheirOwnStrings(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "..", "plugins", "*", ManifestName))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(dirs) == 0 {
		t.Fatal("no shipped plugin manifests found; this test is checking nothing")
	}

	for _, path := range dirs {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		m, err := ParseManifest(raw)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}

		t.Run(m.ID, func(t *testing.T) {
			want := map[string]bool{}
			for _, s := range m.TranslatableStrings() {
				want[s] = true
			}
			for _, lang := range shippedLanguages {
				cat, ok := m.Lang[lang]
				if !ok {
					t.Errorf("no %s catalogue; the sidebar says %q to a %s operator", lang, m.Name, lang)
					continue
				}
				for s := range want {
					if v, ok := cat[s]; !ok || v == "" {
						t.Errorf("%s: %q is not translated", lang, short(s))
					}
				}
				for k := range cat {
					if !want[k] {
						t.Errorf("%s: %q is translated but is not a string this manifest declares — "+
							"an orphan, the same thing `tools/i18n` reports for the program's own catalogues",
							lang, short(k))
					}
				}
			}
			// The source is English since v2.0. A shipped manifest whose own
			// name needs no German translation is one that is already German.
			if de, ok := m.Lang["de"]; ok {
				if de[m.Name] == m.Name && m.Name != "" {
					t.Errorf("the German for %q is %q — the source is not English", m.Name, de[m.Name])
				}
			}
		})
	}
}

// A manifest carrying a catalogue survives the round trip through JSON, because
// that is how it actually arrives: read from an archive, stored in a column,
// read back.
func TestTheCatalogueSurvivesTheRoundTrip(t *testing.T) {
	m := Manifest{
		ID: "gaestebuch", ABI: ABIVersion, Name: "Guestbook", Version: "1.0.0",
		Hooks: []string{HookContent},
		Lang:  map[string]map[string]string{"de": {"Guestbook": "Gästebuch"}},
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	back, err := ParseManifest(raw)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if got := back.NameIn("de"); got != "Gästebuch" {
		t.Errorf("after the round trip NameIn(de) = %q", got)
	}
}

func short(s string) string {
	if len(s) > 48 {
		return s[:48] + "…"
	}
	return s
}

// The two paths a shipped plugin's words take, in two languages.
//
// Two and not one: a test that only checks German passes on a build that
// ignores the language argument entirely and always answers German. The plugin
// list and the screen title read the manifest; the sidebar reads AdminLink,
// which carries the catalogue with it so that drawing a menu costs no lock.
func TestAShippedPluginSaysItsNameInTheOperatorsLanguage(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "plugins", "bestellung", ManifestName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	m, err := ParseManifest(raw)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

	link := AdminLink{PluginID: m.ID, Label: m.Admin.Label, lang: m.Lang}

	for _, tc := range []struct{ lang, name, label string }{
		{"de", "Hofladen", "Bestellungen"},
		{"fr", "Vente à la ferme", "Commandes"},
		// The source, for a language nobody translated into.
		{"kl", "Farm shop", "Orders"},
	} {
		if got := m.NameIn(tc.lang); got != tc.name {
			t.Errorf("NameIn(%s) = %q, want %q", tc.lang, got, tc.name)
		}
		if got := m.AdminLabelIn(tc.lang); got != tc.label {
			t.Errorf("AdminLabelIn(%s) = %q, want %q", tc.lang, got, tc.label)
		}
		if got := link.LabelIn(tc.lang); got != tc.label {
			t.Errorf("the sidebar says %q in %s, want %q — AdminLink lost the catalogue", got, tc.lang, tc.label)
		}
		if got := m.DescriptionIn(tc.lang); got == "" {
			t.Errorf("DescriptionIn(%s) is empty", tc.lang)
		}
	}
}
