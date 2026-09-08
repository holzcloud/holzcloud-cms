package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// The preview is the one screen that exists to show what will be published, and
// until this test it showed the internal syntax instead.
//
// A gallery bound to an album stores a marker and nothing else — that is GAL-03,
// and it is why the public page expands on every request. previewPageContent
// wrote template.HTML(pg.ContentHTML) straight out, so an editor who bound a
// gallery to an album and pressed "Vorschau" read [[album:sommer-2025:0]] where
// the pictures belong. It matters more here than the same gap does for a
// snippet: a snippet leaves a page with one sentence missing, an album leaves a
// page with a bracketed token where the whole block was.

// previewAlbumPage gives the fixture's website a published page carrying one
// gallery block bound to the album, written the way a real save writes it —
// block.Render over the block list, which is what renderBlocks calls.
func (f *albumFixture) previewAlbumPage(t *testing.T, slug string) {
	t.Helper()
	blocks := []block.Block{{Type: block.TypeGallery, AlbumSlug: f.albumA.Slug}}

	encoded, err := block.Encode(blocks, block.Builtin)
	if err != nil {
		t.Fatalf("block.Encode: %v", err)
	}
	decoded, err := block.Decode(encoded, block.Builtin)
	if err != nil {
		t.Fatalf("block.Decode: %v", err)
	}
	look := func(int64) (block.Image, bool) { return block.Image{}, false }
	html := block.Render(decoded, block.Builtin, look, page.RenderMarkdown)
	if !strings.Contains(html, "[[album:") {
		t.Fatalf("the fixture page carries no marker, so this test proves nothing: %q", html)
	}
	if _, err := f.h.pages.CreatePage(context.Background(), page.PageCreate{
		WebsiteID: f.siteA.ID,
		Title:     "Galerie",
		Slug:      slug,
		Markdown:  block.PlainText(decoded, block.Builtin),
		Blocks:    encoded,
		HTML:      html,
		Status:    "published",
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
}

func (f *albumFixture) preview(t *testing.T, slug string) string {
	t.Helper()
	target := fmt.Sprintf("/admin/websites/%d/preview/%s", f.siteA.ID, slug)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("id", id64(f.siteA.ID))
	req.SetPathValue("slug", slug)

	rec := httptest.NewRecorder()
	if err := f.h.HandlePreviewPage(rec, req); err != nil {
		t.Fatalf("HandlePreviewPage: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

// TestThePreviewShowsTheAlbumAndNotTheMarker is the property.
func TestThePreviewShowsTheAlbumAndNotTheMarker(t *testing.T) {
	f := newAlbumFixture(t)
	f.mustAdd(t, f.albumA.ID, f.mediaA, "Ein Bild")
	f.previewAlbumPage(t, "galerie")

	body := f.preview(t, "galerie")
	if strings.Contains(body, "[[album:") {
		t.Errorf("the preview shows the internal syntax where the pictures belong:\n%s", body)
	}
	if !strings.Contains(body, "Ein Bild") {
		t.Errorf("the preview does not show the album's picture:\n%s", body)
	}
}

// TestThePreviewOfAnEmptyAlbumShowsNoMarkerEither is the boundary the fix could
// most easily miss: an album that exists and has nothing in it expands to
// nothing, and nothing is not the marker.
func TestThePreviewOfAnEmptyAlbumShowsNoMarkerEither(t *testing.T) {
	f := newAlbumFixture(t)
	f.previewAlbumPage(t, "galerie")

	if body := f.preview(t, "galerie"); strings.Contains(body, "[[album:") {
		t.Errorf("an empty album left its marker on the preview:\n%s", body)
	}
}

// TestThePreviewWithoutAnAlbumStoreShowsNoMarker is the unwired case, and it is
// the same rule internal/public follows: the zero Set expands every marker to
// nothing rather than printing it.
func TestThePreviewWithoutAnAlbumStoreShowsNoMarker(t *testing.T) {
	f := newAlbumFixture(t)
	f.previewAlbumPage(t, "galerie")
	f.h.albumStore = nil

	if body := f.preview(t, "galerie"); strings.Contains(body, "[[album:") {
		t.Errorf("a handler with no album store printed the marker into the preview:\n%s", body)
	}
}
