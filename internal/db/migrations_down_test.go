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
