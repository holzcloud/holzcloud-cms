package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
	"github.com/holzcloud/holzcloud-cms/internal/tmplspec"
)

// cmdTemplate dispatches `holzcloud template …`.
func cmdTemplate(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: holzcloud template check <dir|zip> | holzcloud template spec")
	}
	switch args[0] {
	case "check":
		return cmdTemplateCheck(args[1:])
	case "spec":
		return cmdTemplateSpec(args[1:])
	default:
		return fmt.Errorf("unknown template subcommand %q — try check or spec", args[0])
	}
}

// cmdTemplateSpec prints the authoring specification.
//
// It is printed rather than only shipped in the repository because the person
// writing a template usually has the binary and not the source tree — and
// because handing the whole contract to an agent should be one command, not a
// hunt through a website.
func cmdTemplateSpec(args []string) error {
	fs := flag.NewFlagSet("template spec", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, err := os.Stdout.WriteString(tmplspec.Markdown())
	return err
}

// cmdTemplateCheck runs the upload's checks without installing anything.
//
// The point is the feedback loop. Whoever writes a template — a person, or an
// agent that cannot see the admin UI — can find out what is wrong with it
// before it goes anywhere near a live site, and gets exactly the message the
// upload would have given.
func cmdTemplateCheck(args []string) error {
	return runTemplateCheck(args, os.Stdout, os.Stderr)
}

// runTemplateCheck is cmdTemplateCheck with its two streams passed in, so a
// test can read what an author would read.
func runTemplateCheck(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("template check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	quiet := flags.Bool("quiet", false, "print nothing when the template is fine")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: holzcloud template check <directory|archive.zip>")
	}
	target := flags.Arg(0)

	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", target, err)
	}

	// dir is what both checks read. An archive is unpacked once: extracting it
	// twice would mean judging two trees and reporting on whichever came
	// second.
	dir := target
	var problems []string
	switch {
	case info.IsDir():
		problems = tmplmgr.CheckTemplateDir(target, defaultThemeFS())
	case strings.EqualFold(filepath.Ext(target), ".zip"):
		unpacked, cleanup, err := unpackForCheck(target)
		if cleanup != nil {
			defer cleanup()
		}
		if err != nil {
			// The archive did not survive extraction. That IS the problem, and
			// there is no tree to say anything further about.
			problems = []string{err.Error()}
			break
		}
		dir = unpacked
	default:
		return fmt.Errorf("%s is neither a directory nor a .zip archive", target)
	}

	// The words the theme mints and does not translate. They are reported and
	// never refused — a half-translated theme works, showing the author's own
	// English where a translation is missing, and refusing the upload over it
	// would be worse than the thing being reported. So they are printed on
	// their own, above, whether or not anything else is wrong.
	notes := untranslatedWords(dir)

	// Which contract this check reads the theme against, and which one the
	// theme says it was written for. Stated, not judged: a theme declaring
	// nothing is fine, a different version is a note beside the others, and
	// what a difference means is not decided here.
	line, versionNote := contractLine(target, dir)
	if versionNote != "" {
		notes = append(notes, versionNote)
	}
	for _, n := range notes {
		fmt.Fprintf(stderr, "  note: %s\n\n", strings.ReplaceAll(n, "\n    ", "\n        "))
	}
	if !*quiet {
		fmt.Fprintln(stdout, line)
	}

	if len(problems) == 0 {
		if !*quiet {
			if len(notes) > 0 {
				fmt.Fprintf(stdout, "%s: nothing that stops it working, %d note(s) above\n", target, len(notes))
			} else {
				fmt.Fprintf(stdout, "%s: no problems found\n", target)
			}
		}
		// A note is not a failure. An author converting a theme one language at
		// a time has to be able to run this and get on with it.
		return nil
	}

	fmt.Fprintf(stderr, "%s: %d problem(s)\n\n", target, len(problems))
	for _, p := range problems {
		fmt.Fprintf(stderr, "  %s\n\n", strings.ReplaceAll(p, "; ", "\n  "))
	}
	fmt.Fprintf(stderr, "The specification is at `holzcloud template spec`.\n")

	// A non-zero status so this composes with a script or an agent's own loop.
	return errSilent
}

// checkArchive unpacks into a temporary directory and checks that, so an
// archive is judged exactly as the upload would judge it — the extraction
// limits included.
func unpackForCheck(path string) (dir string, cleanup func(), err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", nil, err
	}

	tempDir, err := os.MkdirTemp("", "holzcloud-check-*")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { os.RemoveAll(tempDir) }

	// ExtractTemplate applies every check itself and reports the first thing
	// that makes the archive unacceptable.
	dest := filepath.Join(tempDir, "theme")
	if err := tmplmgr.ExtractTemplate(f, info.Size(), dest, maxCheckSize, defaultThemeFS()); err != nil {
		return "", cleanup, err
	}
	return dest, cleanup, nil
}

// maxCheckSize is the uncompressed budget the check applies.
//
// It matches the server's default rather than the configured value: the point
// of checking locally is to find out whether an archive will be accepted
// somewhere else, and that somewhere else is usually a default install.
const maxCheckSize = 10 << 20

// defaultThemeFS is the built-in default theme, whose views fill in for the
// ones an archive leaves out. They are still rendered through the uploaded
// layout, so the check needs them to judge that combination.
func defaultThemeFS() fs.FS {
	sub, err := fs.Sub(staticFS, "templates/public/default")
	if err != nil {
		return nil
	}
	return sub
}

// errSilent ends the process with a non-zero status without printing again;
// everything worth saying has already gone to stderr.
var errSilent = errors.New("")

// untranslatedWords reports the theme's own words that have no translation.
//
// It reads the directory, or the archive unpacked into one, so an author gets
// the same answer either way — the same reason checkArchive exists.
func untranslatedWords(dir string) []string {
	var out []string
	for _, p := range tmpl.CheckCatalogs(os.DirFS(dir)) {
		out = append(out, p.String())
	}
	return out
}

// contractLine is the line naming the contract the check reads against and
// what the theme in dir declares. target is the name the author typed, so the
// line speaks of the archive rather than of the directory it was unpacked to.
//
// A theme.json that cannot be read leaves the declared half out; the checks
// report that file as a problem of its own.
//
// note is set when the theme declares a version other than the current one. It
// says that the two differ and nothing about what that means.
func contractLine(target, dir string) (line, note string) {
	version, declared, err := tmpl.DeclaredContract(os.DirFS(dir))
	switch {
	case err != nil:
		return "template contract " + tmpl.ContractVersion, ""
	case !declared:
		return fmt.Sprintf("template contract %s; %s declares no contract version (%s)",
			tmpl.ContractVersion, target, tmpl.ManifestName), ""
	}
	line = fmt.Sprintf("template contract %s; %s declares %s", tmpl.ContractVersion, target, version)
	if version != tmpl.ContractVersion {
		note = fmt.Sprintf("%s was written for template contract %s; this check reads template contract %s",
			target, version, tmpl.ContractVersion)
	}
	return line, note
}
