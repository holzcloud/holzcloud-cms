package public

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestChangingAnAlbumMovesTheFeedsDates is GAL-03 on the feed's two dates.
//
// The entries already carried the album's new pictures, and the ETag moved with
// them. The dates did not: HandleFeed took newest from the pages and the
// snippets and never from album.Set.Latest(), so <updated> and Last-Modified
// kept the old moment after an album changed. A reader that trusts the feed's
// date has no reason to look again, and neither has a cache that sends only a
// date. The page path had this fixed and held by
// TestChangingAnAlbumMovesThePagesLastModified; the feed is the second renderer
// that was not told.
//
// Found by the v1.6 milestone audit. Same fixed stamp in the future as the page
// test, so the assertion is about which value was chosen and not about how
// fast the machine is.
func TestChangingAnAlbumMovesTheFeedsDates(t *testing.T) {
	h, database, ws, _, a := albumFixture(t)

	moved := "2031-04-05T06:07:08Z"
	if _, err := database.Write.Exec(
		`UPDATE albums SET updated_at = $1 WHERE id = $2`, moved, a.ID); err != nil {
		t.Fatalf("move the album's stamp: %v", err)
	}

	rec, err := request(h.HandleFeed, ws, "GET", "/feed.xml")
	if err != nil {
		t.Fatalf("HandleFeed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("feed status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	// The entry's own <updated> is the page's and stays so; only the feed's
	// element can carry 2031, so a Contains cannot be satisfied by the entry.
	if !strings.Contains(body, "<updated>"+moved+"</updated>") {
		t.Errorf("the feed's <updated> does not carry the album's stamp %s — a reader keeps the old gallery:\n%s",
			moved, body)
	}
	want, err := time.Parse(time.RFC3339, moved)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := rec.Header().Get("Last-Modified"); got != want.UTC().Format(http.TimeFormat) {
		t.Errorf("Last-Modified = %q; want the album's own stamp %q", got, want.UTC().Format(http.TimeFormat))
	}
}
