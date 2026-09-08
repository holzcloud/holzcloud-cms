package public

import (
	"context"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// The door the film actually comes through, driven end to end.
//
// album.Store.AddItem checks the website and deliberately not the file type:
// internal/admin's requireOwnPicture is where "a film chosen where a photo
// belongs" is caught, and its comment says the store correctly has no opinion.
// The bundle importer bypasses that handler — it calls AddItem directly and
// resolves the picture through mediaByName, which can name an mp4 in the
// archive's media list — so an album really can hold a film, and until the
// renderer grew the guard the visitor was served <img src="…film.mp4">.
//
// This test does not go through the importer, because the importer is not the
// property: the property is that whatever is in the album, no film reaches the
// page as a picture. It puts the film in through the same store call the
// importer makes.
func TestAFilmInAnAlbumIsNotServedAsAPicture(t *testing.T) {
	h, database, ws, albums, a := albumFixture(t)
	ctx := context.Background()

	film, err := media.NewStore(database).Create(ctx, ws.ID, "hobeln.mp4", "hobeln.mp4",
		"video/mp4", 40960, "hobeln-hash")
	if err != nil {
		t.Fatalf("media.Create film: %v", err)
	}
	if _, err := albums.AddItem(ctx, ws.ID, a.ID, film.ID, "Ein Film", ""); err != nil {
		t.Fatalf("AddItem film: %v — the premise of this test is that the store allows it", err)
	}

	body := fetch(t, h, ws, "galerie")
	if strings.Contains(body, "hobeln.mp4") {
		t.Errorf("the album's film reached the page:\n%s", body)
	}
	// The album's real pictures are still there — the guard drops the film, not
	// the gallery.
	for _, want := range []string{"schrank.jpg", "tisch.jpg"} {
		if !strings.Contains(body, want) {
			t.Errorf("%q is missing from the served gallery:\n%s", want, body)
		}
	}
}
