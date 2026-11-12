package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
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
}

func TestAnEditorWithoutPublishingStillChangesUnusedFiles(t *testing.T) {
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
