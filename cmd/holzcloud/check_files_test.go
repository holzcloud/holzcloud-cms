package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// The lines `holzcloud check -files DIR` prints are a contract: restore.sh
// requires the "files:" marker, because an older binary ignores the flag, and
// stops on every line that starts with "MISSING ". These tests pin them byte
// for byte.

// checkFilesCase is a data directory with a migrated database holding one
// website, and an empty backup directory. It returns the database (still
// open, for seeding), the website and the backup directory.
func checkFilesCase(t *testing.T) (*db.DB, int64, string) {
	t.Helper()
	dataDir := t.TempDir()
	t.Setenv("HOLZCLOUD_DATA_DIR", dataDir)
	database, err := db.Open(filepath.Join(dataDir, "holzcloud.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	res, err := database.Write.Exec(`INSERT INTO websites (name, description) VALUES ('Test', '')`)
	if err != nil {
		t.Fatalf("insert website: %v", err)
	}
	ws, _ := res.LastInsertId()
	return database, ws, t.TempDir()
}

// putBackupFile writes a file where the backup layout keeps it.
func putBackupFile(t *testing.T, backup string, websiteID int64, filename string) {
	t.Helper()
	p := media.Path(backup, websiteID, filename)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func runCheckLines(t *testing.T, files string) ([]string, error) {
	t.Helper()
	var out bytes.Buffer
	err := runCheck(&out, files)
	return strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n"), err
}

func sameLines(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestCheckFilesPrintsTheMarkerAndEveryMissingFile(t *testing.T) {
	t.Run("a missing original", func(t *testing.T) {
		database, ws, backup := checkFilesCase(t)
		store := media.NewStore(database)
		ctx := context.Background()
		if _, err := store.Create(ctx, ws, "a.jpg", "Urlaub.jpg", "image/jpeg", 1, "hash-a"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		gone, err := store.Create(ctx, ws, "b.jpg", "Hochzeit.jpg", "image/jpeg", 1, "hash-b")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		putBackupFile(t, backup, ws, "a.jpg")

		lines, err := runCheckLines(t, backup)
		if exitStatus(err) != 3 {
			t.Errorf("exit status %d (%v); want 3", exitStatus(err), err)
		}
		sameLines(t, lines, []string{
			"integrity: ok",
			"files: 2 rows checked, 1 missing",
			fmt.Sprintf(`MISSING media/%d/b.jpg (media row %d, website %d, original name "Hochzeit.jpg")`, ws, gone.ID, ws),
		})
	})

	t.Run("a missing variant after the originals", func(t *testing.T) {
		database, ws, backup := checkFilesCase(t)
		store := media.NewStore(database)
		ctx := context.Background()
		a, err := store.Create(ctx, ws, "a.jpg", "Urlaub.jpg", "image/jpeg", 1, "hash-a")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		gone, err := store.Create(ctx, ws, "b.jpg", "Hochzeit.jpg", "image/jpeg", 1, "hash-b")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		res, err := database.Write.Exec(`INSERT INTO media_variants (media_id, label, filename, width, height, size_bytes)
			VALUES ($1, 'medium', 'a-medium.jpg', 1, 1, 1)`, a.ID)
		if err != nil {
			t.Fatalf("insert variant: %v", err)
		}
		variant, _ := res.LastInsertId()
		putBackupFile(t, backup, ws, "a.jpg")

		lines, err := runCheckLines(t, backup)
		if exitStatus(err) != 3 {
			t.Errorf("exit status %d (%v); want 3", exitStatus(err), err)
		}
		sameLines(t, lines, []string{
			"integrity: ok",
			"files: 3 rows checked, 2 missing",
			fmt.Sprintf(`MISSING media/%d/b.jpg (media row %d, website %d, original name "Hochzeit.jpg")`, ws, gone.ID, ws),
			fmt.Sprintf(`MISSING media/%d/a-medium.jpg (media_variants row %d, label "medium", of media row %d)`, ws, variant, a.ID),
			"a missing variant can be rebuilt with: holzcloud thumbnails",
		})
	})

	t.Run("a website without media", func(t *testing.T) {
		_, _, backup := checkFilesCase(t)
		lines, err := runCheckLines(t, backup)
		if err != nil {
			t.Errorf("err = %v; want nil", err)
		}
		sameLines(t, lines, []string{
			"integrity: ok",
			"files: 0 rows checked, 0 missing",
		})
	})
}

func TestCheckFilesEndsWithStatusThreeWhenAFileIsMissing(t *testing.T) {
	database, ws, backup := checkFilesCase(t)
	if _, err := media.NewStore(database).Create(context.Background(), ws, "a.jpg", "a.jpg", "image/jpeg", 1, "hash-a"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err := runCheckLines(t, backup)
	if err == nil {
		t.Fatal("a missing file ended without an error")
	}
	if err.Error() != "" {
		t.Errorf("the report is on stdout, yet the error says %q", err.Error())
	}
	if got := exitStatus(err); got != 3 {
		t.Errorf("exitStatus = %d; want 3", got)
	}

	putBackupFile(t, backup, ws, "a.jpg")
	if _, err := runCheckLines(t, backup); err != nil {
		t.Errorf("nothing missing, yet err = %v", err)
	}

	if got := exitStatus(errors.New("x")); got != 1 {
		t.Errorf("exitStatus(plain error) = %d; want 1", got)
	}
	if got := exitStatus(nil); got != 0 {
		t.Errorf("exitStatus(nil) = %d; want 0", got)
	}
	if got := exitStatus(fmt.Errorf("wrapped: %w", &exitError{code: 3})); got != 3 {
		t.Errorf("exitStatus(wrapped) = %d; want 3", got)
	}
}

// restore.sh decides on the line that starts with "files: ". A name stored in
// the database — an upload name, a variant label, a crafted file name — must
// not be able to put a second such line into the report.
func TestCheckFilesQuotesANameSoItCannotStartALine(t *testing.T) {
	database, ws, backup := checkFilesCase(t)
	const fake = "files: 0 rows checked, 0 missing"
	store := media.NewStore(database)
	ctx := context.Background()
	orig, err := store.Create(ctx, ws, "a.jpg", "x\n"+fake, "image/jpeg", 1, "hash-a")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	res, err := database.Write.Exec(`INSERT INTO media_variants (media_id, label, filename, width, height, size_bytes)
		VALUES ($1, $2, 'a-thumb.jpg', 1, 1, 1)`, orig.ID, "t\n"+fake)
	if err != nil {
		t.Fatalf("insert variant: %v", err)
	}
	variant, _ := res.LastInsertId()
	res, err = database.Write.Exec(`INSERT INTO media (website_id, filename, original_name, mime_type, size_bytes, content_hash)
		VALUES ($1, $2, 'y', 'image/jpeg', 1, 'hash-c')`, ws, "c\n"+fake)
	if err != nil {
		t.Fatalf("insert media: %v", err)
	}
	crafted, _ := res.LastInsertId()

	lines, err := runCheckLines(t, backup)
	if exitStatus(err) != 3 {
		t.Errorf("exit status %d (%v); want 3", exitStatus(err), err)
	}
	marker := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "files: ") {
			marker++
		}
	}
	if marker != 1 {
		t.Errorf("%d lines start with %q; want 1:\n%s", marker, "files: ", strings.Join(lines, "\n"))
	}
	sameLines(t, lines, []string{
		"integrity: ok",
		"files: 3 rows checked, 3 missing",
		fmt.Sprintf(`MISSING media/%d/a.jpg (media row %d, website %d, original name "x\nfiles: 0 rows checked, 0 missing")`, ws, orig.ID, ws),
		fmt.Sprintf(`MISSING (media row %d, website %d): file name "c\nfiles: 0 rows checked, 0 missing" is not a plain file name`, crafted, ws),
		fmt.Sprintf(`MISSING media/%d/a-thumb.jpg (media_variants row %d, label "t\nfiles: 0 rows checked, 0 missing", of media row %d)`, ws, variant, orig.ID),
		"a missing variant can be rebuilt with: holzcloud thumbnails",
	})
}

func TestCheckFilesRefusesADirectoryThatDoesNotExist(t *testing.T) {
	checkFilesCase(t)
	nowhere := filepath.Join(t.TempDir(), "nowhere")
	aFile := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(aFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, dir := range []string{nowhere, aFile} {
		lines, err := runCheckLines(t, dir)
		if err == nil {
			t.Errorf("-files %s: no error", dir)
			continue
		}
		if want := "check -files: " + dir + " is not a directory"; err.Error() != want {
			t.Errorf("error %q; want %q", err.Error(), want)
		}
		if got := exitStatus(err); got != 1 {
			t.Errorf("exit status %d; want 1", got)
		}
		for _, l := range lines {
			if strings.HasPrefix(l, "files: ") {
				t.Errorf("a marker was printed for a directory that is not there: %q", l)
			}
		}
	}
	err := cmdCheck([]string{"-files", ""})
	if exitStatus(err) != 1 {
		t.Errorf("-files with an empty value: %v; want an error with status 1", err)
	}
	// An empty value names no directory, so the message names none either:
	// not "check -files:  is not a directory" (code review 32, IN-03).
	if err != nil {
		if want := "check -files: needs a directory"; err.Error() != want {
			t.Errorf("-files with an empty value: error %q; want %q", err.Error(), want)
		}
	}
}

func TestCheckFilesRejectsAPositionalArgumentAndAnUnknownFlag(t *testing.T) {
	checkFilesCase(t)
	for _, args := range [][]string{{"somewhere"}, {"-nope"}, {"-files", t.TempDir(), "extra"}} {
		err := cmdCheck(args)
		if err == nil {
			t.Errorf("check %q: no error", args)
			continue
		}
		if got := exitStatus(err); got != 1 {
			t.Errorf("check %q: exit status %d; want 1", args, got)
		}
	}
}

func TestCheckWithoutFilesBehavesAsBefore(t *testing.T) {
	database, ws, _ := checkFilesCase(t)
	// A file the data directory lacks is not looked for without -files.
	if _, err := media.NewStore(database).Create(context.Background(), ws, "a.jpg", "a.jpg", "image/jpeg", 1, "hash-a"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	lines, err := runCheckLines(t, "")
	if err != nil {
		t.Errorf("err = %v; want nil", err)
	}
	sameLines(t, lines, []string{"integrity: ok"})
	if err := cmdCheck(nil); err != nil {
		t.Errorf("cmdCheck(nil) = %v; want nil", err)
	}
}
