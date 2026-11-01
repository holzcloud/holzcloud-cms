package db

import "testing"

// Accounts that exist when the column is added are marked as having seen the
// last release without it; accounts created afterwards start empty. The
// difference is the whole feature: the first is shown the next release once,
// the second is never shown anything it has nothing to catch up on.
func TestSeenVersionMigrationMarksExistingAccountsOnly(t *testing.T) {
	conn := atVersion(t, 60)

	oldID := mustExec(t, conn,
		`INSERT INTO users (email, password, role) VALUES ('alt@test', 'x', 'admin')`)
	migrateToHead(t, conn)

	var seen string
	if err := conn.QueryRow(`SELECT seen_version FROM users WHERE id = $1`, oldID).Scan(&seen); err != nil {
		t.Fatalf("read old: %v", err)
	}
	if seen != "0.1.2" {
		t.Errorf("an account that predates the column has seen_version %q, want 0.1.2", seen)
	}

	newID := mustExec(t, conn,
		`INSERT INTO users (email, password, role) VALUES ('neu@test', 'x', 'editor')`)
	if err := conn.QueryRow(`SELECT seen_version FROM users WHERE id = $1`, newID).Scan(&seen); err != nil {
		t.Fatalf("read new: %v", err)
	}
	if seen != "" {
		t.Errorf("an account created after the column has seen_version %q, want empty", seen)
	}
}
