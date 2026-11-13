package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"
)

// highestEmbedded reads the highest migration level this binary carries from
// the embedded file names, so no test here names a level as a number and a
// migration added later changes nothing in this file.
func highestEmbedded(t *testing.T) int64 {
	t.Helper()
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	var highest int64
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			continue
		}
		v, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			t.Fatalf("migration %s: %v", name, err)
		}
		if v > highest {
			highest = v
		}
	}
	if highest == 0 {
		t.Fatal("no embedded migrations found")
	}
	return highest
}

const newerMigration = `-- +goose Up
CREATE TABLE newer_binary_table (id INTEGER PRIMARY KEY) STRICT;

-- +goose Down
DROP TABLE newer_binary_table;
`

// applyNewer applies the given migrations through a real goose provider, the
// way a newer holzcloud would, and returns the error of Up. No version row is
// written by hand.
func applyNewer(t *testing.T, conn *sql.DB, files fstest.MapFS) error {
	t.Helper()
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, files)
	if err != nil {
		t.Fatalf("newer provider: %v", err)
	}
	_, err = provider.Up(context.Background())
	return err
}

// newerDatabase returns a database a newer binary has migrated one level
// beyond what this binary embeds.
func newerDatabase(t *testing.T) (*sql.DB, int64) {
	t.Helper()
	highest := highestEmbedded(t)
	conn := atVersion(t, highest)
	files := fstest.MapFS{
		fmt.Sprintf("%05d_newer.sql", highest+1): {Data: []byte(newerMigration)},
	}
	if err := applyNewer(t, conn, files); err != nil {
		t.Fatalf("apply newer migration: %v", err)
	}
	if v, err := CurrentVersion(conn); err != nil || v != highest+1 {
		t.Fatalf("newer database: level %d, err %v; want %d", v, err, highest+1)
	}
	return conn, highest
}

func TestTheGuardLetsADatabaseAtItsOwnLevelPass(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "own.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	if err := RefuseNewerSchema(context.Background(), database.Write); err != nil {
		t.Fatalf("a database at the binary's own level was refused: %v", err)
	}
}

func TestTheGuardLetsAnOlderDatabasePassAndItMigrates(t *testing.T) {
	highest := highestEmbedded(t)
	conn := atVersion(t, highest-1)
	if err := RefuseNewerSchema(context.Background(), conn); err != nil {
		t.Fatalf("an older database was refused: %v", err)
	}
	if err := RunMigrations(conn); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	v, err := CurrentVersion(conn)
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if v != highest {
		t.Fatalf("after migrating: level %d, want %d", v, highest)
	}
}

func TestTheGuardRefusesADatabaseANewerBinaryMigrated(t *testing.T) {
	conn, highest := newerDatabase(t)

	err := RefuseNewerSchema(context.Background(), conn)
	if err == nil {
		t.Fatal("a database a newer binary migrated was not refused")
	}
	var newer *NewerSchemaError
	if !errors.As(err, &newer) {
		t.Fatalf("error is not a *NewerSchemaError: %T %v", err, err)
	}
	if newer.DBLevel != highest+1 || newer.BinaryLevel != highest {
		t.Fatalf("levels: database %d, binary %d; want %d and %d",
			newer.DBLevel, newer.BinaryLevel, highest+1, highest)
	}
	msg := err.Error()
	for _, want := range []string{
		fmt.Sprintf("the database is at migration level %d,", highest+1),
		fmt.Sprintf("this binary knows migrations up to %d:", highest),
		"a newer holzcloud has migrated it",
		`"Rolling back"`,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
	// The operator reads this in the journal: two levels and a document
	// name, nothing that names a path on the server.
	if strings.Contains(msg, "/var/") || strings.Contains(msg, ".sqlite") {
		t.Errorf("message names a path: %s", msg)
	}
}

func TestTheGuardRefusesAHalfAppliedNewerChain(t *testing.T) {
	highest := highestEmbedded(t)
	conn := atVersion(t, highest)
	files := fstest.MapFS{
		fmt.Sprintf("%05d_newer.sql", highest+1):  {Data: []byte(newerMigration)},
		fmt.Sprintf("%05d_broken.sql", highest+2): {Data: []byte("-- +goose Up\nTHIS IS NOT SQL;\n")},
	}
	if err := applyNewer(t, conn, files); err == nil {
		t.Fatal("the broken newer migration applied")
	}
	if v, err := CurrentVersion(conn); err != nil || v != highest+1 {
		t.Fatalf("half-applied chain: level %d, err %v; want %d", v, err, highest+1)
	}

	err := RefuseNewerSchema(context.Background(), conn)
	var newer *NewerSchemaError
	if !errors.As(err, &newer) {
		t.Fatalf("a half-applied newer chain was not refused: %v", err)
	}
	if newer.DBLevel != highest+1 {
		t.Fatalf("database level %d, want %d", newer.DBLevel, highest+1)
	}
}

func TestTheGuardKnowsTheHighestEmbeddedMigration(t *testing.T) {
	conn, highest := newerDatabase(t)
	var newer *NewerSchemaError
	if err := RefuseNewerSchema(context.Background(), conn); !errors.As(err, &newer) {
		t.Fatalf("not refused: %v", err)
	}
	if newer.BinaryLevel != highest {
		t.Fatalf("binary level %d, the last embedded file says %d", newer.BinaryLevel, highest)
	}
}

func TestRunMigrationsRefusesANewerDatabase(t *testing.T) {
	conn, highest := newerDatabase(t)
	err := RunMigrations(conn)
	var newer *NewerSchemaError
	if !errors.As(err, &newer) {
		t.Fatalf("RunMigrations on a newer database: %v", err)
	}
	if v, verr := CurrentVersion(conn); verr != nil || v != highest+1 {
		t.Fatalf("level after refusal: %d, err %v; want %d", v, verr, highest+1)
	}
}
