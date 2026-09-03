package media

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SaveVariants records the scaled copies of an image and the original's size.
//
// One transaction, because a half-written set would make a theme emit a srcset
// pointing at a width that does not exist.
func (s *Store) SaveVariants(ctx context.Context, mediaID int64, width, height int, variants []Variant) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save variants: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE media SET width = $1, height = $2 WHERE id = $3`, width, height, mediaID); err != nil {
		return fmt.Errorf("store image dimensions: %w", err)
	}

	for _, v := range variants {
		// A re-run replaces what is there: regenerating the set after a change
		// to the widths must not collide with the old rows.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO media_variants (media_id, label, filename, width, height, size_bytes)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (media_id, label) DO UPDATE SET
			     filename = excluded.filename, width = excluded.width,
			     height = excluded.height, size_bytes = excluded.size_bytes`,
			mediaID, v.Label, v.Filename, v.Width, v.Height, v.SizeBytes); err != nil {
			return fmt.Errorf("insert variant %q: %w", v.Label, err)
		}
	}
	return tx.Commit()
}

// VariantsFor returns the scaled copies of one image, narrowest first.
func (s *Store) VariantsFor(ctx context.Context, mediaID int64) ([]Variant, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, media_id, label, filename, width, height, size_bytes
		 FROM media_variants WHERE media_id = $1 ORDER BY width`, mediaID)
	if err != nil {
		return nil, fmt.Errorf("list variants: %w", err)
	}
	defer rows.Close()
	return scanVariants(rows)
}

func scanVariants(rows *sql.Rows) ([]Variant, error) {
	var out []Variant
	for rows.Next() {
		var v Variant
		if err := rows.Scan(&v.ID, &v.MediaID, &v.Label, &v.Filename,
			&v.Width, &v.Height, &v.SizeBytes); err != nil {
			return nil, fmt.Errorf("scan variant: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ServedFile is what the public media route needs to answer a request: the
// bytes' type and the website that owns them.
type ServedFile struct {
	WebsiteID int64
	Filename  string
	MimeType  string
}

// ResolveServed finds a public file name, original or scaled copy.
//
// Variants live in the same directory as their original and are reachable under
// the same /media/ prefix, so they have to resolve here or every srcset entry
// would 404. The MIME type comes from the variant's own extension, because a
// PNG original may have JPEG copies.
func (s *Store) ResolveServed(ctx context.Context, websiteID int64, filename string) (*ServedFile, error) {
	m, err := s.GetByFilename(ctx, websiteID, filename)
	if err != nil {
		return nil, err
	}
	if m != nil {
		return &ServedFile{WebsiteID: m.WebsiteID, Filename: m.Filename, MimeType: m.MimeType}, nil
	}

	// The join is what keeps one site's route from reaching another's file.
	var mime string
	err = s.DB.Read.QueryRowContext(ctx,
		`SELECT v.filename FROM media_variants v
		 JOIN media m ON m.id = v.media_id
		 WHERE m.website_id = $1 AND v.filename = $2`, websiteID, filename).Scan(&filename)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve variant: %w", err)
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".png":
		mime = "image/png"
	default:
		mime = "image/jpeg"
	}
	return &ServedFile{WebsiteID: websiteID, Filename: filename, MimeType: mime}, nil
}

// ImageSet is everything a theme needs to emit one responsive <img>.
type ImageSet struct {
	WebsiteID int64
	Filename  string
	Width     int
	Height    int
	AltText   string
	Variants  []Variant
	// Version is how often the served bytes changed. It goes into every address
	// this set produces, because a media URL is cached as immutable for a year
	// and cropping made that promise untrue — a page saved before a crop would
	// otherwise keep showing the picture as it was.
	Version int
}

// versionQuery is what makes a changed picture a different address.
func (i ImageSet) versionQuery() string {
	if i.Version <= 0 {
		return ""
	}
	return "?v=" + strconv.Itoa(i.Version)
}

// VersionedPath is the address a browser should request.
func (i ImageSet) VersionedPath() string { return i.Path() + i.versionQuery() }

// Path is the public, same-origin address of the original.
func (i ImageSet) Path() string {
	return "/media/" + strconv.FormatInt(i.WebsiteID, 10) + "/" + i.Filename
}

// SrcSet renders the candidate list, widest last, with the original included so
// a high-density screen still has something to reach for.
func (i ImageSet) SrcSet() string {
	if len(i.Variants) == 0 {
		return ""
	}
	var b strings.Builder
	base := "/media/" + strconv.FormatInt(i.WebsiteID, 10) + "/"
	// Every candidate carries the version: the scaled copies are regenerated
	// whenever the original changes, so they are just as much a broken promise
	// as the original if their address stays the same.
	q := i.versionQuery()
	for _, v := range i.Variants {
		fmt.Fprintf(&b, "%s%s%s %dw, ", base, v.Filename, q, v.Width)
	}
	fmt.Fprintf(&b, "%s%s%s %dw", base, i.Filename, q, i.Width)
	return b.String()
}

// Index maps a public media path to its responsive information.
//
// The key is the whole "/media/<site>/<file>" path, not the bare file name: one
// site may legitimately embed another's image, and a bare name would then hand
// it this site's srcset — three candidates pointing at files that do not exist
// under that address.
type Index map[string]ImageSet

// LoadImageSets builds the index for the images a document refers to.
//
// Only the files actually named in the HTML are looked up: a site with a
// thousand uploads should not pay for all of them to render one page.
func (s *Store) LoadImageSets(ctx context.Context, websiteID int64, pageHTML string) (Index, error) {
	names := extractFilenames(websiteID, pageHTML)
	if len(names) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(names))
	args := []any{websiteID}
	for i, n := range names {
		args = append(args, n)
		placeholders[i] = "$" + strconv.Itoa(i+2)
	}

	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT m.id, m.filename, m.width, m.height, m.alt_text, m.version
		 FROM media m
		 WHERE m.website_id = $1 AND m.filename IN (`+strings.Join(placeholders, ", ")+`)
		   AND m.width > 0`, args...)
	if err != nil {
		return nil, fmt.Errorf("load image sets: %w", err)
	}
	defer rows.Close()

	idx := Index{}
	byID := map[int64]string{}
	for rows.Next() {
		var id int64
		var set ImageSet
		if err := rows.Scan(&id, &set.Filename, &set.Width, &set.Height, &set.AltText, &set.Version); err != nil {
			return nil, fmt.Errorf("scan image set: %w", err)
		}
		set.WebsiteID = websiteID
		idx[set.Path()] = set
		byID[id] = set.Path()
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(byID) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(byID))
	vargs := []any{}
	for id := range byID {
		vargs = append(vargs, id)
		ids = append(ids, "$"+strconv.Itoa(len(vargs)))
	}
	vrows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, media_id, label, filename, width, height, size_bytes
		 FROM media_variants WHERE media_id IN (`+strings.Join(ids, ", ")+`) ORDER BY width`, vargs...)
	if err != nil {
		return nil, fmt.Errorf("load variants: %w", err)
	}
	defer vrows.Close()

	variants, err := scanVariants(vrows)
	if err != nil {
		return nil, err
	}
	for _, v := range variants {
		key := byID[v.MediaID]
		set := idx[key]
		set.Variants = append(set.Variants, v)
		idx[key] = set
	}
	return idx, nil
}

// removeVariantFiles deletes the scaled copies of an image from disk.
//
// The rows go with the original through ON DELETE CASCADE; the files do not,
// and orphaned copies on an SD card are exactly the kind of leak nobody notices
// until the card is full.
func (s *Store) removeVariantFiles(ctx context.Context, mediaID, websiteID int64, dataDir string) {
	variants, err := s.VariantsFor(ctx, mediaID)
	if err != nil {
		return
	}
	dir := filepath.Join(dataDir, "media", strconv.FormatInt(websiteID, 10))
	for _, v := range variants {
		_ = os.Remove(filepath.Join(dir, v.Filename))
	}
}
