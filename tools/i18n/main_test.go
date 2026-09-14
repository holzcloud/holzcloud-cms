package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// locales is the message-catalogue directory kept in the repository as source.
func locales() string { return filepath.Join("..", "..", "internal", "i18n", "locales") }

// TestCatalogsSurviveTheRoundTrip is the lock on the catalogue format.
//
// On 2 September a hand-translation pass wrote the four full catalogues back
// with a two-space indent, and the next `-write` reformatted about two and a
// half thousand lines in each of them (see .planning/WINDOWS.md, entry 1). The
// content was never in danger; the diff was. A reformat that large buries the
// one sentence somebody actually changed, and it happens silently, because
// nothing in the repository had an opinion about the format until this file.
//
// So: every catalogue must come back out of writeCatalog as the bytes that went
// into readCatalog. That covers the two things a translator's editor is likely
// to change without meaning to — the indentation and the trailing newline — and
// it covers HTML escaping, because half these sentences carry a <code> or an
// &amp; and an encoder that escaped them would fail here rather than in a diff
// nobody reads.
//
// The seven real files are the input. No catalogue is synthesised: the empty
// map is the one case where a hand-rolled writer and the standard library
// disagree ("{\n}\n" against "{}\n"), and it cannot occur in this repository —
// a test case for it would only lock in an answer to a question nobody asks.
//
// It writes into t.TempDir(), the way tools/mkbundle/pack_test.go does: a
// catalogue written beside the source would show up in everybody's
// `git status`, which is exactly the noise this test exists to prevent.
func TestCatalogsSurviveTheRoundTrip(t *testing.T) {
	dir := locales()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read the catalogue directory: %v", err)
	}

	seen := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		seen++

		path := filepath.Join(dir, e.Name())
		onDisk, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		catalog, err := readCatalog(path)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}

		written := filepath.Join(t.TempDir(), e.Name())
		if err := writeCatalog(written, catalog); err != nil {
			t.Fatalf("write %s: %v", e.Name(), err)
		}
		back, err := os.ReadFile(written)
		if err != nil {
			t.Fatalf("read back %s: %v", e.Name(), err)
		}

		if !bytes.Equal(onDisk, back) {
			t.Errorf("%s: writeCatalog does not return the file it read (%d bytes on disk, %d written)",
				e.Name(), len(onDisk), len(back))
		}
	}

	// The count is what makes this a lock rather than a spot check. A locale
	// added without a thought about the format would otherwise slip past every
	// assertion above simply by not being looked at.
	if seen != 7 {
		t.Errorf("expected 7 catalogues, found %d — a new language belongs in the same round trip", seen)
	}
}

// The classification of a catalogue used to be "does its filename contain a
// hyphen", which is right for the three that exist and silently wrong for the
// next one somebody adds: pt-BR.json, zh-Hans.json and en-GB.json are full
// translations with a hyphen in the name, and every one of them would have been
// filed as a deviation list and never checked for "open" again. The gate would
// have gone on reading green over a catalogue nobody was filling in.
//
// It is a named list now. This test holds the two properties a named list can
// lose that a rule cannot: an entry for a file that is not there, and a file
// that looks regional and is not in the list.
func TestTheRegionalListMatchesWhatIsOnDisk(t *testing.T) {
	entries, err := os.ReadDir(locales())
	if err != nil {
		t.Fatalf("read locales: %v", err)
	}

	onDisk := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		onDisk[strings.TrimSuffix(e.Name(), ".json")] = true
	}

	for name := range regional {
		if !onDisk[name] {
			t.Errorf("regional names %q and there is no %s.json — a stale entry "+
				"here classifies nothing, but it is a lie about the shape of this directory", name, name)
		}
	}

	// The other direction is the one that matters, and it is deliberately a
	// PROMPT and not a rule: a hyphenated file that is not in the list is a full
	// catalogue and IS gate-checked, which is what pt-BR.json should be. The
	// test exists so that adding one is a decision somebody made rather than a
	// default they inherited.
	for name := range onDisk {
		if strings.Contains(name, "-") && !regional[name] {
			t.Logf("%s.json has a hyphen and is not in `regional`, so it is treated "+
				"as a full catalogue and checked for missing keys. That is correct for "+
				"a translation and wrong for a deviation list — if it is one, add it.", name)
		}
	}
}

// A sentence joined across lines with + is one sentence.
//
// It is how a long sentence is written in Go, because lines have a width. Until
// v2.1 this tool read only a single literal, so every sentence written that way
// was collected by nothing and stayed in the source language — twelve of them
// were standing in the tree when somebody finally looked, two of them a year
// old. Banning the idiom would have been the wrong correction.
func TestASentenceJoinedAcrossLinesIsOneSentence(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      string
	}{
		{
			name: "one literal",
			src:  `plugin.T("Please enter your name.")`,
			want: "Please enter your name.",
		},
		{
			name: "two joined",
			src:  `plugin.T("Please enter your name. " + "We cannot answer otherwise.")`,
			want: "Please enter your name. We cannot answer otherwise.",
		},
		{
			name: "three joined, as gofmt leaves them",
			src: `plugin.T("A form of your own asks exactly what you want to know. " +
				"Put it into a page with its marker, " +
				"the way you would a snippet.")`,
			want: "A form of your own asks exactly what you want to know. " +
				"Put it into a page with its marker, the way you would a snippet.",
		},
		{
			name: "parenthesised",
			src:  `plugin.T(("Saved." + ""))`,
			want: "Saved.",
		},
		{
			// A sentence with a value glued into it is NOT a sentence with a
			// value in it: a language that wants the name first cannot say so.
			// It stays invisible here on purpose, and tools/english reports it.
			name: "joined with a value",
			src:  `plugin.T("Hello, " + name)`,
			want: "",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "x.go",
				"package p\nfunc f(name string) { "+c.src+" }", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			var got string
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				if s, ok := literalText(call.Args[0]); ok {
					got = s
				}
				return true
			})
			if got != c.want {
				t.Errorf("= %q, want %q", got, c.want)
			}
		})
	}
}

// -schweiz rebuilds de-CH.json, and "rebuild" has to include letting go.
//
// Window 23: an entry whose key no longer exists in the source survived every
// run, including the second, because writeSwiss folded the whole existing file
// back in to protect the hand-written deviations. It protected the orphans with
// them. Nothing said so; the first sign was TestFassungKeysExistInTheSource
// turning red, and the cure was to delete the line by hand (e306eeb).
//
// A dropped key is printed rather than dropped quietly. The reason the rest of
// this tool never deletes an orphan — a translation thrown away is one somebody
// has to do again — does not hold for this file: de-CH is GENERATED, so every
// rule-derived line comes back by itself the moment its key does.
func TestSchweizLetsGoOfAKeyTheSourceNoLongerHas(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "de-CH.json")

	if err := writeCatalog(path, map[string]string{
		"Save":                "Sichern",           // hand-written, key still live
		"A mass of grosze":    "Ein Haufen grosze", // a key that has gone away
		"Delete the website?": "Website löschen?",  // rule-derived, key still live
	}); err != nil {
		t.Fatal(err)
	}

	german := map[string]string{
		"Save":                "Speichern",
		"Delete the website?": "Website löschen?",
	}
	sources := []string{"Delete the website?", "Save"}

	for pass := 1; pass <= 2; pass++ {
		if err := writeSwiss(path, sources, german); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
		got, err := readCatalog(path)
		if err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
		if _, still := got["A mass of grosze"]; still {
			t.Errorf("pass %d: the orphan is still there", pass)
		}
		if got["Save"] != "Sichern" {
			t.Errorf("pass %d: the hand-written line was lost: %q", pass, got["Save"])
		}
	}
}
