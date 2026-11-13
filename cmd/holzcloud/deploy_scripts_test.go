package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// CI ran no shell test before this file. deploy/test/backup-robust.sh and
// deploy/test/restore-robust.sh are the only proof of what backup.sh and
// restore.sh do when a night or a restore goes wrong: a vanished file, a night
// that removes its own directory, a retention step that cannot remove, a
// damaged backup, a row whose file the backup lacks. They need nothing but
// bash and run in seconds, so `go test` runs them here. When they run as root
// (some containers), backup-robust.sh switches its retention case to a fake rm
// by itself, because root removes what a read-only directory protects.
//
// restore.sh also parses the binary's output: it believes "nothing missing"
// only after the marker line of `holzcloud check -files`, and it collects the
// lines that start with "MISSING ". The script and the binary are written on
// two sides of one contract, so TestRestoreReadsWhatCheckFilesPrints holds them
// together: the expression is read out of restore.sh and matched against what
// runCheck prints.

// restoreScript is the restore.sh the contract test reads.
var restoreScript = filepath.Join("..", "..", "deploy", "restore.sh")

// runHermetic runs one of the hermetic script tests and requires status 0 and
// a last non-empty line that matches ok.
func runHermetic(t *testing.T, script string, ok *regexp.Regexp) {
	t.Helper()
	if testing.Short() {
		t.Skip("the hermetic script tests do not run with -short")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bash, script)
	out, runErr := cmd.CombinedOutput()

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	last := ""
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			last = lines[i]
			break
		}
	}
	if runErr == nil && ok.MatchString(last) {
		return
	}
	tail := lines
	if len(tail) > 40 {
		tail = tail[len(tail)-40:]
	}
	t.Errorf("bash %s: %v; last line %q, want one matching %s\nlast %d lines:\n%s",
		script, runErr, last, ok, len(tail), strings.Join(tail, "\n"))
}

func TestTheHermeticBackupTestPasses(t *testing.T) {
	runHermetic(t, filepath.Join("..", "..", "deploy", "test", "backup-robust.sh"),
		regexp.MustCompile(`^BACKUP-ROBUST OK \(\d+ cases\)$`))
}

func TestTheHermeticRestoreTestPasses(t *testing.T) {
	runHermetic(t, filepath.Join("..", "..", "deploy", "test", "restore-robust.sh"),
		regexp.MustCompile(`^RESTORE-ROBUST OK \(\d+ cases\)$`))
}

// TestRestoreReadsWhatCheckFilesPrints: a backup that lacks a media file and a
// variant, checked by the real runCheck, must give exactly the lines restore.sh
// acts on. A marker that drifted on either side would let restore.sh take the
// binary for one older than 0.1.5 and report nothing (threat T-32-06).
func TestRestoreReadsWhatCheckFilesPrints(t *testing.T) {
	const wantMarker = `^files: [0-9]+ rows checked, [0-9]+ missing$`

	b, err := os.ReadFile(restoreScript)
	if err != nil {
		t.Fatalf("read %s: %v", restoreScript, err)
	}
	script := string(b)

	// The expression restore.sh greps the marker with, as the script spells it.
	found := regexp.MustCompile(`grep -E '(\^files: [^']*)'`).FindStringSubmatch(script)
	if found == nil {
		t.Fatalf("%s has no `grep -E '^files: …'` for the marker line", restoreScript)
	}
	if found[1] != wantMarker {
		t.Errorf("%s greps the marker with %q; want %q", restoreScript, found[1], wantMarker)
	}
	for _, want := range []string{`'^MISSING '`, "exit 3"} {
		if !strings.Contains(script, want) {
			t.Errorf("%s does not contain %s", restoreScript, want)
		}
	}
	// POSIX ERE and Go's syntax agree on everything this expression uses.
	marker, err := regexp.Compile(found[1])
	if err != nil {
		t.Fatalf("compile %q: %v", found[1], err)
	}

	database, ws, backup := checkFilesCase(t)
	store := media.NewStore(database)
	m, err := store.Create(context.Background(), ws, "b.jpg", "Hochzeit.jpg", "image/jpeg", 1, "hash-b")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := database.Write.Exec(`INSERT INTO media_variants (media_id, label, filename, width, height, size_bytes)
		VALUES ($1, 'medium', 'b-medium.jpg', 1, 1, 1)`, m.ID); err != nil {
		t.Fatalf("insert variant: %v", err)
	}

	var out bytes.Buffer
	runErr := runCheck(&out, backup, true)
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")

	markers, missing := 0, 0
	for _, line := range lines {
		switch {
		case marker.MatchString(line):
			markers++
		case strings.HasPrefix(line, "integrity: "):
		case strings.HasPrefix(line, "MISSING "):
			missing++
		case line == "a missing variant can be rebuilt with: holzcloud thumbnails":
		default:
			t.Errorf("a line restore.sh does not expect: %q", line)
		}
	}
	if markers != 1 {
		t.Errorf("%d lines match the marker restore.sh greps for; want 1\noutput:\n%s", markers, out.String())
	}
	if missing != 2 {
		t.Errorf("%d MISSING lines; want 2 (the original and its variant)\noutput:\n%s", missing, out.String())
	}
	if got := exitStatus(runErr); got != 3 {
		t.Errorf("exit status %d (%v); want 3, which restore.sh reads as restored with files missing", got, runErr)
	}
}
