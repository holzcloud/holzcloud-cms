package media

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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
// website's logo and favicon, and products (their main picture, their gallery
// and the pictures of their description, which goes through the same Markdown
// pipeline as a page). An album picture is the sharp case, because
// album_items cascades away when its file is deleted, so a live gallery would
// lose a picture without a word. Every one of those places is a branch of the
// one statement in usedAmong.
//
// A file is also shown by its address where no HTML is stored: a menu entry
// linking "Speisekarte.pdf", a redirect whose target is the file, a link field
// of a page or snippet, a block's Markdown. Those are text that contains the
// address, and each is a branch of its own.
//
// Stored HTML is searched with instr and never with a pattern match: "_" and
// "%" occur in file names, and a wildcard would make a_b.jpg in use because a
// page shows axb.jpg.
//
// Image field values are the id as text inside the fields JSON ("werte" and
// the rows of "gruppen"). Only a value whose key is that of an image field of
// the website counts: a number field "3", a page reference "3" or a choice "1"
// would otherwise lock file 3 or file 1 — usually the logo and the first
// uploads — for everybody without the publishing right, and the badge "In use"
// would mislead. The keys are those of every image field definition of the
// website, of pages, groups, snippets and block kinds alike: a key of one
// namespace counting in another only makes the lock stricter, which is the
// safe direction. Block pictures are the JSON integer under "medium"
// (also inside a gallery's items) or "vorschaubild" (a video's poster), and an
// own block kind's image field is the id as text under "felder".
//
// Every json_tree is guarded with json_valid: on modernc.org/sqlite one
// malformed value in any row fails the whole statement with "malformed JSON",
// and field.Decode tolerates such values, so they can exist.
func (s *Store) InUse(ctx context.Context, m *Media) (bool, error) {
	used, err := s.usedAmong(ctx, m.WebsiteID, []Media{*m})
	if err != nil {
		return true, err
	}
	return used[m.ID], nil
}

// InUseAmong answers InUse for each file of a list, by id, in one statement
// per website rather than one per file: the JSON of every page and snippet is
// walked once for the whole list, not once for every card of a media list.
// InUse asks the same statement with a list of one, so there is a single
// definition of "in use". On an error the caller decides what it means; on the
// media list it means every file counts as in use.
func (s *Store) InUseAmong(ctx context.Context, items []Media) (map[int64]bool, error) {
	out := make(map[int64]bool, len(items))
	bySite := map[int64][]Media{}
	var order []int64
	for _, m := range items {
		if _, seen := bySite[m.WebsiteID]; !seen {
			order = append(order, m.WebsiteID)
		}
		bySite[m.WebsiteID] = append(bySite[m.WebsiteID], m)
	}
	for _, ws := range order {
		files := bySite[ws]
		for len(files) > 0 {
			n := min(len(files), inUseChunk)
			used, err := s.usedAmong(ctx, ws, files[:n])
			if err != nil {
				return out, err
			}
			for _, m := range files[:n] {
				out[m.ID] = used[m.ID]
			}
			files = files[n:]
		}
	}
	return out, nil
}

// inUseChunk bounds the files of one statement, so that a long list never
// comes near SQLite's limit on bound parameters (three per file).
const inUseChunk = 200

// usedAmong answers, for files of one website, which of them are in use.
//
// Two lists are collected once and materialised: nums, every media id a row
// names directly (media_usage, preview and main pictures, block pictures,
// albums, logo and favicon, product galleries), and texts, every piece of
// stored text that can carry a file's address or, where idok is set, a file's
// id as an image field value. Each file is then one look into each list.
func (s *Store) usedAmong(ctx context.Context, websiteID int64, files []Media) (map[int64]bool, error) {
	used := make(map[int64]bool, len(files))
	if len(files) == 0 {
		return used, nil
	}
	args := make([]any, 0, 1+3*len(files))
	args = append(args, websiteID)
	values := make([]string, 0, len(files))
	for _, m := range files {
		n := len(args)
		values = append(values, fmt.Sprintf("($%d, $%d, $%d)", n+1, n+2, n+3))
		args = append(args, m.ID,
			"/media/"+strconv.FormatInt(websiteID, 10)+"/"+m.Filename,
			strconv.FormatInt(m.ID, 10))
	}
	query := `WITH ` + usedCTEs(`VALUES `+strings.Join(values, ", ")) + ` SELECT id FROM used`
	rows, err := s.DB.Read.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("media in use: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("media in use: %w", err)
		}
		used[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("media in use: %w", err)
	}
	return used, nil
}

// usedCTEs is the one definition of "in use", as the WITH list of a
// statement. ids is the body of the CTE ids(id, path, idtext) — the files the
// statement asks about, each with its address and its id as text — and $1 must
// be their website. The list ends in used(id), the ids that something of the
// website shows.
//
// usedAmong asks it for a list of files given as VALUES; the library's
// "Unused" filter and its count ask it for every file of the website
// (libraryIDs), so that the sidebar cannot call a file unused whose card says
// "In use".
func usedCTEs(ids string) string {
	return `ids(id, path, idtext) AS (` + ids + `),
		imagekeys(k) AS MATERIALIZED (
		    SELECT key FROM page_field_defs WHERE website_id = $1 AND kind = 'bild'
		),
		nums(n) AS MATERIALIZED (
		    SELECT u.media_id FROM media_usage u JOIN pages p ON p.id = u.page_id
		     WHERE u.media_id IN (SELECT id FROM ids) AND p.deleted_at IS NULL
		    UNION ALL
		    SELECT p.featured_media_id FROM pages p
		     WHERE p.website_id = $1 AND p.deleted_at IS NULL
		    UNION ALL
		    SELECT j.atom FROM pages p,
		           json_tree(CASE WHEN json_valid(p.blocks) THEN p.blocks ELSE '[]' END) j
		     WHERE p.website_id = $1 AND p.deleted_at IS NULL
		       AND j.key IN ('medium', 'vorschaubild') AND j.type = 'integer'
		    UNION ALL
		    SELECT ai.media_id FROM album_items ai JOIN albums a ON a.id = ai.album_id
		     WHERE a.website_id = $1
		    UNION ALL
		    SELECT w.logo_media_id FROM websites w WHERE w.id = $1
		    UNION ALL
		    SELECT w.favicon_media_id FROM websites w WHERE w.id = $1
		    UNION ALL
		    SELECT pr.featured_media_id FROM products pr WHERE pr.website_id = $1
		    UNION ALL
		    SELECT pm.media_id FROM product_media pm JOIN products pr ON pr.id = pm.product_id
		     WHERE pr.website_id = $1
		),
		texts(atom, idok) AS MATERIALIZED (
		    SELECT p.content_html, 0 FROM pages p
		     WHERE p.website_id = $1 AND p.deleted_at IS NULL
		    UNION ALL
		    SELECT j.atom, j.key IN (SELECT k FROM imagekeys) FROM pages p,
		           json_tree(CASE WHEN json_valid(p.fields) THEN p.fields ELSE '{}' END) j
		     WHERE p.website_id = $1 AND p.deleted_at IS NULL AND j.type = 'text'
		    UNION ALL
		    SELECT j.atom, j.path GLOB '*.felder' AND j.key IN (SELECT k FROM imagekeys) FROM pages p,
		           json_tree(CASE WHEN json_valid(p.blocks) THEN p.blocks ELSE '[]' END) j
		     WHERE p.website_id = $1 AND p.deleted_at IS NULL AND j.type = 'text'
		    UNION ALL
		    SELECT sn.content_html, 0 FROM snippets sn WHERE sn.website_id = $1
		    UNION ALL
		    SELECT j.atom, j.key IN (SELECT k FROM imagekeys) FROM snippets sn,
		           json_tree(CASE WHEN json_valid(sn.fields) THEN sn.fields ELSE '{}' END) j
		     WHERE sn.website_id = $1 AND j.type = 'text'
		    UNION ALL
		    SELECT pr.description_html, 0 FROM products pr WHERE pr.website_id = $1
		    UNION ALL
		    SELECT mi.url, 0 FROM menu_items mi JOIN menus mn ON mn.id = mi.menu_id
		     WHERE mn.website_id = $1
		    UNION ALL
		    SELECT rd.to_path, 0 FROM redirects rd WHERE rd.website_id = $1
		),
		used(id) AS (
		    SELECT ids.id FROM ids
		     WHERE ids.id IN (SELECT n FROM nums WHERE n IS NOT NULL)
		        OR EXISTS (SELECT 1 FROM texts t
		                    WHERE (t.idok AND t.atom = ids.idtext) OR instr(t.atom, ids.path) > 0)
		)`
}

// libraryIDs is the ids of usedCTEs for every file of the website $1.
const libraryIDs = `SELECT m.id, '/media/' || m.website_id || '/' || m.filename, CAST(m.id AS TEXT)
		    FROM media m WHERE m.website_id = $1`
