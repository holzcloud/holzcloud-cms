package privacy

import (
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
)

// The "request about my data" page (Phase 33.2, D-01). The general rules
// every composition keeps — notice first, no legal advice last, fill-ins, no
// link and no HTML, Swiss spelling, the fallback of the text language — are
// compose_test.go's and hold for this kind because it registers itself. What
// is checked here is what only this page has to say.

// requestMarker is the form marker of plan 03, written out here and not taken
// from internal/public, which this package does not import.
const requestMarker = "[[datenanfrage]]"

// requestLanguages lists the languages with texts of this kind and fails when
// German is not among them, so that no test here passes over an empty list.
func requestLanguages(t *testing.T) []string {
	t.Helper()
	langs := languagesOf(t, KindRequest)
	found := false
	for _, l := range langs {
		if l == "de" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the request page has no German texts (languages: %v)", langs)
	}
	return langs
}

// requestBlockHead is the first line of a block of the request page in a
// language: the heading or opening words that tell the block apart.
func requestBlockHead(t *testing.T, lang, name string) string {
	t.Helper()
	h := firstLine(t, lang, KindRequest, name)
	if strings.TrimSpace(h) == "" {
		t.Fatalf("%s/request/%s.md opens with an empty line", lang, name)
	}
	return h
}

// requestHas reports whether the composed text carries the block, found by
// its first line standing alone on a line.
func requestHas(t *testing.T, text, lang, name string) bool {
	t.Helper()
	return strings.Contains("\n"+text, "\n"+requestBlockHead(t, lang, name)+"\n")
}

// The marker is where plan 03's form goes. It must be there exactly once,
// because the form is drawn once per page, and it must stand alone between
// blank lines: only then does Markdown make it a paragraph of its own, which
// the form replaces whole. The template notice comes first and the
// no-legal-advice sentence last, whatever the frameworks.
func TestTheRequestPageCarriesTheFormOnce(t *testing.T) {
	checked := 0
	for _, lang := range requestLanguages(t) {
		notice, closing := TemplateNotice(lang), NoLegalAdvice(lang)
		for _, set := range sets() {
			for _, f := range []Features{{}, {Shop: true}, allFeatures()} {
				text := ComposeRequest(Request{Lang: lang, Set: set, Features: f})
				checked++
				where := lang + " " + set.String() + " " + featureString(f)
				if n := strings.Count(text, requestMarker); n != 1 {
					t.Errorf("%s: the marker occurs %d times, want 1", where, n)
					continue
				}
				lines := strings.Split(text, "\n")
				for i, line := range lines {
					if !strings.Contains(line, requestMarker) {
						continue
					}
					if line != requestMarker {
						t.Errorf("%s: the marker shares its line: %q", where, line)
					}
					if i == 0 || lines[i-1] != "" || i+1 >= len(lines) || lines[i+1] != "" {
						t.Errorf("%s: the marker is not a paragraph of its own", where)
					}
				}
				first := strings.Index(text, notice)
				if first < 0 || first > strings.Index(text, requestMarker) {
					t.Errorf("%s: the template notice does not come before the form", where)
				}
				if got := lastNonEmptyLine(text); got != closing {
					t.Errorf("%s: last line %q, want the no-legal-advice sentence", where, got)
				}
			}
		}
	}
	t.Logf("%d compositions checked", checked)
}

// requestDeadline is one framework's deadline block and the citations a
// reader checks it by: the article and the number of days or the month.
var requestDeadline = []struct {
	fw      legal.Framework
	block   string
	anchors map[string][]string
}{
	{legal.EU, "deadline-eu", map[string][]string{"de": {"Art. 12 Abs. 3 DSGVO", "eines Monats", "zwei weitere Monate"}, "en": {"Art. 12(3) GDPR", "one month", "two further months"}}},
	{legal.CH, "deadline-ch", map[string][]string{"de": {"Art. 25 Abs. 7 DSG", "Art. 18 DSV", "30 Tagen", "kostenlos"}, "en": {"Art. 25(7) FADP", "Art. 18 DPO", "30 days", "free of charge"}}},
	{legal.UK, "deadline-uk", map[string][]string{"de": {"Art. 12 Abs. 3", "UK GDPR", "eines Monats"}, "en": {"Art. 12(3)", "UK GDPR", "one month"}}},
	{legal.US, "deadline-us", map[string][]string{"de": {"§ 7021", "10 Geschäftstagen", "45 Kalendertagen"}, "en": {"§ 7021", "10 business days", "45 calendar days"}}},
}

// The page names the deadline of each chosen framework and of no other: a
// deadline the website does not owe is a promise the operator cannot keep,
// and one it owes and does not name is a right withheld (T-33.2-54).
func TestTheRequestPageNamesTheDeadlinesOfItsFrameworks(t *testing.T) {
	for _, lang := range requestLanguages(t) {
		for _, set := range sets() {
			text := ComposeRequest(Request{Lang: lang, Set: set, Features: Features{}})
			for _, d := range requestDeadline {
				has := requestHas(t, text, lang, d.block)
				if has != set.Has(d.fw) {
					t.Errorf("%s %q: the %s deadline present = %v, want %v", lang, set, d.fw, has, set.Has(d.fw))
				}
			}
			for _, name := range []string{"glance", "rights", "form", "identity"} {
				if !requestHas(t, text, lang, name) {
					t.Errorf("%s %q: the %s block is missing", lang, set, name)
				}
			}
		}
		for _, d := range requestDeadline {
			text := ComposeRequest(Request{Lang: lang, Set: legal.ParseSet(string(d.fw)), Features: Features{}})
			anchors, ok := d.anchors[lang]
			if !ok {
				continue
			}
			for _, a := range anchors {
				if !strings.Contains(text, a) {
					t.Errorf("%s with %s chosen: the deadline does not say %q", lang, d.fw, a)
				}
			}
		}
	}

	// The cases the plan names, spelled out.
	eu := ComposeRequest(Request{Lang: "de", Set: legal.ParseSet("eu")})
	if !strings.Contains(eu, "Art. 12 Abs. 3 DSGVO") || strings.Contains(eu, "§ 7021") {
		t.Errorf("set eu: want the EU deadline and not the US one:\n%s", eu)
	}
	chus := ComposeRequest(Request{Lang: "de", Set: legal.ParseSet("ch,us")})
	for _, w := range []string{"30 Tagen", "Art. 25 Abs. 7 DSG", "45 Kalendertagen", "§ 7021"} {
		if !strings.Contains(chus, w) {
			t.Errorf("set ch,us: the text lacks %q", w)
		}
	}
	if strings.Contains(chus, "DSGVO") {
		t.Errorf("set ch,us: the text names the GDPR")
	}
	none := ComposeRequest(Request{Lang: "de", Set: legal.Set{}})
	for _, w := range []string{"DSGVO", "§ 7021", "Art. 25 Abs. 7", "UK GDPR"} {
		if strings.Contains(none, w) {
			t.Errorf("no framework: the text still names %q", w)
		}
	}
	if !strings.Contains(none, requestMarker) {
		t.Errorf("no framework: the form is gone")
	}
}

// With a shop, an erased order keeps its invoice record until the seller's
// retention period ends (D-06, D-07), and the person asking for erasure is
// told so before asking. Without a shop the page says nothing about orders.
func TestTheRequestPageSaysWhatIsKeptOnlyWithAShop(t *testing.T) {
	for _, lang := range requestLanguages(t) {
		for _, set := range sets() {
			without := ComposeRequest(Request{Lang: lang, Set: set, Features: Features{Forms: true, ProtectedPages: true}})
			if requestHas(t, without, lang, "kept") {
				t.Errorf("%s %q: the page speaks of orders on a website without a shop", lang, set)
			}
			with := ComposeRequest(Request{Lang: lang, Set: set, Features: Features{Shop: true}})
			if !requestHas(t, with, lang, "kept") {
				t.Errorf("%s %q: the page of a shop does not say what an order keeps", lang, set)
			}
		}
	}
	kept := ComposeRequest(Request{Lang: "de", Set: legal.ParseSet("eu,ch"), Features: Features{Shop: true}})
	for _, w := range []string{"Rechnung", "Aufbewahrungsfrist", "E-Mail-Adresse und Telefonnummer"} {
		if !strings.Contains(kept, w) {
			t.Errorf("the German kept block lacks %q", w)
		}
	}
}
