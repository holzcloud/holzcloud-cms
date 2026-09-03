package page

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Redirects stay in the core, unlike the search and the 404 log.
//
// They look like an optional feature and are not one. recordSlugChange below
// runs inside the same transaction as the rename: a link cannot be broken by a
// save that then failed to write the redirect. A plugin cannot be given that —
// it is told after the fact, and a plugin that misses the message leaves an old
// address quietly dead with nothing anywhere to say so.
//
// So this is not "a feature for keeping old URLs alive". It is the CMS not
// breaking its own addresses when someone renames a page, and that belongs to
// whoever does the renaming.

// Redirect maps an old address to a new one.
type Redirect struct {
	ID        int64
	WebsiteID int64
	FromPath  string
	ToPath    string
	Code      int
	// Source is "auto" for a redirect written by a rename and "manual" for one
	// the operator entered, typically when migrating an existing site.
	Source    string
	Hits      int64
	CreatedAt time.Time
}

// recordSlugChange keeps old addresses working after a rename.
//
// It runs inside the same transaction as the rename, so a link cannot be broken
// by a save that then failed to write the redirect. Three statements, each for
// a specific way this goes wrong:
//
//  1. the redirect itself, replacing any older one for the same address;
//  2. deleting a redirect that points *away* from the new address, so moving a
//     page back does not create a loop;
//  3. repointing redirects that ended at the old address, so a page renamed
//     twice collapses to one hop instead of needing a runtime hop limit.
func recordSlugChange(ctx context.Context, tx *sql.Tx, websiteID int64, oldSlug, newSlug string) error {
	if oldSlug == newSlug || oldSlug == "" || newSlug == "" {
		return nil
	}
	from, to := "/"+oldSlug, "/"+newSlug

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM redirects WHERE website_id = $1 AND from_path = $2`, websiteID, to); err != nil {
		return fmt.Errorf("clear reverse redirect: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE redirects SET to_path = $1 WHERE website_id = $2 AND to_path = $3`,
		to, websiteID, from); err != nil {
		return fmt.Errorf("collapse redirect chain: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO redirects (website_id, from_path, to_path, source) VALUES ($1, $2, $3, 'auto')
		 ON CONFLICT (website_id, from_path) DO UPDATE SET to_path = excluded.to_path`,
		websiteID, from, to); err != nil {
		return fmt.Errorf("record redirect: %w", err)
	}
	return nil
}

// LookupRedirect finds the target for a path and counts the hit.
//
// It is called only after the page lookup came up empty, so it costs nothing on
// the hot path — an indexed lookup on the way to what would otherwise be a 404.
func (s *Store) LookupRedirect(ctx context.Context, websiteID int64, path string) (*Redirect, error) {
	var r Redirect
	var createdAt string
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, website_id, from_path, to_path, code, source, hits, created_at
		 FROM redirects WHERE website_id = $1 AND from_path = $2`, websiteID, path).
		Scan(&r.ID, &r.WebsiteID, &r.FromPath, &r.ToPath, &r.Code, &r.Source, &r.Hits, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup redirect: %w", err)
	}
	r.CreatedAt, _ = time.Parse(timeLayout, createdAt)

	// The counter is what tells the operator which old URL still brings
	// traffic, so it is worth a write — but never at the cost of the response.
	if _, err := s.DB.Write.ExecContext(ctx,
		`UPDATE redirects SET hits = hits + 1 WHERE id = $1`, r.ID); err != nil {
		return &r, nil
	}
	return &r, nil
}

// ListRedirects returns the redirects of a website, most used first.
func (s *Store) ListRedirects(ctx context.Context, websiteID int64) ([]Redirect, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, from_path, to_path, code, source, hits, created_at
		 FROM redirects WHERE website_id = $1 ORDER BY hits DESC, from_path`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("list redirects: %w", err)
	}
	defer rows.Close()

	var out []Redirect
	for rows.Next() {
		var r Redirect
		var createdAt string
		if err := rows.Scan(&r.ID, &r.WebsiteID, &r.FromPath, &r.ToPath, &r.Code,
			&r.Source, &r.Hits, &createdAt); err != nil {
			return nil, fmt.Errorf("scan redirect: %w", err)
		}
		r.CreatedAt, _ = time.Parse(timeLayout, createdAt)
		out = append(out, r)
	}
	return out, rows.Err()
}

// AddRedirect stores a redirect the operator entered by hand, which is how an
// existing site's old URL map gets carried over.
func (s *Store) AddRedirect(ctx context.Context, websiteID int64, from, to string, code int) error {
	if code != 302 {
		code = 301
	}
	_, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO redirects (website_id, from_path, to_path, code, source)
		 VALUES ($1, $2, $3, $4, 'manual')
		 ON CONFLICT (website_id, from_path)
		 DO UPDATE SET to_path = excluded.to_path, code = excluded.code, source = 'manual'`,
		websiteID, from, to, code)
	if err != nil {
		return fmt.Errorf("add redirect: %w", err)
	}
	return nil
}

// DeleteRedirect removes one redirect.
func (s *Store) DeleteRedirect(ctx context.Context, websiteID, id int64) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM redirects WHERE id = $1 AND website_id = $2`, id, websiteID)
	if err != nil {
		return fmt.Errorf("delete redirect: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
