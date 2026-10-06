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
	m := goodManifest()
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
	// Arrive switched off: a plugin that starts running as it is uploaded is one
	// nobody could look at first.
	if p.Enabled {
		t.Error("freshly installed and already switched on")
	}
	if p.Manifest == nil || p.Manifest.ID != "weiterleitungen" {
		t.Errorf("the manifest did not come back: %+v", p.Manifest)
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
	// Whoever updates expects it to keep running — not to switch itself off and
	// lose its assignment.
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

func TestAnAddressIsGivenOutOnlyOnce(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()

	if err := s.Install(ctx, paket("suche", "/suche")); err != nil {
		t.Fatal(err)
	}
	err := s.Install(ctx, paket("andere-suche", "/suche"))
	if !errors.Is(err, ErrRouteTaken) {
		t.Fatalf("erwartet ErrRouteTaken, bekommen: %v", err)
	}
	// The message has to say whom the address belongs to — otherwise the
	// operator searches through a dozen plugins.
	if !strings.Contains(err.Error(), "Suche") {
		t.Errorf("the message does not name the owner: %v", err)
	}
	// The same address in a new version of the same plugin is no collision with
	// itself.
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

	// A missing key is not a fault but the first run.
	if _, ok, err := s.StoreGet(ctx, "eins", 1, "gibtsnicht"); err != nil || ok {
		t.Errorf("missing key: ok=%v err=%v", ok, err)
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
	// Without escaping, "a%" would be a pattern and would match everything.
	if len(got) != 1 || got["a%b"] != "treffer" {
		t.Errorf("the prefix was read as a pattern: %v", got)
	}
}

func TestSpeicherGrenzen(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreSet(ctx, "eins", 0, strings.Repeat("k", MaxKeyBytes+1), "x"); err == nil {
		t.Error("an over-long key was accepted")
	}
	if err := s.StoreSet(ctx, "eins", 0, "gross", strings.Repeat("x", MaxValueBytes+1)); err == nil {
		t.Error("an over-large value was accepted")
	}
}

func TestRemovingTakesTheDataWithIt(t *testing.T) {
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
	// Installed again it must inherit nothing from its predecessor: the old
	// version may have held the data in a different shape.
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.StoreGet(ctx, "eins", 0, "farbe"); ok {
		t.Error("after removing and reinstalling, the old value was still there")
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
	// A second run must not fail on "table already exists".
	if err := s.ApplyMigrations(ctx, "eins", ms); err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}

	// Changed SQL under an old name: otherwise nothing happens, and the schema
	// silently no longer fits the code that expects it.
	changed := []Migration{{Name: "0001.sql", SQL: `CREATE TABLE anders (b TEXT) STRICT;`}}
	err := s.ApplyMigrations(ctx, "eins", changed)
	if err == nil || !strings.Contains(err.Error(), "has changed") {
		t.Errorf("a changed migration was not reported: %v", err)
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
	// Neither the table nor the entry may have been left standing, or the next
	// start reports "already applied" for something that never ran.
	var n int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM plugin_migrations WHERE plugin_id = 'eins'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d Migrationen als angewendet vermerkt", n)
	}
}

func TestAFailureIsNotedAndTruncated(t *testing.T) {
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
		t.Errorf("error length: %d", len(p.LastError))
	}
	// A new version clears the old fault away.
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.Get(ctx, "eins"); p.LastError != "" {
		t.Errorf("the old error stayed after the update: %q", p.LastError)
	}
}

func TestMigrationSandbox(t *testing.T) {
	ctx := context.Background()
	bad := map[string]string{
		"attach":         `ATTACH DATABASE '/tmp/x.db' AS x;`,
		"attach mixed":   `aTtAcH database '/tmp/x.db' as x;`,
		"attach comment": "/* hi */ -- x\n ATTACH DATABASE 'y' AS y;",
		"pragma":         `PRAGMA writable_schema = 1;`,
		"vacuum":         `VACUUM INTO '/tmp/copy.db';`,
		"extension":      `INSERT INTO plugin_eins_a SELECT load_extension('x');`,
		"core table":     `CREATE TABLE users2 (a TEXT);`,
		"drop core":      `DROP TABLE users;`,
		"alter core":     `ALTER TABLE users ADD COLUMN x TEXT;`,
		"insert core":    `INSERT INTO users (id) VALUES (1);`,
		"update core":    `UPDATE pages SET title = 'x';`,
		"delete core":    `DELETE FROM plugin_migrations;`,
		"read secrets":   `INSERT INTO plugin_eins_a SELECT password_hash FROM users;`,
		"hidden second":  `CREATE TABLE plugin_eins_a (a TEXT); DROP TABLE users;`,
		"quoted":         `DROP TABLE "users";`,
		"trigger":        `CREATE TRIGGER plugin_eins_t AFTER INSERT ON plugin_eins_a BEGIN DELETE FROM users; END;`,
		"other plugin":   `CREATE TABLE plugin_zwei_a (a TEXT);`,
		"store foreign":  `DELETE FROM plugin_store WHERE plugin_id = 'zwei';`,
		"store nameless": `DELETE FROM plugin_store;`,
		"temp":           `CREATE TEMP TABLE plugin_eins_a (a TEXT);`,
		"schema":         `DELETE FROM sqlite_master;`,
		"unterminated":   `CREATE TABLE plugin_eins_a (a TEXT) -- ok` + "\n" + `; INSERT INTO plugin_eins_a VALUES ('x`,
	}
	for name, q := range bad {
		t.Run(name, func(t *testing.T) {
			s, _ := neuerSpeicher(t)
			if err := s.Install(ctx, paket("eins")); err != nil {
				t.Fatal(err)
			}
			err := s.ApplyMigrations(ctx, "eins", []Migration{{Name: "0001.sql", SQL: q}})
			if err == nil || !strings.Contains(err.Error(), "refused") {
				t.Fatalf("hostile migration was not refused: %v", err)
			}
			var n int
			if err := s.DB.Read.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM plugin_migrations WHERE plugin_id = 'eins'`).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Errorf("%d migration(s) recorded", n)
			}
		})
	}

	good := `-- own tables
CREATE TABLE IF NOT EXISTS plugin_eins_a (a TEXT, b TEXT) STRICT;
CREATE UNIQUE INDEX plugin_eins_a_idx ON plugin_eins_a (a);
INSERT INTO plugin_eins_a (a, b) VALUES ('x;y', 'it''s -- fine');
INSERT OR IGNORE INTO plugin_store (plugin_id, website_id, key, value, updated_at)
  SELECT 'eins', m.website_id, 'k' || m.id, '{}', 'now' FROM form_messages m;
UPDATE plugin_eins_a SET b = 'z';
ALTER TABLE plugin_eins_a ADD COLUMN c TEXT;
`
	s, _ := neuerSpeicher(t)
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyMigrations(ctx, "eins", []Migration{{Name: "0001.sql", SQL: good}}); err != nil {
		t.Fatalf("legitimate migration refused: %v", err)
	}
}
