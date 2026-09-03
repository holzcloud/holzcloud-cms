package activity

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

const timeLayout = "2006-01-02T15:04:05Z"

// Filter is the shape of a query. A zero field does not filter.
//
// Action takes two forms: an exact string like "page.create", or a family with
// a trailing ".*" — "page.*" catches every page action. The second form is
// what the screen's dropdown offers, because "everything about pages" is the
// question people actually ask.
type Filter struct {
	WebsiteID *int64
	UserID    *int64
	Action    string
	From      *time.Time
	To        *time.Time
}

// Store reads and writes activity_log.
type Store struct {
	DB *db.DB
}

// NewStore returns a store on the given database.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

// Log records one action.
//
// It returns nothing on purpose. A record that cannot be written is a fault
// worth a line in the server log, but the save it accompanies has already
// happened and the person who made it did nothing wrong — turning that into an
// error page would be the audit trail damaging the work it exists to document.
func (s *Store) Log(ctx context.Context, e Entry) {
	metaJSON, err := json.Marshal(sanitize(e.Metadata))
	if err != nil {
		slog.Error("activity log marshal", "err", err, "action", e.Action)
		return
	}
	_, err = s.DB.Write.ExecContext(ctx,
		`INSERT INTO activity_log (user_id, actor_email, action, entity_type, entity_id, website_id, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		nullableInt64(e.UserID), e.ActorEmail, e.Action, e.EntityType, e.EntityID,
		nullableInt64(e.WebsiteID), string(metaJSON))
	if err != nil {
		slog.Error("activity log write", "err", err, "action", e.Action)
	}
}

// List returns the matching entries newest first, and the total count beside
// them so the caller can draw the pager.
//
// The query always carries a LIMIT. A protocol grows without anyone deciding
// to let it, and a screen that asks for all of it works for a year and then
// stops working on the installation that needed it most.
func (s *Store) List(ctx context.Context, f Filter, limit, offset int) ([]Entry, int, error) {
	if limit <= 0 {
		limit = 50
	}

	where, args := buildWhere(f)
	var total int
	if err := s.DB.Read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM activity_log"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}

	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, limit, offset)
	listQuery := `SELECT id, user_id, actor_email, action, entity_type, entity_id, website_id, metadata, created_at
	              FROM activity_log` + where +
		` ORDER BY created_at DESC, id DESC LIMIT $` +
		intToParam(len(args)+1) + ` OFFSET $` + intToParam(len(args)+2)
	rows, err := s.DB.Read.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		e, err := scanEntryRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// Purge deletes everything older than before and records that it did so, both
// in one transaction.
//
// The record is written first and the delete then excludes it by id, not by
// time. Excluding by time would depend on the clock agreeing with itself
// between two statements; excluding by id cannot go wrong, and a purge that
// erased its own trace would be the one gap in the protocol worth having.
func (s *Store) Purge(ctx context.Context, before time.Time, actor Entry) (int64, error) {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if actor.Action == "" {
		actor.Action = ActionActivityPurge
	}
	if actor.Metadata == nil {
		actor.Metadata = map[string]any{}
	}
	actor.Metadata["before"] = before.Format("2006-01-02")
	metaJSON, err := json.Marshal(sanitize(actor.Metadata))
	if err != nil {
		return 0, fmt.Errorf("marshal sentinel metadata: %w", err)
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO activity_log (user_id, actor_email, action, entity_type, entity_id, website_id, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		nullableInt64(actor.UserID), actor.ActorEmail, actor.Action, actor.EntityType, actor.EntityID,
		nullableInt64(actor.WebsiteID), string(metaJSON))
	if err != nil {
		return 0, fmt.Errorf("insert sentinel: %w", err)
	}
	sentinelID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}

	delRes, err := tx.ExecContext(ctx,
		`DELETE FROM activity_log WHERE id < $1 AND created_at < $2`,
		sentinelID, before.Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("delete: %w", err)
	}
	deleted, _ := delRes.RowsAffected()

	// Die Zahl steht erst jetzt fest, also wird sie nachgetragen. Sie gehört in
	// die Zeile: "aufgeräumt" ohne Umfang beantwortet die Frage nicht, die man
	// später an so eine Zeile stellt.
	actor.Metadata["rows_deleted"] = deleted
	metaJSON2, err := json.Marshal(sanitize(actor.Metadata))
	if err != nil {
		return 0, fmt.Errorf("marshal sentinel metadata (post-delete): %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE activity_log SET metadata = $1 WHERE id = $2`,
		string(metaJSON2), sentinelID); err != nil {
		return 0, fmt.Errorf("update sentinel metadata: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return deleted, nil
}

// --- helpers --------------------------------------------------------------

func nullableInt64(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

// buildWhere turns a Filter into a WHERE clause and its arguments. Every value
// travels as a positional parameter — the filters come from a query string,
// and a filter built by concatenation is a query somebody else gets to write.
func buildWhere(f Filter) (string, []any) {
	var clauses []string
	var args []any
	if f.WebsiteID != nil {
		clauses = append(clauses, "website_id = $"+intToParam(len(args)+1))
		args = append(args, *f.WebsiteID)
	}
	if f.UserID != nil {
		clauses = append(clauses, "user_id = $"+intToParam(len(args)+1))
		args = append(args, *f.UserID)
	}
	if f.Action != "" {
		if strings.HasSuffix(f.Action, ".*") {
			clauses = append(clauses, "action LIKE $"+intToParam(len(args)+1))
			args = append(args, strings.TrimSuffix(f.Action, ".*")+".%")
		} else {
			clauses = append(clauses, "action = $"+intToParam(len(args)+1))
			args = append(args, f.Action)
		}
	}
	if f.From != nil {
		clauses = append(clauses, "created_at >= $"+intToParam(len(args)+1))
		args = append(args, f.From.Format(timeLayout))
	}
	if f.To != nil {
		clauses = append(clauses, "created_at <= $"+intToParam(len(args)+1))
		args = append(args, f.To.Format(timeLayout))
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func intToParam(n int) string { return fmt.Sprintf("%d", n) }

func scanEntryRow(row interface{ Scan(...any) error }) (Entry, error) {
	var e Entry
	var userID, websiteID sql.NullInt64
	var metaRaw, createdAt string
	if err := row.Scan(&e.ID, &userID, &e.ActorEmail, &e.Action, &e.EntityType, &e.EntityID,
		&websiteID, &metaRaw, &createdAt); err != nil {
		return Entry{}, err
	}
	if userID.Valid {
		v := userID.Int64
		e.UserID = &v
	}
	if websiteID.Valid {
		v := websiteID.Int64
		e.WebsiteID = &v
	}
	if metaRaw != "" {
		var m map[string]any
		if err := json.Unmarshal([]byte(metaRaw), &m); err == nil {
			e.Metadata = m
		}
	}
	e.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	return e, nil
}
