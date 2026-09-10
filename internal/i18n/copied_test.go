package i18n

import "testing"

// TestNoLongSentenceIsCopiedAsItsOwnTranslation (threat T-10-51). A German
// sentence pasted as its own English value counts as translated: the gate says
// 0 offen, and the screen says it in German. Plan 10-09 scanned for it once;
// nothing held it afterwards. Short identical values are left alone on purpose
// — a product name or "Holzcloud" is the same in every language.
func TestNoLongSentenceIsCopiedAsItsOwnTranslation(t *testing.T) {
	cats := readCatalogs(t)
	for _, lang := range []string{"en", "es", "fr", "it"} {
		cat, ok := cats[lang]
		if !ok {
			t.Fatalf("no %s catalogue", lang)
		}
		for key, value := range cat {
			if len(key) > 40 && value == key {
				t.Errorf("%s.json: %q is its own translation", lang, key)
			}
		}
	}
}
