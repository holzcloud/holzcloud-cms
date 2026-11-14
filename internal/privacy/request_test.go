package privacy

import (
	"slices"
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
	{legal.EU, "deadline-eu", map[string][]string{
		"de": {"Art. 12 Abs. 3 DSGVO", "eines Monats", "zwei weitere Monate"},
		"en": {"Art. 12(3) GDPR", "one month", "two further months"},
		"fr": {"art. 12, par. 3, RGPD", "d’un mois", "deux mois supplémentaires"},
		"it": {"art. 12 par. 3 RGPD", "entro un mese", "altri due mesi"},
	}},
	{legal.CH, "deadline-ch", map[string][]string{
		"de": {"Art. 25 Abs. 7 DSG", "Art. 18 DSV", "30 Tagen", "kostenlos"},
		"en": {"Art. 25(7) FADP", "Art. 18 of the Data Protection Ordinance", "Art. 16(5) DPO", "30 days", "free of charge"},
		"fr": {"art. 25, al. 7, LPD", "art. 18 de l’ordonnance sur la protection des données, OPDo", "30 jours", "gratuitement"},
		"it": {"art. 25 cpv. 7 LPD", "art. 18 dell’ordinanza sulla protezione dei dati, OPDa", "30 giorni", "gratuita"},
	}},
	{legal.UK, "deadline-uk", map[string][]string{
		"de": {"Art. 12 Abs. 3", "UK GDPR", "eines Monats"},
		"en": {"Art. 12(3)", "UK GDPR", "one month"},
		"fr": {"art. 12, par. 3", "UK GDPR", "d’un mois"},
		"it": {"art. 12 par. 3", "UK GDPR", "entro un mese"},
	}},
	{legal.US, "deadline-us", map[string][]string{
		"de": {"§ 7021", "10 Geschäftstagen", "45 Kalendertagen"},
		"en": {"§ 7021", "10 business days", "45 calendar days"},
		"fr": {"§ 7021", "10 jours ouvrables", "45 jours civils"},
		"it": {"§ 7021", "10 giorni lavorativi", "45 giorni di calendario"},
	}},
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

// The English page is the German page in English: the same blocks under the
// same conditions, the same marker, and the same numbers and citations, so a
// website in two languages promises one thing (T-33.2-54).
func TestTheEnglishRequestPageMatchesTheGerman(t *testing.T) {
	if got := TextLang(KindRequest, "en"); got != "en" {
		t.Fatalf("the request page has no English texts: en reads %q", got)
	}
	de := ComposeRequest(Request{Lang: "de", Set: allSet(), Features: Features{Shop: true}})
	en := ComposeRequest(Request{Lang: "en", Set: allSet(), Features: Features{Shop: true}})

	for _, name := range []string{"glance", "rights", "deadline-eu", "deadline-ch", "deadline-uk", "deadline-us", "form", "identity", "kept", "closing"} {
		if !requestHas(t, en, "en", name) {
			t.Errorf("the English page lacks the %s block", name)
		}
	}
	if n := strings.Count(en, "\n"+requestMarker+"\n"); n != 1 {
		t.Errorf("the English page carries the marker on a line of its own %d times, want 1", n)
	}
	if i := strings.Index(en, "**"+TemplateNotice("en")+"**"); i < 0 || i > 200 {
		t.Errorf("the English page does not open with the English template notice (at %d)", i)
	}
	if strings.Contains(en, TemplateNotice("de")) {
		t.Errorf("the English page carries the German notice")
	}

	// The deadline numbers the plan names, in both languages.
	for _, pair := range [][2]string{
		{"Art. 12 Abs. 3", "Art. 12(3)"},
		{"30 Tagen", "30 days"},
		{"Art. 25 Abs. 7", "Art. 25(7)"},
		{"45 Kalendertagen", "45 calendar days"},
		{"§ 7021", "§ 7021"},
	} {
		if !strings.Contains(de, pair[0]) || !strings.Contains(en, pair[1]) {
			t.Errorf("German %q present %v, English %q present %v; want both",
				pair[0], strings.Contains(de, pair[0]), pair[1], strings.Contains(en, pair[1]))
		}
	}
	// And the same numbers block by block, in the same order: a number one
	// language names and the other does not is a promise only one makes.
	for _, set := range sets() {
		for _, f := range []Features{{}, {Shop: true}} {
			d := requestDigits(ComposeRequest(Request{Lang: "de", Set: set, Features: f}))
			e := requestDigits(ComposeRequest(Request{Lang: "en", Set: set, Features: f}))
			if d != e {
				t.Errorf("%s %s: the numbers differ:\n de %s\n en %s", set, featureString(f), d, e)
			}
		}
	}
	for _, w := range enfrGerman {
		if strings.Contains(en, w) {
			t.Errorf("the English page contains the German %q", w)
		}
	}
}

// requestDigits is the runs of digits of a text, in order: the numbers and
// citations it carries, without the words around them.
func requestDigits(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
			in = true
			continue
		}
		if in {
			b.WriteByte(' ')
			in = false
		}
	}
	return strings.TrimSpace(b.String())
}

// requestBlocks are the blocks of the request page, in their order, all of
// them shown when every framework is chosen and the website has a shop.
var requestBlocks = []string{"glance", "rights", "deadline-eu", "deadline-ch", "deadline-uk", "deadline-us", "form", "identity", "kept", "closing"}

// ownRequestPage holds a language's request page to what plan 06 promised
// it: its own text for the language and its Swiss form, its own title and
// address, every block, the marker once on a line of its own, and its own
// template notice first, not the English or the German one.
func ownRequestPage(t *testing.T, lang, slug, also, title, draftSlug string) string {
	t.Helper()
	for _, tag := range []string{lang, lang + "-CH"} {
		if got := TextLang(KindRequest, tag); got != lang {
			t.Errorf("TextLang(request, %q) = %q, want %s", tag, got, lang)
		}
	}
	if got := Slug(KindRequest, lang); got != slug {
		t.Errorf("Slug(request, %s) = %q, want %s", lang, got, slug)
	}
	if got := Title(KindRequest, lang+"-CH"); got != title {
		t.Errorf("Title(request, %s-CH) = %q, want %q", lang, got, title)
	}
	if got := DraftSlug(KindRequest, lang); got != draftSlug {
		t.Errorf("DraftSlug(request, %s) = %q, want %s", lang, got, draftSlug)
	}
	for _, s := range []string{slug, also} {
		if !IsRoleSlug(KindRequest, s) {
			t.Errorf("%q is not a role slug of the request page", s)
		}
	}
	if TextLang(KindRequest, lang) != lang {
		return ""
	}

	text := ComposeRequest(Request{Lang: lang, Set: allSet(), Features: Features{Shop: true}})
	for _, name := range requestBlocks {
		if !requestHas(t, text, lang, name) {
			t.Errorf("%s: the page lacks the %s block", lang, name)
		}
	}
	if n := strings.Count(text, "\n"+requestMarker+"\n"); n != 1 || strings.Count(text, requestMarker) != 1 {
		t.Errorf("%s: the marker stands on a line of its own %d times, want once and nowhere else", lang, n)
	}
	if i := strings.Index(text, "**"+TemplateNotice(lang)+"**"); i < 0 || i > 200 {
		t.Errorf("%s: the page does not open with its own template notice (at %d)", lang, i)
	}
	for _, other := range []string{"de", "en"} {
		if strings.Contains(text, TemplateNotice(other)) {
			t.Errorf("%s: the page carries the %s notice", lang, other)
		}
	}
	if got := lastNonEmptyLine(text); got != NoLegalAdvice(lang) {
		t.Errorf("%s: last line %q, want its own no-legal-advice sentence", lang, got)
	}
	// The Swiss switch changes German spelling and nothing else.
	if swiss := ComposeRequest(Request{Lang: lang + "-CH", Swiss: true, Set: allSet(), Features: Features{Shop: true}}); swiss != text {
		t.Errorf("%s: the Swiss text is not the plain one", lang)
	}
	for _, w := range enfrGerman {
		if strings.Contains(text, w) {
			t.Errorf("%s: the page contains the German %q", lang, w)
		}
	}
	return text
}

// French and Italian pages get the request page in their own language (plan
// 06), and in the terms the Swiss federal texts of that language use for the
// Swiss deadline. Spanish keeps reading English (33.1 D-16).
func TestFrenchAndItalianHaveTheirOwnRequestPage(t *testing.T) {
	fr := ownRequestPage(t, "fr", "mes-donnees", "demande-mes-donnees", "Demande concernant mes données", "mes-donnees-modele")
	it := ownRequestPage(t, "it", "i-miei-dati", "richiesta-miei-dati", "Richiesta sui miei dati", "i-miei-dati-modello")

	for _, w := range []string{"art. 12, par. 3, RGPD", "art. 25, al. 7, LPD", "art. 18 de l’ordonnance sur la protection des données, OPDo", "art. 16, al. 5, OPDo", "art. 12, par. 3, et art. 12A UK GDPR", "§ 7021 des règlements CCPA"} {
		if fr != "" && !strings.Contains(fr, w) {
			t.Errorf("the French page does not cite %q", w)
		}
	}
	for _, w := range []string{"art. 12 par. 3 RGPD", "art. 25 cpv. 7 LPD", "art. 18 dell’ordinanza sulla protezione dei dati, OPDa", "art. 16 cpv. 5 OPDa", "art. 12 par. 3 e art. 12A UK GDPR", "§ 7021 del regolamento CCPA"} {
		if it != "" && !strings.Contains(it, w) {
			t.Errorf("the Italian page does not cite %q", w)
		}
	}
	// Italian addresses the visitor with Lei, as its privacy page does.
	if it != "" && (!strings.Contains(it, " Sua ") || strings.Contains(it, " tuoi ") || strings.Contains(it, " tua ")) {
		t.Errorf("the Italian page does not address the visitor with Lei")
	}

	// French typography, as TestFrenchTypography holds the privacy page to.
	for _, bad := range []string{"'", " :", " ;", " ?", " !", " %", "&nbsp;", "&#", "« ", " »"} {
		if i := strings.Index(fr, bad); i >= 0 {
			t.Errorf("the French page contains %q near %q", bad, fr[max(0, i-30):min(len(fr), i+30)])
		}
	}

	for _, tag := range []string{"es", "es-MX"} {
		if got := TextLang(KindRequest, tag); got != "en" {
			t.Errorf("TextLang(request, %q) = %q, want en", tag, got)
		}
	}
	if got := Title(KindRequest, "es"); got != "Request about my data" {
		t.Errorf("Title(request, es) = %q, want Request about my data", got)
	}
}

// The Romansh page (plan 06) was written by a machine in Rumantsch Grischun
// and no Romansh speaker has checked it. Every composition says so in bold,
// right after the template notice and before the form (33.1 D-07,
// T-33.2-29), so publishing it unchecked is a choice the operator makes
// knowingly.
func TestRomanshRequestPageIsMarkedAsMachineWritten(t *testing.T) {
	text := ownRequestPage(t, "rm", "mias-datas", "dumonda-mias-datas", "Dumonda davart mias datas", "mias-datas-model")
	if text == "" {
		return
	}
	machine := langFile(t, "rm")["machine_notice"]
	if strings.TrimSpace(machine) == "" {
		t.Fatal("texts/rm/lang.json has no machine notice")
	}
	notice, bold := "**"+TemplateNotice("rm")+"**", "**"+machine+"**"
	for _, set := range sets() {
		for _, f := range []Features{{}, {Shop: true}} {
			c := ComposeRequest(Request{Lang: "rm", Set: set, Features: f})
			where := set.String() + " " + featureString(f)
			if n := strings.Count(c, machine); n != 1 {
				t.Errorf("%s: the machine notice occurs %d times, want 1", where, n)
				continue
			}
			if !strings.Contains(c, notice+" "+bold) {
				t.Errorf("%s: the machine notice does not follow the template notice in bold", where)
			}
			if strings.Index(c, bold) > strings.Index(c, requestMarker) {
				t.Errorf("%s: the machine notice comes after the form", where)
			}
		}
	}
	// Romansh addresses the visitor with Vus, as its privacy page does.
	if !strings.Contains(text, "Vossa dumonda") || strings.Contains(text, " tias ") || strings.Contains(text, " tia ") {
		t.Errorf("the Romansh page does not address the visitor with Vus")
	}
	for _, w := range []string{"RGPD", "LPD", "UK GDPR", "§ 7021"} {
		if !strings.Contains(text, w) {
			t.Errorf("the Romansh page does not cite %q", w)
		}
	}
}

// requestLanguagesRead are the languages TestEveryLanguageStatesTheSameDeadlines
// compares; a language with request texts that is not among them fails it, so
// that no translation escapes the comparison.
var requestLanguagesRead = []string{"de", "en", "fr", "it", "rm"}

// Every language states the deadlines the German and the English page state,
// with the same citations (T-33.2-28): the numbers of each framework's
// deadline are there, and every composition carries the same numbers in the
// same order as the English one, block by block. A digit is the same in every
// language, so this reads across translations without understanding them.
func TestEveryLanguageStatesTheSameDeadlines(t *testing.T) {
	for _, l := range languagesOf(t, KindRequest) {
		if !slices.Contains(requestLanguagesRead, l) {
			t.Errorf("%s has request texts this test does not compare", l)
		}
	}
	want := []struct {
		set   string
		words []string
	}{
		{"eu", []string{"12"}},
		{"ch", []string{"30", "25"}},
		{"uk", []string{"12A"}},
		{"us", []string{"45", "7021"}},
	}
	for _, lang := range requestLanguagesRead {
		if got := TextLang(KindRequest, lang); got != lang {
			t.Errorf("%s: the request page reads %q, not its own text", lang, got)
			continue
		}
		for _, w := range want {
			text := ComposeRequest(Request{Lang: lang, Set: legal.ParseSet(w.set)})
			for _, n := range w.words {
				if !strings.Contains(text, n) {
					t.Errorf("%s with %s chosen: the page does not say %q", lang, w.set, n)
				}
			}
		}
		for _, set := range sets() {
			for _, f := range []Features{{}, {Shop: true}} {
				e := requestDigits(ComposeRequest(Request{Lang: "en", Set: set, Features: f}))
				if got := requestDigits(ComposeRequest(Request{Lang: lang, Set: set, Features: f})); got != e {
					t.Errorf("%s %s %s: the numbers differ from the English page:\n %s %s\n en %s", lang, set, featureString(f), lang, got, e)
				}
			}
		}
	}
}
