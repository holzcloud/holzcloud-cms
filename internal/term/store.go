// Package term stores the free-form labels that group content across menus.
//
// Labels stay in the core, unlike the search or the contact form. They look
// optional and are not: a label is part of how a page is written, not a feature
// beside it. The field sits in the editor, the values reach every theme through
// Page.Terms and Site.Terms, the archive groups by them, and the export carries
// them. A plugin can reach none of those four places, so moving labels out
// would mean assigning them on a separate screen and showing them nowhere —
// worse than what exists, in exchange for a line in a manifest.
package term

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// Term is one label.
type Term struct {
	ID   int64
	Slug string
	Name string
	// Count is how many published items carry the label. It is only filled by
	// the listing queries that ask for it.
	Count int
}

// URL is the label's public archive address.
func (t Term) URL() string { return "/tag/" + t.Slug }

// Store handles SQL operations for terms.
type Store struct {
	DB *db.DB
}

// NewStore creates a new term store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

// MaxPerPage bounds how many labels one item may carry.
//
// Not a database constraint but an editorial one: thirty labels on one entry
// describe nothing, and the field is the kind that invites them.
const MaxPerPage = 12

// MaxNameLength bounds a single label.
const MaxNameLength = 60

// Parse reads the comma-separated field an editor types.
//
// Duplicates that differ only in case or spacing are folded together — "Möbel"
// and "möbel" are one label, and storing both would split an archive in two
// with no visible reason.
func Parse(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		name := strings.Join(strings.Fields(part), " ")
		if name == "" {
			continue
		}
		if len([]rune(name)) > MaxNameLength {
			name = string([]rune(name)[:MaxNameLength])
		}
		slug := page.Slugify(name)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		out = append(out, name)
		if len(out) == MaxPerPage {
			break
		}
	}
	return out
}

// Format renders a page's labels back into the editor's field.
func Format(terms []Term) string {
	names := make([]string, len(terms))
	for i, t := range terms {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}

// SetForPage replaces the labels of one item.
//
// Everything happens in one transaction: a half-applied change would leave the
// entry showing labels it no longer has and missing ones it does.
func (s *Store) SetForPage(ctx context.Context, websiteID, pageID int64, names []string) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin set terms: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM page_terms WHERE page_id = $1`, pageID); err != nil {
		return fmt.Errorf("clear terms: %w", err)
	}

	for _, name := range names {
		slug := page.Slugify(name)
		if slug == "" {
			continue
		}
		// The label may already exist under a different spelling of the same
		// slug; the existing name wins, so an archive does not rename itself
		// because someone typed it in lower case once.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO terms (website_id, slug, name) VALUES ($1, $2, $3)
			 ON CONFLICT (website_id, slug) DO NOTHING`, websiteID, slug, name); err != nil {
			return fmt.Errorf("create term %q: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO page_terms (page_id, term_id)
			 SELECT $1, id FROM terms WHERE website_id = $2 AND slug = $3`,
			pageID, websiteID, slug); err != nil {
			return fmt.Errorf("attach term %q: %w", name, err)
		}
	}
	return tx.Commit()
}

// ForPage returns the labels of one item, alphabetically.
func (s *Store) ForPage(ctx context.Context, pageID int64) ([]Term, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT t.id, t.slug, t.name FROM terms t
		 JOIN page_terms pt ON pt.term_id = t.id
		 WHERE pt.page_id = $1 ORDER BY t.name COLLATE NOCASE`, pageID)
	if err != nil {
		return nil, fmt.Errorf("list terms for page: %w", err)
	}
	defer rows.Close()

	var terms []Term
	for rows.Next() {
		var t Term
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name); err != nil {
			return nil, fmt.Errorf("scan term: %w", err)
		}
		terms = append(terms, t)
	}
	return terms, rows.Err()
}

// ForPages returns the labels of several items at once, keyed by page id.
//
// One query rather than one per row: an archive of ten entries would otherwise
// cost eleven round trips to show its labels.
func (s *Store) ForPages(ctx context.Context, ids []int64) (map[int64][]Term, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
		placeholders[i] = "$" + strconv.Itoa(i+1)
	}

	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT pt.page_id, t.id, t.slug, t.name FROM terms t
		 JOIN page_terms pt ON pt.term_id = t.id
		 WHERE pt.page_id IN (`+strings.Join(placeholders, ", ")+`)
		 ORDER BY t.name COLLATE NOCASE`, args...)
	if err != nil {
		return nil, fmt.Errorf("list terms for pages: %w", err)
	}
	defer rows.Close()

	out := map[int64][]Term{}
	for rows.Next() {
		var pageID int64
		var t Term
		if err := rows.Scan(&pageID, &t.ID, &t.Slug, &t.Name); err != nil {
			return nil, fmt.Errorf("scan term: %w", err)
		}
		out[pageID] = append(out[pageID], t)
	}
	return out, rows.Err()
}

// GetBySlug returns one label of a website, or nil.
func (s *Store) GetBySlug(ctx context.Context, websiteID int64, slug string) (*Term, error) {
	var t Term
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, slug, name FROM terms WHERE website_id = $1 AND slug = $2`,
		websiteID, slug).Scan(&t.ID, &t.Slug, &t.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get term: %w", err)
	}
	return &t, nil
}

// ListWithCounts returns the labels that have at least one publicly visible
// item, with how many.
//
// A label whose only entry is a draft must not appear: it would lead a visitor
// to an empty archive, and it would leak the fact that the draft exists.
func (s *Store) ListWithCounts(ctx context.Context, websiteID int64) ([]Term, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT t.id, t.slug, t.name, COUNT(p.id) AS n
		 FROM terms t
		 JOIN page_terms pt ON pt.term_id = t.id
		 JOIN pages p ON p.id = pt.page_id
		 WHERE t.website_id = $1`+page.ListablePredicateFor("p")+`
		 GROUP BY t.id ORDER BY n DESC, t.name COLLATE NOCASE`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("list terms: %w", err)
	}
	defer rows.Close()

	var terms []Term
	for rows.Next() {
		var t Term
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Count); err != nil {
			return nil, fmt.Errorf("scan term: %w", err)
		}
		terms = append(terms, t)
	}
	return terms, rows.Err()
}

// ListAll returns every label of a website with its total usage, including
// labels only draft content carries — this is the admin's view.
func (s *Store) ListAll(ctx context.Context, websiteID int64) ([]Term, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT t.id, t.slug, t.name,
		        (SELECT COUNT(*) FROM page_terms pt
		         JOIN pages p ON p.id = pt.page_id
		         WHERE pt.term_id = t.id AND p.deleted_at IS NULL) AS n
		 FROM terms t WHERE t.website_id = $1
		 ORDER BY t.name COLLATE NOCASE`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("list all terms: %w", err)
	}
	defer rows.Close()

	var terms []Term
	for rows.Next() {
		var t Term
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Count); err != nil {
			return nil, fmt.Errorf("scan term: %w", err)
		}
		terms = append(terms, t)
	}
	return terms, rows.Err()
}

// Delete removes a label. The rows in page_terms go with it through the
// cascade; the content itself is untouched.
func (s *Store) Delete(ctx context.Context, websiteID, id int64) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM terms WHERE id = $1 AND website_id = $2`, id, websiteID)
	if err != nil {
		return fmt.Errorf("delete term: %w", err)
	}
	return nil
}

// Rename changes the visible name of a label without moving its address.
//
// The slug stays: it is what links and search results point at, and renaming
// "Möbel" to "Möbelbau" should not break every link that already exists.
func (s *Store) Rename(ctx context.Context, websiteID, id int64, name string) error {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return fmt.Errorf("a label needs a name")
	}
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE terms SET name = $1 WHERE id = $2 AND website_id = $3`, name, id, websiteID)
	if err != nil {
		return fmt.Errorf("rename term: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("term %d not found", id)
	}
	return nil
}

// SetForProduct replaces a product's categories, creating labels that do not
// exist yet.
//
// A near-copy of SetForPage rather than a shared helper: the two differ only in
// the join table, and the alternative — passing a table name into a query
// builder — turns two readable statements into one that has to be trusted. The
// behaviour that matters is identical and deliberate: a label that already
// exists keeps its spelling, so a shop category does not rename itself because
// someone typed it in lower case once.
func (s *Store) SetForProduct(ctx context.Context, websiteID, productID int64, names []string) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin set product terms: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM product_terms WHERE product_id = $1`, productID); err != nil {
		return fmt.Errorf("clear product terms: %w", err)
	}

	for _, name := range names {
		slug := page.Slugify(name)
		if slug == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO terms (website_id, slug, name) VALUES ($1, $2, $3)
			 ON CONFLICT (website_id, slug) DO NOTHING`, websiteID, slug, name); err != nil {
			return fmt.Errorf("create term %q: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO product_terms (product_id, term_id)
			 SELECT $1, id FROM terms WHERE website_id = $2 AND slug = $3`,
			productID, websiteID, slug); err != nil {
			return fmt.Errorf("attach term %q: %w", name, err)
		}
	}
	return tx.Commit()
}
