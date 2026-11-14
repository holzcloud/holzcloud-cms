package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// postValueForm sends a form the way the Values screen's form does, through
// the real router, signed in with cookie.
func postValueForm(handler http.Handler, target string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Host = "admin.test"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// A value is made on the screen and read on the published page, through the
// router main() serves: the screen writes the store, and the public page reads
// the same store on its next request. Nothing about the page is saved in
// between — the page was written with the marker before the value existed.
func TestAValueMadeOnTheScreenShowsOnThePublicPage(t *testing.T) {
	handler, sm, database := testRouter(t)
	ctx := context.Background()
	admin := seedUser(t, handler, sm, database, "admin-werte@test", "admin")
	// An administrator owes a second factor before any screen opens; this one
	// has confirmed it, so the request reaches the Values screen.
	if _, err := database.Write.ExecContext(ctx,
		`UPDATE users SET totp_secret = 'JBSWY3DPEHPK3PXP', totp_confirmed_at = CURRENT_TIMESTAMP
		  WHERE email = $1`, "admin-werte@test"); err != nil {
		t.Fatalf("confirm the second factor: %v", err)
	}

	domains := domain.NewStore(database)
	ws, err := domains.CreateWebsite(ctx, "Werte", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	if _, err := domains.AddDomain(ctx, ws.ID, "werte.test", true); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	const markdown = "Alpha [[wert:version-holzcloud-cms]]: neu"
	html, err := page.RenderMarkdown(markdown)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if _, err := page.NewStore(database).CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "Stand", Slug: "stand", Markdown: markdown, HTML: html,
		Status: "published",
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	list := fmt.Sprintf("/admin/websites/%d/werte", ws.ID)
	rec := postValueForm(handler, list, url.Values{
		"key": {"version-holzcloud-cms"}, "text": {"0.1.5"},
	}, admin)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST %s: status %d, want 303; body: %s", list, rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != list {
		t.Errorf("POST %s: Location = %q, want the list", list, loc)
	}

	rec = do(handler, http.MethodGet, list, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d, want 200", list, rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"[[wert:version-holzcloud-cms]]", "0.1.5"} {
		if !strings.Contains(body, want) {
			t.Errorf("the Values screen does not list %q", want)
		}
	}

	get := httptest.NewRequest(http.MethodGet, "/stand", nil)
	get.Host = "werte.test"
	pub := httptest.NewRecorder()
	handler.ServeHTTP(pub, get)
	if pub.Code != http.StatusOK {
		t.Fatalf("GET /stand: status %d, want 200", pub.Code)
	}
	served, _ := io.ReadAll(pub.Result().Body)
	if !strings.Contains(string(served), "Alpha 0.1.5: neu") {
		t.Errorf("the published page does not show the value made on the screen")
	}
	if strings.Contains(string(served), "[[wert:") {
		t.Errorf("the published page still carries a raw marker")
	}
}
