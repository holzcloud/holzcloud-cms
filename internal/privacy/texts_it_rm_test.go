package privacy

import (
	"slices"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
)

// The Italian privacy page (D-07). The structural and safety tests of
// compose_test.go and privacy_page_test.go cover every language with files;
// these tests hold what only the Italian text has to say: the names of the
// authorities and the provisions a reader looks for, in the official Swiss
// Italian and EU Italian terms.

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
