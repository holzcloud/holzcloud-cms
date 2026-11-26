package public

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// submitFrom is submit with the context the request arrives on.
func submitFrom(t *testing.T, ctx context.Context, h *Handler, ws *domain.Website, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "http://velowerkstatt.test/formular",
		strings.NewReader(form.Encode())).WithContext(ctx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))

	rec := httptest.NewRecorder()
	h.PluginMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the submission went past the plugin to the core")
	})).ServeHTTP(rec, req)
	return rec
}

// One visitor who disconnects must not switch the contact form off for the
// next: the wasm module used to be closed with the request's context and never
// came back, on every website, until a restart.
func TestADroppedVisitorLeavesTheContactFormWorking(t *testing.T) {
	h, database, ws, _ := formularAufbau(t)

	form := func(name string) url.Values {
		return url.Values{
			"seite":     {"kontakt"},
			"pfad":      {"/kontakt"},
			"gestellt":  {timeToken(t, database, -10*time.Second)},
			"name":      {name},
			"email":     {"eva@example.test"},
			"nachricht": {"Guten Tag, ich haette gern eine Auskunft."},
		}
	}

	gone, cancel := context.WithCancel(context.Background())
	cancel()
	rec := submitFrom(t, gone, h, ws, form("Wer geht"))
	if ort := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || !strings.Contains(ort, "formular=gesendet") {
		t.Fatalf("a submission whose visitor is gone: %d %q", rec.Code, ort)
	}

	rec = submitFrom(t, context.Background(), h, ws, form("Wer bleibt"))
	if ort := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || !strings.Contains(ort, "formular=gesendet") {
		t.Fatalf("the next visitor, after one left: %d %q", rec.Code, ort)
	}
	if n := messages(t, database); n != 2 {
		t.Errorf("%d messages stored, expected 2", n)
	}
}
