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

// Der Hofladen aus plugins/bestellung, gegen die echte Laufzeit.
//
// Er ist das erste Plugin, das die eigenen Felder einer Website liest, und
// damit der Beweis, dass die Kette hält: Feld in der Verwaltung, Wert an der
// Seite, Wert im Plugin, Formular auf der Website, Bestellung im Speicher.
func TestHofladenLaeuftDurch(t *testing.T) {
	modul := wasmtest.Modul(t, "../../plugins/bestellung/plugin.wasm")
	roh, err := os.ReadFile("../../plugins/bestellung/plugin.json")
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
	if err := store.Install(ctx, &plugin.Package{
		Manifest: m, Module: modul, SHA256: strings.Repeat("c", 64),
	}); err != nil {
		t.Fatal(err)
	}

	// Zwei Produkte und eines, das vergriffen ist. Sie kommen aus der
	// Seitenfunktion des Hosts, wie im Betrieb — mit ihren eigenen Feldern.
	seiten := func(_ context.Context, websiteID int64, q plugin.PagesQuery) (plugin.PagesResult, error) {
		if !q.WithFields {
			// Das Plugin muss die Felder ausdrücklich anfordern; täte es das
			// nicht, fände es nie ein Produkt, und dieser Test soll das merken.
			return plugin.PagesResult{}, nil
		}
		return plugin.PagesResult{Pages: []plugin.PageInfo{
			{ID: 1, Slug: "seife", Title: "Schafmilchseife",
				Fields: map[string]string{"preis": "8,50", "einheit": "Stück", "verfuegbarkeit": "frisch"}},
			{ID: 2, Slug: "joghurt", Title: "Joghurt",
				Fields: map[string]string{"preis": "7,00", "einheit": "Glas", "verfuegbarkeit": "frisch"}},
			{ID: 3, Slug: "wolle", Title: "Rohwolle",
				Fields: map[string]string{"preis": "12,00", "verfuegbarkeit": "vergriffen"}},
			// Eine Seite ohne Preis ist kein Produkt.
			{ID: 4, Slug: "hof", Title: "Der Hof"},
		}, Total: 4}, nil
	}

	var verschickt []string
	r, err := plugin.NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	r.WithPages(seiten)
	r.WithNotify(func(_ context.Context, _ int64, a plugin.NotifyArg) (bool, string, error) {
		verschickt = append(verschickt, a.Subject+"\n"+a.Body)
		return true, "", nil
	})
	defer r.Close(ctx)
	if err := r.Load(ctx, m, modul); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Eine Seite ohne die Marke bleibt unangetastet.
	var out plugin.ContentOut
	if err := r.Dispatch(ctx, m.ID, plugin.HookContent, 1,
		plugin.ContentIn{WebsiteID: 1, HTML: "<p>nichts</p>"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Changed {
		t.Errorf("eine Seite ohne Marke wurde verändert: %+v", out)
	}

	// Mit Marke steht dort das Formular — mit den bestellbaren Produkten und
	// ohne ein Mengenfeld für das vergriffene.
	out = plugin.ContentOut{}
	if err := r.Dispatch(ctx, m.ID, plugin.HookContent, 1,
		plugin.ContentIn{WebsiteID: 1, Slug: "bestellen", HTML: "<p>[[bestellung]]</p>"}, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Changed {
		t.Fatal("die Marke wurde nicht ersetzt")
	}
	for _, nötig := range []string{
		`name="menge_seife"`, `name="menge_joghurt"`,
		"Schafmilchseife", "8,50", "Rohwolle", "nicht bestellbar",
	} {
		if !strings.Contains(out.HTML, nötig) {
			t.Errorf("%q fehlt im Formular", nötig)
		}
	}
	if strings.Contains(out.HTML, `name="menge_wolle"`) {
		t.Error("das vergriffene Produkt hat ein Mengenfeld bekommen")
	}
	// Die Seite „Der Hof“ hat keinen Preis und ist deshalb kein Produkt.
	if strings.Contains(out.HTML, "Der Hof") {
		t.Error("eine Seite ohne Preis steht in der Produktliste")
	}
	// Ein Absatz um das Formular wäre ungültiges HTML.
	if strings.Contains(out.HTML, "<p><div") {
		t.Error("das Formular steckt in einem Absatz")
	}

	zeitmarke := zwischen(out.HTML, `name="gestellt" value="`, `"`)
	if zeitmarke == "" {
		t.Fatal("keine Zeitmarke im Formular")
	}

	// Ein sofort abgeschicktes Formular wird abgelehnt: ein Mensch braucht
	// länger als zwei Sekunden. Erst prüfen, dann warten — sonst liefe der Rest
	// des Tests in diese Ablehnung und bestünde aus den falschen Gründen.
	antwortSofort := absenden(t, r, ctx, m.ID, url.Values{
		"seite": {"bestellen"}, "gestellt": {zeitmarke},
		"name": {"Anna"}, "email": {"anna@example.ch"}, "menge_seife": {"1"},
	})
	if !strings.Contains(antwortSofort.Location, "abgelaufen") {
		t.Errorf("ein sofort abgeschicktes Formular kam durch: %+v", antwortSofort)
	}
	time.Sleep(2100 * time.Millisecond)

	// Ohne Menge wird nichts angenommen.
	antwort := absenden(t, r, ctx, m.ID, url.Values{
		"seite": {"bestellen"}, "gestellt": {zeitmarke},
		"name": {"Anna"}, "email": {"anna@example.ch"},
	})
	if !strings.Contains(antwort.Location, "mindestens+einem+Produkt") {
		t.Errorf("eine Bestellung ohne Menge wurde angenommen: %+v", antwort)
	}

	// Ein gefüllter Honigtopf sieht wie ein Erfolg aus und wird verworfen.
	antwort = absenden(t, r, ctx, m.ID, url.Values{
		"seite": {"bestellen"}, "gestellt": {zeitmarke},
		"name": {"Bot"}, "email": {"bot@example.ch"},
		"menge_seife": {"1"}, "website": {"https://spam.example"},
	})
	if !strings.Contains(antwort.Location, "bestellung=gesendet") {
		t.Errorf("der Honigtopf hat sich verraten: %+v", antwort)
	}

	// Eine erfundene Zeitmarke wird abgelehnt: sonst wäre sie kein Schutz.
	antwort = absenden(t, r, ctx, m.ID, url.Values{
		"seite": {"bestellen"}, "gestellt": {"1700000000.erfunden"},
		"name": {"Anna"}, "email": {"anna@example.ch"}, "menge_seife": {"1"},
	})
	if !strings.Contains(antwort.Location, "abgelaufen") {
		t.Errorf("eine erfundene Zeitmarke kam durch: %+v", antwort)
	}

	// Und die richtige Bestellung.
	antwort = absenden(t, r, ctx, m.ID, url.Values{
		"seite": {"bestellen"}, "gestellt": {zeitmarke},
		"name": {"Anna Muster"}, "email": {"anna@example.ch"},
		"telefon":     {"079 123 45 67"},
		"menge_seife": {"3"}, "menge_joghurt": {"2"},
	})
	if !strings.Contains(antwort.Location, "bestellung=gesendet") {
		t.Fatalf("die Bestellung wurde nicht angenommen: %+v", antwort)
	}

	// Der Betreiber wurde benachrichtigt, und die Summe stimmt: 3×8,50 + 2×7,00.
	if len(verschickt) != 1 {
		t.Fatalf("%d Benachrichtigungen, want 1", len(verschickt))
	}
	if !strings.Contains(verschickt[0], "39,50") {
		t.Errorf("die Summe fehlt oder stimmt nicht:\n%s", verschickt[0])
	}
	if !strings.Contains(verschickt[0], "Anna Muster") {
		t.Errorf("der Name fehlt:\n%s", verschickt[0])
	}

	// Die Bestellung liegt im Speicher des Plugins — und nur die eine, denn der
	// Honigtopf-Versuch wurde verworfen.
	werte, err := store.StoreList(ctx, m.ID, 1, "bestellung:", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(werte) != 1 {
		t.Fatalf("%d Bestellungen gespeichert, want 1", len(werte))
	}

	// Und sie steht auf dem Bildschirm in der Verwaltung.
	var admin plugin.AdminOut
	if err := r.Dispatch(ctx, m.ID, plugin.HookAdmin, 1,
		plugin.AdminIn{WebsiteID: 1, Method: "GET"}, &admin); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(admin.HTML, "Anna Muster") || !strings.Contains(admin.HTML, "39,50") {
		t.Errorf("die Bestellung fehlt in der Verwaltung: %s", admin.HTML)
	}
}

// absenden schickt eine Bestellung durch den Routen-Haken.
func absenden(t *testing.T, r *plugin.Runtime, ctx context.Context, id string, form url.Values) plugin.RequestOut {
	t.Helper()
	var out plugin.RequestOut
	if err := r.Dispatch(ctx, id, plugin.HookRoute, 1, plugin.RequestIn{
		WebsiteID: 1, Method: "POST", Path: "/bestellung", Body: form.Encode(),
	}, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// zwischen liest den Wert zwischen zwei Zeichenketten.
func zwischen(s, vor, nach string) string {
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
