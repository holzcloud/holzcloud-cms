// Command assembled finds operator sentences that are put together in Go.
//
//	go run ./tools/assembled          # report every one
//	go run ./tools/assembled -check   # exit non-zero if there is one
//
// # Why this exists
//
// CLAUDE.md states the rule and names the hole in the same breath: every word
// an operator reads goes through the catalogue, and **a sentence built with
// fmt.Sprintf is invisible to it, wherever it stands**. The collector reads
// CALL SITES — the arguments of web.T, web.Titlef, the SetFlash family, i18n.N
// — so a sentence that reaches one of those as a variable, or as the result of
// fmt.Sprintf, is not merely untranslated. It is UNREPORTED: `go run ./tools/i18n`
// says neither *open* nor *orphaned* about it, because it does not know it
// exists.
//
// That is the worst kind of hole in a gate, because the gate keeps saying it is
// green. Measured on 2026-09-17, the tree held eleven of them:
//
//   - seven fmt.Sprintf sentences — two of them still German on a program whose
//     source language has been English since v2.0: "%d Nachrichten werden erneut
//     versucht." and "Zugeschnitten auf %d × %d Pixel."
//   - `message := "Website angelegt"` handed to SetFlashSuccess as a variable
//   - `src.Title + " (Kopie)"`, so every installation in every language titled
//     its copies in German
//
// None of them was found by reading. Each turned up while writing a test for
// the screen it sat on, which is the whole argument of v2.4's TEST-01.
//
// # What it looks for
//
// One shape, and narrowly: a call to fmt.Sprintf standing directly in an
// argument of a function that SHOWS ITS ARGUMENT TO AN OPERATOR — web.T,
// web.Titlef, the SetFlash family, FormErrors.Add, i18n.N, i18n.T. That is the
// whole of the measured problem and it has no false positives, which matters
// more than reach: a gate that also cries about a CSP header or a `"%s:%d"`
// teaches people to waive it, and a waived gate holds nothing.
//
// The first draft was broader — any sentence-shaped Sprintf anywhere, plus any
// concatenation with a sentence-shaped literal — and found 185 things, of which
// roughly seven were real. Content-Security-Policy is assembled from eight
// string fragments and every one of them reads like prose to a crude test. The
// broad version is in the history if the narrow one ever proves too narrow.
//
// # What it deliberately does not look for
//
// A sentence assigned to a variable and handed on later, or glued together with
// +, and then passed to one of those functions. Both shapes existed here and
// both were found by writing a test for the screen rather than by a tool:
// `message := "Website angelegt"` and `src.Title + " (Kopie)"`. Catching them
// needs data flow through the function, which is a different kind of program
// from this one. If a third appears, that is the argument for building it.
//
// # The waiver
//
// A line marked `//nolint:assembled` with a reason after it, the same shape
// tools/english uses. One line, at its own site, so a reader sees why rather
// than finding a list somewhere else that has drifted.
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
	"sort"
	"strconv"
	"strings"
)

// roots are the trees whose strings an operator can read. The tools themselves
// and the test fixtures are not among them: a tool's output is for whoever ran
// it, in the one language this repository is written in.
var roots = []string{"internal", "cmd"}

// skipDirs are not operator-facing at all.
var skipDirs = map[string]bool{
	".git": true, ".planning": true, "node_modules": true, "locales": true,
	"testdata": true, "migrations": true,
}

type finding struct {
	path string
	line int
	text string
	why  string
}

func main() {
	check := flag.Bool("check", false, "exit non-zero if anything is found")
	root := flag.String("root", ".", "repository root")
	flag.Parse()

	var found []finding
	for _, r := range roots {
		err := filepath.WalkDir(filepath.Join(*root, r), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipDirs[d.Name()] {
					return fs.SkipDir
				}
				return nil
			}
			// A test's own messages are for whoever reads the failure.
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			found = append(found, check1(path)...)
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}

	sort.Slice(found, func(i, j int) bool {
		if found[i].path != found[j].path {
			return found[i].path < found[j].path
		}
		return found[i].line < found[j].line
	})
	for _, f := range found {
		fmt.Printf("%s:%d: %s\n    %s\n", f.path, f.line, f.why, f.text)
	}
	if len(found) == 0 {
		fmt.Println("no operator sentence is assembled in Go")
		return
	}
	fmt.Printf("\n%d assembled sentences\n", len(found))
	if *check {
		os.Exit(1)
	}
}

func check1(path string) []finding {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		// A file that does not parse is the compiler's problem, stated more
		// clearly than here.
		return nil
	}
	lines := strings.Split(string(src), "\n")
	waived := map[int]bool{}
	for i, l := range lines {
		if strings.Contains(l, "//nolint:assembled") {
			waived[i+1] = true
		}
	}
	clean := strings.TrimPrefix(filepath.ToSlash(path), "./")

	var out []finding
	at := func(p token.Pos) int { return fset.Position(p).Line }

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !showsItsArgument(call.Fun) {
			return true
		}
		for _, arg := range call.Args {
			inner, ok := arg.(*ast.CallExpr)
			if !ok || !isSprintf(inner.Fun) || len(inner.Args) == 0 {
				continue
			}
			text, _ := literal(inner.Args[0])
			line := at(inner.Pos())
			if waived[line] || waived[at(call.Pos())] {
				continue
			}
			out = append(out, finding{clean, line, text,
				"fmt.Sprintf inside " + name(call.Fun) + " — the collector reads the call site, so it sees a call and not a sentence"})
		}
		return true
	})
	return out
}

// showsItsArgument reports whether a call puts its argument in front of an
// operator. These are exactly the names tools/i18n collects from, which is what
// makes the hole precise: the collector wants a literal here and gets a call.
func showsItsArgument(fun ast.Expr) bool {
	switch name(fun) {
	case "web.T", "web.Titlef", "web.SetFlashError", "web.SetFlashSuccess",
		"web.SetFlashWarning", "i18n.N", "i18n.T", "i18n.Tf",
		"SetFlashError", "SetFlashSuccess", "SetFlashWarning", "Add", "warnf":
		return true
	}
	return false
}

func isSprintf(fun ast.Expr) bool { return name(fun) == "fmt.Sprintf" }

// name spells a call's function the way it is written: "fmt.Sprintf", "web.T",
// or a bare "Add" for a method.
func name(fun ast.Expr) string {
	switch v := fun.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if pkg, ok := v.X.(*ast.Ident); ok {
			return pkg.Name + "." + v.Sel.Name
		}
		return v.Sel.Name
	}
	return ""
}

func literal(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// sentenceShaped decides whether a literal is something an operator would read
// as language rather than as machinery.
//
// The test is deliberately crude and deliberately narrow: a space, at least two
// runs of letters, and none of the shapes that are obviously not prose. Being
// narrow is the point — a gate that cries about `"%s:%d"` teaches people to
// waive it, and a waived gate holds nothing.
func sentenceShaped(s string) bool {
	if !strings.Contains(s, " ") {
		return false
	}
	// A path, a query, a header, a MIME type, a SQL fragment.
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	switch {
	case strings.HasPrefix(trimmed, "/"),
		strings.HasPrefix(trimmed, "http://"), strings.HasPrefix(trimmed, "https://"),
		strings.Contains(s, "\n"),
		strings.Contains(s, "$1"),
		looksLikeSQL(trimmed):
		return false
	}
	// At least two words of two letters or more, so "%d x" and "a %s" are not
	// sentences and "Password must be at least %d characters" is.
	words := 0
	for _, f := range strings.Fields(s) {
		letters := 0
		for _, r := range f {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				letters++
			}
		}
		if letters >= 2 {
			words++
		}
	}
	return words >= 2
}

func looksLikeSQL(s string) bool {
	upper := strings.ToUpper(s)
	for _, kw := range []string{"SELECT ", "INSERT ", "UPDATE ", "DELETE ", "CREATE ", "ALTER "} {
		if strings.HasPrefix(upper, kw) {
			return true
		}
	}
	return false
}
