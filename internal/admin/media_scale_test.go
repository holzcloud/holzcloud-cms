package admin

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// The question after an upload: a picture over media.MaxSide on its longest
// side is held back and the operator is asked, on a page that is a plain form.

type scaleFlash struct{ err, ok, warn string }

// serveUser runs a handler as a signed-in user and reads the flash it left.
func serveUser(t *testing.T, h *Handler, sm *scs.SessionManager, userID int64,
	fn func(http.ResponseWriter, *http.Request) error, req *http.Request) (*httptest.ResponseRecorder, scaleFlash) {
	t.Helper()
	rec := httptest.NewRecorder()
	var handlerErr error
	var f scaleFlash
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, userID)
		handlerErr = fn(w, r)
		f = scaleFlash{
			err:  sm.GetString(r.Context(), auth.SessionKeyFlashError),
			ok:   sm.GetString(r.Context(), auth.SessionKeyFlashSuccess),
			warn: sm.GetString(r.Context(), auth.SessionKeyFlashWarning),
		}
	})).ServeHTTP(rec, req)
	if handlerErr != nil {
		t.Fatalf("handler: %v", handlerErr)
	}
	return rec, f
}

func multiUploadRequest(t *testing.T, websiteID int64, files map[string][]byte, order []string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, name := range order {
		part, err := mw.CreateFormFile("media", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/media/upload", websiteID), &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	return req
}

func scaleGet(websiteID int64, token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/admin/websites/%d/media/scale?token=%s", websiteID, url.QueryEscape(token)), nil)
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	return req
}

func scalePost(websiteID int64, token, decision string) *http.Request {
	v := url.Values{"token": {token}}
	if decision != "" {
		v.Set("decision", decision)
	}
	return postForm(fmt.Sprintf("/admin/websites/%d/media/scale", websiteID), v,
		map[string]string{"id": strconv.FormatInt(websiteID, 10)})
}

// tokenOf reads the token out of the redirect an upload answered with.
func tokenOf(t *testing.T, rec *httptest.ResponseRecorder, websiteID int64) string {
	t.Helper()
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	prefix := fmt.Sprintf("/admin/websites/%d/media/scale?token=", websiteID)
	if !strings.HasPrefix(loc, prefix) {
		t.Fatalf("redirected to %q, want the scale page", loc)
	}
	return strings.TrimPrefix(loc, prefix)
}

func stagingEntries(t *testing.T, h *Handler, websiteID int64) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(h.cfg.DataDir, "media-staging", strconv.FormatInt(websiteID, 10)))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func libraryOf(t *testing.T, h *Handler, websiteID int64) []media.Media {
	t.Helper()
	items, _, err := h.mediaStore.List(context.Background(), websiteID, media.Filter{}, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	return items
}

func sizeOnDisk(t *testing.T, h *Handler, websiteID int64, m media.Media) (int, int) {
	t.Helper()
	w, hgt, err := media.Dimensions(media.Path(h.cfg.DataDir, websiteID, m.Filename))
	if err != nil {
		t.Fatal(err)
	}
	return w, hgt
}

func TestALargeUploadIsHeldBackAndAsked(t *testing.T) {
	h, sm, _, ws := mediaAdmin(t)

	rec, _ := serveUser(t, h, sm, 1, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "hof.jpg", photo(t, 3000, 1500)))
	token := tokenOf(t, rec, ws.ID)

	if n := len(libraryOf(t, h, ws.ID)); n != 0 {
		t.Fatalf("%d files in the library before the answer, want 0", n)
	}
	if n := stagingEntries(t, h, ws.ID); n != 1 {
		t.Fatalf("%d staged uploads, want 1", n)
	}

	page, _ := serveUser(t, h, sm, 1, h.HandleMediaScaleForm, scaleGet(ws.ID, token))
	if page.Code != http.StatusOK {
		t.Fatalf("question page: status %d", page.Code)
	}
	body := page.Body.String()
	for _, want := range []string{
		"hof.jpg", "3000", "1500", "2560", "1280",
		fmt.Sprintf(`action="/admin/websites/%d/media/scale"`, ws.ID),
		`name="token" value="` + token + `"`,
		`name="decision" value="scale"`,
		`name="decision" value="keep"`,
		`method="POST"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the question page lacks %q", want)
		}
	}
	// Plain form: the page has one path, with and without the script.
	if strings.Contains(body, "hx-post") {
		t.Error("the question page carries an hx-post; it must be a plain form")
	}
}

func TestYesStoresAScaledCopyWithoutCameraData(t *testing.T) {
	h, sm, _, ws := mediaAdmin(t)
	rec, _ := serveUser(t, h, sm, 1, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "hof.jpg", photo(t, 3000, 1500)))
	token := tokenOf(t, rec, ws.ID)

	rec, flash := serveUser(t, h, sm, 1, h.HandleMediaScale, scalePost(ws.ID, token, "scale"))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != fmt.Sprintf("/admin/websites/%d/media", ws.ID) {
		t.Fatalf("answer: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if flash.ok == "" {
		t.Error("no success flash after the scale-down")
	}

	items := libraryOf(t, h, ws.ID)
	if len(items) != 1 {
		t.Fatalf("%d files, want 1", len(items))
	}
	w, hgt := sizeOnDisk(t, h, ws.ID, items[0])
	if w != 2560 || hgt != 1280 {
		t.Errorf("stored %d x %d, want 2560 x 1280", w, hgt)
	}
	raw, err := os.ReadFile(media.Path(h.cfg.DataDir, ws.ID, items[0].Filename))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("Bauernhof")) {
		t.Error("the camera block of the source survived")
	}
	if n := stagingEntries(t, h, ws.ID); n != 0 {
		t.Errorf("%d staged uploads left after the answer", n)
	}
}

func TestNoStoresTheOriginal(t *testing.T) {
	h, sm, _, ws := mediaAdmin(t)
	rec, _ := serveUser(t, h, sm, 1, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "hof.jpg", photo(t, 3000, 1500)))
	token := tokenOf(t, rec, ws.ID)

	req := scalePost(ws.ID, token, "keep")
	req.Header.Set("HX-Request", "true")
	rec, _ = serveUser(t, h, sm, 1, h.HandleMediaScale, req)
	if rec.Header().Get("HX-Redirect") != fmt.Sprintf("/admin/websites/%d/media", ws.ID) {
		t.Errorf("an htmx request is answered with HX-Redirect, got %q", rec.Header().Get("HX-Redirect"))
	}

	items := libraryOf(t, h, ws.ID)
	if len(items) != 1 {
		t.Fatalf("%d files, want 1", len(items))
	}
	if w, hgt := sizeOnDisk(t, h, ws.ID, items[0]); w != 3000 || hgt != 1500 {
		t.Errorf("stored %d x %d, want the original 3000 x 1500", w, hgt)
	}
	if n := stagingEntries(t, h, ws.ID); n != 0 {
		t.Errorf("%d staged uploads left", n)
	}
}

func TestWhatIsNotAskedAboutIsStoredAtOnce(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="5000" height="5000"/>`)
	pal := color.Palette{color.Black, color.White}
	frame := func() *image.Paletted { return image.NewPaletted(image.Rect(0, 0, 3000, 10), pal) }
	var anim bytes.Buffer
	if err := gif.EncodeAll(&anim, &gif.GIF{Image: []*image.Paletted{frame(), frame()}, Delay: []int{5, 5}}); err != nil {
		t.Fatal(err)
	}

	for name, content := range map[string][]byte{
		"genau.jpg": photo(t, 2560, 400),
		"logo.svg":  svg,
		"film.gif":  anim.Bytes(),
	} {
		h, sm, _, ws := mediaAdmin(t)
		rec, _ := serveUser(t, h, sm, 1, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, name, content))
		if rec.Header().Get("Location") != fmt.Sprintf("/admin/websites/%d/media", ws.ID) {
			t.Errorf("%s: redirected to %q, want the library", name, rec.Header().Get("Location"))
		}
		if n := len(libraryOf(t, h, ws.ID)); n != 1 {
			t.Errorf("%s: %d files stored, want 1", name, n)
		}
		if n := stagingEntries(t, h, ws.ID); n != 0 {
			t.Errorf("%s: staged although it is exempt", name)
		}
	}

	// Over the megapixel budget: the existing note covers it, no question.
	h, sm, _, ws := mediaAdmin(t)
	h.cfg.MaxMegapixels = 2
	rec, _ := serveUser(t, h, sm, 1, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "riesig.jpg", photo(t, 3000, 1500)))
	if rec.Header().Get("Location") != fmt.Sprintf("/admin/websites/%d/media", ws.ID) {
		t.Errorf("over budget: redirected to %q", rec.Header().Get("Location"))
	}
	if n := len(libraryOf(t, h, ws.ID)); n != 1 {
		t.Errorf("over budget: %d files stored, want 1", n)
	}
}

func TestOneDecisionCoversAWholeUpload(t *testing.T) {
	h, sm, _, ws := mediaAdmin(t)
	files := map[string][]byte{
		"a.jpg":     photo(t, 3000, 1500),
		"b.jpg":     photo(t, 1500, 3000),
		"klein.png": onePixelPNG,
	}
	rec, flash := serveUser(t, h, sm, 1, h.HandleMediaUpload, multiUploadRequest(t, ws.ID, files, []string{"a.jpg", "klein.png", "b.jpg"}))
	token := tokenOf(t, rec, ws.ID)

	// The small one is stored at once and says so.
	items := libraryOf(t, h, ws.ID)
	if len(items) != 1 || items[0].OriginalName != "klein.png" {
		t.Fatalf("library after the upload: %+v", items)
	}
	if flash.ok == "" {
		t.Error("the file that was stored at once left no message")
	}

	page, _ := serveUser(t, h, sm, 1, h.HandleMediaScaleForm, scaleGet(ws.ID, token))
	body := page.Body.String()
	if !strings.Contains(body, "a.jpg") || !strings.Contains(body, "b.jpg") || strings.Contains(body, "klein.png") {
		t.Errorf("the page should list exactly the two large files:\n%s", body)
	}

	serveUser(t, h, sm, 1, h.HandleMediaScale, scalePost(ws.ID, token, "scale"))
	items = libraryOf(t, h, ws.ID)
	if len(items) != 3 {
		t.Fatalf("%d files after the decision, want 3", len(items))
	}
	for _, m := range items {
		w, hgt := sizeOnDisk(t, h, ws.ID, m)
		switch m.OriginalName {
		case "a.jpg":
			if w != 2560 || hgt != 1280 {
				t.Errorf("a.jpg is %d x %d", w, hgt)
			}
		case "b.jpg":
			if w != 1280 || hgt != 2560 {
				t.Errorf("b.jpg is %d x %d", w, hgt)
			}
		}
	}
}

func TestAStagedUploadBelongsToItsWebsiteAndItsUser(t *testing.T) {
	h, sm, _, ws := mediaAdmin(t)
	other, err := h.domains.CreateWebsite(context.Background(), "Andere", "")
	if err != nil {
		t.Fatal(err)
	}
	rec, _ := serveUser(t, h, sm, 1, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "hof.jpg", photo(t, 3000, 1500)))
	token := tokenOf(t, rec, ws.ID)

	// Another website's route: expired, never revealed, never stored.
	rec, flash := serveUser(t, h, sm, 1, h.HandleMediaScale, scalePost(other.ID, token, "keep"))
	if rec.Code != http.StatusSeeOther || flash.err == "" {
		t.Errorf("a foreign website: status %d, flash %q", rec.Code, flash.err)
	}
	page, _ := serveUser(t, h, sm, 1, h.HandleMediaScaleForm, scaleGet(other.ID, token))
	if strings.Contains(page.Body.String(), "hof.jpg") {
		t.Error("another website's route showed the staged file")
	}
	if n := len(libraryOf(t, h, other.ID)); n != 0 {
		t.Errorf("another website's library got %d files", n)
	}

	// Another user: not found.
	rec, _ = serveUser(t, h, sm, 2, h.HandleMediaScale, scalePost(ws.ID, token, "keep"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("another user: status %d, want 404", rec.Code)
	}
	rec, _ = serveUser(t, h, sm, 2, h.HandleMediaScaleForm, scaleGet(ws.ID, token))
	if rec.Code != http.StatusNotFound {
		t.Errorf("another user, GET: status %d, want 404", rec.Code)
	}
	if n := len(libraryOf(t, h, ws.ID)); n != 0 {
		t.Errorf("a stranger stored %d files", n)
	}

	// And the owner still can.
	serveUser(t, h, sm, 1, h.HandleMediaScale, scalePost(ws.ID, token, "keep"))
	if n := len(libraryOf(t, h, ws.ID)); n != 1 {
		t.Errorf("the owner's answer stored %d files, want 1", n)
	}
}

func TestAnUnknownOrMalformedTokenIsExpiredNeverAnError(t *testing.T) {
	h, sm, _, ws := mediaAdmin(t)
	for _, token := range []string{"", "nope", "../../etc", strings.Repeat("a", 32), strings.Repeat("A", 32)} {
		for name, req := range map[string]*http.Request{
			"GET": scaleGet(ws.ID, token), "POST": scalePost(ws.ID, token, "scale"),
		} {
			fn := h.HandleMediaScaleForm
			if name == "POST" {
				fn = h.HandleMediaScale
			}
			rec, flash := serveUser(t, h, sm, 1, fn, req)
			if rec.Code != http.StatusSeeOther || flash.err == "" {
				t.Errorf("%s %q: status %d, flash %q; want an expired flash and a redirect", name, token, rec.Code, flash.err)
			}
			if rec.Header().Get("Location") != fmt.Sprintf("/admin/websites/%d/media", ws.ID) {
				t.Errorf("%s %q: redirected to %q", name, token, rec.Header().Get("Location"))
			}
		}
	}
}

func TestAnAnswerThatIsNeitherChangesNothing(t *testing.T) {
	h, sm, _, ws := mediaAdmin(t)
	rec, _ := serveUser(t, h, sm, 1, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "hof.jpg", photo(t, 3000, 1500)))
	token := tokenOf(t, rec, ws.ID)

	for _, decision := range []string{"", "maybe", "SCALE"} {
		rec, _ = serveUser(t, h, sm, 1, h.HandleMediaScale, scalePost(ws.ID, token, decision))
		want := fmt.Sprintf("/admin/websites/%d/media/scale?token=%s", ws.ID, token)
		if rec.Header().Get("Location") != want {
			t.Errorf("decision %q: redirected to %q, want the question again", decision, rec.Header().Get("Location"))
		}
	}
	if n := len(libraryOf(t, h, ws.ID)); n != 0 {
		t.Errorf("%d files stored without an answer", n)
	}
	if n := stagingEntries(t, h, ws.ID); n != 1 {
		t.Errorf("%d staged uploads, want the one still waiting", n)
	}
}

func TestAReplacedStagingFileStoresNothing(t *testing.T) {
	h, sm, _, ws := mediaAdmin(t)
	rec, _ := serveUser(t, h, sm, 1, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "hof.jpg", photo(t, 3000, 1500)))
	token := tokenOf(t, rec, ws.ID)

	staged := filepath.Join(h.cfg.DataDir, "media-staging", strconv.FormatInt(ws.ID, 10), token, "0")
	if err := os.WriteFile(staged, []byte("MZ this is a program and not a picture"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, flash := serveUser(t, h, sm, 1, h.HandleMediaScale, scalePost(ws.ID, token, "keep"))
	if n := len(libraryOf(t, h, ws.ID)); n != 0 {
		t.Errorf("a replaced staging file was stored (%d files)", n)
	}
	if flash.err == "" && flash.warn == "" {
		t.Error("the refusal was not reported")
	}
}

func TestTheAIUploadNeverAsks(t *testing.T) {
	h, _, _, ws := mediaAdmin(t)
	m, existed, _, err := h.OpUploadMedia(context.Background(), ws.ID, "gross.jpg", photo(t, 3000, 1500))
	if err != nil || existed {
		t.Fatalf("OpUploadMedia: %v (existed %v)", err, existed)
	}
	if w, hgt := sizeOnDisk(t, h, ws.ID, *m); w != 3000 || hgt != 1500 {
		t.Errorf("stored %d x %d, want the full 3000 x 1500", w, hgt)
	}
	if n := stagingEntries(t, h, ws.ID); n != 0 {
		t.Errorf("the AI path staged %d uploads", n)
	}
}

// portraitPhoto is photo with a real EXIF orientation block in front of the
// text-only one: the bitmap is w x h as stored, the tag says how it is turned.
func portraitPhoto(t *testing.T, w, h, orientation int) []byte {
	t.Helper()
	jpg := photo(t, w, h)
	tiff := []byte{
		'M', 'M', 0, 42, 0, 0, 0, 8, // header, IFD0 at 8
		0, 1, // one entry
		0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, byte(orientation), 0, 0, // orientation, SHORT, count 1
		0, 0, 0, 0, // no next IFD
	}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	n := len(payload) + 2
	out := append([]byte{}, jpg[:2]...)
	out = append(out, 0xFF, 0xE1, byte(n>>8), byte(n))
	out = append(out, payload...)
	return append(out, jpg[2:]...)
}

// A phone photo is stored as a landscape bitmap and carries the turn as a tag.
// The question says what the visitor will see, and Yes stores exactly that.
func TestScaleQuestionUsesTheUprightPicture(t *testing.T) {
	h, sm, _, ws := mediaAdmin(t)
	rec, _ := serveUser(t, h, sm, 1, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "hoch.jpg", portraitPhoto(t, 3000, 1500, 6)))
	token := tokenOf(t, rec, ws.ID)

	page, _ := serveUser(t, h, sm, 1, h.HandleMediaScaleForm, scaleGet(ws.ID, token))
	body := page.Body.String()
	if !strings.Contains(body, "1500 × 3000 px, becomes 1280 × 2560 px") {
		t.Errorf("the question does not announce the upright size:\n%s", body)
	}
	if strings.Contains(body, "3000 × 1500") {
		t.Error("the question prints the stored size of the bitmap")
	}

	serveUser(t, h, sm, 1, h.HandleMediaScale, scalePost(ws.ID, token, "scale"))
	items := libraryOf(t, h, ws.ID)
	if len(items) != 1 {
		t.Fatalf("%d files, want 1", len(items))
	}
	if w, hgt := sizeOnDisk(t, h, ws.ID, items[0]); w != 1280 || hgt != 2560 {
		t.Errorf("stored %d x %d, want 1280 x 2560", w, hgt)
	}
	raw, err := os.ReadFile(media.Path(h.cfg.DataDir, ws.ID, items[0].Filename))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("Exif")) || bytes.Contains(raw, []byte("Bauernhof")) {
		t.Error("the stored copy carries camera data")
	}

	// Answered No, the original is stored as it came: the unchanged path.
	h, sm, _, ws = mediaAdmin(t)
	rec, _ = serveUser(t, h, sm, 1, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "hoch.jpg", portraitPhoto(t, 3000, 1500, 6)))
	token = tokenOf(t, rec, ws.ID)
	serveUser(t, h, sm, 1, h.HandleMediaScale, scalePost(ws.ID, token, "keep"))
	items = libraryOf(t, h, ws.ID)
	if len(items) != 1 {
		t.Fatalf("%d files after No, want 1", len(items))
	}
	if w, hgt := sizeOnDisk(t, h, ws.ID, items[0]); w != 3000 || hgt != 1500 {
		t.Errorf("after No stored %d x %d, want the original 3000 x 1500", w, hgt)
	}
}
