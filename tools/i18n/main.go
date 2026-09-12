// Command i18n keeps the message catalogues in step with the source.
//
//	go run ./tools/i18n            # report what is missing
//	go run ./tools/i18n -write     # add the missing keys with empty values
//	go run ./tools/i18n -schweiz   # rebuild de-CH.json from de.json
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
//
// # What this tool does NOT see
//
// Written down because "0 open, 0 orphaned" is a release gate, and a gate has
// to be honest about its edge. Three directories are read and no others:
//
//	cmd/holzcloud/templates/admin   .html only
//	internal                        .go, minus _test.go
//	internal/i18n/locales           the catalogues themselves
//
// So a German sentence in any of these is invisible here, and the report says
// nothing about it — not "open", not "orphaned", nothing, because the tool
// does not know it exists:
//
//   - cmd/holzcloud/templates/public — the shipped themes. They carry no
//     translation call at all and cannot: the public FuncMap
//     (internal/template/loader.go) has no t, th or tf. That is a decision
//     about the theme contract, recorded in
//     .planning/audits/v1.6-I18N-REICHWEITE.md, and not an oversight here.
//   - cmd/holzcloud/*.go — main.go, cli.go, cli_template.go. English by design
//     today. Anything a browser or an operator reads that gets added there
//     needs a home under internal/ or a fourth root in this file.
//   - plugins/ and sdk/ — the WASM modules build their own admin screens and
//     the SDK has no translation channel at all.
//
// Two shapes are invisible even inside the roots. Only ONE argument at ONE
// index is collected per call, so a German literal arriving through a %s of a
// tf or Titlef is a leak; and the template regex is anchored to the start of
// the action, so {{if eq (t "…")}}, {{$x := t "…"}} and {{.Foo | t}} are not
// matched. Neither shape is in the tree today, and both are cheap to reach for
// by accident.
//
// # The header count is of COLLECTABLE strings, not of German literals
//
// "N strings in the source" counts what this tool can see, which is not
// the same as what a person wrote. A sentence that has always been in the code
// but sat in a shape the collector cannot read — assembled with fmt.Sprintf,
// returned from a helper, surfaced through err.Error() — is absent from that
// number, and it JOINS the number on the day somebody moves it into a
// collectable shape, with no new sentence written anywhere.
//
// Measured 2026-09-08: seven keys counted as added between two commits although
// every one of them predated the first. They became collectable at 6efb3ba,
// which moved six album sentences out of a helper and into SetFlash*. A plan
// whose gate read "the count rises by exactly the two sentences this phase
// added" therefore failed against a tree where the property held perfectly.
//
// So a delta in this number answers "how much more can be translated", not
// "how much more was written". Where the difference matters, diff the KEY SET
// through this tool rather than comparing two totals.
//
// i18n.SourceStrings, which the admin's own language screen counts, is built
// from the union of the catalogues rather than from source. It therefore
// inherits every blind spot above exactly: a string this tool never collected
// is in no catalogue, so that screen reports full coverage over it. Two green
// lights, one blind spot, and the second is the one an operator looks at.
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
var callsInTemplates = regexp.MustCompile(`\{\{-?\s*(?:t|th|tf|thf)\s+("(?:[^"\\]|\\.)*")`)

// goFuncs are the Go functions whose string argument a person reads, by the
// ARGUMENT INDEX at which that string sits — not a count.
//
// SetFlash* translate what they are given, Errors.Add is rendered through {{t}}
// in the template, and NewLayoutData translates the title it is handed.
//
// Matched by NAME alone, with no package or receiver check, so any method
// called Add, T or N under internal/ lands here. That errs towards collecting
// too much, which costs a stray key; the other direction costs a German
// sentence on an English screen.
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
	// Tf has no call site with a literal today. It is here because i18n.Tf
	// exists and is exported: the day somebody writes i18n.Tf(lang, "…") the
	// sentence has to be collected, and finding that out from a French screen
	// is finding it out too late.
	"Tf": 1,
	// tr and trs are internal/admin/page_form.go's two wrappers around Titlef
	// and T for the places that have no request. Without them here their two
	// sentences were collected by nothing and stayed German through the whole
	// of v2.0's flip — criterion 9 in one file.
	"tr":  1,
	"trs": 1,
}

// regional names the catalogues that are deviation lists rather than
// translations. Named one by one, and that is the whole point.
//
// The test used to be "does the filename contain a hyphen", which is right for
// exactly these three and silently wrong for the next locale somebody adds:
// pt-BR.json, zh-Hans.json and en-GB.json are full translations with a hyphen
// in the name, and each would have been classified as a deviation list and
// never checked for "open" again. The gate would have gone on reading green
// over a catalogue nobody was filling in.
//
// A list has to be edited when a regional fassung is added, which is the
// property wanted here: adding one is a decision, and a decision should cost a
// line.
var regional = map[string]bool{
	"de-CH": true,
	"fr-CH": true,
	"it-CH": true,
}

func main() {
	write := flag.Bool("write", false, "add missing keys to the catalogues")
	swiss := flag.Bool("schweiz", false, "rebuild de-CH.json from de.json")
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
	fmt.Printf("%d strings in the source\n", len(sorted))

	if *swiss {
		// de-CH derives from de.json and no longer from the source.
		//
		// Until v2.0 the source WAS German, so the Swiss edition could be
		// derived from the keys themselves. Since the flip the keys are
		// English, and applying the ß rule to an English sentence would produce
		// a de-CH entry that is not German at all.
		german, err := readCatalog(filepath.Join(dir, "de.json"))
		if err != nil {
			fail(fmt.Errorf("de-CH derives from de.json, which could not be read: %w", err))
		}
		if err := writeSwiss(filepath.Join(dir, "de-CH.json"), sorted, german); err != nil {
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
		if regional[strings.TrimSuffix(e.Name(), ".json")] {
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
			upkeep := "read only, kept by hand"
			if e.Name() == "de-CH.json" {
				upkeep = "generated by -schweiz"
			}
			fmt.Printf("%-12s %d deviations, %d without a counterpart — %s\n", e.Name(), len(catalog), wrong, upkeep)
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
		fmt.Printf("%-12s %d translated, %d open, %d orphaned\n", e.Name(), done, missing, len(stale))
		// Name the orphans. Nothing is deleted — a sentence often comes back a
		// commit later — but a report that says only "1 orphaned" leaves
		// somebody to search the file by hand.
		for i, k := range stale {
			if i == 5 {
				fmt.Printf("  … and %d more\n", len(stale)-5)
				break
			}
			fmt.Printf("  orphaned: %q\n", k)
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
var swissSpelling = strings.NewReplacer("ß", "ss", "„", "«", "“", "»") //nolint:german — the letter the Swiss rule replaces

// writeSwiss rebuilds de-CH.json.
//
// Entries the rule produces are rewritten; entries about something else — a
// word Switzerland simply uses differently — are kept as they are. So a
// translator can add one by hand without the next run throwing it away.
func writeSwiss(path string, sources []string, german map[string]string) error {
	kept, err := readCatalog(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	out := map[string]string{}
	byRule := 0
	for _, key := range sources {
		// The key is English; what the rule applies to is its GERMAN
		// translation. A key with no German translation has nothing to make
		// Swiss, and the gate reports it as open on de.json rather than here.
		de, ok := german[key]
		if !ok || de == "" {
			continue
		}
		if swiss := swissSpelling.Replace(de); swiss != de {
			out[key] = swiss
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
	fmt.Printf("de-CH.json  %d by rule, %d by hand\n", byRule, byHand)
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
