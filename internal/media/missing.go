package media

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// MissingFile is a database row whose file is absent below the directory
// MissingFiles was asked about.
//
// Table says which kind of file it is, and that is the point of the report: a
// variant (media_variants) is a scaled copy that `holzcloud thumbnails` can
// rebuild from its original, an original (media) is gone for good.
type MissingFile struct {
	// Rel is the file's place below the directory, media/<website_id>/<filename>,
	// with forward slashes. Empty when Unsafe.
	Rel string
	// Table is "media" or "media_variants".
	Table string
	// RowID is the id in Table. WebsiteID is the website whose directory holds
	// the file; for a variant that is its media row's website.
	RowID, WebsiteID int64
	// MediaID is the media row a variant belongs to; zero for an original.
	MediaID int64
	// Label is a variant's label; OriginalName is an original's upload name.
	Label, OriginalName string
	// Filename is the name the row stores.
	Filename string
	// Unsafe marks a row whose file name is not a plain file name. Such a name
	// is never joined into a path, so it is reported as missing unseen.
	Unsafe bool
}

// MissingFiles reads every row of media and returns how many it checked and
// those whose file is absent below root, which has the data directory's
// layout — the data directory itself or a backup of it. Rows come in id order.
//
// The layout is spelled once, in Path; the file's place in the report is Path
// below an empty root, so both cannot drift apart.
//
// Only an absent file counts as missing. Any other error from looking at it —
// a permission problem, an I/O error — is returned, because "cannot tell" is
// not "not there".
func (s *Store) MissingFiles(ctx context.Context, root string) (checked int, missing []MissingFile, err error) {
	originals, err := s.originalRows(ctx)
	if err != nil {
		return 0, nil, err
	}
	for _, m := range originals {
		checked++
		gone, err := absent(root, m.WebsiteID, m.Filename)
		if err != nil {
			return 0, nil, err
		}
		if gone {
			m.Rel = filepath.ToSlash(Path("", m.WebsiteID, m.Filename))
			missing = append(missing, m)
		}
	}
	return checked, missing, nil
}

// originalRows reads the media table as MissingFile candidates, in id order.
func (s *Store) originalRows(ctx context.Context) ([]MissingFile, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, filename, original_name FROM media ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list media: %w", err)
	}
	defer rows.Close()
	var out []MissingFile
	for rows.Next() {
		m := MissingFile{Table: "media"}
		if err := rows.Scan(&m.RowID, &m.WebsiteID, &m.Filename, &m.OriginalName); err != nil {
			return nil, fmt.Errorf("scan media: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list media: %w", err)
	}
	return out, nil
}

// absent reports whether the file Path names below root does not exist.
func absent(root string, websiteID int64, filename string) (bool, error) {
	_, err := os.Stat(Path(root, websiteID, filename))
	if err == nil {
		return false, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	return false, err
}
