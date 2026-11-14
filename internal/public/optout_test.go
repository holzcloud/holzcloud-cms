package public

import (
	"context"
	"encoding/json"
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
	"github.com/holzcloud/holzcloud-cms/internal/mail"
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

// optoutAddSite adds a further website with the given set to an existing
// database, for the tests that need two.
func optoutAddSite(t *testing.T, database *db.DB, name, set string) *domain.Website {
	t.Helper()
	ws := seedWebsite(t, database, name)
	if _, err := database.Write.Exec(`UPDATE websites SET legal_frameworks = $1 WHERE id = $2`, set, ws.ID); err != nil {
		t.Fatalf("set frameworks: %v", err)
	}
	fresh, err := domain.NewStore(database).GetWebsite(context.Background(), ws.ID)
	if err != nil || fresh == nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	return fresh
}

func optoutPost(h *Handler, ws *domain.Website, form url.Values, hdr http.Header) *httptest.ResponseRecorder {
	return optoutDo(h.HandleOptOut, ws, http.MethodPost, "/widerspruch", form, hdr)
}

func TestTheHoneypotAndTheStampAnswerLikeASuccess(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "")
	optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")

	// Only what a robot does lands here. A stamp that is genuine but too old
	// or too young is a person's, and gets its own answer
	// (TestAGenuineButStaleStampAsksToSendAgain).
	for _, tc := range []struct {
		name   string
		change func(url.Values)
	}{
		{"a filled honeypot", func(f url.Values) { f.Set("website", "https://spam.example") }},
		{"a filled honeypot with a stale stamp", func(f url.Values) {
			f.Set("website", "https://spam.example")
			f.Set("stempel", optoutStamp(time.Second))
		}},
		{"no stamp", func(f url.Values) { f.Del("stempel") }},
		{"a forged stamp", func(f url.Values) { f.Set("stempel", "1700000000-1900000000.forged") }},
		{"a stamp signed with another key", func(f url.Values) {
			issued := time.Now().Add(-10 * time.Second)
			f.Set("stempel", sharelink.New([]byte("other")).Token(issued.Unix(), issued.Add(time.Hour)))
		}},
	} {
		form := optoutForm("do-not-sell")
		tc.change(form)
		rec := optoutPost(h, ws, form, nil)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/do-not-sell?widerspruch=eingegangen" {
			t.Errorf("%s: %d %q; want the success redirect, so a robot cannot tell which trap caught it",
				tc.name, rec.Code, rec.Header().Get("Location"))
		}
		if n := optoutRows(t, database, ws.ID); n != 0 {
			t.Errorf("%s: %d rows stored; want none", tc.name, n)
		}
	}
}

// A stamp that verifies came from this page, so a request carrying one is a
// person's: a tab left open past the stamp's lifetime, or a form the browser
// filled in and the visitor sent at once. Telling them "recorded" while
// storing nothing would be untrue about a request the law says must be taken
// (CCPA regs § 7026(a)(1)). They are asked to send it again instead, and the
// page they land on carries a fresh stamp.
func TestAGenuineButStaleStampAsksToSendAgain(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "")
	optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")

	expired := time.Now().Add(-optoutMaxAge - time.Minute)
	for _, tc := range []struct {
		name  string
		stamp string
	}{
		{"an expired stamp", optoutSigner.Token(expired.Unix(), expired.Add(optoutMaxAge))},
		{"a stamp younger than a person reads", optoutStamp(0)},
	} {
		form := optoutForm("do-not-sell")
		form.Set("stempel", tc.stamp)
		rec := optoutPost(h, ws, form, nil)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/do-not-sell?widerspruch=abgelaufen" {
			t.Errorf("%s: %d %q; want 303 to ?widerspruch=abgelaufen", tc.name, rec.Code, rec.Header().Get("Location"))
		}
		if n := optoutRows(t, database, ws.ID); n != 0 {
			t.Errorf("%s: %d rows stored; want none", tc.name, n)
		}
	}

	body := optoutGet(t, h, ws, "/do-not-sell?widerspruch=abgelaufen", nil).Body.String()
	again := i18n.T(ws.Locale, "The form was open too long or sent too quickly, so the request has not been recorded. Please send it again.")
	if again == "The form was open too long or sent too quickly, so the request has not been recorded. Please send it again." {
		t.Fatalf("the German catalogue lacks the send-again sentence")
	}
	if !strings.Contains(body, htmlText(again)) || !strings.Contains(body, "contact-form__notice--error") {
		t.Errorf("the page does not ask to send again with %q:\n%s", again, body)
	}
	if strings.Contains(body, htmlText(i18n.T(ws.Locale, optoutRecorded))) {
		t.Errorf("the page says the request was recorded:\n%s", body)
	}
	if !strings.Contains(body, `name="stempel"`) {
		t.Errorf("the page has no fresh stamp to send again with:\n%s", body)
	}
}

func TestTheHourlyCapRefuses(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "")
	optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")
	other := optoutAddSite(t, database, "Nachbar", "us")
	optoutPage(t, database, other.ID, "do-not-sell", optoutMarkerPage, "")

	for i := 0; i < 30; i++ {
		if _, err := database.Write.Exec(
			`INSERT INTO privacy_requests (website_id, kind, name) VALUES ($1, 'optout', 'earlier')`, ws.ID); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	rec := optoutPost(h, ws, optoutForm("do-not-sell"), nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/do-not-sell?widerspruch=spaeter" {
		t.Errorf("the 31st opt-out in an hour: %d %q; want 303 to ?widerspruch=spaeter", rec.Code, rec.Header().Get("Location"))
	}
	if n := optoutRows(t, database, ws.ID); n != 30 {
		t.Errorf("%d rows; want the 30 there were", n)
	}
	body := optoutGet(t, h, ws, "/do-not-sell?widerspruch=spaeter", nil).Body.String()
	notice := i18n.T(ws.Locale, "Too many requests at the moment. Please try again in an hour.")
	if !strings.Contains(body, htmlText(notice)) || !strings.Contains(body, "contact-form__notice--error") {
		t.Errorf("the page does not show the cap notice %q:\n%s", notice, body)
	}

	rec = optoutPost(h, other, optoutForm("do-not-sell"), nil)
	if rec.Header().Get("Location") != "/do-not-sell?widerspruch=eingegangen" || optoutRows(t, database, other.ID) != 1 {
		t.Errorf("another website was capped by the first one's requests: %q", rec.Header().Get("Location"))
	}
}

func TestACrossSitePostIsRefused(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "")
	optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")

	rec := optoutPost(h, ws, optoutForm("do-not-sell"), http.Header{"Sec-Fetch-Site": {"cross-site"}})
	if rec.Code != http.StatusForbidden {
		t.Errorf("a cross-site POST = %d; want 403", rec.Code)
	}
	if n := optoutRows(t, database, ws.ID); n != 0 {
		t.Fatalf("a cross-site POST stored %d rows", n)
	}
	for _, hdr := range []http.Header{{"Sec-Fetch-Site": {"same-origin"}}, nil} {
		before := optoutRows(t, database, ws.ID)
		rec := optoutPost(h, ws, optoutForm("do-not-sell"), hdr)
		if rec.Code != http.StatusSeeOther || optoutRows(t, database, ws.ID) != before+1 {
			t.Errorf("Sec-Fetch-Site %v: %d; want accepted and stored", hdr, rec.Code)
		}
	}
}

func TestABodyTooLargeIsRefused(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "")
	optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")

	form := optoutForm("do-not-sell")
	form.Set("note", strings.Repeat("x", 20<<10))
	rec := optoutPost(h, ws, form, nil)
	if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest {
		t.Errorf("a 20 KiB body = %d; want 413 or 400", rec.Code)
	}
	if n := optoutRows(t, database, ws.ID); n != 0 {
		t.Errorf("a body over the limit stored %d rows", n)
	}
}

type optoutMail struct{ to, subject, body, replyTo string }

func optoutOutbox(t *testing.T, database *db.DB, websiteID int64) []optoutMail {
	t.Helper()
	rows, err := database.Read.Query(
		`SELECT recipient, subject, body, reply_to FROM mail_outbox WHERE website_id = $1 ORDER BY id`, websiteID)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	defer rows.Close()
	var out []optoutMail
	for rows.Next() {
		var m optoutMail
		if err := rows.Scan(&m.to, &m.subject, &m.body, &m.replyTo); err != nil {
			t.Fatalf("scan outbox: %v", err)
		}
		out = append(out, m)
	}
	return out
}

func optoutReload(t *testing.T, database *db.DB, ws *domain.Website, set string) *domain.Website {
	t.Helper()
	if _, err := database.Write.Exec(`UPDATE websites SET `+set+` WHERE id = $1`, ws.ID); err != nil {
		t.Fatalf("settings: %v", err)
	}
	fresh, err := domain.NewStore(database).GetWebsite(context.Background(), ws.ID)
	if err != nil || fresh == nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	return fresh
}

func TestTheOperatorIsToldInTheWebsitesLanguage(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "")
	optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")
	ws = optoutReload(t, database, ws, `locale = 'fr', notify_email = 'owner@example.com'`)
	enabled := mail.NewQueue(database, mail.NewSender(mail.Config{Host: "smtp.example", Port: 587, From: "cms@example.com"}), nil)
	h.SetNotify(domain.NewStore(database), enabled)

	form := optoutForm("do-not-sell")
	form.Set("note", "merci")
	if rec := optoutPost(h, ws, form, optoutGPC("1")); rec.Code != http.StatusSeeOther {
		t.Fatalf("POST = %d", rec.Code)
	}
	msgs := optoutOutbox(t, database, ws.ID)
	if len(msgs) != 1 {
		t.Fatalf("%d messages; want exactly one", len(msgs))
	}
	m := msgs[0]
	subject := i18n.T("fr", "New opt-out request (Do Not Sell or Share)")
	if subject == "New opt-out request (Do Not Sell or Share)" {
		t.Fatalf("the French catalogue lacks the subject")
	}
	if m.to != "owner@example.com" || m.subject != "["+ws.Name+"] "+subject {
		t.Errorf("message to %q, subject %q; want owner@example.com, %q", m.to, m.subject, "["+ws.Name+"] "+subject)
	}
	for _, want := range []string{"Ada", "ada@example.com", "merci",
		i18n.T("fr", "The browser sent the Global Privacy Control signal."),
		i18n.T("fr", "The request is listed under Settings → Opt-outs of this website.")} {
		if !strings.Contains(m.body, want) {
			t.Errorf("the body lacks %q:\n%s", want, m.body)
		}
	}
	if m.replyTo != "" {
		t.Errorf("reply_to = %q; the visitor's address must stay in the body", m.replyTo)
	}

	// No notification address: nothing is sent, and the request is still kept.
	ws = optoutReload(t, database, ws, `notify_email = ''`)
	optoutPost(h, ws, optoutForm("do-not-sell"), nil)
	if n := len(optoutOutbox(t, database, ws.ID)); n != 1 {
		t.Errorf("%d messages after a request without a notification address; want still 1", n)
	}

	// Mail not set up: nothing is sent either.
	ws = optoutReload(t, database, ws, `notify_email = 'owner@example.com'`)
	h.SetNotify(domain.NewStore(database), mail.NewQueue(database, nil, nil))
	optoutPost(h, ws, optoutForm("do-not-sell"), nil)
	if n := len(optoutOutbox(t, database, ws.ID)); n != 1 {
		t.Errorf("%d messages with no mail server; want still 1", n)
	}
	if n := optoutRows(t, database, ws.ID); n != 3 {
		t.Errorf("%d requests stored; want all 3, sent or not", n)
	}
}

func optoutVaryNames(rec *httptest.ResponseRecorder) map[string]bool {
	names := map[string]bool{}
	for _, v := range rec.Header().Values("Vary") {
		for _, part := range strings.Split(v, ",") {
			names[strings.ToLower(strings.TrimSpace(part))] = true
		}
	}
	return names
}

func TestTheWidgetPageVariesOnGPC(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "")
	optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")

	with := optoutGet(t, h, ws, "/do-not-sell", optoutGPC("1"))
	without := optoutGet(t, h, ws, "/do-not-sell", nil)
	for name, rec := range map[string]*httptest.ResponseRecorder{"with": with, "without": without} {
		vary := optoutVaryNames(rec)
		if !vary["hx-request"] || !vary["sec-gpc"] {
			t.Errorf("%s Sec-GPC: Vary = %v; want HX-Request and Sec-GPC (a shared cache must not hand one answer to the other)",
				name, rec.Header().Values("Vary"))
		}
	}
	if with.Body.String() == without.Body.String() {
		t.Error("the bodies with and without the signal are the same")
	}
	if with.Header().Get("ETag") == without.Header().Get("ETag") {
		t.Error("the ETags with and without the signal are the same")
	}
}

func TestGPCChangesNothingElse(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "")
	optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")
	optoutPage(t, database, ws.ID, "home", "<p>Willkommen</p>", "")
	optoutPage(t, database, ws.ID, "ueber-uns", "<p>Wir sind ein Hofladen.</p>", "")

	for _, tc := range []struct {
		target string
		fn     func(http.ResponseWriter, *http.Request) error
	}{
		{"/ueber-uns", h.HandlePage},
		{"/", h.HandleHome},
	} {
		with := optoutDo(tc.fn, ws, http.MethodGet, tc.target, nil, optoutGPC("1"))
		without := optoutDo(tc.fn, ws, http.MethodGet, tc.target, nil, nil)
		if with.Code != http.StatusOK || without.Code != http.StatusOK {
			t.Fatalf("%s: %d / %d; want 200", tc.target, with.Code, without.Code)
		}
		if with.Body.String() != without.Body.String() {
			t.Errorf("%s differs with Sec-GPC: 1 — honouring the signal must change nothing but the widget (D-12)", tc.target)
		}
		if optoutVaryNames(with)["sec-gpc"] || optoutVaryNames(without)["sec-gpc"] {
			t.Errorf("%s names Sec-GPC in Vary without carrying the widget: %v", tc.target, with.Header().Values("Vary"))
		}
	}
}

func TestGPCJSONOnlyForUS(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "")
	rec := optoutDo(h.HandleGPCJSON, ws, http.MethodGet, "/.well-known/gpc.json", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("gpc.json on a US website = %d; want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q; want application/json", ct)
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	if doc["gpc"] != true || len(doc) != 1 {
		t.Errorf("gpc.json = %v; want {\"gpc\": true}", doc)
	}

	other := optoutAddSite(t, database, "Nachbar", "eu,ch")
	if rec := optoutDo(h.HandleGPCJSON, other, http.MethodGet, "/.well-known/gpc.json", nil, nil); rec.Code != http.StatusNotFound {
		t.Errorf("gpc.json on a website without US = %d; want 404", rec.Code)
	}
}
