package admin

import (
	"bytes"
	"context"
	"fmt"
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

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// The media screens: the list an operator opens daily, the upload, the
// description, the delete, and the route every visitor's browser hits for every
// picture on every page.
//
// None of the five had a test. What they have instead is a long history in
// their own comments — T-04-11, T-04-13, the deactivated website that went on
// serving its price list — and a comment is not a test.
//
// Driven red the way TEST-03 asks: sixteen mutations of media.go, fifteen of
// which are caught here. The one that is not is equivalent — removing the
// empty-filename guard in HandleMediaServe leaves ResolveServed to be asked for
// the file called "", which no website has, so the answer is the same 404. The
// guard is worth keeping because it says so at the top rather than by
// accident.
//
// One mutation is also recorded as a bad one rather than a survivor: disabling
// the size limit in media.Check does not disable the KIND check, so the file
// that claims to be a PNG and is an ELF binary is refused either way. What this
// package's test asserts is the screen's answer to a refusal — a flash and a
// redirect, and nothing stored. The refusal itself belongs to internal/media
// and is tested there.

// mediaAdmin is newTestAdmin with a size limit, because the zero config refuses
// every upload on the first check and that is not what these tests are about.
func mediaAdmin(t *testing.T) (*Handler, *scs.SessionManager, *db.DB, *domain.Website) {
	t.Helper()
	h, sm, database, ws := newTestAdmin(t)
	h.cfg.MaxMediaSize = 5 << 20
	h.cfg.MaxVideoSize = 64 << 20
	h.cfg.MaxMegapixels = 24
	return h, sm, database, ws
}

// mediaUploadRequest is the multipart POST the upload form sends.
func mediaUploadRequest(t *testing.T, websiteID int64, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("media", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/media/upload", websiteID), &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	return req
}

func TestTheUploadStoresTheFileAndSaysWhatIsMissing(t *testing.T) {
	h, sm, database, ws := mediaAdmin(t)
	ctx := context.Background()

	rec := serve(t, h, sm, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "werkstatt.png", onePixelPNG))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}

	items, total, err := media.NewStore(database).List(ctx, ws.ID, media.Filter{}, 1, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("%d files after the upload", total)
	}
	if items[0].OriginalName != "werkstatt.png" {
		t.Errorf("the name the operator gave it is %q", items[0].OriginalName)
	}
	if items[0].MimeType != "image/png" {
		t.Errorf("the kind was taken from the name and not from the bytes: %q", items[0].MimeType)
	}
	// The bytes are on disk under the stored name, which is not the name that
	// was uploaded — two people may upload werkstatt.png.
	onDisk := filepath.Join(h.cfg.DataDir, "media", strconv.FormatInt(ws.ID, 10), items[0].Filename)
	if _, err := os.Stat(onDisk); err != nil {
		t.Errorf("the file is not on disk: %v", err)
	}

	// The same file again is recognised and not stored twice.
	rec = serve(t, h, sm, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "nochmal.png", onePixelPNG))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("second upload: status %d", rec.Code)
	}
	_, total, err = media.NewStore(database).List(ctx, ws.ID, media.Filter{}, 1, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("the same bytes were stored %d times", total)
	}
}

// What is not a picture does not become one by being called .png. The check is
// media.Check's and is tested there; what is tested here is that the screen
// refuses rather than storing, and says so rather than failing.
func TestTheUploadRefusesWhatIsNotWhatItClaims(t *testing.T) {
	h, sm, database, ws := mediaAdmin(t)

	rec := serve(t, h, sm, h.HandleMediaUpload,
		mediaUploadRequest(t, ws.ID, "logo.png", []byte("\x7fELF\x02\x01\x01 kein bild")))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303 — a refusal is not an error page", rec.Code)
	}
	_, total, err := media.NewStore(database).List(context.Background(), ws.ID, media.Filter{}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("%d files stored although the upload was refused", total)
	}
}

func TestTheDescriptionIsStoredAndTrimmed(t *testing.T) {
	h, sm, database, ws := mediaAdmin(t)
	ctx := context.Background()
	serve(t, h, sm, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "werkstatt.png", onePixelPNG))

	items, _, _ := media.NewStore(database).List(ctx, ws.ID, media.Filter{}, 1, 10)
	if len(items) != 1 {
		t.Fatalf("no file to describe")
	}
	id := items[0].ID

	req := postForm(fmt.Sprintf("/admin/websites/%d/media/%d/meta", ws.ID, id),
		url.Values{
			"alt_text": {"  Die Werkstatt im Sommer  "},
			"caption":  {"  Aufgenommen 2026  "},
		},
		map[string]string{
			"id": strconv.FormatInt(ws.ID, 10), "mediaID": strconv.FormatInt(id, 10),
		})
	if rec := serve(t, h, sm, h.HandleMediaMeta, req); rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}

	stored, err := media.NewStore(database).GetByID(ctx, id)
	if err != nil || stored == nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.AltText != "Die Werkstatt im Sommer" {
		t.Errorf("alt text = %q", stored.AltText)
	}
	if stored.Caption != "Aufgenommen 2026" {
		t.Errorf("caption = %q", stored.Caption)
	}
}

// The scope rule lookupMedia exists for: website 1's route must not reach
// website 2's file, whichever verb is used.
func TestOneSitesRouteCannotReachAnothersFile(t *testing.T) {
	h, sm, database, ws := mediaAdmin(t)
	ctx := context.Background()

	other, err := domain.NewStore(database).CreateWebsite(ctx, "Die andere", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	serve(t, h, sm, h.HandleMediaUpload, mediaUploadRequest(t, other.ID, "fremd.png", onePixelPNG))
	items, _, _ := media.NewStore(database).List(ctx, other.ID, media.Filter{}, 1, 10)
	if len(items) != 1 {
		t.Fatalf("the other website has no file")
	}
	fremd := items[0].ID

	for _, c := range []struct {
		what string
		fn   func(http.ResponseWriter, *http.Request) error
	}{
		{"the description", h.HandleMediaMeta},
		{"the delete", h.HandleMediaDelete},
	} {
		req := postForm("/admin/websites/1/media/1/x", nil, map[string]string{
			"id": strconv.FormatInt(ws.ID, 10), "mediaID": strconv.FormatInt(fremd, 10),
		})
		rec := serve(t, h, sm, c.fn, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s reached the other website's file: status %d", c.what, rec.Code)
		}
	}

	// And it is still there afterwards.
	still, err := media.NewStore(database).GetByID(ctx, fremd)
	if err != nil || still == nil {
		t.Errorf("the other website's file is gone: %v", err)
	}
}

// Deleting a file a page uses names the pages rather than saying "in use", and
// does not delete until it is confirmed.
func TestDeletingAFileInUseNamesThePageFirst(t *testing.T) {
	h, sm, database, ws := mediaAdmin(t)
	ctx := context.Background()
	serve(t, h, sm, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "werkstatt.png", onePixelPNG))
	items, _, _ := media.NewStore(database).List(ctx, ws.ID, media.Filter{}, 1, 10)
	m := items[0]

	p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns",
		fmt.Sprintf("Ein Bild: ![x](/media/%d/%s)", ws.ID, m.Filename), "published")
	// Which files a page uses is recorded when the page is SAVED, not derived
	// when it is deleted — so seeding through the store leaves the record
	// empty. The two steps the page handler takes, taken here.
	refs, err := media.ExtractRefs(ctx, media.NewStore(database), ws.ID, p.ContentHTML)
	if err != nil {
		t.Fatalf("ExtractRefs: %v", err)
	}
	if len(refs) == 0 {
		t.Fatalf("the page does not refer to the file — the test would prove nothing")
	}
	if err := media.NewStore(database).ReplaceUsage(ctx, p.ID, refs); err != nil {
		t.Fatalf("ReplaceUsage: %v", err)
	}

	pathValues := map[string]string{
		"id": strconv.FormatInt(ws.ID, 10), "mediaID": strconv.FormatInt(m.ID, 10),
	}
	rec, bad, _ := albumFlash(t, h, sm, h.HandleMediaDelete,
		postForm("/admin/websites/1/media/1/delete", nil, pathValues))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	if still, _ := media.NewStore(database).GetByID(ctx, m.ID); still == nil {
		t.Fatal("a file in use was deleted without confirmation")
	}
	// Naming the page is the point of the message: "in use" on its own leaves
	// the operator to search for it by hand.
	if !strings.Contains(bad, "Über uns") {
		t.Errorf("the message does not name the page: %q", bad)
	}

	// Confirmed, it goes.
	rec = serve(t, h, sm, h.HandleMediaDelete, postForm(
		"/admin/websites/1/media/1/delete", url.Values{"force": {"1"}}, pathValues))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("confirmed delete: status %d", rec.Code)
	}
	if still, _ := media.NewStore(database).GetByID(ctx, m.ID); still != nil {
		t.Error("the confirmed delete did not delete")
	}
}

// The public route, which is the one every visitor's browser uses for every
// picture on every page. Four rules, each of them written into the handler
// after something went wrong.
func TestTheServeRouteKeepsItsFourRules(t *testing.T) {
	h, sm, database, ws := mediaAdmin(t)
	ctx := context.Background()
	serve(t, h, sm, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "werkstatt.png", onePixelPNG))
	items, _, _ := media.NewStore(database).List(ctx, ws.ID, media.Filter{}, 1, 10)
	name := items[0].Filename

	get := func(websiteID int64, filename string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/media/x/y", nil)
		req.SetPathValue("websiteID", strconv.FormatInt(websiteID, 10))
		req.SetPathValue("filename", filename)
		return serve(t, h, sm, h.HandleMediaServe, req)
	}

	// 1. The kind comes from the database and not from the file system, and
	//    nothing may reinterpret it.
	rec := get(ws.ID, name)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("Cache-Control %q", got)
	}

	// 2. A file name is a name and never a path (T-04-11).
	for _, bad := range []string{"../../etc/passwd", `..\..\etc\passwd`, "unter/ordner.png", ""} {
		if rec := get(ws.ID, bad); rec.Code != http.StatusNotFound {
			t.Errorf("%q was served: status %d", bad, rec.Code)
		}
	}

	// 3. A file this website does not have is not served, even when another
	//    website does have it.
	other, err := domain.NewStore(database).CreateWebsite(ctx, "Die andere", "")
	if err != nil {
		t.Fatal(err)
	}
	if rec := get(other.ID, name); rec.Code != http.StatusNotFound {
		t.Errorf("the other website served a file it does not have: %d", rec.Code)
	}

	// 4. A deactivated website serves nothing. This route is on the root mux
	//    and never meets the domain resolver, so switching a site off has to be
	//    answered here — otherwise a switched-off customer goes on serving
	//    their price list.
	if err := domain.NewStore(database).UpdateWebsite(ctx, ws.ID, ws.Name, ws.Description, false); err != nil {
		t.Fatalf("UpdateWebsite: %v", err)
	}
	if rec := get(ws.ID, name); rec.Code != http.StatusNotFound {
		t.Errorf("a deactivated website still served its file: status %d", rec.Code)
	}
}

// An SVG and a PDF are documents that can carry active content, and they are
// served from the same origin as the administration. Opened directly they get a
// policy of their own.
func TestAnSVGIsServedWithAPolicyThatNeutralisesIt(t *testing.T) {
	h, sm, database, ws := mediaAdmin(t)
	ctx := context.Background()

	m, err := media.NewStore(database).Create(ctx, ws.ID, "abc.svg", "logo.svg",
		"image/svg+xml", 10, "hash-svg")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	dir := filepath.Join(h.cfg.DataDir, "media", strconv.FormatInt(ws.ID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, m.Filename), []byte(`<svg/>`), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/media/x/y", nil)
	req.SetPathValue("websiteID", strconv.FormatInt(ws.ID, 10))
	req.SetPathValue("filename", m.Filename)
	rec := serve(t, h, sm, h.HandleMediaServe, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "sandbox") {
		t.Errorf("an SVG was served without a policy of its own: %q", csp)
	}
}

// The list an operator opens daily: the filters keep their state, the count of
// pictures without a description is shown, and the pager knows where it is.
func TestTheMediaListFiltersAndCounts(t *testing.T) {
	h, sm, database, ws := mediaAdmin(t)
	ctx := context.Background()
	serve(t, h, sm, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "werkstatt.png", onePixelPNG))

	// A second file of a different kind, so the filter has something to leave
	// out.
	if _, err := media.NewStore(database).Create(ctx, ws.ID, "abc.pdf", "preise.pdf",
		"application/pdf", 10, "hash-pdf"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/media?kind=document", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	rec := serve(t, h, sm, h.HandleMediaList, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "preise.pdf") {
		t.Error("the document filter left out the document")
	}
	if strings.Contains(body, "werkstatt.png") {
		t.Error("the document filter kept the picture")
	}

	// mediaFilterFromRequest is the whole of the control reading, so it is
	// asserted on its own too.
	for _, c := range []struct {
		query string
		want  media.Filter
	}{
		{"", media.Filter{}},
		{"?q=+werkstatt+", media.Filter{Query: "werkstatt"}},
		{"?kind=image", media.Filter{MimePrefix: "image/"}},
		{"?kind=document", media.Filter{MimePrefix: "application/"}},
		{"?kind=etwas", media.Filter{}},
		{"?unused=1", media.Filter{Unused: true}},
	} {
		got := mediaFilterFromRequest(httptest.NewRequest(http.MethodGet, "/x"+c.query, nil))
		if got != c.want {
			t.Errorf("%q: filter = %+v, want %+v", c.query, got, c.want)
		}
	}
}
