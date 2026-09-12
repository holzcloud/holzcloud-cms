package admin

import (
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/csvimport"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// The example file has to come back.
//
// An operator downloads it, fills it in and uploads it, and since v2.0 its
// heading row is written in their language. That makes the catalogue and
// csvimport.fixedSpellings two halves of one contract: translate a heading
// without teaching the importer the new spelling and the operator gets a file
// this program does not recognise — every column unmapped, from the very
// download the screen told them to use.
//
// The failure this test was written after is the shape to remember. The
// catalogue translated "Terms" as "Labels", because it also translated
// "Beschriftung" that way — a collision .planning/GLOSSARY.md had recorded as a
// defect long before. Nothing was broken by translating the heading; the break
// was already in the catalogue, and putting the word on a file that has to come
// back is what made it cost something.
func TestEveryTranslatedExampleHeadingIsRecognisedOnTheWayBackIn(t *testing.T) {
	for _, lang := range append([]string{i18n.Source}, languageCodes(t)...) {
		header, _ := csvExampleColumns(nil, lang)
		if len(header) != 5 {
			t.Fatalf("%s: the example has %d fixed columns, expected 5", lang, len(header))
		}
		m := csvimport.AutoMap(header, nil)
		for column, heading := range header {
			if got := m.Targets[column].Kind; got == csvimport.TargetNone {
				t.Errorf("%s: the heading %q of the example file is not recognised on the way "+
					"back in — csvimport.fixedSpellings has no entry that folds from it, so an "+
					"operator who fills in this very file gets an unmapped column",
					lang, heading)
			}
		}
		// And all five have to land on five DIFFERENT targets: two headings
		// folding to one key would leave a column silently unmapped with the
		// other one holding the target.
		seen := map[string]bool{}
		for _, target := range m.Targets {
			if target.Kind == csvimport.TargetNone {
				continue
			}
			if seen[target.Kind] {
				t.Errorf("%s: two headings of the example both claim the target %q", lang, target.Kind)
			}
			seen[target.Kind] = true
		}
	}
}

// The other half of the same contract, stated so a failure says which word.
//
// A heading is recognised through field.SlugifyKey, so what has to be in
// fixedSpellings is the FOLDED spelling and not the word as it is written:
// "Título" arrives as "titulo" and "Étiquettes" as "etiquettes".
func TestTheFoldedSpellingOfEveryExampleHeadingIsKnown(t *testing.T) {
	for _, lang := range append([]string{i18n.Source}, languageCodes(t)...) {
		header, _ := csvExampleColumns(nil, lang)
		for _, heading := range header {
			folded := field.SlugifyKey(heading)
			if folded == "" {
				t.Errorf("%s: the heading %q folds to nothing", lang, heading)
			}
			if !csvimport.KnownHeading(folded) {
				t.Errorf("%s: %q folds to %q, which csvimport.fixedSpellings does not carry",
					lang, heading, folded)
			}
		}
	}
}

func languageCodes(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, l := range i18n.Languages() {
		if l.Code != i18n.Source {
			out = append(out, l.Code)
		}
	}
	if len(out) == 0 {
		t.Fatalf("no languages besides the source; this test would prove nothing")
	}
	return out
}
