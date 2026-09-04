// Command wasm builds the WebAssembly guest modules this repository keeps
// committed, and reports what came out.
//
//	go run ./tools/wasm                           # build all six into the tree
//	go run ./tools/wasm jahreszahl                # build one of them
//	go run ./tools/wasm -check                    # compare against the committed files
//	go run ./tools/wasm -print-hashes             # print hash, size and compiler
//	go run ./tools/wasm -out /tmp/wasm            # build out of the tree
//
// Six .wasm files live in git: the five plugins under plugins/ and the echo
// module the plugin-host tests load from internal/plugin/testdata. They are
// committed so the tests run without a second toolchain, and that is exactly
// how they go stale — nothing today notices when the SDK moves and the binary
// beside it does not. This tool exists to make the committed file and its
// source comparable again, by building each module in a way that produces the
// same bytes on every machine.
//
// Two settings do the whole job, and both are load-bearing.
//
// The compiler is pinned with GOTOOLCHAIN, inside this tool rather than in CI's
// setup-go or a toolchain line in go.mod. The hash of a guest tracks the
// compiler that produced it — a patch bump moves hundreds of kilobytes without
// a line of source changing — so a contributor and the runner have to use the
// same one, or the comparison is red for a reason nobody caused.
//
// And -buildvcs=false is not an optimisation, it is the precondition. go build
// defaults to -buildvcs=auto and stamps vcs.revision, vcs.time and vcs.modified
// into the module. vcs.revision is the git SHA at build time, so a rebuild
// happens at a different commit than the one that produced the committed file
// by construction, and the two can never match. No toolchain pin rescues that.
// The flag drops those 133 bytes of git metadata from each guest, and yes, that
// is a deliberate reduction in provenance: the commit that carries the artifact
// is its provenance. Anyone tempted to re-enable stamping for traceability is
// trading a check that runs on every push for metadata nobody reads, and the
// trade is silent — the comparison simply goes red forever.
//
// Four modes, exactly one at a time. Without a flag the built modules replace
// the committed ones. -check builds the same modules and compares them against
// what is in the tree, naming both hashes, both sizes and both embedded Go
// versions on a mismatch — that last field earns its place because a mismatch
// has two very different causes, the pin moved or the source changed, and the
// report should let a human tell them apart without a second command. -check
// reports every mismatch before it exits 1; a tool that stops at the first of
// six turns one CI run into six. -print-hashes reports without comparing, and
// -out builds into a directory of your choosing and touches nothing in the
// tree — the answer if a future Go release ever breaks the byte equality
// between a contributor's machine and the runner.
//
// Four of the plugins also carry a .zip beside them, holding a byte-identical
// copy of the module. Rebuilding the modules without repacking those would
// recreate one layer up exactly the staleness this tool exists to prevent, so
// the archives are packed here too, from the module of the same run, and every
// mode covers all ten artifacts at once.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The compiler every guest is built with. It has a floor: the go command
// refuses to load a module declaring a higher minimum than the running
// toolchain, and internal/plugin/testdata/echo lives in the root module, so
// this must be greater than or equal to the `go` directive in ./go.mod
// (go 1.26.6 today). bodenPruefen enforces that at startup.
//
// Bare, never "go1.26.6+auto" — the +auto form selects a newer toolchain when
// one is needed, which is the pin defeating itself.
const goToolchain = "go1.26.6"

// ziel is one committed guest module: where it is built and where the built
// file belongs in the repository.
type ziel struct {
	name   string // what a positional argument selects it by
	dir    string // module directory, relative to the repository root
	out    string // committed artifact, relative to the repository root
	archiv string // committed archive beside it, empty when the plugin has none
}

// echo is not a plugin and therefore has no archive; every plugin has one.
//
// kontaktformular used to be excluded here because it carries a migrations/
// directory that the packer's fixed two-entry layout would have dropped. That
// was a hazard documented as an exception rather than guarded: the next plugin
// to grow an assets/ or migrations/ directory would have been silently
// mispacked, with nothing to notice it. packen now reads the plugin's own
// directory, so the exception has no reason left.
var ziele = []ziel{
	{"bestellung", "plugins/bestellung", "plugins/bestellung/plugin.wasm", "plugins/bestellung/bestellung.zip"},
	{"jahreszahl", "plugins/jahreszahl", "plugins/jahreszahl/plugin.wasm", "plugins/jahreszahl/jahreszahl.zip"},
	{"kontaktformular", "plugins/kontaktformular", "plugins/kontaktformular/plugin.wasm", "plugins/kontaktformular/kontaktformular.zip"},
	{"nicht-gefunden", "plugins/nicht-gefunden", "plugins/nicht-gefunden/plugin.wasm", "plugins/nicht-gefunden/nicht-gefunden.zip"},
	{"suche", "plugins/suche", "plugins/suche/plugin.wasm", "plugins/suche/suche.zip"},
	{"echo", "internal/plugin/testdata/echo", "internal/plugin/testdata/echo.wasm", ""},
}

// What an archive holds: the manifest, then the module, flat and in that order.
// This is what the four committed archives contain today and what the zip -j
// line in plugins/README.md produces.
const (
	manifestName = "plugin.json"
	modulName    = "plugin.wasm"
	assetDir     = "assets/"
	migrationDir = "migrations/"
)

// sammeln reads one of the two directories the package format admits and
// returns its files as archive entries, sorted by name so two runs over the
// same directory produce the same bytes — filepath.WalkDir already walks in
// lexical order, and the sort says so rather than relying on it.
//
// A missing directory is the normal case, not an error: four of the five
// plugins carry neither.
func sammeln(quelle, verzeichnis string) ([]struct {
	name   string
	inhalt []byte
}, error) {
	wurzel := filepath.Join(quelle, filepath.FromSlash(verzeichnis))
	if _, err := os.Stat(wurzel); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	var gefunden []struct {
		name   string
		inhalt []byte
	}
	err := filepath.WalkDir(wurzel, func(pfad string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(wurzel, pfad)
		if err != nil {
			return err
		}
		inhalt, err := os.ReadFile(pfad)
		if err != nil {
			return err
		}
		gefunden = append(gefunden, struct {
			name   string
			inhalt []byte
		}{verzeichnis + filepath.ToSlash(rel), inhalt})
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(gefunden, func(a, b struct {
		name   string
		inhalt []byte
	}) int {
		return strings.Compare(a.name, b.name)
	})
	return gefunden, nil
}

// The timestamp every archive entry carries. archive/zip stamps the current
// time when Modified is left unset, so two runs from identical input would
// produce different bytes and the comparison could never hold. The value itself
// is arbitrary and only has to be constant; this is the earliest instant the
// zip format can represent, which reads as "deliberately none" rather than as a
// date somebody might mistake for provenance — the commit carrying the archive
// is its provenance, exactly as with -buildvcs=false.
var archivZeit = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

// artefakt is one produced file, held in memory. Every mode works from these
// bytes and none of them builds its own: what -check compares is byte for byte
// what the default mode would write, because it is the same slice.
type artefakt struct {
	label  string // short name for the report column
	ziel   string // committed path, relative to the repository root
	datei  string // file name used by -out
	quelle string // the target it came from, so a report can name the command that rebuilds it
	inhalt []byte
}

// The variables the build controls. They are stripped from the inherited
// environment and set once, instead of appended on top of it. os/exec does
// deduplicate and keeps the later value, but that is a property of the parent's
// exec package rather than of the child, and a Go program reading its own
// environment keeps the *first* mention of a key. Filtering removes the
// dependence on which of the two rules happens to apply.
// GOWASM and GOFIPS140 are on the list because they change the produced bytes
// with identical source and an identical pinned compiler. GOWASM is the acute
// one — every target here is a wasm build, so it is the knob most likely to be
// set for an unrelated reason and least likely to be suspected afterwards.
var gesteuert = []string{
	"GOOS", "GOARCH", "CGO_ENABLED", "GOTOOLCHAIN", "GOENV",
	"GOFLAGS", "GOEXPERIMENT", "GOWASM", "GOFIPS140",
}

func main() {
	root := flag.String("root", ".", "repository root")
	printHashes := flag.Bool("print-hashes", false, "build the selected modules and print hash, size and compiler")
	check := flag.Bool("check", false, "compare the built modules against the committed files, exit 1 on a mismatch")
	out := flag.String("out", "", "build the selected modules into this directory instead of the working tree")
	flag.Parse()

	modi := 0
	for _, gewaehlt := range []bool{*printHashes, *check, *out != ""} {
		if gewaehlt {
			modi++
		}
	}
	if modi > 1 {
		usage("exactly one mode at a time")
	}

	auswahl, err := waehlen(flag.Args())
	if err != nil {
		usage(err.Error())
	}
	if err := bodenPruefen(*root); err != nil {
		fail(err)
	}

	switch {
	case *check:
		abweichungen, err := vergleichen(*root, auswahl)
		if err != nil {
			fail(err)
		}
		if abweichungen > 0 {
			fmt.Printf("\n%d Datei(en) sind nicht aktuell. Neu bauen mit: go run ./tools/wasm\n", abweichungen)
			os.Exit(1)
		}
	case *printHashes:
		if err := hashesDrucken(*root, auswahl); err != nil {
			fail(err)
		}
	case *out != "":
		if err := nachVerzeichnis(*root, auswahl, *out); err != nil {
			fail(err)
		}
	default:
		if err := inDenBaum(*root, auswahl); err != nil {
			fail(err)
		}
	}
}

// waehlen resolves positional arguments to targets. No argument means all six.
func waehlen(namen []string) ([]ziel, error) {
	if len(namen) == 0 {
		return ziele, nil
	}
	var gewaehlt []ziel
	for _, n := range namen {
		i := slices.IndexFunc(ziele, func(z ziel) bool { return z.name == n })
		if i < 0 {
			return nil, fmt.Errorf("unknown target %q", n)
		}
		gewaehlt = append(gewaehlt, ziele[i])
	}
	return gewaehlt, nil
}

// goVorgabe finds the `go` directive of a go.mod without parsing the file.
var goVorgabe = regexp.MustCompile(`(?m)^go[ \t]+([0-9]+(?:\.[0-9]+)*)`)

// bodenPruefen refuses to do anything when the pinned toolchain is older than
// the root go.mod's `go` directive. internal/plugin/testdata/echo lives in the
// root module, so in that situation the go command declines to load exactly one
// of the six targets, deep inside a build, and the comparison goes half red for
// a reason that has nothing to do with the guests. One sentence naming both
// numbers is worth more than five green lines and one puzzling failure. Whoever
// raises the directive raises the constant in the same commit.
func bodenPruefen(root string) error {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	m := goVorgabe.FindSubmatch(b)
	if m == nil {
		return nil
	}
	boden := string(m[1])
	if kleiner(strings.TrimPrefix(goToolchain, "go"), boden) {
		return fmt.Errorf("die feste Werkzeugkette %s ist älter als die Vorgabe »go %s« in go.mod; "+
			"echo liegt im Wurzelmodul und liesse sich damit nicht bauen. "+
			"goToolchain in tools/wasm/main.go im selben Commit anheben", goToolchain, boden)
	}
	return nil
}

// kleiner compares two dotted version numbers component by component. A missing
// component counts as zero, so "1.26" is below "1.26.6".
func kleiner(a, b string) bool {
	az, bz := zahlen(a), zahlen(b)
	for i := 0; i < len(az) || i < len(bz); i++ {
		var x, y int
		if i < len(az) {
			x = az[i]
		}
		if i < len(bz) {
			y = bz[i]
		}
		if x != y {
			return x < y
		}
	}
	return false
}

func zahlen(v string) []int {
	teile := strings.Split(v, ".")
	n := make([]int, 0, len(teile))
	for _, t := range teile {
		i, _ := strconv.Atoi(t)
		n = append(n, i)
	}
	return n
}

// jeArtefakt builds every selected target in one temporary directory and hands
// each produced artifact to fn. The build happens outside the working tree in
// every mode, including the one that writes into it: a build that fails then
// cannot leave a truncated file behind, because it never had the destination
// open. Installing into the tree is atomarSchreiben's job.
func jeArtefakt(root string, auswahl []ziel, fn func(artefakt) error) error {
	tmp, err := os.MkdirTemp("", "holzcloud-wasm-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	for _, z := range auswahl {
		artefakte, err := erzeugen(root, z, tmp)
		if err != nil {
			return err
		}
		for _, a := range artefakte {
			if err := fn(a); err != nil {
				return err
			}
		}
	}
	return nil
}

// erzeugen builds one target and returns what it produced.
//
// The build file is named after the target and not after the destination's
// basename: five of the six destinations are called plugin.wasm, so basenames
// would collide in a single directory — which is also what -out would do.
func erzeugen(root string, z ziel, bauplatz string) ([]artefakt, error) {
	datei := z.name + ".wasm"
	nach := filepath.Join(bauplatz, datei)
	if err := bauen(root, z, nach); err != nil {
		return nil, fmt.Errorf("%s: %w", z.name, err)
	}
	inhalt, err := os.ReadFile(nach)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", z.name, err)
	}
	artefakte := []artefakt{{label: z.name, ziel: z.out, datei: datei, quelle: z.name, inhalt: inhalt}}
	if z.archiv == "" {
		return artefakte, nil
	}

	manifest, err := os.ReadFile(filepath.Join(root, z.dir, manifestName))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", z.name, err)
	}
	archiv, err := packen(filepath.Join(root, z.dir), manifest, inhalt)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", z.name, err)
	}
	return append(artefakte, artefakt{
		label:  z.name + ".zip",
		ziel:   z.archiv,
		datei:  filepath.Base(z.archiv),
		quelle: z.name,
		inhalt: archiv,
	}), nil
}

// packen builds one plugin archive in memory. The module it packs is the one
// this run just built, never the file lying in the tree — that is what keeps
// the archive from ever describing a module other than the one beside it.
//
// Built in memory and handed back as one slice, the shape tools/mkbundle's
// packer uses and for the reason its comment gives: an archive interrupted
// half-way still looks like a file.
//
// Deflate, not Store: a wasm guest compresses to under 30 %, and 21 MB of
// uncompressed copies in git would cost more than the compression buys. The
// price is that the archive bytes depend on compress/flate — that is, on the
// toolchain running *this* tool, not on the pinned one that builds the guests.
// Go has not changed that output in years, and if a release ever does, the fix
// is the same as for a compiler bump: repack in the commit that raises the
// version.
func packen(quelle string, manifest, modul []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	eintraege := []struct {
		name   string
		inhalt []byte
	}{
		{manifestName, manifest},
		{modulName, modul},
	}
	// Everything else the format admits, read from the plugin's own directory.
	// Writing a fixed two-entry archive was the earlier shape, and it was wrong
	// in the quiet way: a plugin that grew an assets/ or migrations/ directory
	// got an archive missing it, -check called that archive current, and from
	// the next commit the blocking gate enforced the loss. An operator would
	// have installed a plugin whose stylesheet 404s or whose table was never
	// created, with correct-looking source lying beside it.
	for _, verzeichnis := range []string{assetDir, migrationDir} {
		gefunden, err := sammeln(quelle, verzeichnis)
		if err != nil {
			return nil, err
		}
		eintraege = append(eintraege, gefunden...)
	}
	for _, e := range eintraege {
		kopf := &zip.FileHeader{Name: e.name, Method: zip.Deflate, Modified: archivZeit}
		kopf.SetMode(0o644)
		w, err := zw.CreateHeader(kopf)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(e.inhalt); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// inDenBaum replaces the committed files with what was just built.
func inDenBaum(root string, auswahl []ziel) error {
	return jeArtefakt(root, auswahl, func(a artefakt) error {
		pfad := filepath.Join(root, a.ziel)
		if err := atomarSchreiben(pfad, a.inhalt); err != nil {
			return err
		}
		fmt.Printf("%-19s -> %-40s %5.1f MiB\n", a.label, a.ziel, float64(len(a.inhalt))/(1<<20))
		return nil
	})
}

// nachVerzeichnis builds into a directory of the caller's choosing and touches
// nothing in the repository. This is the documented answer if the byte equality
// between a contributor's machine and the runner ever breaks: CI builds into
// its own directory and the tests read from there.
func nachVerzeichnis(root string, auswahl []ziel, verzeichnis string) error {
	if err := os.MkdirAll(verzeichnis, 0o755); err != nil {
		return err
	}
	return jeArtefakt(root, auswahl, func(a artefakt) error {
		pfad := filepath.Join(verzeichnis, a.datei)
		if err := atomarSchreiben(pfad, a.inhalt); err != nil {
			return err
		}
		fmt.Printf("%-19s -> %-40s %5.1f MiB\n", a.label, pfad, float64(len(a.inhalt))/(1<<20))
		return nil
	})
}

// hashesDrucken reports what a build would produce, without comparing it to
// anything and without writing into the working tree — a mode that modified the
// artifacts it describes could not be run to answer a question.
func hashesDrucken(root string, auswahl []ziel) error {
	return jeArtefakt(root, auswahl, func(a artefakt) error {
		summe, groesse, version := beschreiben(a.inhalt)
		fmt.Printf("%-19s %s %9d Bytes  gebaut mit %s\n", a.label, summe, groesse, version)
		return nil
	})
}

// vergleichen compares a fresh build against the committed file and reports
// every difference before returning how many it found. A missing file is a
// difference, not a crash: the run keeps going and the count carries it.
func vergleichen(root string, auswahl []ziel) (int, error) {
	abweichungen := 0
	err := jeArtefakt(root, auswahl, func(a artefakt) error {
		neuSumme, neuGroesse, neuVersion := beschreiben(a.inhalt)
		alt, err := os.ReadFile(filepath.Join(root, a.ziel))
		if errors.Is(err, os.ErrNotExist) {
			abweichungen++
			fmt.Printf("%s fehlt\n", a.ziel)
			fmt.Printf("  im Repository: %s\n", "keine Datei")
			fmt.Printf("  neu gebaut:    %s %9d Bytes  gebaut mit %s\n", neuSumme, neuGroesse, neuVersion)
			fmt.Printf("  Neu bauen mit: go run ./tools/wasm %s\n\n", a.quelle)
			return nil
		}
		if err != nil {
			return err
		}
		altSumme, altGroesse, altVersion := beschreiben(alt)
		if altSumme == neuSumme {
			fmt.Printf("%-19s aktuell\n", a.label)
			return nil
		}
		abweichungen++
		fmt.Printf("%s ist nicht aktuell\n", a.ziel)
		fmt.Printf("  im Repository: %s %9d Bytes  gebaut mit %s\n", altSumme, altGroesse, altVersion)
		fmt.Printf("  neu gebaut:    %s %9d Bytes  gebaut mit %s\n", neuSumme, neuGroesse, neuVersion)
		fmt.Printf("  Neu bauen mit: go run ./tools/wasm %s\n\n", a.quelle)
		return nil
	})
	return abweichungen, err
}

// atomarSchreiben writes through a temporary file in the destination's own
// directory and then renames. Same directory for two reasons: a rename across
// filesystems fails, and a half-written file that already carries the
// destination's name is indistinguishable from a finished one.
func atomarSchreiben(pfad string, inhalt []byte) error {
	f, err := os.CreateTemp(filepath.Dir(pfad), "."+filepath.Base(pfad)+".tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name) // a no-op once the rename below succeeded
	if _, err := f.Write(inhalt); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return err
	}
	return os.Rename(name, pfad)
}

// bauen runs one go build in the target's own module. nach is absolute, or
// relative to the module directory — go writes -o relative to its working
// directory, not to ours.
func bauen(root string, z ziel, nach string) error {
	cmd := exec.Command("go", "build",
		"-buildmode=c-shared",
		"-trimpath",
		"-buildvcs=false",
		// One exec argument, not two, and without quotes: the shell quoting in
		// the READMEs does not survive os/exec, and literal quotes would reach
		// the linker.
		"-ldflags=-s -w",
		"-o", nach, ".")
	cmd.Dir = filepath.Join(root, z.dir)

	env := make([]string, 0, len(os.Environ())+len(gesteuert))
	for _, e := range os.Environ() {
		if k, _, ok := strings.Cut(e, "="); ok && slices.Contains(gesteuert, k) {
			continue
		}
		env = append(env, e)
	}
	cmd.Env = append(env,
		"GOOS=wasip1",
		"GOARCH=wasm",
		"CGO_ENABLED=0",
		"GOTOOLCHAIN="+goToolchain,
		// GOENV=off first, because everything below it depends on it. Setting a
		// variable to the empty string does NOT neutralise it: cmd/go falls back
		// to the go/env file whenever the process value is empty, so a
		// `go env -w GOFLAGS=-tags=zzz` run once, months ago, would still reach
		// this build and change its bytes. Only switching the env file off makes
		// the emptying below mean what it says.
		"GOENV=off",
		// Emptied rather than inherited: a GOFLAGS or GOEXPERIMENT in a
		// contributor's shell changes the produced bytes, and a comparison that
		// depends on somebody's dotfiles compares nothing.
		"GOFLAGS=",
		"GOEXPERIMENT=",
		"GOWASM=",
		"GOFIPS140=",
	)
	// A compile error is the most useful thing this tool ever prints. Pass it
	// straight through instead of swallowing it into an error string.
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// versionMuster finds the compiler version the go command writes into every
// built module. A byte scan, not a wasm parser: the version is the only thing
// wanted from the file, and a parser is a dependency and a second thing to keep
// working.
var versionMuster = regexp.MustCompile(`go1\.[0-9]+(?:\.[0-9]+)?`)

// beschreiben reports the three fields a mismatch needs to be diagnosed from.
// The hash follows the shape of hashFile in tools/mkbundle — crypto/sha256 into
// encoding/hex — kept here rather than imported, because that is a different
// package.
func beschreiben(inhalt []byte) (summe string, groesse int64, version string) {
	h := sha256.Sum256(inhalt)
	version = "unbekannt"
	if m := versionMuster.Find(inhalt); m != nil {
		version = string(m)
	}
	return hex.EncodeToString(h[:]), int64(len(inhalt)), version
}

func usage(grund string) {
	fmt.Fprintln(os.Stderr, "usage: wasm [-check | -print-hashes | -out <dir>] [target...]")
	fmt.Fprintln(os.Stderr, "  (no flag)      build the selected modules into the working tree")
	fmt.Fprintln(os.Stderr, "  -check         compare against the committed files, exit 1 on a mismatch")
	fmt.Fprintln(os.Stderr, "  -print-hashes  print hash, size and compiler, write nothing")
	fmt.Fprintln(os.Stderr, "  -out <dir>     build into <dir>, touch nothing in the tree")
	fmt.Fprintln(os.Stderr, "  targets: bestellung jahreszahl kontaktformular nicht-gefunden suche echo")
	fmt.Fprintln(os.Stderr, "  (no target means all six)")
	fmt.Fprintln(os.Stderr, "  "+grund)
	os.Exit(2)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
