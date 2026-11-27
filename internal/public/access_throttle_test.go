package public

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// gateWrongLine is what the test gate prints after a wrong password.
const gateWrongLine = "falsches Passwort"

// guess posts the gate's form for the page as the client at remote.
func guess(h *Handler, ws *domain.Website, remote, seite, password string) *httptest.ResponseRecorder {
	form := url.Values{"seite": {seite}, "passwort": {password}}
	req := httptest.NewRequest(http.MethodPost, "/freischalten", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "demo.test"
	req.RemoteAddr = remote
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	rec := httptest.NewRecorder()
	_ = h.HandleUnlock(rec, req)
	return rec
}

// gateSite is a website with two protected pages, "preise" and "geheim", whose
// password is "richtig-1" in both cases.
func gateSite(t *testing.T) (*Handler, *domain.Website, *db.DB) {
	t.Helper()
	h, database := newTestHandler(t)
	signers(h)
	h.SetClientIP(web.NewClientIPResolver(nil))
	ws := seedWebsite(t, database, "Demo")
	for _, slug := range []string{"preise", "geheim"} {
		seedPage(t, database, ws.ID, slug, slug, "Inhalt "+slug, "published")
		pg := pageBySlug(t, page.NewStore(database), ws.ID, slug)
		protect(t, database, pg.ID, "richtig-1")
	}
	return h, ws, database
}

func assertRefused(t *testing.T, rec *httptest.ResponseRecorder, status int, password string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d", rec.Code, status)
	}
	if ra, err := strconv.Atoi(rec.Header().Get("Retry-After")); err != nil || ra < 1 {
		t.Errorf("Retry-After = %q, want whole seconds of at least 1", rec.Header().Get("Retry-After"))
	}
	body := rec.Body.String()
	if !strings.Contains(body, `name="passwort"`) {
		t.Errorf("not the theme's gate:\n%s", body)
	}
	if strings.Contains(body, gateWrongLine) {
		t.Error("a refusal claims the password was wrong; nothing was checked")
	}
	if strings.Contains(body, password) {
		t.Error("the answer echoes the password")
	}
	if len(rec.Result().Cookies()) != 0 || rec.Header().Get("Set-Cookie") != "" {
		t.Error("a refusal sets a cookie")
	}
}

func TestTheGateIsThrottledPerClientAndPage(t *testing.T) {
	h, ws, _ := gateSite(t)
	for i := 1; i <= 10; i++ {
		rec := guess(h, ws, "198.51.100.7:4000", "preise", fmt.Sprintf("falsch-%d", i))
		if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), gateWrongLine) {
			t.Fatalf("attempt %d: status %d, wrong line shown: %v", i, rec.Code, strings.Contains(rec.Body.String(), gateWrongLine))
		}
	}

	// The eleventh is the right password and is refused all the same.
	assertRefused(t, guess(h, ws, "198.51.100.7:4001", "preise", "richtig-1"), http.StatusTooManyRequests, "richtig-1")

	// Another page is another budget; another client is another budget.
	if rec := guess(h, ws, "198.51.100.7:4002", "geheim", "richtig-1"); rec.Code != http.StatusSeeOther {
		t.Errorf("the same client on another page: %d, want 303", rec.Code)
	}
	if rec := guess(h, ws, "198.51.100.8:4000", "preise", "richtig-1"); rec.Code != http.StatusSeeOther {
		t.Errorf("another client on the same page: %d, want 303", rec.Code)
	}
}

func TestTheGateHasALooseLimitPerPage(t *testing.T) {
	h, ws, _ := gateSite(t)
	for i := 1; i <= 100; i++ {
		rec := guess(h, ws, fmt.Sprintf("203.0.113.%d:4000", i), "preise", "falsch-geraten")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d from its own address: status %d, want 401", i, rec.Code)
		}
	}
	assertRefused(t, guess(h, ws, "203.0.113.200:4000", "preise", "richtig-1"), http.StatusTooManyRequests, "richtig-1")
}

func TestAPrivateClientAddressHasNoClientBudget(t *testing.T) {
	h, ws, _ := gateSite(t)
	// A proxy that is not in the trusted list: every visitor arrives as one
	// private address.
	h.SetClientIP(web.NewClientIPResolver([]netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}))
	for i := 1; i <= 11; i++ {
		rec := guess(h, ws, "172.17.0.1:4000", "preise", fmt.Sprintf("falsch-%d", i))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status %d, want 401 (only the page's budget applies)", i, rec.Code)
		}
	}
	if rec := guess(h, ws, "172.17.0.1:4000", "preise", "richtig-1"); rec.Code != http.StatusSeeOther {
		t.Errorf("the twelfth, with the right password: %d, want 303", rec.Code)
	}
}

func TestAParallelBurstAtTheGateStaysWithinTheLimit(t *testing.T) {
	h, ws, _ := gateSite(t)
	var (
		mu     sync.Mutex
		counts = map[int]int{}
		wg     sync.WaitGroup
	)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := guess(h, ws, "198.51.100.7:4000", "preise", fmt.Sprintf("falsch-%d", i))
			mu.Lock()
			counts[rec.Code]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if counts[http.StatusUnauthorized] > gateClientMax {
		t.Errorf("%d checks ran for one client, the limit is %d: %v", counts[http.StatusUnauthorized], gateClientMax, counts)
	}
	if counts[http.StatusInternalServerError] != 0 {
		t.Errorf("a burst caused server errors: %v", counts)
	}
	if got := counts[http.StatusUnauthorized] + counts[http.StatusTooManyRequests] + counts[http.StatusServiceUnavailable]; got != 50 {
		t.Errorf("unexpected statuses: %v", counts)
	}
}

func TestABusyGateAnswers503AndIsNotCounted(t *testing.T) {
	h, ws, _ := gateSite(t)

	gate := auth.NewGate(1, time.Millisecond, time.Millisecond)
	release, err := gate.Acquire(0)
	if err != nil {
		t.Fatal(err)
	}
	previous := auth.SetArgon2Gate(gate)
	t.Cleanup(func() { auth.SetArgon2Gate(previous) })

	// More busy answers than the client budget: none of them may be counted.
	for i := 0; i < 15; i++ {
		assertRefused(t, guess(h, ws, "198.51.100.7:4000", "preise", "richtig-1"), http.StatusServiceUnavailable, "richtig-1")
	}
	release()

	for i := 1; i <= 10; i++ {
		rec := guess(h, ws, "198.51.100.7:4000", "preise", fmt.Sprintf("falsch-%d", i))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d after the busy ones: status %d, want 401 (a busy answer was counted)", i, rec.Code)
		}
	}
	assertRefused(t, guess(h, ws, "198.51.100.7:4000", "preise", "richtig-1"), http.StatusTooManyRequests, "richtig-1")
}

func TestAnOversizedUnlockBodyIsRefused(t *testing.T) {
	h, ws, _ := gateSite(t)
	big := strings.Repeat("x", 5<<10)

	rec := guess(h, ws, "198.51.100.7:4000", "preise", big)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("a 5 KiB body: status %d, want 413", rec.Code)
	}
	// Before any page lookup: a page that does not exist gets the same answer.
	rec = guess(h, ws, "198.51.100.7:4000", "gibt-es-nicht", big)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("a 5 KiB body for no page: status %d, want 413", rec.Code)
	}
}
