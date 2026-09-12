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

// echoArchive packs the test module as an uploadable zip.
func echoArchive(t *testing.T, anpassen func(*Manifest)) []byte {
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
	w.Write(echoModule(t))
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
	einspielen(t, m, echoArchive(t, nil))

	// Installed means off. Look first, then switch on.
	st, err := m.Get(ctx, "echo")
	if err != nil {
		t.Fatal(err)
	}
	if st.Enabled || st.Running {
		t.Fatalf("freshly installed and already active: %+v", st)
	}
	// And switched off it touches no page.
	if got := m.FilterContent(ctx, site, ContentIn{WebsiteID: site, Slug: "home", Title: "Start", HTML: "<p>x</p>"}); got != "<p>x</p>" {
		t.Errorf("ein ausgeschaltetes Plugin hat gefiltert: %q", got)
	}

	if err := m.Enable(ctx, "echo"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	// Switched on but assigned to no website yet: still nothing.
	if got := m.FilterContent(ctx, site, ContentIn{WebsiteID: site, Slug: "home", Title: "Start", HTML: "<p>x</p>"}); got != "<p>x</p>" {
		t.Errorf("filtering happened with no mapping: %q", got)
	}

	if err := m.SetWebsites(ctx, "echo", []int64{site}); err != nil {
		t.Fatal(err)
	}
	if got := m.FilterContent(ctx, site, ContentIn{WebsiteID: site, Slug: "home", Title: "Start", HTML: "<p>x</p>"}); got != "<p>x</p><!-- echo -->" {
		t.Errorf("the page was not filtered: %q", got)
	}
	// Another website stays untouched.
	if got := m.FilterContent(ctx, site+1, ContentIn{WebsiteID: site + 1, Slug: "home", Title: "Start", HTML: "<p>x</p>"}); got != "<p>x</p>" {
		t.Errorf("another website was filtered: %q", got)
	}

	// The extra file lies on disk, under the plugin.
	if b, err := os.ReadFile(m.AssetPath("echo", "stil.css")); err != nil || string(b) != ".echo{}" {
		t.Errorf("Beigabe: %q %v", b, err)
	}
}

func TestAusschaltenUndEntfernen(t *testing.T) {
	m, store, site := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchive(t, nil))
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
		t.Error("it still runs after being switched off")
	}
	if got := m.FilterContent(ctx, site, ContentIn{WebsiteID: site, Slug: "home", Title: "", HTML: "<p>x</p>"}); got != "<p>x</p>" {
		t.Errorf("filtering happened after it was switched off: %q", got)
	}

	// Something in its own store, so that removing has something to take along.
	if err := store.StoreSet(ctx, "echo", site, "farbe", "grün"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(m.root(), "echo")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("the directory is missing before the removal: %v", err)
	}

	if err := m.Remove(ctx, "echo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("das Verzeichnis blieb liegen")
	}
	if _, err := m.Get(ctx, "echo"); err == nil {
		t.Error("the plugin is still in the database")
	}
}

func TestNeustartLaedtWasEingeschaltetIst(t *testing.T) {
	m, store, site := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchive(t, nil))
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
		t.Errorf("after the restart it does not filter: %q", got)
	}
}

func TestFehlendesModulIstKeinAbsturz(t *testing.T) {
	m, store, _ := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchive(t, nil))
	if err := m.Enable(ctx, "echo"); err != nil {
		t.Fatal(err)
	}

	// Somebody has deleted the file.
	if err := os.Remove(filepath.Join(m.root(), "echo", ModuleName)); err != nil {
		t.Fatal(err)
	}
	rt2, err := NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer rt2.Close(ctx)

	// A broken module must not stop a server that serves four other websites
	// from starting.
	m2, err := NewManager(ctx, store, rt2, m.dataDir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("ein fehlendes Modul hat den Start verhindert: %v", err)
	}
	st, _ := m2.Get(ctx, "echo")
	if st.Running {
		t.Error("it runs although the module is missing")
	}
	if st.LastError == "" {
		t.Error("the reason is nowhere")
	}
}

func TestEinschaltenMeldetWennEsNichtHochkommt(t *testing.T) {
	m, _, _ := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchive(t, nil))
	if err := os.Remove(filepath.Join(m.root(), "echo", ModuleName)); err != nil {
		t.Fatal(err)
	}
	// No green tick for something that is not running.
	if err := m.Enable(ctx, "echo"); err == nil {
		t.Error("Enable meldete Erfolg, obwohl das Modul fehlt")
	}
}

func TestEreignisErreichtDasPlugin(t *testing.T) {
	m, store, site := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchive(t, nil))
	if err := m.Enable(ctx, "echo"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetWebsites(ctx, "echo", []int64{site}); err != nil {
		t.Fatal(err)
	}

	m.Emit("test", site, map[string]string{"tue": "schreiben", "key": "k", "value": "v"})

	// Emit does not wait — a plugin that learns slowly about a saved page must
	// not make saving slow.
	var gefunden bool
	for i := 0; i < 100 && !gefunden; i++ {
		if _, ok, _ := store.StoreGet(ctx, "echo", site, "k"); ok {
			gefunden = true
			break
		}
		waitABit()
	}
	if !gefunden {
		t.Error("the event did not arrive")
	}
}

func TestVerwaltungsBildschirmUndSeitenleiste(t *testing.T) {
	m, _, site := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchive(t, nil))
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
	// The test module does not answer "admin", so it hands back nothing. That
	// must not be a fault.
	if _, err := m.Admin(ctx, "echo", AdminIn{WebsiteID: site, Method: "GET"}); err != nil {
		t.Errorf("Admin: %v", err)
	}
}

func TestAdressenWerdenNurVomBesitzerBedient(t *testing.T) {
	m, _, site := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchive(t, func(mf *Manifest) {
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
	// Not on a website the plugin is not assigned to.
	if _, ok := m.RouteOwner("/echo", site+1); ok {
		t.Error("die Adresse gilt auf einer fremden Website")
	}
	if _, ok := m.RouteOwner("/etwas-anderes", site); ok {
		t.Error("an unclaimed address has an owner")
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
		t.Errorf("an escaping key yielded a path: %s", p)
	}
}

func TestZweiPluginsFilternInStabilerReihenfolge(t *testing.T) {
	m, _, site := neuerManager(t)
	ctx := context.Background()
	for _, id := range []string{"bbb", "aaa"} {
		einspielen(t, m, echoArchive(t, func(mf *Manifest) { mf.ID = id; mf.Name = id }))
		if err := m.Enable(ctx, id); err != nil {
			t.Fatal(err)
		}
		if err := m.SetWebsites(ctx, id, []int64{site}); err != nil {
			t.Fatal(err)
		}
	}
	// Both append the same token; what matters is that it happens twice and is
	// the same on every run.
	erste := m.FilterContent(ctx, site, ContentIn{WebsiteID: site, Slug: "home", Title: "", HTML: "<p>x</p>"})
	if strings.Count(erste, "<!-- echo -->") != 2 {
		t.Fatalf("not both filtered: %q", erste)
	}
	for i := 0; i < 5; i++ {
		if got := m.FilterContent(ctx, site, ContentIn{WebsiteID: site, Slug: "home", Title: "", HTML: "<p>x</p>"}); got != erste {
			t.Fatalf("die Reihenfolge schwankt: %q gegen %q", got, erste)
		}
	}
}

// waitABit is a short sleep, so that the intent stays readable in the test.
func waitABit() { time.Sleep(10 * time.Millisecond) }
