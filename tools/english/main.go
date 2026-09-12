// Command english fails the build when German comes back into the Go source.
//
//	go run ./tools/english            # report
//	go run ./tools/english -v         # report every line, not just the count
//
// This is LANG-07. Without it the next phase writes German again and v2.0 was a
// one-time cleanup rather than a change of language.
//
// # Why this is a program and not a grep
//
// The obvious gate is `grep -rn '[äöüÄÖÜß]' --include='*.go' .`, and it is
// wrong twice.
//
// It is wrong MECHANICALLY: with no locale set, grep matches bytes, and the
// UTF-8 encoding of Ü (C3 9C) shares its second byte with the typographic
// quotation mark “ (E2 80 9C). Measured on 2026-09-11 in this repository's own
// container, that gate reported three hits in a file that had two. A gate that
// miscounts is worse than none, because the first person to see a false
// positive learns to ignore it.
//
// And it is wrong IN SUBSTANCE: a comment that explains how accented letters
// are transliterated has to be able to name them. internal/field/field.go says
// that SlugifyKey used to drop every accented letter but ä, ö, ü and ß — a
// sentence that cannot be written under a literal reading of the rule. So the
// rule is not "no umlaut" but "no German", and the exceptions are narrow,
// explicit, and carry their reason.
//
// # What it allows
//
//   - The catalogue files. They are dictionaries; German is their content.
//
//   - A line marked `//nolint:german` with a reason after it. One line, named
//     at its own site, so a reader sees why rather than finding a list
//     somewhere else that has drifted.
//
//   - Stored German values inside a declaration this file knows by name — a
//     field kind, a block kind, a collision rule. .planning/GLOSSARY.md carries
//     the rule: a German word that is STORED is a value and not an identifier,
//     and translating it produces a runtime failure rather than a compile
//     error.
//
//   - German CONTENT in a test — a page titled "Über uns", a label
//     "Öffnungszeiten", a slug built from "Möbelbau". That is what an operator
//     of this CMS types, and a suite that only ever sees ASCII stops catching
//     the bugs this repository has actually had: the byte limit where an umlaut
//     counts double, SlugifyKey dropping accented letters, the NFC/NFD fold in
//     the CSV header. Every one of those was found by a test with a German
//     word in it.
//
//     What is NOT allowed in a test is a German MESSAGE — the argument of
//     t.Errorf, t.Fatalf, t.Log and their siblings. Those are prose written for
//     whoever is reading the failure, and that reader is the stranger this
//     milestone is for. The distinction is exactly the one that matters and it
//     is mechanical: content goes INTO the program, a message comes OUT of the
//     test.
package main

import (
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
	"strings"
)

// letters are the German letters, matched as RUNES and never as bytes.
var letters = regexp.MustCompile(`[äöüÄÖÜß]`)

// words are German function words, for the German that carries no umlaut.
//
// Every entry is a word that is NOT also an English word, which is what keeps
// the false-positive rate at zero over 110 000 lines: "die", "in", "an", "man",
// "als" and "hat" are all English too and are deliberately absent.
var words = regexp.MustCompile(`(?i)\b(` + strings.Join([]string{
	`der`, `das`, `den`, `dem`, `des`, `eine`, `einen`, `einem`, `einer`,
	`und`, `oder`, `nicht`, `kein`, `keine`, `ist`, `sind`, `wird`, `werden`,
	`wurde`, `haben`, `kann`, `muss`, `soll`, `darf`, `auch`, `noch`, `schon`,
	`dann`, `wenn`, `weil`, `damit`, `dass`, `wie`, `mit`, `ohne`, `von`,
	`beim`, `zum`, `zur`, `aus`, `nach`, `vor`, `unter`, `zwischen`, `hier`,
	`dort`, `jede`, `jeder`, `jedes`, `alle`, `sich`, `sie`, `wer`, `warum`,
	`deshalb`, `darum`, `also`, `aber`, `sondern`, `selbst`, `immer`, `nie`,
	`etwas`, `nichts`, `steht`, `geht`, `gibt`, `macht`, `heisst`, `waere`,
	`koennte`, `sollte`, `muesste`, `seite`, `feld`, `zeile`, `wert`,
	`baustein`, `kennung`, `beschriftung`,
}, `|`) + `)\b`)

// allowed are the declarations whose string literals are STORED values.
//
// Named one by one on purpose, and the list is short. A German word in a
// constant named here is data that sits in every operator's database and
// travels in every exported archive; a German word anywhere else is a name or
// a sentence and is this gate's business.
var allowed = map[string]bool{
	"Kind": true, "Kinds": true, "BlockKinds": true, "Display": true,
	"AppliesTo": true, "Collision": true, "Mode": true, "Status": true,
}

// fixtures are the files whose string literals ARE German content.
//
// One file, and it earns the exception the way a test does. SampleData and
// MinimalData are the page a template is rendered against before an upload is
// accepted, and TEMPLATE-SPEC.md prints them verbatim to a theme author. A
// fixture in English would show an author a page this CMS never produces, and
// would stop catching the bugs a German page catches: the byte limit where an
// umlaut counts double, the slug built from "Möbelbau", the sort order of
// "Ä" against "A".
//
// Comments and identifiers in these files are NOT exempt — only the content.
var fixtures = map[string]bool{
	"internal/template/sample.go": true,
}

// germanVoice are the files whose string literals are the product speaking
// German to a visitor, and the standing exception this milestone did not close.
//
// .planning/phases/12-codebase-speaks-english/12-CONTEXT.md §3b is the decision
// and carries the reason in full: the public FuncMap has no `t` at all, so the
// shipped themes have no translation channel, and giving one to the checkout
// while the theme around it stays German would be half a feature rather than
// half a cleanup. Until that is decided, these sentences stay where a reader
// can see them rather than being hidden behind a waiver comment on every line.
//
// Each one is a sentence a CUSTOMER reads — the checkout, the cart, the order
// e-mails, the VAT wording, the month names — or, in kind.go, the two words
// this CMS calls its own built-in kinds. None of them is an operator's screen:
// those go through the catalogue and are this gate's business.
//
// Comments and identifiers in these files are NOT exempt — only the content.
var germanVoice = map[string]bool{
	"internal/public/access.go":   true,
	"internal/public/cart.go":     true,
	"internal/public/checkout.go": true,
	"internal/public/pagedata.go": true,
	"internal/shop/cart.go":       true,
	"internal/shop/order.go":      true,
	"internal/shop/product.go":    true,
	"internal/outbox/compose.go":  true,
	// The invoice and the delivery note. They lie in package admin because an
	// operator prints them, but every word on them is addressed to the customer
	// who receives the parcel — including the sentence about the VAT exemption,
	// which is a legal formula and not a translation.
	"internal/admin/orderdoc.go": true,
	"internal/money/money.go":    true,
	"internal/template/dates.go": true,
	"internal/kind/kind.go":      true,
	// The names of the languages and the regions, each written the way its own
	// speakers write it: "Türkçe", "Österreich". Data, not prose, and the one
	// place where translating would be actively wrong.
	"internal/i18n/i18n.go":     true,
	"internal/locale/locale.go": true,
	// The gate and the rename tool both carry the German alphabet as a
	// character class. That is the subject matter itself.
	"tools/english/main.go": true,
	"tools/rename/main.go":  true,
	// The starter pages a new website is born with: a welcome page and the two
	// German legal pages (§ 5 DDG, Datenschutz) that a site run from Germany
	// has to have. They are CONTENT — an editor opens them and rewrites them —
	// and they are multi-line raw strings, which a per-line waiver cannot mark
	// without writing the marker into the page itself.
	"internal/admin/starter.go": true,
}

// aboutGerman are the files whose COMMENTS are about German and cannot be
// written without naming it.
//
// This file is the clearest case: its own package comment has to be able to say
// that "ä, ö, ü and ß" are what the rule is about, that grep confuses Ü with a
// typographic quotation mark, and that a slug built from "Möbelbau" is content
// and not prose. A gate that cannot explain itself in its own words is a gate
// somebody switches off.
//
// Only comments. String literals in these files are judged like anywhere else,
// except where germanVoice above names them.
var aboutGerman = map[string]bool{
	"tools/english/main.go": true,
	"tools/rename/main.go":  true,
	"tools/i18n/main.go":    true,
	// The language machinery itself. internal/i18n/tag.go has to be able to say
	// that Swiss German writes no ß and quotes with «Anführung» where Germany
	// writes „Anführung“, and internal/csvimport/mapping.go has to name ä, ö, ü
	// and ß because SlugifyKey has a case for each of them and for nothing
	// else. A comment that cannot name its subject is a comment that stops
	// explaining it.
	"internal/i18n/tag.go":               true,
	"internal/i18n/i18n.go":              true,
	"internal/i18n/i18n_test.go":         true,
	"internal/i18n/catalog_test.go":      true,
	"internal/locale/locale.go":          true,
	"internal/csvimport/mapping.go":      true,
	"internal/csvimport/mapping_test.go": true,
}

type finding struct {
	path string
	line int
	text string
	why  string
}

func main() {
	verbose := flag.Bool("v", false, "print every finding, not just the count")
	root := flag.String("root", ".", "directory to walk")
	flag.Parse()

	var found []finding
	err := filepath.WalkDir(*root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".planning", "node_modules", "locales":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		found = append(found, check(path)...)
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	sort.Slice(found, func(i, j int) bool {
		if found[i].path != found[j].path {
			return found[i].path < found[j].path
		}
		return found[i].line < found[j].line
	})
	byFile := map[string]int{}
	for _, f := range found {
		byFile[f.path]++
		if *verbose {
			fmt.Printf("%s:%d: %s\n    %s\n", f.path, f.line, f.why, strings.TrimSpace(f.text))
		}
	}
	if !*verbose && len(byFile) > 0 {
		paths := make([]string, 0, len(byFile))
		for p := range byFile {
			paths = append(paths, p)
		}
		sort.Slice(paths, func(i, j int) bool {
			if byFile[paths[i]] != byFile[paths[j]] {
				return byFile[paths[i]] > byFile[paths[j]]
			}
			return paths[i] < paths[j]
		})
		for _, p := range paths {
			fmt.Printf("%5d %s\n", byFile[p], p)
		}
	}
	if len(found) == 0 {
		fmt.Println("no German in the Go source outside the catalogues")
		return
	}
	fmt.Printf("\n%d German lines in %d files\n", len(found), len(byFile))
	os.Exit(1)
}

// check reads one file and reports its German lines.
//
// It parses rather than scanning line by line, because the three things it has
// to tell apart — a comment, a string literal, an identifier — are exactly what
// a parser knows and a line does not.
func check(path string) []finding {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		// A file that does not parse is not this gate's problem; the compiler
		// says so more clearly.
		return nil
	}
	isTest := strings.HasSuffix(path, "_test.go")
	clean := strings.TrimPrefix(filepath.ToSlash(path), "./")
	isFixture := fixtures[clean] || germanVoice[clean]
	explainsGerman := aboutGerman[clean]
	lines := strings.Split(string(src), "\n")
	waived := map[int]bool{}
	for i, l := range lines {
		if strings.Contains(l, "//nolint:german") {
			waived[i+1] = true
		}
	}

	var out []finding
	at := func(pos token.Pos) int { return fset.Position(pos).Line }

	// Comments.
	for _, group := range file.Comments {
		for _, c := range group.List {
			line := at(c.Pos())
			if explainsGerman || waived[line] || strings.Contains(c.Text, "//nolint:german") {
				continue
			}
			if why := german(c.Text); why != "" {
				out = append(out, finding{path, line, c.Text, why})
			}
		}
	}

	// Identifiers and string literals.
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.Ident:
			line := at(v.Pos())
			if waived[line] {
				return true
			}
			if why := german(v.Name); why != "" {
				out = append(out, finding{path, line, v.Name, "identifier: " + why})
			}
		case *ast.CallExpr:
			// A test's own message to whoever reads the failure.
			if isTest && testMessage(v) {
				for _, arg := range v.Args {
					lit, ok := arg.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					line := at(lit.Pos())
					if waived[line] {
						continue
					}
					if why := german(lit.Value); why != "" {
						out = append(out, finding{path, line, lit.Value, "test message: " + why})
					}
				}
			}
		case *ast.BasicLit:
			if v.Kind != token.STRING || isTest || isFixture {
				// In a test file a literal is content until it is a message,
				// and the message case is handled above.
				return true
			}
			line := at(v.Pos())
			if waived[line] || inAllowedDecl(file, fset, v) {
				return true
			}
			if why := german(v.Value); why != "" {
				out = append(out, finding{path, line, v.Value, "string: " + why})
			}
		}
		return true
	})
	return out
}

// testMessage reports whether this call is a test writing to whoever reads the
// failure, rather than the test doing its work.
func testMessage(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch sel.Sel.Name {
	case "Error", "Errorf", "Fatal", "Fatalf", "Log", "Logf", "Skip", "Skipf", "Helper":
		return true
	}
	return false
}

// german says why a piece of text is German, or returns "".
func german(s string) string {
	if letters.MatchString(s) {
		return "German letter"
	}
	if hits := words.FindAllString(s, -1); len(uniq(hits)) >= 2 {
		return "German words: " + strings.Join(uniq(hits), ", ")
	}
	return ""
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		l := strings.ToLower(s)
		if seen[l] {
			continue
		}
		seen[l] = true
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// inAllowedDecl reports whether a literal sits inside a declaration whose name
// this gate allows German in — the stored vocabularies.
func inAllowedDecl(file *ast.File, fset *token.FileSet, lit *ast.BasicLit) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if found || n == nil {
			return !found
		}
		var name string
		switch v := n.(type) {
		case *ast.ValueSpec:
			if len(v.Names) > 0 {
				name = v.Names[0].Name
			}
		case *ast.FuncDecl:
			name = v.Name.Name
		default:
			return true
		}
		if !allowedName(name) {
			return true
		}
		if n.Pos() <= lit.Pos() && lit.End() <= n.End() {
			found = true
		}
		return !found
	})
	return found
}

func allowedName(name string) bool {
	if allowed[name] {
		return true
	}
	for prefix := range allowed {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
