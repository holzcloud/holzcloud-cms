package media

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// InUseError says a file cannot be deleted because pages still show it.
//
// The old Delete was an unconditional DELETE plus os.Remove: every page
// embedding the file showed a broken image from that moment on, with no warning
// and no query that could have found the damage afterwards.
type InUseError struct {
	// Pages are the titles of the pages that reference the file, so the message
	// can name them instead of just refusing.
	Pages []string
}

func (e *InUseError) Error() string {
	return fmt.Sprintf("media is used on %d page(s)", len(e.Pages))
}

// ExtractRefs finds the media ids a page's rendered HTML refers to.
//
// It parses rather than pattern-matches: an attribute can be single-quoted,
// unquoted or spread over lines, and a regexp would miss those and would also
// match a URL inside a code block that is not a reference at all.
func ExtractRefs(ctx context.Context, s *Store, websiteID int64, pageHTML string) ([]int64, error) {
	filenames := extractFilenames(websiteID, pageHTML)
	if len(filenames) == 0 {
		return nil, nil
	}

	var ids []int64
	for _, name := range filenames {
		m, err := s.GetByFilename(ctx, websiteID, name)
		if err != nil {
			return nil, err
		}
		if m != nil {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

// extractFilenames walks the document and collects the file names under this
// website's media prefix.
func extractFilenames(websiteID int64, pageHTML string) []string {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return nil
	}
	prefix := "/media/" + strconv.FormatInt(websiteID, 10) + "/"

	seen := map[string]bool{}
	var names []string
	add := func(value string) {
		for _, candidate := range splitSrcset(value) {
			if !strings.HasPrefix(candidate, prefix) {
				continue
			}
			name := strings.TrimPrefix(candidate, prefix)
			// A query or fragment is not part of the stored file name.
			if i := strings.IndexAny(name, "?#"); i >= 0 {
				name = name[:i]
			}
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, a := range n.Attr {
				switch a.Key {
				case "src", "href", "srcset", "poster":
					add(a.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return names
}

// splitSrcset handles both a plain URL and a srcset list.
func splitSrcset(value string) []string {
	if !strings.Contains(value, ",") {
		return []string{strings.TrimSpace(value)}
	}
	var out []string
	for _, part := range strings.Split(value, ",") {
		fields := strings.Fields(part)
		if len(fields) > 0 {
			out = append(out, fields[0])
		}
	}
	return out
}

// ReplaceUsage records exactly which files a page refers to.
//
// It runs in one transaction so a page is never briefly recorded as using
// nothing, which would let a concurrent delete slip through.
func (s *Store) ReplaceUsage(ctx context.Context, pageID int64, mediaIDs []int64) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM media_usage WHERE page_id = $1`, pageID); err != nil {
		return fmt.Errorf("clear media usage: %w", err)
	}
	for _, id := range mediaIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO media_usage (page_id, media_id) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`, pageID, id); err != nil {
			return fmt.Errorf("record media usage: %w", err)
		}
	}
	return tx.Commit()
}

// UsedOnPages returns the titles of the live pages referring to a file.
func (s *Store) UsedOnPages(ctx context.Context, mediaID int64) ([]string, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT p.title FROM media_usage u
		 JOIN pages p ON p.id = u.page_id
		 WHERE u.media_id = $1 AND p.deleted_at IS NULL
		 ORDER BY p.title`, mediaID)
	if err != nil {
		return nil, fmt.Errorf("media usage: %w", err)
	}
	defer rows.Close()

	var titles []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, fmt.Errorf("scan media usage: %w", err)
		}
		titles = append(titles, title)
	}
	return titles, rows.Err()
}

// Delete removes a media record and its disk file.
//
// force is the operator's explicit "yes, break those pages" — without it a file
// still shown somewhere is refused, with the page titles in the error.
func (s *Store) Delete(ctx context.Context, id int64, dataDir string, force bool) error {
	var websiteID int64
	var filename string
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT website_id, filename FROM media WHERE id = $1`, id).
		Scan(&websiteID, &filename)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get media for delete: %w", err)
	}

	if !force {
		pages, err := s.UsedOnPages(ctx, id)
		if err != nil {
			return err
		}
		if len(pages) > 0 {
			return &InUseError{Pages: pages}
		}
	}

	// Read the variant names before the row goes, because the cascade takes
	// them with it and the files would then have no name left to delete by.
	s.removeVariantFiles(ctx, id, websiteID, dataDir)

	if _, err := s.DB.Write.ExecContext(ctx, `DELETE FROM media WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete media: %w", err)
	}

	diskPath := filepath.Join(dataDir, "media", strconv.FormatInt(websiteID, 10), filename)
	_ = os.Remove(diskPath)
	return nil
}
