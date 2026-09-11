// Command rename rewrites Go identifiers and nothing else.
//
//	go run ./tools/rename -map words.tsv            # report what would change
//	go run ./tools/rename -map words.tsv -write     # do it
//	go run ./tools/rename -inventory                # list German-looking identifiers
//
// It exists because this rename cannot be done with sed. The same word is an
// identifier in one line and DATA in the next, and getting that wrong produces
// a bug that compiles:
//
//   - A catalogue key is a German sentence. Rewriting one inside a string
//     literal orphans the entry and the screen falls back to the key.
//   - A form field name ("beschriftung", "art") is a contract between an HTML
//     form and its handler, and both halves are strings.
//   - A stored value ("langtext", "uebergehen") is a row in every operator's
//     database. .planning/GLOSSARY.md records the runtime failure that taught
//     this: translating the value in `kollision IN ('uebergehen', ...)`
//     compiles cleanly and is refused by SQLite when a CHECK constraint meets
//     it, on a path no test exercised.
//
// So it reads the file with go/scanner and rewrites token.IDENT and nothing
// else. Comments and string literals are copied through byte for byte, which is
// also what keeps this separable from the comment sweep — the roadmap asks for
// the rename in one commit and the translation in the next, so that
// `git log --follow` keeps a file's lineage at any similarity threshold.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func main() {
	var (
		mapPath   = flag.String("map", "", "TSV file of old<TAB>new identifier pairs")
		write     = flag.Bool("write", false, "rewrite the files instead of reporting")
		inventory = flag.Bool("inventory", false, "list identifiers that look German")
		root      = flag.String("root", ".", "directory to walk")
	)
	flag.Parse()

	files, err := goFiles(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *inventory {
		takeInventory(files)
		return
	}
	if *mapPath == "" {
		fmt.Fprintln(os.Stderr, "need -map or -inventory")
		os.Exit(2)
	}
	pairs, err := readMap(*mapPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	total, touched := 0, 0
	for _, path := range files {
		n, err := rewrite(path, pairs, *write)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			os.Exit(1)
		}
		if n > 0 {
			total += n
			touched++
			fmt.Printf("%5d %s\n", n, path)
		}
	}
	verb := "would rename"
	if *write {
		verb = "renamed"
	}
	fmt.Printf("%s %d identifiers in %d files\n", verb, total, touched)
}

// goFiles walks the tree the way the rest of this project's tools do: the
// module and the five plugin modules, never .git and never .planning — the
// planning record is written in the language it was written in and stays that
// way.
func goFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".planning", "node_modules", "testdata":
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func readMap(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	pairs := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			return nil, fmt.Errorf("not a pair: %q", line)
		}
		pairs[parts[0]] = parts[1]
	}
	return pairs, sc.Err()
}

// rewrite replaces whole identifiers in one file.
//
// The scanner gives the position and the literal text of every token. Only
// token.IDENT is considered, so a word inside a string or a comment is never
// touched however it is spelled.
func rewrite(path string, pairs map[string]string, write bool) (int, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	fset := token.NewFileSet()
	file := fset.AddFile(path, fset.Base(), len(src))

	var s scanner.Scanner
	var scanErr error
	s.Init(file, src, func(_ token.Position, msg string) {
		if scanErr == nil {
			scanErr = fmt.Errorf("scan: %s", msg)
		}
	}, 0)

	type edit struct {
		at, width int
		with      string
	}
	var edits []edit
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok != token.IDENT {
			continue
		}
		if to, ok := pairs[lit]; ok {
			edits = append(edits, edit{at: file.Offset(pos), width: len(lit), with: to})
		}
	}
	if scanErr != nil {
		return 0, scanErr
	}
	if len(edits) == 0 || !write {
		return len(edits), nil
	}
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

// german is what an identifier written in German looks like when it does not
// carry an umlaut to give it away. Deliberately a list of stems rather than a
// cleverness: a guess that fires on "start" because it contains "art" costs
// more time than the fifty words it would have found.
var german = regexp.MustCompile(`(?i)` + strings.Join([]string{
	`seite`, `feld`, `zeile`, `kennung`, `baustein`, `galerie`, `bild`, `datei`,
	`wert`, `pruef`, `loesch`, `aendern`, `anleg`, `spalte`, `zustand`, `sprache`,
	`schlagwort`, `auswahl`, `gruppe`, `nutzer`, `benutzer`, `passwort`, `anmeld`,
	`abmeld`, `vorlage`, `archiv`, `sicher`, `beitrag`, `entwurf`, `adresse`,
	`ueberschrift`, `absatz`, `verweis`, `menge`, `preis`, `bestell`, `waehl`,
	`fehler`, `meldung`, `bericht`, `einlesen`, `hochlad`, `herunterlad`,
	`uebersetz`, `abschnitt`, `reihenfolge`, `vorgabe`, `beschriftung`, `hinweis`,
	`bedingung`, `darstellung`, `pflicht`, `mehrzahl`, `sortierung`, `modus`,
	`kollision`, `dateiname`, `daten`, `erstellt`, `geaendert`, `anzahl`, `laenge`,
	`breite`, `hoehe`, `farbe`, `schrift`, `rahmen`, `wanderung`, `ablage`,
	`marke`, `urteil`, `grund`, `zelle`, `kopfzeile`, `zuordnung`, `probelauf`,
}, `|`))

// takeInventory prints every identifier that carries an umlaut or matches one
// of the German stems, with how many times it occurs and where.
//
// It is the honest way to start: the roadmap's counts were taken on 2026-09-06
// and the tree has grown by a quarter since, so the list is measured rather
// than assumed.
func takeInventory(files []string) {
	count := map[string]int{}
	where := map[string]map[string]bool{}
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		fset := token.NewFileSet()
		f := fset.AddFile(path, fset.Base(), len(src))
		var s scanner.Scanner
		s.Init(f, src, func(token.Position, string) {}, 0)
		for {
			_, tok, lit := s.Scan()
			if tok == token.EOF {
				break
			}
			if tok != token.IDENT {
				continue
			}
			if !looksGerman(lit) {
				continue
			}
			count[lit]++
			if where[lit] == nil {
				where[lit] = map[string]bool{}
			}
			where[lit][filepath.Dir(path)] = true
		}
	}
	names := make([]string, 0, len(count))
	for n := range count {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if count[names[i]] != count[names[j]] {
			return count[names[i]] > count[names[j]]
		}
		return names[i] < names[j]
	})
	for _, n := range names {
		dirs := make([]string, 0, len(where[n]))
		for d := range where[n] {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)
		if len(dirs) > 3 {
			dirs = append(dirs[:3], "…")
		}
		fmt.Printf("%5d  %-32s %s\n", count[n], n, strings.Join(dirs, " "))
	}
	fmt.Printf("%d distinct German-looking identifiers\n", len(names))
}

func looksGerman(s string) bool {
	if strings.ContainsAny(s, "äöüÄÖÜß") {
		return true
	}
	return german.MatchString(s)
}
