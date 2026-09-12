package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

// rollBackTheEnglishRename takes 00054 back down before a test rolls back an
// older migration, and it is not a convenience — without it these three tests
// cannot run at all.
//
// 00054 renames twenty-four columns and six indexes into English. Every down
// half written before it names its columns in German, because that is what they
// were called when it was written, and a released migration is never edited. So
// at head the German names do not exist and 00047's DROP INDEX
// idx_page_field_defs_kennung_oben finds nothing to drop.
//
// That is not a defect in either file. Rolling a single old migration back
// while newer ones stand is something only a test does; goose itself goes
// newest-first, and newest-first is exactly what this function restores. The
// tests below therefore do what an operator would do — take the top one off
// first — and then test the half they are about.
func rollBackTheEnglishRename(t *testing.T, ctx context.Context, provider *goose.Provider) {
	t.Helper()
	if _, err := provider.ApplyVersion(ctx, 54, false); err != nil {
		t.Fatalf("roll 00054 back so the German column names exist again: %v", err)
	}
}

// TestMigration00047DownAndUp drives the backward half of 00047.
//
// Nothing else in the tree drives it — which is why it is the half that ships
// broken. The test lies in package db and not in package db_test, because it
// needs migrationProvider: RunMigrations knows only the way up.
//
// What is checked are the three places where the rollback can reach too far or
// not far enough: the DELETE condition may only hit fields of a snippet, the
// restored index has to have the shape from 00038 and not the one from 00029,
// and afterwards the migration has to go up again.
func TestMigration00047DownAndUp(t *testing.T) {
	ctx := context.Background()
	database, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if err := RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	provider, err := migrationProvider(database.Write)
	if err != nil {
		t.Fatalf("migrationProvider: %v", err)
	}
	rollBackTheEnglishRename(t, ctx, provider)

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

	// Two fields with the same key: one on the page, one on the snippet. That
	// both may stand side by side is the one half of the index pair; that the
	// rollback hits only the second is the other.
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

	// --- runter -------------------------------------------------------------
	if _, err := provider.ApplyVersion(ctx, 47, false); err != nil {
		t.Fatalf("roll 00047 back: %v", err)
	}

	var uebrig int
	if err := database.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM page_field_defs`).Scan(&uebrig); err != nil {
		t.Fatalf("count fields: %v", err)
	}
	if uebrig != 1 {
		t.Fatalf("after the rollback %d fields are there, expected 1 (the page field only)", uebrig)
	}
	var kennung string
	if err := database.Read.QueryRowContext(ctx,
		`SELECT kennung FROM page_field_defs`).Scan(&kennung); err != nil {
		t.Fatalf("Feld lesen: %v", err)
	}
	if kennung != "telefon" {
		t.Errorf("remaining field = %q, expected \"telefon\"", kennung)
	}

	var indexSQL string
	if err := database.Read.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_page_field_defs_kennung_oben'`).
		Scan(&indexSQL); err != nil {
		t.Fatalf("Index lesen: %v", err)
	}
	if !strings.Contains(indexSQL, "block_type_id") {
		t.Errorf("wiederhergestellter Index = %q — ohne block_type_id ist das die Form aus 00029 "+
			"und damit eine Wanderung zu weit back", indexSQL)
	}
	if strings.Contains(indexSQL, "snippet_id") {
		t.Errorf("restored index = %q — snippet_id does not belong in it after the rollback", indexSQL)
	}

	// --- und wieder rauf ----------------------------------------------------
	if _, err := provider.ApplyVersion(ctx, 47, true); err != nil {
		t.Fatalf("00047 erneut anwenden: %v", err)
	}
	if err := RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations after the trip back up: %v", err)
	}

	var wieder string
	if err := database.Read.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_page_field_defs_key_snippet'`).
		Scan(&wieder); err != nil {
		t.Fatalf("snippet index after the trip back up: %v", err)
	}
	var leer string
	if err := database.Read.QueryRowContext(ctx,
		`SELECT fields FROM snippets WHERE id = $1`, snippetID).Scan(&leer); err != nil {
		t.Fatalf("snippets.fields after the trip back up: %v", err)
	}
	if leer != "" {
		t.Errorf("snippets.fields = %q, erwartet leer", leer)
	}
}

// TestMigration00048DownAndUp drives the backward half of 00048.
//
// For the same reason as above: nothing else in the tree drives it. And for a
// second — 00048 is a correction, and a correction restores the *wrong* state
// when it is rolled back. Whoever writes the Down by copying the new shape
// instead of the old takes nothing back at all; that is noticed nowhere,
// because both shapes are valid SQL and carry the same name. The test therefore
// reads the index text and not merely its presence.
func TestMigration00048DownAndUp(t *testing.T) {
	ctx := context.Background()
	database, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if err := RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	provider, err := migrationProvider(database.Write)
	if err != nil {
		t.Fatalf("migrationProvider: %v", err)
	}
	rollBackTheEnglishRename(t, ctx, provider)

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

	// --- runter -------------------------------------------------------------
	if _, err := provider.ApplyVersion(ctx, 48, false); err != nil {
		t.Fatalf("roll 00048 back: %v", err)
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
		t.Errorf("after the trip back up: index = %q, expected the narrower form with parent_id", got)
	}

	// And back to head, so the database this test leaves behind is the one every
	// other test starts from.
	if err := RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations after the trip back up: %v", err)
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

	provider, err := migrationProvider(database.Write)
	if err != nil {
		t.Fatalf("migrationProvider: %v", err)
	}
	rollBackTheEnglishRename(t, ctx, provider)

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

// TestMigration00050DownAndUp runs the down half of 00050.
//
// For the reason the three tests above already name: nothing else in the tree
// runs a down half, so it is the half that ships broken.
//
// 00050 creates and changes nothing, so — like 00049 and unlike the index swaps
// 00047 and 00048 — its rollback cannot restore the wrong shape. What it can do
// is take away too little, and it has three chances to: an index left standing,
// a child table left standing, or a parent dropped before its child. The test
// therefore counts all three objects in sqlite_master rather than looking for
// the parent table alone.
func TestMigration00050DownAndUp(t *testing.T) {
	ctx := context.Background()
	database, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if err := RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Counts what of 00050 currently stands in sqlite_master: the two tables and
	// the index. Three means all there, one or two means half gone, zero means
	// gone.
	present := func(where string) int {
		t.Helper()
		var n int
		if err := database.Read.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master
			  WHERE (type='table' AND name='albums')
			     OR (type='table' AND name='album_items')
			     OR (type='index' AND name='idx_album_items_album_sort')`).Scan(&n); err != nil {
			t.Fatalf("%s: read sqlite_master: %v", where, err)
		}
		return n
	}

	if got := present("after the up"); got != 3 {
		t.Fatalf("after the up: %d of 3 objects from 00050 stand there (two tables and one index)", got)
	}

	res, err := database.Write.ExecContext(ctx,
		`INSERT INTO websites (name, description) VALUES ('Test Site', '')`)
	if err != nil {
		t.Fatalf("create website: %v", err)
	}
	websiteID, _ := res.LastInsertId()

	res, err = database.Write.ExecContext(ctx,
		`INSERT INTO media (website_id, filename, original_name, mime_type, size_bytes)
		 VALUES ($1, 'bench.jpg', 'bench.jpg', 'image/jpeg', 1024)`, websiteID)
	if err != nil {
		t.Fatalf("create media row: %v", err)
	}
	mediaID, _ := res.LastInsertId()

	res, err = database.Write.ExecContext(ctx,
		`INSERT INTO albums (website_id, slug, name) VALUES ($1, 'workshop', 'Workshop')`, websiteID)
	if err != nil {
		t.Fatalf("create album: %v", err)
	}
	albumID, _ := res.LastInsertId()

	if _, err := database.Write.ExecContext(ctx,
		`INSERT INTO album_items (album_id, media_id, alt, caption, sort_order)
		 VALUES ($1, $2, 'A bench', 'Oak, 2024', 0)`, albumID, mediaID); err != nil {
		t.Fatalf("create picture row: %v", err)
	}

	countIn := func(where, table string) int {
		t.Helper()
		var n int
		if err := database.Read.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM `+table).Scan(&n); err != nil {
			t.Fatalf("%s: count %s: %v", where, table, err)
		}
		return n
	}

	if n := countIn("before the rollback", "albums"); n != 1 {
		t.Fatalf("before the rollback %d albums stand there, expected 1", n)
	}
	if n := countIn("before the rollback", "album_items"); n != 1 {
		t.Fatalf("before the rollback %d picture rows stand there, expected 1", n)
	}

	provider, err := migrationProvider(database.Write)
	if err != nil {
		t.Fatalf("migrationProvider: %v", err)
	}

	// --- down ---------------------------------------------------------------
	if _, err := provider.ApplyVersion(ctx, 50, false); err != nil {
		t.Fatalf("roll 00050 back: %v", err)
	}
	if got := present("after the rollback"); got != 0 {
		t.Errorf("after the rollback %d objects from 00050 still stand there, expected 0 — "+
			"the rollback must take the index, the child table and the parent, not some of the three", got)
	}

	// --- and back up again ---------------------------------------------------
	if _, err := provider.ApplyVersion(ctx, 50, true); err != nil {
		t.Fatalf("apply 00050 again: %v", err)
	}
	if got := present("after the trip back up"); got != 3 {
		t.Errorf("after the trip back up: %d of 3 objects from 00050 stand there", got)
	}
	if n := countIn("after the trip back up", "albums"); n != 0 {
		t.Errorf("after the trip back up %d albums stand there, expected 0 — the table is new", n)
	}
	if n := countIn("after the trip back up", "album_items"); n != 0 {
		t.Errorf("after the trip back up %d picture rows stand there, expected 0 — "+
			"a picture row that outlived its album would be an orphan the cascade should have taken", n)
	}

	if err := RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations after the trip back up: %v", err)
	}
}

// TestMigration00054DownAndUp runs the down half of 00054.
//
// For the reason the four tests above already name: nothing else in the tree
// runs a down half, so it is the half that ships broken. 00054 has two extra
// ways to ship broken that none of them has.
//
// The first is that it renames rather than creates, so its rollback can restore
// the wrong *name* and nothing would notice — every name is valid SQL and the
// table keeps working either way. The test therefore reads the column list out
// of the schema in both directions instead of asking whether the table is
// there.
//
// The second is that it rebuilds six indexes by hand, and one of them is the
// snippet index that 00048 exists to correct. Copying 00047's wider form into
// 00054 would silently undo 00048 — two migrations later, in a file that is
// nominally about spelling. So the down half is checked for the *narrower* form
// with parent_id, which is the one that must come back.
func TestMigration00054DownAndUp(t *testing.T) {
	ctx := context.Background()
	database, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if err := RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	columns := func(where, table string) map[string]bool {
		t.Helper()
		rows, err := database.Read.QueryContext(ctx, `SELECT name FROM pragma_table_info($1)`, table)
		if err != nil {
			t.Fatalf("%s: read columns of %s: %v", where, table, err)
		}
		defer rows.Close()
		out := map[string]bool{}
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				t.Fatalf("%s: scan column of %s: %v", where, table, err)
			}
			out[n] = true
		}
		return out
	}

	// --- at head, everything is English -------------------------------------
	defs := columns("after the up", "page_field_defs")
	for _, want := range []string{
		"key", "label", "kind", "required", "hint", "choices",
		"applies_to", "condition", "display", "max_values", "range_min", "range_max",
	} {
		if !defs[want] {
			t.Errorf("after the up page_field_defs has no column %q", want)
		}
	}
	for _, gone := range []string{
		"kennung", "beschriftung", "art", "pflicht", "hinweis", "auswahl",
		"gilt_fuer", "bedingung", "darstellung", "max_werte", "min_wert", "max_wert",
	} {
		if defs[gone] {
			t.Errorf("after the up page_field_defs still has the German column %q", gone)
		}
	}

	// pages is the one table where the German name does NOT become `kind`: the
	// English `kind` has been taken since 00014 and means page-or-post. Both
	// must stand, side by side, or the rename has collapsed two questions into
	// one column.
	pages := columns("after the up", "pages")
	if !pages["kind"] || !pages["content_kind"] {
		t.Errorf("after the up pages has kind=%v content_kind=%v — both are needed: "+
			"kind is page-or-post since 00014, content_kind is the website's own kind",
			pages["kind"], pages["content_kind"])
	}
	if pages["art"] {
		t.Errorf("after the up pages still has the German column art")
	}

	// --- a row written before the rollback, read after the trip back up ------
	websiteID := mustExec(t, database.Write,
		`INSERT INTO websites (name, description) VALUES ('Rename Site', '')`)
	if _, err := database.Write.ExecContext(ctx,
		`INSERT INTO page_field_defs (website_id, key, label, kind, hint, choices, applies_to)
		 VALUES ($1, 'preis', 'Preis', 'zahl', 'in Euro', '', 'beides')`, websiteID); err != nil {
		t.Fatalf("create field def: %v", err)
	}

	provider, err := migrationProvider(database.Write)
	if err != nil {
		t.Fatalf("migrationProvider: %v", err)
	}

	// --- down ---------------------------------------------------------------
	if _, err := provider.ApplyVersion(ctx, 54, false); err != nil {
		t.Fatalf("roll 00054 back: %v", err)
	}

	back := columns("after the rollback", "page_field_defs")
	for _, want := range []string{"kennung", "beschriftung", "art", "pflicht", "hinweis", "auswahl", "gilt_fuer"} {
		if !back[want] {
			t.Errorf("after the rollback page_field_defs has no column %q — a rollback that "+
				"leaves the new name in place has rolled nothing back, and every name here "+
				"is valid SQL either way", want)
		}
	}
	if back["key"] || back["label"] {
		t.Errorf("after the rollback page_field_defs still carries English names")
	}
	if p := columns("after the rollback", "pages"); !p["art"] || p["content_kind"] {
		t.Errorf("after the rollback pages has art=%v content_kind=%v, expected true/false", p["art"], p["content_kind"])
	}

	// The value is still there and still says what it said. A rename cannot lose
	// a row, and this asserts that the rollback really was a rename and not a
	// rebuild that dropped one.
	var label, art string
	if err := database.Read.QueryRowContext(ctx,
		`SELECT beschriftung, art FROM page_field_defs WHERE kennung = 'preis'`).Scan(&label, &art); err != nil {
		t.Fatalf("read the field back under its German names: %v", err)
	}
	if label != "Preis" || art != "zahl" {
		t.Errorf("after the rollback the row reads %q/%q, expected \"Preis\"/\"zahl\"", label, art)
	}

	// 00048's correction must survive a trip through 00054 in both directions.
	var snippetIndex string
	if err := database.Read.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_page_field_defs_kennung_textbaustein'`).
		Scan(&snippetIndex); err != nil {
		t.Fatalf("read the snippet index after the rollback: %v", err)
	}
	if !strings.Contains(snippetIndex, "parent_id") {
		t.Errorf("after the rollback the snippet index = %q — without parent_id this is "+
			"00047's wider form, and 00054 has silently undone the correction 00048 exists to make",
			snippetIndex)
	}

	// --- and back up again ---------------------------------------------------
	if _, err := provider.ApplyVersion(ctx, 54, true); err != nil {
		t.Fatalf("apply 00054 again: %v", err)
	}
	again := columns("after the trip back up", "page_field_defs")
	if !again["key"] || again["kennung"] {
		t.Errorf("after the trip back up page_field_defs has key=%v kennung=%v", again["key"], again["kennung"])
	}
	var labelAgain string
	if err := database.Read.QueryRowContext(ctx,
		`SELECT label FROM page_field_defs WHERE key = 'preis'`).Scan(&labelAgain); err != nil {
		t.Fatalf("read the field after the trip back up: %v", err)
	}
	if labelAgain != "Preis" {
		t.Errorf("after the trip back up the row reads %q, expected \"Preis\"", labelAgain)
	}
	if err := database.Read.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_page_field_defs_key_snippet'`).
		Scan(&snippetIndex); err != nil {
		t.Fatalf("read the snippet index after the trip back up: %v", err)
	}
	if !strings.Contains(snippetIndex, "parent_id") {
		t.Errorf("after the trip back up the snippet index = %q, expected 00048's narrower form", snippetIndex)
	}
}
