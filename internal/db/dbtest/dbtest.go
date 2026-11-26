// Package dbtest hands a test a migrated database without running the
// migrations again.
//
// Every test that needed a database used to open a file and run the whole
// embedded migration chain against it: half a second on a plain run and
// several seconds under the race detector, paid once per test. Here the chain
// runs once per test binary, the result is snapshotted into one consistent
// file, and each test gets a byte copy of that snapshot in its own temporary
// directory. The copies are independent files; nothing is shared between two
// tests but the bytes they started from.
//
// The package may be imported from _test.go files only. The production binary
// must never link it: `go list -deps ./cmd/holzcloud` must not name it.
//
// A test whose subject is the migrations themselves (the tests of
// internal/db, the schema guard test of cmd/holzcloud) must keep migrating for
// real: a copy of a finished schema cannot show that the chain reaches it.
package dbtest

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

var (
	buildOnce sync.Once
	template  []byte
	buildErr  error
)

// Open returns a migrated database in a file of its own under tb.TempDir().
// The database is closed when the test ends.
func Open(tb testing.TB) *db.DB {
	tb.Helper()
	return OpenAt(tb, filepath.Join(tb.TempDir(), "test.sqlite"))
}

// OpenAt returns a migrated database at path. The path must not exist yet: a
// copy never overwrites a file, because a test that handed over a path with a
// database already in it would silently lose that database. The database is
// closed when the test ends.
func OpenAt(tb testing.TB, path string) *db.DB {
	tb.Helper()

	buildOnce.Do(func() { template, buildErr = build() })
	if buildErr != nil {
		tb.Fatalf("dbtest: building the migrated template: %v", buildErr)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		tb.Fatalf("dbtest: creating %s: %v", path, err)
	}
	if _, err := f.Write(template); err != nil {
		f.Close()
		tb.Fatalf("dbtest: writing %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		tb.Fatalf("dbtest: closing %s: %v", path, err)
	}

	database, err := db.Open(path)
	if err != nil {
		tb.Fatalf("dbtest: db.Open: %v", err)
	}
	tb.Cleanup(database.Close)
	return database
}

// build migrates a scratch database and returns a snapshot of it. VACUUM INTO
// writes one finished file, so nothing of the result is left pending in a
// write-ahead log that a byte copy would miss.
func build() ([]byte, error) {
	dir, err := os.MkdirTemp("", "dbtest-template-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	database, err := db.Open(filepath.Join(dir, "migrated.sqlite"))
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database.Write); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	snapshot := filepath.Join(dir, "snapshot.sqlite")
	if err := vacuumInto(database.Write, snapshot); err != nil {
		return nil, fmt.Errorf("snapshot: %w", err)
	}
	return os.ReadFile(snapshot)
}

func vacuumInto(w *sql.DB, path string) error {
	_, err := w.Exec(`VACUUM INTO ?`, path)
	return err
}
