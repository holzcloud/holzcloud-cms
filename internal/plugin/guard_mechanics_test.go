package plugin

// These tests pin what the migration guard relies on in the SQLite driver. They
// are not about the guard's policy: they fail when a future modernc.org/sqlite
// changes the behaviour the policy is built on, so that a driver bump is a
// decision and not a silent hole.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// callback is one authorizer call, as the driver hands it over.
type callback struct {
	action                           sqlite.AuthorizerActionCode
	arg1, arg2, database, triggerOrV string
}

func (c callback) String() string {
	return fmt.Sprintf("%s(%q, %q, db=%q, in=%q)", actionName(c.action), c.arg1, c.arg2, c.database, c.triggerOrV)
}

// recordingConn opens a Tx on a dedicated write connection and installs an
// allow-all authorizer that records every call it receives.
func recordingConn(t *testing.T, s *Store) (*sql.Conn, *sql.Tx, func() []callback) {
	t.Helper()
	ctx := context.Background()
	conn, err := s.DB.Write.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.Rollback() })

	var mu sync.Mutex
	var got []callback
	err = conn.Raw(func(dc any) error {
		reg, ok := dc.(sqlite.AuthorizerRegisterer)
		if !ok {
			return errors.New("driver connection has no authorizer")
		}
		return reg.RegisterAuthorizer(func(a sqlite.AuthorizerActionCode, a1, a2, dbn, trig string) sqlite.AuthorizerReturnCode {
			mu.Lock()
			got = append(got, callback{a, a1, a2, dbn, trig})
			mu.Unlock()
			return sqlite.AuthorizerOK
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	return conn, tx, func() []callback {
		mu.Lock()
		defer mu.Unlock()
		out := slices.Clone(got)
		got = nil
		return out
	}
}

func readsOf(cbs []callback, table string) []string {
	var cols []string
	for _, c := range cbs {
		if c.action == sqlite.AuthRead && c.arg1 == table {
			cols = append(cols, c.arg2)
		}
	}
	return cols
}

func TestTheAuthorizerSeesResolvedTableNames(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	// The plugin row has to exist before the Tx takes the only write connection.
	if err := s.Install(ctx, paket("kontaktformular")); err != nil {
		t.Fatal(err)
	}
	_, tx, take := recordingConn(t, s)

	for name, q := range map[string]string{
		"single quotes": `SELECT token FROM 'sessions'`,
		"brackets":      `SELECT token FROM [sessions]`,
		"double quotes": `SELECT token FROM "sessions"`,
		"backticks":     "SELECT token FROM `sessions`",
		"cte":           `WITH x AS (SELECT token FROM sessions) SELECT * FROM x`,
		"subselect":     `SELECT (SELECT token FROM sessions)`,
		"alias":         `SELECT a.token FROM sessions AS a`,
	} {
		take()
		rows, err := tx.QueryContext(ctx, q)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		rows.Close()
		got := readsOf(take(), "sessions")
		if !slices.Contains(got, "token") {
			t.Errorf("%s: no AuthRead of sessions.token, got reads %v", name, got)
		}
	}

	// The callback list of the shipped migration, which is the evidence for the
	// policy. The text is read from the committed file, never copied here.
	src, err := os.ReadFile("../../plugins/kontaktformular/migrations/0001_uebernahme.sql")
	if err != nil {
		t.Fatal(err)
	}
	take()
	if _, err := tx.ExecContext(ctx, string(src)); err != nil {
		t.Fatal(err)
	}
	for _, c := range take() {
		t.Logf("kontaktformular 0001: %s", c)
	}

	for _, q := range []string{
		`CREATE TABLE plugin_eins_a (t TEXT)`,
		`CREATE TABLE plugin_eins_b (k TEXT PRIMARY KEY, n INTEGER)`,
		`CREATE INDEX plugin_eins_b_n ON plugin_eins_b (n)`,
		`CREATE TABLE plugin_eins_c (id INTEGER PRIMARY KEY AUTOINCREMENT, v TEXT)`,
		`INSERT INTO plugin_eins_c (v) VALUES ('x')`,
		`INSERT INTO plugin_eins_a SELECT v FROM plugin_eins_c`,
		`UPDATE plugin_eins_a SET t = 'y'`,
		`DELETE FROM plugin_eins_a`,
		`ALTER TABLE plugin_eins_a ADD COLUMN u TEXT`,
		`DROP INDEX plugin_eins_b_n`,
		`DROP TABLE plugin_eins_b`,
	} {
		take()
		if _, err := tx.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		for _, c := range take() {
			t.Logf("%s => %s", q, c)
		}
	}
}

func TestARawCallbackReturningBadConnDiscardsTheConnection(t *testing.T) {
	ctx := context.Background()

	// Without the discard the armed connection goes back into the pool and every
	// later core write is refused: this is what the guard exists to prevent.
	poisoned, _ := neuerSpeicher(t)
	pc, err := poisoned.DB.Write.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := pc.Raw(denyAllButTransactions); err != nil {
		t.Fatal(err)
	}
	pc.Close()
	if _, err := poisoned.DB.Write.ExecContext(ctx, `UPDATE websites SET name = name`); err == nil ||
		!strings.Contains(err.Error(), "authorization denied") {
		t.Fatalf("core write on a connection that kept its policy: %v", err)
	}

	s, _ := neuerSpeicher(t)
	conn, err := s.DB.Write.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Raw(denyAllButTransactions); err != nil {
		t.Fatal(err)
	}
	// The Tx has to end before Raw may discard the connection: closing the Conn
	// takes the lock the open Tx holds for reading.
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	err = conn.Raw(func(any) error { return driver.ErrBadConn })
	if !errors.Is(err, driver.ErrBadConn) {
		t.Fatalf("Raw returned %v, want driver.ErrBadConn", err)
	}
	if err := conn.Close(); !errors.Is(err, sql.ErrConnDone) {
		t.Fatalf("Close after a discard returned %v, want sql.ErrConnDone", err)
	}

	// The pool opens a fresh connection with no policy on it.
	if _, err := s.DB.Write.ExecContext(ctx, `UPDATE websites SET name = name`); err != nil {
		t.Fatalf("core write after a discarded connection: %v", err)
	}
}

// denyAllButTransactions installs a policy that is never removed, what a failed
// removal would leave. It lets the transaction end, since COMMIT and ROLLBACK
// are authorized like any statement, and denies everything else.
func denyAllButTransactions(dc any) error {
	return dc.(sqlite.AuthorizerRegisterer).RegisterAuthorizer(
		func(a sqlite.AuthorizerActionCode, _, _, _, _ string) sqlite.AuthorizerReturnCode {
			if a == sqlite.AuthTransaction {
				return sqlite.AuthorizerOK
			}
			return sqlite.AuthorizerDeny
		})
}

func TestThePreUpdateHookSeesThePluginColumn(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	for _, id := range []string{"eins", "zwei"} {
		if err := s.Install(ctx, paket(id)); err != nil {
			t.Fatal(err)
		}
	}

	conn, err := s.DB.Write.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	type event struct {
		op       int32
		table    string
		pluginID string
	}
	var mu sync.Mutex
	var events []event
	err = conn.Raw(func(dc any) error {
		reg, ok := dc.(sqlite.HookRegisterer)
		if !ok {
			return errors.New("driver connection has no hooks")
		}
		reg.RegisterPreUpdateHook(func(d sqlite.SQLitePreUpdateData) {
			dest := make([]any, d.Count())
			var rerr error
			if d.Op == sqlite3.SQLITE_DELETE {
				rerr = d.Old(dest...)
			} else {
				rerr = d.New(dest...)
			}
			id := fmt.Sprint(dest[0])
			if rerr != nil {
				id = "error: " + rerr.Error()
			}
			mu.Lock()
			events = append(events, event{d.Op, d.TableName, id})
			mu.Unlock()
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	take := func() []event {
		mu.Lock()
		defer mu.Unlock()
		out := slices.Clone(events)
		events = nil
		return out
	}

	exec := func(verb, id, v string) {
		t.Helper()
		q := fmt.Sprintf(`INSERT %s INTO plugin_store (plugin_id, website_id, key, value, updated_at)
			VALUES ('%s', 0, 'k', '%s', 'x')`, verb, id, v)
		if _, err := tx.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}

	exec("", "eins", "1")
	if got := take(); len(got) != 1 || got[0] != (event{sqlite3.SQLITE_INSERT, "plugin_store", "eins"}) {
		t.Errorf("INSERT: %+v", got)
	}

	// OR REPLACE over an existing key: the delete of the old row has to be seen,
	// or a plugin could remove a foreign row without ever inserting one.
	exec("OR REPLACE", "eins", "2")
	got := take()
	want := []event{{sqlite3.SQLITE_DELETE, "plugin_store", "eins"}, {sqlite3.SQLITE_INSERT, "plugin_store", "eins"}}
	if !slices.Equal(got, want) {
		t.Errorf("INSERT OR REPLACE: %+v, want %+v", got, want)
	}

	// OR IGNORE over an existing key writes nothing and so reports nothing.
	exec("OR IGNORE", "eins", "3")
	if got := take(); len(got) != 0 {
		t.Errorf("INSERT OR IGNORE: %+v, want nothing", got)
	}

	// UPDATE reports the old row through Old and the new one through New.
	if _, err := tx.ExecContext(ctx, `UPDATE plugin_store SET plugin_id = 'zwei'`); err != nil {
		t.Fatal(err)
	}
	if got := take(); len(got) != 1 || got[0].op != sqlite3.SQLITE_UPDATE || got[0].pluginID != "zwei" {
		t.Errorf("UPDATE: %+v", got)
	}
}

func TestARenameIsAuthorizedByItsOldNameOnly(t *testing.T) {
	s, _ := neuerSpeicher(t)
	ctx := context.Background()
	_, tx, take := recordingConn(t, s)

	if _, err := tx.ExecContext(ctx, `CREATE TABLE plugin_eins_a (a TEXT)`); err != nil {
		t.Fatal(err)
	}
	take()
	if _, err := tx.ExecContext(ctx, `ALTER TABLE plugin_eins_a RENAME TO users2`); err != nil {
		t.Fatal(err)
	}
	var alters []callback
	for _, c := range take() {
		if strings.Contains(c.arg1, "users2") || strings.Contains(c.arg2, "users2") {
			t.Errorf("a callback names the new table: %s", c)
		}
		if c.action == sqlite.AuthAlterTable {
			alters = append(alters, c)
		}
	}
	if len(alters) != 1 || alters[0].arg2 != "plugin_eins_a" {
		t.Errorf("alter callbacks: %v", alters)
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_schema WHERE name = 'users2'`).Scan(&n); err != nil || n != 1 {
		t.Errorf("users2 in sqlite_schema: %d, %v", n, err)
	}

	// A TEXT primary key adds an autoindex whose tbl_name is the table.
	if _, err := tx.ExecContext(ctx, `CREATE TABLE plugin_eins_b (k TEXT PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatal(err)
	}
	var name, tbl string
	if err := tx.QueryRowContext(ctx,
		`SELECT name, tbl_name FROM sqlite_schema WHERE type = 'index' AND tbl_name = 'plugin_eins_b'`).Scan(&name, &tbl); err != nil {
		t.Fatal(err)
	}
	if name != "sqlite_autoindex_plugin_eins_b_1" {
		t.Errorf("autoindex is called %q", name)
	}
}
