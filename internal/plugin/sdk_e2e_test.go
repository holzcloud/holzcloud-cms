package plugin_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// Das Beispiel-Plugin aus plugins/jahreszahl, mit dem SDK gebaut, gegen die
// echte Laufzeit. Es prüft die Kette als Ganzes: SDK, Aufrufkonvention, Host,
// Berechtigungen und eigener Speicher.
func TestBeispielPluginLaeuftDurch(t *testing.T) {
	modul, err := os.ReadFile("../../plugins/jahreszahl/plugin.wasm")
	if err != nil {
		t.Skipf("plugins/jahreszahl/plugin.wasm fehlt: %v", err)
	}
	roh, err := os.ReadFile("../../plugins/jahreszahl/plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := plugin.ParseManifest(roh)
	if err != nil {
		t.Fatalf("das mitgelieferte Manifest ist ungültig: %v", err)
	}

	database, err := db.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatal(err)
	}
	store := plugin.NewStore(database)

	ctx := context.Background()
	if err := store.Install(ctx, &plugin.Package{Manifest: m, Module: modul, SHA256: strings.Repeat("b", 64)}); err != nil {
		t.Fatal(err)
	}
	r, err := plugin.NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close(ctx)
	if err := r.Load(ctx, m, modul); err != nil {
		t.Fatalf("Load: %v", err)
	}

	jahr := time.Now().Format("2006")

	// Eine Seite ohne die Marke bleibt unangetastet.
	var out plugin.ContentOut
	if err := r.Dispatch(ctx, m.ID, plugin.HookContent, 1,
		plugin.ContentIn{WebsiteID: 1, HTML: "<p>nichts</p>"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Changed {
		t.Errorf("eine Seite ohne Marke wurde verändert: %+v", out)
	}

	// Eine Seite mit der Marke bekommt das Jahr.
	out = plugin.ContentOut{}
	if err := r.Dispatch(ctx, m.ID, plugin.HookContent, 1,
		plugin.ContentIn{WebsiteID: 1, HTML: "<footer>© [[jahr]] Velowerkstatt</footer>"}, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Changed || !strings.Contains(out.HTML, jahr) || strings.Contains(out.HTML, "[[jahr]]") {
		t.Fatalf("die Marke wurde nicht ersetzt: %+v", out)
	}

	// Der eigene Speicher hat mitgezählt.
	if v, ok, err := store.StoreGet(ctx, m.ID, 1, "ersetzungen"); err != nil || !ok || v != "1" {
		t.Errorf("Zähler: %q (%v, %v)", v, ok, err)
	}

	// Und der Admin-Haken rendert.
	var admin plugin.AdminOut
	if err := r.Dispatch(ctx, m.ID, plugin.HookAdmin, 1,
		plugin.AdminIn{WebsiteID: 1, Method: "GET"}, &admin); err != nil {
		t.Fatal(err)
	}
	if admin.Title != "Jahreszahl" || !strings.Contains(admin.HTML, "Bisher ersetzt") {
		t.Errorf("Admin-Bildschirm: %+v", admin)
	}
	if !strings.Contains(admin.HTML, ">1<") {
		t.Errorf("der Zähler steht nicht auf dem Bildschirm: %s", admin.HTML)
	}
	_ = json.Marshal
}
