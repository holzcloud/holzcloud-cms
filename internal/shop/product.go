// Package shop is the catalogue: products, their prices and what a visitor is
// allowed to see of them.
package shop

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/money"
)

const timeLayout = "2006-01-02T15:04:05Z"

// Status values a product can have.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// ErrSlugTaken is returned when a website already has a product at that path.
var ErrSlugTaken = errors.New("es gibt bereits ein Produkt mit dieser Adresse")

// ErrNotFound says the product does not exist, or exists but belongs to some
// other website. The two are deliberately the same answer: telling a caller
// apart from "wrong website" would confirm that the id names something real
// somewhere else, which is the one fact a foreign caller must not learn.
var ErrNotFound = errors.New("dieses Produkt gibt es auf dieser Website nicht")

// Product is one item in the catalogue.
type Product struct {
	ID        int64
	WebsiteID int64
	Slug      string
	Title     string
	Subtitle  string

	DescriptionMarkdown string
	DescriptionHTML     string
	Excerpt             string
	SKU                 string

	// PriceGross is the advertised price, in rappen. See the migration for why
	// the gross figure is the stored one.
	PriceGross money.Amount
	TaxRate    money.TaxRate

	// Stock is nil when the operator does not track quantities. Zero means sold
	// out, which is a different thing entirely.
	Stock           *int
	WeightGrams     int
	DeliveryNote    string
	Status          string
	FeaturedMediaID *int64
	Position        int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// IsPublished reports whether the product may be shown to a visitor.
func (p *Product) IsPublished() bool { return p.Status == StatusPublished }

// Orderable reports whether the product can go into a basket right now.
//
// Untracked stock is always orderable: a workshop that builds to order has no
// quantity, and refusing the sale because a column is NULL would be the CMS
// inventing a rule the operator never set.
func (p *Product) Orderable() bool {
	if !p.IsPublished() {
		return false
	}
	return p.Stock == nil || *p.Stock > 0
}

// Store handles SQL operations for products.
type Store struct {
	DB *db.DB
	// now is injectable so the tests can pin timestamps.
	now func() time.Time
}

// NewStore creates a product store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

func (s *Store) clock() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

// productColumns is the projection every scan expects, in order.
const productColumns = `id, website_id, slug, title, subtitle,
	description_markdown, description_html, excerpt, sku,
	price_gross, tax_bp, stock, weight_grams, delivery_note,
	status, featured_media_id, position, created_at, updated_at`

// scanProduct reads one product row. extra receives destinations for columns a
// caller appended to the projection — the basket adds its quantity that way
// rather than running a second query per line.
func scanProduct(row interface{ Scan(...any) error }, extra ...any) (*Product, error) {
	var p Product
	var stock, featured sql.NullInt64
	var createdAt, updatedAt string
	var taxBP int

	dest := []any{
		&p.ID, &p.WebsiteID, &p.Slug, &p.Title, &p.Subtitle,
		&p.DescriptionMarkdown, &p.DescriptionHTML, &p.Excerpt, &p.SKU,
		&p.PriceGross, &taxBP, &stock, &p.WeightGrams, &p.DeliveryNote,
		&p.Status, &featured, &p.Position, &createdAt, &updatedAt,
	}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return nil, err
	}

	p.TaxRate = money.TaxRate(taxBP)
	if stock.Valid {
		v := int(stock.Int64)
		p.Stock = &v
	}
	if featured.Valid {
		p.FeaturedMediaID = &featured.Int64
	}
	p.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	p.UpdatedAt, _ = time.Parse(timeLayout, updatedAt)
	return &p, nil
}

func scanProducts(rows *sql.Rows) ([]*Product, error) {
	defer rows.Close()
	var out []*Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Create stores a new product.
func (s *Store) Create(ctx context.Context, p *Product) (int64, error) {
	now := s.clock().Format(timeLayout)

	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO products (website_id, slug, title, subtitle,
			description_markdown, description_html, excerpt, sku,
			price_gross, tax_bp, stock, weight_grams, delivery_note,
			status, featured_media_id, position, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		p.WebsiteID, p.Slug, p.Title, p.Subtitle,
		p.DescriptionMarkdown, p.DescriptionHTML, p.Excerpt, p.SKU,
		int64(p.PriceGross), int(p.TaxRate), nullableInt(p.Stock), p.WeightGrams, p.DeliveryNote,
		p.Status, nullableID(p.FeaturedMediaID), p.Position, now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrSlugTaken
		}
		return 0, fmt.Errorf("create product: %w", err)
	}
	return res.LastInsertId()
}

// Update writes a product back, and only into the website that owns it.
//
// websiteID is a parameter rather than a field read off p because p arrives
// from a form: an admin handler builds it from what was posted and sets
// WebsiteID to whatever the address said. Trusting that field would make the
// scope self-asserted by the very request being checked. The caller must hand
// over the website it has already had authorised, and the WHERE clause holds
// it against the row.
//
// The condition is in SQL rather than a Get-then-compare in Go for two
// reasons. It is one statement, so nothing can move the row between the check
// and the write; and it cannot be skipped by the next caller, which is exactly
// how this became a cross-website write in the first place — see
// internal/admin/product_scope_test.go.
func (s *Store) Update(ctx context.Context, websiteID int64, p *Product) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE products SET slug=$1, title=$2, subtitle=$3,
			description_markdown=$4, description_html=$5, excerpt=$6, sku=$7,
			price_gross=$8, tax_bp=$9, stock=$10, weight_grams=$11, delivery_note=$12,
			status=$13, featured_media_id=$14, position=$15, updated_at=$16
		 WHERE id=$17 AND website_id=$18`,
		p.Slug, p.Title, p.Subtitle,
		p.DescriptionMarkdown, p.DescriptionHTML, p.Excerpt, p.SKU,
		int64(p.PriceGross), int(p.TaxRate), nullableInt(p.Stock), p.WeightGrams, p.DeliveryNote,
		p.Status, nullableID(p.FeaturedMediaID), p.Position, s.clock().Format(timeLayout),
		p.ID, websiteID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrSlugTaken
		}
		return fmt.Errorf("update product: %w", err)
	}
	// Silence would be the dangerous answer: the handler would flash "saved"
	// and redirect over a write that never happened.
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a product from the website that owns it.
//
// Scoped for the same reason as Update, even though today's only caller checks
// the ownership itself first. A delete is the one that cannot be undone, so it
// is the last place to rely on every future caller remembering.
func (s *Store) Delete(ctx context.Context, websiteID, id int64) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM products WHERE id = $1 AND website_id = $2`, id, websiteID)
	if err != nil {
		return fmt.Errorf("delete product: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Get reads one product by id, regardless of status. For the admin.
func (s *Store) Get(ctx context.Context, id int64) (*Product, error) {
	p, err := scanProduct(s.DB.Read.QueryRowContext(ctx,
		`SELECT `+productColumns+` FROM products WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// List returns every product of a website, drafts included. For the admin.
func (s *Store) List(ctx context.Context, websiteID int64) ([]*Product, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+productColumns+` FROM products WHERE website_id = $1
		 ORDER BY position, id`, websiteID)
	if err != nil {
		return nil, err
	}
	return scanProducts(rows)
}

// GetPublished reads one published product by its path.
//
// The status condition is in the query rather than in a check afterwards. A
// draft that is filtered out in Go is a draft that leaks the moment someone
// adds a second call site and forgets the check — the same rule the page store
// follows for exactly that reason.
func (s *Store) GetPublished(ctx context.Context, websiteID int64, slug string) (*Product, error) {
	p, err := scanProduct(s.DB.Read.QueryRowContext(ctx,
		`SELECT `+productColumns+` FROM products
		 WHERE website_id = $1 AND slug = $2 AND status = 'published'`,
		websiteID, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// ListPublished returns the visible catalogue, in the operator's order.
func (s *Store) ListPublished(ctx context.Context, websiteID int64, limit, offset int) ([]*Product, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+productColumns+` FROM products
		 WHERE website_id = $1 AND status = 'published'
		 ORDER BY position, id
		 LIMIT $2 OFFSET $3`, websiteID, limit, offset)
	if err != nil {
		return nil, err
	}
	return scanProducts(rows)
}

// CountPublished is the total for the pager.
func (s *Store) CountPublished(ctx context.Context, websiteID int64) (int, error) {
	var n int
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM products WHERE website_id = $1 AND status = 'published'`,
		websiteID).Scan(&n)
	return n, err
}

// ListPublishedByTerm is the category listing.
func (s *Store) ListPublishedByTerm(ctx context.Context, websiteID, termID int64, limit, offset int) ([]*Product, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+prefixed("p")+` FROM products p
		 JOIN product_terms pt ON pt.product_id = p.id
		 WHERE p.website_id = $1 AND p.status = 'published' AND pt.term_id = $2
		 ORDER BY p.position, p.id
		 LIMIT $3 OFFSET $4`, websiteID, termID, limit, offset)
	if err != nil {
		return nil, err
	}
	return scanProducts(rows)
}

// CountPublishedByTerm is the total for a category pager.
func (s *Store) CountPublishedByTerm(ctx context.Context, websiteID, termID int64) (int, error) {
	var n int
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM products p
		 JOIN product_terms pt ON pt.product_id = p.id
		 WHERE p.website_id = $1 AND p.status = 'published' AND pt.term_id = $2`,
		websiteID, termID).Scan(&n)
	return n, err
}

// prefixed qualifies the projection for a joined query.
func prefixed(alias string) string {
	cols := strings.Split(productColumns, ",")
	for i, c := range cols {
		cols[i] = alias + "." + strings.TrimSpace(c)
	}
	return strings.Join(cols, ", ")
}

// SetTerms replaces a product's categories.
func (s *Store) SetTerms(ctx context.Context, productID int64, termIDs []int64) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM product_terms WHERE product_id = $1`, productID); err != nil {
		return err
	}
	for _, id := range termIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO product_terms (product_id, term_id) VALUES ($1, $2)`,
			productID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// TermIDs returns a product's categories.
func (s *Store) TermIDs(ctx context.Context, productID int64) ([]int64, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT term_id FROM product_terms WHERE product_id = $1`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetGallery replaces a product's extra pictures.
//
// The website is a parameter although product_media has no website_id column,
// and that is exactly why. This is the third table in the tree whose rows are
// addressed by a second id alone — page_terms, menu_items and this one — and
// the other two are where this codebase has already shipped the same defect:
// an id out of a form, a DELETE with no website predicate, and rows whose two
// halves end up on different websites.
//
// It has never fired here, for the one reason that is not a reason: nothing
// calls SetGallery yet. The guard is written before the first caller rather
// than after the first report, and the product is looked up through the
// website so the caller cannot pass a foreign one.
func (s *Store) SetGallery(ctx context.Context, websiteID, productID int64, mediaIDs []int64) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var owned bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM products WHERE id = $1 AND website_id = $2)`,
		productID, websiteID).Scan(&owned); err != nil {
		return err
	}
	if !owned {
		return ErrNotFound
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM product_media WHERE product_id = $1`, productID); err != nil {
		return err
	}
	for i, id := range mediaIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO product_media (product_id, media_id, position) VALUES ($1, $2, $3)`,
			productID, id, i); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GalleryIDs returns a product's extra pictures in order.
//
// No website here, and the asymmetry is deliberate rather than forgotten: the
// caller has already resolved the product through its website to have its id
// at all, and a read of a picture list leaks nothing a foreign product's own
// screen would not. The write is the one that can put two websites into one
// row, so the write is the one that carries the check.
func (s *Store) GalleryIDs(ctx context.Context, productID int64) ([]int64, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT media_id FROM product_media WHERE product_id = $1 ORDER BY position`,
		productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// AdjustStock decrements the tracked quantity, refusing to go below zero.
//
// The guard is in the WHERE clause, not in Go: two orders for the last item
// arriving at the same moment would both read stock = 1 and both write 0, and
// the second sale would be one nobody can ship.
func (s *Store) AdjustStock(ctx context.Context, productID int64, by int) (bool, error) {
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE products SET stock = stock + $1
		 WHERE id = $2 AND stock IS NOT NULL AND stock + $1 >= 0`,
		by, productID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableID(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}
