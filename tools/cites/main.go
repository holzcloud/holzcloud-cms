// Command cites checks the file-and-line citations in this project's comments.
//
//	go run ./tools/cites            # report every citation and what it lands on
//	go run ./tools/cites -check     # exit non-zero if one cannot be resolved
//	go run ./tools/cites -drift     # only the ones that look wrong
//
// # Why this exists
//
// This repository puts its reasoning in comments rather than in a wiki, and the
// comments cite each other: "media.go:377", "field.go:740-800", "pagedata.go
// :384-399". That is a good habit with one bad property — a line number is a
// fact about a file at one moment, and every edit above it makes the citation
// point somewhere else. Nothing notices.
//
// Measured on 2026-09-15: 108 citations, one pointing past the end of its file
// (media.go:377 in a file of 322 lines) and at least one pointing at an
// unrelated comment line. Both sat in the SAME comment, four of whose six
// citations were still right — which is what rot looks like. It is not a
// document going wrong all at once; it is a document going wrong one line at a
// time while the rest of it stays true and keeps the reader trusting it.
//
// # What it can and cannot decide
//
// It can decide whether a citation resolves: the file exists, the line exists.
// That is mechanical — but it is NOT what -check holds, and the difference is
// the whole lesson. A citation that resolves today drifts tomorrow and says
// nothing while it does. So -check refuses the FORM: after `-fix` there are no
// line citations left, and a new one fails the build.
//
// It cannot decide whether the line still says what the citing comment claims.
// Nothing can, short of reading both. What it does instead is name the
// declaration the cited line falls inside, so that a reader comparing the two
// has the answer in front of them — and it flags the shapes that are almost
// always drift: a citation landing on a blank line, on a bare closing brace, or
// on a line that is itself only a comment.
//
// # The durable form
//
// A citation that names a SYMBOL instead of a line does not rot: "media.go's
// HandleMediaServe" survives every edit above it. Where this tool can see which
// declaration was meant, it says so, so that converting a citation is a matter
// of copying the name rather than working it out again.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// citation is one "file.go:123" inside one comment.
type citation struct {
	from string // the file the comment is in
	line int    // where the comment is
	ref  string // the file it names, as written
	at   int    // the line it names
	text string // the whole citation as written, range and all
}

// citePattern matches a Go file name followed by a line number.
//
// The file part deliberately allows a path, because a few citations carry one
// ("internal/public/pagedata.go:384"), and deliberately does not allow spaces:
// a sentence that happens to end in ".go" before a number is not a citation.
var citePattern = regexp.MustCompile(`([A-Za-z0-9_.\-/]+\.go):(\d+)(-\d+)?`)

// declPattern matches the declarations a citation can usefully land inside.
var declPattern = regexp.MustCompile(`^(?:func (?:\([^)]*\) )?(\w+)|type (\w+)|var (\w+)|const (\w+)|var \(|const \()`)

func main() {
	check := flag.Bool("check", false, "exit non-zero if any comment still cites a line number")
	drift := flag.Bool("drift", false, "report only citations that look wrong")
	fix := flag.Bool("fix", false, "rewrite every citation that names a line into one that names a declaration")
	flag.Parse()

	files, err := goFiles(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	// The index is by base name as well as by path, because most citations are
	// written the short way — the same way somebody says "media.go" out loud.
	byPath := map[string]bool{}
	byBase := map[string][]string{}
	for _, f := range files {
		byPath[f] = true
		byBase[filepath.Base(f)] = append(byBase[filepath.Base(f)], f)
	}

	self := filepath.Clean("tools/cites/main.go")
	var cites []citation
	for _, f := range files {
		// This file's own documentation quotes citations as examples — a dead
		// one, an ambiguous one, an invented one — because that is how the
		// problem is explained. Reporting them would mean this tool can never
		// be green while it still says what it is for.
		if f == self {
			continue
		}
		cites = append(cites, citationsIn(f)...)
	}

	if *fix {
		rewrite(cites, files, byPath, byBase)
		return
	}

	var dead, ambiguous, suspicious int
	for _, c := range cites {
		target, why := resolve(c, files, byPath, byBase)
		if target == "" {
			if strings.HasPrefix(why, "ambiguous") {
				ambiguous++
			} else {
				dead++
			}
			fmt.Printf("%s:%d  %s:%d  — %s\n", c.from, c.line, c.ref, c.at, why)
			continue
		}
		lines := readLines(target)
		if c.at > len(lines) {
			dead++
			fmt.Printf("%s:%d  %s:%d  — that file has %d lines\n", c.from, c.line, c.ref, c.at, len(lines))
			continue
		}
		text := strings.TrimSpace(lines[c.at-1])
		odd := text == "" || text == "}" || text == ")" || strings.HasPrefix(text, "//")
		if odd {
			suspicious++
		}
		if *drift && !odd {
			continue
		}
		note := ""
		if d := enclosing(lines, c.at); d != "" {
			note = "  [in " + d + "]"
		}
		mark := " "
		if odd {
			mark = "?"
		}
		fmt.Printf("%s %s:%d  %s:%d  %s%s\n", mark, c.from, c.line, c.ref, c.at, short(text), note)
	}

	fmt.Printf("\n%d citations, %d unresolvable, %d ambiguous, %d landing on a blank line, a brace or a comment\n",
		len(cites), dead, ambiguous, suspicious)

	// The gate is not "every citation resolves" but "there are none". A
	// citation that resolves today is one that drifts tomorrow, silently, and
	// the tree it was measured in had 45 of its 108 already sitting on a blank
	// line or a closing brace. Holding the weaker rule would have kept every
	// one of them.
	//
	// The replacement costs nothing: a declaration's name is shorter than the
	// file and line it replaces, and it is the form this project's prose
	// already used everywhere it was not citing a number.
	if *check && len(cites) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d comments still cite a line number. Run: go run ./tools/cites -fix\n", len(cites))
		fmt.Fprintln(os.Stderr, "A line number is a fact about a file at one moment; a name is a fact about the code.")
		os.Exit(1)
	}
}

// rewrite turns "media.go:377" into "media.go's HandleMediaServe".
//
// The point is not tidiness. A line number is a fact about a file at one
// moment; a declaration name is a fact about the code. 45 of this tree's 108
// citations had already drifted onto a blank line, a closing brace or an
// unrelated comment — 42 per cent — while every one of them still read as
// though it were true.
//
// A citation this cannot place is left exactly as it was and named at the end.
// That is the honest half: where the cited line sits at package level, or in an
// import block, or in a file this tool cannot pick between, there is no name to
// put there and guessing one would be worse than the number.
func rewrite(cites []citation, all []string, byPath map[string]bool, byBase map[string][]string) {
	// Grouped by file and applied from the bottom up, so that replacing a
	// shorter string never moves the ones still to come.
	byFile := map[string][]citation{}
	for _, c := range cites {
		byFile[c.from] = append(byFile[c.from], c)
	}

	var changed, left int
	for path, list := range byFile {
		lines := readLines(path)
		touched := false
		for i := len(list) - 1; i >= 0; i-- {
			c := list[i]
			target, _ := resolve(c, all, byPath, byBase)
			if target == "" {
				left++
				fmt.Printf("left  %s:%d  %s — cannot tell which file\n", c.from, c.line, c.text)
				continue
			}
			tl := readLines(target)
			if c.at > len(tl) {
				left++
				fmt.Printf("left  %s:%d  %s — that file has %d lines\n", c.from, c.line, c.text, len(tl))
				continue
			}
			name := enclosing(tl, c.at)
			if name == "" {
				left++
				fmt.Printf("left  %s:%d  %s — no declaration around that line\n", c.from, c.line, c.text)
				continue
			}
			was := lines[c.line-1]
			now := strings.Replace(was, c.text, reference(c.from, target, name), 1)
			if now == was {
				left++
				continue
			}
			lines[c.line-1] = now
			touched = true
			changed++
		}
		if touched {
			if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
		}
	}
	fmt.Printf("\n%d citations now name a declaration, %d left as they were\n", changed, left)
}

// reference is how a citation names a declaration in prose.
//
// Go's own form, because this is Go and the reader already knows it: a bare
// name inside the package, package.Name outside it. Not "menu.go's
// HandleMenuList" — the file is not the interesting part, and saying it makes
// the sentence longer than the number it replaces, which is how a careful
// wrapping at eighty columns turns into sixty-nine lines that are too long.
//
// This is also the form the prose here already uses when it is not citing a
// line: "media.LoadImageSets' rule", "page.Slugify and not safeDownloadName".
// The rewrite is therefore not a new convention. It is the one the project
// already had, applied to the citations that had drifted away from it.
func reference(from, target, name string) string {
	if filepath.Dir(from) == filepath.Dir(target) {
		return name
	}
	pkg := packageOf(target)
	// A command's package is "main", and "main.asCSV" names nothing a reader
	// can look up — this tree has fifteen package mains. The directory is what
	// identifies it, which is how anybody refers to them out loud anyway:
	// "the kontaktformular plugin", "tools/i18n".
	if pkg == "main" {
		return filepath.Dir(target) + "'s " + name
	}
	return pkg + "." + name
}

// packageOf reads a file's package clause, falling back to its directory.
func packageOf(path string) string {
	for _, l := range readLines(path) {
		if rest, ok := strings.CutPrefix(l, "package "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return filepath.Base(filepath.Dir(path))
}

// resolve turns the name a comment wrote into a path on disk.
//
// A citation with a slash in it is taken as written. A bare name is looked for
// first in the citing file's own directory — which is where a comment saying
// "media.go" almost always means — and then across the tree.
func resolve(c citation, all []string, byPath map[string]bool, byBase map[string][]string) (string, string) {
	if strings.Contains(c.ref, "/") {
		ref := filepath.Clean(c.ref)
		if byPath[ref] {
			return ref, ""
		}
		// A partial path — "term/store.go" for internal/term/store.go — is how
		// these are usually written, because the package is the part a reader
		// needs and the prefix is noise. Match on the tail.
		var m []string
		for _, f := range all {
			if strings.HasSuffix(f, string(filepath.Separator)+ref) {
				m = append(m, f)
			}
		}
		return one(m, c)
	}
	local := filepath.Join(filepath.Dir(c.from), c.ref)
	if byPath[local] {
		return local, ""
	}
	return one(byBase[c.ref], c)
}

// one picks the single file a citation can mean, or says it cannot tell.
//
// The tie-break is the line number itself: "main.go:882" cannot mean a main.go
// of forty lines, and this tree has fifteen files called main.go. It is a weak
// signal and it is stated as one — where it leaves more than one candidate, the
// citation is reported as ambiguous rather than guessed at.
func one(m []string, c citation) (string, string) {
	switch len(m) {
	case 0:
		return "", "no such file"
	case 1:
		return m[0], ""
	}
	var fits []string
	for _, f := range m {
		if len(readLines(f)) >= c.at {
			fits = append(fits, f)
		}
	}
	if len(fits) == 1 {
		return fits[0], ""
	}
	return "", "ambiguous: " + strings.Join(m, ", ")
}

// enclosing names the declaration a cited line belongs to, or nothing.
//
// This is the whole point of the report: it is the name the citation could have
// used instead of a number, and a name does not rot.
//
// It looks FORWARD before it looks back, and that is not a refinement — it is
// the difference between right and wrong. A citation very often points at a
// declaration's doc comment rather than at its body, because the doc comment is
// where the reasoning is and the reasoning is what was being cited. Walking
// backwards from there lands on the PREVIOUS declaration and attributes the
// sentence to the wrong function.
//
// Measured while building this: menu.go:54-77 was cited for "menuOfWebsite's
// role as the ONLY guard". Line 54 is menuOfWebsite's own doc comment; the
// declaration above it is menuItemNode. A backwards-only walk rewrote that
// citation to name menuItemNode — a new lie, in a tool written to remove one.
//
// It also refuses rather than guesses. A line sitting between two declarations
// belongs to neither, and the walk back stops at the closing brace of the one
// before it instead of stepping over it.
func enclosing(lines []string, at int) string {
	// Forward: a run of comment and blank lines that ends at a declaration is
	// that declaration's doc block, and the citation is part of it.
	for j := at - 1; j < len(lines); j++ {
		t := strings.TrimSpace(lines[j])
		if t == "" || strings.HasPrefix(t, "//") {
			continue
		}
		if name := declName(lines[j]); name != "" {
			return name
		}
		break
	}
	// Backwards: the body the line sits in, and nothing further.
	for i := at - 1; i >= 0; i-- {
		if lines[i] == "}" || lines[i] == ")" {
			return ""
		}
		if name := declName(lines[i]); name != "" {
			return name
		}
	}
	return ""
}

// declName is the name a declaration line introduces, empty for anything else —
// including a grouped `var (`, which introduces no single name.
func declName(line string) string {
	m := declPattern.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	for _, g := range m[1:] {
		if g != "" {
			return g
		}
	}
	return ""
}

func citationsIn(path string) []citation {
	var out []citation
	for i, l := range readLines(path) {
		s := strings.TrimSpace(l)
		if !strings.HasPrefix(s, "//") {
			continue
		}
		for _, m := range citePattern.FindAllStringSubmatch(s, -1) {
			at, err := strconv.Atoi(m[2])
			if err != nil || at == 0 {
				continue
			}
			out = append(out, citation{from: path, line: i + 1, ref: m[1], at: at, text: m[0]})
		}
	}
	return out
}

func readLines(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Split(string(b), "\n")
}

// goFiles lists the Go source this project owns.
//
// data/ is runtime state and never source; testdata holds fixtures that are not
// this project's prose.
func goFiles(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "data", "node_modules", "testdata", ".planning":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") {
			out = append(out, filepath.Clean(p))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func short(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 58 {
		return s[:58] + "…"
	}
	return s
}
