package admin

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRedirectVariesOnHXRequest: h.redirect answers the same request with a 303
// or with HX-Redirect depending on HX-Request, and CLAUDE.md asks for Vary:
// HX-Request on every handler that branches on that header. The sign-out goes
// through it, and so does every other redirect in this package (code review
// IN-03). Cache-Control: no-store keeps a shared cache from storing the answer
// today; this is the rule held where it is written, not a cache that saves it.
func TestRedirectVariesOnHXRequest(t *testing.T) {
	h, _, _, _ := newTestAdmin(t)
	for _, htmx := range []bool{false, true} {
		req := httptest.NewRequest("POST", "/admin/logout", nil)
		if htmx {
			req.Header.Set("HX-Request", "true")
		}
		rec := httptest.NewRecorder()
		if err := h.redirect(rec, req, "/admin/"); err != nil {
			t.Fatalf("redirect: %v", err)
		}
		if vary := strings.Join(rec.Header().Values("Vary"), ","); !strings.Contains(vary, "HX-Request") {
			t.Errorf("HX-Request=%v: Vary = %q; the answer branches on HX-Request and must say so", htmx, vary)
		}
	}
}
