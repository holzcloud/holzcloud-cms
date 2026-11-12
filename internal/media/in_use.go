package media

import (
	"context"
	"fmt"
	"strconv"
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
// through the editor, so it misses an imported or restored page, a page's
// preview image, image fields, block pictures, text snippets, albums, the
// website's logo and favicon, and products. An album picture is the sharp
// case, because album_items cascades away when its file is deleted, so a live
// gallery would lose a picture without a word. Every one of those places is an
// EXISTS arm of the one statement below.
//
// Stored HTML is searched with instr and never with a pattern match: "_" and
// "%" occur in file names, and a wildcard would make a_b.jpg in use because a
// page shows axb.jpg.
//
// Image field values are the id as text inside the fields JSON ("werte" and
// the rows of "gruppen"). The field arms match any text value equal to the id,
// not only those of image fields, because the field definitions live in Go and
// not in the row; such a false positive only makes the lock stricter, which is
// the safe direction. Block pictures are the JSON integer under "medium" (also
// inside a gallery's items) or "vorschaubild" (a video's poster), and an own
// block kind's image field is the id as text under "felder".
//
// Every json_tree is guarded with json_valid: on modernc.org/sqlite one
// malformed value in any row fails the whole statement with "malformed JSON",
// and field.Decode tolerates such values, so they can exist.
func (s *Store) InUse(ctx context.Context, m *Media) (bool, error) {
	path := "/media/" + strconv.FormatInt(m.WebsiteID, 10) + "/" + m.Filename
	idText := strconv.FormatInt(m.ID, 10)
	var used bool
	err := s.DB.Read.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM media_usage u JOIN pages p ON p.id = u.page_id
		                WHERE u.media_id = $1 AND p.deleted_at IS NULL)
		    OR EXISTS (SELECT 1 FROM pages p
		                WHERE p.website_id = $2 AND p.deleted_at IS NULL
		                  AND (p.featured_media_id = $1 OR instr(p.content_html, $3) > 0))
		    OR EXISTS (SELECT 1 FROM pages p,
		                json_tree(CASE WHEN json_valid(p.fields) THEN p.fields ELSE '{}' END) j
		                WHERE p.website_id = $2 AND p.deleted_at IS NULL
		                  AND j.type = 'text' AND j.atom = $4)
		    OR EXISTS (SELECT 1 FROM pages p,
		                json_tree(CASE WHEN json_valid(p.blocks) THEN p.blocks ELSE '[]' END) j
		                WHERE p.website_id = $2 AND p.deleted_at IS NULL
		                  AND ((j.key IN ('medium', 'vorschaubild') AND j.type = 'integer' AND j.atom = $1)
		                    OR (j.type = 'text' AND j.atom = $4 AND j.path GLOB '*.felder')))
		    OR EXISTS (SELECT 1 FROM snippets sn
		                WHERE sn.website_id = $2 AND instr(sn.content_html, $3) > 0)
		    OR EXISTS (SELECT 1 FROM snippets sn,
		                json_tree(CASE WHEN json_valid(sn.fields) THEN sn.fields ELSE '{}' END) j
		                WHERE sn.website_id = $2 AND j.type = 'text' AND j.atom = $4)
		    OR EXISTS (SELECT 1 FROM album_items ai JOIN albums a ON a.id = ai.album_id
		                WHERE a.website_id = $2 AND ai.media_id = $1)
		    OR EXISTS (SELECT 1 FROM websites w
		                WHERE w.id = $2 AND (w.logo_media_id = $1 OR w.favicon_media_id = $1))
		    OR EXISTS (SELECT 1 FROM products pr
		                WHERE pr.website_id = $2 AND pr.featured_media_id = $1)
		    OR EXISTS (SELECT 1 FROM product_media pm JOIN products pr ON pr.id = pm.product_id
		                WHERE pr.website_id = $2 AND pm.media_id = $1)`,
		m.ID, m.WebsiteID, path, idText).Scan(&used)
	if err != nil {
		return true, fmt.Errorf("media in use: %w", err)
	}
	return used, nil
}

// InUseAmong answers InUse for each file of a list, by id. A list screen holds
// at most DefaultPerPage cards, so one question per card is cheap and keeps a
// single definition of "in use". It stops at the first error; the caller
// decides what an error means, and on the media list it means every file
// counts as in use.
func (s *Store) InUseAmong(ctx context.Context, items []Media) (map[int64]bool, error) {
	out := make(map[int64]bool, len(items))
	for i := range items {
		used, err := s.InUse(ctx, &items[i])
		if err != nil {
			return out, err
		}
		out[items[i].ID] = used
	}
	return out, nil
}
