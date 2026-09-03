package page

import (
	"context"
	"database/sql"
	"fmt"
)

// ListArchive returns one page of published posts, newest first, and the total.
//
// The order is by publication date rather than by last edit: correcting a typo
// in a two-year-old entry must not lift it back to the top of the archive.
func (s *Store) ListArchive(ctx context.Context, websiteID int64, pageNum, perPage int) ([]Page, int, error) {
	return s.ListArchiveIn(ctx, websiteID, "", pageNum, perPage)
}

// ListArchiveIn is the same for one language. The French archive lists the
// French entries; a German entry among them would be a dead end for a reader
// who cannot read it.
func (s *Store) ListArchiveIn(ctx context.Context, websiteID int64, loc string, pageNum, perPage int) ([]Page, int, error) {
	return s.listWhere(ctx, websiteID, loc, "kind", KindPost, false, pageNum, perPage)
}

// ListOfKind is the overview of one content kind: the posts, or the entries of
// a kind the operator defined.
//
// The sort is a flag and not a string from the caller, because it ends up in
// SQL and cannot be a bound parameter. Two orders are what an overview needs —
// by date for anything that happens, by title for anything that is.
func (s *Store) ListOfKind(ctx context.Context, websiteID int64, loc, typeKey string, byTitle bool, pageNum, perPage int) ([]Page, int, error) {
	return s.listWhere(ctx, websiteID, loc, "art", typeKey, byTitle, pageNum, perPage)
}

// listWhere is the shared body: the same query against kind or against art.
//
// column is one of two literals from this file and never a caller's string —
// it cannot be a bound parameter, and interpolating something from outside is
// how an injection is written.
func (s *Store) listWhere(ctx context.Context, websiteID int64, loc, column, value string, byTitle bool, pageNum, perPage int) ([]Page, int, error) {
	if column != "kind" && column != "art" {
		return nil, 0, fmt.Errorf("unbekannte Spalte %q", column)
	}
	if perPage <= 0 {
		perPage = 10
	}
	if pageNum <= 0 {
		pageNum = 1
	}

	var total int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pages WHERE website_id = $1 AND locale = $2 AND `+column+` = $3`+ListablePredicate,
		websiteID, loc, value).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count archive: %w", err)
	}

	order := ` ORDER BY COALESCE(published_at, created_at) DESC, id DESC`
	if byTitle {
		order = ` ORDER BY title COLLATE NOCASE ASC, id ASC`
	}
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+pageColumns+` FROM pages WHERE website_id = $1 AND locale = $4 AND `+column+` = $5`+ListablePredicate+
			order+` LIMIT $2 OFFSET $3`,
		websiteID, perPage, (pageNum-1)*perPage, loc, value)
	if err != nil {
		return nil, 0, fmt.Errorf("list archive: %w", err)
	}
	defer rows.Close()

	var posts []Page
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan archive entry: %w", err)
		}
		posts = append(posts, *p)
	}
	return posts, total, rows.Err()
}

// AdjacentPosts returns the entries published just before and just after one.
//
// An archive without them is a dead end: a reader who followed a link to a
// single entry has no way onward except back to the list.
func (s *Store) AdjacentPosts(ctx context.Context, websiteID int64, p *Page) (prev, next *Page, err error) {
	if p == nil || !p.IsPost() {
		return nil, nil, nil
	}
	// COALESCE mirrors the archive's own ordering, so "the next one" here and
	// "the one above it in the list" are always the same entry.
	anchor := p.CreatedAt
	if p.PublishedAt != nil {
		anchor = *p.PublishedAt
	}
	stamp := anchor.UTC().Format(timeLayout)

	// Neighbours stay inside the language: walking the archive must not drop a
	// reader into an entry they cannot read.
	newer, err := s.adjacent(ctx, websiteID, p.Locale, stamp, p.ID, true)
	if err != nil {
		return nil, nil, err
	}
	older, err := s.adjacent(ctx, websiteID, p.Locale, stamp, p.ID, false)
	if err != nil {
		return nil, nil, err
	}
	// "prev" is the older entry, which is what a reader going backwards through
	// an archive expects.
	return older, newer, nil
}

func (s *Store) adjacent(ctx context.Context, websiteID int64, loc, stamp string, id int64, newer bool) (*Page, error) {
	cmp, order := "<", "DESC"
	if newer {
		cmp, order = ">", "ASC"
	}
	// The id breaks ties, so two entries published in the same second still have
	// a stable order and neither can be skipped.
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT `+pageColumns+` FROM pages
		 WHERE website_id = $1 AND locale = $4 AND kind = 'post'`+ListablePredicate+`
		   AND (COALESCE(published_at, created_at), id) `+cmp+` ($2, $3)
		 ORDER BY COALESCE(published_at, created_at) `+order+`, id `+order+` LIMIT 1`,
		websiteID, stamp, id, loc)

	p, err := scanPage(row)
	if err == sql.ErrNoRows {
		// No neighbour is the normal case at either end of the archive.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("adjacent post: %w", err)
	}
	return p, nil
}

// ChangeKind moves a record between the pages and the archive.
//
// Something written as a page often turns out to be an entry, and rewriting it
// under a new address would break every link to it.
func (s *Store) ChangeKind(ctx context.Context, id int64, kind string) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE pages SET kind = $1, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
		 version = version + 1 WHERE id = $2`, NormalizeKind(kind), id)
	if err != nil {
		return fmt.Errorf("change kind: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("change kind: page %d not found", id)
	}
	return nil
}
