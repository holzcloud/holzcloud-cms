package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// The Requests screen under Settings → Privacy (Phase 33.2, plan 03). Every
// helper of this file starts with privacyRequests.

func privacyRequestsScreen(websiteID int64) string {
	return "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/datenschutz/anfragen"
}

// privacyRequestsAdd stores a data request through the store the public form
// uses and pins its arrival time.
func privacyRequestsAdd(t *testing.T, database *db.DB, r dsr.Request, createdAt string) int64 {
	t.Helper()
	id, err := dsr.NewStore(database).Add(context.Background(), r)
	if err != nil {
		t.Fatalf("Add(%+v): %v", r, err)
	}
	if _, err := database.Write.Exec(`UPDATE privacy_requests SET created_at = $1 WHERE id = $2`, createdAt, id); err != nil {
		t.Fatalf("pin created_at: %v", err)
	}
	return id
}

// privacyRequestsGet renders the screen of one website.
func privacyRequestsGet(t *testing.T, h *Handler, sm *scs.SessionManager, websiteID int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, privacyRequestsScreen(websiteID), nil)
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	return serve(t, h, sm, h.HandlePrivacyRequests, req)
}

func TestTheRequestsScreenListsDataRequests(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	if _, err := database.Write.Exec(`UPDATE websites SET timezone = 'Europe/Zurich' WHERE id = $1`, ws.ID); err != nil {
		t.Fatal(err)
	}
	other, err := h.domains.CreateWebsite(context.Background(), "Andere", "")
	if err != nil {
		t.Fatal(err)
	}
	older := privacyRequestsAdd(t, database, dsr.Request{WebsiteID: ws.ID, Kind: dsr.KindErasure, Name: "Erste Anfrage", Email: "ada@example.com"}, "2026-01-10T09:00:00Z")
	newer := privacyRequestsAdd(t, database, dsr.Request{WebsiteID: ws.ID, Kind: dsr.KindAccess, Name: "Zweite Anfrage", Email: "bea@example.com"}, "2026-01-11T09:00:00Z")
	privacyRequestsAdd(t, database, dsr.Request{WebsiteID: ws.ID, Kind: dsr.KindOptOut, Name: "Ein Widerspruch"}, "2026-01-12T09:00:00Z")
	privacyRequestsAdd(t, database, dsr.Request{WebsiteID: other.ID, Kind: dsr.KindAccess, Name: "Fremde Anfrage", Email: "cleo@example.com"}, "2026-01-12T09:00:00Z")

	rec := privacyRequestsGet(t, h, sm, ws.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, absent := range []string{"Ein Widerspruch", "Fremde Anfrage", "cleo@example.com"} {
		if strings.Contains(body, absent) {
			t.Errorf("the screen shows %q", absent)
		}
	}
	first, second := strings.Index(body, "Zweite Anfrage"), strings.Index(body, "Erste Anfrage")
	if first < 0 || second < 0 || first > second {
		t.Fatalf("the two requests are not listed newest first (%d, %d)", first, second)
	}
	row := optoutRow(body, "Zweite Anfrage")
	if !strings.Contains(row, "11.01.2026 10:00 CET") {
		t.Errorf("09:00 UTC on a Swiss website is not shown as 10:00 CET:\n%s", row)
	}
	if !strings.Contains(row, `href="mailto:bea@example.com"`) {
		t.Errorf("the address is not a mailto link:\n%s", row)
	}
	if !strings.Contains(row, optoutSays("Access")) {
		t.Errorf("the row does not name the kind:\n%s", row)
	}
	search := privacyScreen(ws.ID) + "?email=bea%40example.com&amp;anfrage=" + strconv.FormatInt(newer, 10)
	if !strings.Contains(row, `href="`+search+`"`) {
		t.Errorf("the row does not link the person search %s:\n%s", search, row)
	}
	if !strings.Contains(optoutRow(body, "Erste Anfrage"), optoutSays("Erasure")) {
		t.Errorf("the erasure request does not name its kind")
	}
	if !strings.Contains(optoutRow(body, "Erste Anfrage"), "anfrage="+strconv.FormatInt(older, 10)+`"`) {
		t.Errorf("the erasure request does not link the person search with its own id")
	}
	if strings.Contains(body, "hx-confirm") {
		t.Error("the screen carries a confirmation")
	}
}

// The screen names the public page the requests come from, in the words the
// legal pages card uses for it, and leads to where that page is made.
func TestTheRequestsScreenNamesWhereRequestsComeFrom(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	body := privacyRequestsGet(t, h, sm, ws.ID).Body.String()
	for _, key := range []string{
		"Request about my data",
		"Visitors send requests about their personal data through the form on this page: access, correction, erasure.",
		"Create or open the page on the settings screen",
	} {
		if !strings.Contains(body, optoutSays(key)) {
			t.Errorf("the screen lacks %q", key)
		}
		if i18n.T("de", key) == key {
			t.Errorf("the German catalogue lacks %q", key)
		}
	}
	if want := `href="/admin/websites/` + strconv.FormatInt(ws.ID, 10) + `#legal-pages"`; !strings.Contains(body, want) {
		t.Errorf("the screen does not link the legal pages card: want %s", want)
	}
}

// privacyRequestsAgo is a stored time d before now, in the store's layout.
func privacyRequestsAgo(d time.Duration) string {
	return time.Now().Add(-d).UTC().Format("2006-01-02T15:04:05Z")
}

// privacyRequestsLog is the privacy entries of the activity log: action,
// entity id and metadata, oldest first.
func privacyRequestsLog(t *testing.T, database *db.DB) [][3]string {
	t.Helper()
	rows, err := database.Read.Query(
		`SELECT action, entity_id, metadata FROM activity_log WHERE action LIKE 'privacy.%' ORDER BY id`)
	if err != nil {
		t.Fatalf("read activity_log: %v", err)
	}
	defer rows.Close()
	var out [][3]string
	for rows.Next() {
		var action, meta string
		var id int64
		if err := rows.Scan(&action, &id, &meta); err != nil {
			t.Fatal(err)
		}
		out = append(out, [3]string{action, strconv.FormatInt(id, 10), meta})
	}
	return out
}

// privacyRequestsPost posts a form to one of the screen's actions as a user
// and hands back the answer and both flashes.
func privacyRequestsPost(t *testing.T, h *Handler, sm *scs.SessionManager, userID int64, fn func(http.ResponseWriter, *http.Request) error,
	target string, form url.Values, path map[string]string, hx bool) (*httptest.ResponseRecorder, string, string) {
	t.Helper()
	req := postForm(target, form, path)
	if hx {
		req = htmx(req)
	}
	return screenValueServe(t, h, sm, userID, fn, req)
}

func TestTheRequestsScreenShowsWhenEachIsDue(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	optoutFrameworks(t, database, ws.ID, "eu,ch")
	privacyRequestsAdd(t, database, dsr.Request{WebsiteID: ws.ID, Kind: dsr.KindAccess, Name: "Zehn Tage", Email: "ada@example.com"},
		privacyRequestsAgo(10*24*time.Hour))
	// A fixed day in a month of 31 days, so that the Swiss 30 days end a day
	// before the EU's month: 15 October, due on 14 November (CH) and not on
	// 15 November (EU). Ten days ago could fall in a month of 30, where the
	// two coincide.
	privacyRequestsAdd(t, database, dsr.Request{WebsiteID: ws.ID, Kind: dsr.KindAccess, Name: "Mitte Oktober", Email: "dora@example.com"},
		"2026-10-15T10:00:00Z")
	privacyRequestsAdd(t, database, dsr.Request{WebsiteID: ws.ID, Kind: dsr.KindErasure, Name: "Vierzig Tage", Email: "bea@example.com"},
		privacyRequestsAgo(40*24*time.Hour))
	answered := privacyRequestsAdd(t, database, dsr.Request{WebsiteID: ws.ID, Kind: dsr.KindErasure, Name: "Beantwortet", Email: "cleo@example.com"},
		privacyRequestsAgo(40*24*time.Hour))
	if err := dsr.NewStore(database).Answer(context.Background(), ws.ID, answered, dsr.OutcomeGranted, ""); err != nil {
		t.Fatal(err)
	}

	body := privacyRequestsGet(t, h, sm, ws.ID).Body.String()
	if row := optoutRow(body, "Mitte Oktober"); !strings.Contains(row, "14.11.2026") || !strings.Contains(row, "(CH)") || strings.Contains(row, "15.11.2026") {
		t.Errorf("the row does not show the Swiss date 14.11.2026, the earlier one, with its framework:\n%s", row)
	}
	row := optoutRow(body, "Zehn Tage")
	if row == "" {
		t.Fatalf("the request of ten days is not listed:\n%s", body)
	}
	if strings.Contains(row, optoutSays("Overdue")) {
		t.Errorf("a request of ten days is marked overdue:\n%s", row)
	}
	if late := optoutRow(body, "Vierzig Tage"); !strings.Contains(late, optoutSays("Overdue")) {
		t.Errorf("an open request of forty days is not marked overdue:\n%s", late)
	}
	if done := optoutRow(body, "Beantwortet"); strings.Contains(done, optoutSays("Overdue")) {
		t.Errorf("an answered request is marked overdue:\n%s", done)
	}
	if strings.Contains(body, optoutSays("No legal framework is chosen for this website, so no deadline is shown.")) {
		t.Error("a website with frameworks says none is chosen")
	}

	optoutFrameworks(t, database, ws.ID, "")
	none := privacyRequestsGet(t, h, sm, ws.ID).Body.String()
	if !strings.Contains(none, optoutSays("No legal framework is chosen for this website, so no deadline is shown.")) {
		t.Error("a website without a framework does not say why no deadline is shown")
	}
	if strings.Contains(optoutRow(none, "Mitte Oktober"), "14.11.2026") || strings.Contains(none, optoutSays("Overdue")) {
		t.Error("a website without a framework still shows a deadline")
	}
}

func TestTheAnswerIsRecordedOnce(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	h.SetActivityStore(activity.NewStore(database))
	// The opt-out list's rights: somebody without the publishing right
	// records too, because recording publishes nothing.
	editor := editorWithoutPublishing(t, database, ws.ID)
	optoutFrameworks(t, database, ws.ID, "eu")
	id := privacyRequestsAdd(t, database, dsr.Request{WebsiteID: ws.ID, Kind: dsr.KindAccess, Name: "Ada Lovelace", Email: "ada@example.com"},
		privacyRequestsAgo(48*time.Hour))
	list := privacyRequestsScreen(ws.ID)
	target := func(id int64) string { return list + "/" + strconv.FormatInt(id, 10) + "/antwort" }
	path := func(id string) map[string]string { return websiteRoute(ws.ID, "requestID", id) }
	idStr := strconv.FormatInt(id, 10)

	// A refusal or a partial answer without its basis is sent back.
	for _, outcome := range []string{dsr.OutcomeRefused, dsr.OutcomePartly} {
		rec, _, good := privacyRequestsPost(t, h, sm, editor, h.HandlePrivacyRequestAnswer, target(id),
			url.Values{"outcome": {outcome}, "basis": {"  "}}, path(idStr), false)
		if rec.Code != http.StatusUnprocessableEntity || good != "" {
			t.Errorf("%s without a basis: status %d, success %q; want 422 and none", outcome, rec.Code, good)
		}
		if !strings.Contains(rec.Body.String(), optoutSays("Please give the basis for the refusal.")) {
			t.Errorf("%s without a basis: the screen does not say what is missing", outcome)
		}
	}
	rec, _, _ := privacyRequestsPost(t, h, sm, editor, h.HandlePrivacyRequestAnswer, target(id),
		url.Values{"outcome": {"maybe"}}, path(idStr), false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("outcome maybe: status %d; want 422", rec.Code)
	}
	if got := optoutStored(t, database, ws.ID, id); got.Outcome != "" || got.HandledAt != nil {
		t.Fatalf("a refused answer was stored: %+v", got)
	}

	rec, bad, good := privacyRequestsPost(t, h, sm, editor, h.HandlePrivacyRequestAnswer, target(id),
		url.Values{"outcome": {dsr.OutcomePartly}, "basis": {"Order data sent; the invoice is kept (tax law)."}}, path(idStr), false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != list {
		t.Fatalf("status %d to %q; want 303 to %q", rec.Code, rec.Header().Get("Location"), list)
	}
	if bad != "" || good != "Answer recorded" {
		t.Errorf("flashes: error %q, success %q; want the success sentence alone", bad, good)
	}
	first := optoutStored(t, database, ws.ID, id)
	if first.Outcome != dsr.OutcomePartly || first.HandledAt == nil {
		t.Fatalf("after the answer: %+v", first)
	}
	row := optoutRow(privacyRequestsGet(t, h, sm, ws.ID).Body.String(), "Ada Lovelace")
	for _, want := range []string{optoutSays("Partly granted"), "Order data sent; the invoice is kept (tax law).", first.HandledAt.In(tmpl.LoadLocation(ws.TimeZone)).Format("02.01.2006")} {
		if !strings.Contains(row, want) {
			t.Errorf("the answered row lacks %q:\n%s", want, row)
		}
	}
	if strings.Contains(row, "/antwort") {
		t.Errorf("the answered row still offers the answer form:\n%s", row)
	}
	log := privacyRequestsLog(t, database)
	if len(log) != 1 || log[0][0] != "privacy.answer" || log[0][1] != idStr ||
		!strings.Contains(log[0][2], `"outcome":"partly"`) || !strings.Contains(log[0][2], `"request":`+idStr) {
		t.Errorf("activity = %v; want one privacy.answer with the request and the outcome", log)
	}
	if strings.Contains(log[0][2], "ada@example.com") || strings.Contains(log[0][2], "Lovelace") {
		t.Errorf("the activity entry carries the person: %s", log[0][2])
	}

	// A second answer changes nothing, and says so; htmx gets its redirect.
	rec, bad, good = privacyRequestsPost(t, h, sm, editor, h.HandlePrivacyRequestAnswer, target(id),
		url.Values{"outcome": {dsr.OutcomeGranted}}, path(idStr), true)
	if rec.Header().Get("HX-Redirect") != list || good != "" || bad != "This request was already answered." {
		t.Errorf("a second answer: HX-Redirect %q, error %q, success %q", rec.Header().Get("HX-Redirect"), bad, good)
	}
	if again := optoutStored(t, database, ws.ID, id); again.Outcome != dsr.OutcomePartly || !again.HandledAt.Equal(*first.HandledAt) {
		t.Errorf("a second answer changed the record: %+v", again)
	}
	if n := len(privacyRequestsLog(t, database)); n != 1 {
		t.Errorf("%d activity entries; want still 1", n)
	}

	// Another website's request, an opt-out, and nonsense are 404.
	other, err := h.domains.CreateWebsite(context.Background(), "Andere", "")
	if err != nil {
		t.Fatal(err)
	}
	foreign := privacyRequestsAdd(t, database, dsr.Request{WebsiteID: other.ID, Kind: dsr.KindAccess, Email: "bea@example.com"}, privacyRequestsAgo(time.Hour))
	optout := privacyRequestsAdd(t, database, dsr.Request{WebsiteID: ws.ID, Kind: dsr.KindOptOut, Name: "Cleo"}, privacyRequestsAgo(time.Hour))
	for _, rid := range []string{strconv.FormatInt(foreign, 10), strconv.FormatInt(optout, 10), "abc"} {
		rec, _, good := privacyRequestsPost(t, h, sm, editor, h.HandlePrivacyRequestAnswer, list+"/"+rid+"/antwort",
			url.Values{"outcome": {dsr.OutcomeGranted}}, path(rid), false)
		if rec.Code != http.StatusNotFound || good != "" {
			t.Errorf("request %s: status %d, success %q; want 404", rid, rec.Code, good)
		}
	}
	if got := optoutStored(t, database, other.ID, foreign); got.Outcome != "" {
		t.Errorf("the other website's request was answered: %+v", got)
	}
}

func TestARequestByMailIsEnteredByHand(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	h.SetActivityStore(activity.NewStore(database))
	// The opt-out list's rights: somebody without the publishing right
	// records too, because recording publishes nothing.
	editor := editorWithoutPublishing(t, database, ws.ID)
	if _, err := database.Write.Exec(`UPDATE websites SET timezone = 'Europe/Zurich' WHERE id = $1`, ws.ID); err != nil {
		t.Fatal(err)
	}
	zurich, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		t.Skipf("no time zone data: %v", err)
	}
	list := privacyRequestsScreen(ws.ID)
	target := list + "/neu"
	path := websiteRoute(ws.ID)
	day := time.Now().In(zurich).AddDate(0, 0, -3).Format("2006-01-02")
	form := func(change func(url.Values)) url.Values {
		f := url.Values{"kind": {dsr.KindAccess}, "manner": {dsr.MannerEmail}, "received": {day},
			"name": {"Per Brief"}, "email": {"brief@example.com"}, "note": {"came by mail"}}
		if change != nil {
			change(f)
		}
		return f
	}

	for _, tc := range []struct {
		name   string
		change func(url.Values)
		say    string
	}{
		{"not an address", func(f url.Values) { f.Set("email", "brief@example.com?bcc=x") }, "Please give the e-mail address the request is about."},
		{"no address", func(f url.Values) { f.Set("email", "") }, "Please give the e-mail address the request is about."},
		{"no date", func(f url.Values) { f.Set("received", "") }, "Please give the day the request arrived."},
		{"a date ahead", func(f url.Values) { f.Set("received", time.Now().AddDate(0, 0, 2).Format("2006-01-02")) }, "Please give the day the request arrived."},
		{"the form as manner", func(f url.Values) { f.Set("manner", dsr.MannerForm) }, "Please choose how the request arrived."},
		{"an opt-out", func(f url.Values) { f.Set("kind", dsr.KindOptOut) }, "Please choose what the person asks for."},
	} {
		rec, _, good := privacyRequestsPost(t, h, sm, editor, h.HandlePrivacyRequestNew, target, form(tc.change), path, false)
		if rec.Code != http.StatusUnprocessableEntity || good != "" {
			t.Errorf("%s: status %d, success %q; want 422", tc.name, rec.Code, good)
		}
		if !strings.Contains(rec.Body.String(), optoutSays(tc.say)) {
			t.Errorf("%s: the screen does not say %q", tc.name, tc.say)
		}
		if !strings.Contains(rec.Body.String(), "<details class=\"privacy-entry\" open") {
			t.Errorf("%s: the entry form is closed after a refusal", tc.name)
		}
	}
	if l, _ := dsr.NewStore(database).ListData(context.Background(), ws.ID); len(l) != 0 {
		t.Fatalf("%d requests stored by refused entries", len(l))
	}

	rec, _, good := privacyRequestsPost(t, h, sm, editor, h.HandlePrivacyRequestNew, target, form(nil), path, false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != list || good != "Request entered" {
		t.Fatalf("status %d to %q, success %q", rec.Code, rec.Header().Get("Location"), good)
	}
	stored, _ := dsr.NewStore(database).ListData(context.Background(), ws.ID)
	if len(stored) != 1 || stored[0].Manner != dsr.MannerEmail || stored[0].Email != "brief@example.com" ||
		stored[0].CreatedAt.In(zurich).Format("2006-01-02") != day {
		t.Fatalf("stored %+v; want one access request by e-mail of %s", stored, day)
	}
	row := optoutRow(privacyRequestsGet(t, h, sm, ws.ID).Body.String(), "Per Brief")
	if !strings.Contains(row, optoutSays("E-mail")) {
		t.Errorf("the row does not name the manner:\n%s", row)
	}
	log := privacyRequestsLog(t, database)
	if len(log) != 1 || log[0][0] != "privacy.request_add" || log[0][1] != strconv.FormatInt(stored[0].ID, 10) {
		t.Errorf("activity = %v; want one privacy.request_add with the new id", log)
	}
	if strings.Contains(log[0][2], "brief@example.com") {
		t.Errorf("the activity entry carries the address: %s", log[0][2])
	}
}

// Recording an answer or a request removes nothing, so the screen asks no
// confirmation, and every form works without the script: a plain POST with
// its CSRF field.
func TestTheRequestsScreenHasNoConfirmation(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	privacyRequestsAdd(t, database, dsr.Request{WebsiteID: ws.ID, Kind: dsr.KindAccess, Email: "ada@example.com"}, privacyRequestsAgo(time.Hour))
	body := privacyRequestsGet(t, h, sm, ws.ID).Body.String()
	if strings.Contains(body, "hx-confirm") {
		t.Error("the screen carries an htmx confirmation")
	}
	forms := strings.Split(body, "<form")[1:]
	if n := strings.Count(body, `action="/admin/websites/`+strconv.FormatInt(ws.ID, 10)+`/datenschutz/anfragen`); n != 2 {
		t.Fatalf("%d forms of the screen; want the entry form and an answer form", n)
	}
	for _, f := range forms {
		f = f[:strings.Index(f, "</form>")]
		// The screen's own forms; the layout's (sign out) are not its.
		if !strings.Contains(f, "/datenschutz/anfragen") {
			continue
		}
		action := privacyRequestsBetween(f, `action="`, `"`)
		if action == "" || !strings.Contains(f, `method="POST"`) || !strings.Contains(f, `name="gorilla.csrf.Token"`) {
			t.Errorf("a form without a plain POST, an action or its CSRF field:\n%s", f)
		}
		if post := privacyRequestsBetween(f, `hx-post="`, `"`); post != "" && post != action {
			t.Errorf("hx-post %q differs from action %q", post, action)
		}
		if !strings.Contains(f, `hx-disabled-elt="this"`) {
			t.Errorf("a submit button without hx-disabled-elt:\n%q", f)
		}
	}
}

// privacyRequestsBetween is the text after open up to the next close, or "".
func privacyRequestsBetween(s, open, close string) string {
	i := strings.Index(s, open)
	if i < 0 {
		return ""
	}
	s = s[i+len(open):]
	j := strings.Index(s, close)
	if j < 0 {
		return ""
	}
	return s[:j]
}
