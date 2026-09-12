package plugin_test

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/plugin/wasmtest"
)

// The farm shop from plugins/bestellung, against the real runtime.
//
// It is the first plugin that reads a website's own fields, and thereby the
// proof that the chain holds: field in the admin, value on the page, value in
// the plugin, form on the website, order in the store.
func TestHofladenLaeuftDurch(t *testing.T) {
	module := wasmtest.Module(t, "../../plugins/bestellung/plugin.wasm")
	raw, err := os.ReadFile("../../plugins/bestellung/plugin.json")
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
	if err := store.Install(ctx, &plugin.Package{
		Manifest: m, Module: module, SHA256: strings.Repeat("c", 64),
	}); err != nil {
		t.Fatal(err)
	}

	// Two products and one that is sold out. They come from the host's page
	// function, as in operation — with their own fields.
	pages := func(_ context.Context, websiteID int64, q plugin.PagesQuery) (plugin.PagesResult, error) {
		if !q.WithFields {
			// The plugin has to ask for the fields expressly; if it did not, it
			// would never find a product, and this test should notice that.
			return plugin.PagesResult{}, nil
		}
		return plugin.PagesResult{Pages: []plugin.PageInfo{
			{ID: 1, Slug: "seife", Title: "Schafmilchseife",
				Fields: map[string]string{"preis": "8,50", "einheit": "Stück", "verfuegbarkeit": "frisch"}},
			{ID: 2, Slug: "joghurt", Title: "Joghurt",
				Fields: map[string]string{"preis": "7,00", "einheit": "Glas", "verfuegbarkeit": "frisch"}},
			{ID: 3, Slug: "wolle", Title: "Rohwolle",
				Fields: map[string]string{"preis": "12,00", "verfuegbarkeit": "vergriffen"}},
			// A page without a price is no product.
			{ID: 4, Slug: "hof", Title: "Der Hof"},
		}, Total: 4}, nil
	}

	var verschickt []string
	r, err := plugin.NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	r.WithPages(pages)
	r.WithNotify(func(_ context.Context, _ int64, a plugin.NotifyArg) (bool, string, error) {
		verschickt = append(verschickt, a.Subject+"\n"+a.Body)
		return true, "", nil
	})
	defer r.Close(ctx)
	if err := r.Load(ctx, m, module); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// A page without the token stays untouched.
	var out plugin.ContentOut
	if err := r.Dispatch(ctx, m.ID, plugin.HookContent, 1,
		plugin.ContentIn{WebsiteID: 1, HTML: "<p>nichts</p>"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Changed {
		t.Errorf("a page with no marker was changed: %+v", out)
	}

	// With the token the form stands there — with the orderable products and
	// without a quantity field for the one that is sold out.
	out = plugin.ContentOut{}
	if err := r.Dispatch(ctx, m.ID, plugin.HookContent, 1,
		plugin.ContentIn{WebsiteID: 1, Slug: "bestellen", HTML: "<p>[[bestellung]]</p>"}, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Changed {
		t.Fatal("the marker was not replaced")
	}
	for _, wanted := range []string{
		`name="menge_seife"`, `name="menge_joghurt"`,
		"Schafmilchseife", "8,50", "Rohwolle", "nicht bestellbar",
	} {
		if !strings.Contains(out.HTML, wanted) {
			t.Errorf("%q is missing from the form", wanted)
		}
	}
	if strings.Contains(out.HTML, `name="menge_wolle"`) {
		t.Error("das vergriffene Produkt hat ein Mengenfeld bekommen")
	}
	// The page "Der Hof" has no price and is therefore no product.
	if strings.Contains(out.HTML, "Der Hof") {
		t.Error("a page with no price is in the product list")
	}
	// A paragraph around the form would be invalid HTML.
	if strings.Contains(out.HTML, "<p><div") {
		t.Error("the form is stuck inside a paragraph")
	}

	timeToken := between(out.HTML, `name="gestellt" value="`, `"`)
	if timeToken == "" {
		t.Fatal("keine Zeitmarke im Formular")
	}

	// A form submitted at once is refused: a human needs longer than two
	// seconds. Check first, then wait — otherwise the rest of the test would run
	// into that refusal and would pass for the wrong reasons.
	antwortSofort := submit(t, r, ctx, m.ID, url.Values{
		"seite": {"bestellen"}, "gestellt": {timeToken},
		"name": {"Anna"}, "email": {"anna@example.ch"}, "menge_seife": {"1"},
	})
	if !strings.Contains(antwortSofort.Location, "has+expired") {
		t.Errorf("ein sofort abgeschicktes Formular kam durch: %+v", antwortSofort)
	}
	time.Sleep(2100 * time.Millisecond)

	// Without a quantity nothing is accepted.
	antwort := submit(t, r, ctx, m.ID, url.Values{
		"seite": {"bestellen"}, "gestellt": {timeToken},
		"name": {"Anna"}, "email": {"anna@example.ch"},
	})
	if !strings.Contains(antwort.Location, "at+least+one+product") {
		t.Errorf("an order with no quantity was accepted: %+v", antwort)
	}

	// A filled honeypot looks like a success and is discarded.
	antwort = submit(t, r, ctx, m.ID, url.Values{
		"seite": {"bestellen"}, "gestellt": {timeToken},
		"name": {"Bot"}, "email": {"bot@example.ch"},
		"menge_seife": {"1"}, "website": {"https://spam.example"},
	})
	if !strings.Contains(antwort.Location, "bestellung=gesendet") {
		t.Errorf("the honeypot gave itself away: %+v", antwort)
	}

	// An invented time token is refused: otherwise it would be no protection.
	antwort = submit(t, r, ctx, m.ID, url.Values{
		"seite": {"bestellen"}, "gestellt": {"1700000000.erfunden"},
		"name": {"Anna"}, "email": {"anna@example.ch"}, "menge_seife": {"1"},
	})
	if !strings.Contains(antwort.Location, "has+expired") {
		t.Errorf("eine erfundene Zeitmarke kam durch: %+v", antwort)
	}

	// Und die richtige Bestellung.
	antwort = submit(t, r, ctx, m.ID, url.Values{
		"seite": {"bestellen"}, "gestellt": {timeToken},
		"name": {"Anna Muster"}, "email": {"anna@example.ch"},
		"telefon":     {"079 123 45 67"},
		"menge_seife": {"3"}, "menge_joghurt": {"2"},
	})
	if !strings.Contains(antwort.Location, "bestellung=gesendet") {
		t.Fatalf("the order was not accepted: %+v", antwort)
	}

	// The operator was notified, and the total is right: 3×8.50 + 2×7.00.
	if len(verschickt) != 1 {
		t.Fatalf("%d Benachrichtigungen, want 1", len(verschickt))
	}
	if !strings.Contains(verschickt[0], "39,50") {
		t.Errorf("the total is missing or wrong:\n%s", verschickt[0])
	}
	if !strings.Contains(verschickt[0], "Anna Muster") {
		t.Errorf("der Name fehlt:\n%s", verschickt[0])
	}

	// The order lies in the plugin's store — and only the one, because the
	// honeypot attempt was discarded.
	values, err := store.StoreList(ctx, m.ID, 1, "bestellung:", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 {
		t.Fatalf("%d Bestellungen gespeichert, want 1", len(values))
	}

	// And it stands on the screen in the admin.
	var admin plugin.AdminOut
	if err := r.Dispatch(ctx, m.ID, plugin.HookAdmin, 1,
		plugin.AdminIn{WebsiteID: 1, Method: "GET"}, &admin); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(admin.HTML, "Anna Muster") || !strings.Contains(admin.HTML, "39,50") {
		t.Errorf("die Bestellung fehlt in der Verwaltung: %s", admin.HTML)
	}
}

// submit sends an order through the route hook.
func submit(t *testing.T, r *plugin.Runtime, ctx context.Context, id string, form url.Values) plugin.RequestOut {
	t.Helper()
	var out plugin.RequestOut
	if err := r.Dispatch(ctx, id, plugin.HookRoute, 1, plugin.RequestIn{
		WebsiteID: 1, Method: "POST", Path: "/bestellung", Body: form.Encode(),
	}, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// between reads the value between two strings.
func between(s, vor, nach string) string {
	i := strings.Index(s, vor)
	if i < 0 {
		return ""
	}
	rest := s[i+len(vor):]
	j := strings.Index(rest, nach)
	if j < 0 {
		return ""
	}
	return rest[:j]
}
