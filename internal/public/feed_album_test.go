package public

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// The feed is a live public route, and a subscriber is a visitor.
//
// internal/album/expand.go states the rule: "a visitor must not see the
// internal syntax on a live page." expandForFeed states its own half: "so a
// subscriber sees the same text as a visitor rather than the raw marker." Both
// were false for an album, because expandForFeed expanded snippet markers and
// nothing else — so every post carrying an album-backed gallery shipped
// [[album:<slug>:<n>]] into every reader, and leaked the album's address with
// it.
//
// The phase's own ordering gate could never have caught this. It measures
// positions INSIDE pageContent, so it cannot see a second renderer that never
// calls album.Expand at all.

func feedOf(t *testing.T, h *Handler, ws *domain.Website) string {
	t.Helper()
	rec, err := request(h.HandleFeed, ws, "GET", "/feed.xml")
	if err != nil {
		t.Fatalf("HandleFeed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("feed status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

// TestTheFeedDoesNotShipTheRawAlbumMarker is the requirement, both directions.
//
// The marker must be gone AND the pictures must be there — the first alone is
// satisfied by an expansion that drops the block, which is the same silence in
// a different costume.
func TestTheFeedDoesNotShipTheRawAlbumMarker(t *testing.T) {
	h, _, ws, albums, a := albumFixture(t)
	ctx := context.Background()

	// A caption, so the assertion can be about the album's own content and not
	// merely about the absence of a bracket.
	pics, err := albums.Pictures(ctx, ws.ID, a.ID)
	if err != nil || len(pics) == 0 {
		t.Fatalf("Pictures: %v, %v", pics, err)
	}
	if err := albums.UpdateItem(ctx, ws.ID, a.ID, pics[0].ID,
		pics[0].Item.MediaID, "Ein Schrank", "Eiche, geoelt"); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}

	body := feedOf(t, h, ws)

	// The marker survives the XML escaping unchanged — [[ and ]] are not
	// special in chardata — so a plain Contains is the honest check.
	if marker := block.AlbumMarker(a.Slug, 0); strings.Contains(body, marker) {
		t.Errorf("the feed carries the raw marker %s; every subscriber's reader shows it, and it leaks the album's address:\n%s",
			marker, body)
	}
	if strings.Contains(body, "[[album:") {
		t.Errorf("the feed carries an album marker at some other block position:\n%s", body)
	}
	if !strings.Contains(body, "Eiche, geoelt") {
		t.Errorf("the feed does not carry the album's pictures either — the marker was dropped rather than expanded:\n%s", body)
	}
}

// TestTheFeedSurvivesAnUnwiredAlbumStore is the other half of the same rule.
//
// A build with no album store, or one whose query failed, must still not print
// the marker: an empty gallery costs its own block, a visible marker costs the
// post's credibility. album.Expand over a zero Set does exactly that, which is
// the shape loadSnippets has always had.
func TestTheFeedSurvivesAnUnwiredAlbumStore(t *testing.T) {
	h, database, ws, _, a := albumFixture(t)

	// Take the store away, which is both "not built with albums" and, in its
	// effect on this path, "the query failed".
	h.SetAlbumStore(nil)
	_ = database

	body := feedOf(t, h, ws)
	if strings.Contains(body, block.AlbumMarker(a.Slug, 0)) {
		t.Errorf("with no album store the feed printed the raw marker instead of nothing:\n%s", body)
	}
}
