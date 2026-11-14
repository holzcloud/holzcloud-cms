package public

import (
	"context"
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
)

// The request form of Phase 33.2 (D-01): [[datenanfrage]] on any website, and
// POST /datenanfrage. It stands on the opt-out form's machinery, so the
// opt-out tests' helpers serve here too; the ones of this file start with
// datarequest.

// The sentences the form prints, repeated and not imported for the reason
// optout_test.go gives.
const (
	datarequestRecorded = "Your request has been recorded. The operator of this website answers it after checking that it is yours."
	datarequestAddress  = "Please give the e-mail address your request is about."
	datarequestChoose   = "Please choose what you would like."
)

const datarequestMarkerPage = `<p>Text</p><p>[[datenanfrage]]</p>`

// datarequestStamp mints the form's hidden time stamp on ws, age ago.
func datarequestStamp(ws *domain.Website, age time.Duration) string {
	return DataRequestStamp(optoutSigner, ws.ID, time.Now().Add(-age))
}

func datarequestForm(ws *domain.Website, slug string) url.Values {
	return url.Values{
		"seite":   {slug},
		"stempel": {datarequestStamp(ws, 10*time.Second)},
		"art":     {dsr.KindAccess},
		"name":    {"Ada"},
		"email":   {"ada@example.com"},
		"note":    {"bitte"},
		"website": {""},
	}
}

func datarequestPost(h *Handler, ws *domain.Website, form url.Values, hdr http.Header) *httptest.ResponseRecorder {
	return optoutDo(h.HandleDataRequest, ws, http.MethodPost, "/datenanfrage", form, hdr)
}

// datarequestOrder stores an order for an address, so that the CMS holds
// data about it.
func datarequestOrder(t *testing.T, database *db.DB, websiteID int64, email string) {
	t.Helper()
	if _, err := database.Write.Exec(`INSERT INTO orders (website_id, number, token, audience, currency,
		email, name, street, postal_code, city,
		items_net, items_tax, items_gross, total_net, total_tax, total_gross,
		created_at, updated_at)
		VALUES ($1, '2026-0001', 'tok-1', 'private', 'CHF', $2, 'Known Customer', 'Weg 1', '3000', 'Bern',
		2500, 0, 2500, 2500, 0, 2500, '2026-04-01T08:00:00Z', '2026-04-01T08:00:00Z')`,
		websiteID, email); err != nil {
		t.Fatalf("insert order: %v", err)
	}
}

func TestTheRequestFormShowsOnAnyWebsite(t *testing.T) {
	// No US here: the request form is not the opt-out's, it belongs on
	// every website.
	h, database, ws := optoutSite(t, "eu,ch", "")
	optoutPage(t, database, ws.ID, "meine-daten", datarequestMarkerPage+`<p>[[datenanfrage]]</p>`, "")

	body := optoutGet(t, h, ws, "/meine-daten", nil).Body.String()
	if strings.Contains(body, "[[datenanfrage]]") {
		t.Fatalf("the marker reached the visitor:\n%s", body)
	}
	if strings.Contains(body, "<p><form") || strings.Contains(body, "<p></p>") {
		t.Errorf("the paragraph around the marker is still there:\n%s", body)
	}
	if n := strings.Count(body, dataRequestWidgetMark); n != 1 {
		t.Errorf("%d forms on the page; want one, a second marker renders nothing", n)
	}

	doc, err := xhtml.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	form := optoutFind(doc, func(n *xhtml.Node) bool {
		_, ok := optoutAttr(n, dataRequestWidgetMark)
		return ok && n.Data == "form"
	})
	if form == nil {
		t.Fatalf("no form carries %s:\n%s", dataRequestWidgetMark, body)
	}
	if action, _ := optoutAttr(form, "action"); action != "/datenanfrage" {
		t.Errorf("form action = %q; want /datenanfrage", action)
	}
	if method, _ := optoutAttr(form, "method"); !strings.EqualFold(method, "post") {
		t.Errorf("form method = %q; want post", method)
	}
	sel := optoutFind(form, func(n *xhtml.Node) bool { v, _ := optoutAttr(n, "name"); return n.Data == "select" && v == "art" })
	if sel == nil {
		t.Fatalf("the form has no select \"art\":\n%s", body)
	}
	var values []string
	for c := sel.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == xhtml.ElementNode && c.Data == "option" {
			v, _ := optoutAttr(c, "value")
			values = append(values, v)
		}
	}
	if strings.Join(values, ",") != "access,correction,erasure" {
		t.Errorf("the options are %v; want access, correction, erasure", values)
	}
	email := optoutFind(form, optoutNamed("email"))
	if email == nil {
		t.Fatal("the form has no field email")
	}
	if _, ok := optoutAttr(email, "required"); !ok {
		t.Error("the address is not required")
	}
	if typ, _ := optoutAttr(email, "type"); typ != "email" {
		t.Errorf("the address field has type %q; want email", typ)
	}
	for _, name := range []string{"name", "note"} {
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
	trap := optoutFind(form, func(n *xhtml.Node) bool { return optoutHasClass(n, "contact-form__trap") })
	if trap == nil || optoutFind(trap, optoutNamed("website")) == nil {
		t.Error("the honeypot \"website\" is not inside .contact-form__trap")
	}
	if strings.Contains(body, "hx-") {
		t.Error("the public form carries an htmx attribute; the public site loads no script")
	}
}

func TestARequestIsStored(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,ch", "")
	optoutPage(t, database, ws.ID, "meine-daten", datarequestMarkerPage, "")
	datarequestOrder(t, database, ws.ID, "known@example.com")

	form := datarequestForm(ws, "meine-daten")
	form.Set("email", "  Ada@Example.com ")
	rec := datarequestPost(h, ws, form, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST = %d; want 303\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/meine-daten?datenanfrage=eingegangen" {
		t.Errorf("Location = %q; want /meine-daten?datenanfrage=eingegangen", loc)
	}
	if c := rec.Header().Values("Set-Cookie"); len(c) != 0 {
		t.Errorf("the request set a cookie: %v", c)
	}
	list, err := dsr.NewStore(database).ListData(context.Background(), ws.ID)
	if err != nil {
		t.Fatalf("ListData: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("%d rows; want exactly one", len(list))
	}
	got := list[0]
	if got.Kind != dsr.KindAccess || got.Manner != dsr.MannerForm || got.Email != "Ada@Example.com" ||
		got.Name != "Ada" || got.Note != "bitte" || got.Locale != ws.Locale || got.GPC {
		t.Errorf("stored %+v; want access, form, Ada@Example.com as typed, Ada, bitte", got)
	}

	// The same answer for an address the website knows: the form is no
	// oracle for "is this address a customer here?".
	known := datarequestForm(ws, "meine-daten")
	known.Set("email", "known@example.com")
	knownRec := datarequestPost(h, ws, known, nil)
	if knownRec.Code != rec.Code || knownRec.Header().Get("Location") != rec.Header().Get("Location") {
		t.Errorf("a known address: %d %q; want exactly the answer of an unknown one",
			knownRec.Code, knownRec.Header().Get("Location"))
	}

	page := optoutGet(t, h, ws, "/meine-daten?datenanfrage=eingegangen", nil).Body.String()
	say := i18n.T(ws.Locale, datarequestRecorded)
	if say == datarequestRecorded {
		t.Fatal("the German catalogue lacks the recorded sentence")
	}
	if !strings.Contains(page, htmlText(say)) {
		t.Errorf("the page after the redirect does not say the request is recorded:\n%s", page)
	}
	for _, leak := range []string{"known@example.com", "Known Customer", "Ada@Example.com", "2026-0001"} {
		if strings.Contains(page, leak) {
			t.Errorf("the page shows %q", leak)
		}
	}
}

func TestTheRequestFormRefusesLikeTheOptOut(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,ch", "")
	optoutPage(t, database, ws.ID, "meine-daten", datarequestMarkerPage, "")
	rows := func() int { return optoutRows(t, database, ws.ID) }

	// What a robot does gets the success answer and stores nothing.
	for _, tc := range []struct {
		name   string
		change func(url.Values)
	}{
		{"a filled honeypot", func(f url.Values) { f.Set("website", "https://spam.example") }},
		{"no stamp", func(f url.Values) { f.Del("stempel") }},
		{"a forged stamp", func(f url.Values) { f.Set("stempel", "1700000000-1900000000.forged") }},
		{"an opt-out stamp", func(f url.Values) { f.Set("stempel", optoutStamp(ws, 10*time.Second)) }},
	} {
		form := datarequestForm(ws, "meine-daten")
		tc.change(form)
		rec := datarequestPost(h, ws, form, nil)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/meine-daten?datenanfrage=eingegangen" {
			t.Errorf("%s: %d %q; want the success redirect", tc.name, rec.Code, rec.Header().Get("Location"))
		}
		if n := rows(); n != 0 {
			t.Errorf("%s: %d rows stored; want none", tc.name, n)
		}
	}

	// A request stamp is a forgery on the opt-out form, the other way round.
	optoutSet := optoutReload(t, database, ws, `legal_frameworks = 'eu,us'`)
	optoutPage(t, database, ws.ID, "do-not-sell", optoutMarkerPage, "")
	cross := optoutForm(optoutSet, "do-not-sell")
	cross.Set("stempel", datarequestStamp(optoutSet, 10*time.Second))
	if rec := optoutPost(h, optoutSet, cross, nil); rec.Header().Get("Location") != "/do-not-sell?widerspruch=eingegangen" || rows() != 0 {
		t.Errorf("a request stamp on the opt-out form: %q, %d rows; want the silent success and none", rec.Header().Get("Location"), rows())
	}

	for _, tc := range []struct {
		name    string
		change  func(url.Values)
		outcome string
	}{
		{"a stamp three hours old", func(f url.Values) { f.Set("stempel", datarequestStamp(ws, 3*time.Hour)) }, "abgelaufen"},
		{"no address", func(f url.Values) { f.Set("email", "") }, "adresse"},
		{"not an address", func(f url.Values) { f.Set("email", "ada@example.com?bcc=x@evil.example") }, "adresse"},
		{"an unknown kind", func(f url.Values) { f.Set("art", "delete-everything") }, "art"},
		{"an opt-out as the kind", func(f url.Values) { f.Set("art", dsr.KindOptOut) }, "art"},
		{"a note too long", func(f url.Values) { f.Set("note", strings.Repeat("x", dsr.MaxNote+1)) }, "zulang"},
	} {
		form := datarequestForm(ws, "meine-daten")
		tc.change(form)
		rec := datarequestPost(h, ws, form, nil)
		if want := "/meine-daten?datenanfrage=" + tc.outcome; rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want {
			t.Errorf("%s: %d %q; want 303 to %s", tc.name, rec.Code, rec.Header().Get("Location"), want)
		}
		if n := rows(); n != 0 {
			t.Errorf("%s: %d rows stored; want none", tc.name, n)
		}
	}
	page := optoutGet(t, h, ws, "/meine-daten?datenanfrage=adresse", nil).Body.String()
	if say := i18n.T(ws.Locale, datarequestAddress); say == datarequestAddress || !strings.Contains(page, htmlText(say)) {
		t.Errorf("the page does not ask for the address (%q)", say)
	}
	page = optoutGet(t, h, ws, "/meine-daten?datenanfrage=art", nil).Body.String()
	if say := i18n.T(ws.Locale, datarequestChoose); say == datarequestChoose || !strings.Contains(page, htmlText(say)) {
		t.Errorf("the page does not ask to choose (%q)", say)
	}

	if rec := datarequestPost(h, ws, datarequestForm(ws, "meine-daten"), http.Header{"Sec-Fetch-Site": {"cross-site"}}); rec.Code != http.StatusForbidden {
		t.Errorf("a cross-site POST = %d; want 403", rec.Code)
	}
	if rec := datarequestPost(h, ws, datarequestForm(ws, "meine-daten"), http.Header{"Origin": {"https://evil.example"}}); rec.Code != http.StatusForbidden {
		t.Errorf("a foreign Origin = %d; want 403", rec.Code)
	}
	big := datarequestForm(ws, "meine-daten")
	big.Set("note", strings.Repeat("x", 17<<10))
	if rec := datarequestPost(h, ws, big, nil); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("a 17 KiB body = %d; want 413", rec.Code)
	}
	if n := rows(); n != 0 {
		t.Errorf("%d rows stored by the refused posts; want none", n)
	}
}

func TestTheOperatorIsToldAboutTheRequest(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,ch", "")
	optoutPage(t, database, ws.ID, "meine-daten", datarequestMarkerPage, "")
	ws = optoutReload(t, database, ws, `locale = 'fr', notify_email = 'owner@example.com'`)
	h.SetNotify(domain.NewStore(database), mail.NewQueue(database, mail.NewSender(mail.Config{Host: "smtp.example", Port: 587, From: "cms@example.com"}), nil))

	if rec := datarequestPost(h, ws, datarequestForm(ws, "meine-daten"), nil); rec.Code != http.StatusSeeOther {
		t.Fatalf("POST = %d", rec.Code)
	}
	msgs := optoutOutbox(t, database, ws.ID)
	if len(msgs) != 1 {
		t.Fatalf("%d messages; want exactly one", len(msgs))
	}
	m := msgs[0]
	subject := i18n.T("fr", "New request for access to personal data")
	if subject == "New request for access to personal data" {
		t.Fatal("the French catalogue lacks the subject")
	}
	if m.to != "owner@example.com" || m.subject != "["+ws.Name+"] "+subject {
		t.Errorf("message to %q, subject %q; want owner@example.com, %q", m.to, m.subject, "["+ws.Name+"] "+subject)
	}
	for _, want := range []string{"ada@example.com", "Ada", "bitte",
		i18n.T("fr", "The request is listed under Settings → Privacy → Requests of this website.")} {
		if !strings.Contains(m.body, want) {
			t.Errorf("the body lacks %q:\n%s", want, m.body)
		}
	}
	if m.replyTo != "" {
		t.Errorf("reply_to = %q; the visitor's address must stay in the body", m.replyTo)
	}
}
