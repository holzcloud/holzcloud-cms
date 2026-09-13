// Command themewords writes each shipped theme's lang/<tag>.json out of one
// shared vocabulary.
//
// The eight themes mint the same words. Measured 2026-09-13, before any of them
// carried a catalogue: 150 distinct strings in 955 places, and nearly every one
// of them in all eight. Eight hand-kept catalogues would be eight copies
// drifting apart from their first correction onward — somebody improves the
// French for "Carry on shopping" in `journal` and seven themes keep the old
// one, with nothing to say so.
//
// So there is one source, words.json, and this writes the eight sets out of it.
// Each theme gets only the keys its own templates ask for, because a catalogue
// full of words a theme never says is noise for whoever reads it and a lie in
// the count `holzcloud template check` prints.
//
// A theme a stranger uploads has nothing to do with this: it brings its own
// lang/ or it does not, and `template check` tells its author which words are
// missing. This tool is for the themes we ship.
//
//	go run ./tools/themewords          write them
//	go run ./tools/themewords -check   say whether what is on disk is current
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// themeRoot is where the shipped themes live.
const themeRoot = "cmd/holzcloud/templates/public"

// sourceFile is the one vocabulary. It lives beside this program rather than
// beside the themes, because it is not part of any one of them.
const sourceFile = "tools/themewords/words.json"

// mintedByTheProgram are words a theme needs and its templates never ask for.
//
// The titles of the 404 and the maintenance page are filled IN to
// {{.Page.Title}} by internal/template, so KeysUsed cannot see them: no
// template contains them, and yet every theme has to translate them or a French
// site serves a page titled "Seite nicht gefunden". //nolint:german — the title this rule removes
// They are the only two, and
// Loader.Word is the only thing that reads them.
var mintedByTheProgram = []string{"Page not found", "Down for maintenance"}

// languages are the tags written out. English is not among them: the key IS the
// English, and a file that repeated every key as its own value would be a
// hundred and thirty-seven chances to let the two drift apart.
var languages = []string{"de", "fr", "it", "es"}

func main() {
	check := flag.Bool("check", false, "report whether the catalogues on disk match the source")
	root := flag.String("root", ".", "repository root")
	flag.Parse()

	raw, err := os.ReadFile(filepath.Join(*root, sourceFile))
	if err != nil {
		fail(err)
	}
	var words map[string]map[string]string
	if err := json.Unmarshal(raw, &words); err != nil {
		fail(fmt.Errorf("%s: %w", sourceFile, err))
	}

	themes, err := os.ReadDir(filepath.Join(*root, themeRoot))
	if err != nil {
		fail(err)
	}

	stale, missing := 0, map[string][]string{}
	for _, theme := range themes {
		if !theme.IsDir() {
			continue
		}
		dir := filepath.Join(*root, themeRoot, theme.Name())
		keys := tmpl.KeysUsed(os.DirFS(dir))
		if len(keys) == 0 {
			continue
		}
		for _, k := range mintedByTheProgram {
			if !contains(keys, k) {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)

		// A key the templates ask for and the source does not carry is the one
		// failure worth stopping for: the word would show as itself on every
		// page, in every language, and nothing else would say so.
		for _, k := range keys {
			if _, ok := words[k]; !ok {
				missing[theme.Name()] = append(missing[theme.Name()], k)
			}
		}

		for _, lang := range languages {
			out := map[string]string{}
			for _, k := range keys {
				if entry, ok := words[k]; ok {
					if v := entry[lang]; v != "" {
						out[k] = v
					}
				}
			}
			path := filepath.Join(dir, "lang", lang+".json")
			body, err := json.MarshalIndent(out, "", " ")
			if err != nil {
				fail(err)
			}
			body = append(body, '\n')

			old, _ := os.ReadFile(path)
			if string(old) == string(body) {
				continue
			}
			stale++
			if *check {
				fmt.Printf("out of date: %s\n", path)
				continue
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				fail(err)
			}
			if err := os.WriteFile(path, body, 0o644); err != nil {
				fail(err)
			}
			fmt.Printf("%-9s %-3s %3d words\n", theme.Name(), lang, len(out))
		}
	}

	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for t := range missing {
			names = append(names, t)
		}
		sort.Strings(names)
		for _, t := range names {
			sort.Strings(missing[t])
			fmt.Fprintf(os.Stderr, "%s asks for %d words that %s does not carry: %v\n",
				t, len(missing[t]), sourceFile, missing[t])
		}
		os.Exit(1)
	}

	switch {
	case *check && stale > 0:
		fmt.Fprintf(os.Stderr, "\n%d catalogue(s) out of date. Rebuild with: go run ./tools/themewords\n", stale)
		os.Exit(1)
	case *check:
		fmt.Println("every shipped theme's catalogue matches", sourceFile)
	case stale == 0:
		fmt.Println("nothing to do; the catalogues already match", sourceFile)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
