package public

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// bothPricesSite is a shop that offers the private and the business price,
// which is the only state in which the switch answers at all.
func bothPricesSite(t *testing.T) (*Handler, *domain.Website) {
	t.Helper()
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Demo")
	shopSite(t, h, database, ws)
	setBothPrices(t, database, ws)
	return h, ws
}

func setBothPrices(t *testing.T, database *db.DB, ws *domain.Website) {
	t.Helper()
	if _, err := database.Write.ExecContext(context.Background(),
		`UPDATE websites SET price_display = ? WHERE id = ?`, shop.DisplayBoth, ws.ID); err != nil {
		t.Fatalf("price_display: %v", err)
	}
	ws.PriceDisplay = shop.DisplayBoth
}

func postPriceSwitch(t *testing.T, h *Handler, ws *domain.Website, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "demo.test"
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	rec := httptest.NewRecorder()
	if err := h.HandlePriceSwitch(rec, req); err != nil {
		t.Fatalf("HandlePriceSwitch: %v", err)
	}
	return rec
}

// A visitor chooses where the switch sends them back to, and a path that a
// browser reads as another host must not be followed (L1).
func TestPriceSwitchReturnsOnlyToALocalPath(t *testing.T) {
	h, ws := bothPricesSite(t)

	rec := postPriceSwitch(t, h, ws, "/preise",
		"ansicht=business&zurueck="+url.QueryEscape("/shop/tisch"))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/shop/tisch" {
		t.Fatalf("a local path: status %d, Location %q; want 303 to /shop/tisch",
			rec.Code, rec.Header().Get("Location"))
	}

	for name, back := range map[string]string{
		"backslash":    `/\evil.example`,
		"double slash": "//evil.example",
		"tab":          "/\tevil.example",
	} {
		t.Run(name, func(t *testing.T) {
			rec := postPriceSwitch(t, h, ws, "/preise",
				"ansicht=business&zurueck="+url.QueryEscape(back))
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status %d, want 303", rec.Code)
			}
			if got, want := rec.Header().Get("Location"), ws.ShopURL(); got != want {
				t.Errorf("Location %q for return path %q, want %q", got, back, want)
			}
		})
	}
}

// A link can plant a query string; only a form the visitor posted can carry
// the return path.
func TestPriceSwitchReadsTheReturnPathFromTheBodyOnly(t *testing.T) {
	h, ws := bothPricesSite(t)

	rec := postPriceSwitch(t, h, ws, "/preise?zurueck=/kontakt", "ansicht=private")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}
	if got, want := rec.Header().Get("Location"), ws.ShopURL(); got != want {
		t.Errorf("Location %q, want %q: the query string was read", got, want)
	}
}
