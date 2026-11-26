package plugin

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"modernc.org/sqlite"
)

// A plugin's migration is SQL from an uploaded archive. It runs on a write
// connection of its own with a policy installed in SQLite's authorizer, which
// is asked about every table, column and function a statement touches, by the
// names SQLite resolved: quoting, aliasing, a CTE or a subselect change nothing
// about what is asked. The policy denies by default.
//
// The policy is armed only around the plugin's SQL and the connection is
// handed back to the pool without it. There is one write connection, shared by
// the whole core: a policy left on it would make every core write fail.

// migrationPrefix is the table-name prefix a plugin owns.
func migrationPrefix(id string) string {
	return "plugin_" + strings.ReplaceAll(id, "-", "_") + "_"
}

// formMessagesColumns are the columns of form_messages a migration may read:
// the ones the shipped contact-form migration reads. A column the core adds
// later is not on the list and so stays out of reach.
var formMessagesColumns = map[string]bool{
	"website_id": true, "created_at": true, "id": true, "name": true,
	"email": true, "subject": true, "body": true, "read_at": true, "page_id": true,
}

// pagesColumns are the columns of pages a migration may read: the slug of the
// page a message came from, and the key to find it by. Never access_password,
// never the text of a draft.
var pagesColumns = map[string]bool{"id": true, "slug": true}

// deniedFunctions are called by name from SQL and reach outside the database.
var deniedFunctions = map[string]bool{
	"load_extension": true, "readfile": true, "writefile": true,
	"edit": true, "fts3_tokenizer": true,
}

// migrationGuard is the policy for one plugin's migration.
type migrationGuard struct {
	id     string
	prefix string

	// armed is true only while the plugin's own SQL runs. Before and after it
	// the connection runs the host's statements (COMMIT, reading the schema,
	// recording the migration), and those have to pass.
	armed atomic.Bool

	mu     sync.Mutex
	denial string
	// inDDL is true from the moment SQLite announces a schema change of an own
	// table until the next statement that is not one: SQLite reads its own
	// schema table while it renames, adds a column or drops, and those reads are
	// asked for like any other. See mayRead.
	inDDL bool
}

func newMigrationGuard(id string) *migrationGuard {
	return &migrationGuard{id: id, prefix: migrationPrefix(id)}
}

// refusal returns what the guard refused, or "".
func (g *migrationGuard) refusal() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.denial
}

// deny remembers the first denial and answers it. The first one is the cause;
// what follows in the same statement are consequences.
func (g *migrationGuard) deny(what string) sqlite.AuthorizerReturnCode {
	g.mu.Lock()
	if g.denial == "" {
		g.denial = what
	}
	g.mu.Unlock()
	return sqlite.AuthorizerDeny
}

// asciiLower folds the way SQLite folds identifiers: ASCII only.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// owns reports whether a table or index name lies in the plugin's own
// namespace. SQLite compares names without regard to case.
func (g *migrationGuard) owns(name string) bool {
	n := asciiLower(name)
	return strings.HasPrefix(n, g.prefix) && len(n) > len(g.prefix)
}

// ownAutoIndex reports whether name is the index SQLite makes for a UNIQUE or
// PRIMARY KEY constraint of one of the plugin's own tables.
func (g *migrationGuard) ownAutoIndex(name, table string) bool {
	n := asciiLower(name)
	t := asciiLower(table)
	return g.owns(t) && strings.HasPrefix(n, "sqlite_autoindex_"+t+"_")
}

// isSchemaTable names SQLite's own bookkeeping tables. DDL writes to them
// internally; a user statement that writes to them is refused by SQLite itself.
func isSchemaTable(name string) bool {
	switch asciiLower(name) {
	case "sqlite_master", "sqlite_schema":
		return true
	}
	return false
}

// authorize is the SQLite authorizer callback. It must not touch the
// connection. While the guard is not armed it allows everything.
func (g *migrationGuard) authorize(action sqlite.AuthorizerActionCode, arg1, arg2, database, triggerOrView string) sqlite.AuthorizerReturnCode {
	if !g.armed.Load() {
		return sqlite.AuthorizerOK
	}
	// Statements inside a trigger or a view are never the plugin's own text, and
	// another database is never the plugin's to reach.
	if triggerOrView != "" {
		return g.deny("running inside " + triggerOrView)
	}
	if database != "" && database != "main" {
		return g.deny("reaching database " + database)
	}

	switch action {
	case sqlite.AuthCreateTable:
		// Only a table in the plugin's own namespace.
		if g.owns(arg1) {
			return g.schemaChange()
		}
	case sqlite.AuthCreateIndex:
		// An index on an own table, named in the own namespace or one of the
		// automatic indexes of a UNIQUE or PRIMARY KEY constraint.
		if g.owns(arg2) && (g.owns(arg1) || g.ownAutoIndex(arg1, arg2)) {
			return g.schemaChange()
		}
	case sqlite.AuthDropTable:
		if g.owns(arg1) {
			return g.schemaChange()
		}
	case sqlite.AuthDropIndex:
		if g.owns(arg2) && (g.owns(arg1) || g.ownAutoIndex(arg1, arg2)) {
			return g.schemaChange()
		}
	case sqlite.AuthAlterTable:
		// arg2 is the table as it is named now. A rename is judged by that old
		// name only; the new one is looked at after the statement has run.
		if g.owns(arg2) {
			return g.schemaChange()
		}
	case sqlite.AuthReindex:
		// CREATE INDEX asks for it on the index it has just made.
		if g.owns(arg1) || g.ownAutoIndex(arg1, "") {
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthInsert:
		switch {
		case g.owns(arg1):
			g.statement()
			return sqlite.AuthorizerOK
		case asciiLower(arg1) == "plugin_store":
			// The shipped contact form copies its old messages here.
			g.statement()
			return sqlite.AuthorizerOK
		case isSchemaTable(arg1):
			// Every CREATE writes its own row. SQLite refuses a user statement
			// that does it directly.
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthUpdate:
		switch {
		case g.owns(arg1):
			g.statement()
			return sqlite.AuthorizerOK
		case isSchemaTable(arg1):
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthDelete:
		// DROP TABLE deletes the table's rows and its entry in the schema. The
		// statement is not told apart from a DROP here: it is already running.
		if g.owns(arg1) || isSchemaTable(arg1) {
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthRead:
		if g.mayRead(arg1, arg2) {
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthPragma:
		// The check ALTER TABLE ADD COLUMN runs over the table's existing rows,
		// asked for by the plugin's own table. No other pragma.
		if asciiLower(arg1) == "quick_check" && g.owns(arg2) {
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthSelect, sqlite.AuthRecursive:
		g.statement()
		return sqlite.AuthorizerOK
	case sqlite.AuthFunction:
		if !deniedFunctions[asciiLower(arg2)] {
			return sqlite.AuthorizerOK
		}
	}
	return g.deny(describeAction(action, arg1, arg2))
}

// schemaChange notes that a schema change of an own object has been announced
// and allows it.
func (g *migrationGuard) schemaChange() sqlite.AuthorizerReturnCode {
	g.mu.Lock()
	g.inDDL = true
	g.mu.Unlock()
	return sqlite.AuthorizerOK
}

// statement notes that a statement that reads or writes data has begun. The
// first thing SQLite asks about such a statement is its SELECT, INSERT or
// UPDATE, so by the time any column is asked for the flag is already down.
func (g *migrationGuard) statement() {
	g.mu.Lock()
	g.inDDL = false
	g.mu.Unlock()
}

func (g *migrationGuard) schemaReadAllowed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.inDDL
}

// mayRead is the read allowlist.
func (g *migrationGuard) mayRead(table, column string) bool {
	t := asciiLower(table)
	c := asciiLower(column)
	switch {
	case g.owns(t):
		return true
	case t == "form_messages":
		// An empty column is what COUNT(*) asks for.
		return c == "" || formMessagesColumns[c]
	case t == "pages":
		return c == "" || pagesColumns[c]
	case t == "plugins":
		// The foreign-key parent of plugin_store.plugin_id: SQLite asks for it
		// whenever a row is inserted there. Nothing else of plugins.
		return c == "id"
	case t == "pragma_quick_check":
		// ALTER TABLE ADD COLUMN on a STRICT table or one with rows checks the
		// existing rows through it. It answers only with integrity messages.
		return c == "quick_check"
	case isSchemaTable(t):
		// SQLite reads its schema table while it changes the schema: the row id
		// of a new entry, and the type, name, rootpage and text of the entries
		// a rename, an added column or a drop touches. The same read written as
		// a query (SELECT sql FROM sqlite_master) would show the plugin the
		// definition of every table of the host, so a column other than the row
		// id is allowed only while such a schema change is running, and never
		// in a statement that reads or writes data.
		return c == "rowid" || g.schemaReadAllowed()
	}
	return false
}

// describeAction words a denial for the person who installs the plugin.
func describeAction(action sqlite.AuthorizerActionCode, arg1, arg2 string) string {
	switch action {
	case sqlite.AuthRead:
		if arg2 == "" {
			return "reading " + arg1
		}
		return "reading " + arg1 + "." + arg2
	case sqlite.AuthInsert:
		return "inserting into " + arg1
	case sqlite.AuthUpdate:
		return "updating " + arg1 + "." + arg2
	case sqlite.AuthDelete:
		return "deleting from " + arg1
	case sqlite.AuthFunction:
		return "calling " + arg2
	}
	parts := []string{actionName(action)}
	for _, a := range []string{arg1, arg2} {
		if a != "" {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, " ")
}

var actionNames = map[sqlite.AuthorizerActionCode]string{
	sqlite.AuthCreateIndex: "CREATE INDEX", sqlite.AuthCreateTable: "CREATE TABLE",
	sqlite.AuthCreateTempIndex: "CREATE TEMP INDEX", sqlite.AuthCreateTempTable: "CREATE TEMP TABLE",
	sqlite.AuthCreateTempTrigger: "CREATE TEMP TRIGGER", sqlite.AuthCreateTempView: "CREATE TEMP VIEW",
	sqlite.AuthCreateTrigger: "CREATE TRIGGER", sqlite.AuthCreateView: "CREATE VIEW",
	sqlite.AuthDelete: "DELETE", sqlite.AuthDropIndex: "DROP INDEX", sqlite.AuthDropTable: "DROP TABLE",
	sqlite.AuthDropTempIndex: "DROP TEMP INDEX", sqlite.AuthDropTempTable: "DROP TEMP TABLE",
	sqlite.AuthDropTempTrigger: "DROP TEMP TRIGGER", sqlite.AuthDropTempView: "DROP TEMP VIEW",
	sqlite.AuthDropTrigger: "DROP TRIGGER", sqlite.AuthDropView: "DROP VIEW",
	sqlite.AuthInsert: "INSERT", sqlite.AuthPragma: "PRAGMA", sqlite.AuthRead: "READ",
	sqlite.AuthSelect: "SELECT", sqlite.AuthTransaction: "TRANSACTION", sqlite.AuthUpdate: "UPDATE",
	sqlite.AuthAttach: "ATTACH", sqlite.AuthDetach: "DETACH", sqlite.AuthAlterTable: "ALTER TABLE",
	sqlite.AuthReindex: "REINDEX", sqlite.AuthAnalyze: "ANALYZE", sqlite.AuthCreateVTable: "CREATE VIRTUAL TABLE",
	sqlite.AuthDropVTable: "DROP VIRTUAL TABLE", sqlite.AuthFunction: "FUNCTION",
	sqlite.AuthSavepoint: "SAVEPOINT", sqlite.AuthRecursive: "RECURSIVE",
}

func actionName(a sqlite.AuthorizerActionCode) string {
	if n, ok := actionNames[a]; ok {
		return n
	}
	return "an unknown action"
}

// install puts the guard on a driver connection. A connection that cannot take
// it is a refusal: the migration must not run unguarded.
func (g *migrationGuard) install(dc any) error {
	reg, ok := dc.(sqlite.AuthorizerRegisterer)
	if !ok {
		return errors.New("the driver connection has no authorizer")
	}
	return reg.RegisterAuthorizer(g.authorize)
}

// removeGuard takes every policy off a driver connection.
func removeGuard(dc any) error {
	reg, ok := dc.(sqlite.AuthorizerRegisterer)
	if !ok {
		return errors.New("the driver connection has no authorizer")
	}
	return reg.RegisterAuthorizer(nil)
}

// applyGuarded runs one migration under the guard and records it, all in one
// transaction on a connection of its own.
//
// Nothing between Conn and the deferred Close may use the write pool: its one
// connection is the one held here.
func (s *Store) applyGuarded(ctx context.Context, id string, m Migration, sum string) (err error) {
	conn, err := s.DB.Write.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// BEGIN IMMEDIATE runs before any policy exists.
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	g := newMigrationGuard(id)
	committed := false

	// The teardown runs on every way out, a panic included. It ends the
	// transaction and takes the policy off the connection. When it cannot take
	// it off it answers driver.ErrBadConn, which makes database/sql close the
	// connection instead of handing it to the next writer; the transaction has
	// ended by then, so the close does not wait on itself.
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
		remove := s.removeGuard
		if remove == nil {
			remove = removeGuard
		}
		rerr := conn.Raw(func(dc any) error {
			if err := remove(dc); err != nil {
				return driver.ErrBadConn
			}
			return nil
		})
		if rerr != nil && !errors.Is(rerr, sql.ErrConnDone) && !errors.Is(rerr, driver.ErrBadConn) && err == nil {
			err = rerr
		}
	}()

	if err := conn.Raw(g.install); err != nil {
		return fmt.Errorf("migration %q of plugin %q refused: it cannot be checked: %w", m.Name, id, err)
	}

	g.armed.Store(true)
	_, execErr := tx.ExecContext(ctx, m.SQL)
	g.armed.Store(false)

	if why := g.refusal(); why != "" {
		return fmt.Errorf("migration %q of plugin %q refused: %s", m.Name, id, why)
	}
	if execErr != nil {
		return fmt.Errorf("migration %q of plugin %q failed: %w", m.Name, id, execErr)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO plugin_migrations (plugin_id, name, sha256, applied_at) VALUES ($1, $2, $3, $4)`,
		id, m.Name, sum, time.Now().UTC().Format(timeLayout)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
