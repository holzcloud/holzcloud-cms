package db_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

func migratedDB(t *testing.T) (*db.DB, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sqlite")

	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return database, path
}

func TestQuickCheckReportsOK(t *testing.T) {
	database, _ := migratedDB(t)

	result, err := db.QuickCheck(context.Background(), database.Read)
	if err != nil {
		t.Fatalf("QuickCheck: %v", err)
	}
	if result != "ok" {
		t.Errorf("QuickCheck on a fresh database = %q; want ok", result)
	}
}

// A backup nobody verified is not a backup. Without the check a corrupted
// database is copied faithfully over the last good snapshot.
func TestBackupWritesAndVerifies(t *testing.T) {
	database, _ := migratedDB(t)
	if _, err := database.Write.Exec(
		`INSERT INTO websites (name, description) VALUES ('Test', '')`); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(t.TempDir(), "snapshot.sqlite")
	written, err := db.Backup(context.Background(), database, target)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if written != target {
		t.Errorf("Backup returned %q; want %q", written, target)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("snapshot missing: %v", err)
	}
	if info.Size() == 0 {
		t.Error("snapshot is empty")
	}

	// The snapshot must be a usable database carrying the data.
	restored, err := db.Open(target)
	if err != nil {
		t.Fatalf("reopen snapshot: %v", err)
	}
	defer restored.Close()

	var name string
	if err := restored.Read.QueryRow(`SELECT name FROM websites`).Scan(&name); err != nil {
		t.Fatalf("read from snapshot: %v", err)
	}
	if name != "Test" {
		t.Errorf("snapshot content = %q; want Test", name)
	}
}

// Into a directory, the snapshot gets a timestamped name.
func TestBackupIntoDirectory(t *testing.T) {
	database, _ := migratedDB(t)
	dir := t.TempDir()

	written, err := db.Backup(context.Background(), database, dir)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if filepath.Dir(written) != dir {
		t.Errorf("snapshot %q is not inside %q", written, dir)
	}
	if !strings.HasSuffix(written, ".sqlite") {
		t.Errorf("unexpected snapshot name: %s", written)
	}
}

// Overwriting a snapshot would destroy the copy an operator is about to
// restore from.
func TestBackupRefusesToOverwrite(t *testing.T) {
	database, _ := migratedDB(t)
	target := filepath.Join(t.TempDir(), "snapshot.sqlite")

	if _, err := db.Backup(context.Background(), database, target); err != nil {
		t.Fatalf("first backup: %v", err)
	}
	if _, err := db.Backup(context.Background(), database, target); err == nil {
		t.Error("a second backup overwrote the first")
	}
}

func TestStatsReportsSizes(t *testing.T) {
	database, _ := migratedDB(t)

	stats, err := db.Stats(context.Background(), database)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.SizeBytes == 0 || stats.PageCount == 0 {
		t.Errorf("implausible stats: %+v", stats)
	}
}

func TestMaintainCheckpointsAndSucceeds(t *testing.T) {
	database, _ := migratedDB(t)
	for i := 0; i < 50; i++ {
		if _, err := database.Write.Exec(
			`INSERT INTO websites (name, description) VALUES ('x', '')`); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Maintain(context.Background(), database); err != nil {
		t.Fatalf("Maintain: %v", err)
	}
}

func TestCurrentVersionMatchesMigrations(t *testing.T) {
	database, _ := migratedDB(t)

	version, err := db.CurrentVersion(database.Write)
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if version < 7 {
		t.Errorf("schema version = %d; want at least 7", version)
	}

	pending, err := db.HasPendingMigrations(database.Write)
	if err != nil {
		t.Fatalf("HasPendingMigrations: %v", err)
	}
	if pending {
		t.Error("migrations still pending right after running them")
	}
}

// The pre-migration snapshot runs before any schema exists. Verifying it by
// querying an application table made every first start fail.
func TestBackupOfAnEmptyDatabase(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "fresh.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()
	// Deliberately no RunMigrations — this is what a first start looks like.

	target := filepath.Join(t.TempDir(), "pre-upgrade.sqlite")
	if _, err := db.Backup(context.Background(), database, target); err != nil {
		t.Fatalf("backup of an unmigrated database failed: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("snapshot missing: %v", err)
	}
}

// Snapshots accumulate one per upgrade, so the oldest have to go.
func TestPruneSnapshotsKeepsTheNewest(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"pre-upgrade-v1-20260101-000000.sqlite",
		"pre-upgrade-v2-20260201-000000.sqlite",
		"pre-upgrade-v3-20260301-000000.sqlite",
		"pre-upgrade-v4-20260401-000000.sqlite",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// An unrelated file must survive.
	if err := os.WriteFile(filepath.Join(dir, "holzcloud.sqlite"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := db.PruneSnapshots(dir, 2, ""); err != nil {
		t.Fatalf("PruneSnapshots: %v", err)
	}

	for _, gone := range names[:2] {
		if _, err := os.Stat(filepath.Join(dir, gone)); err == nil {
			t.Errorf("%s should have been pruned", gone)
		}
	}
	for _, kept := range names[2:] {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s should have been kept: %v", kept, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "holzcloud.sqlite")); err != nil {
		t.Error("the live database was removed")
	}
}

// The version stands before the timestamp in a snapshot's name, so the order
// of the names is the order of the version strings: v0.1.10 before v0.1.5,
// v0.1.4-65-g… before v0.1.4-9-g…, dev before v…. Pruning has to go by the
// timestamp, or the snapshot just written is the first to go.
func TestPruneSnapshotsOrdersByTimeNotByVersion(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"pre-upgrade-v0.1.5-20261201-000000.sqlite",
		"pre-upgrade-v0.1.6-20270101-000000.sqlite",
		"pre-upgrade-v0.1.7-20270201-000000.sqlite",
		"pre-upgrade-v0.1.8-20270301-000000.sqlite",
		"pre-upgrade-v0.1.9-20270401-000000.sqlite",
		"pre-upgrade-v0.1.10-20270501-000000.sqlite",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.PruneSnapshots(dir, 5, ""); err != nil {
		t.Fatalf("PruneSnapshots: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, names[0])); err == nil {
		t.Errorf("%s is the oldest and should have been pruned", names[0])
	}
	for _, kept := range names[1:] {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s should have been kept: %v", kept, err)
		}
	}
}

// The snapshot just written is the one the way back names, so it survives
// even when its timestamp is not the newest: a clock set back, or a name that
// cannot be read as a time.
func TestPruneSnapshotsNeverRemovesTheOneJustWritten(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"pre-upgrade-dev-20200101-000000.sqlite", // just written, clock set back
		"pre-upgrade-v0.1.4-65-gabc1234-20270101-000000.sqlite",
		"pre-upgrade-v0.1.4-9-gdef5678-20270201-000000.sqlite",
		"pre-upgrade-v0.1.5-20270301-000000.sqlite",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	justWritten := filepath.Join(dir, names[0])
	if err := db.PruneSnapshots(dir, 2, justWritten); err != nil {
		t.Fatalf("PruneSnapshots: %v", err)
	}
	if _, err := os.Stat(justWritten); err != nil {
		t.Errorf("the snapshot just written was pruned: %v", err)
	}
	// Two in all: the one just written and the newest of the rest.
	if _, err := os.Stat(filepath.Join(dir, names[3])); err != nil {
		t.Errorf("%s is the newest of the rest and should have been kept: %v", names[3], err)
	}
	for _, gone := range names[1:3] {
		if _, err := os.Stat(filepath.Join(dir, gone)); err == nil {
			t.Errorf("%s should have been pruned", gone)
		}
	}
}
