package privacy

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// combos returns all 32 combinations of the five booleans, including the
// inconsistent ones (Payment or PriceChoice without Shop).
func combos() []Features {
	var out []Features
	for i := 0; i < 32; i++ {
		out = append(out, Features{
			Shop:           i&1 != 0,
			PriceChoice:    i&2 != 0,
			Payment:        i&4 != 0,
			ProtectedPages: i&8 != 0,
			Forms:          i&16 != 0,
		})
	}
	return out
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n "), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func TestComposeIsDeterministicAndMarkedAsTemplate(t *testing.T) {
	for _, f := range combos() {
		a, b := Compose(f), Compose(f)
		if a != b {
			t.Fatalf("%+v: two calls gave different text", f)
		}
		if n := strings.Count(a, TemplateNotice); n != 1 {
			t.Errorf("%+v: template notice occurs %d times, want 1", f, n)
		}
		if i := strings.Index(a, TemplateNotice); i < 0 || i > 400 {
			t.Errorf("%+v: template notice at byte %d, want within the first 400", f, i)
		}
		if got := lastNonEmptyLine(a); got != NoLegalAdvice {
			t.Errorf("%+v: last line is %q, want the no-legal-advice sentence", f, got)
		}
		if n := strings.Count(a, Fillin); n < 2 {
			t.Errorf("%+v: only %d fill-in markers, want at least 2", f, n)
		}
		if n := strings.Count(a, "\n## "+HeadingCookies+"\n"); n != 1 {
			t.Errorf("%+v: cookies heading line occurs %d times, want 1", f, n)
		}
		for _, bad := range []string{"http://", "https://", "<"} {
			if strings.Contains(a, bad) {
				t.Errorf("%+v: text contains %q: no link and no raw HTML belong in it", f, bad)
			}
		}
	}
	z := Compose(Features{})
	for _, h := range []string{"## " + headingResponsi, "## " + headingLog, "## " + HeadingCookies, "## " + headingRights, "\n---\n"} {
		if !strings.Contains(z, h) {
			t.Errorf("zero features: %q missing", h)
		}
	}
	if !HasTemplateNotice(z) || HasTemplateNotice("Datenschutz") {
		t.Error("HasTemplateNotice does not tell a template from other text")
	}
}

func TestCookieRowsMatchTheRegister(t *testing.T) {
	known := []string{"hc_warenkorb", "hc_preise", "hc_frei_N"}
	for _, f := range combos() {
		text := Compose(f)
		reg := Cookies(f)
		listed := map[string]bool{}
		for _, c := range reg {
			listed[c.Name] = true
			if !strings.Contains(text, "`"+c.Name+"`") {
				t.Errorf("%+v: cookie %s not in a code span", f, c.Name)
			}
			if !strings.Contains(text, "| "+LifetimeText(c.Lifetime)+" |") {
				t.Errorf("%+v: lifetime %q of %s not in its row", f, LifetimeText(c.Lifetime), c.Name)
			}
		}
		for _, k := range known {
			if !listed[k] && strings.Contains(text, k) {
				t.Errorf("%+v: %s occurs in the text but is not in the register for these features", f, k)
			}
		}
		if len(reg) == 0 {
			if !strings.Contains(text, noCookies) {
				t.Errorf("%+v: no cookies, but the no-cookie sentence is missing", f)
			}
			if strings.Contains(text, cookieHeaderRow) {
				t.Errorf("%+v: no cookies, but a cookie table is printed", f)
			}
		} else {
			if strings.Contains(text, noCookies) {
				t.Errorf("%+v: cookies listed, but the text also says there are none", f)
			}
			if !strings.Contains(text, cookieHeaderRow) {
				t.Errorf("%+v: cookies listed without the table header", f)
			}
		}
	}
	// The wording of each trigger, as the visitor reads it.
	all := Compose(allFeatures())
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
}

func TestComposeRendersTwoTables(t *testing.T) {
	html, err := page.RenderMarkdown(Compose(allFeatures()))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(html, "<table"); n != 2 {
		t.Fatalf("rendered text has %d tables, want 2", n)
	}
	for _, h := range []string{"Name", "Zweck", "Laufzeit", "Rechtsgrundlage", "Empfänger", "Aufgabe", "Übermittelte Daten", "Vertrag"} {
		if !strings.Contains(html, "<th>"+h+"</th>") {
			t.Errorf("header cell %q missing", h)
		}
	}
	// Every body row of both tables has four cells.
	tables := strings.Split(html, "<table")[1:]
	for i, tb := range tables {
		tb = tb[:strings.Index(tb, "</table>")]
		body := tb[strings.Index(tb, "<tbody>"):]
		rows := strings.Split(body, "<tr>")[1:]
		if len(rows) == 0 {
			t.Errorf("table %d has no body row", i)
		}
		for _, r := range rows {
			if n := strings.Count(r, "<td"); n != 4 {
				t.Errorf("table %d: body row has %d cells, want 4: %s", i, n, r)
			}
		}
	}
}

func TestLifetimeText(t *testing.T) {
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
		if got := LifetimeText(d); got != want {
			t.Errorf("LifetimeText(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestLogSentencesComeFromTheConstants(t *testing.T) {
	text := Compose(Features{})
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
	if !strings.Contains(Compose(Features{}), m[1]+" Tage") {
		t.Errorf("the drop-in keeps %s days but the composed text does not say %s Tage: the text moved away from the host", m[1], m[1])
	}
}

func TestBlocksAppearWithTheirFeature(t *testing.T) {
	headings := []struct {
		h  string
		on func(Features) bool
	}{
		{headingOrders, func(f Features) bool { return f.Shop }},
		{headingPayment, func(f Features) bool { return f.Shop && f.Payment }},
		{headingProtect, func(f Features) bool { return f.ProtectedPages }},
		{headingForms, func(f Features) bool { return f.Forms }},
	}
	for _, f := range combos() {
		text := Compose(f)
		last := -1
		for _, h := range headings {
			line := "\n## " + h.h + "\n"
			n := strings.Count(text, line)
			want := 0
			if h.on(f) {
				want = 1
			}
			if n != want {
				t.Errorf("%+v: heading %q occurs %d times, want %d", f, h.h, n, want)
				continue
			}
			if n == 1 {
				i := strings.Index(text, line)
				if i < last {
					t.Errorf("%+v: heading %q is out of order", f, h.h)
				}
				last = i
				// Blocks sit between the cookies section and the rights.
				if i < strings.Index(text, "\n## "+HeadingCookies+"\n") || i > strings.Index(text, "\n## "+headingRights+"\n") {
					t.Errorf("%+v: heading %q is outside the space between cookies and rights", f, h.h)
				}
			}
		}
	}
}

func TestPaymentBlockNamesWhatIsSent(t *testing.T) {
	on := Compose(Features{Shop: true, Payment: true})
	for _, want := range []string{"Payrexx", "Art. 28", "Name, E-Mail-Adresse, Straße, Postleitzahl, Ort und Land",
		"Betrag", "Währung", "Bestellnummer", "Verwendungszweck", "Zahlungsseite"} {
		if !strings.Contains(on, want) {
			t.Errorf("payment text lacks %q", want)
		}
	}
	// The fill-in marker stands in the block and in the provider row.
	block := on[strings.Index(on, "## "+headingPayment):]
	block = block[:strings.Index(block, "## "+headingRights)]
	if !strings.Contains(block, "Datum des Vertrags "+Fillin) {
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
	if !strings.Contains(row, Fillin) || !strings.Contains(row, "Art. 28") {
		t.Errorf("provider row lacks the fill-in marker or the contract hint: %s", row)
	}
	// The hosting row names Art. 28 as well, so that article is not a mark of
	// the provider; the provider's name and its page are.
	off := Compose(Features{Shop: true})
	for _, no := range []string{"Payrexx", "Zahlungsanbieter", "Zahlungsseite", headingPayment} {
		if strings.Contains(off, no) {
			t.Errorf("without payment the text still contains %q", no)
		}
	}
}

func TestInconsistentFeaturesAreNormalised(t *testing.T) {
	zero := Compose(Features{})
	for _, f := range []Features{{Payment: true}, {PriceChoice: true}, {Payment: true, PriceChoice: true}} {
		if Compose(f) != zero {
			t.Errorf("%+v without a shop does not give the text of the zero features", f)
		}
	}
	if Compose(Features{Forms: true, Payment: true}) != Compose(Features{Forms: true}) {
		t.Error("payment without a shop changes the text of a website with forms")
	}
}
