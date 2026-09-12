package i18n

import (
	"strings"
	"testing"
)

// TestNoLongSentenceIsCopiedAsItsOwnTranslation (threat T-10-51). A source
// sentence pasted as its own translation counts as translated: the gate says
// 0 offen, and the screen says it in the wrong language. Plan 10-09 scanned for
// it once; nothing held it afterwards. Short identical values are left alone on
// purpose — a product name or "Holzcloud" is the same in every language.
//
// The list is de, es, fr, it since v2.0: the source is English, so en.json is
// gone and de.json is new. German is the one most at risk of this, because it
// is the language every sentence in this repository was written in first.
func isCodeSample(s string) bool {
	return strings.HasPrefix(s, "<code>") && strings.HasSuffix(s, "</code>") &&
		!strings.Contains(strings.TrimSuffix(strings.TrimPrefix(s, "<code>"), "</code>"), "<")
}

func TestNoLongSentenceIsCopiedAsItsOwnTranslation(t *testing.T) {
	cats := readCatalogs(t)
	for _, lang := range []string{"de", "es", "fr", "it"} {
		cat, ok := cats[lang]
		if !ok {
			t.Fatalf("no %s catalogue", lang)
		}
		for key, value := range cat {
			if len(key) <= 40 || value != key {
				continue
			}
			// A key that is nothing but a code sample is the same in every
			// language, however long it is: the Markdown for an image is
			// Markdown in French too. Only the ALT text inside it is prose, and
			// a translator who wants to localise the example may — this just
			// does not demand it.
			if isCodeSample(key) {
				continue
			}
			t.Errorf("%s.json: %q is its own translation", lang, key)
		}
	}
}
