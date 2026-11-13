package media

import (
	"context"
	"fmt"
	"testing"
)

// "In use" decides whether somebody without the publishing right may delete
// or change a file, so every place a file is shown has a subtest of its own,
// each on a fresh database holding exactly one use. Before that use is put in
// place the same file must answer false: otherwise a question that always
// says "in use" would pass every arm.

// inUseCase is a fresh website with one file, a.jpg.
func inUseCase(t *testing.T) (*Store, int64, *Media) {
	t.Helper()
	s, ws := newTestStore(t)
	m, err := s.Create(context.Background(), ws, "a.jpg", "a.jpg", "image/jpeg", 10, "hash-a")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return s, ws, m
}

func mustExec(t *testing.T, s *Store, query string, args ...any) int64 {
	t.Helper()
	res, err := s.DB.Write.Exec(query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func inUse(t *testing.T, s *Store, m *Media) bool {
	t.Helper()
	used, err := s.InUse(context.Background(), m)
	if err != nil {
		t.Fatalf("InUse: %v", err)
	}
	return used
}

// seedProduct inserts a product of the website and returns its id.
func seedProduct(t *testing.T, s *Store, websiteID int64, slug string) int64 {
	t.Helper()
	return mustExec(t, s,
		`INSERT INTO products (website_id, slug, title, created_at, updated_at)
		 VALUES ($1, $2, $2, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, websiteID, slug)
}

func TestInUseSeesEveryPlaceAFileIsShown(t *testing.T) {
	path := func(ws int64) string { return fmt.Sprintf("/media/%d/a.jpg", ws) }

	arms := []struct {
		name string
		use  func(t *testing.T, s *Store, ws int64, m *Media)
	}{
		{"a draft page recorded in media_usage", func(t *testing.T, s *Store, ws int64, m *Media) {
			page := seedPage(t, s, ws, "Entwurf", "entwurf")
			mustExec(t, s, `UPDATE pages SET status = 'draft' WHERE id = $1`, page)
			mustExec(t, s, `INSERT INTO media_usage (page_id, media_id) VALUES ($1, $2)`, page, m.ID)
		}},
		{"a page whose stored HTML shows it without a media_usage row", func(t *testing.T, s *Store, ws int64, m *Media) {
			page := seedPage(t, s, ws, "Importiert", "importiert")
			mustExec(t, s, `UPDATE pages SET content_html = $1 WHERE id = $2`,
				`<p><img src="`+path(ws)+`" alt=""></p>`, page)
		}},
		{"a page's preview image", func(t *testing.T, s *Store, ws int64, m *Media) {
			page := seedPage(t, s, ws, "Vorschau", "vorschau")
			mustExec(t, s, `UPDATE pages SET featured_media_id = $1 WHERE id = $2`, m.ID, page)
		}},
		{"a page's image field", func(t *testing.T, s *Store, ws int64, m *Media) {
			page := seedPage(t, s, ws, "Feld", "feld")
			mustExec(t, s, `UPDATE pages SET fields = $1 WHERE id = $2`,
				fmt.Sprintf(`{"werte":{"ansicht":"%d"}}`, m.ID), page)
		}},
		{"an image field in a group row", func(t *testing.T, s *Store, ws int64, m *Media) {
			page := seedPage(t, s, ws, "Gruppe", "gruppe")
			mustExec(t, s, `UPDATE pages SET fields = $1 WHERE id = $2`,
				fmt.Sprintf(`{"gruppen":{"team":[{"name":"Anna"},{"name":"Ben","foto":"%d"}]}}`, m.ID), page)
		}},
		{"a block picture", func(t *testing.T, s *Store, ws int64, m *Media) {
			page := seedPage(t, s, ws, "Block", "block")
			mustExec(t, s, `UPDATE pages SET blocks = $1 WHERE id = $2`,
				fmt.Sprintf(`[{"typ":"text","markdown":"Hallo"},{"typ":"bild","medium":%d}]`, m.ID), page)
		}},
		{"a picture inside a block item", func(t *testing.T, s *Store, ws int64, m *Media) {
			page := seedPage(t, s, ws, "Galerie", "galerie")
			mustExec(t, s, `UPDATE pages SET blocks = $1 WHERE id = $2`,
				fmt.Sprintf(`[{"typ":"galerie","eintraege":[{"medium":%d,"alt":"Bank"}]}]`, m.ID), page)
		}},
		{"a video block's poster", func(t *testing.T, s *Store, ws int64, m *Media) {
			page := seedPage(t, s, ws, "Film", "film")
			mustExec(t, s, `UPDATE pages SET blocks = $1 WHERE id = $2`,
				fmt.Sprintf(`[{"typ":"video","vorschaubild":%d}]`, m.ID), page)
		}},
		{"an image field of an own block kind", func(t *testing.T, s *Store, ws int64, m *Media) {
			page := seedPage(t, s, ws, "Eigen", "eigen")
			mustExec(t, s, `UPDATE pages SET blocks = $1 WHERE id = $2`,
				fmt.Sprintf(`[{"typ":"angebot","felder":{"titel":"Tisch","bild":"%d"}}]`, m.ID), page)
		}},
		{"a text snippet's HTML", func(t *testing.T, s *Store, ws int64, m *Media) {
			mustExec(t, s, `INSERT INTO snippets (website_id, key, name, content_html) VALUES ($1, 'fuss', 'Fuss', $2)`,
				ws, `<img src="`+path(ws)+`">`)
		}},
		{"a text snippet's image field", func(t *testing.T, s *Store, ws int64, m *Media) {
			mustExec(t, s, `INSERT INTO snippets (website_id, key, name, fields) VALUES ($1, 'kopf', 'Kopf', $2)`,
				ws, fmt.Sprintf(`{"werte":{"logo":"%d"}}`, m.ID))
		}},
		{"an album picture", func(t *testing.T, s *Store, ws int64, m *Media) {
			album := mustExec(t, s, `INSERT INTO albums (website_id, slug, name) VALUES ($1, 'werkstatt', 'Werkstatt')`, ws)
			mustExec(t, s, `INSERT INTO album_items (album_id, media_id) VALUES ($1, $2)`, album, m.ID)
		}},
		{"the website's logo", func(t *testing.T, s *Store, ws int64, m *Media) {
			mustExec(t, s, `UPDATE websites SET logo_media_id = $1 WHERE id = $2`, m.ID, ws)
		}},
		{"the website's favicon", func(t *testing.T, s *Store, ws int64, m *Media) {
			mustExec(t, s, `UPDATE websites SET favicon_media_id = $1 WHERE id = $2`, m.ID, ws)
		}},
		{"a product's main picture", func(t *testing.T, s *Store, ws int64, m *Media) {
			product := seedProduct(t, s, ws, "tisch")
			mustExec(t, s, `UPDATE products SET featured_media_id = $1 WHERE id = $2`, m.ID, product)
		}},
		{"a product's gallery", func(t *testing.T, s *Store, ws int64, m *Media) {
			product := seedProduct(t, s, ws, "stuhl")
			mustExec(t, s, `INSERT INTO product_media (product_id, media_id, position) VALUES ($1, $2, 0)`, product, m.ID)
		}},
		{"a product's description", func(t *testing.T, s *Store, ws int64, m *Media) {
			product := seedProduct(t, s, ws, "bank")
			mustExec(t, s, `UPDATE products SET description_html = $1 WHERE id = $2`,
				`<p><img src="`+path(ws)+`" alt="Bank"></p>`, product)
		}},
	}

	for _, arm := range arms {
		t.Run(arm.name, func(t *testing.T) {
			s, ws, m := inUseCase(t)
			if inUse(t, s, m) {
				t.Fatal("in use before anything shows it")
			}
			arm.use(t, s, ws, m)
			if !inUse(t, s, m) {
				t.Error("not in use")
			}
		})
	}
}

func TestInUseIgnoresTheWastebasketOtherWebsitesAndMalformedFields(t *testing.T) {
	t.Run("a page in the wastebasket", func(t *testing.T) {
		s, ws, m := inUseCase(t)
		page := seedPage(t, s, ws, "Weg", "weg")
		mustExec(t, s, `UPDATE pages SET content_html = $1, featured_media_id = $2,
			fields = $3, blocks = $4, deleted_at = '2026-01-01T00:00:00Z' WHERE id = $5`,
			fmt.Sprintf(`<img src="/media/%d/a.jpg">`, ws), m.ID,
			fmt.Sprintf(`{"werte":{"bild":"%d"}}`, m.ID),
			fmt.Sprintf(`[{"typ":"bild","medium":%d}]`, m.ID), page)
		mustExec(t, s, `INSERT INTO media_usage (page_id, media_id) VALUES ($1, $2)`, page, m.ID)
		if inUse(t, s, m) {
			t.Error("a page in the wastebasket makes the file in use")
		}
	})

	t.Run("a page of another website naming the same file name", func(t *testing.T) {
		s, _, m := inUseCase(t)
		other := mustExec(t, s, `INSERT INTO websites (name, description) VALUES ('Andere', '')`)
		page := seedPage(t, s, other, "Fremd", "fremd")
		mustExec(t, s, `UPDATE pages SET content_html = $1 WHERE id = $2`,
			fmt.Sprintf(`<img src="/media/%d/a.jpg">`, other), page)
		mustExec(t, s, `INSERT INTO snippets (website_id, key, name, content_html) VALUES ($1, 'fuss', 'Fuss', $2)`,
			other, fmt.Sprintf(`<img src="/media/%d/a.jpg">`, other))
		if inUse(t, s, m) {
			t.Error("another website's page makes the file in use")
		}
	})

	t.Run("a malformed fields value", func(t *testing.T) {
		s, ws, m := inUseCase(t)
		page := seedPage(t, s, ws, "Kaputt", "kaputt")
		mustExec(t, s, `UPDATE pages SET fields = 'kaputt{', blocks = 'kaputt[' WHERE id = $1`, page)
		mustExec(t, s, `INSERT INTO snippets (website_id, key, name, fields) VALUES ($1, 'kaputt', 'Kaputt', 'kaputt{')`, ws)
		used, err := s.InUse(context.Background(), m)
		if err != nil {
			t.Fatalf("a malformed value breaks the question: %v", err)
		}
		if used {
			t.Error("only malformed values, and the file is in use")
		}
		mustExec(t, s, `INSERT INTO snippets (website_id, key, name, content_html) VALUES ($1, 'fuss', 'Fuss', $2)`,
			ws, fmt.Sprintf(`<img src="/media/%d/a.jpg">`, ws))
		used, err = s.InUse(context.Background(), m)
		if err != nil {
			t.Fatalf("a malformed value beside a use breaks the question: %v", err)
		}
		if !used {
			t.Error("a snippet shows the file beside a malformed value, and it is not in use")
		}
	})

	t.Run("no wildcards in a file name", func(t *testing.T) {
		s, ws := newTestStore(t)
		m, err := s.Create(context.Background(), ws, "a_b.jpg", "a_b.jpg", "image/jpeg", 10, "hash")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		page := seedPage(t, s, ws, "Andere Datei", "andere")
		mustExec(t, s, `UPDATE pages SET content_html = $1 WHERE id = $2`,
			fmt.Sprintf(`<img src="/media/%d/axb.jpg">`, ws), page)
		if inUse(t, s, m) {
			t.Error("a_b.jpg is in use because a page shows axb.jpg")
		}
	})
}

func TestInUseAmongMatchesInUse(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	var files []Media
	for _, name := range []string{"album.jpg", "schnipsel.jpg", "frei.jpg"} {
		m, err := s.Create(ctx, ws, name, name, "image/jpeg", 10, "hash-"+name)
		if err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
		files = append(files, *m)
	}
	album := mustExec(t, s, `INSERT INTO albums (website_id, slug, name) VALUES ($1, 'werkstatt', 'Werkstatt')`, ws)
	mustExec(t, s, `INSERT INTO album_items (album_id, media_id) VALUES ($1, $2)`, album, files[0].ID)
	mustExec(t, s, `INSERT INTO snippets (website_id, key, name, content_html) VALUES ($1, 'fuss', 'Fuss', $2)`,
		ws, fmt.Sprintf(`<img src="/media/%d/schnipsel.jpg">`, ws))

	got, err := s.InUseAmong(ctx, files)
	if err != nil {
		t.Fatalf("InUseAmong: %v", err)
	}
	want := map[int64]bool{files[0].ID: true, files[1].ID: true, files[2].ID: false}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("file %d: in use %v, want %v", id, got[id], w)
		}
		m := files[0]
		for _, f := range files {
			if f.ID == id {
				m = f
			}
		}
		if single := inUse(t, s, &m); single != got[id] {
			t.Errorf("file %d: InUseAmong %v, InUse %v", id, got[id], single)
		}
	}

	empty, err := s.InUseAmong(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Errorf("empty list: %v, %v; want an empty map", empty, err)
	}
	if empty == nil {
		t.Error("empty list: a nil map, want an empty one")
	}
}
