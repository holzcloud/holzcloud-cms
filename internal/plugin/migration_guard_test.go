package plugin

import (
	"context"
	"os"
	"strings"
	"testing"
)

// expectRefused applies a migration of plugin "eins" and holds the outcome to
// what a refusal has to leave behind: an error that says refused and names the
// thing that was refused, no record of the migration, no table of the plugin
// behind, and a core write that goes through at once.
func expectRefused(t *testing.T, s *Store, q, word string) {
	t.Helper()
	ctx := context.Background()
	err := s.ApplyMigrations(ctx, "eins", []Migration{{Name: "0001.sql", SQL: q}})
	if err == nil {
		t.Fatalf("not refused:\n%s", q)
	}
	if !strings.Contains(err.Error(), "refused") || !strings.Contains(err.Error(), word) {
		t.Fatalf("error %q does not say refused and name %q", err, word)
	}
	var n int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM plugin_migrations WHERE plugin_id = 'eins'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d migration(s) recorded", n)
	}
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_schema WHERE name LIKE 'plugin\_eins\_%' ESCAPE '\'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d object(s) of the plugin left behind", n)
	}
	if _, err := s.DB.Write.ExecContext(ctx, `UPDATE plugins SET last_error = '' WHERE id = 'eins'`); err != nil {
		t.Errorf("core write after the refusal: %v", err)
	}
}

func TestAMigrationCannotReadCoreSecrets(t *testing.T) {
	// Each case creates the plugin's own table first, so it fails on the rule it
	// is about and not on a missing table.
	const own = `CREATE TABLE plugin_eins_a (t TEXT); `
	cases := []struct{ name, sql, word string }{
		{"single quoted", own + `INSERT INTO plugin_eins_a SELECT token FROM 'sessions';`, "sessions"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := neuerSpeicher(t)
			if err := s.Install(context.Background(), paket("eins")); err != nil {
				t.Fatal(err)
			}
			expectRefused(t, s, c.sql, c.word)
		})
	}
}

func TestTheShippedContactFormMigrationApplies(t *testing.T) {
	m, store, site := neuerManager(t)
	ctx := context.Background()
	seedLegacyMessages(t, store, site)

	zip, err := os.ReadFile("../../plugins/kontaktformular/kontaktformular.zip")
	if err != nil {
		t.Fatal(err)
	}
	einspielen(t, m, zip)

	var n int
	var key string
	if err := store.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*), MIN(key) FROM plugin_store WHERE plugin_id = 'kontaktformular'`).Scan(&n, &key); err != nil {
		t.Fatal(err)
	}
	if n != 1 || !strings.HasPrefix(key, "nachricht:") {
		t.Errorf("plugin_store holds %d row(s), first key %q", n, key)
	}
	st, err := m.Get(ctx, "kontaktformular")
	if err != nil {
		t.Fatal(err)
	}
	if st.LastError != "" {
		t.Errorf("LastError = %q", st.LastError)
	}
}

// seedLegacyMessages writes one page and one old contact message, the way the
// core did before the contact form became a plugin.
func seedLegacyMessages(t *testing.T, store *Store, site int64) {
	t.Helper()
	ctx := context.Background()
	res, err := store.DB.Write.ExecContext(ctx,
		`INSERT INTO pages (website_id, slug, title, status) VALUES ($1, 'kontakt', 'Kontakt', 'published')`, site)
	if err != nil {
		t.Fatal(err)
	}
	page, _ := res.LastInsertId()
	if _, err := store.DB.Write.ExecContext(ctx, `
		INSERT INTO form_messages (website_id, page_id, name, email, subject, body, created_at, read_at)
		VALUES ($1, $2, 'Anna', 'anna@example.org', 'Hallo', 'Guten Tag', '2025-01-02T03:04:05Z', NULL)`,
		site, page); err != nil {
		t.Fatal(err)
	}
}
