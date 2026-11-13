package admin

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// A file in use is part of what the public sees; a file nobody uses is not.
//
// Uploading stays open to an editor without the publishing right, and so does
// every change to a file nobody shows (D-06): a file in the library is on no
// page until somebody puts it there. A file that a page, a snippet, an album,
// the website's logo or a product shows is changed or deleted only by somebody
// who may publish. The album case is the sharp one: album_items cascades away
// when its file is deleted, so a delete that asked media_usage alone would
// empty a live gallery without a word.

// mediaInUseText is the refusal such an editor reads at a file in use. Tests
// run in English, so the rendered sentence is the catalogue key.
const mediaInUseText = "This file is in use on the website. Changing or deleting it is not part of your account – ask somebody who may publish."

// wantMediaRefused is wantRefused for the media sentence.
func wantMediaRefused(t *testing.T, what string, rec *httptest.ResponseRecorder, flash, to string) {
	t.Helper()
	if rec.Code != http.StatusSeeOther {
		t.Errorf("%s: status %d, want 303", what, rec.Code)
	}
	if got := rec.Header().Get("Location"); got != to {
		t.Errorf("%s: Location %q, want %q", what, got, to)
	}
	if flash != mediaInUseText {
		t.Errorf("%s: flash %q, want the media sentence", what, flash)
	}
	if !strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "HX-Request") {
		t.Errorf("%s: Vary %q does not name HX-Request", what, rec.Header().Values("Vary"))
	}
}

// wantMediaRefusedHtmx is wantRefusedHtmx for the media sentence.
func wantMediaRefusedHtmx(t *testing.T, what string, rec *httptest.ResponseRecorder, flash, to string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Errorf("%s (htmx): status %d, want 200", what, rec.Code)
	}
	if got := rec.Header().Get("HX-Redirect"); got != to {
		t.Errorf("%s (htmx): HX-Redirect %q, want %q", what, got, to)
	}
	if flash != mediaInUseText {
		t.Errorf("%s (htmx): flash %q, want the media sentence", what, flash)
	}
	if !strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "HX-Request") {
		t.Errorf("%s (htmx): Vary %q does not name HX-Request", what, rec.Header().Values("Vary"))
	}
}

func mediaListAddress(websiteID int64) string {
	return fmt.Sprintf("/admin/websites/%d/media", websiteID)
}

func mediaAddress(websiteID, mediaID int64) string {
	return fmt.Sprintf("/admin/websites/%d/media/%d", websiteID, mediaID)
}

func mediaRoute(websiteID, mediaID int64) map[string]string {
	return websiteRoute(websiteID, "mediaID", id64(mediaID))
}

// mediaStillThere reports whether the file's row survived.
func (f *albumFixture) mediaStillThere(t *testing.T, mediaID int64) bool {
	t.Helper()
	m, err := f.h.mediaStore.GetByID(context.Background(), mediaID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	return m != nil
}

func TestAnEditorWithoutPublishingCannotChangeAFileInUse(t *testing.T) {
	t.Run("delete", func(t *testing.T) {
		f := newAlbumFixture(t)
		h, sm, ws := f.h, f.sm, f.siteA
		editor := editorWithoutPublishing(t, f.db, ws.ID)
		f.mustAdd(t, f.albumA.ID, f.mediaA, "Hobelbank")
		route := mediaRoute(ws.ID, f.mediaA)
		target := mediaAddress(ws.ID, f.mediaA) + "/delete"
		back := mediaListAddress(ws.ID)

		rec, flash := serveAs(t, h, sm, editor, h.HandleMediaDelete, postForm(target, url.Values{}, route))
		wantMediaRefused(t, "delete", rec, flash, back)
		// ?force=1 is the publisher's "yes, break those pages"; it is read
		// after the question, so it cannot answer it.
		rec, flash = serveAs(t, h, sm, editor, h.HandleMediaDelete, postForm(target+"?force=1", url.Values{}, route))
		wantMediaRefused(t, "delete ?force=1", rec, flash, back)
		rec, flash = serveAs(t, h, sm, editor, h.HandleMediaDelete, postForm(target, url.Values{"force": {"1"}}, route))
		wantMediaRefused(t, "delete force=1 form", rec, flash, back)
		rec, flash = serveAs(t, h, sm, editor, h.HandleMediaDelete, htmx(postForm(target+"?force=1", url.Values{}, route)))
		wantMediaRefusedHtmx(t, "delete", rec, flash, back)

		if !f.mediaStillThere(t, f.mediaA) {
			t.Fatal("the file an album shows was deleted")
		}
		if n := len(f.pictures(t, f.albumA.ID)); n != 1 {
			t.Errorf("%d pictures in the album, want 1", n)
		}

		// The control: somebody who may publish deletes it, exactly as before.
		publisher := editorWhoMayPublish(t, f.db, ws.ID)
		rec, flash = serveAs(t, h, sm, publisher, h.HandleMediaDelete, postForm(target+"?force=1", url.Values{}, route))
		if rec.Code != http.StatusSeeOther || flash != "" {
			t.Errorf("publisher: status %d (%q), want 303 and no error", rec.Code, flash)
		}
		if f.mediaStillThere(t, f.mediaA) {
			t.Error("publisher: the file is still there")
		}
	})

	// The remaining doors are held with a file a text snippet shows: a use
	// media_usage cannot see, so a guard that asked only that table would let
	// every one of them through.

	t.Run("describe", func(t *testing.T) {
		h, sm, database, ws := mediaAdmin(t)
		editor := editorWithoutPublishing(t, database, ws.ID)
		m := seedImage(t, h, ws.ID, "bank.jpg", 1600, 600)
		if err := h.mediaStore.UpdateMeta(context.Background(), m.ID, "Hobelbank", "In der Werkstatt"); err != nil {
			t.Fatalf("UpdateMeta: %v", err)
		}
		showInASnippet(t, database, ws.ID, m.Filename)
		target := mediaAddress(ws.ID, m.ID) + "/meta"
		values := url.Values{"alt_text": {"Etwas anderes"}, "caption": {"Neu"}}

		rec, flash := serveAs(t, h, sm, editor, h.HandleMediaMeta, postForm(target, values, mediaRoute(ws.ID, m.ID)))
		wantMediaRefused(t, "describe", rec, flash, mediaListAddress(ws.ID))
		rec, flash = serveAs(t, h, sm, editor, h.HandleMediaMeta, htmx(postForm(target, values, mediaRoute(ws.ID, m.ID))))
		wantMediaRefusedHtmx(t, "describe", rec, flash, mediaListAddress(ws.ID))

		after := mustGetMedia(t, h, m.ID)
		if after.AltText != "Hobelbank" || after.Caption != "In der Werkstatt" {
			t.Errorf("the description of a file in use changed: %q / %q", after.AltText, after.Caption)
		}
	})

	t.Run("crop", func(t *testing.T) {
		h, sm, database, ws := mediaAdmin(t)
		editor := editorWithoutPublishing(t, database, ws.ID)
		m := seedImage(t, h, ws.ID, "bank.jpg", 1600, 600)
		showInASnippet(t, database, ws.ID, m.Filename)
		before := mediaBytes(t, h, m)

		rec, flash := serveAs(t, h, sm, editor, h.HandleMediaCropSave, cropRequest(t, m, url.Values{
			"form": {"1-1"}, "naehe": {"100"}, "drehung": {"0"}, "fokus_x": {"10"}, "fokus_y": {"70"},
		}))
		wantMediaRefused(t, "crop", rec, flash, cropAddress(m))

		after := mustGetMedia(t, h, m.ID)
		if after.IsCropped() || after.Width != 1600 || after.Height != 600 {
			t.Errorf("a file in use was cropped: %d × %d, %+v", after.Width, after.Height, after.Crop)
		}
		if !bytes.Equal(mediaBytes(t, h, m), before) {
			t.Error("the bytes of a file in use changed")
		}
	})

	t.Run("focus", func(t *testing.T) {
		h, sm, database, ws := mediaAdmin(t)
		editor := editorWithoutPublishing(t, database, ws.ID)
		m := seedImage(t, h, ws.ID, "bank.jpg", 1600, 600)
		showInASnippet(t, database, ws.ID, m.Filename)

		// The click on the picture, the way the crop screen sends it.
		rec, flash := serveAs(t, h, sm, editor, h.HandleMediaCropSave, cropRequest(t, m, url.Values{
			"form": {""}, "fokus_x": {"50"}, "fokus_y": {"50"},
			"gezeigt_breite": {"640"}, "gezeigt_hoehe": {"240"},
			"fokus.x": {"160"}, "fokus.y": {"180"},
		}))
		wantMediaRefused(t, "focus", rec, flash, cropAddress(m))

		after := mustGetMedia(t, h, m.ID)
		if after.Crop != m.Crop {
			t.Errorf("the focus of a file in use moved: %+v, was %+v", after.Crop, m.Crop)
		}
	})

	t.Run("original back", func(t *testing.T) {
		h, sm, database, ws := mediaAdmin(t)
		editor := editorWithoutPublishing(t, database, ws.ID)
		m := seedImage(t, h, ws.ID, "bank.jpg", 1600, 600)
		// Cropped first, by a request without a user — the publisher's path.
		serve(t, h, sm, h.HandleMediaCropSave, cropRequest(t, m, url.Values{"form": {"1-1"}, "fokus_x": {"15"}, "fokus_y": {"80"}}))
		cropped := mustGetMedia(t, h, m.ID)
		if !cropped.IsCropped() {
			t.Fatal("the premise: the file is not cropped")
		}
		showInASnippet(t, database, ws.ID, m.Filename)
		before := mediaBytes(t, h, cropped)

		rec, flash := serveAs(t, h, sm, editor, h.HandleMediaCropSave, cropRequest(t, cropped, url.Values{"zuruecksetzen": {"1"}}))
		wantMediaRefused(t, "original back", rec, flash, cropAddress(m))
		rec, flash = serveAs(t, h, sm, editor, h.HandleMediaCropSave, htmx(cropRequest(t, cropped, url.Values{"zuruecksetzen": {"1"}})))
		wantMediaRefusedHtmx(t, "original back", rec, flash, cropAddress(m))

		after := mustGetMedia(t, h, m.ID)
		if !after.IsCropped() || after.Width != cropped.Width || after.Height != cropped.Height {
			t.Errorf("the original of a file in use was put back: %d × %d, %+v", after.Width, after.Height, after.Crop)
		}
		if !bytes.Equal(mediaBytes(t, h, cropped), before) {
			t.Error("the bytes of a file in use changed")
		}
	})
}

// showInASnippet makes a text snippet of the website show the file.
func showInASnippet(t *testing.T, database *db.DB, websiteID int64, filename string) {
	t.Helper()
	if _, err := database.Write.Exec(
		`INSERT INTO snippets (website_id, key, name, content_html) VALUES ($1, $2, $2, $3)`,
		websiteID, "zeigt-"+filename, fmt.Sprintf(`<p><img src="/media/%d/%s" alt=""></p>`, websiteID, filename)); err != nil {
		t.Fatalf("insert snippet: %v", err)
	}
}

func mustGetMedia(t *testing.T, h *Handler, id int64) *media.Media {
	t.Helper()
	m, err := h.mediaStore.GetByID(context.Background(), id)
	if err != nil || m == nil {
		t.Fatalf("GetByID(%d): %v, %v", id, m, err)
	}
	return m
}

// mediaBytes is the served file as it lies on disk.
func mediaBytes(t *testing.T, h *Handler, m *media.Media) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(media.WebsiteDir(h.cfg.DataDir, m.WebsiteID), m.Filename))
	if err != nil {
		t.Fatalf("read %s: %v", m.Filename, err)
	}
	return b
}

func cropAddress(m *media.Media) string {
	return fmt.Sprintf("/admin/websites/%d/media/%d/zuschnitt", m.WebsiteID, m.ID)
}

// pngOf is a real picture of the given size, encoded the way a camera would
// not but a browser accepts: enough for the crop pipeline to decode.
func pngOf(t *testing.T, w, hgt int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, hgt))
	for y := 0; y < hgt; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 6), uint8(y * 8), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

// uploadAs uploads one file as this person and hands back its row.
func uploadAs(t *testing.T, h *Handler, sm *scs.SessionManager, userID, websiteID int64, name string, data []byte) *media.Media {
	t.Helper()
	rec, flash := serveAs(t, h, sm, userID, h.HandleMediaUpload, mediaUploadRequest(t, websiteID, name, data))
	if rec.Code != http.StatusSeeOther || flash != "" {
		t.Fatalf("upload %s: status %d (%q), want 303 and no error", name, rec.Code, flash)
	}
	items, _, err := h.mediaStore.List(context.Background(), websiteID, media.Filter{Query: name}, 1, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for i := range items {
		if items[i].OriginalName == name {
			return &items[i]
		}
	}
	t.Fatalf("upload %s: no row", name)
	return nil
}

func TestAnEditorWithoutPublishingStillChangesUnusedFiles(t *testing.T) {
	t.Run("upload", func(t *testing.T) {
		h, sm, database, ws := mediaAdmin(t)
		editor := editorWithoutPublishing(t, database, ws.ID)
		uploadAs(t, h, sm, editor, ws.ID, "werkstatt.png", onePixelPNG)
	})

	t.Run("describe", func(t *testing.T) {
		h, sm, database, ws := mediaAdmin(t)
		editor := editorWithoutPublishing(t, database, ws.ID)
		m := seedImage(t, h, ws.ID, "frei.jpg", 160, 60)

		rec, flash := serveAs(t, h, sm, editor, h.HandleMediaMeta, postForm(mediaAddress(ws.ID, m.ID)+"/meta",
			url.Values{"alt_text": {"Hobelbank"}, "caption": {"In der Werkstatt"}}, mediaRoute(ws.ID, m.ID)))
		if rec.Code != http.StatusSeeOther || flash != "" {
			t.Errorf("status %d (%q), want 303 and no error", rec.Code, flash)
		}
		after := mustGetMedia(t, h, m.ID)
		if after.AltText != "Hobelbank" || after.Caption != "In der Werkstatt" {
			t.Errorf("the description of a file nobody uses was not stored: %q / %q", after.AltText, after.Caption)
		}
	})

	t.Run("crop", func(t *testing.T) {
		h, sm, database, ws := mediaAdmin(t)
		editor := editorWithoutPublishing(t, database, ws.ID)
		m := uploadAs(t, h, sm, editor, ws.ID, "skizze.png", pngOf(t, 40, 30))

		rec, flash := serveAs(t, h, sm, editor, h.HandleMediaCropSave, cropRequest(t, m, url.Values{
			"form": {"1-1"}, "naehe": {"100"}, "drehung": {"0"}, "fokus_x": {"50"}, "fokus_y": {"50"},
		}))
		if rec.Code != http.StatusSeeOther || flash != "" {
			t.Errorf("status %d (%q), want 303 and no error", rec.Code, flash)
		}
		after := mustGetMedia(t, h, m.ID)
		if !after.IsCropped() || after.Width != after.Height {
			t.Errorf("a file nobody uses was not cropped: %d × %d, %+v", after.Width, after.Height, after.Crop)
		}
	})

	t.Run("delete", func(t *testing.T) {
		f := newAlbumFixture(t)
		h, sm, ws := f.h, f.sm, f.siteA
		editor := editorWithoutPublishing(t, f.db, ws.ID)
		unused := f.seedMedia(t, ws.ID, "frei.jpg")

		rec, flash := serveAs(t, h, sm, editor, h.HandleMediaDelete,
			postForm(mediaAddress(ws.ID, unused)+"/delete", url.Values{}, mediaRoute(ws.ID, unused)))
		if rec.Code != http.StatusSeeOther || flash != "" {
			t.Errorf("status %d (%q), want 303 and no error", rec.Code, flash)
		}
		if f.mediaStillThere(t, unused) {
			t.Error("a file nobody uses was not deleted")
		}
	})
}

// When the usage question fails, the file counts as in use (D-08): the
// question decides whether a picture disappears from a live page, and an
// error must not be read as "nobody uses it".
func TestAFailedUsageLookupKeepsTheFileLocked(t *testing.T) {
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	h, sm, database, ws := mediaAdmin(t)
	editor := editorWithoutPublishing(t, database, ws.ID)
	publisher := editorWhoMayPublish(t, database, ws.ID)
	m := seedImage(t, h, ws.ID, "frei.jpg", 160, 60)
	if _, err := database.Write.Exec(`DROP TABLE album_items`); err != nil {
		t.Fatalf("drop album_items: %v", err)
	}
	target := mediaAddress(ws.ID, m.ID) + "/delete"

	rec, flash := serveAs(t, h, sm, editor, h.HandleMediaDelete, postForm(target, url.Values{}, mediaRoute(ws.ID, m.ID)))
	wantMediaRefused(t, "delete with a failing usage question", rec, flash, mediaListAddress(ws.ID))
	mustGetMedia(t, h, m.ID)
	out := logged.String()
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "media usage lookup failed") {
		t.Errorf("no ERROR line for the failed usage question; log:\n%s", out)
	}

	// Nothing is asked for somebody who may publish, so the same failure does
	// not stand in their way.
	rec, flash = serveAs(t, h, sm, publisher, h.HandleMediaDelete, postForm(target, url.Values{}, mediaRoute(ws.ID, m.ID)))
	if rec.Code != http.StatusSeeOther || flash != "" {
		t.Errorf("publisher: status %d (%q), want 303 and no error", rec.Code, flash)
	}
	if got, _ := h.mediaStore.GetByID(context.Background(), m.ID); got != nil {
		t.Error("publisher: the file is still there")
	}
}

// mediaCard is the markup of one card of the media list, found by the id of
// its address field, which every card carries once.
func mediaCard(t *testing.T, body string, id int64) string {
	t.Helper()
	marker := fmt.Sprintf(`id="url-%d"`, id)
	for _, card := range strings.Split(body, `<div class="media-card">`) {
		if strings.Contains(card, marker) {
			return card
		}
	}
	t.Fatalf("no card for file %d", id)
	return ""
}

// postFormsTo are the form tags of a body that post to an address containing
// what. The layout carries a POST form of its own (signing out), so the
// question is about this file's forms only.
func postFormsTo(body, what string) []string {
	var out []string
	for _, tag := range regexp.MustCompile(`(?is)<form\b[^>]*>`).FindAllString(body, -1) {
		if strings.Contains(strings.ToLower(tag), `method="post"`) && strings.Contains(tag, what) {
			out = append(out, tag)
		}
	}
	return out
}

func TestTheMediaScreensOfferNoChangeToAFileInUse(t *testing.T) {
	h, sm, database, ws := mediaAdmin(t)
	editor := editorWithoutPublishing(t, database, ws.ID)
	publisher := editorWhoMayPublish(t, database, ws.ID)
	used := seedImage(t, h, ws.ID, "gezeigt.jpg", 160, 60)
	unused := seedImage(t, h, ws.ID, "frei.jpg", 160, 60)
	showInASnippet(t, database, ws.ID, used.Filename)
	route := websiteRoute(ws.ID)
	badge := "media-card__in-use"
	controls := func(id int64) []string {
		return []string{
			fmt.Sprintf("/media/%d/meta", id),
			fmt.Sprintf("/media/%d/zuschnitt", id),
			fmt.Sprintf("/media/%d/delete", id),
		}
	}

	body := getAs(t, sm, putUser(sm, editor), h.HandleMediaList, mediaListAddress(ws.ID), route)
	card := mediaCard(t, body, used.ID)
	if !strings.Contains(card, badge) || !strings.Contains(card, ">In use<") {
		t.Error("the card of a file in use carries no In use badge")
	}
	for _, c := range controls(used.ID) {
		if strings.Contains(card, c) {
			t.Errorf("the card of a file in use offers %s", c)
		}
	}
	card = mediaCard(t, body, unused.ID)
	if strings.Contains(card, badge) {
		t.Error("the card of a file nobody uses carries the In use badge")
	}
	for _, c := range controls(unused.ID) {
		if !strings.Contains(card, c) {
			t.Errorf("the card of a file nobody uses does not offer %s", c)
		}
	}

	cropRoute := mediaRoute(ws.ID, used.ID)
	body = getAs(t, sm, putUser(sm, editor), h.HandleMediaCrop, cropAddress(used), cropRoute)
	if !strings.Contains(body, mediaInUseText) {
		t.Error("the crop screen of a file in use does not say why nothing can be changed")
	}
	if forms := postFormsTo(body, fmt.Sprintf("/media/%d/", used.ID)); len(forms) > 0 {
		t.Errorf("the crop screen of a file in use offers %d form(s) that post: %v", len(forms), forms)
	}

	// A person who may publish sees every control on both cards and no badge.
	body = getAs(t, sm, putUser(sm, publisher), h.HandleMediaList, mediaListAddress(ws.ID), route)
	for _, id := range []int64{used.ID, unused.ID} {
		card := mediaCard(t, body, id)
		if strings.Contains(card, badge) {
			t.Errorf("publisher: card %d carries the In use badge", id)
		}
		for _, c := range controls(id) {
			if !strings.Contains(card, c) {
				t.Errorf("publisher: card %d does not offer %s", id, c)
			}
		}
	}
	body = getAs(t, sm, putUser(sm, publisher), h.HandleMediaCrop, cropAddress(used), cropRoute)
	if strings.Contains(body, mediaInUseText) {
		t.Error("publisher: the crop screen shows the media sentence")
	}
	if forms := postFormsTo(body, fmt.Sprintf("/media/%d/zuschnitt", used.ID)); len(forms) == 0 {
		t.Error("publisher: the crop screen offers no form")
	}
}
