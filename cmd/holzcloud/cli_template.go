package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
	flags := flag.NewFlagSet("template check", flag.ContinueOnError)
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

	var problems []string
	switch {
	case info.IsDir():
		problems = tmplmgr.CheckTemplateDir(target, defaultThemeFS())
	case strings.EqualFold(filepath.Ext(target), ".zip"):
		problems, err = checkArchive(target)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s is neither a directory nor a .zip archive", target)
	}

	if len(problems) == 0 {
		if !*quiet {
			fmt.Printf("%s: no problems found\n", target)
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "%s: %d problem(s)\n\n", target, len(problems))
	for _, p := range problems {
		fmt.Fprintf(os.Stderr, "  %s\n\n", strings.ReplaceAll(p, "; ", "\n  "))
	}
	fmt.Fprintf(os.Stderr, "The specification is at `holzcloud template spec`.\n")

	// A non-zero status so this composes with a script or an agent's own loop.
	return errSilent
}

// checkArchive unpacks into a temporary directory and checks that, so an
// archive is judged exactly as the upload would judge it — the extraction
// limits included.
func checkArchive(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "holzcloud-check-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	// ExtractTemplate applies every check itself and reports the first thing
	// that makes the archive unacceptable.
	dest := filepath.Join(tempDir, "theme")
	if err := tmplmgr.ExtractTemplate(f, info.Size(), dest, maxCheckSize, defaultThemeFS()); err != nil {
		return []string{err.Error()}, nil
	}
	return nil, nil
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
