package plugin

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

func neuerSpeicher(t *testing.T) (*Store, *domain.Store) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return NewStore(database), domain.NewStore(database)
}

func paket(id string, routen ...string) *Package {
	m := gutesManifest()
	m.ID = id
	m.Name = strings.ToUpper(id[:1]) + id[1:]
	if len(routen) > 0 {
		m.Hooks = append(m.Hooks, HookRoute)
		m.Routes = routen
	}
	return &Package{Manifest: &m, Module: wasm, SHA256: strings.Repeat("a", 64)}
}

func TestEinspielenUndLesen(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()

	if err := s.Install(ctx, paket("weiterleitungen")); err != nil {
		t.Fatalf("Install: %v", err)
	}
	p, err := s.Get(ctx, "weiterleitungen")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// Ausgeschaltet ankommen: ein Plugin, das mit dem Hochladen zu laufen
	// beginnt, ist eines, das sich niemand vorher ansehen konnte.
	if p.Enabled {
		t.Error("frisch eingespielt und schon eingeschaltet")
	}
	if p.Manifest == nil || p.Manifest.ID != "weiterleitungen" {
		t.Errorf("Manifest kam nicht zurück: %+v", p.Manifest)
	}
	if _, err := s.Get(ctx, "gibtsnicht"); !errors.Is(err, ErrNotFound) {
		t.Errorf("erwartet ErrNotFound, bekommen: %v", err)
	}
}

func TestNeueFassungBehaeltEingeschaltetUndWebsites(t *testing.T) {
	s, dom := neuerSpeicher(t)
	ctx := context.Background()
	ws, err := dom.CreateWebsite(ctx, "Velowerkstatt", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Install(ctx, paket("suche")); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEnabled(ctx, "suche", true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetWebsites(ctx, "suche", []int64{ws.ID}); err != nil {
		t.Fatal(err)
	}

	// Dieselbe Kennung, neue Fassung.
	neu := paket("suche")
	neu.Manifest.Version = "2.0.0"
	if err := s.Install(ctx, neu); err != nil {
		t.Fatalf("Install (Aktualisierung): %v", err)
	}

	p, _ := s.Get(ctx, "suche")
	// Wer aktualisiert, erwartet, dass es weiterläuft — nicht, dass es sich
	// selbst abschaltet und die Zuordnung verliert.
	if !p.Enabled {
		t.Error("die Aktualisierung hat das Plugin abgeschaltet")
	}
	if len(p.Websites) != 1 || p.Websites[0] != ws.ID {
		t.Errorf("die Zuordnung ging verloren: %v", p.Websites)
	}
	if p.Version != "2.0.0" {
		t.Errorf("Fassung: %q", p.Version)
	}
}

func TestAdresseWirdNurEinmalVergeben(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()

	if err := s.Install(ctx, paket("suche", "/suche")); err != nil {
		t.Fatal(err)
	}
	err := s.Install(ctx, paket("andere-suche", "/suche"))
	if !errors.Is(err, ErrRouteTaken) {
		t.Fatalf("erwartet ErrRouteTaken, bekommen: %v", err)
	}
	// Die Meldung muss sagen, wem die Adresse gehört — sonst sucht der
	// Betreiber unter einem Dutzend Plugins.
	if !strings.Contains(err.Error(), "Suche") {
		t.Errorf("die Meldung nennt den Besitzer nicht: %v", err)
	}
	// Dieselbe Adresse in einer neuen Fassung desselben Plugins ist kein
	// Zusammenstoss mit sich selbst.
	if err := s.Install(ctx, paket("suche", "/suche")); err != nil {
		t.Errorf("die eigene Adresse wurde als vergeben gemeldet: %v", err)
	}
}

func TestEigenerSpeicherIstProPluginUndWebsiteGetrennt(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	for _, id := range []string{"eins", "zwei"} {
		if err := s.Install(ctx, paket(id)); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.StoreSet(ctx, "eins", 1, "farbe", "grün"); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreSet(ctx, "eins", 2, "farbe", "blau"); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreSet(ctx, "zwei", 1, "farbe", "rot"); err != nil {
		t.Fatal(err)
	}

	for _, f := range []struct {
		id   string
		site int64
		want string
	}{{"eins", 1, "grün"}, {"eins", 2, "blau"}, {"zwei", 1, "rot"}} {
		got, ok, err := s.StoreGet(ctx, f.id, f.site, "farbe")
		if err != nil || !ok || got != f.want {
			t.Errorf("%s/%d: %q (%v, %v), erwartet %q", f.id, f.site, got, ok, err, f.want)
		}
	}

	// Ein fehlender Schlüssel ist kein Fehler, sondern der erste Lauf.
	if _, ok, err := s.StoreGet(ctx, "eins", 1, "gibtsnicht"); err != nil || ok {
		t.Errorf("fehlender Schlüssel: ok=%v err=%v", ok, err)
	}
}

func TestSpeicherPraefixIstKeinMuster(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"a%b": "treffer", "axb": "kein treffer", "ab": "auch nicht"} {
		if err := s.StoreSet(ctx, "eins", 0, k, v); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.StoreList(ctx, "eins", 0, "a%", 100)
	if err != nil {
		t.Fatal(err)
	}
	// Ohne Maskierung wäre "a%" ein Muster und träfe alles.
	if len(got) != 1 || got["a%b"] != "treffer" {
		t.Errorf("das Präfix wurde als Muster gelesen: %v", got)
	}
}

func TestSpeicherGrenzen(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreSet(ctx, "eins", 0, strings.Repeat("k", MaxKeyBytes+1), "x"); err == nil {
		t.Error("ein zu langer Schlüssel wurde angenommen")
	}
	if err := s.StoreSet(ctx, "eins", 0, "gross", strings.Repeat("x", MaxValueBytes+1)); err == nil {
		t.Error("ein zu grosser Wert wurde angenommen")
	}
}

func TestEntfernenNimmtDieDatenMit(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreSet(ctx, "eins", 0, "farbe", "grün"); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(ctx, "eins"); err != nil {
		t.Fatal(err)
	}
	// Wieder eingespielt darf es nichts vom Vorgänger erben: die alte Fassung
	// kann die Daten in einer anderen Form gehalten haben.
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.StoreGet(ctx, "eins", 0, "farbe"); ok {
		t.Error("nach dem Entfernen und Wiedereinspielen war der alte Wert noch da")
	}
	if err := s.Remove(ctx, "gibtsnicht"); !errors.Is(err, ErrNotFound) {
		t.Errorf("erwartet ErrNotFound, bekommen: %v", err)
	}
}

func TestEigeneMigrationenLaufenEinmal(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	ms := []Migration{{Name: "0001.sql", SQL: `CREATE TABLE plugin_eins_daten (a TEXT) STRICT;`}}

	if err := s.ApplyMigrations(ctx, "eins", ms); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	// Ein zweiter Lauf darf nicht an "table already exists" scheitern.
	if err := s.ApplyMigrations(ctx, "eins", ms); err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}

	// Geänderte SQL unter altem Namen: sonst passiert nichts, und das Schema
	// passt still nicht mehr zu dem Code, der es erwartet.
	geaendert := []Migration{{Name: "0001.sql", SQL: `CREATE TABLE anders (b TEXT) STRICT;`}}
	err := s.ApplyMigrations(ctx, "eins", geaendert)
	if err == nil || !strings.Contains(err.Error(), "geändert") {
		t.Errorf("eine geänderte Migration wurde nicht gemeldet: %v", err)
	}
}

func TestFehlgeschlageneMigrationLaesstNichtsHalbesZurueck(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	ms := []Migration{{Name: "0001.sql", SQL: `
		CREATE TABLE plugin_eins_a (x TEXT) STRICT;
		DAS IST KEIN SQL;`}}
	if err := s.ApplyMigrations(ctx, "eins", ms); err == nil {
		t.Fatal("fehlerhaftes SQL wurde angenommen")
	}
	// Weder die Tabelle noch der Eintrag dürfen stehen geblieben sein, sonst
	// meldet der nächste Start "bereits angewendet" für etwas, das nie lief.
	var n int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM plugin_migrations WHERE plugin_id = 'eins'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d Migrationen als angewendet vermerkt", n)
	}
}

func TestFehlerWirdVermerktUndGekuerzt(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	if err := s.SetError(ctx, "eins", strings.Repeat("x", 5000)); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Get(ctx, "eins")
	if p.LastError == "" || len(p.LastError) > 2100 {
		t.Errorf("Fehlerlänge: %d", len(p.LastError))
	}
	// Eine neue Fassung räumt den alten Fehler weg.
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.Get(ctx, "eins"); p.LastError != "" {
		t.Errorf("der alte Fehler blieb nach der Aktualisierung stehen: %q", p.LastError)
	}
}
