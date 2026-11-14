package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// The Retention screen (Phase 33.2, plan 04). Every helper of this file starts
// with retention, so the plans that run beside it in this package keep theirs
// apart.

// retentionAdmin is the test handler with the privacy service and the
// activity log wired the way main.go wires them.
func retentionAdmin(t *testing.T) (*Handler, *scs.SessionManager, *db.DB, *domain.Website, *dsr.Service) {
	t.Helper()
	h, sm, database, ws := newTestAdmin(t)
	s := dsr.NewService(database)
	h.SetDataRights(s)
	h.SetActivityStore(activity.NewStore(database))
	return h, sm, database, ws, s
}

func retentionAddress(websiteID int64) string {
	return "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/datenschutz/aufbewahrung"
}

// retentionSays is a sentence of the catalogue as the screen prints it in the
// source language.
func retentionSays(key string, args ...any) string {
	s := i18n.T(i18n.Source, key)
	if len(args) > 0 {
		s = fmt.Sprintf(s, args...)
	}
	return template.HTMLEscapeString(s)
}

// retentionGet renders the screen.
func retentionGet(t *testing.T, h *Handler, sm *scs.SessionManager, websiteID int64) string {
	t.Helper()
	rec := privacyGet(t, h, sm, h.HandlePrivacyRetention, retentionAddress(websiteID), websiteID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

// retentionInput is the opening tag of the form field of a kind.
func retentionInput(body, kind string) string {
	at := strings.Index(body, `name="`+kind+`"`)
	if at < 0 {
		return ""
	}
	start := strings.LastIndex(body[:at], "<input")
	end := strings.Index(body[at:], ">")
	if start < 0 || end < 0 {
		return ""
	}
	return body[start : at+end+1]
}

// retentionSection is the part of the page from the marker to the next
// marker of the same shape, for asserting what one row says.
func retentionSection(body, id string) string {
	at := strings.Index(body, `id="`+id+`"`)
	if at < 0 {
		return ""
	}
	rest := body[at:]
	if end := strings.Index(rest[1:], `id="retention-`); end >= 0 {
		return rest[:end+1]
	}
	return rest
}

func retentionStored(t *testing.T, database *db.DB, websiteID int64) map[string]int {
	t.Helper()
	rows, err := database.Read.Query(`SELECT kind, months FROM retention_periods WHERE website_id = $1`, websiteID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var kind string
		var months int
		if err := rows.Scan(&kind, &months); err != nil {
			t.Fatal(err)
		}
		out[kind] = months
	}
	return out
}

func TestTheRetentionScreenShowsPeriodsAndTheLastRun(t *testing.T) {
	h, sm, database, ws, s := retentionAdmin(t)

	body := retentionGet(t, h, sm, ws.ID)
	input := retentionInput(body, dsr.RetentionFormSubmission)
	if !strings.Contains(input, `value="12"`) {
		t.Errorf("the form submissions field does not show 12 months: %q", input)
	}
	row := retentionSection(body, "retention-"+dsr.RetentionFormSubmission)
	if !strings.Contains(row, retentionSays("Form submissions")) || !strings.Contains(row, retentionSays("Default")) {
		t.Errorf("the form submissions row does not name itself or its default:\n%s", row)
	}
	logs := retentionSection(body, "retention-logs")
	if !strings.Contains(logs, retentionSays("%d days, enforced by the system journal (deploy/journald-holzcloud.conf) for the whole server; it cannot be changed here.", 14)) {
		t.Errorf("the log line does not say 14 days and name the journald drop-in:\n%s", logs)
	}
	if strings.Contains(logs, "<input") {
		t.Errorf("the log period can be typed into:\n%s", logs)
	}
	if !strings.Contains(body, retentionSays("No run yet.")) {
		t.Error("before any run the screen does not say so")
	}
	if strings.Contains(body, "hx-confirm") {
		t.Error("the screen carries an htmx confirmation; the password is the step in front")
	}
	if !strings.Contains(body, `action="`+retentionAddress(ws.ID)+`"`) {
		t.Errorf("the form does not post to %s", retentionAddress(ws.ID))
	}

	// A run: one old message deleted.
	if _, err := database.Write.Exec(`INSERT INTO form_messages (website_id, name, email, subject, body, created_at)
		VALUES ($1, 'Ada', 'ada@example.com', 'Eier', 'Eier?', '2020-01-01T10:00:00Z')`, ws.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 14, 2, 30, 0, 0, time.UTC)
	if _, err := s.RunRetention(context.Background(), now); err != nil {
		t.Fatalf("RunRetention: %v", err)
	}
	body = retentionGet(t, h, sm, ws.ID)
	run := retentionSection(body, "retention-run")
	if run == "" {
		t.Fatalf("the screen has no last-run box:\n%s", body)
	}
	// Testseite runs on the default zone; the time is shown on the
	// website's clock, whichever that is.
	if !strings.Contains(run, retentionSays("Successful")) {
		t.Errorf("the run box does not say the run succeeded:\n%s", run)
	}
	if !strings.Contains(run, "14.11.2026") {
		t.Errorf("the run box does not show the day of the run:\n%s", run)
	}
	if !strings.Contains(run, retentionSays("%d deleted", 1)) {
		t.Errorf("the run box does not show this website's count:\n%s", run)
	}
	if strings.Contains(body, retentionSays("No run yet.")) {
		t.Error("after a run the screen still says there was none")
	}
}

func TestChangingAPeriodAsksForThePasswordAndThePublishingRight(t *testing.T) {
	h, sm, database, ws, _ := retentionAdmin(t)
	form := url.Values{dsr.RetentionFormSubmission: {"6"}}

	editor := editorWithoutPublishing(t, database, ws.ID)
	rec, flash := serveAs(t, h, sm, editor, h.HandlePrivacyRetention,
		postForm(retentionAddress(ws.ID), form, websiteRoute(ws.ID)))
	wantRefused(t, "retention save", rec, flash, retentionAddress(ws.ID))
	if got := retentionStored(t, database, ws.ID); len(got) != 0 {
		t.Errorf("an editor without the publishing right stored %v", got)
	}

	rec, flash, _ = albumFlash(t, h, sm, h.HandlePrivacyRetention,
		postForm(retentionAddress(ws.ID), form, websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != retentionAddress(ws.ID) {
		t.Fatalf("save: status %d to %q (%q), want 303 to the screen", rec.Code, rec.Header().Get("Location"), flash)
	}
	if got := retentionStored(t, database, ws.ID); got[dsr.RetentionFormSubmission] != 6 {
		t.Errorf("stored %v, want form_submission 6", got)
	}

	var n int
	var metadata string
	if err := database.Read.QueryRow(`SELECT COUNT(*), COALESCE(MAX(metadata), '') FROM activity_log
		WHERE action = $1 AND website_id = $2`, activity.ActionPrivacyPeriods, ws.ID).Scan(&n, &metadata); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("%d activity entries %s, want 1", n, activity.ActionPrivacyPeriods)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(metadata), &meta); err != nil {
		t.Fatalf("metadata %q: %v", metadata, err)
	}
	if meta[dsr.RetentionFormSubmission] != float64(6) {
		t.Errorf("metadata = %v, want form_submission: 6", meta)
	}

	// A period out of range is refused with its sentence, and nothing changes.
	bad := url.Values{dsr.RetentionFormSubmission: {"0"}}
	rec, _, _ = albumFlash(t, h, sm, h.HandlePrivacyRetention,
		postForm(retentionAddress(ws.ID), bad, websiteRoute(ws.ID)))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a period of 0: status %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), retentionSays("A period must be between 1 and 600 months.")) {
		t.Error("a period of 0 is refused without its sentence")
	}
	if got := retentionStored(t, database, ws.ID); got[dsr.RetentionFormSubmission] != 6 {
		t.Errorf("after a refused save stored %v, want form_submission still 6", got)
	}
}

func TestTheOrderPeriodNamesItsLaw(t *testing.T) {
	h, sm, database, ws, _ := retentionAdmin(t)
	for _, c := range []struct {
		country string
		years   int
		basis   string
	}{
		{"CH", 10, "Switzerland: business records and booking vouchers are kept for ten years (Art. 958f CO; Art. 70 VAT Act)."},
		{"AT", 10, "No retention period was researched for the country in this website's address, so the longest researched one, ten years, applies. Set a longer period if your law requires one."},
		{"DE", 8, "Germany: booking vouchers and copies of invoices are kept for eight years (§ 147 AO, § 257 HGB, § 14b UStG)."},
	} {
		if _, err := database.Write.Exec(`UPDATE websites SET country = $1 WHERE id = $2`, c.country, ws.ID); err != nil {
			t.Fatal(err)
		}
		body := retentionGet(t, h, sm, ws.ID)
		input := retentionInput(body, dsr.RetentionOrder)
		if !strings.Contains(input, `value="`+strconv.Itoa(c.years)+`"`) || !strings.Contains(input, `min="`+strconv.Itoa(c.years)+`"`) {
			t.Errorf("%s: the order field is not %d years with a minimum of %d: %q", c.country, c.years, c.years, input)
		}
		row := retentionSection(body, "retention-"+dsr.RetentionOrder)
		if !strings.Contains(row, retentionSays(c.basis)) {
			t.Errorf("%s: the order row does not name its law:\n%s", c.country, row)
		}
		if !strings.Contains(row, retentionSays("At least %d years", c.years)) {
			t.Errorf("%s: the order row does not say its minimum:\n%s", c.country, row)
		}
		if !strings.Contains(row, retentionSays("If your business year is not the calendar year, or a tax assessment is still open, set a longer period: the law's period may then end later.")) {
			t.Errorf("%s: the order row does not tell an operator with another business year to set a longer period:\n%s", c.country, row)
		}
	}

	request := retentionSection(retentionGet(t, h, sm, ws.ID), "retention-"+dsr.RetentionRequest)
	if !strings.Contains(request, retentionSays("At least 24 months: CCPA regs § 7101 ask for the record of a request for 24 months, and a website with younger requests cannot be deleted either.")) {
		t.Errorf("the request row does not say why it keeps 24 months:\n%s", request)
	}
	if input := retentionInput(request, dsr.RetentionRequest); !strings.Contains(input, `min="24"`) {
		t.Errorf("the request field allows less than 24 months: %q", input)
	}

	// Below the law is refused with the minimum, and nothing is stored.
	rec, _, _ := albumFlash(t, h, sm, h.HandlePrivacyRetention,
		postForm(retentionAddress(ws.ID), url.Values{dsr.RetentionOrder: {"7"}}, websiteRoute(ws.ID)))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("7 years on a DE website: status %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `id="period-order-error"`) {
		t.Error("7 years is refused without an error at the field")
	}
	if got := retentionStored(t, database, ws.ID); len(got) != 0 {
		t.Errorf("a refused period stored %v", got)
	}
}
