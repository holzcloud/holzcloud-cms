package plugin

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// echoArchiv packt das Prüfmodul als hochladbares Zip.
func echoArchiv(t *testing.T, anpassen func(*Manifest)) []byte {
	t.Helper()
	m := gutesManifest()
	m.ID = "echo"
	m.Name = "Echo"
	m.Hooks = []string{HookContent, HookEvent, HookAdmin}
	m.Permissions = []string{PermStore, PermLog}
	m.Admin = &AdminEntry{Label: "Echo", PerWebsite: true}
	if anpassen != nil {
		anpassen(&m)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create(ManifestName)
	json.NewEncoder(w).Encode(m)
	w, _ = zw.Create(ModuleName)
	w.Write(echoModul(t))
	w, _ = zw.Create(AssetDir + "stil.css")
	w.Write([]byte(".echo{}"))
	zw.Close()
	return buf.Bytes()
}

func neuerManager(t *testing.T) (*Manager, *Store, int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatal(err)
	}
	ws, err := domain.NewStore(database).CreateWebsite(context.Background(), "Velowerkstatt", "")
	if err != nil {
		t.Fatal(err)
	}

	store := NewStore(database)
	ctx := context.Background()
	rt, err := NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rt.Close(context.Background()) })

	m, err := NewManager(ctx, store, rt, dir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return m, store, ws.ID
}

func einspielen(t *testing.T, m *Manager, a []byte) *Manifest {
	t.Helper()
	man, err := m.Install(context.Background(), bytes.NewReader(a), int64(len(a)))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	return man
}

func TestGanzerWegVomZipBisZurSeite(t *testing.T) {
	m, _, site := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchiv(t, nil))

	// Eingespielt heisst aus. Erst schauen, dann einschalten.
	st, err := m.Get(ctx, "echo")
	if err != nil {
		t.Fatal(err)
	}
	if st.Enabled || st.Running {
		t.Fatalf("frisch eingespielt und schon aktiv: %+v", st)
	}
	// Und ausgeschaltet fasst es keine Seite an.
	if got := m.FilterContent(ctx, site, ContentIn{WebsiteID: site, Slug: "home", Title: "Start", HTML: "<p>x</p>"}); got != "<p>x</p>" {
		t.Errorf("ein ausgeschaltetes Plugin hat gefiltert: %q", got)
	}

	if err := m.Enable(ctx, "echo"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	// Eingeschaltet, aber noch keiner Website zugeordnet: immer noch nichts.
	if got := m.FilterContent(ctx, site, ContentIn{WebsiteID: site, Slug: "home", Title: "Start", HTML: "<p>x</p>"}); got != "<p>x</p>" {
		t.Errorf("ohne Zuordnung wurde gefiltert: %q", got)
	}

	if err := m.SetWebsites(ctx, "echo", []int64{site}); err != nil {
		t.Fatal(err)
	}
	if got := m.FilterContent(ctx, site, ContentIn{WebsiteID: site, Slug: "home", Title: "Start", HTML: "<p>x</p>"}); got != "<p>x</p><!-- echo -->" {
		t.Errorf("die Seite wurde nicht gefiltert: %q", got)
	}
	// Eine andere Website bleibt unberührt.
	if got := m.FilterContent(ctx, site+1, ContentIn{WebsiteID: site + 1, Slug: "home", Title: "Start", HTML: "<p>x</p>"}); got != "<p>x</p>" {
		t.Errorf("eine fremde Website wurde gefiltert: %q", got)
	}

	// Die Beigabe liegt auf der Platte, unter dem Plugin.
	if b, err := os.ReadFile(m.AssetPath("echo", "stil.css")); err != nil || string(b) != ".echo{}" {
		t.Errorf("Beigabe: %q %v", b, err)
	}
}

func TestAusschaltenUndEntfernen(t *testing.T) {
	m, store, site := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchiv(t, nil))
	if err := m.Enable(ctx, "echo"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetWebsites(ctx, "echo", []int64{site}); err != nil {
		t.Fatal(err)
	}

	if err := m.Disable(ctx, "echo"); err != nil {
		t.Fatal(err)
	}
	if st, _ := m.Get(ctx, "echo"); st.Running {
		t.Error("nach dem Ausschalten läuft es noch")
	}
	if got := m.FilterContent(ctx, site, ContentIn{WebsiteID: site, Slug: "home", Title: "", HTML: "<p>x</p>"}); got != "<p>x</p>" {
		t.Errorf("nach dem Ausschalten wurde gefiltert: %q", got)
	}

	// Etwas im eigenen Speicher, damit das Entfernen etwas mitzunehmen hat.
	if err := store.StoreSet(ctx, "echo", site, "farbe", "grün"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(m.root(), "echo")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("das Verzeichnis fehlt schon vor dem Entfernen: %v", err)
	}

	if err := m.Remove(ctx, "echo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("das Verzeichnis blieb liegen")
	}
	if _, err := m.Get(ctx, "echo"); err == nil {
		t.Error("das Plugin steht noch in der Datenbank")
	}
}

func TestNeustartLaedtWasEingeschaltetIst(t *testing.T) {
	m, store, site := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchiv(t, nil))
	if err := m.Enable(ctx, "echo"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetWebsites(ctx, "echo", []int64{site}); err != nil {
		t.Fatal(err)
	}

	// Ein zweiter Manager auf demselben Verzeichnis ist ein Neustart.
	rt2, err := NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer rt2.Close(ctx)
	m2, err := NewManager(ctx, store, rt2, m.dataDir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	if got := m2.FilterContent(ctx, site, ContentIn{WebsiteID: site, Slug: "home", Title: "", HTML: "<p>x</p>"}); got != "<p>x</p><!-- echo -->" {
		t.Errorf("nach dem Neustart filtert es nicht: %q", got)
	}
}

func TestFehlendesModulIstKeinAbsturz(t *testing.T) {
	m, store, _ := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchiv(t, nil))
	if err := m.Enable(ctx, "echo"); err != nil {
		t.Fatal(err)
	}

	// Jemand hat die Datei gelöscht.
	if err := os.Remove(filepath.Join(m.root(), "echo", ModuleName)); err != nil {
		t.Fatal(err)
	}
	rt2, err := NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer rt2.Close(ctx)

	// Ein kaputtes Modul darf einen Server, der vier andere Websites bedient,
	// nicht am Starten hindern.
	m2, err := NewManager(ctx, store, rt2, m.dataDir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("ein fehlendes Modul hat den Start verhindert: %v", err)
	}
	st, _ := m2.Get(ctx, "echo")
	if st.Running {
		t.Error("es läuft, obwohl das Modul fehlt")
	}
	if st.LastError == "" {
		t.Error("der Grund steht nirgends")
	}
}

func TestEinschaltenMeldetWennEsNichtHochkommt(t *testing.T) {
	m, _, _ := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchiv(t, nil))
	if err := os.Remove(filepath.Join(m.root(), "echo", ModuleName)); err != nil {
		t.Fatal(err)
	}
	// Kein grüner Haken für etwas, das nicht läuft.
	if err := m.Enable(ctx, "echo"); err == nil {
		t.Error("Enable meldete Erfolg, obwohl das Modul fehlt")
	}
}

func TestEreignisErreichtDasPlugin(t *testing.T) {
	m, store, site := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchiv(t, nil))
	if err := m.Enable(ctx, "echo"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetWebsites(ctx, "echo", []int64{site}); err != nil {
		t.Fatal(err)
	}

	m.Emit("test", site, map[string]string{"tue": "schreiben", "key": "k", "value": "v"})

	// Emit wartet nicht — ein Plugin, das langsam von einer gespeicherten
	// Seite erfährt, darf das Speichern nicht langsam machen.
	var gefunden bool
	for i := 0; i < 100 && !gefunden; i++ {
		if _, ok, _ := store.StoreGet(ctx, "echo", site, "k"); ok {
			gefunden = true
			break
		}
		waitABit()
	}
	if !gefunden {
		t.Error("das Ereignis kam nicht an")
	}
}

func TestVerwaltungsBildschirmUndSeitenleiste(t *testing.T) {
	m, _, site := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchiv(t, nil))
	if err := m.Enable(ctx, "echo"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetWebsites(ctx, "echo", []int64{site}); err != nil {
		t.Fatal(err)
	}

	links := m.AdminLinks()
	if len(links) != 1 || links[0].Label != "Echo" || !links[0].PerWebsite {
		t.Errorf("Seitenleiste: %+v", links)
	}
	// Das Prüfmodul beantwortet "admin" nicht, gibt also nichts zurück. Das
	// darf kein Fehler sein.
	if _, err := m.Admin(ctx, "echo", AdminIn{WebsiteID: site, Method: "GET"}); err != nil {
		t.Errorf("Admin: %v", err)
	}
}

func TestAdressenWerdenNurVomBesitzerBedient(t *testing.T) {
	m, _, site := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchiv(t, func(mf *Manifest) {
		mf.Hooks = append(mf.Hooks, HookRoute)
		mf.Routes = []string{"/echo"}
	}))
	if err := m.Enable(ctx, "echo"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetWebsites(ctx, "echo", []int64{site}); err != nil {
		t.Fatal(err)
	}

	if _, ok := m.RouteOwner("/echo", site); !ok {
		t.Error("die beanspruchte Adresse hat keinen Besitzer")
	}
	// Nicht auf einer Website, der das Plugin nicht zugeordnet ist.
	if _, ok := m.RouteOwner("/echo", site+1); ok {
		t.Error("die Adresse gilt auf einer fremden Website")
	}
	if _, ok := m.RouteOwner("/etwas-anderes", site); ok {
		t.Error("eine nicht beanspruchte Adresse hat einen Besitzer")
	}
}

func TestBeigabenPfadKannNichtEntkommen(t *testing.T) {
	m, _, _ := neuerManager(t)
	for _, name := range []string{"../plugin.wasm", "../../t.sqlite", "/etc/passwd", "a/../../x"} {
		if p := m.AssetPath("echo", name); p != "" {
			t.Errorf("%q ergab einen Pfad: %s", name, p)
		}
	}
	if p := m.AssetPath("../andere", "stil.css"); p != "" {
		t.Errorf("eine entkommende Kennung ergab einen Pfad: %s", p)
	}
}

func TestZweiPluginsFilternInStabilerReihenfolge(t *testing.T) {
	m, _, site := neuerManager(t)
	ctx := context.Background()
	for _, id := range []string{"bbb", "aaa"} {
		einspielen(t, m, echoArchiv(t, func(mf *Manifest) { mf.ID = id; mf.Name = id }))
		if err := m.Enable(ctx, id); err != nil {
			t.Fatal(err)
		}
		if err := m.SetWebsites(ctx, id, []int64{site}); err != nil {
			t.Fatal(err)
		}
	}
	// Beide hängen dieselbe Marke an; entscheidend ist, dass es zweimal
	// passiert und bei jedem Lauf gleich.
	erste := m.FilterContent(ctx, site, ContentIn{WebsiteID: site, Slug: "home", Title: "", HTML: "<p>x</p>"})
	if strings.Count(erste, "<!-- echo -->") != 2 {
		t.Fatalf("nicht beide haben gefiltert: %q", erste)
	}
	for i := 0; i < 5; i++ {
		if got := m.FilterContent(ctx, site, ContentIn{WebsiteID: site, Slug: "home", Title: "", HTML: "<p>x</p>"}); got != erste {
			t.Fatalf("die Reihenfolge schwankt: %q gegen %q", got, erste)
		}
	}
}

// waitABit ist ein kurzer Schlaf, damit die Absicht im Test lesbar bleibt.
func waitABit() { time.Sleep(10 * time.Millisecond) }
