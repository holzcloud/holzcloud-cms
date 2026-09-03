package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// The link check moved from the 404 screen onto the redirect screen when the
// 404 list became a plugin. It has to still run, and it has to still be visible:
// a check that quietly stopped reporting is worse than no check.
func TestRedirectScreenReportsBrokenInternalLinks(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	seedPage(t, database, ws.ID, "Über uns", "ueber-uns",
		"Mehr auf [der Hofseite](/hofseite) und im [Laden](/laden).", "published")
	seedPage(t, database, ws.ID, "Laden", "laden", "Offen am Samstag.", "published")

	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/redirects", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	body := serve(t, h, sm, h.HandleRedirectList, req).Body.String()

	if !strings.Contains(body, "/hofseite") {
		t.Error("der Link auf die fehlende Seite wird nicht gemeldet")
	}
	if strings.Contains(body, ">/laden<") {
		t.Error("ein Link auf eine vorhandene Seite wurde als kaputt gemeldet")
	}
}

// A redirect is the second way to fix a broken link, so an existing one must
// count as resolved — otherwise the list never empties and stops being read.
func TestRedirectMakesALinkResolve(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	seedPage(t, database, ws.ID, "Start", "start", "Siehe [alte Seite](/alte-seite).", "published")

	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/redirects", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	if !strings.Contains(serve(t, h, sm, h.HandleRedirectList, req).Body.String(), "/alte-seite") {
		t.Fatal("ohne Weiterleitung muss der Link als kaputt gelten")
	}

	if err := h.pages.AddRedirect(context.Background(), ws.ID, "/alte-seite", "/start", 301); err != nil {
		t.Fatalf("AddRedirect: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/websites/1/redirects", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	body := serve(t, h, sm, h.HandleRedirectList, req).Body.String()
	if !strings.Contains(body, "Alle internen Links führen irgendwohin") {
		t.Error("der Link gilt trotz Weiterleitung noch als kaputt")
	}
}

// The link on a broken entry fills the form rather than creating anything: the
// target is a judgement call, and a one-click redirect to a guessed page would
// be a wrong redirect nobody notices.
func TestBrokenLinkPrefillsTheForm(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	seedPage(t, database, ws.ID, "Start", "start", "Siehe [alt](/alte-seite).", "published")

	req := httptest.NewRequest(http.MethodGet,
		"/admin/websites/1/redirects?from=/alte-seite", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	body := serve(t, h, sm, h.HandleRedirectList, req).Body.String()

	if !strings.Contains(body, `id="from_path" name="from_path" class="form-input" required`) ||
		!strings.Contains(body, `value="/alte-seite"`) {
		t.Error("die alte Adresse steht nicht im Formular")
	}
}
