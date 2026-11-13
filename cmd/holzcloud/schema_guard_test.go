package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"

	"github.com/holzcloud/holzcloud-cms/internal/config"
	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// newerDatabaseIn leaves a database in dataDir that a newer holzcloud has
// migrated one level beyond this binary: migrated to this binary's head, then
// one more migration applied through a real goose provider. It returns this
// binary's level, read from the database after RunMigrations, so no level is
// written here as a number.
func newerDatabaseIn(t *testing.T, dataDir string) int64 {
	t.Helper()
	database, err := db.Open(filepath.Join(dataDir, "holzcloud.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	highest, err := db.CurrentVersion(database.Write)
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	files := fstest.MapFS{
		fmt.Sprintf("%05d_newer.sql", highest+1): {Data: []byte(
			"-- +goose Up\nCREATE TABLE newer_binary_table (id INTEGER PRIMARY KEY) STRICT;\n\n" +
				"-- +goose Down\nDROP TABLE newer_binary_table;\n")},
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, database.Write, files)
	if err != nil {
		t.Fatalf("newer provider: %v", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("apply newer migration: %v", err)
	}
	return highest
}

func levelIn(t *testing.T, dataDir string) int64 {
	t.Helper()
	database, err := db.Open(filepath.Join(dataDir, "holzcloud.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()
	v, err := db.CurrentVersion(database.Write)
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	return v
}

func TestEveryDatabaseSubcommandRefusesANewerDatabase(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("HOLZCLOUD_DATA_DIR", dataDir)
	highest := newerDatabaseIn(t, dataDir)

	for name, run := range map[string]func() error{
		"check":          func() error { return cmdCheck(nil) },
		"backup":         func() error { return cmdBackup([]string{t.TempDir()}) },
		"migrate status": func() error { return cmdMigrate([]string{"status"}) },
	} {
		t.Run(name, func(t *testing.T) {
			err := run()
			var newer *db.NewerSchemaError
			if !errors.As(err, &newer) {
				t.Fatalf("%s on a newer database: %v", name, err)
			}
			if newer.DBLevel != highest+1 || newer.BinaryLevel != highest {
				t.Errorf("levels: database %d, binary %d; want %d and %d",
					newer.DBLevel, newer.BinaryLevel, highest+1, highest)
			}
			if strings.HasPrefix(err.Error(), "migrations:") {
				t.Errorf("the refusal is wrapped: %s", err)
			}
			if !strings.HasPrefix(err.Error(), "the database is at migration level ") {
				t.Errorf("the operator does not read the refusal first: %s", err)
			}
		})
	}
	if v := levelIn(t, dataDir); v != highest+1 {
		t.Fatalf("level after the refusals: %d, want %d", v, highest+1)
	}
}

func snapshotsIn(t *testing.T, dataDir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dataDir, "pre-upgrade-*.sqlite"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return matches
}

func TestTheServerRefusesANewerDatabaseBeforeItSnapshots(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("HOLZCLOUD_DATA_DIR", dataDir)
	highest := newerDatabaseIn(t, dataDir)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	database, err := openDatabase(cfg)
	if database != nil {
		database.Close()
	}
	var newer *db.NewerSchemaError
	if !errors.As(err, &newer) {
		t.Fatalf("openDatabase on a newer database: %v", err)
	}
	if newer.DBLevel != highest+1 || newer.BinaryLevel != highest {
		t.Errorf("levels: database %d, binary %d; want %d and %d",
			newer.DBLevel, newer.BinaryLevel, highest+1, highest)
	}
	if got := snapshotsIn(t, dataDir); len(got) != 0 {
		t.Errorf("a refused database got a pre-upgrade snapshot: %v", got)
	}
	if v := levelIn(t, dataDir); v != highest+1 {
		t.Errorf("level after the refusal: %d, want %d", v, highest+1)
	}
}

// The refusal of a newer schema names "the pre-upgrade snapshot that the newer
// binary wrote before it migrated" as the way back. A subcommand migrates as
// well, so it writes that snapshot as the service does; otherwise a new binary
// that first touched the database through `user list` or `backup` would leave
// no snapshot to go back to (code review 32, IN-05).
func TestASubcommandSnapshotsBeforeItMigrates(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("HOLZCLOUD_DATA_DIR", dataDir)

	if err := cmdCheck(nil); err != nil {
		t.Fatalf("check on an empty data directory: %v", err)
	}
	got := snapshotsIn(t, dataDir)
	if len(got) != 1 {
		t.Fatalf("after a subcommand migrated: %d pre-upgrade snapshots, want 1: %v", len(got), got)
	}
	if v := levelIn(t, dataDir); v == 0 {
		t.Fatal("the subcommand did not migrate")
	}

	// Nothing pending now: no further snapshot.
	if err := cmdCheck(nil); err != nil {
		t.Fatalf("second check: %v", err)
	}
	if again := snapshotsIn(t, dataDir); len(again) != 1 {
		t.Errorf("after a subcommand with nothing pending: %d pre-upgrade snapshots, want 1: %v", len(again), again)
	}
}

// A database one level below, with rows in it, is covered by
// TestTheGuardLetsAnOlderDatabasePassAndItMigrates in internal/db: package main
// cannot reach the unexported migration provider that migrates part-way, and no
// export is added for a test.
func TestTheServerStartsOnADatabaseAtOrBelowItsLevel(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("HOLZCLOUD_DATA_DIR", dataDir)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	// (a) An empty data directory is level 0, below the binary: the first
	// start writes its small snapshot and migrates, as before.
	database, err := openDatabase(cfg)
	if err != nil {
		t.Fatalf("first start: %v", err)
	}
	highest, err := db.CurrentVersion(database.Write)
	database.Close()
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if highest == 0 {
		t.Fatal("the first start did not migrate")
	}
	if got := snapshotsIn(t, dataDir); len(got) != 1 {
		t.Fatalf("after the first start: %d pre-upgrade snapshots, want 1: %v", len(got), got)
	}

	// (b) The same directory, now at the binary's own level: no refusal and
	// no further snapshot.
	database, err = openDatabase(cfg)
	if err != nil {
		t.Fatalf("second start at the binary's own level: %v", err)
	}
	database.Close()
	if got := snapshotsIn(t, dataDir); len(got) != 1 {
		t.Fatalf("after the second start: %d pre-upgrade snapshots, want 1: %v", len(got), got)
	}
	if v := levelIn(t, dataDir); v != highest {
		t.Fatalf("level after the second start: %d, want %d", v, highest)
	}
}
