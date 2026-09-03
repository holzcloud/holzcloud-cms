package term

import (
	"context"
	"fmt"

	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// ListTagged returns one page of the publicly visible content carrying a label,
// newest first, plus the total.
//
// Pages and posts both appear: a label describes a subject, and a visitor
// following "Möbel" wants everything about it, not only the dated entries.
func (s *Store) ListTagged(ctx context.Context, websiteID, termID int64, pageNum, perPage int) ([]page.Page, int, error) {
	return s.ListTaggedIn(ctx, websiteID, termID, "", pageNum, perPage)
}

// ListTaggedIn is the same for one language.
//
// The labels themselves are shared across languages — a label is a subject, and
// a subject is the same subject in French. What is listed under it is not: at
// /fr/tag/moebel a visitor gets the French content carrying that label.
func (s *Store) ListTaggedIn(ctx context.Context, websiteID, termID int64, loc string, pageNum, perPage int) ([]page.Page, int, error) {
	if perPage <= 0 {
		perPage = 10
	}
	if pageNum <= 0 {
		pageNum = 1
	}

	var total int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pages p
		 JOIN page_terms pt ON pt.page_id = p.id
		 WHERE p.website_id = $1 AND pt.term_id = $2 AND p.locale = $3`+page.ListablePredicateFor("p"),
		websiteID, termID, loc).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tagged: %w", err)
	}

	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+page.ColumnsFor("p")+` FROM pages p
		 JOIN page_terms pt ON pt.page_id = p.id
		 WHERE p.website_id = $1 AND pt.term_id = $2 AND p.locale = $5`+page.ListablePredicateFor("p")+`
		 ORDER BY COALESCE(p.published_at, p.created_at) DESC, p.id DESC
		 LIMIT $3 OFFSET $4`,
		websiteID, termID, perPage, (pageNum-1)*perPage, loc)
	if err != nil {
		return nil, 0, fmt.Errorf("list tagged: %w", err)
	}
	defer rows.Close()

	items, err := page.ScanRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
