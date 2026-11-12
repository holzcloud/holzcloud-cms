package media

import (
	"context"
	"fmt"
)

// InUse reports whether anything of the file's website that is public, or
// becomes public with one click, shows the file.
//
// Drafts count (D-07): a draft is one click from public, and counting it is
// the safe direction — the question decides whether somebody without the
// publishing right may delete or change the file. A page in the wastebasket
// does not count.
//
// media_usage alone is not the answer: it is filled only by pages saved
// through the editor. An album picture is the sharp case, because album_items
// cascades away when its file is deleted, so a live gallery would lose a
// picture without a word.
func (s *Store) InUse(ctx context.Context, m *Media) (bool, error) {
	var used bool
	err := s.DB.Read.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM media_usage u JOIN pages p ON p.id = u.page_id
		                WHERE u.media_id = $1 AND p.deleted_at IS NULL)
		    OR EXISTS (SELECT 1 FROM album_items ai JOIN albums a ON a.id = ai.album_id
		                WHERE a.website_id = $2 AND ai.media_id = $1)`,
		m.ID, m.WebsiteID).Scan(&used)
	if err != nil {
		return true, fmt.Errorf("media in use: %w", err)
	}
	return used, nil
}
