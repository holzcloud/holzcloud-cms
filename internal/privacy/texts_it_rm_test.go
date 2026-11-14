package privacy

import (
	"slices"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// The Italian and the Romansh privacy page (D-07). The structural and safety
// tests of compose_test.go and privacy_page_test.go cover every language with
// files; these tests hold what only these two texts have to say: the names of
// the authorities and the provisions a reader looks for, in the official Swiss
// Italian and EU Italian terms, and for Romansh the notice that a machine
// wrote it.

// itrmText composes the privacy page of a language with one framework chosen
// and every feature on.
func itrmText(lang string, fw legal.Framework) string {
	return ComposePrivacy(Request{Lang: lang, Set: legal.ParseSet(string(fw)), Features: allFeatures()})
}

func TestItalianPrivacyTemplateNamesItsAuthorities(t *testing.T) {
	want := map[legal.Framework][]string{
		legal.EU: {"autorità di controllo", "RGPD", "regolamento (UE) 2016/679", "direttiva 2002/58/CE"},
		legal.CH: {"Incaricato federale della protezione dei dati e della trasparenza (IFPDT)", "LPD", "30 giorni"},
		legal.UK: {"Information Commission", "164A"},
		legal.US: {"45 giorni", "Global Privacy Control", "Sec-GPC: 1", "Do Not Track"},
	}
	for _, fw := range frameworkBlocks {
		text := itrmText("it", fw)
		for _, w := range want[fw] {
			if !strings.Contains(text, w) {
				t.Errorf("with %s chosen the Italian text lacks %q", fw, w)
			}
		}
	}
}

// A page in Swiss Italian reads the Italian text. The region is dropped, and
// Swiss spelling is a rule of German only.
func TestItalianFollowsSwissItalianPages(t *testing.T) {
	if got := TextLang(KindPrivacy, "it-CH"); got != "it" {
		t.Fatalf("TextLang(privacy, it-CH) = %q, want it", got)
	}
	for _, set := range sets() {
		it := ComposePrivacy(Request{Lang: "it", Set: set, Features: allFeatures()})
		if !strings.Contains(it, TemplateNotice("it")) {
			t.Errorf("%s: the Italian text does not carry the Italian notice", set)
		}
		for _, swiss := range []bool{false, true} {
			ch := ComposePrivacy(Request{Lang: "it-CH", Swiss: swiss, Set: set, Features: allFeatures()})
			if ch != it {
				t.Errorf("%s: an it-CH page (Swiss %v) gets another text than an it page", set, swiss)
			}
		}
	}
}

func TestItalianSlugsAreRoles(t *testing.T) {
	if got := Slug(KindPrivacy, "it"); got != "protezione-dei-dati" {
		t.Errorf("Slug(privacy, it) = %q, want protezione-dei-dati", got)
	}
	if got := Title(KindPrivacy, "it"); got != "Dichiarazione sulla protezione dei dati" {
		t.Errorf("Title(privacy, it) = %q", got)
	}
	if got := DraftSlug(KindPrivacy, "it"); got != "protezione-dei-dati-modello" {
		t.Errorf("DraftSlug(privacy, it) = %q, want protezione-dei-dati-modello", got)
	}
	roles := RoleSlugs(KindPrivacy)
	for _, s := range []string{"protezione-dei-dati", "privacy", "informativa-privacy"} {
		if !slices.Contains(roles, s) {
			t.Errorf("RoleSlugs = %v, want %q among them", roles, s)
		}
	}
}

// Every Romansh composition says, in bold and right after the template notice,
// that a machine wrote it in Rumantsch Grischun and that a Romansh speaker must
// check it before it is published (D-07). No Romansh speaker has checked these
// texts; the notice is what makes that honest.
func TestRomanshCarriesTheMachineNotice(t *testing.T) {
	words := langFile(t, "rm")
	machine, notice := words["machine_notice"], words["template_notice"]
	if strings.TrimSpace(machine) == "" {
		t.Fatal("texts/rm/lang.json has no machine notice")
	}
	if TextLang(KindPrivacy, "rm") != "rm" {
		t.Fatalf("TextLang(privacy, rm) = %q: Romansh has no privacy texts of its own", TextLang(KindPrivacy, "rm"))
	}
	bold := "**" + machine + "**"
	for _, set := range sets() {
		for _, f := range []Features{{}, allFeatures()} {
			text := ComposePrivacy(Request{Lang: "rm", Set: set, Features: f})
			where := set.String() + " " + featureString(f)
			if n := strings.Count(text, machine); n != 1 {
				t.Errorf("%s: machine notice occurs %d times, want 1", where, n)
				continue
			}
			i := strings.Index(text, bold)
			if i < 0 {
				t.Errorf("%s: the machine notice is not in bold", where)
				continue
			}
			if i > 600 {
				t.Errorf("%s: machine notice at byte %d, want within the first 600", where, i)
			}
			if n := strings.Index(text, notice); n < 0 || n > i {
				t.Errorf("%s: the machine notice does not follow the template notice", where)
			}
		}
	}
}

func TestRomanshNamesTheSwissAuthority(t *testing.T) {
	want := map[legal.Framework][]string{
		legal.EU: {"RGPD", "2016/679"},
		legal.CH: {"IFPDT", "LPD", "235.1"},
		legal.UK: {"UK GDPR", "164A"},
		legal.US: {"Sec-GPC: 1", "CCPA", "CalOPPA"},
	}
	for _, fw := range frameworkBlocks {
		text := itrmText("rm", fw)
		for _, w := range want[fw] {
			if !strings.Contains(text, w) {
				t.Errorf("with %s chosen the Romansh text lacks %q", fw, w)
			}
		}
	}
}

// Romansh is never the main language of a website: the settings offer it
// nowhere, so a Romansh privacy page only ever arrives as an extra-language
// draft beside an original in the main language that the operator can compare
// it with.
func TestRomanshIsNeverAMainLanguage(t *testing.T) {
	for _, l := range tmpl.SupportedLocales {
		if baseOf(l.Code) == "rm" {
			t.Errorf("SupportedLocales offers %q as a main language", l.Code)
		}
	}
	if got := TextLang(KindPrivacy, "rm"); got != "rm" {
		t.Errorf("TextLang(privacy, rm) = %q, want rm", got)
	}
	if got := TextLang(KindPrivacy, "rm-CH"); got != "rm" {
		t.Errorf("TextLang(privacy, rm-CH) = %q, want rm", got)
	}
}

func TestRomanshSlugsAreRoles(t *testing.T) {
	if got := Slug(KindPrivacy, "rm"); got != "protecziun-da-datas" {
		t.Errorf("Slug(privacy, rm) = %q, want protecziun-da-datas", got)
	}
	if got := Title(KindPrivacy, "rm"); got != "Decleraziun davart la protecziun da datas" {
		t.Errorf("Title(privacy, rm) = %q", got)
	}
	if got := DraftSlug(KindPrivacy, "rm"); got != "protecziun-da-datas-model" {
		t.Errorf("DraftSlug(privacy, rm) = %q, want protecziun-da-datas-model", got)
	}
	if roles := RoleSlugs(KindPrivacy); !slices.Contains(roles, "protecziun-da-datas") {
		t.Errorf("RoleSlugs = %v, want protecziun-da-datas among them", roles)
	}
}
