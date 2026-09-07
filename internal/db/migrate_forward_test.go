package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// Every migration test in this package opens an empty database and runs the
// whole chain over nothing. That proves the SQL parses and the final schema is
// what it should be, and it cannot prove the one thing a migration is actually
// dangerous for: moving rows that already exist.
//
// 00045 is the case. It rebuilds `pages` — thirty columns — because SQLite
// cannot alter a table-level UNIQUE, and because DROP TABLE with foreign keys
// on is a silent DELETE FROM it has to rebuild everything hanging off it as
// well: page_revisions, media_usage, page_terms with CASCADE, and menu_items
// and form_messages whose page_id would otherwise be set to NULL. Its own
// header says so at length.
//
// A column left out of one of those six INSERT … SELECT pairs, or a foreign
// key that lands on the wrong row after the renames, is invisible against an
// empty database and is a data loss against a real one. So: build the graph at
// 00044, migrate forward to the end, and read every row back.

// atVersion opens a fresh database migrated up to one version and no further.
func atVersion(t *testing.T, version int64) *sql.DB {
	t.Helper()
	database, err := Open(filepath.Join(t.TempDir(), "forward.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(database.Close)

	provider, err := migrationProvider(database.Write)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	if _, err := provider.UpTo(context.Background(), version); err != nil {
		t.Fatalf("goose up to %d: %v", version, err)
	}
	return database.Write
}

// migrateToHead runs the rest of the chain on a database that already carries
// rows.
func migrateToHead(t *testing.T, conn *sql.DB) {
	t.Helper()
	provider, err := migrationProvider(conn)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("goose up: %v", err)
	}
}

func mustExec(t *testing.T, conn *sql.DB, query string, args ...any) int64 {
	t.Helper()
	res, err := conn.ExecContext(context.Background(), query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestAPopulatedDatabaseSurvivesTheRebuildOfPages(t *testing.T) {
	conn := atVersion(t, 44)
	ctx := context.Background()

	websiteID := mustExec(t, conn, `INSERT INTO websites (name) VALUES ('Testseite')`)
	userID := mustExec(t, conn,
		`INSERT INTO users (name, email, password, role) VALUES ('T', 'a@test', 'x', 'admin')`)
	mediaID := mustExec(t, conn,
		`INSERT INTO media (website_id, filename, original_name, mime_type, size_bytes)
		 VALUES ($1, 'bild.jpg', 'Bild.jpg', 'image/jpeg', 1234)`, websiteID)

	// Every column 00045 has to carry across is filled with a value that is not
	// the default, so a column dropped from the INSERT list shows up as a
	// changed value and not as an equal-by-accident empty string.
	pageID := mustExec(t, conn, `
		INSERT INTO pages (website_id, title, slug, content_markdown, content_html,
			status, published_at, version, created_by, updated_by, excerpt, meta_description,
			featured_media_id, noindex, publish_at, unpublish_at, review_state, kind,
			access, access_password, access_hint, blocks, fields, locale, art)
		VALUES ($1, 'Kontakt', 'kontakt', 'Der Text', '<p>Der Text</p>',
			'published', '2026-01-02T03:04:05Z', 7, $2, $2, 'Der Anriss', 'Die Beschreibung',
			$3, 1, '2026-02-01T00:00:00Z', '2099-01-01T00:00:00Z', 'pending', 'post',
			'password', 'geheim', 'Bei der Réception', '[{"typ":"text"}]', '{"werte":{"a":"b"}}',
			'fr', 'produkt')`, websiteID, userID, mediaID)

	// The five tables that hang off it and would be emptied or nulled by a
	// careless rebuild.
	revID := mustExec(t, conn,
		`INSERT INTO page_revisions (page_id, user_id, title, slug, content_markdown, status)
		 VALUES ($1, $2, 'Kontakt v1', 'kontakt', 'Alt', 'draft')`, pageID, userID)
	mustExec(t, conn, `INSERT INTO media_usage (media_id, page_id) VALUES ($1, $2)`, mediaID, pageID)
	termID := mustExec(t, conn,
		`INSERT INTO terms (website_id, name, slug) VALUES ($1, 'Möbel', 'moebel')`, websiteID)
	mustExec(t, conn, `INSERT INTO page_terms (page_id, term_id) VALUES ($1, $2)`, pageID, termID)
	menuID := mustExec(t, conn,
		`INSERT INTO menus (website_id, name, location_key) VALUES ($1, 'Hauptmenü', 'main')`, websiteID)
	itemID := mustExec(t, conn,
		`INSERT INTO menu_items (menu_id, title, item_type, page_id, sort_order)
		 VALUES ($1, 'Kontakt', 'page', $2, 0)`, menuID, pageID)

	migrateToHead(t, conn)

	// The page itself, column by column.
	var (
		title, slug, md, html, status, publishedAt string
		excerpt, metaDesc, reviewState, kind       string
		access, accessPassword, accessHint         string
		blocks, fields, locale, art                string
		publishAt, unpublishAt                     string
		version, noindex                           int
		featured, createdBy, updatedBy             sql.NullInt64
	)
	err := conn.QueryRowContext(ctx, `
		SELECT title, slug, content_markdown, content_html, status, published_at,
		       excerpt, meta_description, review_state, kind, access, access_password,
		       access_hint, blocks, fields, locale, art, publish_at, unpublish_at,
		       version, noindex, featured_media_id, created_by, updated_by
		  FROM pages WHERE id = $1`, pageID).Scan(
		&title, &slug, &md, &html, &status, &publishedAt,
		&excerpt, &metaDesc, &reviewState, &kind, &access, &accessPassword,
		&accessHint, &blocks, &fields, &locale, &art, &publishAt, &unpublishAt,
		&version, &noindex, &featured, &createdBy, &updatedBy)
	if err != nil {
		t.Fatalf("the page did not survive the migration at all: %v", err)
	}

	for _, c := range []struct{ column, got, want string }{
		{"title", title, "Kontakt"},
		{"slug", slug, "kontakt"},
		{"content_markdown", md, "Der Text"},
		{"content_html", html, "<p>Der Text</p>"},
		{"status", status, "published"},
		{"published_at", publishedAt, "2026-01-02T03:04:05Z"},
		{"excerpt", excerpt, "Der Anriss"},
		{"meta_description", metaDesc, "Die Beschreibung"},
		{"review_state", reviewState, "pending"},
		{"kind", kind, "post"},
		{"access", access, "password"},
		{"access_password", accessPassword, "geheim"},
		{"access_hint", accessHint, "Bei der Réception"},
		{"blocks", blocks, `[{"typ":"text"}]`},
		{"fields", fields, `{"werte":{"a":"b"}}`},
		{"locale", locale, "fr"},
		{"art", art, "produkt"},
		{"publish_at", publishAt, "2026-02-01T00:00:00Z"},
		{"unpublish_at", unpublishAt, "2099-01-01T00:00:00Z"},
	} {
		if c.got != c.want {
			t.Errorf("pages.%s came through the rebuild as %q, want %q", c.column, c.got, c.want)
		}
	}
	if version != 7 {
		t.Errorf("pages.version came through as %d, want 7 — a page that loses its version "+
			"token loses every concurrent-edit guard that rests on it", version)
	}
	if noindex != 1 {
		t.Errorf("pages.noindex came through as %d, want 1", noindex)
	}
	if !featured.Valid || featured.Int64 != mediaID {
		t.Errorf("pages.featured_media_id came through as %v, want %d", featured, mediaID)
	}
	if !createdBy.Valid || createdBy.Int64 != userID {
		t.Errorf("pages.created_by came through as %v, want %d", createdBy, userID)
	}
	if !updatedBy.Valid || updatedBy.Int64 != userID {
		t.Errorf("pages.updated_by came through as %v, want %d", updatedBy, userID)
	}

	// And the five that hang off it. Each of these is a row a rebuild can
	// delete or a foreign key it can null, and the header of 00045 names every
	// one of them as the reason it is as long as it is.
	for _, c := range []struct {
		what  string
		query string
		args  []any
		want  int
	}{
		{"page_revisions", `SELECT COUNT(*) FROM page_revisions WHERE id = $1 AND page_id = $2`, []any{revID, pageID}, 1},
		{"media_usage", `SELECT COUNT(*) FROM media_usage WHERE media_id = $1 AND page_id = $2`, []any{mediaID, pageID}, 1},
		{"page_terms", `SELECT COUNT(*) FROM page_terms WHERE page_id = $1 AND term_id = $2`, []any{pageID, termID}, 1},
		{"menu_items", `SELECT COUNT(*) FROM menu_items WHERE id = $1 AND page_id = $2`, []any{itemID, pageID}, 1},
	} {
		var n int
		if err := conn.QueryRowContext(ctx, c.query, c.args...).Scan(&n); err != nil {
			t.Errorf("%s: %v", c.what, err)
			continue
		}
		if n != c.want {
			t.Errorf("%s lost its row through the rebuild of pages (found %d, want %d) — "+
				"this is the loss an empty-database migration cannot show", c.what, n, c.want)
		}
	}

	// The constraint the whole rebuild exists for, exercised rather than read
	// off the schema: the same address in two languages.
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO pages (website_id, title, slug, locale) VALUES ($1, 'Contact', 'kontakt', 'it')`,
		websiteID); err != nil {
		t.Errorf("a second language could not take the same address: %v — that is the "+
			"constraint 00045 was written to change", err)
	}
}
