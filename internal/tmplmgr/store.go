package tmplmgr

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

const timeLayout = "2006-01-02T15:04:05Z"

// Store handles SQL operations for templates and website-template associations.
type Store struct {
	DB      *db.DB
	DataDir string
}

// NewStore creates a new template store.
func NewStore(database *db.DB, dataDir string) *Store {
	return &Store{DB: database, DataDir: dataDir}
}

func scanTemplate(row interface{ Scan(...any) error }) (*Template, error) {
	var t Template
	var createdAt string
	var builtin int
	err := row.Scan(&t.ID, &t.Name, &t.Slug, &builtin, &createdAt)
	if err != nil {
		return nil, err
	}
	t.IsBuiltin = builtin == 1
	t.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	return &t, nil
}

// Create inserts a new template record.
func (s *Store) Create(ctx context.Context, name, slug string) (*Template, error) {
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO templates (name, slug) VALUES ($1, $2)`, name, slug)
	if err != nil {
		return nil, fmt.Errorf("create template: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetByID(ctx, id)
}

// CreateBuiltin inserts a built-in template record (or marks existing as built-in).
func (s *Store) CreateBuiltin(ctx context.Context, name, slug string) (*Template, error) {
	existing, err := s.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		// Ensure it's marked as built-in
		if !existing.IsBuiltin {
			_, err := s.DB.Write.ExecContext(ctx,
				`UPDATE templates SET is_builtin = 1 WHERE id = $1`, existing.ID)
			if err != nil {
				return nil, fmt.Errorf("mark builtin: %w", err)
			}
			existing.IsBuiltin = true
		}
		return existing, nil
	}
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO templates (name, slug, is_builtin) VALUES ($1, $2, 1)`, name, slug)
	if err != nil {
		return nil, fmt.Errorf("create builtin template: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetByID(ctx, id)
}

// GetBySlug returns a template by slug.
func (s *Store) GetBySlug(ctx context.Context, slug string) (*Template, error) {
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, name, slug, is_builtin, created_at FROM templates WHERE slug = $1`, slug)
	t, err := scanTemplate(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get template by slug: %w", err)
	}
	return t, nil
}

// GetByID returns a template by ID.
func (s *Store) GetByID(ctx context.Context, id int64) (*Template, error) {
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, name, slug, is_builtin, created_at FROM templates WHERE id = $1`, id)
	t, err := scanTemplate(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get template by id: %w", err)
	}
	return t, nil
}

// List returns all templates.
func (s *Store) List(ctx context.Context) ([]Template, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, name, slug, is_builtin, created_at FROM templates ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	var templates []Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan template: %w", err)
		}
		templates = append(templates, *t)
	}
	return templates, rows.Err()
}

// Delete removes a template if it is not active for any website.
func (s *Store) Delete(ctx context.Context, id int64) error {
	active, err := s.IsActiveAnywhere(ctx, id)
	if err != nil {
		return err
	}
	if active {
		return fmt.Errorf("cannot delete template: still active for at least one website")
	}

	// Get slug for disk cleanup
	t, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("template not found")
	}

	_, err = s.DB.Write.ExecContext(ctx, `DELETE FROM website_templates WHERE template_id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete website_templates: %w", err)
	}
	_, err = s.DB.Write.ExecContext(ctx, `DELETE FROM templates WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}

	// Remove disk files
	diskPath := filepath.Join(s.DataDir, "templates", t.Slug)
	_ = os.RemoveAll(diskPath)

	return nil
}

// ActivateForWebsite sets a template as the active template for a website.
// Deactivates any previously active template for that website.
func (s *Store) ActivateForWebsite(ctx context.Context, websiteID, templateID int64) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Deactivate all for this website
	_, err = tx.ExecContext(ctx,
		`UPDATE website_templates SET is_active = 0 WHERE website_id = $1`, websiteID)
	if err != nil {
		return fmt.Errorf("deactivate templates: %w", err)
	}

	// Upsert the activation
	_, err = tx.ExecContext(ctx,
		`INSERT INTO website_templates (website_id, template_id, is_active)
		 VALUES ($1, $2, 1)
		 ON CONFLICT (website_id, template_id) DO UPDATE SET is_active = 1`,
		websiteID, templateID)
	if err != nil {
		return fmt.Errorf("activate template: %w", err)
	}

	return tx.Commit()
}

// DeactivateForWebsite removes the active flag for a template on a website.
func (s *Store) DeactivateForWebsite(ctx context.Context, websiteID, templateID int64) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE website_templates SET is_active = 0 WHERE website_id = $1 AND template_id = $2`,
		websiteID, templateID)
	if err != nil {
		return fmt.Errorf("deactivate template: %w", err)
	}
	return nil
}

// ActiveTemplateSlug returns the slug of the active template for a website.
// Returns empty string if no template is active.
func (s *Store) ActiveTemplateSlug(ctx context.Context, websiteID int64) (string, error) {
	var slug string
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT t.slug FROM templates t
		 JOIN website_templates wt ON wt.template_id = t.id
		 WHERE wt.website_id = $1 AND wt.is_active = 1`,
		websiteID).Scan(&slug)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("active template slug: %w", err)
	}
	return slug, nil
}

// ListForWebsite returns all template associations for a website.
func (s *Store) ListForWebsite(ctx context.Context, websiteID int64) ([]WebsiteTemplate, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT wt.id, wt.website_id, wt.template_id, wt.is_active, wt.created_at,
		        t.name, t.slug
		 FROM website_templates wt
		 JOIN templates t ON t.id = wt.template_id
		 WHERE wt.website_id = $1
		 ORDER BY t.name`,
		websiteID)
	if err != nil {
		return nil, fmt.Errorf("list website templates: %w", err)
	}
	defer rows.Close()

	var wts []WebsiteTemplate
	for rows.Next() {
		var wt WebsiteTemplate
		var active int
		var createdAt string
		err := rows.Scan(&wt.ID, &wt.WebsiteID, &wt.TemplateID, &active, &createdAt,
			&wt.TemplateName, &wt.TemplateSlug)
		if err != nil {
			return nil, fmt.Errorf("scan website template: %w", err)
		}
		wt.IsActive = active == 1
		wt.CreatedAt, _ = time.Parse(timeLayout, createdAt)
		wts = append(wts, wt)
	}
	return wts, rows.Err()
}

// ActiveByWebsite returns the active template ID for every website that has one,
// keyed by website ID.
//
// It replaces a per-template, per-website lookup: the template list previously
// issued one query for each combination, so a handful of sites and themes turned
// into dozens of round trips per page view.
func (s *Store) ActiveByWebsite(ctx context.Context) (map[int64]int64, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT website_id, template_id FROM website_templates WHERE is_active = 1`)
	if err != nil {
		return nil, fmt.Errorf("list active templates: %w", err)
	}
	defer rows.Close()

	active := make(map[int64]int64)
	for rows.Next() {
		var websiteID, templateID int64
		if err := rows.Scan(&websiteID, &templateID); err != nil {
			return nil, fmt.Errorf("scan active template: %w", err)
		}
		active[websiteID] = templateID
	}
	return active, rows.Err()
}

// IsActiveAnywhere checks if a template is active for any website.
func (s *Store) IsActiveAnywhere(ctx context.Context, templateID int64) (bool, error) {
	var count int
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM website_templates WHERE template_id = $1 AND is_active = 1`,
		templateID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check active anywhere: %w", err)
	}
	return count > 0, nil
}
