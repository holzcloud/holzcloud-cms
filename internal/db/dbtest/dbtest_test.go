package dbtest_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/db/dbtest"
)

func maxVersion(t *testing.T, database *db.DB) int64 {
	t.Helper()
	var v int64
	if err := database.Read.QueryRow(`SELECT MAX(version_id) FROM goose_db_version`).Scan(&v); err != nil {
		t.Fatalf("reading the goose version: %v", err)
	}
	return v
}

func TestOpenGivesEachTestItsOwnMigratedDatabase(t *testing.T) {
	first := dbtest.Open(t)
	second := dbtest.Open(t)

	if _, err := first.Write.ExecContext(context.Background(),
		`INSERT INTO websites (name) VALUES ('nur im ersten')`); err != nil {
		t.Fatalf("writing into the first copy: %v", err)
	}
	var n int
	if err := first.Read.QueryRow(`SELECT COUNT(*) FROM websites`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("first copy: %d websites, err %v, want 1", n, err)
	}
	if err := second.Read.QueryRow(`SELECT COUNT(*) FROM websites`).Scan(&n); err != nil {
		t.Fatalf("counting in the second copy: %v", err)
	}
	if n != 0 {
		t.Errorf("a row written in one copy is visible in another: %d websites", n)
	}

	// The copy sits at the level a real run of the migrations reaches.
	real, err := db.Open(filepath.Join(t.TempDir(), "real.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer real.Close()
	if err := db.RunMigrations(real.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	if got, want := maxVersion(t, first), maxVersion(t, real); got != want {
		t.Errorf("copy is at version %d, a real run reaches %d", got, want)
	}

	var mode string
	if err := first.Write.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
		t.Errorf("journal_mode = %q (%v), want wal", mode, err)
	}
	var fk int
	if err := first.Write.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Errorf("foreign_keys = %d (%v), want 1", fk, err)
	}
}

// failed is a testing.TB that records Fatalf and stops the goroutine the way
// the real one does, by panicking with a value the test recovers.
type failed struct {
	testing.TB
	message string
}

type stop struct{}

func (f *failed) Helper() {}
func (f *failed) Fatalf(format string, args ...any) {
	f.message = format
	panic(stop{})
}

func TestOpenAtRefusesAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "da.sqlite")
	dbtest.OpenAt(t, path).Close()

	fake := &failed{TB: t}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(stop); !ok {
					panic(r)
				}
			}
		}()
		dbtest.OpenAt(fake, path)
	}()
	if fake.message == "" {
		t.Fatal("OpenAt on an existing path did not fail the test")
	}
}
