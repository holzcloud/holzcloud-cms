package privacy

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The tests of Phase 30, moved from text_test.go when the German left the Go
// source. Each one keeps its intent; where an assertion changed, the comment
// at the test says what and why.

// deText is the German privacy page with every framework chosen, which is the
// longest text and the one in which an assertion about the core is hardest to
// satisfy by accident.
func deText(f Features) string {
	return ComposePrivacy(Request{Lang: "de", Set: allSet(), Features: f})
}

// heading is the first line of a block, which every block opens with.
func heading(t *testing.T, lang, block string) string {
	t.Helper()
	return firstLine(t, lang, KindPrivacy, block)
}

// tableCells splits a Markdown table row into its trimmed cells.
func tableCells(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	row = strings.TrimSuffix(row, "|")
	var out []string
	for _, c := range strings.Split(row, "|") {
		out = append(out, strings.TrimSpace(c))
	}
	return out
}

// The fourth column used to name the legal basis. It now says, in words that
// hold under every framework, why no consent is needed (meta why_no_consent),
// because a legal basis belongs to one legal order and the bases now stand in each
// framework's own section (D-06). What the column has to say about every
// cookie, and the rest of this test, is unchanged.
func TestCookieRowsMatchTheRegister(t *testing.T) {
	known := []string{"hc_warenkorb", "hc_preise", "hc_frei_N"}
	for _, lang := range languagesOf(t, KindPrivacy) {
		header := meta(KindPrivacy, lang, "cookie_header")
		noCookies := meta(KindPrivacy, lang, "no_cookies")
		why := meta(KindPrivacy, lang, "why_no_consent")
		for _, set := range []legal.Set{{}, allSet()} {
			for _, f := range combos() {
				text := ComposePrivacy(Request{Lang: lang, Set: set, Features: f})
				reg := Cookies(f)
				listed := map[string]bool{}
				for _, c := range reg {
					listed[c.Name] = true
					row := ""
					for _, l := range strings.Split(text, "\n") {
						if strings.HasPrefix(l, "| `"+c.Name+"` |") {
							row = l
						}
					}
					if row == "" {
						t.Errorf("%s %+v: cookie %s has no row with its name in a code span", lang, f, c.Name)
						continue
					}
					if !strings.Contains(row, "| "+lifetimeText(lang, c.Lifetime)+" |") {
						t.Errorf("%s %+v: lifetime %q of %s not in its row", lang, f, lifetimeText(lang, c.Lifetime), c.Name)
					}
					if !strings.HasSuffix(row, "| "+why+" |") {
						t.Errorf("%s %+v: the row of %s does not end with why no consent is needed: %s", lang, f, c.Name, row)
					}
				}
				for _, k := range known {
					if !listed[k] && strings.Contains(text, k) {
						t.Errorf("%s %+v: %s occurs in the text but is not in the register for these features", lang, f, k)
					}
				}
				if len(reg) == 0 {
					if !strings.Contains(text, noCookies) {
						t.Errorf("%s %+v: no cookies, but the no-cookie sentence is missing", lang, f)
					}
					if strings.Contains(text, header) {
						t.Errorf("%s %+v: no cookies, but a cookie table is printed", lang, f)
					}
				} else {
					if strings.Contains(text, noCookies) {
						t.Errorf("%s %+v: cookies listed, but the text also says there are none", lang, f)
					}
					if !strings.Contains(text, header) {
						t.Errorf("%s %+v: cookies listed without the table header", lang, f)
					}
				}
			}
		}
	}
	// The wording of each trigger, as the visitor reads it.
	all := deText(allFeatures())
	for _, want := range []string{
		"Wird gesetzt, wenn du etwas in den Warenkorb legst.",
		"Wird gesetzt, wenn du die Preisansicht wechselst.",
		"Wird gesetzt, wenn du das Passwort einer geschützten Seite eingibst.",
		"| 30 Tage |", "| 12 Stunden |",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("full text lacks %q", want)
		}
	}
	if strings.Contains(all, "| Rechtsgrundlage |") {
		t.Error("the cookie table still has a legal-basis column; the bases belong to the framework sections")
	}
}

func TestComposeRendersTwoTables(t *testing.T) {
	for _, lang := range languagesOf(t, KindPrivacy) {
		html, err := page.RenderMarkdown(ComposePrivacy(Request{Lang: lang, Set: allSet(), Features: allFeatures()}))
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(html, "<table"); n != 2 {
			t.Fatalf("%s: rendered text has %d tables, want 2", lang, n)
		}
		cells := append(tableCells(meta(KindPrivacy, lang, "cookie_header")), tableCells(meta(KindPrivacy, lang, "processor_header"))...)
		if len(cells) != 8 {
			t.Fatalf("%s: the two header rows have %d cells, want 8", lang, len(cells))
		}
		for _, h := range cells {
			if !strings.Contains(html, "<th>"+h+"</th>") {
				t.Errorf("%s: header cell %q missing", lang, h)
			}
		}
		// Every body row of both tables has four cells.
		tables := strings.Split(html, "<table")[1:]
		for i, tb := range tables {
			tb = tb[:strings.Index(tb, "</table>")]
			body := tb[strings.Index(tb, "<tbody>"):]
			rows := strings.Split(body, "<tr>")[1:]
			if len(rows) == 0 {
				t.Errorf("%s: table %d has no body row", lang, i)
			}
			for _, r := range rows {
				if n := strings.Count(r, "<td"); n != 4 {
					t.Errorf("%s: table %d: body row has %d cells, want 4: %s", lang, i, n, r)
				}
			}
		}
	}
}

// The German cases are the seven of Phase 30, unchanged. Every other language
// says the same durations with its own words from its meta.json.
func TestLifetimeTextPerLanguage(t *testing.T) {
	cases := map[time.Duration]string{
		30 * 24 * time.Hour: "30 Tage",
		24 * time.Hour:      "1 Tag",
		12 * time.Hour:      "12 Stunden",
		time.Hour:           "1 Stunde",
		90 * time.Minute:    "90 Minuten",
		time.Minute:         "1 Minute",
		36 * time.Hour:      "36 Stunden",
	}
	for d, want := range cases {
		if got := lifetimeText("de", d); got != want {
			t.Errorf("lifetimeText(de, %v) = %q, want %q", d, got, want)
		}
	}
	key := map[time.Duration]struct {
		k string
		n int
	}{
		30 * 24 * time.Hour: {"lifetime_day_many", 30},
		24 * time.Hour:      {"lifetime_day_one", 1},
		12 * time.Hour:      {"lifetime_hour_many", 12},
		time.Hour:           {"lifetime_hour_one", 1},
		90 * time.Minute:    {"lifetime_minute_many", 90},
		time.Minute:         {"lifetime_minute_one", 1},
		36 * time.Hour:      {"lifetime_hour_many", 36},
	}
	for _, lang := range languagesOf(t, KindPrivacy) {
		for d, c := range key {
			want := strings.ReplaceAll(meta(KindPrivacy, lang, c.k), "{n}", strconv.Itoa(c.n))
			if got := lifetimeText(lang, d); got != want {
				t.Errorf("lifetimeText(%s, %v) = %q, want %q", lang, d, got, want)
			}
		}
	}
}

func TestLogSentencesComeFromTheConstants(t *testing.T) {
	text := ComposePrivacy(Request{Lang: "de"})
	for _, want := range []string{
		"/" + strconv.Itoa(web.LogIPv4Prefix),
		"/" + strconv.Itoa(web.LogIPv6Prefix),
		strconv.Itoa(web.LogRetentionDays) + " Tage",
		"/24", "/48",
		"Meldet sich jemand im Verwaltungsbereich unter /admin falsch an",
		"vollständige IP-Adresse in das System-Protokoll",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("log section lacks %q", want)
		}
	}
	if !strings.Contains(text, "tageweise") {
		t.Error("the text does not say that deletion goes by day")
	}
}

// TestRetentionComesFromTheDropIn ties the sentence of the text to the file the
// host enforces it with (D-03).
func TestRetentionComesFromTheDropIn(t *testing.T) {
	raw, err := os.ReadFile("../../deploy/journald-holzcloud.conf")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^MaxRetentionSec=(\d+)d\s*$`).FindStringSubmatch(string(raw))
	if m == nil {
		t.Fatal("deploy/journald-holzcloud.conf has no MaxRetentionSec=<N>d line")
	}
	days, _ := strconv.Atoi(m[1])
	if days != web.LogRetentionDays {
		t.Errorf("the drop-in keeps %d days but web.LogRetentionDays says %d: one of the two moved, and the text follows the constant", days, web.LogRetentionDays)
	}
	if !strings.Contains(ComposePrivacy(Request{Lang: "de"}), m[1]+" Tage") {
		t.Errorf("the drop-in keeps %s days but the composed text does not say %s Tage: the text moved away from the host", m[1], m[1])
	}
}

// The blocks of the functions sit between the cookies section and the block
// about the accounts of the people who work on the website, which took the
// place of the old rights section when the rights moved into the framework
// sections.
func TestBlocksAppearWithTheirFeature(t *testing.T) {
	blocks := []struct {
		name string
		on   func(Features) bool
	}{
		{"orders", func(f Features) bool { return f.Shop }},
		{"payment", func(f Features) bool { return f.Shop && f.Payment }},
		{"protected", func(f Features) bool { return f.ProtectedPages }},
		{"forms", func(f Features) bool { return f.Forms }},
	}
	for _, lang := range languagesOf(t, KindPrivacy) {
		cookies := "\n" + heading(t, lang, "cookies") + "\n"
		staff := "\n" + heading(t, lang, "staff") + "\n"
		for _, f := range combos() {
			text := ComposePrivacy(Request{Lang: lang, Set: allSet(), Features: f})
			last := -1
			for _, b := range blocks {
				line := "\n" + heading(t, lang, b.name) + "\n"
				n := strings.Count(text, line)
				want := 0
				if b.on(f) {
					want = 1
				}
				if n != want {
					t.Errorf("%s %+v: heading of %s occurs %d times, want %d", lang, f, b.name, n, want)
					continue
				}
				if n == 1 {
					i := strings.Index(text, line)
					if i < last {
						t.Errorf("%s %+v: block %s is out of order", lang, f, b.name)
					}
					last = i
					if i < strings.Index(text, cookies) || i > strings.Index(text, staff) {
						t.Errorf("%s %+v: block %s is outside the space between cookies and staff", lang, f, b.name)
					}
				}
			}
		}
	}
}

// "Art. 28" used to be asserted in the provider row. The core no longer names
// an article of any one law (D-06), so the processing contract is now asserted
// in the EU section, where Art. 28 GDPR belongs; the row keeps its fill-in.
func TestPaymentBlockNamesWhatIsSent(t *testing.T) {
	on := deText(Features{Shop: true, Payment: true})
	for _, want := range []string{"Payrexx", "Name, E-Mail-Adresse, Straße, Postleitzahl, Ort und Land",
		"Betrag", "Währung", "Bestellnummer", "Verwendungszweck", "Zahlungsseite"} {
		if !strings.Contains(on, want) {
			t.Errorf("payment text lacks %q", want)
		}
	}
	fill := Fillin("de")
	payment := heading(t, "de", "payment")
	block := on[strings.Index(on, payment):]
	block = block[:strings.Index(block, heading(t, "de", "staff"))]
	if !strings.Contains(block, "Datum des Vertrags "+fill) {
		t.Error("the payment block has no fill-in marker for the contract date")
	}
	var row string
	for _, l := range strings.Split(on, "\n") {
		if strings.HasPrefix(l, "| Zahlungsanbieter Payrexx") {
			row = l
		}
	}
	if row == "" {
		t.Fatal("no provider row in the processor table")
	}
	if !strings.Contains(row, fill) {
		t.Errorf("provider row lacks the fill-in marker: %s", row)
	}
	eu := ComposePrivacy(Request{Lang: "de", Set: legal.ParseSet("eu"), Features: Features{Shop: true, Payment: true}})
	eu = eu[strings.Index(eu, heading(t, "de", "eu")):]
	if !strings.Contains(eu, "Art. 28") {
		t.Error("the EU section does not name the processing contract of Art. 28")
	}
	// Without payment no framework section and no table names the provider.
	off := deText(Features{Shop: true})
	for _, no := range []string{"Payrexx", "Zahlungsanbieter", "Zahlungsseite", payment} {
		if strings.Contains(off, no) {
			t.Errorf("without payment the text still contains %q", no)
		}
	}
}

func TestInconsistentFeaturesAreNormalised(t *testing.T) {
	for _, set := range sets() {
		r := func(f Features) Request { return Request{Lang: "de", Set: set, Features: f} }
		zero := ComposePrivacy(r(Features{}))
		for _, f := range []Features{{Payment: true}, {PriceChoice: true}, {Payment: true, PriceChoice: true}} {
			if ComposePrivacy(r(f)) != zero {
				t.Errorf("%s %+v without a shop does not give the text of the zero features", set, f)
			}
		}
		if ComposePrivacy(r(Features{Forms: true, Payment: true})) != ComposePrivacy(r(Features{Forms: true})) {
			t.Errorf("%s: payment without a shop changes the text of a website with forms", set)
		}
	}
}

// The core says what the website does; which law makes it lawful is the
// business of each framework's section (D-06).
func TestTheCoreNamesNoLegalBasis(t *testing.T) {
	core := ComposePrivacy(Request{Lang: "de", Features: allFeatures()})
	for _, no := range []string{"Rechtsgrundlage", "DSGVO", "TDDDG", "Art. 6"} {
		if strings.Contains(core, no) {
			t.Errorf("with no framework chosen the text still says %q", no)
		}
	}
	for _, block := range []string{"staff", "media"} {
		if !strings.Contains(core, "\n"+heading(t, "de", block)+"\n") {
			t.Errorf("the core lacks the %s block", block)
		}
	}
}
