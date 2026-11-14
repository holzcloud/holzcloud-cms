package privacy

import (
	"slices"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
)

// The English and French privacy templates. The structural and safety tests
// of compose_test.go and privacy_page_test.go already run over every language
// directory; what is checked here is what only a reader of the language can
// check: that each framework's section names its authority and its
// provisions in that language's official form.

// enfrSection is the privacy page in a language with one framework chosen and
// every function on, which is the text in which a section's sentences are
// hardest to miss.
func enfrSection(lang string, fw legal.Framework) string {
	return ComposePrivacy(Request{Lang: lang, Set: legal.ParseSet(string(fw)), Features: allFeatures()})
}

// enfrWant checks that the text with one framework chosen contains each phrase.
func enfrWant(t *testing.T, lang string, want map[legal.Framework][]string) {
	t.Helper()
	for _, fw := range frameworkBlocks {
		text := enfrSection(lang, fw)
		for _, w := range want[fw] {
			if !strings.Contains(text, w) {
				t.Errorf("with %s chosen the %s text lacks %q", fw, lang, w)
			}
		}
	}
}

func TestEnglishPrivacyTemplateNamesItsAuthorities(t *testing.T) {
	enfrWant(t, "en", map[legal.Framework][]string{
		legal.EU: {"supervisory authority", "Art. 77", "Art. 21", "Art. 28", "Regulation (EU) 2016/679", "Directive 2002/58/EC"},
		legal.CH: {"Federal Data Protection and Information Commissioner (FDPIC)", "non-official translation", "FADP", "Art. 45c", "30 days"},
		legal.UK: {"Information Commission", "section 164A", "section 165", "Schedule A1", "Data Protection Act 2018", "PECR"},
		legal.US: {"45 days", "Global Privacy Control", "Sec-GPC: 1", "Do Not Track", "not sold or shared"},
	})
}

// What Global Privacy Control changes, said in the English US section, as
// TestTheUSSectionSaysWhatGPCChanges asks it of the German one (D-10, D-11,
// D-12).
func TestEnglishUSSectionSaysWhatGPCChanges(t *testing.T) {
	text := enfrSection("en", legal.US)
	us := text[strings.Index(text, heading(t, "en", "us")):]
	for _, w := range []string{
		"as a request to opt out of the sale and sharing of your personal information",
		"Nothing on the website changes as a result, because nothing is sold or shared",
		"shows that the signal was recognised",
		"name, e-mail address and note",
		"neither your IP address nor an identifier of your browser",
		"kept " + Fillin("en"),
	} {
		if !strings.Contains(us, w) {
			t.Errorf("the English US section lacks %q", w)
		}
	}
}

// A Spanish website gets the English template (D-16).
func TestEnglishIsTheTemplateForSpanish(t *testing.T) {
	for _, tag := range []string{"es", "es-MX", "en-GB", "en-US"} {
		if got := TextLang(KindPrivacy, tag); got != "en" {
			t.Errorf("TextLang(privacy, %q) = %q, want en", tag, got)
		}
	}
	for _, set := range sets() {
		en := ComposePrivacy(Request{Lang: "en", Set: set, Features: allFeatures()})
		if es := ComposePrivacy(Request{Lang: "es", Set: set, Features: allFeatures()}); es != en {
			t.Errorf("%s: the Spanish text is not the English one", set)
		}
		if !strings.Contains(en, "**"+TemplateNotice("en")+"**") {
			t.Errorf("%s: the English text does not open with the English notice", set)
		}
	}
	if got := Title(KindPrivacy, "es"); got != "Privacy notice" {
		t.Errorf("Title(privacy, es) = %q, want Privacy notice", got)
	}
}

func TestEnglishSlugsAreRoles(t *testing.T) {
	if got := Slug(KindPrivacy, "en"); got != "privacy" {
		t.Errorf("Slug(privacy, en) = %q, want privacy", got)
	}
	if got := DraftSlug(KindPrivacy, "en"); got != "privacy-template" {
		t.Errorf("DraftSlug(privacy, en) = %q, want privacy-template", got)
	}
	roles := RoleSlugs(KindPrivacy)
	for _, s := range []string{"privacy", "privacy-policy", "datenschutz"} {
		if !slices.Contains(roles, s) {
			t.Errorf("RoleSlugs = %v, want %q among them", roles, s)
		}
	}
}

// enfrGerman are words that only a German block left untranslated would
// bring into an English or French text.
var enfrGerman = []string{"(eintragen)", "DSGVO", "Datenschutz", "Verantwortlich", " und ", " der ", " die ", " nicht ", "ß", "ä", "ö", "ü"}

// English and French texts carry no German. A block copied from the German
// directory and forgotten would pass every structural test, because it has
// the right heading level and the right placeholders.
func TestEnglishAndFrenchCarryNoGerman(t *testing.T) {
	for _, lang := range []string{"en", "fr"} {
		if TextLang(KindPrivacy, lang) != lang {
			continue
		}
		for _, set := range sets() {
			text := ComposePrivacy(Request{Lang: lang, Set: set, Features: allFeatures()})
			for _, w := range enfrGerman {
				if strings.Contains(text, w) {
					t.Errorf("%s %s: the text contains the German %q", lang, set, w)
				}
			}
		}
	}
}
