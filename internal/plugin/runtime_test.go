package plugin

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

//go:generate sh -c "cd testdata/echo && GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -trimpath -ldflags=\"-s -w\" -o ../echo.wasm ."

// echoModul ist in testdata/echo/ gebaut, siehe testdata/README.md:
//
//	cd testdata/echo && GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o ../echo.wasm .
//
// Es liegt gebaut im Repository, damit die Tests ohne zweite Werkzeugkette
// laufen — ein Test, der einen Compiler-Lauf braucht, wird irgendwann
// übersprungen und dann nie wieder ausgeführt.
func echoModul(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/echo.wasm")
	if err != nil {
		t.Skipf("testdata/echo.wasm fehlt: %v", err)
	}
	return b
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

	m := gutesManifest()
	m.ID = "echo"
	m.Name = "Echo"
	m.Hooks = []string{HookContent, HookEvent, HookRequest, HookAdmin}
	m.Permissions = erlaubt

	p := &Package{Manifest: &m, Module: echoModul(t), SHA256: strings.Repeat("a", 64)}
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
	// Das Manifest nennt "route" nicht. Das ist kein Fehler, sondern heisst
	// nur: dieses Plugin kostet an dieser Stelle nichts.
	var out RequestOut
	if err := r.Dispatch(context.Background(), "echo", HookRoute, 1, RequestIn{}, &out); err != nil {
		t.Errorf("ein nicht deklarierter Haken meldete einen Fehler: %v", err)
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

	// Der Host muss es unter genau diesem Plugin und dieser Website sehen.
	v, ok, err := s.StoreGet(ctx, "echo", 7, "farbe")
	if err != nil || !ok || v != "grün" {
		t.Fatalf("der Wert kam nicht an: %q %v %v", v, ok, err)
	}

	var gelesen struct {
		Gelesen string `json:"gelesen"`
	}
	ereignis(map[string]string{"tue": "lesen", "key": "farbe"}, &gelesen)
	if !strings.Contains(gelesen.Gelesen, "grün") {
		t.Errorf("das Plugin hat seinen eigenen Wert nicht gelesen: %q", gelesen.Gelesen)
	}
}

func TestFehlendeBerechtigungWirdVerweigert(t *testing.T) {
	// Nur "store", nicht "settings".
	r, _, _ := neueLaufzeit(t, PermStore)
	var out struct {
		Status  int    `json:"status"`
		Meldung string `json:"meldung"`
	}
	if err := r.Dispatch(context.Background(), "echo", HookEvent, 1,
		EventIn{Name: "test", Data: map[string]string{"tue": "verboten"}}, &out); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if out.Status != StatusDenied {
		t.Errorf("Status %d, erwartet %d (verweigert)", out.Status, StatusDenied)
	}
	// Die Meldung muss sagen, welche Berechtigung fehlt — es ist ein Fehler im
	// Manifest, nicht im Code, und der Autor soll ihn ohne Raten finden.
	if !strings.Contains(out.Meldung, PermSettings) {
		t.Errorf("die Meldung nennt die Berechtigung nicht: %q", out.Meldung)
	}
}

func TestUnbekannteOperationWirdGemeldet(t *testing.T) {
	r, _, _ := neueLaufzeit(t, PermStore)
	var out struct {
		Status  int    `json:"status"`
		Meldung string `json:"meldung"`
	}
	if err := r.Dispatch(context.Background(), "echo", HookEvent, 1,
		EventIn{Name: "test", Data: map[string]string{"tue": "unbekannt"}}, &out); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if out.Status != StatusError || !strings.Contains(out.Meldung, "gibtsnicht") {
		t.Errorf("Status %d, Meldung %q", out.Status, out.Meldung)
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
	// Der Name kommt vom Host und nicht aus der Nachricht: ein Plugin soll
	// keine Zeile schreiben können, die nach jemand anderem aussieht.
	if !strings.Contains(s, `plugin=echo`) || !strings.Contains(s, "etwas ist passiert") {
		t.Errorf("Protokoll: %s", s)
	}
}

func TestKurzerPufferWirdNachgefordert(t *testing.T) {
	r, s, _ := neueLaufzeit(t, PermStore)
	ctx := context.Background()
	// Grösser als der 4096-Byte-Puffer des Gasts, damit er die Grösse
	// nachfragen und noch einmal fragen muss.
	gross := strings.Repeat("x", 20000)
	if err := s.StoreSet(ctx, "echo", 3, "gross", gross); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Laenge int `json:"laenge"`
	}
	if err := r.Dispatch(ctx, "echo", HookEvent, 3,
		EventIn{Name: "test", WebsiteID: 3, Data: map[string]string{"tue": "grosse-antwort", "key": "gross"}},
		&out); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if out.Laenge < len(gross) {
		t.Errorf("nur %d Bytes zurückbekommen, erwartet mindestens %d", out.Laenge, len(gross))
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
	// Ohne Zeitgrenze hält so ein Modul die Verbindung, dann die nächste, und
	// die Website antwortet nicht mehr — ohne eine Zeile im Protokoll.
	if dauer > 3*CallTimeout {
		t.Errorf("der Abbruch dauerte %v, Grenze ist %v", dauer, CallTimeout)
	}
	// Der Grund muss beim Plugin vermerkt sein, nicht nur im Protokoll von vor
	// drei Neustarts.
	p, _ := s.Get(ctx, "echo")
	if p.LastError == "" {
		t.Error("der Abbruch wurde beim Plugin nicht vermerkt")
	}
}

func TestUnsinnigeAntwortBrichtDieAnfrageNicht(t *testing.T) {
	r, s, _ := neueLaufzeit(t, PermStore)
	var out ContentOut
	err := r.Dispatch(context.Background(), "echo", HookEvent, 1,
		EventIn{Name: "test", Data: map[string]string{"tue": "muell"}}, &out)
	if err == nil {
		t.Fatal("ungültiges JSON wurde angenommen")
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Errorf("die Meldung sagt nicht, was falsch war: %v", err)
	}
	if p, _ := s.Get(context.Background(), "echo"); p.LastError == "" {
		t.Error("der Fehler wurde beim Plugin nicht vermerkt")
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

	m := gutesManifest()
	// Ein gültiges, aber leeres Modul: der Kopf stimmt, die Exporte fehlen.
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
		t.Fatal("nach Load nicht geladen")
	}
	r.Unload(ctx, "echo")
	if r.Loaded("echo") {
		t.Error("nach Unload noch geladen")
	}
	if err := r.Dispatch(ctx, "echo", HookContent, 1, ContentIn{}, &ContentOut{}); err == nil {
		t.Error("ein entladenes Plugin wurde aufgerufen")
	}
}
