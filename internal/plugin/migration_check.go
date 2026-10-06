package plugin

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// A plugin's migration is SQL from an uploaded archive, run with the full
// rights of the write connection. The driver exposes no authorizer, so the
// check is made on the text, and it is deliberately narrow: whatever it cannot
// classify with certainty it refuses.
//
// What a migration may do:
//
//   - create, alter and drop tables and indexes whose names start with the
//     plugin's own prefix, plugin_<id>_ (hyphens in the id become underscores);
//   - insert into, update and delete from those tables;
//   - write to plugin_store, but only in a statement that names the plugin's own
//     id as a literal — the shipped contact form copies its old messages there;
//   - read from the tables in migrationReadable besides its own, which is what
//     such a copy needs.
//
// Everything else is refused: ATTACH, PRAGMA, VACUUM, transactions, triggers and
// views (their bodies cannot be checked here), quoted identifiers (they would
// let a table name hide from the scan), temp tables, load_extension and any
// mention of another table of this program.
var migrationReadable = map[string]bool{
	"pages": true, "websites": true, "domains": true, "form_messages": true,
	"plugin_store": true,
}

var reMigrationWord = regexp.MustCompile(`[a-z_][a-z0-9_$]*`)

// migrationPrefix is the table-name prefix a plugin owns.
func migrationPrefix(id string) string {
	return "plugin_" + strings.ReplaceAll(id, "-", "_") + "_"
}

// splitMigration removes comments, masks string literals and splits the text
// into statements. It returns, per statement, the masked lower-case text and the
// literals found in it.
func splitMigration(src string) (stmts []string, lits [][]string, err error) {
	var cur strings.Builder
	var curLits []string
	flush := func() {
		if strings.TrimSpace(cur.String()) != "" {
			stmts = append(stmts, strings.ToLower(cur.String()))
			lits = append(lits, curLits)
		}
		cur.Reset()
		curLits = nil
	}
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '-' && i+1 < len(src) && src[i+1] == '-':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			cur.WriteByte(' ')
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, nil, fmt.Errorf("unterminated comment")
			}
			i += end + 4
			cur.WriteByte(' ')
		case c == '\'':
			var lit strings.Builder
			i++
			closed := false
			for i < len(src) {
				if src[i] == '\'' {
					if i+1 < len(src) && src[i+1] == '\'' {
						lit.WriteByte('\'')
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				lit.WriteByte(src[i])
				i++
			}
			if !closed {
				return nil, nil, fmt.Errorf("unterminated string")
			}
			curLits = append(curLits, lit.String())
			cur.WriteString(" '' ")
		case c == '"' || c == '`' || c == '[':
			return nil, nil, fmt.Errorf("quoted identifiers are not allowed")
		case c == ';':
			flush()
			i++
		default:
			cur.WriteByte(c)
			i++
		}
	}
	flush()
	return stmts, lits, nil
}

// checkMigration reports why a migration may not run, or nil.
func (s *Store) checkMigration(ctx context.Context, id, sqlText string) error {
	stmts, lits, err := splitMigration(sqlText)
	if err != nil {
		return err
	}
	if len(stmts) == 0 {
		return fmt.Errorf("no statement")
	}

	// Every table and index that exists, so a mention of one can be recognised.
	known := map[string]bool{}
	rows, err := s.DB.Read.QueryContext(ctx, `SELECT name FROM sqlite_master`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return err
		}
		known[strings.ToLower(n)] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	prefix := migrationPrefix(id)
	own := func(n string) bool { return strings.HasPrefix(n, prefix) && len(n) > len(prefix) }

	for k, st := range stmts {
		w := reMigrationWord.FindAllString(st, -1)
		if len(w) == 0 {
			return fmt.Errorf("unrecognised statement")
		}
		// Words that are never acceptable wherever they stand.
		for _, x := range w {
			switch {
			case x == "load_extension", x == "attach", x == "detach", x == "pragma",
				x == "vacuum", x == "reindex", x == "trigger", x == "view",
				x == "temp", x == "temporary", x == "with", x == "begin",
				x == "commit", x == "rollback", x == "savepoint", x == "release",
				x == "sqlite_master", x == "sqlite_schema", strings.HasPrefix(x, "sqlite_"):
				return fmt.Errorf("%q is not allowed", x)
			}
			if own(x) || x == "plugin_store" {
				continue
			}
			if known[x] && !migrationReadable[x] {
				return fmt.Errorf("table %q is not the plugin's own", x)
			}
		}

		// The statement's target, by its first words.
		skip := func(i int, words ...string) int {
			for i < len(w) && contains(words, w[i]) {
				i++
			}
			return i
		}
		var target string
		var idx int
		switch w[0] {
		case "create":
			i := skip(1, "unique")
			if i >= len(w) || (w[i] != "table" && w[i] != "index") {
				return fmt.Errorf("only CREATE TABLE and CREATE INDEX are allowed")
			}
			i = skip(i+1, "if", "not", "exists")
			if i >= len(w) {
				return fmt.Errorf("unrecognised statement")
			}
			target, idx = w[i], i
			if !own(target) {
				return fmt.Errorf("%q is not the plugin's own", target)
			}
			if w[1] == "unique" || w[skip(1, "unique")] == "index" {
				// the table an index lands on is the word after ON
				ok := false
				for j := idx + 1; j < len(w)-1; j++ {
					if w[j] == "on" {
						if !own(w[j+1]) {
							return fmt.Errorf("%q is not the plugin's own", w[j+1])
						}
						ok = true
						break
					}
				}
				if !ok {
					return fmt.Errorf("unrecognised statement")
				}
			}
		case "alter":
			if len(w) < 3 || w[1] != "table" || !own(w[2]) {
				return fmt.Errorf("ALTER is only allowed on the plugin's own tables")
			}
		case "drop":
			i := skip(1)
			if i >= len(w) || (w[i] != "table" && w[i] != "index") {
				return fmt.Errorf("only DROP TABLE and DROP INDEX are allowed")
			}
			i = skip(i+1, "if", "exists")
			if i >= len(w) || !own(w[i]) {
				return fmt.Errorf("DROP is only allowed on the plugin's own objects")
			}
		case "insert", "replace":
			i := skip(1, "or", "replace", "ignore", "abort", "fail", "rollback")
			if i >= len(w) || w[i] != "into" || i+1 >= len(w) {
				return fmt.Errorf("unrecognised statement")
			}
			target = w[i+1]
		case "update":
			i := skip(1, "or", "replace", "ignore", "abort", "fail", "rollback")
			if i >= len(w) {
				return fmt.Errorf("unrecognised statement")
			}
			target = w[i]
		case "delete":
			if len(w) < 3 || w[1] != "from" {
				return fmt.Errorf("unrecognised statement")
			}
			target = w[2]
		default:
			return fmt.Errorf("%q statements are not allowed", w[0])
		}

		if w[0] == "insert" || w[0] == "replace" || w[0] == "update" || w[0] == "delete" {
			switch {
			case own(target):
			case target == "plugin_store":
				named := false
				for _, l := range lits[k] {
					if l == id {
						named = true
					}
				}
				if !named {
					return fmt.Errorf("a write to plugin_store has to name the plugin's own id")
				}
			default:
				return fmt.Errorf("%q is not the plugin's own", target)
			}
		}
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
