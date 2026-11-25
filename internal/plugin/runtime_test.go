package plugin

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/plugin/wasmtest"
)

// A "go generate ./..." from the root directory does not reach the five plugins
// — they are modules of their own. This line is therefore only the local
// shortcut for the one module that lies inside the root module; all six are
// built by "go run ./tools/wasm". -buildvcs=false belongs to it: without that
// flag the module carries the git state of the moment and a second build yields
// different bytes.
//go:generate sh -c "cd testdata/echo && GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -trimpath -buildvcs=false -ldflags=\"-s -w\" -o ../echo.wasm ."

// echoModule is built in testdata/echo/, see testdata/README.md and the
// generate line above.
//
// It lies built in the repository so that the tests run without a second
// toolchain — a test that needs a compiler run gets skipped eventually and is
// then never run again.
func echoModule(t *testing.T) []byte {
	return wasmtest.Module(t, "testdata/echo.wasm")
}

func neueLaufzeit(t *testing.T, erlaubt ...string) (*Runtime, *Store, *bytes.Buffer) {
	t.Helper()
	s, _ := neuerSpeicher(t)
	var log bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ctx := context.Background()
	r, err := NewRuntime(ctx, s, logger)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	t.Cleanup(func() { r.Close(context.Background()) })

	m := goodManifest()
	m.ID = "echo"
	m.Name = "Echo"
	m.Hooks = []string{HookContent, HookEvent, HookRequest, HookAdmin}
	m.Permissions = erlaubt

	p := &Package{Manifest: &m, Module: echoModule(t), SHA256: strings.Repeat("a", 64)}
	if err := s.Install(ctx, p); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := r.Load(ctx, &m, p.Module); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return r, s, &log
}

func TestHakenWirdAufgerufenUndAntwortKommtZurueck(t *testing.T) {
	r, _, _ := neueLaufzeit(t, PermStore)
	var out ContentOut
	err := r.Dispatch(context.Background(), "echo", HookContent, 1,
		ContentIn{WebsiteID: 1, Slug: "home", HTML: "<p>Hallo</p>"}, &out)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !out.Changed || out.HTML != "<p>Hallo</p><!-- echo -->" {
		t.Errorf("Antwort: %+v", out)
	}
}

func TestNichtDeklarierterHakenWirdNichtAufgerufen(t *testing.T) {
	r, _, _ := neueLaufzeit(t, PermStore)
	// The manifest does not name "route". That is not a fault but means only:
	// this plugin costs nothing in that place.
	var out RequestOut
	if err := r.Dispatch(context.Background(), "echo", HookRoute, 1, RequestIn{}, &out); err != nil {
		t.Errorf("an undeclared hook reported an error: %v", err)
	}
	if out.Handled {
		t.Error("ein nicht deklarierter Haken hat geantwortet")
	}
}

func TestEigenerSpeicherUeberDieGrenze(t *testing.T) {
	r, s, _ := neueLaufzeit(t, PermStore)
	ctx := context.Background()

	ereignis := func(data map[string]string, out any) {
		t.Helper()
		if err := r.Dispatch(ctx, "echo", HookEvent, 7,
			EventIn{Name: "test", WebsiteID: 7, Data: data}, out); err != nil {
			t.Fatalf("Dispatch: %v", err)
		}
	}
	ereignis(map[string]string{"tue": "schreiben", "key": "farbe", "value": "grün"}, nil)

	// The host has to see it under exactly this plugin and this website.
	v, ok, err := s.StoreGet(ctx, "echo", 7, "farbe")
	if err != nil || !ok || v != "grün" {
		t.Fatalf("the value did not arrive: %q %v %v", v, ok, err)
	}

	var gelesen struct {
		Gelesen string `json:"gelesen"`
	}
	ereignis(map[string]string{"tue": "lesen", "key": "farbe"}, &gelesen)
	if !strings.Contains(gelesen.Gelesen, "grün") {
		t.Errorf("the plugin did not read its own value: %q", gelesen.Gelesen)
	}
}

func TestFehlendeBerechtigungWirdVerweigert(t *testing.T) {
	// Nur "store", nicht "settings".
	r, _, _ := neueLaufzeit(t, PermStore)
	var out struct {
		Status  int    `json:"status"`
		Message string `json:"meldung"`
	}
	if err := r.Dispatch(context.Background(), "echo", HookEvent, 1,
		EventIn{Name: "test", Data: map[string]string{"tue": "verboten"}}, &out); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if out.Status != StatusDenied {
		t.Errorf("Status %d, erwartet %d (verweigert)", out.Status, StatusDenied)
	}
	// The message has to say which permission is missing — it is a fault in the
	// manifest, not in the code, and the author should find it without guessing.
	if !strings.Contains(out.Message, PermSettings) {
		t.Errorf("die Meldung nennt die Berechtigung nicht: %q", out.Message)
	}
}

func TestUnbekannteOperationWirdGemeldet(t *testing.T) {
	r, _, _ := neueLaufzeit(t, PermStore)
	var out struct {
		Status  int    `json:"status"`
		Message string `json:"meldung"`
	}
	if err := r.Dispatch(context.Background(), "echo", HookEvent, 1,
		EventIn{Name: "test", Data: map[string]string{"tue": "unbekannt"}}, &out); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if out.Status != StatusError || !strings.Contains(out.Message, "gibtsnicht") {
		t.Errorf("Status %d, Meldung %q", out.Status, out.Message)
	}
}

func TestProtokollTraegtDenNamenDesPlugins(t *testing.T) {
	r, _, log := neueLaufzeit(t, PermStore, PermLog)
	if err := r.Dispatch(context.Background(), "echo", HookEvent, 1,
		EventIn{Name: "test", Data: map[string]string{"tue": "protokoll", "value": "etwas ist passiert"}},
		nil); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	s := log.String()
	// The name comes from the host and not from the message: a plugin should not
	// be able to write a line that looks like somebody else.
	if !strings.Contains(s, `plugin=echo`) || !strings.Contains(s, "etwas ist passiert") {
		t.Errorf("Protokoll: %s", s)
	}
}

func TestKurzerPufferWirdNachgefordert(t *testing.T) {
	r, s, _ := neueLaufzeit(t, PermStore)
	ctx := context.Background()
	// Larger than the guest's 4096-byte buffer, so that it has to ask for the
	// size and then ask once more.
	gross := strings.Repeat("x", 20000)
	if err := s.StoreSet(ctx, "echo", 3, "gross", gross); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Length int `json:"laenge"`
	}
	if err := r.Dispatch(ctx, "echo", HookEvent, 3,
		EventIn{Name: "test", WebsiteID: 3, Data: map[string]string{"tue": "grosse-antwort", "key": "gross"}},
		&out); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if out.Length < len(gross) {
		t.Errorf("only %d bytes back, expected at least %d", out.Length, len(gross))
	}
}

func TestEndlosschleifeWirdAbgebrochen(t *testing.T) {
	r, s, _ := neueLaufzeit(t, PermStore)
	ctx := context.Background()

	start := time.Now()
	err := r.Dispatch(ctx, "echo", HookEvent, 1,
		EventIn{Name: "test", Data: map[string]string{"tue": "endlos"}}, nil)
	dauer := time.Since(start)

	if err == nil {
		t.Fatal("die Endlosschleife lief durch")
	}
	// Without a time limit a module like this holds the connection, then the
	// next one, and the website stops answering — without a line in the log.
	if dauer > 3*CallTimeout {
		t.Errorf("the abort took %v, the limit is %v", dauer, CallTimeout)
	}
	// The reason has to be noted on the plugin, not only in the log from three
	// restarts ago.
	p, _ := s.Get(ctx, "echo")
	if p.LastError == "" {
		t.Error("the abort was not recorded on the plugin")
	}
}

func TestUnsinnigeAntwortBrichtDieAnfrageNicht(t *testing.T) {
	r, s, _ := neueLaufzeit(t, PermStore)
	var out ContentOut
	err := r.Dispatch(context.Background(), "echo", HookEvent, 1,
		EventIn{Name: "test", Data: map[string]string{"tue": "muell"}}, &out)
	if err == nil {
		t.Fatal("invalid JSON was accepted")
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Errorf("die Meldung sagt nicht, was falsch war: %v", err)
	}
	if p, _ := s.Get(context.Background(), "echo"); p.LastError == "" {
		t.Error("the error was not recorded on the plugin")
	}
}

func TestModulOhneExporteWirdAbgelehnt(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	r, err := NewRuntime(ctx, s, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close(ctx)

	m := goodManifest()
	// A valid but empty module: the header is right, the exports are missing.
	leer := []byte{0x00, 'a', 's', 'm', 0x01, 0x00, 0x00, 0x00}
	err = r.Load(ctx, &m, leer)
	if err == nil || !strings.Contains(err.Error(), GuestAlloc) {
		t.Errorf("erwartet: fehlender Export genannt, bekommen: %v", err)
	}
}

func TestEntladenUndNichtGeladen(t *testing.T) {
	r, _, _ := neueLaufzeit(t, PermStore)
	ctx := context.Background()
	if !r.Loaded("echo") {
		t.Fatal("not loaded after Load")
	}
	r.Unload(ctx, "echo")
	if r.Loaded("echo") {
		t.Error("still loaded after Unload")
	}
	if err := r.Dispatch(ctx, "echo", HookContent, 1, ContentIn{}, &ContentOut{}); err == nil {
		t.Error("ein entladenes Plugin wurde aufgerufen")
	}
}

// Switching a plugin on loads it inside the admin's request. When the browser
// drops that request — a navigation, a closed tab — its context is cancelled
// while the module compiles or runs _initialize, and the runtime, which closes
// a module whose context is done, used to close the plugin mid-start: switched
// on, not running (found by the phase 34 browser record). A load must not
// depend on the caller staying connected.
func TestALoadOutlivesTheRequestThatStartedIt(t *testing.T) {
	s, _ := neuerSpeicher(t)
	r, err := NewRuntime(context.Background(), s, nil)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	t.Cleanup(func() { r.Close(context.Background()) })

	m := goodManifest()
	m.ID = "echo"
	m.Name = "Echo"
	m.Hooks = []string{HookContent}
	p := &Package{Manifest: &m, Module: echoModule(t), SHA256: strings.Repeat("a", 64)}
	if err := s.Install(context.Background(), p); err != nil {
		t.Fatalf("Install: %v", err)
	}

	gone, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Load(gone, &m, p.Module); err != nil {
		t.Fatalf("Load with the request already gone: %v", err)
	}
	var out ContentOut
	if err := r.Dispatch(context.Background(), "echo", HookContent, 1,
		ContentIn{WebsiteID: 1, Slug: "home", HTML: "<p>Hallo</p>"}, &out); err != nil {
		t.Fatalf("the plugin does not answer after a load whose request was gone: %v", err)
	}
	if !out.Changed {
		t.Errorf("the plugin did not run: %+v", out)
	}
}

// spinningModule is a hand-assembled wasm module whose _initialize never
// returns. It exports the names Load requires (GuestAlloc, GuestHandle), so
// that Load's export check passes and the failure comes from the start
// function. All three exports point at function 0, an endless loop.
func spinningModule() []byte {
	return []byte{
		// header: magic and version
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
		// type section: one function type, no parameters, no results
		0x01, 0x04, 0x01, 0x60, 0x00, 0x00,
		// function section: one function of type 0
		0x03, 0x02, 0x01, 0x00,
		// export section: _initialize, hc_alloc, hc_handle, each kind func index 0
		0x07, 0x26, 0x03,
		0x0b, '_', 'i', 'n', 'i', 't', 'i', 'a', 'l', 'i', 'z', 'e', 0x00, 0x00,
		0x08, 'h', 'c', '_', 'a', 'l', 'l', 'o', 'c', 0x00, 0x00,
		0x09, 'h', 'c', '_', 'h', 'a', 'n', 'd', 'l', 'e', 0x00, 0x00,
		// code section: one body, no locals: loop, br 0, end, end
		0x0a, 0x09, 0x01, 0x07, 0x00, 0x03, 0x40, 0x0c, 0x00, 0x0b, 0x0b,
	}
}

// spinningLoad runs Load on spinningModule with a one-second bound and a
// caller whose context is cancelled cancelAfter into the load (never, when it
// is zero). It returns the error and how long the load took.
func spinningLoad(t *testing.T, cancelAfter time.Duration) (error, time.Duration) {
	t.Helper()
	s, _ := neuerSpeicher(t)
	r, err := NewRuntime(context.Background(), s, nil)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	t.Cleanup(func() { r.Close(context.Background()) })
	r.loadTimeout = time.Second

	m := goodManifest()
	m.ID = "spin"
	m.Name = "Spin"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if cancelAfter > 0 {
		time.AfterFunc(cancelAfter, cancel)
	}
	start := time.Now()
	err = r.Load(ctx, &m, spinningModule())
	took := time.Since(start)
	if r.Loaded("spin") {
		t.Error("a module whose start never returns is loaded")
	}
	return err, took
}

// IN-01: the bound is what ends a load whose _initialize never returns.
func TestALoadThatNeverStartsIsCutAtItsBound(t *testing.T) {
	err, took := spinningLoad(t, 0)
	if err == nil || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("expected deadline exceeded, got: %v", err)
	}
	if took < 500*time.Millisecond || took > 10*time.Second {
		t.Errorf("the load took %v, expected about the one-second bound", took)
	}
}

// IN-01: a cancel arriving while _initialize runs is not honoured; only the
// load's own bound ends it.
func TestACancelDuringInitializeDoesNotEndTheLoad(t *testing.T) {
	err, took := spinningLoad(t, 50*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "canceled") {
		t.Fatalf("expected deadline exceeded and not canceled, got: %v", err)
	}
	if took < 500*time.Millisecond {
		t.Errorf("the caller's cancel ended the load after %v", took)
	}
}

// The success path of the same: the request is dropped 50 ms into the load and
// the plugin still comes up.
func TestALoadCancelledMidwayStillRuns(t *testing.T) {
	s, _ := neuerSpeicher(t)
	r, err := NewRuntime(context.Background(), s, nil)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	t.Cleanup(func() { r.Close(context.Background()) })

	m := goodManifest()
	m.ID = "echo"
	m.Name = "Echo"
	m.Hooks = []string{HookContent}
	p := &Package{Manifest: &m, Module: echoModule(t), SHA256: strings.Repeat("a", 64)}
	if err := s.Install(context.Background(), p); err != nil {
		t.Fatalf("Install: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(50*time.Millisecond, cancel)
	if err := r.Load(ctx, &m, p.Module); err != nil {
		t.Fatalf("Load cancelled midway: %v", err)
	}
	var out ContentOut
	if err := r.Dispatch(context.Background(), "echo", HookContent, 1,
		ContentIn{WebsiteID: 1, Slug: "home", HTML: "<p>Hallo</p>"}, &out); err != nil {
		t.Fatalf("the plugin does not answer: %v", err)
	}
	if !out.Changed {
		t.Errorf("the plugin did not run: %+v", out)
	}
}
