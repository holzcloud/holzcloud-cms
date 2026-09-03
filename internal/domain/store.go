package domain

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
)

const timeLayout = "2006-01-02T15:04:05Z"

// Store handles SQL operations for websites and domains.
type Store struct {
	DB *db.DB
}

// NewStore creates a new domain store.
func NewStore(database *db.DB) *Store {
	return &Store{DB: database}
}

// websiteFields is the single source of the website projection. Every read path
// goes through websiteColumns, so a column added to the model cannot be
// forgotten in one query and silently scan as its zero value.
var websiteFields = []string{
	"id", "name", "description", "active", "created_at", "updated_at",
	"locale", "extra_locales", "timezone", "meta_description", "favicon_media_id", "logo_media_id",
	"canonical_redirect", "offline_mode", "offline_message",
	"blog_base", "posts_per_page", "contact_email", "notify_email",
	"org_type", "street", "postal_code", "city", "country", "phone", "opening_hours",
	"token_ink", "token_paper", "token_brand", "token_font", "token_measure", "token_radius",
	"shop_base", "currency", "shipping_gross", "shipping_free_from", "shipping_tax_bp",
	"price_display", "vat_exempt", "vat_number", "return_policy",
	"order_email", "payment_details",
}

// websiteColumns renders the projection, optionally qualified with a table
// alias. The alias is required in the domain join: website_domains also has id
// and created_at, so unqualified names there are ambiguous.
func websiteColumns(alias string) string {
	if alias == "" {
		return strings.Join(websiteFields, ", ")
	}
	qualified := make([]string, len(websiteFields))
	for i, f := range websiteFields {
		qualified[i] = alias + "." + f
	}
	return strings.Join(qualified, ", ")
}

func scanWebsite(row interface{ Scan(...any) error }) (*Website, error) {
	var w Website
	var active, canonicalRedirect int
	var createdAt, updatedAt string
	var faviconID, logoID, shippingFreeFrom sql.NullInt64
	var vatExempt int
	err := row.Scan(&w.ID, &w.Name, &w.Description, &active, &createdAt, &updatedAt,
		&w.Locale, &w.ExtraLocales, &w.TimeZone, &w.MetaDescription, &faviconID, &logoID,
		&canonicalRedirect, &w.OfflineMode, &w.OfflineMessage,
		&w.BlogBase, &w.PostsPerPage, &w.ContactEmail, &w.NotifyEmail,
		&w.OrgType, &w.Street, &w.PostalCode, &w.City, &w.Country, &w.Phone, &w.OpeningHours,
		&w.TokenInk, &w.TokenPaper, &w.TokenBrand, &w.TokenFont, &w.TokenMeasure, &w.TokenRadius,
		&w.ShopBase, &w.Currency, &w.ShippingGross, &shippingFreeFrom, &w.ShippingTaxBP,
		&w.PriceDisplay, &vatExempt, &w.VATNumber, &w.ReturnPolicy,
		&w.OrderEmail, &w.PaymentDetails)
	if err != nil {
		return nil, err
	}
	w.Active = active == 1
	w.CanonicalRedirect = canonicalRedirect == 1
	w.VATExempt = vatExempt == 1
	if shippingFreeFrom.Valid {
		w.ShippingFreeFrom = &shippingFreeFrom.Int64
	}
	if faviconID.Valid {
		w.FaviconMediaID = &faviconID.Int64
	}
	if logoID.Valid {
		w.LogoMediaID = &logoID.Int64
	}
	w.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	w.UpdatedAt, _ = time.Parse(timeLayout, updatedAt)
	return &w, nil
}

// ListWebsites returns all websites.
func (s *Store) ListWebsites(ctx context.Context) ([]Website, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+websiteColumns("")+` FROM websites ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list websites: %w", err)
	}
	defer rows.Close()

	var websites []Website
	for rows.Next() {
		w, err := scanWebsite(rows)
		if err != nil {
			return nil, fmt.Errorf("scan website: %w", err)
		}
		websites = append(websites, *w)
	}
	return websites, rows.Err()
}

// GetWebsite returns a single website by ID.
func (s *Store) GetWebsite(ctx context.Context, id int64) (*Website, error) {
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT `+websiteColumns("")+` FROM websites WHERE id = $1`, id)
	w, err := scanWebsite(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get website: %w", err)
	}
	return w, nil
}

// CreateWebsite inserts a new website and returns it.
func (s *Store) CreateWebsite(ctx context.Context, name, description string) (*Website, error) {
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO websites (name, description) VALUES ($1, $2)`, name, description)
	if err != nil {
		return nil, fmt.Errorf("create website: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetWebsite(ctx, id)
}

// UpdateWebsite updates an existing website.
func (s *Store) UpdateWebsite(ctx context.Context, id int64, name, description string, active bool) error {
	activeInt := 0
	if active {
		activeInt = 1
	}
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE websites SET name = $1, description = $2, active = $3, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = $4`,
		name, description, activeInt, id)
	if err != nil {
		return fmt.Errorf("update website: %w", err)
	}
	return nil
}

// Settings are the website options that have no effect on routing and are
// edited together on the settings screen.
type Settings struct {
	Locale string
	// ExtraLocales are the further languages, as the operator typed them. The
	// store keeps them cleaned, so what comes back out is a list that can be
	// trusted.
	ExtraLocales      string
	TimeZone          string
	MetaDescription   string
	FaviconMediaID    *int64
	LogoMediaID       *int64
	CanonicalRedirect bool
	OfflineMode       string
	OfflineMessage    string
	BlogBase          string
	PostsPerPage      int
	ContactEmail      string
	NotifyEmail       string
	OrgType           string
	Street            string
	PostalCode        string
	City              string
	Country           string
	Phone             string
	OpeningHours      string
}

// UpdateSettings stores the website options.
//
// Validation lives in the handler; the store writes what it is given, except
// that it will not accept an unknown offline mode, which the CHECK constraint
// would otherwise turn into an opaque SQL error.
func (s *Store) UpdateSettings(ctx context.Context, id int64, set Settings) error {
	if set.OfflineMode != "notfound" && set.OfflineMode != "maintenance" {
		set.OfflineMode = "notfound"
	}
	if set.PostsPerPage < 1 || set.PostsPerPage > 100 {
		set.PostsPerPage = 10
	}
	set.Locale = locale.Normalise(set.Locale)
	if !locale.Valid(set.Locale) {
		set.Locale = locale.Default
	}
	// Cleaned on the way in: everything downstream — the address prefix, the
	// language switcher, the check on a page's address — reads this list, and
	// none of them should have to wonder whether it is trustworthy.
	set.ExtraLocales = locale.JoinList(locale.ParseList(set.ExtraLocales, set.Locale))

	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE websites SET locale = $1, timezone = $2, meta_description = $3,
		 favicon_media_id = $4, logo_media_id = $5, canonical_redirect = $6,
		 offline_mode = $7, offline_message = $8,
		 blog_base = $9, posts_per_page = $10, contact_email = $11,
		 notify_email = $12,
		 org_type = $13, street = $14, postal_code = $15, city = $16,
		 country = $17, phone = $18, opening_hours = $19, extra_locales = $20,
		 updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		 WHERE id = $21`,
		set.Locale, set.TimeZone, set.MetaDescription,
		nullableID(set.FaviconMediaID), nullableID(set.LogoMediaID),
		boolToInt(set.CanonicalRedirect), set.OfflineMode, set.OfflineMessage,
		set.BlogBase, set.PostsPerPage, set.ContactEmail, set.NotifyEmail,
		set.OrgType, set.Street, set.PostalCode, set.City, set.Country,
		set.Phone, set.OpeningHours, set.ExtraLocales, id)
	if err != nil {
		return fmt.Errorf("update website settings: %w", err)
	}
	return nil
}

func nullableID(id *int64) any {
	if id == nil {
		return nil
	}
	return *id
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// DeleteWebsite deletes a website by ID.
func (s *Store) DeleteWebsite(ctx context.Context, id int64) error {
	_, err := s.DB.Write.ExecContext(ctx, `DELETE FROM websites WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete website: %w", err)
	}
	return nil
}

// ListDomains returns all domains for a website.
func (s *Store) ListDomains(ctx context.Context, websiteID int64) ([]Domain, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, domain, is_primary, created_at FROM website_domains WHERE website_id = $1 ORDER BY is_primary DESC, domain`,
		websiteID)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}
	defer rows.Close()

	var domains []Domain
	for rows.Next() {
		var d Domain
		var isPrimary int
		var createdAt string
		if err := rows.Scan(&d.ID, &d.WebsiteID, &d.Domain, &isPrimary, &createdAt); err != nil {
			return nil, fmt.Errorf("scan domain: %w", err)
		}
		d.IsPrimary = isPrimary == 1
		d.CreatedAt, _ = time.Parse(timeLayout, createdAt)
		domains = append(domains, d)
	}
	return domains, rows.Err()
}

// AddDomain adds a domain to a website. The name is normalised first, so what
// is stored is exactly what the resolver compares the Host header against.
func (s *Store) AddDomain(ctx context.Context, websiteID int64, domain string, isPrimary bool) (*Domain, error) {
	domain, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}

	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	// The partial unique index allows one primary per website, so an existing
	// one has to be cleared first. Without this the insert would simply fail and
	// the admin would see a constraint error instead of the flag moving.
	if isPrimary {
		if err := clearPrimary(ctx, tx, websiteID); err != nil {
			return nil, err
		}
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO website_domains (website_id, domain, is_primary) VALUES ($1, $2, $3)`,
		websiteID, domain, boolToInt(isPrimary))
	if err != nil {
		return nil, fmt.Errorf("add domain: %w", err)
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &Domain{ID: id, WebsiteID: websiteID, Domain: domain, IsPrimary: isPrimary}, nil
}

// SetPrimaryDomain makes one domain the canonical one for its website.
func (s *Store) SetPrimaryDomain(ctx context.Context, websiteID, domainID int64) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	if err := clearPrimary(ctx, tx, websiteID); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE website_domains SET is_primary = 1 WHERE id = $1 AND website_id = $2`,
		domainID, websiteID)
	if err != nil {
		return fmt.Errorf("set primary domain: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func clearPrimary(ctx context.Context, tx *sql.Tx, websiteID int64) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE website_domains SET is_primary = 0 WHERE website_id = $1 AND is_primary = 1`,
		websiteID); err != nil {
		return fmt.Errorf("clear primary domain: %w", err)
	}
	return nil
}

// PrimaryDomain returns the canonical host of a website, or "" when none is
// marked. It is what canonical URLs and the optional redirect are built from.
func (s *Store) PrimaryDomain(ctx context.Context, websiteID int64) (string, error) {
	var host string
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT domain FROM website_domains WHERE website_id = $1 AND is_primary = 1`,
		websiteID).Scan(&host)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("primary domain: %w", err)
	}
	return host, nil
}

// RemoveDomain deletes a domain by ID.
func (s *Store) RemoveDomain(ctx context.Context, domainID int64) error {
	_, err := s.DB.Write.ExecContext(ctx, `DELETE FROM website_domains WHERE id = $1`, domainID)
	if err != nil {
		return fmt.Errorf("remove domain: %w", err)
	}
	return nil
}

// LookupDomain resolves a hostname to a website via the website_domains table.
// Returns nil if not found or website is not active.
func (s *Store) LookupDomain(ctx context.Context, host string) (*Website, error) {
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT `+websiteColumns("w")+`
		 FROM websites w
		 JOIN website_domains wd ON wd.website_id = w.id
		 WHERE wd.domain = $1 COLLATE NOCASE`,
		host)
	w, err := scanWebsite(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup domain: %w", err)
	}
	return w, nil
}

// DesignTokens is the per-website design override.
type DesignTokens struct {
	Ink     string
	Paper   string
	Brand   string
	Font    string
	Measure int
	Radius  int
}

// UpdateDesignTokens stores the design overrides.
//
// Separate from UpdateSettings because it is a separate screen: the design
// choices and the address of the business have nothing to do with each other,
// and one form that saved both would make every colour change rewrite the
// opening hours.
func (s *Store) UpdateDesignTokens(ctx context.Context, id int64, t DesignTokens) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE websites SET token_ink = $1, token_paper = $2, token_brand = $3,
		 token_font = $4, token_measure = $5, token_radius = $6,
		 updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		 WHERE id = $7`,
		t.Ink, t.Paper, t.Brand, t.Font, t.Measure, t.Radius, id)
	if err != nil {
		return fmt.Errorf("update design tokens: %w", err)
	}
	return nil
}
