package media

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

const timeLayout = "2006-01-02T15:04:05Z"

// Store handles SQL operations for media files.
type Store struct {
	DB *db.DB
}

// NewStore creates a new media store.
func NewStore(database *db.DB) *Store {
	return &Store{DB: database}
}

// columns is the single source of the media projection, so a field added to the
// model cannot be forgotten in one query and silently scan as its zero value.
const columns = `id, website_id, filename, original_name, mime_type, size_bytes, created_at, alt_text, caption, content_hash, width, height, crop_ratio, crop_zoom, crop_rotation, focus_x, focus_y, version`

func scan(row interface{ Scan(...any) error }) (*Media, error) {
	var m Media
	var createdAt string
	if err := row.Scan(&m.ID, &m.WebsiteID, &m.Filename, &m.OriginalName, &m.MimeType,
		&m.SizeBytes, &createdAt, &m.AltText, &m.Caption, &m.ContentHash,
		&m.Width, &m.Height,
		&m.Crop.Ratio, &m.Crop.Zoom, &m.Crop.Rotation, &m.Crop.FocusX, &m.Crop.FocusY,
		&m.Version); err != nil {
		return nil, err
	}
	m.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	return &m, nil
}

// Create inserts a new media record.
func (s *Store) Create(ctx context.Context, websiteID int64, filename, originalName, mimeType string, sizeBytes int64, contentHash string) (*Media, error) {
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO media (website_id, filename, original_name, mime_type, size_bytes, content_hash)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		websiteID, filename, originalName, mimeType, sizeBytes, contentHash)
	if err != nil {
		return nil, fmt.Errorf("create media: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetByID(ctx, id)
}

// UpdateMeta stores the description and caption of a file.
func (s *Store) UpdateMeta(ctx context.Context, id int64, altText, caption string) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE media SET alt_text = $1, caption = $2 WHERE id = $3`, altText, caption, id)
	if err != nil {
		return fmt.Errorf("update media meta: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// FindByHash returns an existing file with the same content, or nil.
//
// Uploading the same photo twice should say so rather than quietly filling the
// card with a second copy.
func (s *Store) FindByHash(ctx context.Context, websiteID int64, hash string) (*Media, error) {
	if hash == "" {
		return nil, nil
	}
	m, err := scan(s.DB.Read.QueryRowContext(ctx,
		`SELECT `+columns+` FROM media WHERE website_id = $1 AND content_hash = $2 LIMIT 1`,
		websiteID, hash))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find media by hash: %w", err)
	}
	return m, nil
}

// CountMissingAltText reports how many images still have no description.
func (s *Store) CountMissingAltText(ctx context.Context, websiteID int64) (int, error) {
	var n int
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM media
		 WHERE website_id = $1 AND alt_text = '' AND mime_type LIKE 'image/%'`,
		websiteID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count missing alt text: %w", err)
	}
	return n, nil
}

// GetByID returns a media record by ID, or nil when it does not exist.
func (s *Store) GetByID(ctx context.Context, id int64) (*Media, error) {
	m, err := scan(s.DB.Read.QueryRowContext(ctx, `SELECT `+columns+` FROM media WHERE id = $1`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get media by id: %w", err)
	}
	return m, nil
}

// GetByFilename returns a media record by website ID and filename.
func (s *Store) GetByFilename(ctx context.Context, websiteID int64, filename string) (*Media, error) {
	m, err := scan(s.DB.Read.QueryRowContext(ctx,
		`SELECT `+columns+` FROM media WHERE website_id = $1 AND filename = $2`, websiteID, filename))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get media by filename: %w", err)
	}
	return m, nil
}

// ErrNotFound is returned when a media row does not exist.
var ErrNotFound = errors.New("media not found")

// DefaultPerPage is the media page size used when the caller passes none.
const DefaultPerPage = 24

// Filter narrows the media list.
type Filter struct {
	// Query matches the original file name and the description. LIKE is the
	// right answer at a few thousand rows per site; FTS5 would be machinery for
	// a list a person scrolls.
	Query string
	// MimePrefix is "image/" or "application/", or empty for everything.
	MimePrefix string
	// Unused restricts the list to files no live page refers to — the view that
	// actually reclaims space on the card.
	Unused bool
}

// List returns one page of media for a website, newest first, together with the
// total number of files.
//
// The grid renders a thumbnail per row, so an unpaginated list would make the
// page grow without bound as a site accumulates uploads.
func (s *Store) List(ctx context.Context, websiteID int64, f Filter, page, perPage int) ([]Media, int, error) {
	if perPage <= 0 {
		perPage = DefaultPerPage
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * perPage

	where := ` WHERE m.website_id = $1`
	args := []any{websiteID}
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, "%"+q+"%")
		where += fmt.Sprintf(
			` AND (m.original_name LIKE $%d COLLATE NOCASE OR m.alt_text LIKE $%d COLLATE NOCASE)`,
			len(args), len(args))
	}
	if f.MimePrefix != "" {
		args = append(args, f.MimePrefix+"%")
		where += fmt.Sprintf(` AND m.mime_type LIKE $%d`, len(args))
	}
	if f.Unused {
		where += ` AND NOT EXISTS (SELECT 1 FROM media_usage u WHERE u.media_id = m.id)`
	}

	var total int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM media m`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count media: %w", err)
	}

	query := `SELECT ` + prefixed("m") + ` FROM media m` + where +
		fmt.Sprintf(` ORDER BY m.created_at DESC, m.id DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	args = append(args, perPage, offset)

	rows, err := s.DB.Read.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list media: %w", err)
	}
	defer rows.Close()

	var items []Media
	for rows.Next() {
		m, err := scan(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan media: %w", err)
		}
		items = append(items, *m)
	}
	return items, total, rows.Err()
}

// prefixed qualifies the projection with a table alias.
func prefixed(alias string) string {
	parts := strings.Split(columns, ", ")
	for i, p := range parts {
		parts[i] = alias + "." + p
	}
	return strings.Join(parts, ", ")
}

// SaveCrop records an editor's choice and the size the picture now has.
//
// The dimensions travel with it because everything that lays out a page needs
// them — the width and height attributes that stop a page jumping while it
// loads, and the srcset that would otherwise offer a variant wider than the
// picture itself.
func (s *Store) SaveCrop(ctx context.Context, id int64, c Crop, width, height int) error {
	c = c.Normalise()
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE media SET crop_ratio = $1, crop_zoom = $2, crop_rotation = $3,
		 focus_x = $4, focus_y = $5, width = $6, height = $7,
		 -- Die ausgelieferte Datei ist eine andere geworden, also braucht sie
		 -- eine andere Adresse. Sonst zeigt jeder Browser, der sie schon hat,
		 -- weiter das alte Bild.
		 version = version + 1
		 WHERE id = $8`,
		c.Ratio, c.Zoom, c.Rotation, c.FocusX, c.FocusY, width, height, id)
	if err != nil {
		return fmt.Errorf("zuschnitt sichern: %w", err)
	}
	return nil
}

// SaveFocus records only where the subject is.
//
// Separate from SaveCrop because it is a separate act: moving the focus point
// changes how a theme frames a picture it has to squeeze into a fixed shape,
// and that needs no re-encoding at all.
func (s *Store) SaveFocus(ctx context.Context, id, x, y int64) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE media SET focus_x = $1, focus_y = $2 WHERE id = $3`,
		clampPercent(int(x)), clampPercent(int(y)), id)
	if err != nil {
		return fmt.Errorf("fokus sichern: %w", err)
	}
	return nil
}
