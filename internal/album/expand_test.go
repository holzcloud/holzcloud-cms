package album

import (
	"context"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/block"
)

// twoPictures is a Set holding one album of two pictures, built by hand so the
// expansion can be exercised without a database at all.
func twoPictures() Set {
	return Set{
		items: map[string][]block.Item{
			"moebel": {
				{MediaID: 1, Caption: "Der Schrank"},
				{MediaID: 2},
			},
		},
		images: map[int64]block.Image{
			1: {URL: "/media/1/schrank.jpg", Alt: "Schrank", Width: 1200, Height: 800},
			2: {URL: "/media/1/tisch.jpg", Alt: "Tisch", Width: 1200, Height: 800},
		},
	}
}

// The whole point of the mechanism in one assertion: what is stored is a
// marker, what is served is the pictures, and the two are separated by a
// request.
func TestExpandReplacesTheMarker(t *testing.T) {
	stored := `<div class="hc-block hc-galerie hc-spalten-3">` +
		block.AlbumMarker("moebel", 0) + `</div>`

	got := Expand(stored, twoPictures())

	if strings.Contains(got, "[[album:") {
		t.Errorf("the marker is still on the page a visitor sees:\n%s", got)
	}
	if !strings.Contains(got, "/media/1/schrank.jpg") || !strings.Contains(got, "/media/1/tisch.jpg") {
		t.Errorf("the album's pictures did not arrive:\n%s", got)
	}
	if !strings.HasPrefix(got, `<div class="hc-block hc-galerie hc-spalten-3">`) {
		t.Errorf("the block's own wrapper was not left alone:\n%s", got)
	}
	// The tiles and the large views are GalleryItems' output and not a second
	// rendering of the same idea — that is GAL-07's one mechanism.
	if !strings.Contains(got, `class="hc-galerie__bild"`) ||
		!strings.Contains(got, `class="hc-galerie__gross"`) {
		t.Errorf("the album was not rendered by the gallery renderer:\n%s", got)
	}
}

// A marker naming an album that is not there expands to nothing, exactly as
// snippet.Expand does and for the reason it states: a visitor must not see the
// internal syntax on a live page, and a deleted album should cost its own block
// and not the article around it.
func TestExpandDropsAnUnknownSlug(t *testing.T) {
	stored := `<p>davor</p><div class="hc-galerie">` +
		block.AlbumMarker("weg", 1) + `</div><p>danach</p>`

	got := Expand(stored, twoPictures())

	if strings.Contains(got, "[[album:") || strings.Contains(got, "weg") {
		t.Errorf("the syntax survived onto the page:\n%s", got)
	}
	if !strings.Contains(got, "davor") || !strings.Contains(got, "danach") {
		t.Errorf("the article around the block was damaged:\n%s", got)
	}
	if !strings.Contains(got, `<div class="hc-galerie"></div>`) {
		t.Errorf("the empty gallery wrapper is not what is left:\n%s", got)
	}
}

// The early return is what keeps this mechanism free for the pages that do not
// use it: identical, not merely equal.
func TestExpandLeavesADocumentWithNoMarkerIdentical(t *testing.T) {
	const stored = `<h1>Über uns</h1><p>Wir bauen Möbel [[snippet:kontakt]].</p>`

	if got := Expand(stored, twoPictures()); got != stored {
		t.Errorf("a page naming no album was rewritten:\nwant %s\ngot  %s", stored, got)
	}
	if got := Expand(stored, Set{}); got != stored {
		t.Errorf("an empty set rewrote a page naming no album:\n%s", got)
	}
}

// One album named three times is one query, not three. This is the half of
// "load only what the page names" that a test can hold; the other half is the
// early return in LoadFor below.
func TestUsedSlugsDeduplicates(t *testing.T) {
	doc := block.AlbumMarker("moebel", 0) + "<p>x</p>" +
		block.AlbumMarker("sommer", 2) + block.AlbumMarker("moebel", 4)

	got := UsedSlugs(doc)
	want := []string{"moebel", "sommer"}

	if len(got) != len(want) {
		t.Fatalf("UsedSlugs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("slug %d = %q, want %q", i, got[i], want[i])
		}
	}
	if UsedSlugs("<p>nothing</p>") != nil {
		t.Error("a page naming no album produced slugs to look up")
	}
}

// The fragment ids come from the position the marker carries, so a page with an
// inline gallery at block 0 and an album gallery at block 3 mints two distinct
// sets and the browser jumps to the picture that was clicked.
func TestExpandedIdsStartFromTheMarkersPosition(t *testing.T) {
	got := Expand(block.AlbumMarker("moebel", 3), twoPictures())

	if !strings.Contains(got, `id="hc-b4-p1"`) || !strings.Contains(got, `id="hc-b4-p2"`) {
		t.Errorf("the ids do not start from the marker's position:\n%s", got)
	}
	if strings.Contains(got, `id="hc-b1-p1"`) {
		t.Errorf("the album minted the ids of the first block:\n%s", got)
	}
}

// A page that names no album must issue no query at all. Asserted with a store
// that has no database: if LoadFor reached for one it would panic, and the only
// reason it does not is the early return on the slugs the HTML does not carry.
func TestLoadForIssuesNoQueryWhenTheHTMLNamesNoAlbum(t *testing.T) {
	set, err := (&Store{}).LoadFor(context.Background(), 1, `<p>keine Galerie</p>`, nil)
	if err != nil {
		t.Fatalf("LoadFor: %v", err)
	}
	if len(set.items) != 0 {
		t.Errorf("a page naming no album came back with %d albums", len(set.items))
	}
}

// The translator reaches the album's controls, so a page carrying an inline
// gallery and an album gallery does not end up with its two sets of controls in
// two languages.
func TestExpandUsesTheInjectedTranslator(t *testing.T) {
	set := twoPictures()
	set.t = strings.ToUpper

	got := Expand(block.AlbumMarker("moebel", 0), set)

	if !strings.Contains(got, "NEXT IMAGE") {
		t.Errorf("the controls did not go through the translator:\n%s", got)
	}
}
