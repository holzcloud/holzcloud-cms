package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const dsnTemplate = "file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-2000)"

// writeDSNSuffix makes transactions on the write pool take their write lock up
// front (BEGIN IMMEDIATE).
//
// With the default deferred begin, a transaction that reads first and writes
// later has to upgrade its lock mid-flight; if another connection wrote in the
// meantime SQLite returns SQLITE_BUSY immediately instead of waiting out
// busy_timeout, because backing off could deadlock. Taking the lock at BEGIN
// turns that into an ordinary wait.
const writeDSNSuffix = "&_txlock=immediate"

type DB struct {
	Write *sql.DB
	Read  *sql.DB
}

func Open(dbPath string) (*DB, error) {
	dsn := fmt.Sprintf(dsnTemplate, dbPath)

	writeDB, err := sql.Open("sqlite", dsn+writeDSNSuffix)
	if err != nil {
		return nil, fmt.Errorf("open write db: %w", err)
	}
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)

	readDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		writeDB.Close()
		return nil, fmt.Errorf("open read db: %w", err)
	}
	readDB.SetMaxOpenConns(5)

	var mode string
	if err := writeDB.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		writeDB.Close()
		readDB.Close()
		return nil, fmt.Errorf("verify WAL: %w", err)
	}
	if mode != "wal" {
		writeDB.Close()
		readDB.Close()
		return nil, fmt.Errorf("expected WAL journal mode, got %q", mode)
	}

	slog.Info("sqlite opened", "journal_mode", mode, "path", dbPath)
	return &DB{Write: writeDB, Read: readDB}, nil
}

// migrationProvider builds the goose provider over the embedded migrations.
func migrationProvider(db *sql.DB) (*goose.Provider, error) {
	migrations, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrations sub fs: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations)
	if err != nil {
		return nil, fmt.Errorf("goose provider: %w", err)
	}
	return provider, nil
}

func RunMigrations(db *sql.DB) error {
	provider, err := migrationProvider(db)
	if err != nil {
		return err
	}
	results, err := provider.Up(context.Background())
	if err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	for _, r := range results {
		slog.Info("migration applied",
			"version", r.Source.Version,
			"type", r.Source.Type,
			"duration_ms", r.Duration.Milliseconds(),
		)
	}
	return nil
}

func (d *DB) Close() {
	d.Write.Close()
	d.Read.Close()
}
