package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/bundle"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	tmpl2 "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/wording"
)

// Moving a website out and back in, and the screen where an operator renames
// what their theme calls things.
//
// Driven red by fourteen mutations, thirteen caught. The survivor is recorded
// at the import round trip: an archive carries no domains, so the resolver
// cache cannot hold anything about a website that has just been imported.
//
// Three assertions had to be fixed before any of that, and all three were wrong
// about the CODE rather than about the behaviour — which is its own lesson
// about writing tests from a reading:
//
//   - `version` in a manifest is the FORMAT version and a number; the program
//     that wrote the file is a second field.
//   - exportFilename's `slug == ""` fallback is dead code: page.Slugify answers
//     "untitled" and never "".
//   - the import bounds the body at ten times the media limit, and the test
//     config's limit was zero — so every archive was refused by the first
//     branch and the later ones were unreachable. The first draft of the
//     refusal test passed with the "Import failed" message deleted.

func TestAWebsiteIsExportedAsAnArchiveThatSaysWhatItIs(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	if err := domain.NewStore(database).UpdateWebsite(ctx, ws.ID, "Holzbau Schmidt", "Möbel", true); err != nil {
		t.Fatal(err)
	}
	seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "Wir bauen **Möbel**.", "published")

	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/export", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	rec := serve(t, h, sm, h.HandleWebsiteExport, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/zip" {
		t.Errorf("Content-Type %q", got)
	}
	// The name sorts by date and survives every filesystem — an operator with
	// twelve of these in a folder should be able to see which is which.
	disp := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "holzbau-schmidt-") || !strings.Contains(disp, ".holzcloud.zip") {
		t.Errorf("Content-Disposition %q", disp)
	}
	// The archive is built while it is written, so nothing may cache it.
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control %q", got)
	}

	// And it really is an archive with a manifest in it.
	body := rec.Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("the download is not a zip: %v", err)
	}
	f, err := zr.Open(bundle.ManifestName)
	if err != nil {
		t.Fatalf("no manifest in the archive: %v", err)
	}
	defer f.Close()
	var m struct {
		// The FORMAT version, which is a number, and the version of the
		// program that wrote the file, which is a string. Two different
		// questions in two fields, and asserting on the wrong one is how this
		// test first went red.
		Version     int    `json:"version"`
		GeneratedBy string `json:"generated_by"`
		Site        struct {
			Name string `json:"name"`
		} `json:"site"`
		Pages []struct {
			Title string `json:"title"`
		} `json:"pages"`
	}
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		t.Fatalf("the manifest is not readable: %v", err)
	}
	if m.Site.Name != "Holzbau Schmidt" {
		t.Errorf("the archive names the website %q", m.Site.Name)
	}
	if len(m.Pages) != 1 || m.Pages[0].Title != "Über uns" {
		t.Errorf("the page is not in the archive: %+v", m.Pages)
	}
	if m.Version != bundle.Version {
		t.Errorf("the archive says format version %d, want %d", m.Version, bundle.Version)
	}
	// Stamped with what wrote it, so a puzzling import can be traced back.
	if m.GeneratedBy == "" {
		t.Error("the archive does not say which version of the program wrote it")
	}
}

// A name with nothing usable in it still produces a file somebody can save.
func TestTheExportFilenameSurvivesAnyWebsiteName(t *testing.T) {
	for _, c := range []struct{ name, want string }{
		{"Holzbau Schmidt", "holzbau-schmidt-"},
		{"Möbel & Stühle", "moebel-stuehle-"}, //nolint:german — the example name
		// page.Slugify answers "untitled" for a name with nothing usable in
		// it, so exportFilename's own `slug == ""` fallback can never run.
		// Asserted as it behaves rather than as it reads.
		{"!!! ???", "untitled-"},
		{"", "untitled-"},
	} {
		got := exportFilename(c.name)
		if !strings.HasPrefix(got, c.want) {
			t.Errorf("%q → %q, want it to start with %q", c.name, got, c.want)
		}
		if !strings.HasSuffix(got, ".holzcloud.zip") {
			t.Errorf("%q → %q, which is not an archive name", c.name, got)
		}
		// Nothing a filesystem or a download header would argue about.
		if strings.ContainsAny(got, `/\:*?"<>| `) {
			t.Errorf("%q → %q, which carries a character some filesystem refuses", c.name, got)
		}
	}
}

// Out and back in, through the two screens rather than through the library:
// what is tested here is that the handlers carry it, and that the report an
// operator reads afterwards says what arrived.
func TestAnArchiveGoesOutOfOneScreenAndInThroughTheOther(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	// The import bounds the body at ten times the media limit, and the test
	// config's limit is zero — which refuses every archive, however small.
	h.cfg.MaxMediaSize = 5 << 20
	seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "Wir bauen **Möbel**.", "published")

	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/export", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	archive := serve(t, h, sm, h.HandleWebsiteExport, req).Body.Bytes()

	rec := serve(t, h, sm, h.HandleWebsiteImport,
		bundleUpload(t, "kopie.holzcloud.zip", archive, "Kopie"))
	if rec.Code != http.StatusOK {
		t.Fatalf("import: status %d", rec.Code)
	}
	// The screen the operator lands on is the report, and it is the BUNDLE's
	// report. The title was corrected for this in v2.0 — and the template went
	// on printing its own <h1> with "WordPress import finished" in it, under
	// base.html's correct one, for two milestones after that. Two headings,
	// one of them a lie, on a screen somebody reaches by importing a file this
	// program wrote itself.
	body := rec.Body.String()
	if strings.Contains(body, "WordPress") {
		t.Error("the bundle import reports itself as a WordPress import")
	}
	if n := strings.Count(body, "<h1"); n != 1 {
		t.Errorf("%d headings on the report screen, want exactly one", n)
	}

	sites, err := domain.NewStore(database).ListWebsites(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 {
		t.Fatalf("%d websites after the import, want 2", len(sites))
	}
	var copied *domain.Website
	for i := range sites {
		if sites[i].ID != ws.ID {
			copied = &sites[i]
		}
	}
	if copied == nil || copied.Name != "Kopie" {
		t.Fatalf("the copy is named %q", copied.Name)
	}
	if p, err := h.pages.GetPageBySlug(ctx, copied.ID, "ueber-uns"); err != nil || p == nil {
		t.Errorf("the page did not come with it: %v", err)
	}

	// The imported website answers once it has a domain.
	//
	// The handler's InvalidateCache call cannot be driven red here and that is
	// recorded rather than papered over: an archive carries NO domains (nothing
	// in bundle's format does), so an imported website is unreachable until
	// somebody adds one, and a host that resolves to nothing is never cached —
	// the resolver stores an entry only on a hit. The call is defensive and
	// costs nothing; what it is defending against would need a bundle that
	// brought a host name with it.
	if _, err := domain.NewStore(database).AddDomain(ctx, copied.ID, "kopie.example", true); err != nil {
		t.Fatal(err)
	}
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Host = "kopie.example"
	rec2 := httptest.NewRecorder()
	h.resolver.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("the imported website does not answer: %d", rec2.Code)
	}
}

// What the import screen refuses, and that it refuses with a sentence rather
// than an error page: an operator who picked the wrong file should be able to
// pick another one.
func TestWhatTheImportScreenRefuses(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	ctx := context.Background()
	// Without a limit the body reader refuses EVERY archive, so all three cases
	// below took the same first branch and the later ones were never reached.
	// The first draft of this test therefore passed with the "Import failed"
	// message deleted.
	h.cfg.MaxMediaSize = 5 << 20

	// Each case is asserted on ITS OWN sentence, for the same reason: "no file"
	// and "this file is not an archive" are two different things to tell
	// somebody, answered by two different branches.
	for _, c := range []struct {
		what, filename, want string
		data                 []byte
	}{
		{"no file at all", "", "File too large or not selected", nil},
		{"something that is not a zip", "notizen.txt", "Import failed", []byte("nur Text")},
		{"a zip with nothing in it", "leer.zip", "Import failed", emptyZip(t)},
	} {
		rec, bad, _ := albumFlash(t, h, sm, h.HandleWebsiteImport,
			bundleUpload(t, c.filename, c.data, "Kopie"))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status %d, want a redirect with a message", c.what, rec.Code)
		}
		if !strings.Contains(bad, c.want) {
			t.Errorf("%s: answered %q, want it to mention %q", c.what, bad, c.want)
		}
	}

	sites, _ := domain.NewStore(database).ListWebsites(ctx)
	if len(sites) != 1 {
		t.Errorf("%d websites after three refusals, want the one that was there", len(sites))
	}
}

// The wording screen: what a theme calls things, and what this operator calls
// them instead.
func TestTheOperatorsOwnWordWinsAndCanBeGivenBack(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	store := wording.NewStore(database)
	h.SetWording(store)

	site := websiteOf(t, database, ws.ID)
	keys, _ := h.themeVocabulary(context.Background(), site)
	if len(keys) == 0 {
		t.Fatal("the shipped theme asks for no words, so this screen would be empty")
	}
	key := keys[0]
	main := site.AllLocales()[0]

	rec, bad, _ := albumFlash(t, h, sm, h.HandleWebsiteWordingSave, postForm(
		"/admin/websites/1/woerter",
		url.Values{"w:" + main + ":" + key: {"Mein Wort"}}, websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}
	own, err := store.All(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(own) != 1 || own[0].Key != key || own[0].Value != "Mein Wort" {
		t.Fatalf("the word was not stored: %+v", own)
	}

	// An EMPTIED box is how somebody says "use the theme's word again". Writing
	// only the filled-in boxes would make a word impossible to give back.
	rec = serve(t, h, sm, h.HandleWebsiteWordingSave, postForm("/admin/websites/1/woerter",
		url.Values{"w:" + main + ":" + key: {""}}, websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("giving it back: status %d", rec.Code)
	}
	own, _ = store.All(ctx, ws.ID)
	if len(own) != 0 {
		t.Errorf("the word could not be given back: %+v", own)
	}
}

// The render keeps a parsed set per website and language with the words baked
// into its FuncMap. Without dropping it the operator saves, reloads the site,
// and sees the old word — which is the same cache-of-a-decision the resolver
// and the theme switch have, in a third layer.
func TestAnOwnWordReachesTheRenderedPageAtOnce(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	site := websiteOf(t, database, ws.ID)
	main := site.AllLocales()[0]

	keys, _ := h.themeVocabulary(context.Background(), site)
	if len(keys) == 0 {
		t.Fatal("the shipped theme asks for no words")
	}

	// Find a word the theme actually renders, so the assertion is about the
	// page and not about the store.
	const mine = "HOLZWORT"
	var used string
	for _, key := range keys {
		if err := h.wording.Set(ctx, ws.ID, main, key, mine); err != nil {
			t.Fatalf("Set %q: %v", key, err)
		}
		h.loader.InvalidateTemplateCache(ws.ID)
		out, err := h.loader.RenderPage(ctx, ws.ID, "page.html", tmpl2.SampleData())
		if err != nil {
			t.Fatalf("RenderPage: %v", err)
		}
		if strings.Contains(string(out), mine) {
			used = key
			break
		}
		if err := h.wording.Set(ctx, ws.ID, main, key, ""); err != nil {
			t.Fatal(err)
		}
	}
	if used == "" {
		t.Skip("no word of this theme's vocabulary reaches page.html, so there is nothing to see change")
	}

	// Put the theme's word back, warm the cache, then save through the SCREEN
	// and look again. Without the invalidation the old word is still there.
	if err := h.wording.Set(ctx, ws.ID, main, used, ""); err != nil {
		t.Fatal(err)
	}
	h.loader.InvalidateTemplateCache(ws.ID)
	before, err := h.loader.RenderPage(ctx, ws.ID, "page.html", tmpl2.SampleData())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(before), mine) {
		t.Fatal("the word is there before it was saved — this test would prove nothing")
	}

	rec := serve(t, h, sm, h.HandleWebsiteWordingSave, postForm("/admin/websites/1/woerter",
		url.Values{"w:" + main + ":" + used: {mine}}, websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	after, err := h.loader.RenderPage(ctx, ws.ID, "page.html", tmpl2.SampleData())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), mine) {
		t.Error("the page still shows the theme's word — the template cache was not " +
			"dropped, so the operator saves, reloads, and sees no change")
	}
}

// A field for a language the website does not publish in, or for a word the
// theme does not ask for, is not saved: both mean the form was built against a
// different state than the one being saved into.
func TestAWordNobodyAskedForIsNotStored(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	store := wording.NewStore(database)
	h.SetWording(store)

	site := websiteOf(t, database, ws.ID)
	keys, _ := h.themeVocabulary(context.Background(), site)
	if len(keys) == 0 {
		t.Fatal("the shipped theme asks for no words")
	}
	main := site.AllLocales()[0]

	rec := serve(t, h, sm, h.HandleWebsiteWordingSave, postForm("/admin/websites/1/woerter",
		url.Values{
			// A language this website does not publish in.
			"w:fr:" + keys[0]: {"Mon mot"},
			// A word the theme never asks for.
			"w:" + main + ":gibt-es-nicht": {"Irgendwas"},
			// Not a word field at all.
			"name": {"Nicht speichern"},
		}, websiteRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	own, err := store.All(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(own) != 0 {
		t.Errorf("a form built against another state wrote %d words nobody chose: %+v",
			len(own), own)
	}
}

// The screen renders, and without a wording store it is a 404 rather than a
// crash.
func TestTheWordingScreenShowsTheThemesWords(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	h.SetWording(wording.NewStore(database))

	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/woerter", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	rec := serve(t, h, sm, h.HandleWebsiteWording, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}

	unknown := httptest.NewRequest(http.MethodGet, "/admin/websites/999/woerter", nil)
	unknown.SetPathValue("id", "999")
	if rec := serve(t, h, sm, h.HandleWebsiteWording, unknown); rec.Code != http.StatusNotFound {
		t.Errorf("an unknown website: status %d, want 404", rec.Code)
	}
}

// bundleUpload is the multipart POST the import form sends. An empty filename
// means no file part at all.
func bundleUpload(t *testing.T, filename string, data []byte, name string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("name", name); err != nil {
		t.Fatal(err)
	}
	if filename != "" {
		part, err := w.CreateFormFile("bundle", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/websites/import", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func emptyZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func websiteOf(t *testing.T, database *db.DB, id int64) *domain.Website {
	t.Helper()
	ws, err := domain.NewStore(database).GetWebsite(context.Background(), id)
	if err != nil || ws == nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	return ws
}
