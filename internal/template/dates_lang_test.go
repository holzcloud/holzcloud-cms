package template

import (
	"testing"
	"time"
)

// A date is written in the language the page is published in.
//
// Until v2.1 the table held German and English, so a French site printed
// "12. März 2026" under chrome that said "Panier" — the same defect as an //nolint:german — the date this rule fixes
// untranslated word, one layer down and harder to notice, because a date looks
// like data rather than like text. Spanish carries its two "de" in the layout
// and not in the month name; French and Italian keep their months lower case,
// which is not a typo a German eye should correct. //nolint:german — the date this rule fixes
func TestDatesInEveryLanguageTheProgramTranslatesInto(t *testing.T) {
	when := time.Date(2026, 3, 12, 9, 0, 0, 0, time.UTC)
	for _, c := range []struct{ locale, want string }{
		{"de", "12. März 2026"},
		{"en", "March 12, 2026"},
		{"fr", "12 mars 2026"},
		{"it", "12 marzo 2026"},
		{"es", "12 de marzo de 2026"},
	} {
		if got := DateText(c.locale, "UTC", when); got != c.want {
			t.Errorf("%s = %q, want %q", c.locale, got, c.want)
		}
	}
}
