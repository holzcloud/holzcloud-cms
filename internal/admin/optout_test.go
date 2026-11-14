package admin

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The Opt-outs screen (phase 33.1, plan 08). Every helper in this file starts
// with optout: plan 06 changes this package in the same wave.

func optoutList(websiteID int64) string {
	return fmt.Sprintf("/admin/websites/%d/widersprueche", websiteID)
}

// optoutFrameworks sets a website's legal frameworks the way the settings
// screen stores them.
func optoutFrameworks(t *testing.T, database *db.DB, websiteID int64, set string) {
	t.Helper()
	if _, err := database.Write.Exec(`UPDATE websites SET legal_frameworks = $1 WHERE id = $2`, set, websiteID); err != nil {
		t.Fatalf("set legal_frameworks %q: %v", set, err)
	}
}

// optoutAdd stores an opt-out through the store the public form uses and
// pins its arrival time, so "newest first" is not decided by the clock.
func optoutAdd(t *testing.T, database *db.DB, r dsr.Request, createdAt string) int64 {
	t.Helper()
	r.Kind = dsr.KindOptOut
	id, err := dsr.NewStore(database).Add(context.Background(), r)
	if err != nil {
		t.Fatalf("Add(%q): %v", r.Name, err)
	}
	if _, err := database.Write.Exec(`UPDATE privacy_requests SET created_at = $1 WHERE id = $2`, createdAt, id); err != nil {
		t.Fatalf("pin created_at: %v", err)
	}
	return id
}

// optoutGet renders the list of one website.
func optoutGet(t *testing.T, h *Handler, sm *scs.SessionManager, websiteID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/admin/websites/"+websiteID+"/widersprueche", nil)
	req.SetPathValue("id", websiteID)
	return serve(t, h, sm, h.HandleOptOutList, req)
}

// optoutRow is the table row that carries a text, or "".
func optoutRow(body, text string) string {
	for _, row := range strings.Split(body, "<tr") {
		if strings.Contains(row, text) {
			return row
		}
	}
	return ""
}

// optoutSays is a catalogue sentence as the screen prints it in the language
// a request without a session gets.
func optoutSays(key string) string {
	return template.HTMLEscapeString(i18n.T(i18n.Source, key))
}

func TestTheOptOutListShowsThisWebsitesRequests(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	optoutFrameworks(t, database, ws.ID, "eu,us")
	other, err := h.domains.CreateWebsite(context.Background(), "Andere", "")
	if err != nil {
		t.Fatal(err)
	}
	optoutFrameworks(t, database, other.ID, "us")

	optoutAdd(t, database, dsr.Request{WebsiteID: ws.ID, Locale: "fr", Name: "Ada Älter",
		Email: "ada@example.com", Note: "Bitte <script>alert(1)</script> nichts verkaufen", GPC: true},
		"2026-01-10T09:00:00Z")
	handled := optoutAdd(t, database, dsr.Request{WebsiteID: ws.ID, Locale: "de", Name: "Bea Neuer",
		Email: "bea@example.com", Note: "Zweite"}, "2026-02-20T09:30:00Z")
	if _, err := database.Write.Exec(`UPDATE privacy_requests SET handled_at = '2026-03-04T10:00:00Z' WHERE id = $1`, handled); err != nil {
		t.Fatal(err)
	}
	optoutAdd(t, database, dsr.Request{WebsiteID: other.ID, Locale: "en", Name: "Fremde Person",
		Email: "fremd@example.com"}, "2026-02-21T09:00:00Z")

	rec := optoutGet(t, h, sm, strconv.FormatInt(ws.ID, 10))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	for _, want := range []string{"Ada Älter", "ada@example.com", "Bea Neuer", "bea@example.com", "Zweite"} {
		if !strings.Contains(body, want) {
			t.Errorf("the list lacks %q", want)
		}
	}
	if strings.Contains(body, "Fremde Person") || strings.Contains(body, "fremd@example.com") {
		t.Error("the list shows another website's opt-out")
	}
	if a, b := strings.Index(body, "Ada Älter"), strings.Index(body, "Bea Neuer"); a < 0 || b < 0 || b > a {
		t.Errorf("not newest first: Bea (20.02.) at %d, Ada (10.01.) at %d", b, a)
	}

	// T-33.1-29: a visitor's text is escaped, never markup.
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("the note reached the screen unescaped")
	}
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("the note is missing in its escaped form")
	}

	ada := optoutRow(body, "Ada Älter")
	if !strings.Contains(ada, optoutSays("Signal sent")) {
		t.Errorf("the GPC request carries no GPC mark:\n%s", ada)
	}
	if !strings.Contains(ada, "10.01.2026") {
		t.Errorf("the arrival date is missing:\n%s", ada)
	}
	if !strings.Contains(ada, optoutSays("French")) {
		t.Errorf("the page language is missing:\n%s", ada)
	}
	bea := optoutRow(body, "Bea Neuer")
	if strings.Contains(bea, optoutSays("Signal sent")) {
		t.Errorf("a request without GPC carries the GPC mark:\n%s", bea)
	}
	if !strings.Contains(bea, "04.03.2026") {
		t.Errorf("the handled date is missing:\n%s", bea)
	}
}

// A visitor's address becomes a mailto: link only when it is one bare
// address. A row from before the form checked addresses, carrying header
// fields after a "?", is shown as text: clicked to answer the request it
// would have sent a copy wherever the stranger said (WR-02).
func TestOnlyAValidAddressBecomesALink(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	optoutFrameworks(t, database, ws.ID, "eu,us")
	optoutAdd(t, database, dsr.Request{WebsiteID: ws.ID, Name: "Ada", Email: "ada@example.com"}, "2026-01-10T09:00:00Z")
	if _, err := database.Write.Exec(
		`INSERT INTO privacy_requests (website_id, kind, name, email) VALUES ($1, 'optout', 'Mallory', $2)`,
		ws.ID, "op@example.com?bcc=thief@evil.example&subject=Re"); err != nil {
		t.Fatal(err)
	}
	body := optoutGet(t, h, sm, strconv.FormatInt(ws.ID, 10)).Body.String()

	if ada := optoutRow(body, "Ada"); !strings.Contains(ada, `href="mailto:ada@example.com"`) {
		t.Errorf("a valid address is not a link:\n%s", ada)
	}
	mallory := optoutRow(body, "Mallory")
	if strings.Contains(mallory, "href=") || strings.Contains(mallory, "mailto:") {
		t.Errorf("an address with header fields became a link:\n%s", mallory)
	}
	if !strings.Contains(mallory, "op@example.com?bcc=thief@evil.example&amp;subject=Re") {
		t.Errorf("the address is not shown as text:\n%s", mallory)
	}
}

func TestTheOptOutListOfAnUnknownWebsiteIs404(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	for _, id := range []string{"9999", "abc"} {
		if rec := optoutGet(t, h, sm, id); rec.Code != http.StatusNotFound {
			t.Errorf("website %q: status %d, want 404", id, rec.Code)
		}
	}
}

func TestTheNavShowsOptOutsOnlyWithUS(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	link := `href="` + optoutList(ws.ID) + `"`

	settings := func(t *testing.T) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/websites/%d", ws.ID), nil)
		req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
		rec := httptest.NewRecorder()
		var herr error
		// The settings list's website entries come from the middleware.
		sm.LoadAndSave(web.WithNav(sm, func(ctx context.Context) ([]domain.Website, error) {
			fresh, err := h.domains.GetWebsite(ctx, ws.ID)
			if err != nil {
				return nil, err
			}
			return []domain.Website{*fresh}, nil
		}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			herr = h.HandleWebsiteEdit(w, r)
		}))).ServeHTTP(rec, req)
		if herr != nil {
			t.Fatal(herr)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		return rec.Body.String()
	}

	optoutFrameworks(t, database, ws.ID, "eu,us")
	if body := settings(t); !strings.Contains(body, link) {
		t.Error("a website with US has no Opt-outs entry in its settings")
	}
	optoutFrameworks(t, database, ws.ID, "eu,ch")
	if body := settings(t); strings.Contains(body, link) {
		t.Error("a website without US shows the Opt-outs entry")
	}
}

func TestTheOptOutListSaysTheRecordIsKept(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	optoutFrameworks(t, database, ws.ID, "eu,us")

	body := optoutGet(t, h, sm, strconv.FormatInt(ws.ID, 10)).Body.String()
	for _, key := range []string{
		"Requests sent through the “Do Not Sell or Share” page of this website. The website sells and shares no personal information; record that you answered by marking a request handled.",
		"Requests are not deleted here: US law asks you to keep this record for at least 24 months.",
		"No opt-out requests yet.",
	} {
		if !strings.Contains(body, optoutSays(key)) {
			t.Errorf("the screen does not say %q", key)
		}
		if i18n.T("de", key) == key {
			t.Errorf("the German catalogue lacks %q", key)
		}
	}
	notUS := "US law is not chosen for this website, so it offers no opt-out form. Requests it received earlier stay listed here."
	if strings.Contains(body, optoutSays(notUS)) {
		t.Error("a website with US is told it has none")
	}

	// Taking US away hides the form, not the record.
	optoutAdd(t, database, dsr.Request{WebsiteID: ws.ID, Locale: "de", Name: "Frühere Anfrage"}, "2026-01-10T09:00:00Z")
	optoutFrameworks(t, database, ws.ID, "eu,ch")
	rec := optoutGet(t, h, sm, strconv.FormatInt(ws.ID, 10))
	if rec.Code != http.StatusOK {
		t.Fatalf("without US: status %d, want 200 — the records stay listed", rec.Code)
	}
	body = rec.Body.String()
	if !strings.Contains(body, "Frühere Anfrage") {
		t.Error("without US the earlier request is no longer listed")
	}
	if !strings.Contains(body, optoutSays(notUS)) {
		t.Error("without US the screen does not say why there is no form")
	}
}

func optoutHandled(websiteID, requestID int64) string {
	return fmt.Sprintf("/admin/websites/%d/widersprueche/%d/erledigt", websiteID, requestID)
}

// optoutMark posts "handled" as a user and hands back the answer and both
// flashes.
func optoutMark(t *testing.T, h *Handler, sm *scs.SessionManager, userID, websiteID int64, requestID string, hx bool) (*httptest.ResponseRecorder, string, string) {
	t.Helper()
	req := postForm(fmt.Sprintf("/admin/websites/%d/widersprueche/%s/erledigt", websiteID, requestID), nil,
		websiteRoute(websiteID, "requestID", requestID))
	if hx {
		req = htmx(req)
	}
	return screenValueServe(t, h, sm, userID, h.HandleOptOutHandled, req)
}

// optoutStored reads one request back, nil when it is not on that website.
func optoutStored(t *testing.T, database *db.DB, websiteID, id int64) *dsr.Request {
	t.Helper()
	r, err := dsr.NewStore(database).Get(context.Background(), websiteID, id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestMarkingAnOptOutHandledStoresTheDateOnce(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	optoutFrameworks(t, database, ws.ID, "eu,us")
	// The order book's rights: somebody without the publishing right records
	// the answer too, because marking publishes nothing.
	editor := editorWithoutPublishing(t, database, ws.ID)
	open := optoutAdd(t, database, dsr.Request{WebsiteID: ws.ID, Locale: "de", Name: "Offen Bleibt"}, "2026-01-10T09:00:00Z")
	done := optoutAdd(t, database, dsr.Request{WebsiteID: ws.ID, Locale: "de", Name: "Wird Erledigt"}, "2026-01-11T09:00:00Z")
	list := optoutList(ws.ID)

	rec, bad, good := optoutMark(t, h, sm, editor, ws.ID, strconv.FormatInt(done, 10), false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != list {
		t.Fatalf("status %d to %q; want 303 to %q", rec.Code, rec.Header().Get("Location"), list)
	}
	if bad != "" || good != "Opt-out marked as handled" {
		t.Errorf("flashes: error %q, success %q; want the success sentence alone", bad, good)
	}
	first := optoutStored(t, database, ws.ID, done)
	if first == nil || first.HandledAt == nil {
		t.Fatalf("handled_at was not stored: %+v", first)
	}
	if got := optoutStored(t, database, ws.ID, open); got.HandledAt != nil {
		t.Errorf("the other request was marked too: %v", got.HandledAt)
	}

	// A second answer does not move the date of the first.
	if _, err := database.Write.Exec(`UPDATE privacy_requests SET handled_at = '2026-01-02T03:04:05Z' WHERE id = $1`, done); err != nil {
		t.Fatal(err)
	}
	rec, _, _ = optoutMark(t, h, sm, editor, ws.ID, strconv.FormatInt(done, 10), true)
	if rec.Header().Get("HX-Redirect") != list {
		t.Errorf("htmx: HX-Redirect %q, want %q", rec.Header().Get("HX-Redirect"), list)
	}
	if got := optoutStored(t, database, ws.ID, done); got.HandledAt == nil || got.HandledAt.Format("2006-01-02T15:04:05Z") != "2026-01-02T03:04:05Z" {
		t.Errorf("a second mark changed the date: %v", got.HandledAt)
	}

	body := optoutGet(t, h, sm, strconv.FormatInt(ws.ID, 10)).Body.String()
	doneRow, openRow := optoutRow(body, "Wird Erledigt"), optoutRow(body, "Offen Bleibt")
	if !strings.Contains(doneRow, "02.01.2026") {
		t.Errorf("the handled row does not show its date:\n%s", doneRow)
	}
	if strings.Contains(doneRow, "/erledigt") {
		t.Errorf("the handled row still offers the button:\n%s", doneRow)
	}
	action := optoutHandled(ws.ID, open)
	for _, want := range []string{
		`method="POST" action="` + action + `"`,
		`hx-post="` + action + `"`,
		`hx-disabled-elt="this"`,
		`name="gorilla.csrf.Token"`,
		optoutSays("Mark as handled"),
	} {
		if !strings.Contains(openRow, want) {
			t.Errorf("the open row lacks %s:\n%s", want, openRow)
		}
	}
	for _, key := range []string{"Opt-out marked as handled", "Mark as handled"} {
		if i18n.T("de", key) == key {
			t.Errorf("the German catalogue lacks %q", key)
		}
	}
}

func TestAnotherWebsitesOptOutCannotBeMarked(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	other, err := h.domains.CreateWebsite(context.Background(), "Andere", "")
	if err != nil {
		t.Fatal(err)
	}
	foreign := optoutAdd(t, database, dsr.Request{WebsiteID: other.ID, Locale: "en", Name: "Fremd"}, "2026-01-10T09:00:00Z")

	for _, id := range []string{strconv.FormatInt(foreign, 10), "abc", "0"} {
		rec, _, good := optoutMark(t, h, sm, 1, ws.ID, id, false)
		if rec.Code != http.StatusNotFound {
			t.Errorf("request %q through website %d: status %d, want 404", id, ws.ID, rec.Code)
		}
		if good != "" {
			t.Errorf("request %q: a success was reported", id)
		}
	}
	if got := optoutStored(t, database, other.ID, foreign); got == nil || got.HandledAt != nil {
		t.Errorf("the other website's request was changed: %+v", got)
	}
	// An unknown website is 404 as well.
	if rec, _, _ := optoutMark(t, h, sm, 1, 9999, strconv.FormatInt(foreign, 10), false); rec.Code != http.StatusNotFound {
		t.Errorf("unknown website: status %d, want 404", rec.Code)
	}
}

func TestTheOptOutScreenCannotDelete(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	optoutFrameworks(t, database, ws.ID, "eu,us")
	optoutAdd(t, database, dsr.Request{WebsiteID: ws.ID, Locale: "de", Name: "Offen"}, "2026-01-10T09:00:00Z")
	done := optoutAdd(t, database, dsr.Request{WebsiteID: ws.ID, Locale: "de", Name: "Erledigt"}, "2026-01-11T09:00:00Z")
	if _, err := database.Write.Exec(`UPDATE privacy_requests SET handled_at = '2026-01-12T09:00:00Z' WHERE id = $1`, done); err != nil {
		t.Fatal(err)
	}

	// The content alone, as htmx asks for it: what this screen adds, without
	// the layout around it.
	req := htmx(httptest.NewRequest(http.MethodGet, optoutList(ws.ID), nil))
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	body := serve(t, h, sm, h.HandleOptOutList, req).Body.String()
	if !strings.Contains(body, "Offen") || !strings.Contains(body, "Erledigt") {
		t.Fatal("the fragment does not list the requests; the guard below would prove nothing")
	}

	if strings.Contains(body, "hx-confirm") {
		t.Error("the opt-out screen carries an hx-confirm (D-14: the count stays 22)")
	}
	for _, tag := range postFormsTo(body, "/") {
		if strings.Contains(tag, "delete") || strings.Contains(tag, "loeschen") {
			t.Errorf("a form deletes: %s", tag)
		}
	}
	for _, word := range []string{"Delete", "Remove", "Delete permanently"} {
		if strings.Contains(body, ">"+optoutSays(word)+"<") {
			t.Errorf("a control is labelled %q", word)
		}
	}
	if strings.Contains(body, "btn--danger") {
		t.Error("the screen carries a danger button")
	}
}

// D-16: a visitor's name, address and note stay in the admin. No assistant
// tool lists, reads or marks an opt-out.
func TestOptOutsAreNotOnMCP(t *testing.T) {
	h, _, _, _ := newTestAdmin(t)
	tools := ai.Tools(ai.Deps{
		Domains: h.domains, Pages: h.pages, Media: h.mediaStore, Fields: h.fields, Ops: h,
		Limits: ai.Limits{DataDir: h.cfg.DataDir},
	})
	if len(tools) < 10 {
		t.Fatalf("%d tools; the registry was not built, the guard would prove nothing", len(tools))
	}
	for _, tool := range tools {
		name := strings.ToLower(tool.Name)
		for _, bad := range []string{"opt", "widerspruch", "request", "dsr", "privacy"} {
			if strings.Contains(name, bad) {
				t.Errorf("tool %q names %q: opt-outs are not exposed over MCP (D-16)", tool.Name, bad)
			}
		}
		desc := strings.ToLower(tool.Description)
		for _, bad := range []string{"opt-out", "optout", "widerspruch", "privacy request", "privacy_requests"} {
			if strings.Contains(desc, bad) {
				t.Errorf("tool %q describes %q: opt-outs are not exposed over MCP (D-16)", tool.Name, bad)
			}
		}
	}
}
