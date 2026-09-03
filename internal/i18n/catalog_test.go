package i18n

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
)

// A translation whose placeholders do not match the German is worse than none:
// fmt fills the arguments in positionally, so a missing %s silently swallows a
// website's name and an extra one prints "%!s(MISSING)" on somebody's screen.
func TestPlaceholdersMatchTheGerman(t *testing.T) {
	for lang, catalog := range readCatalogs(t) {
		for german, translated := range catalog {
			want, got := verbs(german), verbs(translated)
			if want != got {
				t.Errorf("%s: placeholders differ\n  de: %q (%s)\n  %s: %q (%s)",
					lang, german, want, lang, translated, got)
			}
		}
	}
}

// The catalogue is a German-to-other-language dictionary, so a value identical
// to its key is usually a line somebody forgot. Not always — "Text" is "Text"
// in English — so this only reports, loudly, rather than failing.
func TestUntouchedEntriesAreReported(t *testing.T) {
	for lang, catalog := range readCatalogs(t) {
		var same []string
		for german, translated := range catalog {
			if german == translated {
				same = append(same, german)
			}
		}
		t.Logf("%s: %d of %d entries are identical to the German", lang, len(same), len(catalog))
	}
}

// An entry with no value at all should not be in the file: the tool writes one
// when a string is new, and it is a reminder to come back, not a state to ship.
func TestNoEmptyTranslationsShip(t *testing.T) {
	for lang, catalog := range readCatalogs(t) {
		for german, translated := range catalog {
			if strings.TrimSpace(translated) == "" {
				t.Errorf("%s: %q has no translation — run: go run ./tools/i18n -write", lang, german)
			}
		}
	}
}

// A key is a whole German sentence, so a regional fassung is written by copying
// one — and a copy with a word missing is a line that silently does nothing.
// Every key of a fassung has to exist in the full catalogues.
func TestFassungKeysExistInTheSource(t *testing.T) {
	catalogs := readCatalogs(t)

	source := map[string]bool{}
	for lang, catalog := range catalogs {
		if Base(lang) != "" {
			continue
		}
		for german := range catalog {
			source[german] = true
		}
	}
	if len(source) == 0 {
		t.Fatal("no full catalogue to compare against")
	}

	for lang, catalog := range catalogs {
		if Base(lang) == "" {
			continue
		}
		for german := range catalog {
			if !source[german] {
				t.Errorf("%s: %q is not a sentence this administration says — "+
					"run: go run ./tools/i18n", lang, german)
			}
		}
	}
}

// Switzerland writes no ß, in any of its languages. A single one in a Swiss
// fassung is the mistake this file exists to prevent.
func TestSwissFassungenCarryNoSharpS(t *testing.T) {
	for lang, catalog := range readCatalogs(t) {
		if Region(lang) != "CH" {
			continue
		}
		for german, translated := range catalog {
			if strings.Contains(translated, "ß") {
				t.Errorf("%s: %q has a ß in it: %q", lang, german, translated)
			}
		}
	}
}

// readCatalogs reads the shipped files straight off the embedded FS, before
// load() has cleaned anything up — this is about what is in the repository.
func readCatalogs(t *testing.T) map[string]map[string]string {
	t.Helper()
	out := map[string]map[string]string{}

	names, err := fs.Glob(catalogFS, "locales/*.json")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, name := range names {
		data, err := catalogFS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var msgs map[string]string
		if err := json.Unmarshal(data, &msgs); err != nil {
			t.Fatalf("%s is not valid JSON: %v", name, err)
		}
		lang := strings.TrimSuffix(strings.TrimPrefix(name, "locales/"), ".json")
		out[lang] = msgs
	}
	if len(out) == 0 {
		t.Fatal("no catalogues found")
	}
	return out
}
