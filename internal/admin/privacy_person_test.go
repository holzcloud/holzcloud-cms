package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
)

// The Privacy screen (Phase 33.2, plan 01). Every helper of this file starts
// with privacy, so the plans that follow in this package keep theirs apart.

// privacyAdmin is the test handler with the search and the activity log
// wired the way main.go wires them.
func privacyAdmin(t *testing.T) (*Handler, *scs.SessionManager, *db.DB, *domain.Website) {
	t.Helper()
	h, sm, database, ws := newTestAdmin(t)
	h.SetDataRights(dsr.NewService(database))
	h.SetActivityStore(activity.NewStore(database))
	return h, sm, database, ws
}

// privacyOrder stores an order the way the shop writes one.
func privacyOrder(t *testing.T, database *db.DB, websiteID int64, number, email, name string) {
	t.Helper()
	res, err := database.Write.Exec(`INSERT INTO orders (website_id, number, token, audience, currency,
		email, name, street, postal_code, city,
		items_net, items_tax, items_gross, total_net, total_tax, total_gross,
		created_at, updated_at)
		VALUES ($1, $2, $3, 'private', 'CHF', $4, $5, 'Weg 1', '3000', 'Bern',
		2500, 0, 2500, 2500, 0, 2500, '2026-04-01T08:00:00Z', '2026-04-01T08:00:00Z')`,
		websiteID, number, "tok-"+number, email, name)
	if err != nil {
		t.Fatalf("insert order: %v", err)
	}
	id, _ := res.LastInsertId()
	if _, err := database.Write.Exec(`INSERT INTO order_items (order_id, title, quantity, tax_bp,
		unit_gross, unit_net, line_net, line_tax, line_gross) VALUES ($1, 'Brett', 1, 0, 2500, 2500, 2500, 0, 2500)`, id); err != nil {
		t.Fatalf("insert order item: %v", err)
	}
}

func privacyScreen(websiteID int64) string {
	return "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/datenschutz"
}

// privacyGet calls a handler of the screen with the path value set.
func privacyGet(t *testing.T, h *Handler, sm *scs.SessionManager, fn func(http.ResponseWriter, *http.Request) error, target string, websiteID int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	return serve(t, h, sm, fn, req)
}

func TestThePrivacyScreenFindsAnOrder(t *testing.T) {
	h, sm, database, ws := privacyAdmin(t)
	privacyOrder(t, database, ws.ID, "2026-0042", "Ada@Example.COM", "Ada Lovelace")

	rec := privacyGet(t, h, sm, h.HandlePrivacyPerson, privacyScreen(ws.ID)+"?email=ada@example.com", ws.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if want := `href="/admin/websites/` + strconv.FormatInt(ws.ID, 10) + `/bestellungen/2026-0042"`; !strings.Contains(body, want) {
		t.Errorf("the screen does not link the order: want %s", want)
	}
	if !strings.Contains(body, "Ada Lovelace") {
		t.Error("the screen does not show the customer's name")
	}
	if !strings.Contains(body, "datenschutz/export.json?email=ada%40example.com") {
		t.Error("the screen has no link to the JSON download of this address")
	}

	empty := privacyGet(t, h, sm, h.HandlePrivacyPerson, privacyScreen(ws.ID), ws.ID)
	if empty.Code != http.StatusOK {
		t.Fatalf("without an address: status %d, want 200", empty.Code)
	}
	eb := empty.Body.String()
	if !strings.Contains(eb, `name="email"`) {
		t.Error("without an address the screen has no search form")
	}
	if strings.Contains(eb, "export.json") || strings.Contains(eb, "Ada Lovelace") {
		t.Error("without an address the screen shows a result")
	}
}

func TestThePrivacyScreenRefusesWhatIsNotAnAddress(t *testing.T) {
	h, sm, _, ws := privacyAdmin(t)
	rec := privacyGet(t, h, sm, h.HandlePrivacyPerson, privacyScreen(ws.ID)+"?email=nobody", ws.ID)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "form-error") {
		t.Error("the screen does not say what is wrong with the address")
	}
}

func TestThePrivacyExportIsADownload(t *testing.T) {
	h, sm, database, ws := privacyAdmin(t)
	privacyOrder(t, database, ws.ID, "2026-0043", "ada@example.com", "Ada Lovelace")

	rec := privacyGet(t, h, sm, h.HandlePrivacyExport, privacyScreen(ws.ID)+"/export.json?email=ADA@example.com", ws.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	today := time.Now().UTC().Format("2006-01-02")
	want := `attachment; filename="personal-data-` + strconv.FormatInt(ws.ID, 10) + "-" + today + `.json"`
	if cd := rec.Header().Get("Content-Disposition"); cd != want {
		t.Errorf("Content-Disposition = %q, want %q", cd, want)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	var got dsr.Export
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("the body is not JSON: %v", err)
	}
	if orders := got.Records["orders"]; len(orders) != 1 || orders[0].Key != "2026-0043" {
		t.Errorf("records.orders = %+v, want the order 2026-0043", got.Records["orders"])
	}

	for _, bad := range []string{"", "?email=", "?email=nobody", "?email=ada@example.com%3Fbcc%3Dx"} {
		rec := privacyGet(t, h, sm, h.HandlePrivacyExport, privacyScreen(ws.ID)+"/export.json"+bad, ws.ID)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status %d, want 400", bad, rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("%q: the refusal carries a body: %q", bad, rec.Body.String())
		}
	}
}
