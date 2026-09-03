package db_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

func TestOpen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	if n := database.Write.Stats().MaxOpenConnections; n != 1 {
		t.Errorf("Write.MaxOpenConns: want 1, got %d", n)
	}
	if n := database.Read.Stats().MaxOpenConnections; n != 5 {
		t.Errorf("Read.MaxOpenConns: want 5, got %d", n)
	}

	var mode string
	if err := database.Write.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode: want wal, got %s", mode)
	}

	var fk int
	if err := database.Write.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys: want 1, got %d", fk)
	}
}

func TestRunMigrations(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	var name string
	err = database.Read.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='users'`,
	).Scan(&name)
	if err != nil {
		t.Fatalf("users table not found after migrations: %v", err)
	}

	_, err = database.Write.Exec(
		`INSERT INTO users (email, password, role, created_at) VALUES (?, ?, ?, ?)`,
		"test@example.com", "hash", "superuser", "2026-01-01T00:00:00Z",
	)
	if err == nil {
		t.Error("expected CHECK constraint violation for role='superuser', got nil")
	}

	_, err = database.Write.Exec(
		`INSERT INTO users (email, password, role, created_at) VALUES (?, ?, ?, ?)`,
		"Admin@Example.com", "hash", "admin", "2026-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err = database.Write.Exec(
		`INSERT INTO users (email, password, role, created_at) VALUES (?, ?, ?, ?)`,
		"admin@example.com", "hash", "admin", "2026-01-01T00:00:00Z",
	)
	if err == nil {
		t.Error("expected UNIQUE COLLATE NOCASE violation for duplicate email, got nil")
	}
}

func TestRunMigrationsIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("second RunMigrations (idempotent): %v", err)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
