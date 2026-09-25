package ai

import (
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// The media and album tools, wired to a stand-in for the admin handler. The
// real Op methods — the upload through media.Check, the crop — are driven end
// to end in internal/admin (ops_media_test.go), which can import this package;
// this one cannot import that one. What is tested here is what this file owns:
// which key may do what, on which website, and that a destructive call asks
// for confirmation.

// fakeMediaOps is the admin handler as far as these tools need it. The album
// functions go to the real album store, with the admin's picture check.
type fakeMediaOps struct {
	media  *media.Store
	albums *album.Store
	logged []activity.Entry
}

func (f *fakeMediaOps) OpLog(_ context.Context, actor string, e activity.Entry) {
	e.ActorEmail = actor
	f.logged = append(f.logged, e)
}
func (f *fakeMediaOps) OpChanged(int64)        {}
func (f *fakeMediaOps) OpAlbums() *album.Store { return f.albums }

func (f *fakeMediaOps) own(ctx context.Context, websiteID, mediaID int64) error {
	m, err := f.media.GetByID(ctx, mediaID)
	if err != nil || m == nil || m.WebsiteID != websiteID || !m.IsImage() {
		return errors.New("This image does not belong to this website’s media library")
	}
	return nil
}

func (f *fakeMediaOps) OpAddAlbumPicture(ctx context.Context, websiteID, albumID, mediaID int64, alt, caption string) (int64, error) {
	if err := f.own(ctx, websiteID, mediaID); err != nil {
		return 0, err
	}
	return f.albums.AddItem(ctx, websiteID, albumID, mediaID, alt, caption)
}

func (f *fakeMediaOps) OpUpdateAlbumPicture(ctx context.Context, websiteID, albumID, itemID, mediaID int64, alt, caption string) error {
	if err := f.own(ctx, websiteID, mediaID); err != nil {
		return err
	}
	return f.albums.UpdateItem(ctx, websiteID, albumID, itemID, mediaID, alt, caption)
}

func (f *fakeMediaOps) OpMoveAlbumPicture(ctx context.Context, websiteID, albumID, itemID int64, direction string) (bool, error) {
	pictures, err := f.albums.Pictures(ctx, websiteID, albumID)
	if err != nil {
		return false, err
	}
	for i, p := range pictures {
		if p.ID != itemID {
			continue
		}
		switch {
		case direction == "up" && i > 0:
			return true, f.albums.SwapSortOrder(ctx, websiteID, albumID, itemID, pictures[i-1].ID)
		case direction == "down" && i < len(pictures)-1:
			return true, f.albums.SwapSortOrder(ctx, websiteID, albumID, itemID, pictures[i+1].ID)
		}
	}
	return false, nil
}

type mediaFixture struct {
	ts                   *httptest.Server
	database             *db.DB
	ops                  *fakeMediaOps
	eins, zwei           int64
	write, read, fremd   string
	picture, film, alien int64
}

func setUpMedia(t *testing.T, ops any) mediaFixture {
	t.Helper()
	database := newTestDB(t)
	ctx := context.Background()
	domains := domain.NewStore(database)
	eins, _ := domains.CreateWebsite(ctx, "Eins", "")
	zwei, _ := domains.CreateWebsite(ctx, "Zwei", "")

	tokens := NewStore(database)
	write, _, err := tokens.Issue(ctx, "schreibend", 0, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	read, _, _ := tokens.Issue(ctx, "lesend", 0, false, 0)
	fremd, _, _ := tokens.Issue(ctx, "nur zwei", zwei.ID, true, 0)

	mediaStore := media.NewStore(database)
	fake := &fakeMediaOps{media: mediaStore, albums: album.NewStore(database)}
	if ops == nil {
		ops = fake
	}
	picture, err := mediaStore.Create(ctx, eins.ID, "a.jpg", "hof.jpg", "image/jpeg", 10, "h1")
	if err != nil {
		t.Fatal(err)
	}
	film, _ := mediaStore.Create(ctx, eins.ID, "b.mp4", "film.mp4", "video/mp4", 10, "h2")
	alien, _ := mediaStore.Create(ctx, zwei.ID, "c.jpg", "fremd.jpg", "image/jpeg", 10, "h3")

	ts := httptest.NewServer(NewServer(tokens, "Test", slog.New(slog.DiscardHandler), Tools(Deps{
		Domains: domains, Pages: page.NewStore(database), Media: mediaStore,
		Ops:    ops,
		Limits: Limits{DataDir: t.TempDir(), MaxMediaSize: 1 << 20, MaxVideoSize: 1 << 20, MaxMegapixels: 24},
	})))
	t.Cleanup(ts.Close)
	return mediaFixture{
		ts: ts, database: database, ops: fake, eins: eins.ID, zwei: zwei.ID,
		write: write, read: read, fremd: fremd,
		picture: picture.ID, film: film.ID, alien: alien.ID,
	}
}

func mustTool(t *testing.T, f mediaFixture, key, name string, args map[string]any) map[string]any {
	t.Helper()
	out, failed := callTool(t, f.ts, key, name, args)
	if failed {
		t.Fatalf("%s: %v", name, out["text"])
	}
	return out
}

func refusedTool(t *testing.T, f mediaFixture, key, name string, args map[string]any) string {
	t.Helper()
	out, failed := callTool(t, f.ts, key, name, args)
	if !failed {
		t.Fatalf("%s was not refused: %v", name, out)
	}
	text, _ := out["text"].(string)
	return text
}

func TestMediaToolsDescribeAndFocusAPicture(t *testing.T) {
	f := setUpMedia(t, nil)

	out := mustTool(t, f, f.write, "update_media", map[string]any{
		"id": f.picture, "alt_text": "  Der Hof im Winter ", "caption": "Januar",
	})
	if out["description"] != "Der Hof im Winter" || out["caption"] != "Januar" {
		t.Errorf("update_media answered %v", out)
	}
	// Only the caption: the description stays.
	out = mustTool(t, f, f.write, "update_media", map[string]any{"id": f.picture, "caption": ""})
	if out["description"] != "Der Hof im Winter" || out["caption"] != "" {
		t.Errorf("a missing field was not left alone: %v", out)
	}

	out = mustTool(t, f, f.write, "set_media_focus", map[string]any{"id": f.picture, "x": 20, "y": 80})
	focus := out["focus"].(map[string]any)
	if focus["x"] != float64(20) || focus["y"] != float64(80) {
		t.Errorf("focus = %v", focus)
	}
	refusedTool(t, f, f.write, "set_media_focus", map[string]any{"id": f.picture, "x": 120, "y": 0})
	refusedTool(t, f, f.write, "set_media_focus", map[string]any{"id": f.film, "x": 10, "y": 10})

	got := mustTool(t, f, f.read, "get_media", map[string]any{"id": f.picture})
	if got["description"] != "Der Hof im Winter" {
		t.Errorf("get_media = %v", got)
	}
	if len(f.ops.logged) != 3 || !strings.HasPrefix(f.ops.logged[0].ActorEmail, "KI: ") {
		t.Errorf("activity log: %+v", f.ops.logged)
	}
}

func TestMediaToolsStayWithTheirWebsite(t *testing.T) {
	f := setUpMedia(t, nil)

	// The key for website two reaches none of website one's files.
	for _, call := range []struct {
		name string
		args map[string]any
	}{
		{"get_media", map[string]any{"id": f.picture}},
		{"update_media", map[string]any{"id": f.picture, "alt_text": "x"}},
		{"set_media_focus", map[string]any{"id": f.picture, "x": 1, "y": 1}},
		{"crop_media", map[string]any{"id": f.picture, "ratio": "1-1"}},
		{"delete_media", map[string]any{"id": f.picture, "confirm": true, "force": true}},
		{"get_media_library", map[string]any{"website": f.eins}},
		{"list_media_collection", map[string]any{"website": f.eins}},
		{"list_albums", map[string]any{"website": f.eins}},
		{"create_album", map[string]any{"website": f.eins, "name": "Fremd"}},
		{"upload_media", map[string]any{"website": f.eins, "file_name": "a.png", "content_base64": "AAAA"}},
	} {
		refusedTool(t, f, f.fremd, call.name, call.args)
	}

	// An album of website one, named through website two, does not exist.
	al := mustTool(t, f, f.write, "create_album", map[string]any{"website": f.eins, "name": "Sommer"})
	albumID := al["id"]
	refusedTool(t, f, f.fremd, "get_album", map[string]any{"website": f.zwei, "id": albumID})
	refusedTool(t, f, f.fremd, "delete_album", map[string]any{"website": f.zwei, "id": albumID, "confirm": true})

	// And website two's picture cannot be put into website one's album.
	out := mustTool(t, f, f.write, "add_album_images", map[string]any{
		"website": f.eins, "album": albumID,
		"images": []any{map[string]any{"media": f.alien}, map[string]any{"media": f.picture}},
	})
	if out["added"] != float64(1) || len(out["refused"].([]any)) != 1 {
		t.Errorf("a foreign picture went into the album: %v", out)
	}
}

func TestAReadOnlyKeyCannotChangeTheLibrary(t *testing.T) {
	f := setUpMedia(t, nil)
	for _, name := range []string{
		"upload_media", "update_media", "set_media_focus", "crop_media",
		"restore_media_original", "delete_media", "create_album", "rename_album",
		"delete_album", "add_album_images", "update_album_image",
		"remove_album_image", "move_album_image",
	} {
		refusedTool(t, f, f.read, name, map[string]any{"id": f.picture, "website": f.eins})
	}
	// Reading is fine.
	lib := mustTool(t, f, f.read, "get_media_library", map[string]any{"website": f.eins})
	c := lib["collections"].(map[string]any)
	if c["all"] != float64(2) || c["images"] != float64(1) || c["videos"] != float64(1) || c["no_description"] != float64(1) {
		t.Errorf("collections = %v", c)
	}
	list := mustTool(t, f, f.read, "list_media_collection", map[string]any{"website": f.eins, "collection": "no_description"})
	if list["total"] != float64(1) {
		t.Errorf("no_description = %v", list)
	}
}

func TestDeletingAFileAsksAndNamesThePages(t *testing.T) {
	f := setUpMedia(t, nil)
	ctx := context.Background()

	p, err := page.NewStore(f.database).CreatePage(ctx, page.PageCreate{
		WebsiteID: f.eins, Title: "Startseite", Slug: "start", Status: "draft",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := media.NewStore(f.database).ReplaceUsage(ctx, p.ID, []int64{f.picture}); err != nil {
		t.Fatal(err)
	}

	got := mustTool(t, f, f.read, "get_media", map[string]any{"id": f.picture})
	if pages := got["used_on_pages"].([]any); len(pages) != 1 || pages[0] != "Startseite" {
		t.Errorf("used_on_pages = %v", got["used_on_pages"])
	}

	if msg := refusedTool(t, f, f.write, "delete_media", map[string]any{"id": f.picture}); !strings.Contains(msg, "confirm") {
		t.Errorf("without confirm: %q", msg)
	}
	if msg := refusedTool(t, f, f.write, "delete_media", map[string]any{"id": f.picture, "confirm": true}); !strings.Contains(msg, "Startseite") {
		t.Errorf("the page is not named: %q", msg)
	}
	mustTool(t, f, f.write, "delete_media", map[string]any{"id": f.picture, "confirm": true, "force": true})
	if m, _ := media.NewStore(f.database).GetByID(ctx, f.picture); m != nil {
		t.Error("the file is still there")
	}
	// A file on no page goes with confirm alone.
	mustTool(t, f, f.write, "delete_media", map[string]any{"id": f.film, "confirm": true})
}

func TestAnAlbumAllTheWayThrough(t *testing.T) {
	f := setUpMedia(t, nil)
	ctx := context.Background()
	store := media.NewStore(f.database)
	second, _ := store.Create(ctx, f.eins, "d.png", "zwei.png", "image/png", 10, "h4")
	third, _ := store.Create(ctx, f.eins, "e.png", "drei.png", "image/png", 10, "h5")

	al := mustTool(t, f, f.write, "create_album", map[string]any{"website": f.eins, "name": "Sommer 2026"})
	id := al["id"]
	if al["slug"] != "sommer-2026" {
		t.Errorf("slug = %v", al["slug"])
	}
	refusedTool(t, f, f.write, "create_album", map[string]any{"website": f.eins, "name": "Sommer 2026"})

	out := mustTool(t, f, f.write, "add_album_images", map[string]any{
		"website": f.eins, "album": id, "images": []any{
			map[string]any{"media": f.picture, "alt_text": "Hof"},
			map[string]any{"media": second.ID},
			map[string]any{"media": third.ID},
			map[string]any{"media": f.film},
		},
	})
	if out["added"] != float64(3) || len(out["refused"].([]any)) != 1 {
		t.Fatalf("add_album_images = %v", out)
	}
	pictures := out["pictures"].([]any)
	first := pictures[0].(map[string]any)
	last := pictures[2].(map[string]any)

	// The last to the front, by position.
	out = mustTool(t, f, f.write, "move_album_image", map[string]any{
		"website": f.eins, "album": id, "item": last["item"], "position": 1,
	})
	order := out["pictures"].([]any)
	if order[0].(map[string]any)["media"] != float64(third.ID) || order[1].(map[string]any)["media"] != float64(f.picture) {
		t.Errorf("order after the move: %v", order)
	}
	// Upwards from the top is not an error, just nothing.
	out = mustTool(t, f, f.write, "move_album_image", map[string]any{
		"website": f.eins, "album": id, "item": last["item"], "direction": "up",
	})
	if out["note"] == nil {
		t.Errorf("a move that changed nothing says nothing: %v", out)
	}

	mustTool(t, f, f.write, "update_album_image", map[string]any{
		"website": f.eins, "album": id, "item": first["item"], "caption": "Im Januar",
	})
	got := mustTool(t, f, f.read, "get_album", map[string]any{"website": f.eins, "id": id})
	for _, raw := range got["pictures"].([]any) {
		p := raw.(map[string]any)
		if p["item"] == first["item"] && (p["caption"] != "Im Januar" || p["alt_text"] != "Hof") {
			t.Errorf("update_album_image: %v", p)
		}
	}
	refusedTool(t, f, f.write, "update_album_image", map[string]any{
		"website": f.eins, "album": id, "item": first["item"], "media": f.film,
	})

	out = mustTool(t, f, f.write, "remove_album_image", map[string]any{
		"website": f.eins, "album": id, "item": first["item"],
	})
	if len(out["pictures"].([]any)) != 2 {
		t.Errorf("remove_album_image: %v", out)
	}
	if m, _ := store.GetByID(ctx, f.picture); m == nil {
		t.Error("taking a picture out of an album deleted the file")
	}

	file := mustTool(t, f, f.read, "get_media", map[string]any{"id": second.ID})
	if in := file["in_albums"].([]any); len(in) != 1 {
		t.Errorf("in_albums = %v", file["in_albums"])
	}

	renamed := mustTool(t, f, f.write, "rename_album", map[string]any{"website": f.eins, "id": id, "name": "Sommer am See"})
	if renamed["name"] != "Sommer am See" || renamed["slug"] != "sommer-2026" {
		t.Errorf("rename_album = %v", renamed)
	}
	list := mustTool(t, f, f.read, "list_albums", map[string]any{"website": f.eins})
	if a := list["albums"].([]any); len(a) != 1 || a[0].(map[string]any)["pictures"] != float64(2) {
		t.Errorf("list_albums = %v", list)
	}

	refusedTool(t, f, f.write, "delete_album", map[string]any{"website": f.eins, "id": id})
	mustTool(t, f, f.write, "delete_album", map[string]any{"website": f.eins, "id": id, "confirm": true})
	list = mustTool(t, f, f.read, "list_albums", map[string]any{"website": f.eins})
	if len(list["albums"].([]any)) != 0 {
		t.Errorf("the album is still there: %v", list)
	}
}

// Without the admin handler behind it, the upload says so instead of
// panicking or storing a file past the checks.
func TestUploadWithoutTheAdminSaysSo(t *testing.T) {
	f := setUpMedia(t, struct{}{})
	msg := refusedTool(t, f, f.write, "upload_media", map[string]any{
		"website": f.eins, "file_name": "a.png", "content_base64": "iVBORw0KGgo=",
	})
	if !strings.Contains(msg, "not available") {
		t.Errorf("upload without Ops: %q", msg)
	}
	msg = refusedTool(t, f, f.write, "list_albums", map[string]any{"website": f.eins})
	if !strings.Contains(msg, "not available") {
		t.Errorf("albums without Ops: %q", msg)
	}
}

func TestBase64IsReadInEverySpelling(t *testing.T) {
	for _, in := range []string{"aGFsbG8=", "aGFsbG8", "data:image/png;base64,aGFsbG8=", "aGFs\nbG8="} {
		b, err := decodeBase64(in)
		if err != nil || string(b) != "hallo" {
			t.Errorf("%q: %q, %v", in, b, err)
		}
	}
	if _, err := decodeBase64("nicht base64!"); err == nil {
		t.Error("garbage was decoded")
	}
}
