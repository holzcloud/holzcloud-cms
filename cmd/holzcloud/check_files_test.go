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
