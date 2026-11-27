package plugin

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"modernc.org/sqlite"
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
		{"brackets", own + `INSERT INTO plugin_eins_a SELECT password FROM [users];`, "users"},
		{"double quoted", own + `INSERT INTO plugin_eins_a SELECT password FROM "users";`, "users"},
		{"backticks", own + "INSERT INTO plugin_eins_a SELECT password FROM `users`;", "users"},
		{"cte", own + `INSERT INTO plugin_eins_a WITH x AS (SELECT token FROM sessions) SELECT token FROM x;`, "sessions"},
		{"subselect", own + `INSERT INTO plugin_eins_a SELECT (SELECT password FROM users);`, "users"},
		{"count", own + `INSERT INTO plugin_eins_a SELECT count(*) FROM sessions;`, "sessions"},
		{"alias", own + `INSERT INTO plugin_eins_a SELECT s.token FROM sessions AS s;`, "sessions"},
		{"access password", own + `INSERT INTO plugin_eins_a SELECT access_password FROM pages;`, "access_password"},
		{"draft text", own + `INSERT INTO plugin_eins_a SELECT content_markdown FROM pages;`, "content_markdown"},
		{"websites", own + `INSERT INTO plugin_eins_a SELECT name FROM websites;`, "websites"},
		{"domains", own + `INSERT INTO plugin_eins_a SELECT domain FROM website_domains;`, "website_domains"},
		{"plugin store", own + `INSERT INTO plugin_eins_a SELECT value FROM plugin_store;`, "plugin_store"},
		{"plugin migrations", own + `INSERT INTO plugin_eins_a SELECT sha256 FROM plugin_migrations;`, "plugin_migrations"},
		{"plugins", own + `INSERT INTO plugin_eins_a SELECT manifest FROM plugins;`, "plugins"},
		{"extension", own + `INSERT INTO plugin_eins_a SELECT load_extension('x');`, "load_extension"},
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

// pluginStoreRows counts a plugin's rows and their values joined, so a test can
// see that a refused migration changed nothing of another plugin's.
func pluginStoreRows(t *testing.T, s *Store, id string) (int, string) {
	t.Helper()
	var n int
	var vals string
	if err := s.DB.Read.QueryRowContext(context.Background(),
		`SELECT COUNT(*), COALESCE(group_concat(key || '=' || value, ','), '') FROM plugin_store WHERE plugin_id = $1`, id).Scan(&n, &vals); err != nil {
		t.Fatal(err)
	}
	return n, vals
}

func TestAMigrationCannotWriteAnotherPluginsRows(t *testing.T) {
	const row = `INSERT INTO plugin_store (plugin_id, website_id, key, value, updated_at) `
	cases := []struct{ name, sql, word string }{
		{"foreign id", row + `VALUES ('zwei', 0, 'k2', 'evil', 'x');`, "zwei"},
		{"foreign id from a select", row + `SELECT 'zwei', 0, 'k3', 'evil', 'x';`, "zwei"},
		// The old check passed a statement that merely mentioned the plugin's own
		// id. The row it makes belongs to someone else all the same.
		{"own id only in a comment", row + `SELECT CASE WHEN length('x') > 0 THEN 'zwei' ELSE 'eins' END, 0, 'k4', 'evil', 'x'; -- 'eins'`, "zwei"},
		{"foreign id from the legacy table", row + `SELECT CASE WHEN m.id > 0 THEN 'zwei' ELSE 'eins' END, m.website_id, 'm' || m.id, 'evil', 'x' FROM form_messages m;`, "zwei"},
		{"replace a foreign row", `INSERT OR REPLACE INTO plugin_store (plugin_id, website_id, key, value, updated_at) VALUES ('zwei', 0, 'k1', 'evil', 'x');`, "zwei"},
		{"update", `UPDATE plugin_store SET value = 'evil';`, "plugin_store"},
		{"update own id", `UPDATE plugin_store SET value = 'evil' WHERE plugin_id = 'eins';`, "plugin_store"},
		{"delete foreign", `DELETE FROM plugin_store WHERE plugin_id = 'zwei';`, "plugin_store"},
		{"delete nameless", `DELETE FROM plugin_store;`, "plugin_store"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, ds := neuerSpeicher(t)
			ctx := context.Background()
			ws, err := ds.CreateWebsite(ctx, "Velowerkstatt", "")
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"eins", "zwei"} {
				if err := s.Install(ctx, paket(id)); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.StoreSet(ctx, "zwei", 0, "k1", "mine"); err != nil {
				t.Fatal(err)
			}
			// One legacy message, so the case that reads form_messages has a row.
			seedLegacyMessages(t, s, ws.ID)
			wantN, wantVals := pluginStoreRows(t, s, "zwei")

			expectRefused(t, s, c.sql, c.word)

			if n, vals := pluginStoreRows(t, s, "zwei"); n != wantN || vals != wantVals {
				t.Errorf("rows of plugin zwei changed: %d %q, were %d %q", n, vals, wantN, wantVals)
			}
			if n, _ := pluginStoreRows(t, s, "eins"); n != 0 {
				t.Errorf("%d row(s) of plugin eins survived the refusal", n)
			}
		})
	}
}

func TestAMigrationCannotChangeTheCountersOfCoreTables(t *testing.T) {
	for _, c := range []struct{ name, sql, word string }{
		{"update", `UPDATE sqlite_sequence SET seq = 0 WHERE name = 'websites';`, "websites"},
		{"delete", `DELETE FROM sqlite_sequence WHERE name = 'websites';`, "websites"},
		{"rename the entry", `UPDATE sqlite_sequence SET name = 'plugin_eins_a' WHERE name = 'websites';`, "websites"},
		{"insert", `INSERT INTO sqlite_sequence (name, seq) VALUES ('users', 0);`, "sqlite_sequence"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, ds := neuerSpeicher(t)
			ctx := context.Background()
			if _, err := ds.CreateWebsite(ctx, "Velowerkstatt", ""); err != nil {
				t.Fatal(err)
			}
			if err := s.Install(ctx, paket("eins")); err != nil {
				t.Fatal(err)
			}
			expectRefused(t, s, c.sql, c.word)
			var seq int64
			if err := s.DB.Read.QueryRowContext(ctx, `SELECT seq FROM sqlite_sequence WHERE name = 'websites'`).Scan(&seq); err != nil || seq != 1 {
				t.Errorf("counter of websites: %d, %v", seq, err)
			}
		})
	}

	// The plugin's own counters are its own: made, moved by a rename, dropped.
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	own := `CREATE TABLE plugin_eins_a (id INTEGER PRIMARY KEY AUTOINCREMENT, v TEXT);
INSERT INTO plugin_eins_a (v) VALUES ('x');
ALTER TABLE plugin_eins_a RENAME TO plugin_eins_b;
CREATE TABLE plugin_eins_c (id INTEGER PRIMARY KEY AUTOINCREMENT, v TEXT);
INSERT INTO plugin_eins_c (v) VALUES ('y');
DROP TABLE plugin_eins_c;`
	if err := s.ApplyMigrations(ctx, "eins", []Migration{{Name: "0001.sql", SQL: own}}); err != nil {
		t.Fatalf("own counters: %v", err)
	}
}

// A foreign key is a write the authorizer never hears about: SQLite does not
// ask about a REFERENCES clause, and the check it runs on an insert reads only
// columns a migration may read anyway. A child row pointing at plugins, pages
// or form_messages would then make the parent undeletable, the plugin that made
// it and every other one included (CR-01).
func TestAMigrationCannotPinHostRowsWithAForeignKey(t *testing.T) {
	for _, c := range []struct{ name, sql, word string }{
		{"plugins", `CREATE TABLE plugin_eins_pin (p TEXT REFERENCES plugins(id)); INSERT INTO plugin_eins_pin SELECT id FROM plugins;`, "plugins"},
		{"pages", `CREATE TABLE plugin_eins_pin (p INTEGER REFERENCES pages(id)); INSERT INTO plugin_eins_pin SELECT id FROM pages;`, "pages"},
		{"form messages", `CREATE TABLE plugin_eins_pin (p INTEGER REFERENCES form_messages(id)); INSERT INTO plugin_eins_pin SELECT id FROM form_messages;`, "form_messages"},
		{"table constraint", `CREATE TABLE plugin_eins_pin (p INTEGER, FOREIGN KEY (p) REFERENCES pages(id)); INSERT INTO plugin_eins_pin SELECT id FROM pages;`, "pages"},
		{"added column", `CREATE TABLE plugin_eins_pin (t TEXT); ALTER TABLE plugin_eins_pin ADD COLUMN p INTEGER REFERENCES pages(id); INSERT INTO plugin_eins_pin (p) SELECT id FROM pages;`, "pages"},
		{"another plugin", `CREATE TABLE plugin_eins_pin (p INTEGER REFERENCES plugin_zwei_a(id));`, "plugin_zwei_a"},
		{"a table that does not exist", `CREATE TABLE plugin_eins_pin (p INTEGER REFERENCES nosuchtable(id));`, "nosuchtable"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, ds := neuerSpeicher(t)
			ctx := context.Background()
			ws, err := ds.CreateWebsite(ctx, "Velowerkstatt", "")
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"eins", "zwei", "drei"} {
				if err := s.Install(ctx, paket(id)); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.ApplyMigrations(ctx, "zwei", []Migration{{Name: "0001.sql",
				SQL: `CREATE TABLE plugin_zwei_a (id INTEGER PRIMARY KEY); INSERT INTO plugin_zwei_a VALUES (1);`}}); err != nil {
				t.Fatal(err)
			}
			seedLegacyMessages(t, s, ws.ID)

			expectRefused(t, s, c.sql, c.word)

			// Nothing is pinned: an unrelated plugin, the plugin itself, a contact
			// message and the page it came from can all still go.
			if err := s.Remove(ctx, "drei"); err != nil {
				t.Errorf("removing an unrelated plugin: %v", err)
			}
			if err := s.Remove(ctx, "eins"); err != nil {
				t.Errorf("removing the plugin itself: %v", err)
			}
			if _, err := s.DB.Write.ExecContext(ctx, `DELETE FROM form_messages`); err != nil {
				t.Errorf("deleting the contact messages: %v", err)
			}
			if _, err := s.DB.Write.ExecContext(ctx, `DELETE FROM pages`); err != nil {
				t.Errorf("deleting the pages: %v", err)
			}
		})
	}

	// A key between the plugin's own tables is its own business.
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	own := `CREATE TABLE plugin_eins_a (id INTEGER PRIMARY KEY);
CREATE TABLE plugin_eins_b (a INTEGER REFERENCES plugin_eins_a(id) ON DELETE CASCADE);
INSERT INTO plugin_eins_a VALUES (1);
INSERT INTO plugin_eins_b VALUES (1);`
	if err := s.ApplyMigrations(ctx, "eins", []Migration{{Name: "0001.sql", SQL: own}}); err != nil {
		t.Fatalf("a key between own tables: %v", err)
	}
	if err := s.Remove(ctx, "eins"); err != nil {
		t.Errorf("removing a plugin with its own keys: %v", err)
	}
}

func TestARenameOutOfThePrefixIsRefused(t *testing.T) {
	for _, c := range []struct{ name, sql, word string }{
		{"to a core name", `CREATE TABLE plugin_eins_a (a TEXT); ALTER TABLE plugin_eins_a RENAME TO users2;`, "users2"},
		{"to another plugin", `CREATE TABLE plugin_eins_a (a TEXT); ALTER TABLE plugin_eins_a RENAME TO plugin_zwei_a;`, "plugin_zwei_a"},
		{"with a text key", `CREATE TABLE plugin_eins_a (k TEXT PRIMARY KEY); ALTER TABLE plugin_eins_a RENAME TO users2;`, "users2"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, _ := neuerSpeicher(t)
			if err := s.Install(context.Background(), paket("eins")); err != nil {
				t.Fatal(err)
			}
			expectRefused(t, s, c.sql, c.word)
			var n int
			if err := s.DB.Read.QueryRowContext(context.Background(),
				`SELECT COUNT(*) FROM sqlite_schema WHERE name IN ('users2', 'plugin_zwei_a')`).Scan(&n); err != nil || n != 0 {
				t.Errorf("renamed table survived: %d, %v", n, err)
			}
		})
	}

	// Inside the prefix a rename is fine, and so is a table whose text key makes
	// SQLite add an index of its own.
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	ok := `CREATE TABLE plugin_eins_a (k TEXT PRIMARY KEY, v TEXT);
ALTER TABLE plugin_eins_a RENAME TO plugin_eins_b;
CREATE TABLE plugin_eins_c (a TEXT);`
	if err := s.ApplyMigrations(ctx, "eins", []Migration{{Name: "0001.sql", SQL: ok}}); err != nil {
		t.Fatalf("rename inside the prefix: %v", err)
	}
}

func TestAMigrationReadsOnlyTheListedColumns(t *testing.T) {
	cases := []struct {
		name, sql, word string
		refused         bool
	}{
		{"a column the core adds later", `INSERT INTO plugin_eins_a SELECT ip FROM form_messages;`, "ip", true},
		{"a column of pages that is not listed", `INSERT INTO plugin_eins_a SELECT title FROM pages;`, "title", true},
		{"the listed columns", `INSERT INTO plugin_eins_a SELECT website_id || created_at || id || name || email || subject || body || COALESCE(read_at, '') || COALESCE(page_id, '') FROM form_messages;
INSERT INTO plugin_eins_a SELECT id || slug FROM pages;`, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := neuerSpeicher(t)
			ctx := context.Background()
			if err := s.Install(ctx, paket("eins")); err != nil {
				t.Fatal(err)
			}
			// Stands in for a column the core may add one day.
			if _, err := s.DB.Write.ExecContext(ctx, `ALTER TABLE form_messages ADD COLUMN ip TEXT`); err != nil {
				t.Fatal(err)
			}
			q := `CREATE TABLE plugin_eins_a (t TEXT); ` + c.sql
			if c.refused {
				expectRefused(t, s, q, c.word)
				return
			}
			if err := s.ApplyMigrations(ctx, "eins", []Migration{{Name: "0001.sql", SQL: q}}); err != nil {
				t.Fatalf("listed columns refused: %v", err)
			}
		})
	}
}

func TestPluginNamespacesDoNotOverlap(t *testing.T) {
	ctx := context.Background()

	t.Run("install refuses an overlap in either direction", func(t *testing.T) {
		s, _ := neuerSpeicher(t)
		if err := s.Install(ctx, paket("nicht-gefunden")); err != nil {
			t.Fatal(err)
		}
		err := s.Install(ctx, paket("nicht"))
		if err == nil || !strings.Contains(err.Error(), "nicht-gefunden") || !strings.Contains(err.Error(), `"nicht"`) {
			t.Fatalf("installing nicht beside nicht-gefunden: %v", err)
		}

		s2, _ := neuerSpeicher(t)
		if err := s2.Install(ctx, paket("nicht")); err != nil {
			t.Fatal(err)
		}
		err = s2.Install(ctx, paket("nicht-gefunden"))
		if err == nil || !strings.Contains(err.Error(), "nicht-gefunden") || !strings.Contains(err.Error(), `"nicht"`) {
			t.Fatalf("installing nicht-gefunden beside nicht: %v", err)
		}
		// A reinstall is not an overlap with itself.
		if err := s2.Install(ctx, paket("nicht")); err != nil {
			t.Fatalf("reinstalling nicht: %v", err)
		}
		// Neighbours that merely share letters do not overlap.
		if err := s2.Install(ctx, paket("nichtig")); err != nil {
			t.Fatalf("installing nichtig beside nicht: %v", err)
		}
	})

	// A database that already holds both, as an installation from before the
	// rule could: the rows are written by SQL, past Install.
	pair := func(t *testing.T) *Store {
		s, _ := neuerSpeicher(t)
		for _, id := range []string{"nicht", "nicht-gefunden"} {
			_, err := s.DB.Write.ExecContext(ctx, `INSERT INTO plugins (id, name, version, manifest, sha256, enabled, installed_at, updated_at, last_error)
				VALUES ($1, $1, '1', '{}', 'x', 0, 'x', 'x', '')`, id)
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.DB.Write.ExecContext(ctx, `CREATE TABLE plugin_nicht_gefunden_a (t TEXT); INSERT INTO plugin_nicht_gefunden_a VALUES ('mine')`); err != nil {
			t.Fatal(err)
		}
		return s
	}
	for _, c := range []struct{ name, sql string }{
		{"create", `CREATE TABLE plugin_nicht_gefunden_b (t TEXT);`},
		{"read", `CREATE TABLE plugin_nicht_x (t TEXT); INSERT INTO plugin_nicht_x SELECT t FROM plugin_nicht_gefunden_a;`},
		{"insert", `INSERT INTO plugin_nicht_gefunden_a VALUES ('evil');`},
		{"alter", `ALTER TABLE plugin_nicht_gefunden_a ADD COLUMN u TEXT;`},
		{"drop", `DROP TABLE plugin_nicht_gefunden_a;`},
		{"rename in", `CREATE TABLE plugin_nicht_x (t TEXT); ALTER TABLE plugin_nicht_x RENAME TO plugin_nicht_gefunden_y;`},
		{"rename out", `ALTER TABLE plugin_nicht_gefunden_a RENAME TO plugin_nicht_gefunden_z;`},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := pair(t)
			err := s.ApplyMigrations(ctx, "nicht", []Migration{{Name: "0001.sql", SQL: c.sql}})
			if err == nil || !strings.Contains(err.Error(), "refused") || !strings.Contains(err.Error(), "plugin_nicht_gefunden_") {
				t.Fatalf("migration of nicht reached the other namespace: %v", err)
			}
			var n int
			var v string
			if err := s.DB.Read.QueryRowContext(ctx, `SELECT COUNT(*), MIN(t) FROM plugin_nicht_gefunden_a`).Scan(&n, &v); err != nil || n != 1 || v != "mine" {
				t.Errorf("table of nicht-gefunden changed: %d %q %v", n, v, err)
			}
		})
	}

	// The longer plugin keeps its own tables, and the shorter one its own.
	s := pair(t)
	if err := s.ApplyMigrations(ctx, "nicht-gefunden", []Migration{{Name: "0001.sql",
		SQL: `CREATE TABLE plugin_nicht_gefunden_b (t TEXT); INSERT INTO plugin_nicht_gefunden_b SELECT t FROM plugin_nicht_gefunden_a;`}}); err != nil {
		t.Fatalf("nicht-gefunden in its own namespace: %v", err)
	}
	if err := s.ApplyMigrations(ctx, "nicht", []Migration{{Name: "0001.sql", SQL: `CREATE TABLE plugin_nicht_x (t TEXT);`}}); err != nil {
		t.Fatalf("nicht in its own namespace: %v", err)
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

// coreWrites asserts what PLG-03 demands: the core writes through the shared
// pool, and a plugin's last error can be recorded, at once.
func coreWrites(t *testing.T, s *Store, id string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := s.DB.Write.ExecContext(ctx, `UPDATE websites SET name = name`); err != nil {
		t.Fatalf("core write: %v", err)
	}
	if err := s.SetError(ctx, id, "x"); err != nil {
		t.Fatalf("SetError: %v", err)
	}
}

func TestTheCoreWritesAfterARefusedMigration(t *testing.T) {
	cases := []struct{ name, sql string }{
		{"denial", `CREATE TABLE plugin_eins_a (t TEXT); INSERT INTO plugin_eins_a SELECT token FROM sessions;`},
		{"hook violation", `INSERT INTO plugin_store (plugin_id, website_id, key, value, updated_at) VALUES ('zwei', 0, 'k', 'v', 'x');`},
		{"post check", `CREATE TABLE plugin_eins_a (t TEXT); ALTER TABLE plugin_eins_a RENAME TO users2;`},
		{"engine error", `CREATE TABLE plugin_eins_a (t TEXT); SELECT nosuchfunction();`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := neuerSpeicher(t)
			ctx := context.Background()
			for _, id := range []string{"eins", "zwei"} {
				if err := s.Install(ctx, paket(id)); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.ApplyMigrations(ctx, "eins", []Migration{{Name: "0001.sql", SQL: c.sql}}); err == nil {
				t.Fatal("not refused")
			}
			coreWrites(t, s, "eins")

			// And a legitimate migration of another plugin goes through.
			if err := s.ApplyMigrations(ctx, "zwei", []Migration{{Name: "0001.sql", SQL: `CREATE TABLE plugin_zwei_a (t TEXT);`}}); err != nil {
				t.Fatalf("next migration: %v", err)
			}
		})
	}
}

func TestTheCoreWritesWhenTheGuardCannotBeRemoved(t *testing.T) {
	for _, c := range []struct{ name, sql string }{
		{"applied", `CREATE TABLE plugin_eins_a (t TEXT);`},
		{"refused", `INSERT INTO plugin_store (plugin_id, website_id, key, value, updated_at) VALUES ('zwei', 0, 'k', 'v', 'x');`},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, _ := neuerSpeicher(t)
			ctx := context.Background()
			for _, id := range []string{"eins", "zwei"} {
				if err := s.Install(ctx, paket(id)); err != nil {
					t.Fatal(err)
				}
			}
			// The removal fails and leaves a policy on the connection that would
			// refuse every core write. Only a connection that is thrown away
			// gets rid of it.
			s.removeGuard = func(dc any) error {
				if err := dc.(sqlite.AuthorizerRegisterer).RegisterAuthorizer(
					func(sqlite.AuthorizerActionCode, string, string, string, string) sqlite.AuthorizerReturnCode {
						return sqlite.AuthorizerDeny
					}); err != nil {
					return err
				}
				return errors.New("cannot take the guard off")
			}
			done := make(chan struct{})
			go func() {
				defer close(done)
				_ = s.ApplyMigrations(ctx, "eins", []Migration{{Name: "0001.sql", SQL: c.sql}})
			}()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("the migration did not return")
			}
			coreWrites(t, s, "eins")
		})
	}
}

func TestAMigrationThatRunsTooLongIsStopped(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	s.migrationTimeout = 200 * time.Millisecond

	runaway := `CREATE TABLE plugin_eins_a (x INTEGER);
WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c) INSERT INTO plugin_eins_a SELECT x FROM c;`
	start := time.Now()
	err := s.ApplyMigrations(ctx, "eins", []Migration{{Name: "0002.sql", SQL: runaway}})
	if err == nil {
		t.Fatal("the runaway migration was not stopped")
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("stopped after %v", took)
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Errorf("error %q", err)
	}
	var n int
	if err := s.DB.Read.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM plugin_migrations WHERE plugin_id = 'eins' AND name = '0002.sql') +
		(SELECT COUNT(*) FROM sqlite_schema WHERE name = 'plugin_eins_a')`).Scan(&n); err != nil || n != 0 {
		t.Errorf("traces of the stopped migration: %d, %v", n, err)
	}
	coreWrites(t, s, "eins")
}

func TestACallerThatGivesUpDoesNotCutAMigrationShort(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	if err := s.Install(ctx, paket("eins")); err != nil {
		t.Fatal(err)
	}
	// The request that triggered the install ends while the migration runs. Its
	// context is not the migration's: a rollback from outside would come while
	// the policy is armed.
	short, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	slow := `CREATE TABLE plugin_eins_a (x INTEGER);
WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x < 300000) INSERT INTO plugin_eins_a SELECT x FROM c;`
	start := time.Now()
	if err := s.ApplyMigrations(short, "eins", []Migration{{Name: "0001.sql", SQL: slow}}); err != nil {
		t.Fatalf("the migration was cut short: %v", err)
	}
	if time.Since(start) < 10*time.Millisecond {
		t.Skip("the migration finished before the caller gave up; nothing was tested")
	}
	var n int
	if err := s.DB.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM plugin_eins_a`).Scan(&n); err != nil || n != 300000 {
		t.Errorf("%d rows, %v", n, err)
	}
	coreWrites(t, s, "eins")
}
