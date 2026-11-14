package privacy

import (
	"strings"
	"testing"
)

// The "Do Not Sell or Share My Personal Information" page (LEG-03, D-10).
// The general rules every composition keeps — notice first, no legal advice
// last, fill-ins, no link and no HTML — are compose_test.go's and hold for
// this kind because it registers itself. What is checked here is what only
// this page has to say.

// dnsMarker is the widget marker of plan 03, written out here and not taken
// from internal/public, which this package does not import.
const dnsMarker = "[[widerspruch]]"

// dnsStatutoryTitle is the wording of Cal. Civ. Code § 1798.135(a)(1). It is
// what a California visitor looks for, so it is the title in every language.
const dnsStatutoryTitle = "Do Not Sell or Share My Personal Information"

// dnsLanguages lists the languages with DNS texts and fails when there is
// none, so that no test here passes over an empty list.
func dnsLanguages(t *testing.T) []string {
	t.Helper()
	langs := languagesOf(t, KindDNS)
	if len(langs) == 0 {
		t.Fatal("no language has texts for the Do-Not-Sell page")
	}
	return langs
}

// The marker is where plan 03's widget goes. It must be there exactly once,
// because the widget draws one form per page, and it must stand alone between
// blank lines: only then does Markdown make it a paragraph of its own, which
// the widget replaces whole.
func TestTheDNSPageCarriesTheMarkerOnce(t *testing.T) {
	checked := 0
	for _, lang := range dnsLanguages(t) {
		for _, set := range sets() {
			for _, f := range combos() {
				text := ComposeDNS(Request{Lang: lang, Set: set, Features: f})
				checked++
				where := lang + " " + set.String() + " " + featureString(f)
				if n := strings.Count(text, dnsMarker); n != 1 {
					t.Errorf("%s: the marker occurs %d times, want 1", where, n)
					continue
				}
				lines := strings.Split(text, "\n")
				for i, line := range lines {
					if !strings.Contains(line, dnsMarker) {
						continue
					}
					if line != dnsMarker {
						t.Errorf("%s: the marker shares its line: %q", where, line)
					}
					if i == 0 || lines[i-1] != "" || i+1 >= len(lines) || lines[i+1] != "" {
						t.Errorf("%s: the marker is not a paragraph of its own", where)
					}
				}
			}
		}
	}
	t.Logf("%d compositions checked", checked)
}

// The page names the payment provider only when the website takes online
// payment: a page that names a processor the website does not use is as
// untruthful as one that hides one.
func TestTheDNSPageNamesThePaymentProviderOnlyWithPayment(t *testing.T) {
	for _, lang := range dnsLanguages(t) {
		for _, f := range combos() {
			text := ComposeDNS(Request{Lang: lang, Set: allSet(), Features: f})
			want := f.Shop && f.Payment
			if got := strings.Contains(text, "Payrexx"); got != want {
				t.Errorf("%s %s: names Payrexx = %v, want %v", lang, featureString(f), got, want)
			}
		}
	}
}

// Every language cites the law the page answers: the statute that names it,
// the regulation that excuses a business which sells nothing from offering it,
// the two sections it follows, and the signal by its name and its header.
func TestTheDNSPageCitesTheRegulations(t *testing.T) {
	anchors := []string{"1798.135", "7010", "7025", "7026", "sec-gpc", "global privacy control"}
	for _, lang := range dnsLanguages(t) {
		for _, f := range []Features{{}, allFeatures()} {
			text := strings.ToLower(ComposeDNS(Request{Lang: lang, Set: allSet(), Features: f}))
			for _, a := range anchors {
				if !strings.Contains(text, a) {
					t.Errorf("%s %s: the text does not cite %q", lang, featureString(f), a)
				}
			}
		}
	}
}

// The title is the statutory English wording in every language, and the slug
// is the same in every language too: slugs are unique per language, so the
// same address in several languages is allowed.
func TestTheDNSTitleIsTheStatutoryWording(t *testing.T) {
	tags := append(dnsLanguages(t), "de-CH", "es", "pt")
	for _, tag := range tags {
		if got := Title(KindDNS, tag); got != dnsStatutoryTitle {
			t.Errorf("Title(dns, %q) = %q, want %q", tag, got, dnsStatutoryTitle)
		}
		if got := Slug(KindDNS, tag); got != "do-not-sell-or-share" {
			t.Errorf("Slug(dns, %q) = %q, want do-not-sell-or-share", tag, got)
		}
		if got := DraftTitle(KindDNS, tag); !strings.HasPrefix(got, dnsStatutoryTitle+" (") {
			t.Errorf("DraftTitle(dns, %q) = %q, want the title with a note in brackets", tag, got)
		}
		if got := DraftSlug(KindDNS, tag); !strings.HasPrefix(got, "do-not-sell-or-share-") {
			t.Errorf("DraftSlug(dns, %q) = %q, want do-not-sell-or-share-…", tag, got)
		}
	}
}

// dnsPageLanguages are the five languages of D-07. Each has its own Do-Not-Sell
// text; none falls back to another.
var dnsPageLanguages = []string{"de", "en", "fr", "it", "rm"}

// Below the English title the page speaks its own language: the first
// heading differs from one language to the next, while the title stays the
// statutory wording in all five.
func TestTheDNSPageSpeaksThePageLanguage(t *testing.T) {
	have := map[string]bool{}
	for _, l := range dnsLanguages(t) {
		have[l] = true
	}
	seen := map[string]string{}
	for _, lang := range dnsPageLanguages {
		if !have[lang] {
			t.Errorf("%s has no Do-Not-Sell texts of its own", lang)
			continue
		}
		if got := TextLang(KindDNS, lang); got != lang {
			t.Errorf("TextLang(dns, %q) = %q, want the language itself", lang, got)
		}
		text := ComposeDNS(Request{Lang: lang, Set: allSet(), Features: allFeatures()})
		first, _, _ := strings.Cut(text, "\n")
		if !strings.HasPrefix(first, "## ") {
			t.Errorf("%s opens with %q, want a ## heading", lang, first)
		}
		if other, dup := seen[first]; dup {
			t.Errorf("%s and %s open with the same heading %q", other, lang, first)
		}
		seen[first] = lang
		if got := Title(KindDNS, lang); got != dnsStatutoryTitle {
			t.Errorf("Title(dns, %q) = %q, want %q", lang, got, dnsStatutoryTitle)
		}
	}
}

// The Romansh text was written by a machine and says so once, near the top,
// in every composition (D-07).
func TestTheRomanshDNSPageIsMarkedAsMachineWritten(t *testing.T) {
	machine := langFile(t, "rm")["machine_notice"]
	if machine == "" {
		t.Fatal("rm/lang.json has no machine notice")
	}
	for _, f := range combos() {
		text := ComposeDNS(Request{Lang: "rm", Set: allSet(), Features: f})
		if n := strings.Count(text, machine); n != 1 {
			t.Errorf("rm %s: the machine notice occurs %d times, want 1", featureString(f), n)
		}
	}
	for _, lang := range []string{"de", "en", "fr", "it"} {
		if strings.Contains(ComposeDNS(Request{Lang: lang, Set: allSet()}), machine) {
			t.Errorf("%s carries the Romansh machine notice", lang)
		}
	}
}
