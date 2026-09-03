package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/pressly/goose/v3"
)

// QuickCheck runs PRAGMA quick_check and returns its verdict, "ok" when the
// database is sound.
//
// quick_check is used rather than integrity_check because it catches exactly
// the page-level damage a power cut produces, at a fraction of the cost —
// which matters when it runs at every start.
func QuickCheck(ctx context.Context, db *sql.DB) (string, error) {
	var result string
	if err := db.QueryRowContext(ctx, `PRAGMA quick_check(1)`).Scan(&result); err != nil {
		return "", fmt.Errorf("quick_check: %w", err)
	}
	return result, nil
}

// CurrentVersion reports the applied schema version.
func CurrentVersion(db *sql.DB) (int64, error) {
	version, err := goose.GetDBVersion(db)
	if err != nil {
		return 0, fmt.Errorf("schema version: %w", err)
	}
	return version, nil
}

// HasPendingMigrations reports whether migrations are waiting to be applied.
// It is asked before RunMigrations so a snapshot can be taken first.
func HasPendingMigrations(db *sql.DB) (bool, error) {
	provider, err := migrationProvider(db)
	if err != nil {
		return false, err
	}
	status, err := provider.Status(context.Background())
	if err != nil {
		return false, fmt.Errorf("migration status: %w", err)
	}
	for _, s := range status {
		if s.State == goose.StatePending {
			return true, nil
		}
	}
	return false, nil
}

// Backup writes a consistent, defragmented snapshot and verifies it.
//
// VACUUM INTO produces the snapshot through the existing pure-Go driver, so the
// backup no longer shells out to the sqlite3 CLI — the very dependency
// modernc.org/sqlite exists to avoid. The copy is then reopened and checked, because
// a backup nobody has verified is not a backup: without this a corrupted
// database was faithfully copied over the last good one until every retained
// snapshot was broken.
//
// target may be a directory, in which case a timestamped filename is used.
func Backup(ctx context.Context, database *DB, target string) (string, error) {
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		name := fmt.Sprintf("holzcloud-%s.sqlite", time.Now().UTC().Format("20060102-150405"))
		target = filepath.Join(target, name)
	}
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("refusing to overwrite existing file %s", target)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}

	// VACUUM INTO cannot take a bound parameter.
	if _, err := database.Write.ExecContext(ctx, `VACUUM INTO '`+escapeSQLiteString(target)+`'`); err != nil {
		return "", fmt.Errorf("vacuum into %s: %w", target, err)
	}

	verify, err := Open(target)
	if err != nil {
		return "", fmt.Errorf("reopen snapshot for verification: %w", err)
	}
	defer verify.Close()

	result, err := QuickCheck(ctx, verify.Read)
	if err != nil {
		return "", fmt.Errorf("verify snapshot: %w", err)
	}
	if result != "ok" {
		return "", fmt.Errorf("snapshot %s failed its integrity check: %s", target, result)
	}

	// Count the tables rather than rows of a specific one. The pre-migration
	// snapshot runs before any schema exists, so requiring a table here made
	// every first start fail.
	var tables int
	if err := verify.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'`).Scan(&tables); err != nil {
		return "", fmt.Errorf("verify snapshot contents: %w", err)
	}
	slog.Info("backup verified", "path", target, "tables", tables)
	return target, nil
}

// Compact rebuilds the database file, returning its size before and after.
//
// auto_vacuum cannot be enabled on an existing database, so reclaiming space is
// an explicit operation rather than something that happens on its own.
func Compact(ctx context.Context, database *DB) (before, after int64, err error) {
	path, err := databasePath(ctx, database.Read)
	if err != nil {
		return 0, 0, err
	}
	if info, statErr := os.Stat(path); statErr == nil {
		before = info.Size()
	}

	tmp := path + ".compact"
	_ = os.Remove(tmp)
	if _, err := database.Write.ExecContext(ctx, `VACUUM INTO '`+escapeSQLiteString(tmp)+`'`); err != nil {
		return before, 0, fmt.Errorf("vacuum into %s: %w", tmp, err)
	}

	verify, err := Open(tmp)
	if err != nil {
		return before, 0, fmt.Errorf("reopen compacted file: %w", err)
	}
	result, checkErr := QuickCheck(ctx, verify.Read)
	verify.Close()
	if checkErr != nil {
		return before, 0, checkErr
	}
	if result != "ok" {
		_ = os.Remove(tmp)
		return before, 0, fmt.Errorf("compacted file failed its integrity check: %s", result)
	}

	if info, statErr := os.Stat(tmp); statErr == nil {
		after = info.Size()
	}
	// The caller holds the original open, so replacing it is only safe as a
	// one-shot CLI operation — which is why compaction is not a background job.
	if err := os.Rename(tmp, path); err != nil {
		return before, after, fmt.Errorf("replace database: %w", err)
	}
	return before, after, nil
}

// Maintain runs the periodic housekeeping: let SQLite update its statistics,
// fold the WAL back into the main file so it cannot grow without bound, and
// report the sizes an operator needs to see growth coming.
func Maintain(ctx context.Context, database *DB) error {
	if _, err := database.Write.ExecContext(ctx, `PRAGMA optimize`); err != nil {
		return fmt.Errorf("optimize: %w", err)
	}
	var busy, logPages, checkpointed int
	if err := database.Write.QueryRowContext(ctx,
		`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logPages, &checkpointed); err != nil {
		return fmt.Errorf("wal_checkpoint: %w", err)
	}

	stats, err := Stats(ctx, database)
	if err != nil {
		return err
	}
	slog.Info("database maintenance",
		"checkpoint_busy", busy,
		"db_size_bytes", stats.SizeBytes,
		"wal_size_bytes", stats.WALBytes,
		"page_count", stats.PageCount,
		"freelist_count", stats.FreelistCount,
	)
	return nil
}

// DBStats is the size information reported by /readyz and the maintenance job.
type DBStats struct {
	SizeBytes     int64
	WALBytes      int64
	PageCount     int64
	FreelistCount int64
}

// Stats reads the current database sizes.
func Stats(ctx context.Context, database *DB) (DBStats, error) {
	var s DBStats
	if err := database.Read.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&s.PageCount); err != nil {
		return s, fmt.Errorf("page_count: %w", err)
	}
	if err := database.Read.QueryRowContext(ctx, `PRAGMA freelist_count`).Scan(&s.FreelistCount); err != nil {
		return s, fmt.Errorf("freelist_count: %w", err)
	}

	path, err := databasePath(ctx, database.Read)
	if err != nil {
		return s, err
	}
	if info, err := os.Stat(path); err == nil {
		s.SizeBytes = info.Size()
	}
	if info, err := os.Stat(path + "-wal"); err == nil {
		s.WALBytes = info.Size()
	}
	return s, nil
}

// databasePath asks SQLite where the main database actually lives, rather than
// reconstructing it from configuration.
func databasePath(ctx context.Context, db *sql.DB) (string, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA database_list`)
	if err != nil {
		return "", fmt.Errorf("database_list: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var seq int
		var name, file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return "", err
		}
		if name == "main" {
			return file, nil
		}
	}
	return "", fmt.Errorf("no main database in database_list")
}

// escapeSQLiteString escapes a value for a single-quoted SQL literal, needed
// because VACUUM INTO does not accept a bound parameter.
func escapeSQLiteString(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'')
		}
		out = append(out, s[i])
	}
	return string(out)
}

// PruneSnapshots keeps the newest keep pre-upgrade snapshots in dir and removes
// the rest, so a long-lived installation does not accumulate a copy of the
// database for every upgrade it has ever seen.
func PruneSnapshots(dir string, keep int) error {
	matches, err := filepath.Glob(filepath.Join(dir, "pre-upgrade-*.sqlite"))
	if err != nil {
		return fmt.Errorf("list snapshots: %w", err)
	}
	if len(matches) <= keep {
		return nil
	}

	// The filenames carry a sortable UTC timestamp, so lexical order is
	// chronological order.
	sort.Strings(matches)
	for _, path := range matches[:len(matches)-keep] {
		if err := os.Remove(path); err != nil {
			slog.Warn("cannot remove old snapshot", "path", path, "err", err)
			continue
		}
		slog.Info("removed old pre-upgrade snapshot", "path", path)
	}
	return nil
}
