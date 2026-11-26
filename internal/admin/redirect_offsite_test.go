package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// A redirect that leaves the site must not be saved, neither from the screen
// nor through the assistant connection (RDR-02).
func TestRedirectToAnotherServerIsRefused(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	ctx := context.Background()
	target := "/admin/websites/" + strconv.FormatInt(ws.ID, 10) + "/redirects"

	stored := func() []string {
		t.Helper()
		list, err := h.pages.ListRedirects(ctx, ws.ID)
		if err != nil {
			t.Fatalf("ListRedirects: %v", err)
		}
		var from []string
		for _, r := range list {
			from = append(from, r.FromPath+" -> "+r.ToPath)
		}
		return from
	}

	for _, to := range []string{"//evil.example/x", `/\evil.example`} {
		form := url.Values{"from_path": {"/alt"}, "to_path": {to}}
		rec := serve(t, h, sm, h.HandleRedirectList, postForm(target, form, websiteRoute(ws.ID)))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("to_path %q answers %d; want 422", to, rec.Code)
		}
		if got := stored(); len(got) != 0 {
			t.Errorf("to_path %q was stored: %v", to, got)
		}
		if _, _, err := h.OpAddRedirect(ctx, ws.ID, "/alt", to, 301); err == nil {
			t.Errorf("OpAddRedirect accepted %q", to)
		}
		if got := stored(); len(got) != 0 {
			t.Errorf("OpAddRedirect stored %q: %v", to, got)
		}
	}

	form := url.Values{"from_path": {"/alt"}, "to_path": {"/kontakt"}}
	rec := serve(t, h, sm, h.HandleRedirectList, postForm(target, form, websiteRoute(ws.ID)))
	if rec.Code >= 400 {
		t.Errorf("a local target answers %d", rec.Code)
	}
	if got := stored(); len(got) != 1 || got[0] != "/alt -> /kontakt" {
		t.Errorf("stored = %v; want the local redirect", got)
	}
}

// An old row may already hold such a target. The screen says so and marks the
// row, and says nothing when every row is fine.
func TestRedirectScreenMarksRowsThatAreNotFollowed(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	ctx := context.Background()
	list := func() string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/redirects", nil)
		req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
		return serve(t, h, sm, h.HandleRedirectList, req).Body.String()
	}

	if err := h.pages.AddRedirect(ctx, ws.ID, "/c", "/kontakt", 301); err != nil {
		t.Fatalf("AddRedirect: %v", err)
	}
	body := list()
	if n := strings.Count(body, "alert alert--warning"); n != 0 {
		t.Errorf("warning shown %d times with only a good row", n)
	}
	if n := strings.Count(body, "badge badge--warning"); n != 0 {
		t.Errorf("badge shown %d times with only a good row", n)
	}

	for _, row := range []struct{ from, to string }{{"/a", "//evil.example/x"}, {"/b", `/\evil.example`}} {
		if err := h.pages.AddRedirect(ctx, ws.ID, row.from, row.to, 301); err != nil {
			t.Fatalf("AddRedirect: %v", err)
		}
	}
	body = list()
	if n := strings.Count(body, "alert alert--warning"); n != 1 {
		t.Errorf("warning shown %d times; want once", n)
	}
	if n := strings.Count(body, "badge badge--warning"); n != 2 {
		t.Errorf("badge shown %d times; want twice", n)
	}
}
