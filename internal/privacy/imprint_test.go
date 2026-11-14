package privacy

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
)

// The imprint tests read the headings from the files and iterate every
// language that has an imprint directory, so a language added later is
// checked without an edit here.

// imprintFrameworks are the framework blocks of the imprint, in their order.
// The US has no imprint duty, so it has no block (D-09).
var imprintFrameworks = []legal.Framework{legal.EU, legal.CH, legal.UK}

// imprintAnchors are the citations each framework block carries in every
// language, compared case-insensitively; one of the spellings is enough. The
// Swiss classified compilation is the SR in German, English and Romansh and
// the RS in French and Italian, and an imprint should cite it the way its
// readers find it.
var imprintAnchors = map[legal.Framework][]string{
	legal.EU: {"2000/31"},
	legal.CH: {"sr 241", "rs 241"},
	legal.UK: {"2002/2013"},
}

// imprintCites reports whether the lower-cased text carries one of the
// anchors of the framework.
func imprintCites(lower string, fw legal.Framework) bool {
	for _, a := range imprintAnchors[fw] {
		if strings.Contains(lower, a) {
			return true
		}
	}
	return false
}

// imprintHeadings lists the ## headings of a composed imprint, in order.
func imprintHeadings(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			out = append(out, line)
		}
	}
	return out
}

// imprintLanguages lists the languages with imprint texts and fails when there
// is none, so no test here passes by checking nothing.
func imprintLanguages(t *testing.T) []string {
	t.Helper()
	langs := languagesOf(t, KindImprint)
	if len(langs) == 0 {
		t.Fatal("no language has imprint texts")
	}
	return langs
}

func TestEachFrameworkAddsItsImprintBlock(t *testing.T) {
	langs := imprintLanguages(t)
	n := 0
	for _, lang := range langs {
		heading := map[legal.Framework]string{}
		for _, fw := range imprintFrameworks {
			h := firstLine(t, lang, KindImprint, string(fw))
			if !strings.HasPrefix(h, "## ") {
				t.Errorf("%s/imprint/%s.md opens with %q, want a ## heading", lang, fw, h)
			}
			heading[fw] = "\n" + h + "\n"
		}
		contact := "\n" + firstLine(t, lang, KindImprint, "contact") + "\n"
		for _, set := range sets() {
			text := ComposeImprint(Request{Lang: lang, Set: set})
			n++
			lower := strings.ToLower(text)
			last := strings.Index(text, contact)
			if last < 0 {
				t.Fatalf("%s %q: no contact heading", lang, set)
			}
			end := strings.LastIndex(text, "\n---\n")
			if end < 0 {
				t.Fatalf("%s %q: no closing rule", lang, set)
			}
			for _, fw := range imprintFrameworks {
				want := 0
				if set.Has(fw) {
					want = 1
				}
				if got := strings.Count(text, heading[fw]); got != want {
					t.Errorf("%s %q: imprint block %s occurs %d times, want %d", lang, set, fw, got, want)
					continue
				}
				if has := imprintCites(lower, fw); has != set.Has(fw) {
					t.Errorf("%s %q: anchor %q present = %v, want %v", lang, set, imprintAnchors[fw], has, set.Has(fw))
				}
				if want == 1 {
					i := strings.Index(text, heading[fw])
					if i < last {
						t.Errorf("%s %q: block %s stands before the contact block or an earlier framework block", lang, set, fw)
					}
					if i > end {
						t.Errorf("%s %q: block %s stands after the closing", lang, set, fw)
					}
					last = i
				}
			}
			if lang == "de" && set.Has(legal.EU) && !strings.Contains(text, "§ 5 DDG") {
				t.Errorf("de %q: the EU block does not name the German provision", set)
			}
		}
	}
	t.Logf("checked %d imprint compositions", n)
}

func TestTheLeanImprintIsIdentityAndContact(t *testing.T) {
	for _, lang := range imprintLanguages(t) {
		want := []string{
			firstLine(t, lang, KindImprint, "glance"),
			firstLine(t, lang, KindImprint, "identity"),
			firstLine(t, lang, KindImprint, "contact"),
		}
		closing := firstLine(t, lang, KindImprint, "closing")
		empty := ComposeImprint(Request{Lang: lang, Set: legal.ParseSet("")})
		for _, set := range []legal.Set{legal.ParseSet(""), legal.ParseSet("us")} {
			text := ComposeImprint(Request{Lang: lang, Set: set})
			if got := imprintHeadings(text); !slices.Equal(got, want) {
				t.Errorf("%s %q: headings %q, want exactly glance, identity and contact %q", lang, set, got, want)
			}
			if !strings.Contains(text, "\n"+closing+"\n") {
				t.Errorf("%s %q: the closing block is missing", lang, set)
			}
			if text != empty {
				t.Errorf("%s %q: the US set changes the imprint; it has no imprint duty", lang, set)
			}
		}
	}
}

// The EU platform for online dispute resolution was closed: Regulation (EU)
// No 524/2013 was repealed with effect from 20 July 2025 (D-16).
func TestTheImprintNeverMentionsTheODRPlatform(t *testing.T) {
	bad := []string{"odr", "online-streitbeilegung", "streitbeilegungsplattform", "ec.europa.eu", "524/2013"}
	n := 0
	for _, lang := range imprintLanguages(t) {
		for _, set := range sets() {
			for _, swiss := range []bool{false, true} {
				text := strings.ToLower(ComposeImprint(Request{Lang: lang, Swiss: swiss, Set: set}))
				n++
				for _, b := range bad {
					if strings.Contains(text, b) {
						t.Errorf("%s %q: the imprint mentions %q", lang, set, b)
					}
				}
			}
		}
	}
	t.Logf("checked %d imprint compositions", n)
}

// Name, address, e-mail, register and VAT numbers are fill-in markers. The
// website's stored address and VAT number are shown next to the draft, never
// written into it, so Markdown an operator typed cannot reach the template.
func TestTheImprintTakesNoOperatorString(t *testing.T) {
	for _, lang := range imprintLanguages(t) {
		for _, set := range sets() {
			a := ComposeImprint(Request{Lang: lang, Set: set})
			b := ComposeImprint(Request{Lang: lang, Set: set, Features: allFeatures()})
			if a != b {
				t.Errorf("%s %q: the imprint depends on the website's functions", lang, set)
			}
			if a == "" {
				t.Errorf("%s %q: empty imprint", lang, set)
			}
		}
	}
	src, err := os.ReadFile("imprint.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"domain.", "internal/domain", "VATNumber", "Address"} {
		if strings.Contains(string(src), bad) {
			t.Errorf("imprint.go mentions %q: the imprint must not read a stored website field", bad)
		}
	}
}

func TestTheImprintExistsInEveryLanguage(t *testing.T) {
	want := map[string][2]string{
		"de": {"Impressum", "impressum"},
		"en": {"Imprint", "imprint"},
		"fr": {"Mentions légales", "mentions-legales"},
		"it": {"Note legali", "note-legali"},
		"rm": {"Impressum", "impressum"},
	}
	langs := imprintLanguages(t)
	roles := RoleSlugs(KindImprint)
	for lang, ts := range want {
		if !slices.Contains(langs, lang) {
			t.Errorf("%s has no imprint texts", lang)
			continue
		}
		if got := Title(KindImprint, lang); got != ts[0] {
			t.Errorf("Title(imprint, %s) = %q, want %q", lang, got, ts[0])
		}
		if got := Slug(KindImprint, lang); got != ts[1] {
			t.Errorf("Slug(imprint, %s) = %q, want %q", lang, got, ts[1])
		}
		if !slices.Contains(roles, ts[1]) {
			t.Errorf("RoleSlugs(imprint) = %v, lacks %q", roles, ts[1])
		}
		if d := DraftSlug(KindImprint, lang); d == "" || d == ts[1] {
			t.Errorf("DraftSlug(imprint, %s) = %q, want a slug of its own", lang, d)
		}
	}
	if !slices.Contains(roles, "legal-notice") {
		t.Errorf("RoleSlugs(imprint) = %v, lacks legal-notice", roles)
	}
}

// Each language names the Swiss act by the abbreviation SR 241 uses in it.
func TestTheImprintNamesTheSwissActInEachLanguage(t *testing.T) {
	want := map[string][]string{
		"de": {"UWG"},
		"fr": {"LCD"},
		"it": {"LCSl"},
		"en": {"Unfair Competition Act", "UWG"},
		"rm": {"UWG"},
	}
	for lang, names := range want {
		if TextLang(KindImprint, lang) != lang {
			t.Errorf("%s has no imprint texts", lang)
			continue
		}
		text := ComposeImprint(Request{Lang: lang, Set: legal.ParseSet("ch")})
		for _, name := range names {
			if !strings.Contains(text, name) {
				t.Errorf("%s with CH chosen: the imprint does not name %q", lang, name)
			}
		}
		if lean := ComposeImprint(Request{Lang: lang, Set: legal.ParseSet("")}); imprintCites(strings.ToLower(lean), legal.CH) {
			t.Errorf("%s without CH: the imprint cites SR 241", lang)
		}
	}
}
