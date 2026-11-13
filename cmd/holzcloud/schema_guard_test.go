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
