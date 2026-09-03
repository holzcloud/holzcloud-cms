package snippet

import (
	"context"
	"errors"
	"html/template"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

func newTestStore(t *testing.T) (*Store, int64) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	res, err := database.Write.Exec(`INSERT INTO websites (name, description) VALUES ('Test', '')`)
	if err != nil {
		t.Fatalf("insert website: %v", err)
	}
	id, _ := res.LastInsertId()
	return NewStore(database), id
}

// A marker on its own line becomes <p>[[snippet:x]]</p>. Substituting inside
// the paragraph would nest block elements in a <p>, which browsers repair by
// closing the paragraph early and scattering the rest of the block.
func TestExpandReplacesTheWholeParagraph(t *testing.T) {
	snippets := map[string]template.HTML{
		"zeiten": "<h2>Öffnungszeiten</h2>\n<ul><li>Mo–Fr</li></ul>",
	}
	got := Expand("<p>Vorher</p>\n<p>[[snippet:zeiten]]</p>\n<p>Nachher</p>", snippets)

	if strings.Contains(got, "<p><h2>") {
		t.Errorf("block content was nested inside a paragraph: %q", got)
	}
	if !strings.Contains(got, "<h2>Öffnungszeiten</h2>") {
		t.Errorf("snippet was not inserted: %q", got)
	}
	if !strings.Contains(got, "<p>Vorher</p>") || !strings.Contains(got, "<p>Nachher</p>") {
		t.Errorf("surrounding content was lost: %q", got)
	}
}

func TestExpandHandlesInlineMarkers(t *testing.T) {
	got := Expand("<p>Wir sind [[snippet:tel]] erreichbar.</p>",
		map[string]template.HTML{"tel": "unter 0123"})
	if got != "<p>Wir sind unter 0123 erreichbar.</p>" {
		t.Errorf("got %q", got)
	}
}

// An unknown key must not leave the internal syntax on a live page.
func TestExpandDropsUnknownMarkers(t *testing.T) {
	got := Expand("<p>[[snippet:gibtsnicht]]</p>", map[string]template.HTML{})
	if strings.Contains(got, "[[snippet:") {
		t.Errorf("the marker is visible to visitors: %q", got)
	}
}

func TestExpandLeavesContentWithoutMarkersAlone(t *testing.T) {
	const in = "<p>Ganz normaler Text mit [[eckigen]] Klammern.</p>"
	if got := Expand(in, map[string]template.HTML{"x": "y"}); got != in {
		t.Errorf("got %q, want it unchanged", got)
	}
}

func TestUsedKeysFindsEachKeyOnce(t *testing.T) {
	keys := UsedKeys("[[snippet:a]] und [[snippet:b]] und nochmal [[snippet:a]]")
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Errorf("got %v, want [a b]", keys)
	}
}

func TestCreateRejectsADuplicateKey(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	if _, err := s.Create(ctx, ws, "zeiten", "Öffnungszeiten", "Mo–Fr", "<p>Mo–Fr</p>"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.Create(ctx, ws, "zeiten", "Andere", "x", "<p>x</p>"); !errors.Is(err, ErrKeyTaken) {
		t.Errorf("got %v, want ErrKeyTaken", err)
	}
}

// LatestUpdate is what a page's Last-Modified has to account for: editing the
// opening hours changes what every page renders without touching any page row.
func TestLoadRenderedReportsTheNewestChange(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	sn, err := s.Create(ctx, ws, "zeiten", "Öffnungszeiten", "Mo–Fr", "<p>Mo–Fr</p>")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rendered, err := s.LoadRendered(ctx, ws)
	if err != nil {
		t.Fatalf("LoadRendered: %v", err)
	}
	if rendered.HTML["zeiten"] != "<p>Mo–Fr</p>" {
		t.Errorf("rendered map is %+v", rendered.HTML)
	}
	if rendered.LatestUpdate.IsZero() {
		t.Error("LatestUpdate is zero, so a conditional request would serve stale content forever")
	}
	if !rendered.LatestUpdate.Equal(sn.UpdatedAt) {
		t.Errorf("LatestUpdate = %v, want %v", rendered.LatestUpdate, sn.UpdatedAt)
	}
}

func TestCountUsageCountsLivePagesOnly(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()

	for _, row := range []struct{ slug, body, deleted string }{
		{"a", "Text [[snippet:zeiten]]", ""},
		{"b", "Text [[snippet:zeiten]]", "2026-01-01T00:00:00Z"},
		{"c", "Ohne Marker", ""},
	} {
		var deleted any
		if row.deleted != "" {
			deleted = row.deleted
		}
		if _, err := s.DB.Write.ExecContext(ctx,
			`INSERT INTO pages (website_id, title, slug, content_markdown, content_html, status, deleted_at)
			 VALUES ($1, $2, $2, $3, '', 'published', $4)`,
			ws, row.slug, row.body, deleted); err != nil {
			t.Fatalf("insert page: %v", err)
		}
	}

	n, err := s.CountUsage(ctx, ws, "zeiten")
	if err != nil {
		t.Fatalf("CountUsage: %v", err)
	}
	if n != 1 {
		t.Errorf("usage count = %d, want 1 — a trashed page is not a use", n)
	}
}
