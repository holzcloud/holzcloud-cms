// Command wasm builds the WebAssembly guest modules this repository keeps
// committed, and reports what came out.
//
//	go run ./tools/wasm -print-hashes             # build all six, print hash, size and compiler
//	go run ./tools/wasm -print-hashes jahreszahl  # build one of them
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
// -print-hashes writes nothing into the working tree. It builds into a
// temporary directory and prints one line per module: the hash, the size, and
// the Go version found in the built bytes. That last field earns its place
// because a mismatch has two very different causes — the pin moved, or the
// source changed — and the line should let a human tell them apart at a glance.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The compiler every guest is built with. It has a floor: the go command
// refuses to load a module declaring a higher minimum than the running
// toolchain, and internal/plugin/testdata/echo lives in the root module, so
// this must be greater than or equal to the `go` directive in ./go.mod
// (go 1.26.6 today).
//
// Bare, never "go1.26.6+auto" — the +auto form selects a newer toolchain when
// one is needed, which is the pin defeating itself.
const goToolchain = "go1.26.6"

// ziel is one committed guest module: where it is built and where the built
// file belongs in the repository.
type ziel struct {
	name string // what a positional argument selects it by
	dir  string // module directory, relative to the repository root
	out  string // committed artifact, relative to the repository root
}

var ziele = []ziel{
	{"bestellung", "plugins/bestellung", "plugins/bestellung/plugin.wasm"},
	{"jahreszahl", "plugins/jahreszahl", "plugins/jahreszahl/plugin.wasm"},
	{"kontaktformular", "plugins/kontaktformular", "plugins/kontaktformular/plugin.wasm"},
	{"nicht-gefunden", "plugins/nicht-gefunden", "plugins/nicht-gefunden/plugin.wasm"},
	{"suche", "plugins/suche", "plugins/suche/plugin.wasm"},
	{"echo", "internal/plugin/testdata/echo", "internal/plugin/testdata/echo.wasm"},
}

// The variables the build controls. They are stripped from the inherited
// environment and set once, instead of appended on top of it. os/exec does
// deduplicate and keeps the later value, but that is a property of the parent's
// exec package rather than of the child, and a Go program reading its own
// environment keeps the *first* mention of a key. Filtering removes the
// dependence on which of the two rules happens to apply.
var gesteuert = []string{"GOOS", "GOARCH", "CGO_ENABLED", "GOTOOLCHAIN", "GOFLAGS", "GOEXPERIMENT"}

func main() {
	root := flag.String("root", ".", "repository root")
	printHashes := flag.Bool("print-hashes", false, "build the selected modules and print hash, size and compiler")
	check := flag.Bool("check", false, "compare the built modules against the committed files (not implemented yet)")
	out := flag.String("out", "", "build the selected modules into this directory (not implemented yet)")
	flag.Parse()

	// Declared here so the flag set is complete from the first commit; plan
	// 06-05 fills both in. Refusing loudly beats accepting a flag and ignoring
	// it, which is how a CI step passes while checking nothing.
	if *check || *out != "" {
		usage("-check and -out are not implemented yet — plan 06-05 fills them in")
	}
	if !*printHashes {
		usage("nothing to do: pass -print-hashes")
	}

	auswahl, err := waehlen(flag.Args())
	if err != nil {
		usage(err.Error())
	}
	if err := hashesDrucken(*root, auswahl); err != nil {
		fail(err)
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

// hashesDrucken builds each selected target into a temporary directory and
// prints one line per module. The working tree is never written to: the point
// of the mode is to report what a build *would* produce, and a mode that
// modifies the artifacts it is describing cannot be run to answer a question.
func hashesDrucken(root string, auswahl []ziel) error {
	tmp, err := os.MkdirTemp("", "holzcloud-wasm-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	for _, z := range auswahl {
		nach := filepath.Join(tmp, z.name+".wasm")
		if err := bauen(root, z, nach); err != nil {
			return fmt.Errorf("%s: %w", z.name, err)
		}
		summe, groesse, version, err := pruefen(nach)
		if err != nil {
			return fmt.Errorf("%s: %w", z.name, err)
		}
		fmt.Printf("%-16s %s %9d Bytes  gebaut mit %s\n", z.name, summe, groesse, version)
	}
	return nil
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
		// Emptied rather than inherited: a GOFLAGS or GOEXPERIMENT in a
		// contributor's shell changes the produced bytes, and a comparison that
		// depends on somebody's dotfiles compares nothing.
		"GOFLAGS=",
		"GOEXPERIMENT=",
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

// pruefen reads a built module once and reports the three fields a mismatch
// needs to be diagnosed from. The hash follows the shape of hashFile in
// tools/mkbundle — crypto/sha256 into encoding/hex — kept here rather than
// imported, because that is a different package.
func pruefen(pfad string) (summe string, groesse int64, version string, err error) {
	b, err := os.ReadFile(pfad)
	if err != nil {
		return "", 0, "", err
	}
	h := sha256.Sum256(b)
	version = "unbekannt"
	if m := versionMuster.Find(b); m != nil {
		version = string(m)
	}
	return hex.EncodeToString(h[:]), int64(len(b)), version, nil
}

func usage(grund string) {
	fmt.Fprintln(os.Stderr, "usage: wasm -print-hashes [target...]")
	fmt.Fprintln(os.Stderr, "  targets: bestellung jahreszahl kontaktformular nicht-gefunden suche echo")
	fmt.Fprintln(os.Stderr, "  (no target means all six)")
	fmt.Fprintln(os.Stderr, "  "+grund)
	os.Exit(2)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
