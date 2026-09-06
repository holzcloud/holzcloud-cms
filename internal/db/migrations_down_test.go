package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigration00047RunterUndRauf fährt die Rückwärtshälfte von 00047.
//
// Nichts sonst im Baum fährt sie — deshalb ist sie die Hälfte, die kaputt
// ausgeliefert wird. Der Test liegt in package db und nicht in package db_test,
// weil er migrationProvider braucht: RunMigrations kennt nur den Weg nach oben.
//
// Geprüft werden die drei Stellen, an denen die Rücknahme zu weit oder zu kurz
// greifen kann: die DELETE-Bedingung darf nur Felder eines Textbausteins
// treffen, der wiederhergestellte Index muss die Form aus 00038 haben und nicht
// die aus 00029, und danach muss die Wanderung wieder nach oben gehen.
func TestMigration00047RunterUndRauf(t *testing.T) {
	ctx := context.Background()
	database, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if err := RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	res, err := database.Write.ExecContext(ctx,
		`INSERT INTO websites (name, description) VALUES ('Prüfsite', '')`)
	if err != nil {
		t.Fatalf("Website anlegen: %v", err)
	}
	websiteID, _ := res.LastInsertId()

	res, err = database.Write.ExecContext(ctx,
		`INSERT INTO snippets (website_id, key, name) VALUES ($1, 'kontakt', 'Kontakt')`, websiteID)
	if err != nil {
		t.Fatalf("Textbaustein anlegen: %v", err)
	}
	snippetID, _ := res.LastInsertId()

	// Zwei Felder mit derselben Kennung: eines an der Seite, eines am
	// Textbaustein. Dass beide nebeneinander stehen dürfen, ist die eine
	// Hälfte des Indexpaars; dass die Rücknahme nur das zweite trifft, die
	// andere.
	if _, err := database.Write.ExecContext(ctx,
		`INSERT INTO page_field_defs (website_id, kennung, beschriftung, art)
		 VALUES ($1, 'telefon', 'Telefon', 'text')`, websiteID); err != nil {
		t.Fatalf("Seitenfeld anlegen: %v", err)
	}
	if _, err := database.Write.ExecContext(ctx,
		`INSERT INTO page_field_defs (website_id, snippet_id, kennung, beschriftung, art)
		 VALUES ($1, $2, 'telefon', 'Telefon', 'text')`, websiteID, snippetID); err != nil {
		t.Fatalf("Textbausteinfeld anlegen: %v", err)
	}

	provider, err := migrationProvider(database.Write)
	if err != nil {
		t.Fatalf("migrationProvider: %v", err)
	}

	// --- runter -------------------------------------------------------------
	if _, err := provider.ApplyVersion(ctx, 47, false); err != nil {
		t.Fatalf("00047 zurücknehmen: %v", err)
	}

	var uebrig int
	if err := database.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM page_field_defs`).Scan(&uebrig); err != nil {
		t.Fatalf("Felder zählen: %v", err)
	}
	if uebrig != 1 {
		t.Fatalf("nach der Rücknahme sind %d Felder da, erwartet 1 (nur das Seitenfeld)", uebrig)
	}
	var kennung string
	if err := database.Read.QueryRowContext(ctx,
		`SELECT kennung FROM page_field_defs`).Scan(&kennung); err != nil {
		t.Fatalf("Feld lesen: %v", err)
	}
	if kennung != "telefon" {
		t.Errorf("übriges Feld = %q, erwartet \"telefon\"", kennung)
	}

	var indexSQL string
	if err := database.Read.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_page_field_defs_kennung_oben'`).
		Scan(&indexSQL); err != nil {
		t.Fatalf("Index lesen: %v", err)
	}
	if !strings.Contains(indexSQL, "block_type_id") {
		t.Errorf("wiederhergestellter Index = %q — ohne block_type_id ist das die Form aus 00029 "+
			"und damit eine Wanderung zu weit zurück", indexSQL)
	}
	if strings.Contains(indexSQL, "snippet_id") {
		t.Errorf("wiederhergestellter Index = %q — snippet_id gehört nach der Rücknahme nicht mehr hinein", indexSQL)
	}

	// --- und wieder rauf ----------------------------------------------------
	if _, err := provider.ApplyVersion(ctx, 47, true); err != nil {
		t.Fatalf("00047 erneut anwenden: %v", err)
	}
	if err := RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations nach der Rückfahrt: %v", err)
	}

	var wieder string
	if err := database.Read.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_page_field_defs_kennung_textbaustein'`).
		Scan(&wieder); err != nil {
		t.Fatalf("Textbaustein-Index nach der Rückfahrt: %v", err)
	}
	var leer string
	if err := database.Read.QueryRowContext(ctx,
		`SELECT fields FROM snippets WHERE id = $1`, snippetID).Scan(&leer); err != nil {
		t.Fatalf("snippets.fields nach der Rückfahrt: %v", err)
	}
	if leer != "" {
		t.Errorf("snippets.fields = %q, erwartet leer", leer)
	}
}

// TestMigration00048RunterUndRauf fährt die Rückwärtshälfte von 00048.
//
// Aus demselben Grund wie oben: nichts sonst im Baum fährt sie. Und aus einem
// zweiten — 00048 ist eine Berichtigung, und eine Berichtigung stellt bei ihrer
// Rücknahme den *falschen* Zustand wieder her. Wer beim Schreiben des Down die
// neue Form abschreibt statt der alten, nimmt gar nichts zurück; das fällt
// nirgends auf, weil beide Formen gültiges SQL sind und denselben Namen tragen.
// Der Test liest deshalb den Indextext und nicht bloss seine Anwesenheit.
func TestMigration00048RunterUndRauf(t *testing.T) {
	ctx := context.Background()
	database, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if err := RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	indexText := func(wo string) string {
		t.Helper()
		var sql string
		if err := database.Read.QueryRowContext(ctx,
			`SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_page_field_defs_kennung_textbaustein'`).
			Scan(&sql); err != nil {
			t.Fatalf("%s: Textbaustein-Index lesen: %v", wo, err)
		}
		return sql
	}

	if got := indexText("nach oben"); !strings.Contains(got, "parent_id") {
		t.Fatalf("nach oben: Index = %q — ohne parent_id fallen die Unterfelder einer "+
			"Gruppe in den Namensraum der obersten Ebene", got)
	}

	provider, err := migrationProvider(database.Write)
	if err != nil {
		t.Fatalf("migrationProvider: %v", err)
	}

	// --- runter -------------------------------------------------------------
	if _, err := provider.ApplyVersion(ctx, 48, false); err != nil {
		t.Fatalf("00048 zurücknehmen: %v", err)
	}
	zurueck := indexText("nach der Rücknahme")
	if strings.Contains(zurueck, "parent_id") {
		t.Errorf("nach der Rücknahme: Index = %q — die Rücknahme muss die Form aus "+
			"00047 wiederherstellen und nicht die eigene", zurueck)
	}
	if !strings.Contains(zurueck, "snippet_id") {
		t.Errorf("nach der Rücknahme: Index = %q — snippet_id gehört weiterhin hinein, "+
			"00047 hat den Index eingeführt", zurueck)
	}

	// --- und wieder rauf ----------------------------------------------------
	if _, err := provider.ApplyVersion(ctx, 48, true); err != nil {
		t.Fatalf("00048 erneut anwenden: %v", err)
	}
	if got := indexText("nach der Rückfahrt"); !strings.Contains(got, "parent_id") {
		t.Errorf("nach der Rückfahrt: Index = %q, erwartet die engere Form mit parent_id", got)
	}
}

// TestMigration00049DownAndUp runs the down half of 00049.
//
// For the reason the two tests above already name: nothing else in the tree
// runs it, so it is the half that ships broken.
//
// 00049 is not a correction like 00048 and not an index swap like 00047 — it
// creates. Its rollback therefore cannot restore the wrong shape, but it can
// take away too little: a DROP TABLE without the DROP INDEX before it, or the
// other way round. The test therefore looks in sqlite_master for both and not
// merely for the table.
func TestMigration00049DownAndUp(t *testing.T) {
	ctx := context.Background()
	database, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if err := RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Counts what of 00049 currently stands in sqlite_master: the table and its
	// index. Two means there, one means half gone, zero means gone.
	present := func(where string) int {
		t.Helper()
		var n int
		if err := database.Read.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master
			  WHERE (type='table' AND name='csv_imports')
			     OR (type='index' AND name='idx_csv_imports_alter')`).Scan(&n); err != nil {
			t.Fatalf("%s: read sqlite_master: %v", where, err)
		}
		return n
	}

	if got := present("after the up"); got != 2 {
		t.Fatalf("after the up: %d of 2 objects from 00049 stand there (table and index)", got)
	}

	res, err := database.Write.ExecContext(ctx,
		`INSERT INTO users (email, password, role, created_at)
		 VALUES ('staging@example.com', 'hash', 'admin', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	userID, _ := res.LastInsertId()

	res, err = database.Write.ExecContext(ctx,
		`INSERT INTO websites (name, description) VALUES ('Test Site', '')`)
	if err != nil {
		t.Fatalf("create website: %v", err)
	}
	websiteID, _ := res.LastInsertId()

	if _, err := database.Write.ExecContext(ctx,
		`INSERT INTO csv_imports (token_hash, user_id, website_id, modus, daten, erstellt_am)
		 VALUES ('abc', $1, $2, 'bestehend', $3, '2026-01-01T00:00:00Z')`,
		userID, websiteID, []byte("Title\nChair\n")); err != nil {
		t.Fatalf("create staging row: %v", err)
	}

	var rows int
	if err := database.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM csv_imports`).Scan(&rows); err != nil {
		t.Fatalf("count stagings: %v", err)
	}
	if rows != 1 {
		t.Fatalf("before the rollback %d stagings stand there, expected 1", rows)
	}

	provider, err := migrationProvider(database.Write)
	if err != nil {
		t.Fatalf("migrationProvider: %v", err)
	}

	// --- down ---------------------------------------------------------------
	if _, err := provider.ApplyVersion(ctx, 49, false); err != nil {
		t.Fatalf("roll 00049 back: %v", err)
	}
	if got := present("after the rollback"); got != 0 {
		t.Errorf("after the rollback %d objects from 00049 still stand there, expected 0 — "+
			"the rollback must take index and table, not just one of the two", got)
	}

	// --- and back up again ---------------------------------------------------
	if _, err := provider.ApplyVersion(ctx, 49, true); err != nil {
		t.Fatalf("apply 00049 again: %v", err)
	}
	if got := present("after the trip back up"); got != 2 {
		t.Errorf("after the trip back up: %d of 2 objects from 00049 stand there", got)
	}
	if err := database.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM csv_imports`).Scan(&rows); err != nil {
		t.Fatalf("count stagings after the trip back up: %v", err)
	}
	if rows != 0 {
		t.Errorf("after the trip back up %d stagings stand there, expected 0 — the table is new", rows)
	}

	if err := RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations after the trip back up: %v", err)
	}
}
