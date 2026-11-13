package db

import (
	"context"
	"database/sql"
	"fmt"
)

// NewerSchemaError reports a database that a newer holzcloud has migrated
// beyond the highest migration this binary embeds.
//
// The sentence Error returns is quoted by deploy/DEPLOY.md and by the rollback
// record of phase 34, so it changes only together with them.
type NewerSchemaError struct {
	DBLevel, BinaryLevel int64
}

func (e *NewerSchemaError) Error() string {
	return fmt.Sprintf("the database is at migration level %d, and this binary knows migrations up to %d: "+
		"a newer holzcloud has migrated it, and this binary refuses to run against a schema it does not know. "+
		"The way back: start the newer binary again, or restore the pre-upgrade snapshot that the newer binary "+
		"wrote before it migrated, together with the older binary, as deploy/DEPLOY.md describes under \"Rolling back\"",
		e.DBLevel, e.BinaryLevel)
}

// RefuseNewerSchema returns a *NewerSchemaError when the database records a
// migration level above the highest one this binary embeds.
//
// goose does not refuse this on its own: it treats only unapplied versions
// below the database maximum as missing, so a database a newer binary migrated
// meets an older binary in silence, and the older binary would then write rows
// into a schema it does not know. The comparison uses goose's own reading of
// both levels (Provider.GetVersions), the database maximum against the highest
// embedded version.
func RefuseNewerSchema(ctx context.Context, conn *sql.DB) error {
	provider, err := migrationProvider(conn)
	if err != nil {
		return err
	}
	current, target, err := provider.GetVersions(ctx)
	if err != nil {
		return fmt.Errorf("schema level: %w", err)
	}
	if current > target {
		return &NewerSchemaError{DBLevel: current, BinaryLevel: target}
	}
	return nil
}
