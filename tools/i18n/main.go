// Command i18n keeps the message catalogues in step with the source.
//
//	go run ./tools/i18n            # report what is missing
//	go run ./tools/i18n -write     # add the missing keys with empty values
//	go run ./tools/i18n -schweiz   # rebuild de-CH.json from the German
//
// It collects every German string that reaches a person: the {{t}}, {{th}} and
// {{tf}} calls in the admin templates, and the flash messages, form errors and
// page titles in the Go code. Those are the keys — this project translates
// German sentences, not invented identifiers, so the catalogue is a plain
// German-to-other-language dictionary anybody can read.
//
// The three regional catalogues are deviation lists rather than translations,
// and the tool treats them in two different ways. de-CH.json is rebuilt from
// the German source by -schweiz, because a mechanical rule exists for it: the
// sharp s becomes a double s and the German quotation marks become guillemets.
// That rule is swissSpelling, below. fr-CH.json and it-CH.json are maintained
// by hand and are only ever read — their entries are vocabulary choices (natel
// for portable, and so on) that no string replacer derives, and inventing a
// rule for one of the two would put back the asymmetry it was meant to remove.
//
// A key that disappears from the source is reported but never deleted: a
// sentence often comes back one commit later, and a translation thrown away is
// a translation somebody has to do again.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// callsInTemplates matches {{t "…"}}, {{th "…"}} and {{tf "…" …}}.
//
// The string is a Go literal inside the template, so it is read back with
// strconv.Unquote — the same rules the template parser applies.
var callsInTemplates = regexp.MustCompile(`\{\{-?\s*(?:t|th|tf)\s+("(?:[^"\\]|\\.)*")`)

// goFuncs are the Go functions whose first string argument a person reads.
//
// SetFlash* translate what they are given, Errors.Add is rendered through {{t}}
// in the template, and NewLayoutData translates the title it is handed.
var goFuncs = map[string]int{
	"SetFlashError":   2,
	"SetFlashSuccess": 2,
	"SetFlashWarning": 2,
	"Add":             1, // web.FormErrors.Add(field, message)
	"NewLayoutData":   2,
	"Titlef":          1,
	"T":               1,
	// N marks a label that is built at start-up and translated where it is
	// rendered — see i18n.N.
	"N": 0,
}

func main() {
	write := flag.Bool("write", false, "add missing keys to the catalogues")
	swiss := flag.Bool("schweiz", false, "rebuild de-CH.json from the German source")
	root := flag.String("root", ".", "repository root")
	flag.Parse()

	keys := map[string]bool{}
	if err := collectTemplates(filepath.Join(*root, "cmd/holzcloud/templates/admin"), keys); err != nil {
		fail(err)
	}
	if err := collectGo(filepath.Join(*root, "internal"), keys); err != nil {
		fail(err)
	}

	dir := filepath.Join(*root, "internal/i18n/locales")
	entries, err := os.ReadDir(dir)
	if err != nil {
		fail(err)
	}

	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	fmt.Printf("%d Zeichenketten im Quelltext\n", len(sorted))

	if *swiss {
		if err := writeSwiss(filepath.Join(dir, "de-CH.json"), sorted); err != nil {
			fail(err)
		}
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		catalog, err := readCatalog(path)
		if err != nil {
			fail(err)
		}

		// A regional fassung — de-CH beside de — is a list of deviations, not a
		// translation. Filling its gaps would be exactly wrong: an empty key
		// there means "the base language already says it right", and writing
		// nine hundred of them in would turn a readable file of thirty
		// corrections into a file nobody maintains. It is only checked.
		if strings.Contains(strings.TrimSuffix(e.Name(), ".json"), "-") {
			var wrong int
			for k := range catalog {
				if !keys[k] {
					wrong++
					fmt.Printf("  %s: kein Satz im Quelltext: %q\n", e.Name(), k)
				}
			}
			// Which of the three the tool writes is invisible from the outside,
			// and the difference matters to anybody about to edit one. So the
			// line that crosses the screen on every run says it.
			pflege := "nur gelesen, von Hand gepflegt"
			if e.Name() == "de-CH.json" {
				pflege = "wird von -schweiz erzeugt"
			}
			fmt.Printf("%-12s %d Abweichungen, %d ohne Gegenstück — %s\n", e.Name(), len(catalog), wrong, pflege)
			continue
		}

		var missing, done int
		var stale []string
		for _, k := range sorted {
			value, ok := catalog[k]
			switch {
			case !ok:
				missing++
				catalog[k] = ""
			case strings.TrimSpace(value) == "":
				missing++
			default:
				done++
			}
		}
		for k := range catalog {
			if !keys[k] {
				stale = append(stale, k)
			}
		}
		sort.Strings(stale)
		fmt.Printf("%-12s %d übersetzt, %d offen, %d verwaist\n", e.Name(), done, missing, len(stale))
		// Verwaiste beim Namen nennen. Gelöscht wird nichts — ein Satz kommt
		// oft einen Commit später zurück —, aber ein Bericht, der nur "1
		// verwaist" sagt, lässt jemanden die Datei von Hand durchsuchen.
		for i, k := range stale {
			if i == 5 {
				fmt.Printf("  … und %d weitere\n", len(stale)-5)
				break
			}
			fmt.Printf("  verwaist: %q\n", k)
		}

		if *write {
			if err := writeCatalog(path, catalog); err != nil {
				fail(err)
			}
		}
	}
}

func collectTemplates(dir string, keys map[string]bool) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range callsInTemplates.FindAllStringSubmatch(string(data), -1) {
			if s, err := strconv.Unquote(m[1]); err == nil && s != "" {
				keys[s] = true
			}
		}
		return nil
	})
}

func collectGo(dir string, keys map[string]bool) error {
	fset := token.NewFileSet()
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
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
			if s, err := strconv.Unquote(lit.Value); err == nil && s != "" {
				keys[s] = true
			}
			return true
		})
		return nil
	})
}

// swissSpelling is what the German of Switzerland does differently, as a rule
// rather than as a judgement: no ß anywhere, and guillemets pointing outwards
// where Germany sets low-high quotation marks.
//
// Mechanical on purpose. Those two rules cover every sentence in this
// administration, they cannot be got wrong, and they must never drift — a new
// German sentence with a ß in it should turn into a Swiss one by running this,
// not by somebody noticing.
var swissSpelling = strings.NewReplacer("ß", "ss", "„", "«", "“", "»")

// writeSwiss rebuilds de-CH.json.
//
// Entries the rule produces are rewritten; entries about something else — a
// word Switzerland simply uses differently — are kept as they are. So a
// translator can add one by hand without the next run throwing it away.
func writeSwiss(path string, sources []string) error {
	kept, err := readCatalog(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	out := map[string]string{}
	byRule := 0
	for _, german := range sources {
		if swiss := swissSpelling.Replace(german); swiss != german {
			out[german] = swiss
			byRule++
		}
	}
	byHand := 0
	for german, swiss := range kept {
		// A hand-written line wins, but the spelling rule is applied to it as
		// well: somebody adding a Swiss word should not have to remember the ß
		// too, and the two kinds of change can land in the same sentence.
		swiss = swissSpelling.Replace(swiss)
		if out[german] == swiss {
			continue
		}
		out[german] = swiss
		byHand++
	}
	fmt.Printf("de-CH.json  %d nach Regel, %d von Hand\n", byRule, byHand)
	return writeCatalog(path, out)
}

func readCatalog(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	catalog := map[string]string{}
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return catalog, nil
}

// writeCatalog writes the file sorted and flush left, one key per line, so a
// change to one string is one line in a diff rather than a reshuffled file.
//
// The encoder is asked not to escape HTML. The standard setting turns < > &
// into their \u escapes, which is right for JSON embedded in a page and wrong
// here: these files are dictionaries a translator reads, half the sentences
// contain a <code> or an &amp;, and an escaped <code> is not something anybody
// should have to decipher. The escaping is not needed either — nothing serves
// these files to a browser.
//
// Indenting with an empty prefix and an empty indent string is what produces
// the flush-left, one-entry-per-line shape the committed catalogues carry. Two
// near-misses read like simplifications and are neither: Encoder.SetIndent("",
// "") is a no-op that puts the whole catalogue on one line, and
// json.MarshalIndent escapes HTML with no switch to turn it off.
//
// The keys need no sorting here. encoding/json sorts map keys byte-wise, which
// is the order sort.Strings gives.
func writeCatalog(path string, catalog map[string]string) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(catalog); err != nil {
		return err
	}

	var out bytes.Buffer
	if err := json.Indent(&out, buf.Bytes(), "", ""); err != nil {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
