package plugin

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
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

// MigrationTimeout bounds one migration. The shipped ones take milliseconds;
// the bound exists so that a migration cannot hold the single write connection
// for ever, which would stop every write of the core.
const MigrationTimeout = 10 * time.Second

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

	// others are the table prefixes of the other installed plugins. Plugin ids
	// may contain hyphens, so the prefix of one plugin can be the start of
	// another's (nicht, nicht-gefunden); a name then belongs to the plugin with
	// the longest prefix it starts with.
	others []string

	// armed is true only while the plugin's own SQL runs. Before and after it
	// the connection runs the host's statements (COMMIT, reading the schema,
	// recording the migration), and those have to pass.
	armed atomic.Bool

	mu     sync.Mutex
	denial string
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

// record remembers the first thing that was refused. The first one is the
// cause; what follows in the same statement are consequences.
func (g *migrationGuard) record(what string) {
	g.mu.Lock()
	if g.denial == "" {
		g.denial = what
	}
	g.mu.Unlock()
}

// deny records a denial and answers it.
func (g *migrationGuard) deny(what string) sqlite.AuthorizerReturnCode {
	g.record(what)
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
	if !strings.HasPrefix(n, g.prefix) || len(n) == len(g.prefix) {
		return false
	}
	for _, o := range g.others {
		if len(o) > len(g.prefix) && strings.HasPrefix(n, o) {
			return false
		}
	}
	return true
}

// ownAutoIndex reports whether name is the index SQLite makes for a UNIQUE or
// PRIMARY KEY constraint of one of the plugin's own tables.
func (g *migrationGuard) ownAutoIndex(name, table string) bool {
	return g.owns(table) && strings.HasPrefix(asciiLower(name), "sqlite_autoindex_"+asciiLower(table)+"_")
}

// ownsRow reports whether a row of sqlite_schema is the plugin's own.
func (g *migrationGuard) ownsRow(name, table string) bool {
	return g.owns(table) && (g.owns(name) || g.ownAutoIndex(name, table))
}

// isSchemaTable names SQLite's own bookkeeping tables of the main database.
// DDL writes to them internally; a user statement that writes to them is
// refused by SQLite itself.
func isSchemaTable(name string) bool {
	switch asciiLower(name) {
	case "sqlite_master", "sqlite_schema":
		return true
	}
	return false
}

// isTempSchemaTable names the same for the temp database, which RENAME consults
// for triggers and views that could be affected. Nothing can be created there.
func isTempSchemaTable(name string) bool {
	switch asciiLower(name) {
	case "sqlite_temp_master", "sqlite_temp_schema":
		return true
	}
	return false
}

// authorize is the SQLite authorizer callback. It must not touch the
// connection. While the guard is not armed it allows everything.
//
// A panic in here would unwind through the driver's C frames; it is caught and
// answered as a denial instead, so a fault of the guard refuses the migration.
func (g *migrationGuard) authorize(action sqlite.AuthorizerActionCode, arg1, arg2, database, _ string) (rc sqlite.AuthorizerReturnCode) {
	if !g.armed.Load() {
		return sqlite.AuthorizerOK
	}
	defer func() {
		if p := recover(); p != nil {
			rc = g.deny(fmt.Sprintf("the guard failed: %v", p))
		}
	}()
	// The last argument, the trigger or view an access comes from, is not looked
	// at: SQLite names a common table expression there as well, and the rules
	// below hold wherever the access comes from. Triggers and views cannot be
	// created, and the host's own fire only on tables the plugin cannot write.
	//
	// Another database is never the plugin's to reach. RENAME looks at the temp
	// schema for triggers and views it might break; that and nothing else.
	switch database {
	case "", "main":
	case "temp":
		if !isTempSchemaTable(arg1) || (action != sqlite.AuthRead && action != sqlite.AuthUpdate) {
			return g.deny("reaching the temp database")
		}
		return sqlite.AuthorizerOK
	default:
		return g.deny("reaching database " + database)
	}

	switch action {
	case sqlite.AuthCreateTable:
		// Only a table in the plugin's own namespace.
		if g.owns(arg1) {
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthCreateIndex:
		// An index on an own table, named in the own namespace or one of the
		// automatic indexes of a UNIQUE or PRIMARY KEY constraint.
		if g.ownsRow(arg1, arg2) {
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthDropTable:
		if g.owns(arg1) {
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthDropIndex:
		if g.ownsRow(arg1, arg2) {
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthAlterTable:
		// arg2 is the table as it is named now. A rename is judged by that old
		// name only; the new one is looked at after the statement has run.
		if g.owns(arg2) {
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthReindex:
		// CREATE INDEX asks for it on the index it has just made.
		if g.owns(arg1) || strings.HasPrefix(asciiLower(arg1), "sqlite_autoindex_"+g.prefix) {
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthInsert:
		switch {
		case g.owns(arg1):
			return sqlite.AuthorizerOK
		case asciiLower(arg1) == "plugin_store":
			// The shipped contact form copies its old messages here. Whose rows
			// they are is decided by the pre-update hook.
			return sqlite.AuthorizerOK
		case isSchemaTable(arg1):
			// Every CREATE writes its own row. SQLite refuses a user statement
			// that does it directly.
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthUpdate:
		switch {
		case g.owns(arg1):
			return sqlite.AuthorizerOK
		case isSchemaTable(arg1):
			return sqlite.AuthorizerOK
		case asciiLower(arg1) == "sqlite_sequence":
			// A rename moves the counter of an AUTOINCREMENT table along. The
			// counters of other tables are compared before and after, which is
			// what keeps a statement that changes them from standing.
			return sqlite.AuthorizerOK
		}
	case sqlite.AuthDelete:
		// DROP TABLE deletes the table's rows and its entries in the schema and
		// in the sequence table. It is not told apart from a DELETE here: the
		// rows of another table were never readable, and the sequence counters
		// are compared before and after.
		switch {
		case g.owns(arg1), isSchemaTable(arg1):
			return sqlite.AuthorizerOK
		case asciiLower(arg1) == "sqlite_sequence":
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
		return sqlite.AuthorizerOK
	case sqlite.AuthFunction:
		if !deniedFunctions[asciiLower(arg2)] {
			return sqlite.AuthorizerOK
		}
	}
	return g.deny(describeAction(action, arg1, arg2))
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
		// whenever a row is inserted there. The read cannot be told from a
		// SELECT of the migration's own, so any migration can list the ids of
		// the installed plugins. That is all it learns; it cannot pin them with a
		// foreign key of its own, see checkForeignKeys. Nothing else of plugins.
		return c == "id"
	case t == "pragma_quick_check":
		// ALTER TABLE ADD COLUMN on a STRICT table or one with rows checks the
		// existing rows through it. It answers only with integrity messages.
		return c == "quick_check"
	case isSchemaTable(t), t == "sqlite_sequence":
		// Open, and known to be. ALTER TABLE, DROP TABLE and CREATE TABLE read
		// SQLite's schema table and its counter table through the authorizer,
		// the text of every table definition included, and a read that SQLite
		// makes for itself cannot be told from one the plugin writes: it is the
		// same table, the same column and the same call. Rules that guess where
		// a statement begins fail, because the reads of a subquery come before
		// the callback that announces it. What this shows is the definition of
		// the host's tables, which is in its source, and how many rows were ever
		// made in an AUTOINCREMENT table. Changing either is not open: the
		// check after the statement compares both with what they were.
		return true
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

// preUpdate is the pre-update hook. It sees every row SQLite writes to
// plugin_store, with the values, which the authorizer cannot: it is asked about
// the table, not about the row. Whatever the statement looks like, a row that is
// not the plugin's own is recorded and the migration is refused.
//
// A panic in here is caught and recorded as a refusal, as in authorize.
func (g *migrationGuard) preUpdate(d sqlite.SQLitePreUpdateData) {
	if !g.armed.Load() || asciiLower(d.TableName) != "plugin_store" {
		return
	}
	defer func() {
		if p := recover(); p != nil {
			g.record(fmt.Sprintf("the guard failed: %v", p))
		}
	}()
	check := func(read func(dest ...any) error) {
		dest := make([]any, d.Count())
		if err := read(dest...); err != nil || len(dest) == 0 {
			g.record("a plugin_store row that cannot be read")
			return
		}
		if owner, ok := dest[0].(string); !ok || owner != g.id {
			g.record(fmt.Sprintf("writing a plugin_store row of plugin %s", quoteForMessage(dest[0])))
		}
	}
	switch d.Op {
	case sqlite3.SQLITE_INSERT:
		check(d.New)
	case sqlite3.SQLITE_DELETE:
		check(d.Old)
	default:
		check(d.Old)
		check(d.New)
	}
}

// quoteForMessage shows a value from a hostile migration without letting it
// run on.
func quoteForMessage(v any) string {
	s := fmt.Sprint(v)
	if len(s) > 60 {
		s = s[:60] + "…"
	}
	return fmt.Sprintf("%q", s)
}

// install puts the guard on a driver connection. A connection that cannot take
// it is a refusal: the migration must not run unguarded.
func (g *migrationGuard) install(dc any) error {
	reg, ok := dc.(sqlite.AuthorizerRegisterer)
	if !ok {
		return errors.New("the driver connection has no authorizer")
	}
	hooks, ok := dc.(sqlite.HookRegisterer)
	if !ok {
		return errors.New("the driver connection has no hooks")
	}
	if err := reg.RegisterAuthorizer(g.authorize); err != nil {
		return err
	}
	hooks.RegisterPreUpdateHook(g.preUpdate)
	return nil
}

// removeGuard takes every policy off a driver connection.
func removeGuard(dc any) error {
	reg, ok := dc.(sqlite.AuthorizerRegisterer)
	if !ok {
		return errors.New("the driver connection has no authorizer")
	}
	if hooks, ok := dc.(sqlite.HookRegisterer); ok {
		hooks.RegisterPreUpdateHook(nil)
	}
	return reg.RegisterAuthorizer(nil)
}

// schemaRow is one entry of sqlite_schema.
type schemaRow struct{ kind, name, table string }

// readSchema lists the schema through the transaction: only the connection that
// holds an uncommitted change can see it.
func readSchema(ctx context.Context, tx *sql.Tx) (map[schemaRow]bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT type, name, tbl_name FROM sqlite_schema`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[schemaRow]bool{}
	for rows.Next() {
		var r schemaRow
		if err := rows.Scan(&r.kind, &r.name, &r.table); err != nil {
			return nil, err
		}
		out[r] = true
	}
	return out, rows.Err()
}

// counter is one row of sqlite_sequence.
type counter struct {
	name string
	seq  int64
}

// readSequences lists the AUTOINCREMENT counters by rowid. Not by name: nothing
// keeps two rows from carrying the same name, and SQLite reads the first one it
// finds, so a comparison by name can be shown one row while SQLite uses another.
func readSequences(ctx context.Context, tx *sql.Tx) (map[int64]counter, error) {
	rows, err := tx.QueryContext(ctx, `SELECT rowid, name, seq FROM sqlite_sequence`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]counter{}
	for rows.Next() {
		var rowid int64
		var c counter
		if err := rows.Scan(&rowid, &c.name, &c.seq); err != nil {
			return nil, err
		}
		out[rowid] = c
	}
	return out, rows.Err()
}

// readOtherPrefixes lists the table prefixes of the other installed plugins.
func readOtherPrefixes(ctx context.Context, tx *sql.Tx, self string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM plugins`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id != self {
			out = append(out, migrationPrefix(id))
		}
	}
	return out, rows.Err()
}

// checkAfter compares the database with what it was before the migration ran.
//
// The authorizer judges a rename by the old name of the table only, and the
// counters of AUTOINCREMENT tables are touched by SQLite itself when a table is
// renamed or dropped, so neither can be decided while the statement runs. What
// the migration left behind can: every new schema entry has to lie in the
// plugin's own namespace, no entry of anyone else may have gone, and the
// counters of other tables have to be what they were.
func (g *migrationGuard) checkAfter(before, after map[schemaRow]bool, seqBefore, seqAfter map[int64]counter) string {
	var added, removed []schemaRow
	for r := range after {
		if !before[r] {
			added = append(added, r)
		}
	}
	for r := range before {
		if !after[r] {
			removed = append(removed, r)
		}
	}
	// Tables before indexes, then by name, so the message does not depend on the
	// order of a map.
	byKind := func(a, b schemaRow) int {
		if (a.kind == "table") != (b.kind == "table") {
			if a.kind == "table" {
				return -1
			}
			return 1
		}
		return strings.Compare(a.name, b.name)
	}
	slices.SortFunc(added, byKind)
	slices.SortFunc(removed, byKind)
	for _, r := range added {
		if !g.ownsRow(r.name, r.table) {
			return fmt.Sprintf("%s lies outside the plugin's prefix", r.name)
		}
	}
	for _, r := range removed {
		if !g.ownsRow(r.name, r.table) {
			return fmt.Sprintf("%s was removed, which is not the plugin's", r.name)
		}
	}
	// A counter row may change only where it is the plugin's, before and after:
	// a row of a core table that is renamed, reset or deleted, a row of the
	// plugin's that takes a core table's name, and a new row with such a name
	// are all refused. Rowids, so a second row with a core table's name cannot
	// stand in for the first.
	rowids := make([]int64, 0, len(seqBefore)+len(seqAfter))
	for rowid := range seqBefore {
		rowids = append(rowids, rowid)
	}
	for rowid := range seqAfter {
		if _, ok := seqBefore[rowid]; !ok {
			rowids = append(rowids, rowid)
		}
	}
	slices.Sort(rowids)
	for _, rowid := range rowids {
		was, had := seqBefore[rowid]
		is, has := seqAfter[rowid]
		if had && has && was == is {
			continue
		}
		switch {
		case had && !g.owns(was.name):
			return fmt.Sprintf("the counter of %s was changed, which is not the plugin's", was.name)
		case has && !had && !g.owns(is.name):
			return fmt.Sprintf("a counter for %s was added, which is not the plugin's", is.name)
		case has && !g.owns(is.name):
			return fmt.Sprintf("a counter of the plugin was moved to %s, which is not the plugin's", is.name)
		}
	}
	return ""
}

// checkForeignKeys looks at the foreign keys of every table the plugin owns,
// the ones it had before included, because ADD COLUMN can give an old table a
// new key. Every parent has to be the plugin's own.
//
// SQLite never asks the authorizer about a REFERENCES clause, and the check it
// runs when a child row is written reads only the parent's key column, which
// for plugins, pages and form_messages a migration may read. A child row that
// points at one of those makes the parent undeletable while it exists: a
// plugin that nobody can remove, a page, a website or a contact message that
// cannot be deleted. So a key out of the namespace is refused, whatever rows
// the migration wrote.
//
// Runs disarmed, through the transaction.
func (g *migrationGuard) checkForeignKeys(ctx context.Context, tx *sql.Tx, schema map[schemaRow]bool) (string, error) {
	var tables []string
	for r := range schema {
		if r.kind == "table" && g.owns(r.name) {
			tables = append(tables, r.name)
		}
	}
	slices.Sort(tables)
	for _, table := range tables {
		parents, err := foreignKeyParents(ctx, tx, table)
		if err != nil {
			return "", err
		}
		for _, parent := range parents {
			if !g.owns(parent) {
				return fmt.Sprintf("%s has a foreign key to %s, which is not the plugin's", table, parent), nil
			}
		}
	}
	return "", nil
}

// foreignKeyParents lists the tables one table's foreign keys point at.
func foreignKeyParents(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT "table" FROM pragma_foreign_key_list($1)`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var parent string
		if err := rows.Scan(&parent); err != nil {
			return nil, err
		}
		out = append(out, parent)
	}
	return out, rows.Err()
}

// noTransaction reports a ROLLBACK that found no transaction: SQLite rolls a
// transaction back by itself when a write inside it is interrupted, as the
// bound does, and the connection is then clean.
func noTransaction(err error) bool {
	return strings.Contains(err.Error(), "no transaction is active")
}

// applyGuarded runs one migration under the guard and records it, all in one
// transaction on a connection of its own.
//
// Nothing between Conn and the deferred Close may use the write pool: its one
// connection is the one held here.
//
// Waiting for the write connection honours the caller's context. Once the
// migration has the connection it is no longer the caller's: the transaction
// runs on a context that is not cancelled with the request, because a
// cancellation that ended it from outside would roll it back while the policy
// is still armed, and a ROLLBACK is a statement the policy would refuse. The
// plugin's SQL has a bound of its own instead.
func (s *Store) applyGuarded(ctx context.Context, id string, m Migration, sum string) (err error) {
	conn, err := s.DB.Write.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx = context.WithoutCancel(ctx)
	bound := s.migrationTimeout
	if bound <= 0 {
		bound = MigrationTimeout
	}

	// BEGIN IMMEDIATE runs before any policy exists.
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	g := newMigrationGuard(id)
	committed := false

	// The teardown runs on every way out, a panic included. It ends the
	// transaction and takes the policy off the connection.
	//
	// It disarms first. A panic inside the plugin's statement skips the disarm
	// after it, and ROLLBACK is a statement like any other: armed, the policy
	// refuses it, and the connection would go back to the pool inside the
	// plugin's open transaction.
	//
	// The connection is thrown away, by driver.ErrBadConn from Raw, which makes
	// database/sql close it instead of handing it to the next writer, when the
	// policy cannot be taken off, when the rollback failed (other than because
	// SQLite had already ended the transaction, as it does for an interrupted
	// write), and after a panic, which may have left the connection halfway
	// through a statement. The transaction has ended by then, so the close does
	// not wait on itself. A panic is raised again once all of that is done.
	defer func() {
		g.armed.Store(false)
		p := recover()
		discard := p != nil
		if !committed {
			if rerr := tx.Rollback(); rerr != nil && !errors.Is(rerr, sql.ErrTxDone) && !noTransaction(rerr) {
				discard = true
			}
		}
		remove := s.removeGuard
		if remove == nil {
			remove = removeGuard
		}
		rerr := conn.Raw(func(dc any) error {
			if err := remove(dc); err != nil || discard {
				return driver.ErrBadConn
			}
			return nil
		})
		if rerr != nil && !errors.Is(rerr, sql.ErrConnDone) && !errors.Is(rerr, driver.ErrBadConn) && err == nil {
			err = rerr
		}
		if p != nil {
			panic(p)
		}
	}()

	if err := conn.Raw(g.install); err != nil {
		return fmt.Errorf("migration %q of plugin %q refused: it cannot be checked: %w", m.Name, id, err)
	}

	// Disarmed, the host looks at what the migration will start from. Through
	// the transaction, never the read pool: this connection holds the lock.
	failed := func(err error) error {
		return fmt.Errorf("migration %q of plugin %q failed: %w", m.Name, id, err)
	}
	others, err := readOtherPrefixes(ctx, tx, id)
	if err != nil {
		return failed(err)
	}
	g.others = others
	schemaBefore, err := readSchema(ctx, tx)
	if err != nil {
		return failed(err)
	}
	seqBefore, err := readSequences(ctx, tx)
	if err != nil {
		return failed(err)
	}

	// The driver interrupts the statement when the bound passes; the transaction
	// stays open and is rolled back below, with the policy disarmed.
	execCtx, cancel := context.WithTimeout(ctx, bound)
	defer cancel()
	g.armed.Store(true)
	_, execErr := tx.ExecContext(execCtx, m.SQL)
	g.armed.Store(false)

	if why := g.refusal(); why != "" {
		return fmt.Errorf("migration %q of plugin %q refused: %s", m.Name, id, why)
	}
	if execErr != nil {
		return failed(execErr)
	}

	schemaAfter, err := readSchema(ctx, tx)
	if err != nil {
		return failed(err)
	}
	seqAfter, err := readSequences(ctx, tx)
	if err != nil {
		return failed(err)
	}
	if why := g.checkAfter(schemaBefore, schemaAfter, seqBefore, seqAfter); why != "" {
		return fmt.Errorf("migration %q of plugin %q refused: %s", m.Name, id, why)
	}
	why, err := g.checkForeignKeys(ctx, tx, schemaAfter)
	if err != nil {
		return failed(err)
	}
	if why != "" {
		return fmt.Errorf("migration %q of plugin %q refused: %s", m.Name, id, why)
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
