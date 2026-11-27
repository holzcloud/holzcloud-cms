package admin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

func TestABusyPasswordCheckAnswers503WithRetryAfter(t *testing.T) {
	gate := auth.NewGate(1, 10*time.Millisecond, 10*time.Millisecond)
	previous := auth.SetArgon2Gate(gate)
	t.Cleanup(func() { auth.SetArgon2Gate(previous) })
	release, err := gate.Acquire(0)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	h := &Handler{}
	busy := h.ErrHandler(func(w http.ResponseWriter, r *http.Request) error {
		_, err := auth.HashPassword("neues-passwort", auth.Argon2Params{Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
		return err
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/users", nil)
	req = req.WithContext(i18n.WithLang(req.Context(), "de"))
	rec := httptest.NewRecorder()
	busy(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "2" {
		t.Errorf("Retry-After = %q, want 2", got)
	}
	want := web.T(req, "The server is busy checking other passwords. Please try again in a moment.") + "\n"
	if rec.Body.String() != want {
		t.Errorf("body = %q, want %q", rec.Body.String(), want)
	}
	if rec.Body.String() == "The server is busy checking other passwords. Please try again in a moment.\n" {
		t.Error("the busy sentence was not translated for a German request")
	}

	// Any other error is still a 500.
	other := h.ErrHandler(func(w http.ResponseWriter, r *http.Request) error { return errors.New("kaputt") })
	rec = httptest.NewRecorder()
	other(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("another error answered %d, want 500", rec.Code)
	}
}
