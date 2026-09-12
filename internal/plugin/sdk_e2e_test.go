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
	"github.com/holzcloud/holzcloud-cms/internal/plugin/wasmtest"
)

// The example plugin from plugins/jahreszahl, built with the SDK, against the
// real runtime. It checks the chain as a whole: SDK, calling convention, host,
// permissions and the plugin's own store.
func TestBeispielPluginLaeuftDurch(t *testing.T) {
	module := wasmtest.Module(t, "../../plugins/jahreszahl/plugin.wasm")
	raw, err := os.ReadFile("../../plugins/jahreszahl/plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := plugin.ParseManifest(raw)
	if err != nil {
		t.Fatalf("the shipped manifest is invalid: %v", err)
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
	if err := store.Install(ctx, &plugin.Package{Manifest: m, Module: module, SHA256: strings.Repeat("b", 64)}); err != nil {
		t.Fatal(err)
	}
	r, err := plugin.NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close(ctx)
	if err := r.Load(ctx, m, module); err != nil {
		t.Fatalf("Load: %v", err)
	}

	jahr := time.Now().Format("2006")

	// A page without the token stays untouched.
	var out plugin.ContentOut
	if err := r.Dispatch(ctx, m.ID, plugin.HookContent, 1,
		plugin.ContentIn{WebsiteID: 1, HTML: "<p>nichts</p>"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Changed {
		t.Errorf("a page with no marker was changed: %+v", out)
	}

	// A page with the token gets the year.
	out = plugin.ContentOut{}
	if err := r.Dispatch(ctx, m.ID, plugin.HookContent, 1,
		plugin.ContentIn{WebsiteID: 1, HTML: "<footer>© [[jahr]] Velowerkstatt</footer>"}, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Changed || !strings.Contains(out.HTML, jahr) || strings.Contains(out.HTML, "[[jahr]]") {
		t.Fatalf("the marker was not replaced: %+v", out)
	}

	// Its own store has counted along.
	if v, ok, err := store.StoreGet(ctx, m.ID, 1, "ersetzungen"); err != nil || !ok || v != "1" {
		t.Errorf("counter: %q (%v, %v)", v, ok, err)
	}

	// And the admin hook renders.
	var admin plugin.AdminOut
	if err := r.Dispatch(ctx, m.ID, plugin.HookAdmin, 1,
		plugin.AdminIn{WebsiteID: 1, Method: "GET"}, &admin); err != nil {
		t.Fatal(err)
	}
	if admin.Title != "Jahreszahl" || !strings.Contains(admin.HTML, "Bisher ersetzt") {
		t.Errorf("Admin-Bildschirm: %+v", admin)
	}
	if !strings.Contains(admin.HTML, ">1<") {
		t.Errorf("the counter is not on the screen: %s", admin.HTML)
	}
	_ = json.Marshal
}
