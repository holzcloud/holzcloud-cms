package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
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
