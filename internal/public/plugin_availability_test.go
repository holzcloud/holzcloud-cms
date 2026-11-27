package public

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/plugin/wasmtest"
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

// echoRoute loads the echo guest of the plugin tests as a plugin that claims
// /echo, with the bounds of a call set by the test. The guest answers the route
// hook with nothing, so a request that reaches it is declined; what the tests
// below are about is the request that does not reach it.
//
// It mirrors loadPlugin, which builds its runtime inside and so has no place
// for SetLimits, and wires none of the host functions: the echo guest asks for
// none on this path.
func echoRoute(t *testing.T, l plugin.Limits) (*Handler, *domain.Website, *plugin.Manager) {
	t.Helper()
	ctx := context.Background()
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Velowerkstatt")

	module := wasmtest.Module(t, "../plugin/testdata/echo.wasm")
	m := &plugin.Manifest{
		ID: "echo", ABI: plugin.ABIVersion, Name: "Echo", Version: "1.0.0",
		Hooks:  []string{plugin.HookRoute, plugin.HookEvent},
		Routes: []string{"/echo"},
	}
	store := plugin.NewStore(database)
	if err := store.Install(ctx, &plugin.Package{Manifest: m, Module: module, SHA256: strings.Repeat("e", 64)}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.SetEnabled(ctx, m.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if err := store.SetWebsites(ctx, m.ID, []int64{ws.ID}); err != nil {
		t.Fatalf("SetWebsites: %v", err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "plugins", m.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins", m.ID, plugin.ModuleName), module, 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := plugin.NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	t.Cleanup(func() { rt.Close(context.Background()) })
	rt.SetLimits(l)
	manager, err := plugin.NewManager(ctx, store, rt, dir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	h.SetPlugins(manager)
	return h, ws, manager
}

// getEcho asks for /echo through the plugin middleware. A request that gets
// past the plugin to the core is a failure of the test: /echo is the plugin's.
func getEcho(t *testing.T, h *Handler, ws *domain.Website) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "http://velowerkstatt.test/echo", nil)
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	rec := httptest.NewRecorder()
	h.PluginMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("a claimed route fell through to the core")
	})).ServeHTTP(rec, req)
	return rec
}

// endless sends the echo guest into its endless loop, the way a plugin's event
// reaches it in the server: in a goroutine of its own, for the call's own bound.
func endless(m *plugin.Manager, websiteID int64) {
	m.Emit("test", websiteID, map[string]string{"tue": "endlos"})
}

// retryAfter is the whole number of seconds a 503 asks for.
func retryAfter(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()
	n, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil {
		t.Fatalf("Retry-After is not a number of seconds: %q", rec.Header().Get("Retry-After"))
	}
	return n
}

// A route of a plugin that cannot take the request now answers 503 with a time
// to come back, and not a 404: the page exists, it is the plugin that is
// occupied. It also does not hang.
func TestABusyPluginRouteAnswers503(t *testing.T) {
	h, ws, manager := echoRoute(t, plugin.Limits{CallTimeout: 500 * time.Millisecond, QueueWait: 20 * time.Millisecond})

	if rec := getEcho(t, h, ws); rec.Code != http.StatusNotFound {
		t.Fatalf("a plugin that declines the route: %d", rec.Code)
	}

	endless(manager, ws.ID)
	deadline := time.Now().Add(3 * time.Second)
	for {
		start := time.Now()
		rec := getEcho(t, h, ws)
		if rec.Code == http.StatusServiceUnavailable {
			if n := retryAfter(t, rec); n < 1 {
				t.Errorf("Retry-After %d", n)
			}
			if took := time.Since(start); took > 300*time.Millisecond {
				t.Errorf("the busy answer took %v", took)
			}
			if strings.Contains(rec.Body.String(), "busy") {
				t.Errorf("the visitor was shown a technical sentence: %q", rec.Body.String())
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the route never answered 503 while the plugin was occupied (last: %d)", rec.Code)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// After three failed calls in a row the plugin is held back, and its route says
// so with the cooldown as Retry-After.
func TestAHeldBackPluginRouteAnswers503(t *testing.T) {
	h, ws, manager := echoRoute(t, plugin.Limits{CallTimeout: 50 * time.Millisecond})

	for n := 0; n < 3; n++ {
		endless(manager, ws.ID)
	}
	// Three calls of 50 ms each, and the request may not come in between them:
	// a request the plugin declines is a success, and a success starts the count
	// again. Asking later than that is first in the line behind all three, since
	// the line is served in the order it formed.
	time.Sleep(600 * time.Millisecond)
	// What distinguishes a hold from a busy plugin is the time: the cooldown is
	// thirty seconds, the queue wait two.
	deadline := time.Now().Add(5 * time.Second)
	for {
		rec := getEcho(t, h, ws)
		if rec.Code == http.StatusServiceUnavailable && retryAfter(t, rec) > 5 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the route of a held-back plugin never answered 503 with the cooldown (last: %d, Retry-After %q)",
				rec.Code, rec.Header().Get("Retry-After"))
		}
		time.Sleep(10 * time.Millisecond)
	}
}
