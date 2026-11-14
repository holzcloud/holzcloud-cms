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

func TestFrenchPrivacyTemplateNamesItsAuthorities(t *testing.T) {
	enfrWant(t, "fr", map[legal.Framework][]string{
		legal.EU: {"autorité de contrôle", "RGPD", "règlement (UE) 2016/679", "directive 2002/58/CE", "art. 77", "art. 21", "art. 28"},
		legal.CH: {"Préposé fédéral à la protection des données et à la transparence (PFPDT)", "LPD", "RS 235.1", "art. 45c", "30 jours"},
		legal.UK: {"Information Commission", "164A", "section 165", "Schedule A1", "PECR"},
		legal.US: {"45 jours", "Global Privacy Control", "Sec-GPC: 1", "Do Not Track", "ni vendues ni partagées"},
	})
}

// The French US section says what Global Privacy Control changes, like the
// German and the English one.
func TestFrenchUSSectionSaysWhatGPCChanges(t *testing.T) {
	text := enfrSection("fr", legal.US)
	us := text[strings.Index(text, heading(t, "fr", "us")):]
	for _, w := range []string{
		"comme une demande d’opposition à la vente et au partage de vos données personnelles",
		"Rien ne change sur le site pour autant, car rien n’est vendu ni partagé",
		"indique que le signal a été reconnu",
		"nom, adresse e-mail et remarque",
		"ni votre adresse IP ni un identifiant de votre navigateur",
		"conservées " + Fillin("fr"),
	} {
		if !strings.Contains(us, w) {
			t.Errorf("the French US section lacks %q", w)
		}
	}
}

// A page in the French of Switzerland gets the French text. The Swiss switch
// only changes German spelling, so it leaves the French text as it is.
func TestFrenchFollowsSwissFrenchPages(t *testing.T) {
	for _, tag := range []string{"fr", "fr-CH", "fr-FR", "FR-ch"} {
		if got := TextLang(KindPrivacy, tag); got != "fr" {
			t.Errorf("TextLang(privacy, %q) = %q, want fr", tag, got)
		}
	}
	for _, set := range sets() {
		for _, f := range []Features{{}, allFeatures()} {
			fr := ComposePrivacy(Request{Lang: "fr", Set: set, Features: f})
			for _, swiss := range []bool{false, true} {
				if got := ComposePrivacy(Request{Lang: "fr-CH", Swiss: swiss, Set: set, Features: f}); got != fr {
					t.Errorf("%s swiss=%v: the fr-CH text is not the fr text", set, swiss)
				}
			}
			if !strings.Contains(fr, "**"+TemplateNotice("fr")+"**") {
				t.Errorf("%s: the French text does not open with the French notice", set)
			}
		}
	}
}

func TestFrenchSlugsAreRoles(t *testing.T) {
	if got := Slug(KindPrivacy, "fr"); got != "protection-des-donnees" {
		t.Errorf("Slug(privacy, fr) = %q, want protection-des-donnees", got)
	}
	if got := DraftSlug(KindPrivacy, "fr-CH"); got != "protection-des-donnees-modele" {
		t.Errorf("DraftSlug(privacy, fr-CH) = %q, want protection-des-donnees-modele", got)
	}
	roles := RoleSlugs(KindPrivacy)
	for _, s := range []string{"protection-des-donnees", "confidentialite", "politique-de-confidentialite"} {
		if !slices.Contains(roles, s) {
			t.Errorf("RoleSlugs = %v, want %q among them", roles, s)
		}
	}
}

// French typography: a typographic apostrophe, a no-break space before the
// high punctuation marks and inside the angle quotation marks, and no HTML
// entity for either, because the text is Markdown and an entity would be
// printed as written in one place and decoded in another.
func TestFrenchTypography(t *testing.T) {
	if TextLang(KindPrivacy, "fr") != "fr" {
		t.Fatal("there is no French privacy text")
	}
	text := ComposePrivacy(Request{Lang: "fr", Set: allSet(), Features: allFeatures()})
	for _, bad := range []string{"'", " :", " ;", " ?", " !", " %", "&nbsp;", "&#", "« ", " »"} {
		if i := strings.Index(text, bad); i >= 0 {
			t.Errorf("the French text contains %q near %q", bad, text[max(0, i-30):min(len(text), i+30)])
		}
	}
}
