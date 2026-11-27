package public

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/plugin/wasmtest"
)

// echoAttachRoute is echoRoute for a plugin that holds the permission to keep
// attachments, so a multipart POST to /echo is taken in before the plugin is
// asked.
func echoAttachRoute(t *testing.T, l plugin.Limits) (*Handler, *domain.Website, *plugin.Manager) {
	t.Helper()
	ctx := context.Background()
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Velowerkstatt")
	h.SetUploadLimits(5<<20, 50<<20)

	module := wasmtest.Module(t, "../plugin/testdata/echo.wasm")
	m := &plugin.Manifest{
		ID: "echo", ABI: plugin.ABIVersion, Name: "Echo", Version: "1.0.0",
		Hooks:       []string{plugin.HookRoute, plugin.HookEvent},
		Routes:      []string{"/echo"},
		Permissions: []string{plugin.PermAttach},
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

// watchSpool records the most files the spool held at the end of any part, so a
// test of "nothing is left afterwards" can also show that something was there.
func watchSpool(t *testing.T, h *Handler) *int {
	t.Helper()
	most := new(int)
	attachPartHook = func() {
		if n := len(spoolFiles(t, h.dataDir)); n > *most {
			*most = n
		}
	}
	t.Cleanup(func() { attachPartHook = nil })
	return most
}

// postEcho posts a multipart body to the echo route through the middleware.
func postEcho(t *testing.T, h *Handler, ws *domain.Website, body []byte, ctype string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "http://velowerkstatt.test/echo", bytes.NewReader(body))
	req.Header.Set("Content-Type", ctype)
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	rec := httptest.NewRecorder()
	h.PluginMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("a claimed route fell through to the core")
	})).ServeHTTP(rec, req)
	return rec
}

func spoolEmpty(t *testing.T, h *Handler, label string) {
	t.Helper()
	if names := spoolEntries(t, h.dataDir); len(names) != 0 {
		t.Errorf("%s: the spool holds %v", label, names)
	}
}

// The spool of a request is gone when the answer is, however the answer came.
func TestTheSpoolIsEmptyOnEveryExitPath(t *testing.T) {
	picture := func() ([]byte, string) {
		return rawMultipart(t, false, filePart("anhang", "bild.png", bigPNG(64<<10)), fld("name", "Anna"))
	}

	t.Run("the plugin declines", func(t *testing.T) {
		h, ws, _ := echoAttachRoute(t, plugin.Limits{CallTimeout: 500 * time.Millisecond, QueueWait: 20 * time.Millisecond})
		most := watchSpool(t, h)
		body, ctype := picture()
		if rec := postEcho(t, h, ws, body, ctype); rec.Code != http.StatusNotFound {
			t.Fatalf("status %d, want 404", rec.Code)
		}
		if *most != 1 {
			t.Errorf("%d files were in the spool during the take, want 1: the test shows nothing", *most)
		}
		spoolEmpty(t, h, "declined")
	})

	t.Run("the plugin is busy", func(t *testing.T) {
		h, ws, manager := echoAttachRoute(t, plugin.Limits{CallTimeout: 500 * time.Millisecond, QueueWait: 20 * time.Millisecond})
		most := watchSpool(t, h)
		endless(manager, ws.ID)
		deadline := time.Now().Add(3 * time.Second)
		for {
			body, ctype := picture()
			rec := postEcho(t, h, ws, body, ctype)
			spoolEmpty(t, h, "busy loop")
			if rec.Code == http.StatusServiceUnavailable {
				if rec.Header().Get("Retry-After") == "" {
					t.Error("a busy answer without Retry-After")
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("the route never answered 503 (last: %d)", rec.Code)
			}
			time.Sleep(5 * time.Millisecond)
		}
		if *most != 1 {
			t.Errorf("%d files were in the spool during a take, want 1", *most)
		}
	})

	t.Run("the call is over its bound", func(t *testing.T) {
		h, ws, _ := echoAttachRoute(t, plugin.Limits{CallTimeout: time.Microsecond})
		most := watchSpool(t, h)
		body, ctype := picture()
		postEcho(t, h, ws, body, ctype)
		if *most != 1 {
			t.Errorf("%d files were in the spool during the take, want 1", *most)
		}
		spoolEmpty(t, h, "timed out")
	})

	t.Run("the body is refused whole", func(t *testing.T) {
		h, ws, _ := echoAttachRoute(t, plugin.Limits{CallTimeout: 500 * time.Millisecond})
		most := watchSpool(t, h)

		cut, ctype := rawMultipart(t, true, filePart("anhang", "bild.png", bigPNG(64<<10)), fld("name", "Anna"))
		if rec := postEcho(t, h, ws, cut, ctype); rec.Code != http.StatusBadRequest {
			t.Errorf("a cut body: status %d, want 400", rec.Code)
		}
		spoolEmpty(t, h, "400")

		parts := append([]formPart{filePart("anhang", "bild.png", bigPNG(64<<10))}, fieldsOf(maxFormParts, 2)...)
		many, ctype := rawMultipart(t, false, parts...)
		rec := postEcho(t, h, ws, many, ctype)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("65 parts: status %d, want 413", rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) != http.StatusText(http.StatusRequestEntityTooLarge) {
			t.Errorf("the visitor was told %q, want the plain status text", rec.Body.String())
		}
		spoolEmpty(t, h, "413")
		if *most != 1 {
			t.Errorf("%d files were in the spool during a take, want 1", *most)
		}
	})

	t.Run("the contact form answers", spoolAfterTheContactForm)
}

// The same through the real contact form: the plugin answers a submission it
// refused or took for a robot without keeping anything, and the spool is empty
// all the same.
func spoolAfterTheContactForm(t *testing.T) {
	h, database, ws, _ := formularAufbau(t)
	h.SetUploadLimits(5<<20, 50<<20)
	most := watchSpool(t, h)

	form := func(extra ...formPart) ([]byte, string) {
		parts := []formPart{
			fld("seite", "kontakt"), fld("pfad", "/kontakt"),
			fld("gestellt", timeToken(t, database, -10*time.Second)),
			fld("name", "Eva Muster"), fld("email", "eva@example.test"),
			fld("nachricht", "Guten Tag."),
		}
		return rawMultipart(t, false, append(parts, extra...)...)
	}

	// The honeypot: answered like a success, nothing kept.
	body, ctype := form(fld("website", "https://spam.example"), filePart("anhang", "bild.png", onePixel(t)))
	rec := postMultipart(t, h, ws, "/formular", body, ctype)
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=gesendet") {
		t.Fatalf("the honeypot: %d %q", rec.Code, ort)
	}
	if n := messages(t, database); n != 0 {
		t.Errorf("the robot left %d messages", n)
	}
	spoolEmpty(t, h, "honeypot")

	// A file the checks refuse: the form answers with an error.
	body, ctype = form(filePart("anhang", "schad.png", []byte("#!/bin/sh\nrm -rf /\n")))
	rec = postMultipart(t, h, ws, "/formular", body, ctype)
	if ort := rec.Header().Get("Location"); !strings.Contains(ort, "formular=fehler") {
		t.Fatalf("a refused file: %d %q, want formular=fehler", rec.Code, ort)
	}
	spoolEmpty(t, h, "refused file")
	if *most < 1 {
		t.Errorf("nothing was ever in the spool: the test shows nothing")
	}
}
