package public

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// What a visitor is shown when the albums cannot be loaded at all.
//
// There are two such states and neither is exotic: the query fails, or the
// build has no album store wired. In both the page still has to be served, and
// the only question is what stands where the pictures were.
//
// The rule is snippet.Expand's and it is the one this package already follows
// everywhere else: an internal marker never reaches a reader. loadSnippets logs
// its error and returns an EMPTY map, and snippet.Expand still runs — so every
// marker becomes "" and none survives. The album path used to do the opposite:
// it skipped the expansion, and [[album:moebel:0]] was printed to the visitor
// on every request, for ever, with no recovery.
//
// The doc comment that defended it argued "a failing album must cost its own
// block and never the page". That is exactly backwards. Leaving the marker
// costs the PAGE — a bracketed token in the middle of an article, on every
// gallery the site has, plus the album's internal address. Expanding to nothing
// costs the BLOCK — the gallery is missing, the article around it reads.

// TestAFailingAlbumQueryLeavesNoMarkerOnThePage is the query-failed half.
//
// The album store is pointed at a closed database, so LoadFor genuinely fails;
// everything else — the page, the menus, the theme — runs off the good one.
// That is the shape of the real incident: the albums are one query among many
// and it is the only one that went wrong.
func TestAFailingAlbumQueryLeavesNoMarkerOnThePage(t *testing.T) {
	h, database, ws, _, a := albumFixture(t)
	h.SetAlbumStore(album.NewStore(brokenDB(t)))

	stored, _ := storedPage(t, database, "galerie")
	if !strings.Contains(stored, block.AlbumMarker(a.Slug, 0)) {
		t.Fatalf("the fixture does not carry the marker, so this test proves nothing:\n%s", stored)
	}

	body := fetch(t, h, ws, "galerie")
	if strings.Contains(body, "[[album:") {
		t.Errorf("the visitor is served the raw marker although the album query failed:\n%s",
			markerLine(body))
	}
	// The page around it still has to be there — expanding to nothing must not
	// become "serve nothing".
	if !strings.Contains(body, "Galerie") {
		t.Errorf("the page itself did not render:\n%s", body)
	}
}

// TestAPageWithAnAlbumMarkerAndNoStoreLeavesNoMarker is the unwired half.
//
// A build whose album store was never set still serves pages whose HTML was
// written when it was — an older binary rolled back onto the same data
// directory is the concrete case.
func TestAPageWithAnAlbumMarkerAndNoStoreLeavesNoMarker(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	seedAlbumPage(t, database, ws.ID, "galerie", block.Block{
		Type: block.TypeGallery, AlbumSlug: "moebel",
	})

	body := fetch(t, h, ws, "galerie")
	if strings.Contains(body, "[[album:") {
		t.Errorf("a build with no album store printed the raw marker to a visitor:\n%s",
			markerLine(body))
	}
}

// brokenDB is a database that is open enough to be handed to a store and
// closed enough that every statement against it fails.
func brokenDB(t *testing.T) *db.DB {
	t.Helper()
	broken, err := db.Open(filepath.Join(t.TempDir(), "broken.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	broken.Close()
	return broken
}

// markerLine cuts the served body down to what is worth reading in a failure.
func markerLine(body string) string {
	at := strings.Index(body, "[[album:")
	if at < 0 {
		return body
	}
	from, to := at-120, at+120
	if from < 0 {
		from = 0
	}
	if to > len(body) {
		to = len(body)
	}
	return "…" + body[from:to] + "…"
}
