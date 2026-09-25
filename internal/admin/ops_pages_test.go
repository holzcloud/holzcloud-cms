package admin

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// A restore brings back what a revision holds — title, text, blocks — and
// nothing it does not hold is lost on the way. It used to send an update with
// the excerpt, the schedule and the fields left empty, which cleared them, and
// it rendered a block page's plain text, which turned its blocks into prose.
func TestRevisionRestoreKeepsSettingsAndBlocks(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	store := page.NewStore(database)
	p := seedPage(t, database, ws.ID, "Titel", "titel", "", "draft")

	// A block page with an excerpt and a meta description of its own.
	built, err := h.OpSetPageBlocks(ctx, ws.ID, p.ID, 0, []block.Block{
		{Type: block.TypeQuote, Text: "Gutes Holz", Source: "Oma"},
	})
	if err != nil {
		t.Fatalf("OpSetPageBlocks: %v", err)
	}
	if !strings.Contains(built.ContentHTML, "hc-zitat") {
		t.Fatalf("the blocks were not rendered: %q", built.ContentHTML)
	}
	withMeta, err := h.OpEditPage(ctx, ws.ID, p.ID, 0, func(_ *page.Page, u *page.PageUpdate) error {
		u.Meta.Excerpt, u.Meta.MetaDescription = "Kurz", "Für Suchmaschinen"
		return nil
	})
	if err != nil {
		t.Fatalf("OpEditPage: %v", err)
	}

	// Then written over as plain text.
	if err := store.UpdatePage(ctx, p.ID, page.PageUpdate{
		Title: "Titel", Slug: "titel", Markdown: "neu", HTML: "<p>neu</p>", Status: "draft",
		Meta:            page.PageMeta{Excerpt: "Kurz", MetaDescription: "Für Suchmaschinen"},
		ExpectedVersion: withMeta.Version,
	}); err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}
	revs, err := store.ListRevisions(ctx, p.ID)
	if err != nil || len(revs) == 0 || revs[0].Blocks == "" {
		t.Fatalf("no block revision: %v %+v", err, revs)
	}

	// Through the screen, which shares restoreRevision with the tools.
	serve(t, h, sm, h.HandlePageRevisionRestore, postForm("/restore", nil, map[string]string{
		"id": strconv.FormatInt(ws.ID, 10), "pageID": strconv.FormatInt(p.ID, 10),
		"revID": strconv.FormatInt(revs[0].ID, 10),
	}))

	after, _ := store.GetPage(ctx, p.ID)
	if after.Blocks == "" || !strings.Contains(after.ContentHTML, "hc-zitat") {
		t.Errorf("the blocks did not come back: blocks %q, html %q", after.Blocks, after.ContentHTML)
	}
	if after.Excerpt != "Kurz" || after.MetaDescription != "Für Suchmaschinen" {
		t.Errorf("the restore cleared the settings: %q / %q", after.Excerpt, after.MetaDescription)
	}
}

// A copy of a block page is a block page, and a copy is always a draft.
func TestDuplicateCopiesTheBlocks(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	ctx := context.Background()
	p := seedPage(t, database, ws.ID, "Start", "start", "", "published")
	if _, err := h.OpSetPageBlocks(ctx, ws.ID, p.ID, 0, []block.Block{{Type: block.TypeDivider}}); err != nil {
		t.Fatal(err)
	}

	cp, err := h.OpDuplicatePage(ctx, ws.ID, p.ID)
	if err != nil {
		t.Fatalf("OpDuplicatePage: %v", err)
	}
	if cp.Blocks == "" || cp.Status != "draft" || cp.Slug == "start" {
		t.Errorf("copy: blocks %q, status %q, slug %q", cp.Blocks, cp.Status, cp.Slug)
	}
}

// Every Op checks that the page belongs to the website it was named with: the
// ids come from outside.
func TestPageOpsStayWithTheirWebsite(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	ctx := context.Background()
	other, err := h.domains.CreateWebsite(ctx, "Andere", "")
	if err != nil {
		t.Fatal(err)
	}
	p := seedPage(t, database, ws.ID, "Meins", "meins", "text", "draft")

	if _, err := h.OpTrashPage(ctx, other.ID, p.ID); !errors.Is(err, page.ErrNotFound) {
		t.Errorf("OpTrashPage across websites: %v", err)
	}
	if _, err := h.OpSetPageBlocks(ctx, other.ID, p.ID, 0, []block.Block{{Type: block.TypeDivider}}); !errors.Is(err, page.ErrNotFound) {
		t.Errorf("OpSetPageBlocks across websites: %v", err)
	}
	if _, err := h.OpDuplicatePage(ctx, other.ID, p.ID); !errors.Is(err, page.ErrNotFound) {
		t.Errorf("OpDuplicatePage across websites: %v", err)
	}
	if _, _, err := h.OpShareLink(ctx, other.ID, p.ID, 1); !errors.Is(err, page.ErrNotFound) {
		t.Errorf("OpShareLink across websites: %v", err)
	}
}
