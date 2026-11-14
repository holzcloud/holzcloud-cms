package public

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	xhtml "golang.org/x/net/html"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
)

// The opt-out widget, read the way a visitor reads it: a page served through
// the same middleware the server uses, and a form posted the way a browser
// posts it — with no cookie, because the public side has none to give.

// The source sentences the widget prints. They are repeated here and not
// imported, because the collector reads the call sites in optout.go and a
// test that read them from a variable would pass for a key nobody collects.
const (
	optoutHonoured = "Your browser sends the Global Privacy Control signal. Opt-Out Request Honored: " +
		"this website treats it as your request not to sell or share your personal information — " +
		"and it sells and shares none."
	optoutNotSent  = "Your browser does not send the Global Privacy Control signal. You can switch it on in your browser, or use this form."
	optoutRecorded = "Your request has been recorded."
	optoutSubmit   = "Send opt-out request"
)

// optoutSigner is the test's stand-in for main.go's "optout:" signer.
var optoutSigner = sharelink.New([]byte("optout"))

// optoutSite is a website with the given framework set and, when extras is not
// empty, further languages, served by a handler wired the way main.go wires
// it.
func optoutSite(t *testing.T, set, extras string) (*Handler, *db.DB, *domain.Website) {
	t.Helper()
	h, database := newTestHandler(t)
	h.SetRequestStore(dsr.NewStore(database))
	h.SetOptOutSigner(optoutSigner)
	ws := seedWebsite(t, database, "Hofladen")
	if _, err := database.Write.Exec(
		`UPDATE websites SET legal_frameworks = $1, extra_locales = $2 WHERE id = $3`,
		set, extras, ws.ID); err != nil {
		t.Fatalf("set frameworks: %v", err)
	}
	fresh, err := domain.NewStore(database).GetWebsite(context.Background(), ws.ID)
	if err != nil || fresh == nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	return h, database, fresh
}

// optoutPage stores a published page with the HTML exactly as given — the
// shape content_html has after the save path, without asking Markdown to
// produce it.
func optoutPage(t *testing.T, database *db.DB, websiteID int64, slug, html, loc string) *page.Page {
	t.Helper()
	store := page.NewStore(database)
	p, err := store.CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID, Title: "Do Not Sell", Slug: slug, HTML: html, Status: "published",
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if loc != "" {
		if err := store.SetTranslation(context.Background(), websiteID, p.ID, loc, 0); err != nil {
			t.Fatalf("SetTranslation: %v", err)
		}
	}
	return p
}

// optoutDo runs one request through LocaleMiddleware into fn.
func optoutDo(fn func(http.ResponseWriter, *http.Request) error, ws *domain.Website, method, target string, form url.Values, hdr http.Header) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, target, body)
	req.Host = "demo.test"
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, vs := range hdr {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	rec := httptest.NewRecorder()
	LocaleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if method == http.MethodGet {
			r.SetPathValue("slug", strings.TrimPrefix(r.URL.Path, "/"))
		}
		if err := fn(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})).ServeHTTP(rec, req)
	return rec
}

func optoutGet(t *testing.T, h *Handler, ws *domain.Website, target string, hdr http.Header) *httptest.ResponseRecorder {
	t.Helper()
	rec := optoutDo(h.HandlePage, ws, http.MethodGet, target, nil, hdr)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d; want 200\n%s", target, rec.Code, rec.Body.String())
	}
	return rec
}

func optoutGPC(v string) http.Header {
	return http.Header{"Sec-Gpc": []string{v}}
}

// optoutStamp mints the hidden time stamp as the widget does, age ago.
func optoutStamp(age time.Duration) string {
	issued := time.Now().Add(-age)
	return optoutSigner.Token(issued.Unix(), issued.Add(12*time.Hour))
}

func optoutForm(slug string) url.Values {
	return url.Values{
		"seite":   {slug},
		"stempel": {optoutStamp(10 * time.Second)},
		"name":    {"Ada"},
		"email":   {"ada@example.com"},
		"note":    {"bitte"},
		"website": {""},
	}
}

func optoutRows(t *testing.T, database *db.DB, websiteID int64) int {
	t.Helper()
	var n int
	if err := database.Read.QueryRow(
		`SELECT COUNT(*) FROM privacy_requests WHERE website_id = $1`, websiteID).Scan(&n); err != nil {
		t.Fatalf("count requests: %v", err)
	}
	return n
}

// optoutFind walks the parsed page and returns the first element match accepts.
func optoutFind(n *xhtml.Node, match func(*xhtml.Node) bool) *xhtml.Node {
	if n.Type == xhtml.ElementNode && match(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := optoutFind(c, match); found != nil {
			return found
		}
	}
	return nil
}

func optoutAttr(n *xhtml.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func optoutHasClass(n *xhtml.Node, class string) bool {
	v, _ := optoutAttr(n, "class")
	for _, c := range strings.Fields(v) {
		if c == class {
			return true
		}
	}
	return false
}

func optoutNamed(name string) func(*xhtml.Node) bool {
	return func(n *xhtml.Node) bool {
		v, ok := optoutAttr(n, "name")
		return ok && v == name && (n.Data == "input" || n.Data == "textarea")
	}
}

const optoutMarkerPage = `<p>Text</p><p>[[widerspruch]]</p>`

func TestTheWidgetShowsOnAUSWebsite(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "")
	optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")

	body := optoutGet(t, h, ws, "/do-not-sell", nil).Body.String()
	if strings.Contains(body, "[[widerspruch]]") {
		t.Fatalf("the marker reached the visitor:\n%s", body)
	}
	// The paragraph around the marker goes with it: a form inside a <p> is
	// markup the browser repairs by closing the paragraph early.
	if strings.Contains(body, "<p><form") || strings.Contains(body, "<p></p>") {
		t.Errorf("the paragraph around the marker is still there:\n%s", body)
	}

	doc, err := xhtml.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	widget := optoutFind(doc, func(n *xhtml.Node) bool { _, ok := optoutAttr(n, "data-hc-optout"); return ok })
	if widget == nil {
		t.Fatalf("no element carries data-hc-optout:\n%s", body)
	}
	form := optoutFind(widget, func(n *xhtml.Node) bool { return n.Data == "form" })
	if form == nil {
		form = optoutFind(doc, func(n *xhtml.Node) bool { return n.Data == "form" })
	}
	if form == nil {
		t.Fatalf("no form:\n%s", body)
	}
	if action, _ := optoutAttr(form, "action"); !strings.HasSuffix(action, "/widerspruch") {
		t.Errorf("form action = %q; want …/widerspruch", action)
	}
	if method, _ := optoutAttr(form, "method"); !strings.EqualFold(method, "post") {
		t.Errorf("form method = %q; want post", method)
	}
	for _, name := range []string{"name", "email", "note"} {
		if optoutFind(form, optoutNamed(name)) == nil {
			t.Errorf("the form has no field %q", name)
		}
	}
	for _, name := range []string{"seite", "stempel"} {
		f := optoutFind(form, optoutNamed(name))
		if f == nil {
			t.Errorf("the form has no hidden %q", name)
			continue
		}
		if typ, _ := optoutAttr(f, "type"); typ != "hidden" {
			t.Errorf("%q has type %q; want hidden", name, typ)
		}
	}
	if seite := optoutFind(form, optoutNamed("seite")); seite != nil {
		if v, _ := optoutAttr(seite, "value"); v != "do-not-sell" {
			t.Errorf("seite = %q; want the page's slug", v)
		}
	}
	trap := optoutFind(form, func(n *xhtml.Node) bool { return optoutHasClass(n, "contact-form__trap") })
	if trap == nil || optoutFind(trap, optoutNamed("website")) == nil {
		t.Errorf("the honeypot \"website\" is not inside .contact-form__trap")
	}
	if strings.Contains(body, "hx-") {
		t.Errorf("the public widget carries an htmx attribute; the public site loads no script")
	}
}

func TestTheWidgetSaysWhetherGPCWasSent(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "")
	optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")
	honoured := i18n.T(ws.Locale, optoutHonoured)
	notSent := i18n.T(ws.Locale, optoutNotSent)
	if honoured == optoutHonoured || notSent == optoutNotSent {
		t.Fatalf("the German catalogue lacks the status sentences")
	}

	for _, tc := range []struct {
		name     string
		hdr      http.Header
		honoured bool
	}{
		{"Sec-GPC: 1", optoutGPC("1"), true},
		{"no header", nil, false},
		{"Sec-GPC: 0", optoutGPC("0"), false},
	} {
		body := optoutGet(t, h, ws, "/do-not-sell", tc.hdr).Body.String()
		hasHonoured := strings.Contains(body, htmlText(honoured))
		hasNotSent := strings.Contains(body, htmlText(notSent))
		if tc.honoured && (!hasHonoured || hasNotSent) {
			t.Errorf("%s: honoured=%v notSent=%v; want only the honoured sentence", tc.name, hasHonoured, hasNotSent)
		}
		if !tc.honoured && (hasHonoured || !hasNotSent) {
			t.Errorf("%s: honoured=%v notSent=%v; want only the not-sent sentence", tc.name, hasHonoured, hasNotSent)
		}
	}
}

// htmlText is a sentence as it stands in markup: escaped.
func htmlText(s string) string {
	var b strings.Builder
	xhtml.Render(&b, &xhtml.Node{Type: xhtml.TextNode, Data: s})
	return b.String()
}

func TestNoWidgetWithoutUS(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,ch", "")
	optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")

	body := optoutGet(t, h, ws, "/do-not-sell", optoutGPC("1")).Body.String()
	if strings.Contains(body, "[[widerspruch]]") || strings.Contains(body, "data-hc-optout") || strings.Contains(body, "/widerspruch") {
		t.Errorf("a website without US shows the widget or the marker:\n%s", body)
	}
	if !strings.Contains(body, "<p>Text</p>") {
		t.Errorf("the rest of the page went with the marker:\n%s", body)
	}

	rec := optoutDo(h.HandleOptOut, ws, http.MethodPost, "/widerspruch", optoutForm("do-not-sell"), nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("POST /widerspruch without US = %d; want 404", rec.Code)
	}
	if n := optoutRows(t, database, ws.ID); n != 0 {
		t.Errorf("%d rows stored on a website without US", n)
	}
}

func TestAMarkerInAnAttributeIsNotExpanded(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "")
	optoutPage(t, database, ws.ID, "do-not-sell", `<p><a href="/x?[[widerspruch]]">x</a></p>`, "")

	body := optoutGet(t, h, ws, "/do-not-sell", nil).Body.String()
	if strings.Contains(body, "[[widerspruch]]") {
		t.Errorf("the marker stayed in the attribute:\n%s", body)
	}
	if strings.Contains(body, "data-hc-optout") || strings.Contains(body, `action="/widerspruch"`) {
		t.Errorf("a form was spliced into an attribute:\n%s", body)
	}
	if !strings.Contains(body, `<a href="/x?">x</a>`) {
		t.Errorf("the link lost more than the marker:\n%s", body)
	}
}

func TestASubmissionIsStored(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "")
	optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")

	rec := optoutDo(h.HandleOptOut, ws, http.MethodPost, "/widerspruch", optoutForm("do-not-sell"), optoutGPC("1"))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST = %d; want 303\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/do-not-sell?widerspruch=eingegangen" {
		t.Errorf("Location = %q; want /do-not-sell?widerspruch=eingegangen", loc)
	}
	if c := rec.Header().Values("Set-Cookie"); len(c) != 0 {
		t.Errorf("the opt-out set a cookie: %v", c)
	}

	list, err := dsr.NewStore(database).List(context.Background(), ws.ID, dsr.KindOptOut)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("%d rows; want exactly one", len(list))
	}
	got := list[0]
	if got.Name != "Ada" || got.Email != "ada@example.com" || got.Note != "bitte" || !got.GPC || got.Locale != ws.Locale {
		t.Errorf("stored %+v; want Ada, ada@example.com, bitte, GPC, locale %q", got, ws.Locale)
	}

	body := optoutGet(t, h, ws, "/do-not-sell?widerspruch=eingegangen", nil).Body.String()
	if !strings.Contains(body, htmlText(i18n.T(ws.Locale, optoutRecorded))) {
		t.Errorf("the page after the redirect does not say the request is recorded:\n%s", body)
	}
}

func TestSpanishPagesGetEnglishWidgetWords(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "es")
	main := optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")
	es := optoutPage(t, database, ws.ID, "no-vender", optoutMarkerPage, "")
	if err := page.NewStore(database).SetTranslation(context.Background(), ws.ID, es.ID, "es", main.ID); err != nil {
		t.Fatalf("SetTranslation: %v", err)
	}

	spanish := optoutGet(t, h, ws, "/es/no-vender", nil).Body.String()
	if !strings.Contains(spanish, optoutSubmit) {
		t.Errorf("the Spanish page does not carry the English button %q (D-16):\n%s", optoutSubmit, spanish)
	}
	if !strings.Contains(spanish, `action="/es/widerspruch"`) {
		t.Errorf("the Spanish form does not post inside its language:\n%s", spanish)
	}

	german := optoutGet(t, h, ws, "/do-not-sell", nil).Body.String()
	want := i18n.T("de", optoutSubmit)
	if want == optoutSubmit {
		t.Fatalf("the German catalogue lacks %q", optoutSubmit)
	}
	if !strings.Contains(german, want) {
		t.Errorf("the German page does not carry %q:\n%s", want, german)
	}
}
