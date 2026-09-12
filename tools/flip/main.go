// Command flip turns the catalogue's source language from German to English.
//
//	go run ./tools/flip            # report what would change
//	go run ./tools/flip -write     # do it
//
// It is kept rather than thrown away because the roadmap asked for it to be
// (`.planning/ROADMAP.md`, Phase 12): a second pass will want it, and a
// transformation nobody can re-run is a transformation nobody can check.
//
// # What it does
//
// The catalogue is keyed on the German sentence — the German IS the key, there
// are no invented identifiers, and until v2.0 there was no de.json because
// German needed no translation into itself. That is a good design and it has
// one consequence: the source of this program is full of German sentences, and
// "every identifier and every literal is English" cannot be true while it
// stands.
//
// Turning it round is a bijection through en.json. Every German key has exactly
// one English value — measured, and the ten places where two German keys fell
// on one English value were resolved one by one before this ran, because a
// bijection with a collision in it silently merges two sentences into one.
//
// So: replace each German literal in the source with its English, then rewrite
// every catalogue with the English as the key. es.json, fr.json and it.json are
// re-keyed through the same mapping; de.json is the inverse of en.json and is
// new, because German is a translation now.
//
// # Why this cannot be done with sed
//
// The same German sentence is a catalogue key in one line and a stored value,
// a form field name or a test fixture in the next. So the Go half walks the AST
// and rewrites the string literal at the ARGUMENT INDEX the collector reads —
// the same index, from the same table, so the two can never drift — and the
// template half matches the action and not the sentence.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// goFuncs is a copy of tools/i18n's table, and it has to stay a copy of it.
//
// If the collector learns a function this does not know, a sentence is
// collected and never flipped: it stays German in the source, becomes an
// orphan in every catalogue, and the gate reports it. That is the good failure
// mode — loud — and it is why this is duplicated rather than guessed at.
var goFuncs = map[string]int{
	"SetFlashError": 2, "SetFlashSuccess": 2, "SetFlashWarning": 2,
	"Add": 1, "NewLayoutData": 2, "Titlef": 1, "T": 1, "N": 0, "Tf": 1,
}

var callsInTemplates = regexp.MustCompile(`(\{\{-?\s*(?:t|th|tf|thf)\s+)("(?:[^"\\]|\\.)*")`)

func main() {
	write := flag.Bool("write", false, "rewrite the files instead of reporting")
	dir := flag.String("locales", "internal/i18n/locales", "catalogue directory")
	flag.Parse()

	en, err := readCatalog(filepath.Join(*dir, "en.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if bad := collisions(en); len(bad) > 0 {
		fmt.Fprintln(os.Stderr, "en.json is not a bijection; resolve these first:")
		for _, line := range bad {
			fmt.Fprintln(os.Stderr, "  "+line)
		}
		os.Exit(1)
	}

	goHits := flipGo(en, *write)
	tmplHits := flipTemplates(en, *write)
	fmt.Printf("%d literals in Go, %d in templates\n", goHits, tmplHits)

	if !*write {
		fmt.Println("(dry run; pass -write)")
		return
	}
	if err := rewriteCatalogues(*dir, en); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// collisions reports English values that more than one German key falls on.
//
// The flip cannot proceed through one: two German sentences would become one
// English key, de.json could only carry one of them, and the other German
// sentence would be silently replaced by its neighbour on every German screen.
func collisions(en map[string]string) []string {
	byValue := map[string][]string{}
	for k, v := range en {
		byValue[v] = append(byValue[v], k)
	}
	var out []string
	for v, keys := range byValue {
		if len(keys) > 1 {
			sort.Strings(keys)
			out = append(out, fmt.Sprintf("%q <- %v", v, keys))
		}
	}
	sort.Strings(out)
	return out
}

// flipGo rewrites the literal at the collector's argument index.
func flipGo(en map[string]string, write bool) int {
	total := 0
	for _, root := range []string{"internal", "cmd", "tools", "sdk", "plugins"} {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.Contains(path, "/locales/") {
				return nil
			}
			n, err := flipGoFile(path, en, write)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
				os.Exit(1)
			}
			if n > 0 {
				total += n
				fmt.Printf("%5d %s\n", n, path)
			}
			return nil
		})
	}
	return total
}

func flipGoFile(path string, en map[string]string, write bool) (int, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return 0, err
	}
	type edit struct {
		at, width int
		with      string
	}
	var edits []edit
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := ""
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			name = fun.Sel.Name
		case *ast.Ident:
			name = fun.Name
		}
		at, ok := goFuncs[name]
		if !ok || len(call.Args) <= at {
			return true
		}
		lit, ok := call.Args[at].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		german, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		english, ok := en[german]
		if !ok || english == "" {
			return true
		}
		edits = append(edits, edit{
			at:    fset.Position(lit.Pos()).Offset,
			width: len(lit.Value),
			with:  quoteLike(lit.Value, english),
		})
		return true
	})
	if len(edits) == 0 || !write {
		return len(edits), nil
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].at < edits[j].at })
	out := make([]byte, 0, len(src))
	last := 0
	for _, e := range edits {
		out = append(out, src[last:e.at]...)
		out = append(out, e.with...)
		last = e.at + e.width
	}
	out = append(out, src[last:]...)
	return len(edits), os.WriteFile(path, out, 0o644)
}

// quoteLike keeps a raw string raw and an interpreted string interpreted.
//
// A `backquoted` sentence is used where the German carries quotation marks of
// its own; re-quoting it with strconv.Quote would escape them and change what
// reaches the screen.
func quoteLike(original, s string) string {
	if strings.HasPrefix(original, "`") && !strings.Contains(s, "`") {
		return "`" + s + "`"
	}
	return strconv.Quote(s)
}

func flipTemplates(en map[string]string, write bool) int {
	total := 0
	_ = filepath.WalkDir("cmd/holzcloud/templates/admin", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		n := 0
		out := callsInTemplates.ReplaceAllFunc(src, func(m []byte) []byte {
			parts := callsInTemplates.FindSubmatch(m)
			german, err := strconv.Unquote(string(parts[2]))
			if err != nil {
				return m
			}
			english, ok := en[german]
			if !ok || english == "" {
				return m
			}
			n++
			return append(append([]byte{}, parts[1]...), []byte(strconv.Quote(english))...)
		})
		if n > 0 {
			total += n
			fmt.Printf("%5d %s\n", n, path)
			if write {
				_ = os.WriteFile(path, out, 0o644)
			}
		}
		return nil
	})
	return total
}

// rewriteCatalogues re-keys every catalogue on the English sentence.
//
//   - de.json is NEW and is the inverse of en.json: German is a translation now.
//   - es, fr, it are re-keyed through the German→English mapping.
//   - the three regional lists (de-CH, fr-CH, it-CH) are re-keyed the same way.
//   - en.json is removed. English needs no catalogue: a lookup that finds
//     nothing returns the text unchanged, and the text is already English.
func rewriteCatalogues(dir string, en map[string]string) error {
	if err := writeCatalog(filepath.Join(dir, "de.json"), invert(en)); err != nil {
		return err
	}
	for _, name := range []string{"es.json", "fr.json", "it.json", "de-CH.json", "fr-CH.json", "it-CH.json"} {
		path := filepath.Join(dir, name)
		cat, err := readCatalog(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		out := map[string]string{}
		var lost []string
		for german, translated := range cat {
			english, ok := en[german]
			if !ok {
				lost = append(lost, german)
				continue
			}
			out[english] = translated
		}
		if len(lost) > 0 {
			sort.Strings(lost)
			fmt.Printf("%s: %d keys with no English counterpart, dropped: %v\n", name, len(lost), lost)
		}
		if err := writeCatalog(path, out); err != nil {
			return err
		}
		fmt.Printf("%s re-keyed: %d entries\n", name, len(out))
	}
	if err := os.Remove(filepath.Join(dir, "en.json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Println("en.json removed — English is the source language now")
	return nil
}

func invert(en map[string]string) map[string]string {
	out := make(map[string]string, len(en))
	for german, english := range en {
		out[english] = german
	}
	return out
}

// encode is json.Marshal with HTML escaping off.
//
// The default would turn <code> into \u003ccode\u003e in every entry that
// carries markup, and the catalogues carry a lot of it. It is valid JSON and it
// is unreadable, which for a file whose whole purpose is to be read by a
// translator is the wrong trade.
func encode(s string) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return bytes.TrimRight(buf.Bytes(), "\n")
}

func readCatalog(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cat := map[string]string{}
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cat, nil
}

// writeCatalog writes the flush-left one-line-per-entry shape tools/i18n uses.
func writeCatalog(path string, cat map[string]string) error {
	keys := make([]string, 0, len(cat))
	for k := range cat {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("{\n")
	for i, k := range keys {
		b.Write(encode(k))
		b.WriteString(": ")
		b.Write(encode(cat[k]))
		if i < len(keys)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString("}\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
