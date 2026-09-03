package auth

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE sessions (
			token  TEXT PRIMARY KEY,
			data   BLOB NOT NULL,
			expiry REAL NOT NULL
		);
		CREATE INDEX idx_sessions_expiry ON sessions(expiry);
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestStoreCommitAndFind(t *testing.T) {
	db := setupTestDB(t)
	store := NewSQLiteStore(db)

	err := store.Commit("tok1", []byte("hello"), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	data, found, err := store.Find("tok1")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}

func TestStoreFindNonExistent(t *testing.T) {
	db := setupTestDB(t)
	store := NewSQLiteStore(db)

	_, found, err := store.Find("nope")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if found {
		t.Error("expected found=false for non-existent token")
	}
}

func TestStoreDelete(t *testing.T) {
	db := setupTestDB(t)
	store := NewSQLiteStore(db)

	store.Commit("tok2", []byte("data"), time.Now().Add(time.Hour))
	if err := store.Delete("tok2"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, found, _ := store.Find("tok2")
	if found {
		t.Error("expected found=false after delete")
	}
}

func TestStoreCommitOverwrite(t *testing.T) {
	db := setupTestDB(t)
	store := NewSQLiteStore(db)

	store.Commit("tok3", []byte("v1"), time.Now().Add(time.Hour))
	store.Commit("tok3", []byte("v2"), time.Now().Add(time.Hour))
	data, found, _ := store.Find("tok3")
	if !found {
		t.Fatal("expected found=true")
	}
	if string(data) != "v2" {
		t.Errorf("expected 'v2', got %q", string(data))
	}
}

func TestStoreExpiredNotReturned(t *testing.T) {
	db := setupTestDB(t)
	store := NewSQLiteStore(db)

	// Commit with expiry in the past
	store.Commit("expired", []byte("old"), time.Now().Add(-time.Hour))
	_, found, err := store.Find("expired")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if found {
		t.Error("expected found=false for expired session")
	}
}

// Expired sessions are removed by a job rather than a goroutine the store owns,
// so the deletion itself has to work on demand.
func TestDeleteExpired(t *testing.T) {
	database := setupTestDB(t)
	store := NewSQLiteStore(database)

	if err := store.Commit("frisch", []byte("a"), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("commit fresh: %v", err)
	}
	if err := store.Commit("abgelaufen", []byte("b"), time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("commit expired: %v", err)
	}

	if err := store.DeleteExpired(context.Background()); err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}

	if _, found, _ := store.Find("frisch"); !found {
		t.Error("a live session was deleted")
	}
	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token = 'abgelaufen'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("the expired session survived")
	}
}
