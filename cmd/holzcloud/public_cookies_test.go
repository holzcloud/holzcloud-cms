package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// setCookies returns every Set-Cookie header value of a response.
func setCookies(rec *httptest.ResponseRecorder) []string {
	return rec.Result().Header.Values("Set-Cookie")
}

// The price-view cookie is the one cookie the shop sets for a choice the
// visitor makes. Driven through the real router with the installation
// configured as served over TLS but the request arriving as plain HTTP, which
// is what a TLS-ending proxy passes on: the flag must follow the installation,
// not the request.
func TestPriceViewCookieEndToEnd(t *testing.T) {
	handler, _, database := testRouterWith(t, routerTweaks{shop: true, secure: true})
	ctx := context.Background()

	store := domain.NewStore(database)
	ws, err := store.CreateWebsite(ctx, "Laden", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	if _, err := store.AddDomain(ctx, ws.ID, "shop.test", true); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if _, err := database.Write.ExecContext(ctx,
		`UPDATE websites SET shop_base = 'shop', currency = 'CHF',
		 shipping_gross = 0, shipping_tax_bp = 810, price_display = 'both'
		 WHERE id = ?`, ws.ID); err != nil {
		t.Fatalf("switch the shop on: %v", err)
	}
	if _, err := page.NewStore(database).CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "Willkommen", Slug: "home", Markdown: "Hallo", HTML: "<p>Hallo</p>",
		Status: "published",
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	get := httptest.NewRequest(http.MethodGet, "/", nil)
	get.Host = "shop.test"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, get)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /: status %d, want 200", rec.Code)
	}
	if got := setCookies(rec); len(got) != 0 {
		t.Errorf("GET / set a cookie without the visitor doing anything: %q", got)
	}

	form := url.Values{"ansicht": {"business"}, "zurueck": {"/shop"}}
	post := httptest.NewRequest(http.MethodPost, "/preise", strings.NewReader(form.Encode()))
	post.Host = "shop.test"
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, post)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /preise: status %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/shop" {
		t.Errorf("POST /preise: Location = %q, want /shop", loc)
	}
	got := setCookies(rec)
	if len(got) != 1 {
		t.Fatalf("POST /preise: want exactly one Set-Cookie, got %q", got)
	}
	c := got[0]
	if !strings.HasPrefix(c, shop.AudienceCookieName+"=business") {
		t.Errorf("cookie = %q, want %s=business", c, shop.AudienceCookieName)
	}
	for _, want := range []string{"Max-Age=2592000", "HttpOnly", "Secure", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(c, want) {
			t.Errorf("cookie %q lacks %q", c, want)
		}
	}
}
