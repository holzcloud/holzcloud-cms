package bundle

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	"github.com/holzcloud/holzcloud-cms/internal/term"
)

func newStores(t *testing.T) Stores {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	fields := field.NewStore(database)
	return Stores{
		Domains:    domain.NewStore(database),
		Pages:      page.NewStore(database),
		Menus:      menu.NewStore(database),
		Snippets:   snippet.NewStore(database),
		Terms:      term.NewStore(database),
		Media:      media.NewStore(database),
		Albums:     album.NewStore(database),
		Fields:     fields,
		BlockTypes: block.NewStore(database, fields),
		DataDir:    dir,
	}
}

// seedSite builds a website with one of everything, so a round trip has
// something to lose.
func seedSite(t *testing.T, s Stores) int64 {
	t.Helper()
	ctx := context.Background()

	ws, err := s.Domains.CreateWebsite(ctx, "Holzbau Schmidt", "Möbel nach Maß")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	if err := s.Domains.UpdateSettings(ctx, ws.ID, domain.Settings{
		Locale: "de", TimeZone: "Europe/Berlin", OfflineMode: "notfound",
		BlogBase: "aktuelles", PostsPerPage: 5, ContactEmail: "info@example.de",
		OrgType: "HomeAndConstructionBusiness", Street: "Waldweg 3",
		PostalCode: "75173", City: "Pforzheim", Country: "DE",
		Phone: "+49 7231 123456", OpeningHours: "Mo-Fr 08:00-17:00",
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if err := s.Domains.UpdateDesignTokens(ctx, ws.ID, domain.DesignTokens{
		Ink: "#222222", Brand: "#8b5a2b", Font: "serif", Measure: 70, Radius: 0,
	}); err != nil {
		t.Fatalf("UpdateDesignTokens: %v", err)
	}

	// A file, on disk and in the database, so the export has bytes to carry.
	dir := filepath.Join(s.DataDir, "media", "1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	photo := []byte("nicht wirklich ein foto, aber eindeutige bytes")
	if err := os.WriteFile(filepath.Join(dir, "abc-werkstatt.jpg"), photo, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := s.Media.Create(ctx, ws.ID, "abc-werkstatt.jpg", "werkstatt.jpg",
		"image/jpeg", int64(len(photo)), hashBytes(photo))
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	if err := s.Media.UpdateMeta(ctx, m.ID, "Die Werkstatt", "Im Sommer"); err != nil {
		t.Fatal(err)
	}

	about, err := s.Pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "Über uns", Slug: "ueber-uns",
		Markdown: "Wir bauen **Möbel** aus Eiche.", HTML: "<p>x</p>",
		Status: "published",
		Meta:   page.PageMeta{Excerpt: "Wir bauen Möbel.", FeaturedMediaID: &m.ID},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}
	if _, err := s.Pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "Neue Werkbank", Slug: "neue-werkbank",
		Markdown: "Endlich fertig.", HTML: "<p>x</p>",
		Status: "published", Kind: page.KindPost,
	}); err != nil {
		t.Fatalf("create post: %v", err)
	}
	if err := s.Terms.SetForPage(ctx, ws.ID, about.ID, []string{"Möbel", "Eiche"}); err != nil {
		t.Fatalf("SetForPage: %v", err)
	}
	if _, err := s.Snippets.Create(ctx, ws.ID, "zeiten", "Öffnungszeiten",
		"Mo-Fr 8-17 Uhr", "<p>Mo-Fr 8-17 Uhr</p>"); err != nil {
		t.Fatalf("create snippet: %v", err)
	}

	mn, err := s.Menus.CreateMenu(ctx, ws.ID, "Hauptmenü", "main", "")
	if err != nil {
		t.Fatalf("create menu: %v", err)
	}
	parent, err := s.Menus.CreateItem(ctx, mn.ID, nil, "Über uns", "page", "", &about.ID, 0)
	if err != nil {
		t.Fatalf("create menu item: %v", err)
	}
	if _, err := s.Menus.CreateItem(ctx, mn.ID, &parent.ID, "Impressum", "url", "/impressum", nil, 0); err != nil {
		t.Fatalf("create child item: %v", err)
	}
	return ws.ID
}

func exportTo(t *testing.T, s Stores, websiteID int64) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := Export(context.Background(), s, websiteID, "test", &buf); err != nil {
		t.Fatalf("Export: %v", err)
	}
	return buf.Bytes()
}

func TestRoundTripKeepsTheSite(t *testing.T) {
	s := newStores(t)
	websiteID := seedSite(t, s)
	archive := exportTo(t, s, websiteID)

	report, err := Import(context.Background(), s, bytes.NewReader(archive),
		int64(len(archive)), "Holzbau Schmidt (Kopie)")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("a clean round trip warned: %v", report.Warnings)
	}
	if report.Pages != 2 || report.Media != 1 || report.Menus != 1 || report.Snippets != 1 {
		t.Errorf("report = %+v", report)
	}

	ctx := context.Background()
	copied, err := s.Domains.GetWebsite(ctx, report.WebsiteID)
	if err != nil || copied == nil {
		t.Fatalf("the imported website is missing: %v", err)
	}
	if copied.Name != "Holzbau Schmidt (Kopie)" {
		t.Errorf("name = %q", copied.Name)
	}
	for _, c := range []struct{ got, want, what string }{
		{copied.BlogBase, "aktuelles", "archive address"},
		{copied.City, "Pforzheim", "city"},
		{copied.OrgType, "HomeAndConstructionBusiness", "business type"},
		{copied.TokenBrand, "#8b5a2b", "brand colour"},
		{copied.TokenFont, "serif", "typeface"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.what, c.got, c.want)
		}
	}
	if copied.PostsPerPage != 5 || copied.TokenMeasure != 70 || copied.TokenRadius != 0 {
		t.Errorf("numeric settings lost: %+v", copied)
	}

	// The Markdown is what travels; the HTML is re-rendered, so it must come
	// out of this version's renderer rather than the archive.
	about, err := s.Pages.GetPageBySlug(ctx, report.WebsiteID, "ueber-uns")
	if err != nil || about == nil {
		t.Fatalf("the page did not arrive: %v", err)
	}
	if !strings.Contains(about.ContentMarkdown, "**Möbel**") {
		t.Errorf("markdown = %q", about.ContentMarkdown)
	}
	if !strings.Contains(about.ContentHTML, "<strong>Möbel</strong>") {
		t.Errorf("html was not re-rendered: %q", about.ContentHTML)
	}

	post, _ := s.Pages.GetPageBySlug(ctx, report.WebsiteID, "neue-werkbank")
	if post == nil || !post.IsPost() {
		t.Error("the post did not arrive as a post")
	}

	labels, _ := s.Terms.ForPage(ctx, about.ID)
	if term.Format(labels) != "Eiche, Möbel" {
		t.Errorf("labels = %q", term.Format(labels))
	}

	// The featured image has to point at the *new* copy of the file, not at the
	// id it had on the machine it came from.
	if about.FeaturedMediaID == nil {
		t.Fatal("the featured image was lost")
	}
	img, _ := s.Media.GetByID(ctx, *about.FeaturedMediaID)
	if img == nil || img.WebsiteID != report.WebsiteID {
		t.Errorf("the featured image points at another website's file: %+v", img)
	}
	if img.AltText != "Die Werkstatt" {
		t.Errorf("alt text = %q", img.AltText)
	}

	// And the bytes themselves must be on disk under the new website.
	copyPath := filepath.Join(s.DataDir, "media", "2", img.Filename)
	if _, err := os.Stat(copyPath); err != nil {
		t.Errorf("the file was not written: %v", err)
	}

	tree, err := s.Menus.GetMenuTree(ctx, report.WebsiteID, "main")
	if err != nil || len(tree) != 1 {
		t.Fatalf("menu tree = %v (%v)", tree, err)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].Title != "Impressum" {
		t.Errorf("the nesting was lost: %+v", tree[0])
	}
	// The item pointed at a page by slug; it has to resolve to the new copy.
	if tree[0].PageID == nil || *tree[0].PageID != about.ID {
		t.Errorf("the menu item does not point at the imported page: %+v", tree[0])
	}
}

func TestExportCarriesNoSecrets(t *testing.T) {
	s := newStores(t)
	websiteID := seedSite(t, s)

	ctx := context.Background()
	pg, _ := s.Pages.GetPageBySlug(ctx, websiteID, "ueber-uns")
	if err := s.Pages.SetAccess(ctx, pg.ID, page.AccessUpdate{
		Protected: true, Password: "holz2026", Hint: "Steht im Anschreiben.",
	}, auth.Argon2Params{Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}); err != nil {
		t.Fatal(err)
	}

	archive := exportTo(t, s, websiteID)
	// A bundle gets emailed around. A password hash in it is a hash somebody
	// can attack offline at their leisure.
	if bytes.Contains(archive, []byte("$argon2")) {
		t.Error("the archive carries a password hash")
	}

	manifest := readArchiveManifest(t, archive)
	var protected *Page
	for i := range manifest.Pages {
		if manifest.Pages[i].Slug == "ueber-uns" {
			protected = &manifest.Pages[i]
		}
	}
	if protected == nil {
		t.Fatal("the page is missing from the manifest")
	}
	// The setting travels so the import can warn about it; the secret does not.
	if !protected.Protected() {
		t.Error("the protection setting was lost, so an import cannot warn")
	}
	if protected.AccessHint != "Steht im Anschreiben." {
		t.Errorf("hint = %q", protected.AccessHint)
	}

	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "Kopie")
	if err != nil {
		t.Fatal(err)
	}
	imported, _ := s.Pages.GetPageBySlug(ctx, report.WebsiteID, "ueber-uns")
	if imported.Protected() {
		t.Error("the page claims to be protected with no password behind it")
	}
	// Silence here would leave a price list publicly readable and nobody told.
	if !warned(report, "password") {
		t.Errorf("the import did not warn about the lost password: %v", report.Warnings)
	}
}

func TestExportCarriesNoDomains(t *testing.T) {
	s := newStores(t)
	websiteID := seedSite(t, s)
	if _, err := s.Domains.AddDomain(context.Background(), websiteID, "example.de", true); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}

	archive := exportTo(t, s, websiteID)
	// A bundle is meant to land somewhere else. Carrying the host name would
	// either collide with the site already serving it or quietly claim a domain
	// the new machine does not own.
	if bytes.Contains(archive, []byte("example.de")) {
		t.Error("the archive carries a domain name")
	}
}

func TestImportRefusesANewerFormat(t *testing.T) {
	s := newStores(t)
	archive := archiveWith(t, Manifest{Version: Version + 1, Site: Site{Name: "Zukunft"}})

	_, err := Import(context.Background(), s, bytes.NewReader(archive), int64(len(archive)), "")
	if err == nil {
		t.Fatal("an archive from a newer version was accepted")
	}
	// Guessing would mean silently dropping the fields it did not recognise.
	if !strings.Contains(err.Error(), "neueren") {
		t.Errorf("the error does not explain why: %v", err)
	}
}

func TestImportRefusesRubbish(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	notAZip := []byte("das ist kein archiv")
	if _, err := Import(ctx, s, bytes.NewReader(notAZip), int64(len(notAZip)), ""); err == nil {
		t.Error("a file that is not an archive was accepted")
	}

	// A zip with no manifest is a zip of something else entirely.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("irgendwas.txt")
	w.Write([]byte("hallo"))
	zw.Close()
	if _, err := Import(ctx, s, bytes.NewReader(buf.Bytes()), int64(buf.Len()), ""); err == nil {
		t.Error("an archive without a manifest was accepted")
	}
}

func TestImportRefusesAnEscapingFileName(t *testing.T) {
	s := newStores(t)
	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Bösartig"},
		Media:   []Media{{Filename: "../../entkommen.txt", MimeType: "image/jpeg"}},
	})

	report, err := Import(context.Background(), s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Media != 0 {
		t.Error("a file with an escaping name was stored")
	}
	if !warned(report, "not allowed") {
		t.Errorf("the import did not say why: %v", report.Warnings)
	}
	// Nothing may have been written outside the media directory.
	if _, err := os.Stat(filepath.Join(s.DataDir, "entkommen.txt")); err == nil {
		t.Fatal("a file was written outside the media directory")
	}
}

func TestImportNoticesACorruptedFile(t *testing.T) {
	s := newStores(t)
	websiteID := seedSite(t, s)
	archive := exportTo(t, s, websiteID)

	// Flip the stored bytes in place — same length, so the archive's own
	// structure survives and only the content is wrong, the way a bad SD card
	// corrupts a file.
	broken := bytes.Replace(archive, []byte("eindeutige bytes"), []byte("beschaedigte xyz"), 1)
	if bytes.Equal(broken, archive) {
		t.Fatal("the fixture did not change; the test would prove nothing")
	}

	report, err := Import(context.Background(), s, bytes.NewReader(broken), int64(len(broken)), "Kopie")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	// Storing the damage and finding out when a page renders a grey box is the
	// outcome the checksum exists to prevent.
	if report.Media != 0 {
		t.Error("a corrupted file was stored")
	}
	if !warned(report, "damaged") {
		t.Errorf("the corruption was not reported: %v", report.Warnings)
	}
	// And nothing may have been written to disk for it.
	dir := filepath.Join(s.DataDir, "media", "2")
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("a corrupted file was written anyway: %v", entries)
	}
}

func TestImportValidatesDesignTokensFromTheFile(t *testing.T) {
	s := newStores(t)
	archive := archiveWith(t, Manifest{
		Version: Version,
		Site: Site{
			Name: "Bösartig",
			Design: Design{
				Ink:  "#fff;} body{display:none} :root{",
				Font: "url(https://evil.example/f.woff2)",
			},
		},
	})

	report, err := Import(context.Background(), s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	ws, _ := s.Domains.GetWebsite(context.Background(), report.WebsiteID)
	// A bundle is a file anyone can edit, so it is exactly as untrusted as a
	// form field and goes through the same validator.
	if ws.TokenInk != "" || ws.TokenFont != "" {
		t.Errorf("an unvalidated token survived the import: ink=%q font=%q", ws.TokenInk, ws.TokenFont)
	}
}

// archiveWith builds a minimal archive around one manifest.
func archiveWith(t *testing.T, m Manifest) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(ManifestName)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(w).Encode(m); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func readArchiveManifest(t *testing.T, archive []byte) Manifest {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	m, err := readManifest(zr)
	if err != nil {
		t.Fatal(err)
	}
	return *m
}

func warned(r *Report, fragment string) bool {
	for _, w := range r.Warnings {
		if strings.Contains(w, fragment) {
			return true
		}
	}
	return false
}

func TestImportPointsPicturesAtTheNewWebsite(t *testing.T) {
	s := newStores(t)
	// Website 1 exists before the import, so the one the import creates is
	// number 2 — the same off-by-one-site that a real move between machines
	// produces, and the whole point of the test.
	seedSite(t, s)

	photo := []byte("noch ein foto")
	archive := archiveWithFile(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Umgezogen"},
		Media: []Media{{
			Filename: "hof.jpg", OriginalName: "hof.jpg",
			MimeType: "image/jpeg", AltText: "Der Hof", SHA256: hashBytes(photo),
		}},
		Pages: []Page{{
			Title: "Hof", Slug: "hof", Status: "published",
			Markdown: "![Der Hof](/media/1/hof.jpg)\n\n" +
				`<img src="/media/1/hof.jpg" alt="Der Hof">` + "\n\n" +
				"[Fremdes Bild](/media/1/gehoert-woanders-hin.jpg)",
		}},
		Snippets: []Snippet{{
			Key: "marke", Name: "Bildmarke", Markdown: "![Marke](/media/1/hof.jpg)",
		}},
	}, "hof.jpg", photo)

	report, err := Import(context.Background(), s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Media != 1 || report.Pages != 1 {
		t.Fatalf("the fixture did not import: %+v", report)
	}

	pg, err := s.Pages.GetPageBySlug(context.Background(), report.WebsiteID, "hof")
	if err != nil || pg == nil {
		t.Fatalf("GetPageBySlug: %v", err)
	}
	want := fmt.Sprintf("/media/%d/hof.jpg", report.WebsiteID)
	// Twice: the Markdown image and the raw <img>. A page that mixes the two
	// is ordinary, and rewriting only one of them would leave half the
	// pictures broken — the failure mode that is hardest to spot.
	if got := strings.Count(pg.ContentMarkdown, want); got != 2 {
		t.Errorf("expected both links rewritten to %s, found %d in:\n%s", want, got, pg.ContentMarkdown)
	}
	if strings.Contains(pg.ContentMarkdown, "/media/1/hof.jpg") {
		t.Errorf("a link still points at the old website:\n%s", pg.ContentMarkdown)
	}
	if !strings.Contains(pg.ContentHTML, want) {
		t.Errorf("the rendered HTML kept the old path:\n%s", pg.ContentHTML)
	}
	// The archive does not carry this file, so the import has no business
	// claiming it: the link is either a mistake to be seen or a pointer at
	// something else on the machine.
	if !strings.Contains(pg.ContentMarkdown, "/media/1/gehoert-woanders-hin.jpg") {
		t.Errorf("a link to a file outside the bundle was rewritten anyway:\n%s", pg.ContentMarkdown)
	}

	snippets, err := s.Snippets.List(context.Background(), report.WebsiteID)
	if err != nil || len(snippets) != 1 {
		t.Fatalf("List snippets: %v (%d)", err, len(snippets))
	}
	if !strings.Contains(snippets[0].ContentMarkdown, want) {
		t.Errorf("the snippet kept the old path: %s", snippets[0].ContentMarkdown)
	}
}

// archiveWithFile builds an archive around one manifest plus one media file.
func archiveWithFile(t *testing.T, m Manifest, name string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(ManifestName)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(w).Encode(m); err != nil {
		t.Fatal(err)
	}
	f, err := zw.Create(MediaDir + name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A reference is a page number, and a number means nothing on the other
// machine. It travels as an address — and does so even when it points at a
// page that comes later in the archive.
func TestRoundTripKeepsAReferenceForward(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	ws, err := s.Domains.CreateWebsite(ctx, "Hof", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "gehoert_zu", Label: "Gehört zu", Kind: field.KindRef,
	}); err != nil {
		t.Fatalf("create field: %v", err)
	}

	// The target page is created after the referring one, so that the import
	// cannot possibly know it on the first pass.
	quelle, err := s.Pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "Wollpaket", Slug: "wollpaket",
		Markdown: "x", HTML: "<p>x</p>", Status: "published",
	})
	if err != nil {
		t.Fatal(err)
	}
	ziel, err := s.Pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "Schafe", Slug: "schafe",
		Markdown: "x", HTML: "<p>x</p>", Status: "published",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := field.Encode(field.Data{Values: field.Values{
		"gehoert_zu": strconv.FormatInt(ziel.ID, 10),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Pages.SetFields(ctx, quelle.ID, raw); err != nil {
		t.Fatal(err)
	}

	archive := exportTo(t, s, ws.ID)
	geschrieben := manifestOf(t, archive)
	if strings.Contains(geschrieben, `"gehoert_zu": "`+strconv.FormatInt(ziel.ID, 10)+`"`) {
		t.Error("the archive carries the page's id instead of its address")
	}
	if !strings.Contains(geschrieben, `"gehoert_zu": "schafe"`) {
		t.Errorf("the archive does not carry the target page's address:\n%s", geschrieben)
	}

	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "Hof (Kopie)")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("Warnungen: %v", report.Warnings)
	}

	kopien, _, err := s.Pages.ListPages(ctx, report.WebsiteID, page.ListFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	var neueQuelle, neuesZiel *page.Page
	for i, p := range kopien {
		switch p.Slug {
		case "wollpaket":
			neueQuelle = &kopien[i]
		case "schafe":
			neuesZiel = &kopien[i]
		}
	}
	if neueQuelle == nil || neuesZiel == nil {
		t.Fatalf("Seiten fehlen: %+v", kopien)
	}
	got := field.Decode(neueQuelle.Fields).Values["gehoert_zu"]
	if want := strconv.FormatInt(neuesZiel.ID, 10); got != want {
		t.Errorf("Verweis zeigt auf %q, sollte auf die neue Zielseite %q zeigen", got, want)
	}
}

func manifestOf(t *testing.T, archive []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := zr.Open(ManifestName)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	return string(data)
}

// A page built from snippets travelled this far as plain text: the snippets
// were never in the archive. This test is the promise that they are — together
// with their own snippet kind and with the image inside, which travels as a
// file name and is given a new number on the other side.
func TestRoundTripKeepsBlocks(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()
	ws := seedSite(t, s)

	// An image the export can genuinely carry along.
	dir := filepath.Join(s.DataDir, "media", strconv.FormatInt(ws, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	foto := []byte("ein foto")
	if err := os.WriteFile(filepath.Join(dir, "teig.jpg"), foto, 0o644); err != nil {
		t.Fatal(err)
	}
	// The checksum has to match the file: the import throws away a file whose
	// sum does not add up, and that would not be a fault of the bundle here.
	bild, err := s.Media.Create(ctx, ws, "teig.jpg", "teig.jpg", "image/jpeg",
		int64(len(foto)), hashBytes(foto))
	if err != nil {
		t.Fatalf("Media.Create: %v", err)
	}

	// A snippet kind of its own with a text field and an image field.
	art, err := s.BlockTypes.Create(ctx, ws, "Rezeptschritt", "Ein Schritt.")
	if err != nil {
		t.Fatalf("BlockTypes.Create: %v", err)
	}
	for _, d := range []field.Def{
		{Key: "nummer", Label: "Nummer", Kind: field.KindText},
		{Key: "bild", Label: "Bild", Kind: field.KindImage},
	} {
		d.WebsiteID, d.BlockTypeID = ws, art.ID
		if _, err := s.Fields.Create(ctx, d); err != nil {
			t.Fatalf("Feld %s: %v", d.Key, err)
		}
	}

	// The album that the gallery snippet below names. Since 11-06 the
	// reference travels as a name and is derived again on the other side, so
	// the album really has to exist — a slug that names no album of this
	// website is deliberately dropped.
	if _, err := s.Albums.Create(ctx, ws, "Möbel"); err != nil {
		t.Fatalf("Albums.Create: %v", err)
	}

	set := s.BlockTypes.Set(ctx, ws)
	blocks := []block.Block{
		{Type: block.TypeText, Markdown: "Zuerst der Teig."},
		{Type: block.TypeImage, MediaID: bild.ID, Alt: "Der Teig"},
		{Type: "rezeptschritt", Fields: map[string]string{
			"nummer": "Schritt 1", "bild": strconv.FormatInt(bild.ID, 10),
		}},
		// The gallery's display mode. It is here rather than in a test of its
		// own because it is the field whose loss is silent: the value is
		// written into one struct literal in blocks.go on the way out and read
		// back into another on the way in, and a field added to only one of
		// the two leaves in the archive and never comes back, with nothing
		// anywhere to say so.
		// The album reference rides beside the display for the same reason.
		// Since plan 11-06 it is no pass-through: the archive carries the
		// album's NAME and the import derives the address again, so what this
		// asserts is that the two derivations agree. The case where they must
		// not be allowed to agree by accident — a rename between the two — is
		// TestAlbumRoundTripAfterRename.
		{Type: block.TypeGallery, Display: block.DisplaySlideshow, AlbumSlug: "moebel",
			Items: []block.Item{{MediaID: bild.ID, Caption: "Der Teig"}}},
	}
	encoded, err := block.Encode(blocks, set)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, err := s.Pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws, Title: "Brot backen", Slug: "brot-backen",
		Markdown: block.PlainText(blocks, set), Blocks: encoded, Status: "published",
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	var buf bytes.Buffer
	if err := Export(ctx, s, ws, "test", &buf); err != nil {
		t.Fatalf("Export: %v", err)
	}
	report, err := Import(ctx, s, bytes.NewReader(buf.Bytes()), int64(buf.Len()), "Kopie")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	pg, err := s.Pages.GetPageBySlug(ctx, report.WebsiteID, "brot-backen")
	if err != nil || pg == nil {
		t.Fatalf("GetPageBySlug: %v", err)
	}
	if pg.Blocks == "" {
		t.Fatal("the page arrived without blocks")
	}

	neuerSatz := s.BlockTypes.Set(ctx, report.WebsiteID)
	angekommen, err := block.Decode(pg.Blocks, neuerSatz)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(angekommen) != 4 {
		t.Fatalf("%d Bausteine statt 4: %+v", len(angekommen), angekommen)
	}
	if angekommen[3].Display != block.DisplaySlideshow {
		t.Errorf("the gallery's display did not survive the archive: %q",
			angekommen[3].Display)
	}
	if angekommen[3].AlbumSlug != "moebel" {
		t.Errorf("the gallery's album did not survive the archive: %q",
			angekommen[3].AlbumSlug)
	}
	if angekommen[0].Markdown != "Zuerst der Teig." {
		t.Errorf("der Textbaustein: %q", angekommen[0].Markdown)
	}
	if angekommen[2].Type != "rezeptschritt" || angekommen[2].Fields["nummer"] != "Schritt 1" {
		t.Errorf("die eigene Art kam nicht an: %+v", angekommen[2])
	}

	// The image number has to be a new one — that of the copy, not that of the
	// original. This is exactly where a bundle would otherwise reach silently
	// into the library of the wrong website.
	neuesBild, err := s.Media.GetByID(ctx, angekommen[1].MediaID)
	if err != nil || neuesBild == nil {
		t.Fatalf("the block's image does not exist: %v", err)
	}
	if neuesBild.WebsiteID != report.WebsiteID {
		t.Errorf("the image belongs to website %d instead of %d", neuesBild.WebsiteID, report.WebsiteID)
	}
	if angekommen[2].Fields["bild"] != strconv.FormatInt(neuesBild.ID, 10) {
		t.Errorf("the image in the own block points at %q instead of at %d",
			angekommen[2].Fields["bild"], neuesBild.ID)
	}

	// And the page is set, not empty: the HTML is built anew on import.
	if !strings.Contains(pg.ContentHTML, "hc-eigen--rezeptschritt") {
		t.Errorf("the page was not re-rendered:\n%s", pg.ContentHTML)
	}
	if !strings.Contains(pg.ContentHTML, neuesBild.URL()) {
		t.Errorf("the image is missing from the rendered page:\n%s", pg.ContentHTML)
	}
	if !strings.Contains(pg.ContentMarkdown, "Schritt 1") {
		t.Errorf("the plain text for search and excerpt is missing: %q", pg.ContentMarkdown)
	}
}

// ---------------------------------------------------------------------------
// The albums in a bundle — GAL-04, plan 11-06.
//
// Everything in this section is English: the test names, the comments and the
// messages. The file around it is German, and that is deliberate on both
// counts rather than an oversight. The project's language rule
// (.planning/GLOSSARY.md) is that what is written now is written in English;
// the German above it predates that rule and is not being rewritten by this
// plan. Do not translate one half to match the other — neither half was asked
// for.
// ---------------------------------------------------------------------------

// seedAlbumSite builds a website with one picture on disk and one album that
// holds it: the smallest thing an album round trip can be run over.
func seedAlbumSite(t *testing.T, s Stores, siteName, albumName string) (websiteID, albumID, mediaID int64) {
	t.Helper()
	ctx := context.Background()

	ws, err := s.Domains.CreateWebsite(ctx, siteName, "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	dir := filepath.Join(s.DataDir, "media", strconv.FormatInt(ws.ID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The checksum has to match the bytes: an import throws away a file whose
	// sum does not, and that would not be a fault of the album here.
	photo := []byte("die bytes von " + siteName)
	if err := os.WriteFile(filepath.Join(dir, "hobel.jpg"), photo, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := s.Media.Create(ctx, ws.ID, "hobel.jpg", "hobel.jpg", "image/jpeg",
		int64(len(photo)), hashBytes(photo))
	if err != nil {
		t.Fatalf("Media.Create: %v", err)
	}
	a, err := s.Albums.Create(ctx, ws.ID, albumName)
	if err != nil {
		t.Fatalf("Albums.Create: %v", err)
	}
	if _, err := s.Albums.AddItem(ctx, ws.ID, a.ID, m.ID, "Ein Hobel", "In der Werkstatt"); err != nil {
		t.Fatalf("Albums.AddItem: %v", err)
	}
	return ws.ID, a.ID, m.ID
}

// An album's pictures travel as file names, never as ids — the sentence
// blocks.go:11-18 opens with, one level up. And the album itself travels under
// its name and without a slug, because the importing machine derives the slug
// from the name with the one call the album store makes.
func TestExportWritesAlbumsWithFileNames(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()
	ws, _, mediaID := seedAlbumSite(t, s, "Werkstatt", "Referenzen")

	// A second album, so the export is proved to write both rather than the
	// first one it meets.
	second, err := s.Albums.Create(ctx, ws, "Werkzeug")
	if err != nil {
		t.Fatalf("Albums.Create: %v", err)
	}
	if _, err := s.Albums.AddItem(ctx, ws, second.ID, mediaID, "", ""); err != nil {
		t.Fatalf("Albums.AddItem: %v", err)
	}

	written := manifestOf(t, exportTo(t, s, ws))

	for _, want := range []string{
		`"albums"`,
		`"name": "Referenzen"`,
		`"name": "Werkzeug"`,
		`"media": "hobel.jpg"`,
		`"alt": "Ein Hobel"`,
		`"caption": "In der Werkstatt"`,
	} {
		if !strings.Contains(written, want) {
			t.Errorf("the manifest does not carry %s:\n%s", want, written)
		}
	}
	// A number would silently point at somebody else's picture on the machine
	// the bundle lands on.
	for _, unwanted := range []string{
		fmt.Sprintf(`"media": %d`, mediaID),
		fmt.Sprintf(`"media": "%d"`, mediaID),
	} {
		if strings.Contains(written, unwanted) {
			t.Errorf("the manifest carries %s instead of the file name:\n%s", unwanted, written)
		}
	}
	// No slug on an album: carrying both would be two sources for one key.
	if strings.Contains(written, `"slug": "referenzen"`) {
		t.Errorf("the album travels with a slug; the name alone is what travels:\n%s", written)
	}
}

// Every album the manifest declares is created, its pictures resolve to the
// media this import just made, and the count in the report is what was made.
//
// What this test can observe is the end state, which pins the "after the
// media" half of the placement: a picture that resolved to an id of the NEW
// website can only have been looked up in a media list that already existed.
// The "before the pages" half has no observable hook from here — Go offers no
// way to watch the order of two calls — and is held by the source-order gate
// in this plan's verification block instead.
func TestImportCreatesAlbumsBeforePages(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	photo := []byte("ein hobel")
	archive := archiveWithFile(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Ankunft"},
		Media: []Media{{
			Filename: "hobel.jpg", OriginalName: "hobel.jpg",
			MimeType: "image/jpeg", SHA256: hashBytes(photo),
		}},
		Albums: []Album{
			{Name: "Referenzen", Items: []AlbumItem{
				{Media: "hobel.jpg", Alt: "Ein Hobel", Caption: "In der Werkstatt"},
			}},
			{Name: "Werkzeug"},
		},
	}, "hobel.jpg", photo)

	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Albums != 2 {
		t.Errorf("report.Albums = %d, want 2 (warnings: %v)", report.Albums, report.Warnings)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("a clean album import warned: %v", report.Warnings)
	}

	for _, slug := range []string{"referenzen", "werkzeug"} {
		a, err := s.Albums.BySlug(ctx, report.WebsiteID, slug)
		if err != nil || a == nil {
			t.Fatalf("album %q is not on the imported website: %v", slug, err)
		}
	}

	a, err := s.Albums.BySlug(ctx, report.WebsiteID, "referenzen")
	if err != nil || a == nil {
		t.Fatalf("BySlug: %v", err)
	}
	items, err := s.Albums.Items(ctx, report.WebsiteID, a.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("Items = %v, %v", items, err)
	}
	if items[0].Alt != "Ein Hobel" || items[0].Caption != "In der Werkstatt" {
		t.Errorf("the picture's words did not arrive: %+v", items[0])
	}
	arrived, err := s.Media.GetByID(ctx, items[0].MediaID)
	if err != nil || arrived == nil {
		t.Fatalf("the album's picture points at no media row: %v", err)
	}
	if arrived.WebsiteID != report.WebsiteID {
		t.Errorf("the album's picture belongs to website %d instead of %d",
			arrived.WebsiteID, report.WebsiteID)
	}
}

// A number a report names should be believable: report.Albums counts what was
// created and not what the archive claimed (import.go:331-332).
func TestImportReportsAlbumsCreatedNotClaimed(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Behauptet"},
		Albums: []Album{
			{Name: "Referenzen"},
			// A name that cannot become one: whitespace only.
			{Name: "   "},
			// The same address a second time — the slug is what the unique
			// constraint is on, so this one lands on it.
			{Name: "Referenzen"},
		},
	})

	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Albums != 1 {
		t.Errorf("report.Albums = %d, want 1 — three were claimed, one could be made",
			report.Albums)
	}
	if !warned(report, "album 2") {
		t.Errorf("the unusable name is not on the report: %v", report.Warnings)
	}
	if !warned(report, "Referenzen") {
		t.Errorf("the album that could not be made is not named on the report: %v",
			report.Warnings)
	}
	albums, err := s.Albums.List(ctx, report.WebsiteID)
	if err != nil {
		t.Fatal(err)
	}
	if len(albums) != 1 {
		t.Errorf("%d albums on the new website, want 1: %+v", len(albums), albums)
	}
	// And nothing called "untitled": a name that cannot become one is
	// reported and skipped, not stored under a placeholder.
	for _, a := range albums {
		if a.Slug == "untitled" {
			t.Errorf("an unusable name became an album: %+v", a)
		}
	}
}

// The album round trip, and the one thing that makes it a proof.
//
// album.Store.Rename keeps the slug on purpose: a gallery block stores the
// slug, so correcting a typo in an album's name must not take the album away
// from every page that carries it. A page therefore holds the OLD slug while
// the album shows a NEW name. If the block's reference travels as a slug, the
// other machine derives a different one from the name and the reference points
// at nothing — silently, with an empty gallery where the pictures were.
//
// Which is why this test renames before it exports. An album whose name still
// matches its slug round-trips correctly EVEN WHEN THE TRANSLATION IS MISSING,
// so a test without the rename proves nothing at all. The same argument, about
// a term instead of an album, is written out at TestSchlagwortfeldRundreise
// above; Phase 7 shipped that field declared and unproved for exactly this
// reason.
//
// And both directions are asserted, because only the negative one fails when a
// half of the translation is absent.
func TestAlbumRoundTripAfterRename(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()
	ws, albumID, mediaID := seedAlbumSite(t, s, "Werkstatt", "Werkstatt 2024")

	// 1. The stored address is what page.Slugify makes of the original name.
	//    Asserted rather than assumed, so a change to the derivation fails
	//    here loudly instead of quietly testing something else.
	const oldName = "Werkstatt 2024"
	const newName = "Werkstatt 2025"
	oldSlug := page.Slugify(oldName)
	a, err := s.Albums.Get(ctx, ws, albumID)
	if err != nil || a == nil {
		t.Fatalf("Albums.Get: %v", err)
	}
	if a.Slug != oldSlug {
		t.Fatalf("slug = %q, want %q", a.Slug, oldSlug)
	}

	// 2. Rename to a name that slugifies DIFFERENTLY — without that the test
	//    would pass with the translation removed.
	if err := s.Albums.Rename(ctx, ws, albumID, newName); err != nil {
		t.Fatalf("Albums.Rename: %v", err)
	}
	newSlug := page.Slugify(newName)
	if newSlug == oldSlug {
		t.Fatalf("the two names slugify the same (%q); the test would prove nothing", newSlug)
	}
	after, err := s.Albums.Get(ctx, ws, albumID)
	if err != nil || after == nil {
		t.Fatalf("Albums.Get: %v", err)
	}
	if after.Slug != oldSlug {
		t.Fatalf("Rename moved the slug to %q; the whole case rests on it staying", after.Slug)
	}

	// 3. A page whose gallery block carries the album's reference — which is
	//    still the OLD slug, because that is what Rename leaves behind.
	set := s.BlockTypes.Set(ctx, ws)
	blocks := []block.Block{
		{Type: block.TypeText, Markdown: "Unsere Arbeiten."},
		{Type: block.TypeGallery, AlbumSlug: oldSlug},
	}
	encoded, err := block.Encode(blocks, set)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, err := s.Pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws, Title: "Arbeiten", Slug: "arbeiten",
		Markdown: block.PlainText(blocks, set), Blocks: encoded, Status: "published",
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	// 4. Out.
	archive := exportTo(t, s, ws)
	written := manifestOf(t, archive)

	// 5. Both directions. The manifest carries the NEW name and the old slug
	//    appears nowhere in it. Only the second of these fails when the
	//    translation is missing, and it is the one the requirement rests on.
	if !strings.Contains(written, newName) {
		t.Errorf("the manifest does not carry the album's new name %q:\n%s", newName, written)
	}
	if strings.Contains(written, oldSlug) {
		t.Errorf("the manifest still carries the old slug %q; the reference travels as "+
			"a slug and will point at nothing on the other machine:\n%s", oldSlug, written)
	}

	// 6. In, on a website that has never seen this album, and the block's
	//    reference resolves to the album that is actually there.
	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "Werkstatt (Kopie)")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("a clean round trip warned: %v", report.Warnings)
	}
	pg, err := s.Pages.GetPageBySlug(ctx, report.WebsiteID, "arbeiten")
	if err != nil || pg == nil {
		t.Fatalf("GetPageBySlug: %v", err)
	}
	arrived, err := block.Decode(pg.Blocks, s.BlockTypes.Set(ctx, report.WebsiteID))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(arrived) != 2 {
		t.Fatalf("%d blocks instead of 2: %+v", len(arrived), arrived)
	}
	if arrived[1].AlbumSlug != newSlug {
		t.Fatalf("the gallery points at %q; the album on this website is %q",
			arrived[1].AlbumSlug, newSlug)
	}
	copied, err := s.Albums.BySlug(ctx, report.WebsiteID, arrived[1].AlbumSlug)
	if err != nil || copied == nil {
		t.Fatalf("the gallery points at no album of the imported website: %v", err)
	}
	if copied.Name != newName {
		t.Errorf("the arrived album is called %q, want %q", copied.Name, newName)
	}
	// And it is the album with the pictures in it, not an empty one that
	// happens to have the right address.
	items, err := s.Albums.Items(ctx, report.WebsiteID, copied.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("Items = %v, %v", items, err)
	}
	if items[0].MediaID == mediaID {
		t.Errorf("the copied album still points at the original website's picture %d", mediaID)
	}
	picture, err := s.Media.GetByID(ctx, items[0].MediaID)
	if err != nil || picture == nil {
		t.Fatalf("the copied album's picture does not exist: %v", err)
	}
	if picture.WebsiteID != report.WebsiteID {
		t.Errorf("the copied album's picture belongs to website %d instead of %d",
			picture.WebsiteID, report.WebsiteID)
	}
}

// The decisive half of GAL-04 is the negative one, and it is machine-decided
// here rather than read by hand.
//
// The album is renamed after its pictures were placed, so the page holds an
// address that no longer matches the name. The manifest must not contain that
// address anywhere — not in the album list, not on the block — and must carry
// the album by its name instead.
func TestManifestCarriesNoAlbumSlug(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()
	ws, albumID, _ := seedAlbumSite(t, s, "Schreinerei", "Referenzen Aussen")

	oldSlug := page.Slugify("Referenzen Aussen")
	if err := s.Albums.Rename(ctx, ws, albumID, "Aussenanlagen"); err != nil {
		t.Fatalf("Albums.Rename: %v", err)
	}

	set := s.BlockTypes.Set(ctx, ws)
	blocks := []block.Block{{Type: block.TypeGallery, AlbumSlug: oldSlug}}
	encoded, err := block.Encode(blocks, set)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, err := s.Pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws, Title: "Aussen", Slug: "aussen",
		Markdown: block.PlainText(blocks, set), Blocks: encoded, Status: "published",
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	written := manifestOf(t, exportTo(t, s, ws))
	if strings.Contains(written, oldSlug) {
		t.Errorf("the manifest carries the album's old address %q; it must carry the "+
			"name and let the other machine derive the address:\n%s", oldSlug, written)
	}
	if !strings.Contains(written, "Aussenanlagen") {
		t.Errorf("the manifest does not carry the album's name:\n%s", written)
	}
}

// A block naming an album the archive did not bring is reported, the way
// missingMedia reports a file that did not arrive. Without it the operator
// gets an empty gallery and nothing anywhere that says why.
func TestBlockNamingAnAbsentAlbumIsReported(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Halb"},
		Albums:  []Album{{Name: "Referenzen"}},
		Pages: []Page{{
			Title: "Arbeiten", Slug: "arbeiten", Status: "published",
			Blocks: []Block{
				{Type: block.TypeGallery, Album: "Referenzen"},
				{Type: block.TypeGallery, Album: "Aussenanlagen"},
			},
		}},
	})

	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if !warned(report, "Aussenanlagen") {
		t.Errorf("the report does not name the album that did not arrive: %v", report.Warnings)
	}
	if warned(report, "Referenzen") {
		t.Errorf("the album that did arrive is reported as missing: %v", report.Warnings)
	}

	// And the reference is dropped rather than passed through: a value that
	// travelled on would land on whatever this machine derives from it.
	pg, err := s.Pages.GetPageBySlug(ctx, report.WebsiteID, "arbeiten")
	if err != nil || pg == nil {
		t.Fatalf("GetPageBySlug: %v", err)
	}
	arrived, err := block.Decode(pg.Blocks, s.BlockTypes.Set(ctx, report.WebsiteID))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// One block, not two. The reference is dropped rather than derived, and a
	// gallery with neither an album nor a list of its own is empty — so
	// set.Clean removes it, which is plan 11-05's rule and the right outcome:
	// what is left on the page is what can be drawn, and the report above is
	// where the operator reads what went missing and why.
	if len(arrived) != 1 {
		t.Fatalf("%d blocks instead of 1: %+v", len(arrived), arrived)
	}
	if arrived[0].AlbumSlug != "referenzen" {
		t.Errorf("the block naming an album that arrived points at %q", arrived[0].AlbumSlug)
	}
}

// albums is optional, so every archive already written imports unchanged and
// an export from a website without albums is byte-for-byte what it was.
func TestManifestWithoutAlbumsImportsAsBefore(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Ohne Alben"},
		Pages: []Page{{
			Title: "Start", Slug: "start", Status: "published", Markdown: "Hallo.",
		}},
	})
	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Albums != 0 {
		t.Errorf("report.Albums = %d for a manifest with no albums key", report.Albums)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("a manifest with no albums key warned: %v", report.Warnings)
	}
	if report.Pages != 1 {
		t.Errorf("report.Pages = %d, want 1", report.Pages)
	}

	// And the other direction: a website with no albums writes no albums key,
	// which is what makes an optional field not a format break.
	written := manifestOf(t, exportTo(t, s, report.WebsiteID))
	if strings.Contains(written, `"albums"`) {
		t.Errorf("a website without albums wrote an albums key:\n%s", written)
	}
	if strings.Contains(written, `"version": 2`) {
		t.Errorf("the format version moved; an optional key is not a format break")
	}
}

// The further languages of a website did not travel along. The consequences
// were silent and expensive: every translated page arrived under the main
// language, and two menus that differ only in language collided on creation —
// the copy stood there with half its navigation missing.
func TestRoundTripKeepsTheLanguages(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()
	ws := seedSite(t, s)

	if err := s.Domains.UpdateSettings(ctx, ws, domain.Settings{
		Locale: "de", ExtraLocales: "fr, it", TimeZone: "Europe/Zurich",
		OfflineMode: "notfound",
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	// A French page and two menus in the same place, one per language.
	if _, err := s.Pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws, Title: "Contact", Slug: "contact",
		Markdown: "Bonjour.", Status: "published", Locale: "fr",
	}); err != nil {
		t.Fatalf("CreatePage fr: %v", err)
	}
	// Am selben Ort, eines je Sprache — seedSite hat "main" schon vergeben.
	for _, l := range []string{"", "fr"} {
		if _, err := s.Menus.CreateMenu(ctx, ws, "Fusszeile", "footer", l); err != nil {
			t.Fatalf("CreateMenu %q: %v", l, err)
		}
	}

	var buf bytes.Buffer
	if err := Export(ctx, s, ws, "test", &buf); err != nil {
		t.Fatalf("Export: %v", err)
	}
	report, err := Import(ctx, s, bytes.NewReader(buf.Bytes()), int64(buf.Len()), "Kopie")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	kopie, err := s.Domains.GetWebsite(ctx, report.WebsiteID)
	if err != nil || kopie == nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	if got := kopie.Locales(); len(got) != 2 || got[0] != "fr" || got[1] != "it" {
		t.Fatalf("die Sprachen der Kopie: %v", got)
	}

	fr, err := s.Pages.GetPageBySlug(ctx, report.WebsiteID, "contact")
	if err != nil || fr == nil {
		t.Fatalf("the French page is missing: %v", err)
	}
	if fr.Locale != "fr" {
		t.Errorf("the French page arrived as %q", fr.Locale)
	}
	// The three from seedSite and from this test, none of them lost.
	menus, err := s.Menus.ListMenus(ctx, report.WebsiteID)
	if err != nil {
		t.Fatalf("List menus: %v", err)
	}
	if len(menus) != 3 {
		t.Errorf("%d menus instead of 3 — %v", len(menus), report.Warnings)
	}
}

// TestSameAddressInEveryLanguage is the case holzcloud.ch is made of: a product
// name is not translated, so the same address exists in five languages and is
// five pages.
//
// Before the address became unique per language, the import renamed four of
// them to "-2", "-3" and so on, and the translation links — which are resolved
// by address — all landed on whichever language happened to be imported last.
// The site then served five pages that each claimed to be the German one.
func TestSameAddressInEveryLanguage(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()
	ws := seedSite(t, s)

	if err := s.Domains.UpdateSettings(ctx, ws, domain.Settings{
		Locale: "de", ExtraLocales: "fr, it", TimeZone: "Europe/Zurich",
		OfflineMode: "notfound",
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	m := &Manifest{
		Version: Version,
		Site:    Site{Name: "holzcloud", Locale: "de", ExtraLocales: "fr, it"},
		Pages: []Page{
			{Title: "holzcloud-cms", Slug: "produkt", Status: "published", Markdown: "Deutsch."},
			{Title: "holzcloud-cms", Slug: "produkt", Status: "published", Markdown: "Français.",
				Locale: "fr", TranslationOf: "produkt"},
			{Title: "holzcloud-cms", Slug: "produkt", Status: "published", Markdown: "Italiano.",
				Locale: "it", TranslationOf: "produkt"},
		},
		Menus: []Menu{
			{Name: "Hauptmenü", LocationKey: "haupt", Items: []MenuItem{
				{Title: "Produkt", Type: "page", PageSlug: "produkt"}}},
			{Locale: "fr", Name: "Menu", LocationKey: "haupt", Items: []MenuItem{
				{Title: "Produit", Type: "page", PageSlug: "produkt"}}},
		},
	}

	report := &Report{}
	importPages(ctx, s, ws, m, map[string]int64{}, nil, map[string]string{}, block.Set{}, report)
	importMenus(ctx, s, ws, m, report)

	// Three pages, all three under the same address.
	for _, loc := range []string{"", "fr", "it"} {
		pg, err := s.Pages.GetPageBySlugIn(ctx, ws, loc, "produkt")
		if err != nil || pg == nil {
			t.Fatalf("the page in the language %q is missing: %v — %v", loc, err, report.Warnings)
		}
		if pg.Slug != "produkt" {
			t.Errorf("die Sprache %q bekam die Adresse %q statt produkt", loc, pg.Slug)
		}
	}

	de, _ := s.Pages.GetPageBySlugIn(ctx, ws, "", "produkt")
	for _, loc := range []string{"fr", "it"} {
		pg, _ := s.Pages.GetPageBySlugIn(ctx, ws, loc, "produkt")
		if pg.TranslationOf == 0 {
			t.Fatalf("the language %q hangs off no original", loc)
		}
		if pg.TranslationOf != de.ID {
			t.Errorf("the language %q translates page %d instead of %d", loc, pg.TranslationOf, de.ID)
		}
	}

	// And the French menu points at the French page.
	menus, err := s.Menus.ListMenus(ctx, ws)
	if err != nil {
		t.Fatalf("ListMenus: %v", err)
	}
	fr, _ := s.Pages.GetPageBySlugIn(ctx, ws, "fr", "produkt")
	var checked bool
	for _, mn := range menus {
		if mn.Locale != "fr" {
			continue
		}
		items, err := s.Menus.ListItems(ctx, mn.ID)
		if err != nil {
			t.Fatalf("ListItems: %v", err)
		}
		for _, it := range items {
			if it.PageID == nil {
				t.Fatal("the French menu item points at nothing at all")
			}
			if *it.PageID != fr.ID {
				t.Errorf("the French menu item points at page %d instead of %d", *it.PageID, fr.ID)
			}
			checked = true
		}
	}
	if !checked {
		t.Fatal("no French menu found")
	}
}

// A multi-value value travels as what it is: a string with one line per value.
// Only image numbers and page numbers are translated, everything else the
// archive carries unchanged — and exactly that has to stay demonstrable, or a
// later translation silently takes apart the encoding that phase 9 builds
// upon.
func TestMehrfachauswahlUeberlebtDieArchivreise(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	ws, err := s.Domains.CreateWebsite(ctx, "Sägerei", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "sorten", Label: "Sorten", Kind: field.KindMulti,
		Choices: []string{"Eiche", "Buche", "Esche"},
	}); err != nil {
		t.Fatalf("Feld anlegen: %v", err)
	}

	p, err := s.Pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "Bretter", Slug: "bretter",
		Markdown: "x", HTML: "<p>x</p>", Status: "published",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Duplicates and order are part of the value: both have to survive the
	// journey unchanged.
	wert := field.JoinValues([]string{"Esche", "Eiche", "Esche"})
	raw, err := field.Encode(field.Data{Values: field.Values{"sorten": wert}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Pages.SetFields(ctx, p.ID, raw); err != nil {
		t.Fatal(err)
	}

	archive := exportTo(t, s, ws.ID)
	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "Sägerei (Kopie)")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("Warnungen: %v", report.Warnings)
	}

	kopien, _, err := s.Pages.ListPages(ctx, report.WebsiteID, page.ListFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	var kopie *page.Page
	for i := range kopien {
		if kopien[i].Slug == "bretter" {
			kopie = &kopien[i]
		}
	}
	if kopie == nil {
		t.Fatalf("the page is missing from the copy: %+v", kopien)
	}
	got := field.Decode(kopie.Fields).Values["sorten"]
	if got != wert {
		t.Errorf("after the journey %q, wanted %q — the same character for character", got, wert)
	}
	if werte := field.SplitValues(got); len(werte) != 3 {
		t.Errorf("after the journey %d values, wanted 3: %#v", len(werte), werte)
	}
}

// The four properties from migration 00046 have to survive the archive
// journey: a bundle is the handover of a website, and a choice that shows up
// over there as a dropdown again is not the same website.
//
// Three places build a manifest field and three build a definition back out of
// it — the page field, the field inside a group and the field of a snippet
// kind. All three pairs are read here.
func TestNeueFeldeigenschaftenUeberlebenDieArchivreise(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	ws, err := s.Domains.CreateWebsite(ctx, "Schreinerei", "")
	if err != nil {
		t.Fatal(err)
	}

	// A page field as a choice: carries the presentation.
	if _, err := s.Fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "farbe", Label: "Farbe", Kind: field.KindChoice,
		Choices: []string{"hell", "dunkel"},
		Display: field.DisplayButtons,
	}); err != nil {
		t.Fatalf("Auswahlfeld anlegen: %v", err)
	}
	// A page field as a multiple choice: carries the maximum. And one as a
	// range: carries the two bounds. Three fields, because validate empties
	// whatever does not fit the kind — no single field can carry all four.
	if _, err := s.Fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "hoelzer", Label: "Hölzer", Kind: field.KindMulti,
		Choices: []string{"Eiche", "Buche"}, MaxValues: 2,
	}); err != nil {
		t.Fatalf("Mehrfachauswahlfeld anlegen: %v", err)
	}
	if _, err := s.Fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "menge", Label: "Menge", Kind: field.KindRange,
		RangeMin: "1", RangeMax: "9",
	}); err != nil {
		t.Fatalf("Bereichsfeld anlegen: %v", err)
	}
	// The same again inside a group.
	gruppe, err := s.Fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "zeiten", Label: "Zeiten", Kind: field.KindGroup,
	})
	if err != nil {
		t.Fatalf("Gruppe anlegen: %v", err)
	}
	if _, err := s.Fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, ParentID: gruppe.ID, Key: "gfarbe", Label: "Farbe",
		Kind: field.KindChoice, Choices: []string{"hell", "dunkel"},
		Display: field.DisplayButtons,
	}); err != nil {
		t.Fatalf("create field inside the group: %v", err)
	}
	if _, err := s.Fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, ParentID: gruppe.ID, Key: "gmenge", Label: "Menge",
		Kind: field.KindRange, RangeMin: "3", RangeMax: "7",
	}); err != nil {
		t.Fatalf("Bereichsfeld in der Gruppe anlegen: %v", err)
	}

	archive := exportTo(t, s, ws.ID)
	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "Schreinerei (Kopie)")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("Warnungen: %v", report.Warnings)
	}

	kopien, err := s.Fields.List(ctx, report.WebsiteID)
	if err != nil {
		t.Fatal(err)
	}
	nimm := func(defs []field.Def, key string) field.Def {
		t.Helper()
		for _, d := range defs {
			if d.Key == key {
				return d
			}
		}
		t.Fatalf("field %q is missing from the copy", key)
		return field.Def{}
	}

	farbe := nimm(kopien, "farbe")
	if farbe.Display != field.DisplayButtons {
		t.Errorf("display after the journey = %q, wanted %q", farbe.Display, field.DisplayButtons)
	}
	if !farbe.IsButtonRow() {
		t.Error("after the journey the choice is no longer a row of buttons")
	}

	menge := nimm(kopien, "menge")
	if menge.RangeMin != "1" || menge.RangeMax != "9" {
		t.Errorf("bounds after the journey = %q/%q, wanted \"1\"/\"9\"", menge.RangeMin, menge.RangeMax)
	}

	hoelzer := nimm(kopien, "hoelzer")
	if hoelzer.MaxValues != 2 {
		t.Errorf("maximum after the journey = %d, wanted 2", hoelzer.MaxValues)
	}

	inGruppe := nimm(nimm(kopien, "zeiten").Sub, "gfarbe")
	if inGruppe.Display != field.DisplayButtons {
		t.Errorf("display inside the group after the journey = %q, wanted %q",
			inGruppe.Display, field.DisplayButtons)
	}
	inGruppeMenge := nimm(nimm(kopien, "zeiten").Sub, "gmenge")
	if inGruppeMenge.RangeMin != "3" || inGruppeMenge.RangeMax != "7" {
		t.Errorf("bounds inside the group after the journey = %q/%q, wanted \"3\"/\"7\"",
			inGruppeMenge.RangeMin, inGruppeMenge.RangeMax)
	}
}

// A manifest of a website that uses none of the four new properties must carry
// none of the four keys — omitempty is the promise that an archive from before
// this phase looks the same byte for byte. An archive exists to be read and
// patched by hand.
func TestManifestSchweigtUeberUngenutzteEigenschaften(t *testing.T) {
	roh, err := json.Marshal(Field{Key: "preis", Label: "Preis", Kind: field.KindNumber})
	if err != nil {
		t.Fatal(err)
	}
	for _, schluessel := range []string{"display", "max_values", `"min"`, `"max"`} {
		if strings.Contains(string(roh), schluessel) {
			t.Errorf("the manifest names %s although nothing was set: %s", schluessel, roh)
		}
	}
}

// The round trip of a term field — and the one case that breaks it today.
//
// Rename deliberately keeps the slug: existing links should not break. A page
// afterwards carries the *old* slug while the term shows a *new* name. If the
// value travels as a slug, the other machine derives a different slug from the
// name, and the value points at nothing — silently. That is why the rename
// happens here before the export: a term whose name still matches its slug
// would travel intact even without the translation and would prove nothing.
//
// The term is moreover attached to no page. A term that no page term field
// carries used to be counted on import and not created — the second half of
// the same fault, on the same page.
func TestSchlagwortfeldRundreise(t *testing.T) {
	t.Run("umbenannt und an keiner Seite", func(t *testing.T) {
		s := newStores(t)
		ctx := context.Background()

		ws, err := s.Domains.CreateWebsite(ctx, "Werkstatt", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Fields.Create(ctx, field.Def{
			WebsiteID: ws.ID, Key: "thema", Label: "Thema", Kind: field.KindTerm,
		}); err != nil {
			t.Fatalf("Feld anlegen: %v", err)
		}
		seite, err := s.Pages.CreatePage(ctx, page.PageCreate{
			WebsiteID: ws.ID, Title: "Wollpaket", Slug: "wollpaket",
			Markdown: "x", HTML: "<p>x</p>", Status: "published",
		})
		if err != nil {
			t.Fatal(err)
		}

		// Create, rename, detach again: what remains is a term whose slug is
		// still "moebel" while its name has moved on, carried by no page.
		if err := s.Terms.SetForPage(ctx, ws.ID, seite.ID, []string{"Möbel"}); err != nil {
			t.Fatal(err)
		}
		alle, err := s.Terms.ListAll(ctx, ws.ID)
		if err != nil || len(alle) != 1 {
			t.Fatalf("ListAll = %v, %v", alle, err)
		}
		if alle[0].Slug != "moebel" {
			t.Fatalf("slug = %q, wanted moebel", alle[0].Slug)
		}
		if err := s.Terms.Rename(ctx, ws.ID, alle[0].ID, "Möbelbau"); err != nil {
			t.Fatal(err)
		}
		if err := s.Terms.SetForPage(ctx, ws.ID, seite.ID, nil); err != nil {
			t.Fatal(err)
		}

		// The stored value is the *old* slug — exactly what Rename leaves
		// behind.
		raw, err := field.Encode(field.Data{Values: field.Values{"thema": "moebel"}})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Pages.SetFields(ctx, seite.ID, raw); err != nil {
			t.Fatal(err)
		}

		archive := exportTo(t, s, ws.ID)
		geschrieben := manifestOf(t, archive)
		// The archive carries the name, the way a page's term list has always
		// carried it — not the slug.
		if !strings.Contains(geschrieben, `"thema": "Möbelbau"`) {
			t.Errorf("the archive does not carry the term's name:\n%s", geschrieben)
		}
		if strings.Contains(geschrieben, `"thema": "moebel"`) {
			t.Errorf("the archive carries the slug instead of the name:\n%s", geschrieben)
		}

		report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "Werkstatt (Kopie)")
		if err != nil {
			t.Fatalf("Import: %v", err)
		}
		if len(report.Warnings) != 0 {
			t.Errorf("Warnungen: %v", report.Warnings)
		}

		// The term is created on the new website even though no page carries
		// it.
		neue, err := s.Terms.ListAll(ctx, report.WebsiteID)
		if err != nil {
			t.Fatal(err)
		}
		if len(neue) != 1 || neue[0].Name != "Möbelbau" {
			t.Fatalf("terms of the copy = %+v, wanted exactly “Möbelbau”", neue) //nolint:german — the message quotes the German fixture it is about
		}
		if report.Terms != 1 {
			t.Errorf("report.Terms = %d, wollte 1 angelegtes Schlagwort", report.Terms)
		}

		// And the value of the page points at *this* term. The address is a
		// different one than on the source website — "moebel" there,
		// "moebelbau" here — and that is not a fault: the format derives the
		// address of a term from its name (format.go:274-284), so it may move
		// across a round trip. What may not move is which term the field
		// points at.
		kopien, _, err := s.Pages.ListPages(ctx, report.WebsiteID, page.ListFilter{Page: 1, PerPage: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(kopien) != 1 {
			t.Fatalf("Seiten der Kopie = %+v", kopien)
		}
		got := field.Decode(kopien[0].Fields).Values["thema"]
		if got != neue[0].Slug {
			t.Errorf("the field points at %q, this website's term has %q", got, neue[0].Slug)
		}
		if got == "" {
			t.Error("the value was lost on the way")
		}
	})

	t.Run("auch an einer Seite: nur einmal angelegt", func(t *testing.T) {
		s := newStores(t)
		ctx := context.Background()

		ws, err := s.Domains.CreateWebsite(ctx, "Werkstatt", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Fields.Create(ctx, field.Def{
			WebsiteID: ws.ID, Key: "thema", Label: "Thema", Kind: field.KindTerm,
		}); err != nil {
			t.Fatal(err)
		}
		seite, err := s.Pages.CreatePage(ctx, page.PageCreate{
			WebsiteID: ws.ID, Title: "Wollpaket", Slug: "wollpaket",
			Markdown: "x", HTML: "<p>x</p>", Status: "published",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Terms.SetForPage(ctx, ws.ID, seite.ID, []string{"Eiche"}); err != nil {
			t.Fatal(err)
		}
		raw, err := field.Encode(field.Data{Values: field.Values{"thema": "eiche"}})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Pages.SetFields(ctx, seite.ID, raw); err != nil {
			t.Fatal(err)
		}

		archive := exportTo(t, s, ws.ID)
		report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "Werkstatt (Kopie)")
		if err != nil {
			t.Fatalf("Import: %v", err)
		}
		neue, err := s.Terms.ListAll(ctx, report.WebsiteID)
		if err != nil {
			t.Fatal(err)
		}
		// Once from importTerms, once from the page's term list — the same
		// slug derivation, hence the same row.
		if len(neue) != 1 {
			t.Errorf("terms of the copy = %+v, wanted exactly one", neue)
		}
		// And the page still carries it as its own term.
		kopien, _, err := s.Pages.ListPages(ctx, report.WebsiteID, page.ListFilter{Page: 1, PerPage: 10})
		if err != nil || len(kopien) != 1 {
			t.Fatalf("Seiten der Kopie = %+v, %v", kopien, err)
		}
		haengt, err := s.Terms.ForPage(ctx, kopien[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(haengt) != 1 || haengt[0].Name != "Eiche" {
			t.Errorf("the page carries %+v, wanted “Eiche”", haengt)
		}
	})

	// term.MaxPerPage is an editorial limit for one entry, not for an archive.
	// If the manifest's whole list went through term.Parse, the import stopped
	// at the twelfth term — and precisely those that no page carries are the
	// reason importTerms exists at all. The report named the truncated number
	// without a word about it.
	t.Run("fünfzehn Schlagwörter, keines geht verloren", func(t *testing.T) {
		s := newStores(t)
		ctx := context.Background()

		ws, err := s.Domains.CreateWebsite(ctx, "Werkstatt", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Fields.Create(ctx, field.Def{
			WebsiteID: ws.ID, Key: "thema", Label: "Thema", Kind: field.KindTerm,
		}); err != nil {
			t.Fatalf("Feld anlegen: %v", err)
		}
		seite, err := s.Pages.CreatePage(ctx, page.PageCreate{
			WebsiteID: ws.ID, Title: "Wollpaket", Slug: "wollpaket",
			Markdown: "x", HTML: "<p>x</p>", Status: "published",
		})
		if err != nil {
			t.Fatal(err)
		}

		// Fifteen terms, on no page. The last one carries a comma in its name
		// — a manifest is a file written by hand, and the reader for a form
		// field would tear it in two.
		namen := []string{
			"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta", "Eta",
			"Theta", "Iota", "Kappa", "Lambda", "My", "Ny", "Xi",
			"Möbel, Bau",
		}
		if _, err := s.Terms.EnsureNames(ctx, ws.ID, namen); err != nil {
			t.Fatalf("EnsureNames: %v", err)
		}
		alle, err := s.Terms.ListAll(ctx, ws.ID)
		if err != nil || len(alle) != len(namen) {
			t.Fatalf("ListAll = %d terms, %v", len(alle), err)
		}

		// The field points at the last one — the one that would never be
		// created without the fix.
		letztes := alle[len(alle)-1]
		for _, tt := range alle {
			if tt.Name == "Möbel, Bau" {
				letztes = tt
			}
		}
		raw, err := field.Encode(field.Data{Values: field.Values{"thema": letztes.Slug}})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Pages.SetFields(ctx, seite.ID, raw); err != nil {
			t.Fatal(err)
		}

		archive := exportTo(t, s, ws.ID)
		report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "Werkstatt (Kopie)")
		if err != nil {
			t.Fatalf("Import: %v", err)
		}
		if len(report.Warnings) != 0 {
			t.Errorf("Warnungen: %v", report.Warnings)
		}

		neue, err := s.Terms.ListAll(ctx, report.WebsiteID)
		if err != nil {
			t.Fatal(err)
		}
		if len(neue) != len(namen) {
			t.Fatalf("the copy has %d terms, wanted %d", len(neue), len(namen))
		}
		bekannt := map[string]bool{}
		for _, tt := range neue {
			bekannt[tt.Name] = true
		}
		for _, n := range namen {
			if !bekannt[n] {
				t.Errorf("„%s“ fehlt auf der Kopie", n)
			}
		}
		if report.Terms != len(namen) {
			t.Errorf("report.Terms = %d, wollte %d", report.Terms, len(namen))
		}

		// And the field finds its term again on the copy.
		kopien, _, err := s.Pages.ListPages(ctx, report.WebsiteID, page.ListFilter{Page: 1, PerPage: 10})
		if err != nil || len(kopien) != 1 {
			t.Fatalf("Seiten der Kopie = %+v, %v", kopien, err)
		}
		got := field.Decode(kopien[0].Fields).Values["thema"]
		if got == "" {
			t.Fatal("the term field's value was lost")
		}
		gefunden := false
		for _, tt := range neue {
			if tt.Slug == got && tt.Name == "Möbel, Bau" {
				gefunden = true
			}
		}
		if !gefunden {
			t.Errorf("the field points at %q, which on the copy is no “Möbel, Bau”: %+v", got, neue) //nolint:german — the message quotes the German fixture it is about
		}
	})
}

// An archive is a file anyone can edit — just as untrusted as a form field,
// and therefore put through the same checks.
//
// Up to here the import path was the one that wrote without them: field.Encode
// stored whatever stood in the manifest, without field.Clean and without
// field.CheckAll. All other write paths are covered (internal/admin/page.go,
// the tools in internal/ai). Since 07-04 trimTo truncates nothing any more, so
// CheckAll is also the only place where the byte budget still applies at all.
func TestArchivwerteGehenDurchDieselbePruefung(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	zuLang := strings.Repeat("x", field.MaxValueBytes+1)
	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Bösartig"},
		Fields: []Field{
			{Key: "notiz", Label: "Notiz", Kind: field.KindText},
			{Key: "art", Label: "Art", Kind: field.KindChoice, Choices: []string{"Eiche", "Buche"}},
			{Key: "gut", Label: "Gut", Kind: field.KindText},
		},
		Pages: []Page{{
			Title: "Seite", Slug: "seite", Status: "published", Markdown: "x",
			Fields: map[string]string{
				"notiz":     zuLang,
				"art":       "Zement",
				"gut":       "das hier bleibt",
				"gibtesnie": "und dieses Feld gibt es gar nicht",
			},
		}},
	})

	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	seiten, _, err := s.Pages.ListPages(ctx, report.WebsiteID, page.ListFilter{Page: 1, PerPage: 10})
	if err != nil || len(seiten) != 1 {
		t.Fatalf("Seiten = %+v, %v", seiten, err)
	}
	werte := field.Decode(seiten[0].Fields).Values

	if _, da := werte["notiz"]; da {
		t.Errorf("a value over the byte budget was stored (%d bytes)", len(werte["notiz"]))
	}
	if _, da := werte["art"]; da {
		t.Errorf("“Zement” is not one of the choices and was stored regardless: %q", werte["art"])
	}
	// field.Clean removes whatever belongs to no field of this website.
	if _, da := werte["gibtesnie"]; da {
		t.Error("a value with no field definition was stored")
	}
	// And the valid value arrives: the guard does not throw away the whole page.
	if werte["gut"] != "das hier bleibt" {
		t.Errorf("the valid value = %q, wanted “das hier bleibt”", werte["gut"]) //nolint:german — the message quotes the German fixture it is about
	}
	// The report says what is missing — otherwise the operator would have to
	// find the gap themselves.
	if !warned(report, "notiz") || !warned(report, "art") {
		t.Errorf("the report does not name the discarded values: %v", report.Warnings)
	}
}

// The archive path is the only one on which a field key is brought along
// instead of derived: importFields passes Key: f.Key verbatim from the
// manifest (internal/bundle/import.go:351), and a manifest is a file anyone
// can write by hand.
//
// A key like farbe[] would be the form prefix of multi-value disguised as a
// field definition (D-03: multi-value is stated in the name of the form
// field). It is refused — but as a warning in the report and not as an abort
// of the import, the same severity as for every other discarded value: the
// real field next to it arrives all the same.
// The round trip of the fields of a snippet.
//
// An archive that carries out the definitions of a snippet and loses the
// values on import is the silent data loss this project avoids everywhere
// else: the body arrives, the website looks intact, and the half somebody
// typed in is gone. That is why the difficult journey is driven here and not
// the easy one — a text field, a number field and a group with two rows,
// together with order and field kinds on the other side.
func TestTextbausteinfelderUeberlebenDieArchivreise(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	ws, err := s.Domains.CreateWebsite(ctx, "Schreinerei", "")
	if err != nil {
		t.Fatal(err)
	}
	sn, err := s.Snippets.Create(ctx, ws.ID, "footer-kontakt", "Kontakt",
		"Telefon 07721 123456", "<p>Telefon 07721 123456</p>")
	if err != nil {
		t.Fatalf("Snippets.Create: %v", err)
	}

	// A page field with the same key stands next to it: it may neither land in
	// the definitions of the snippet nor receive its value.
	if _, err := s.Fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "telefon", Label: "Seitentelefon", Kind: field.KindText,
	}); err != nil {
		t.Fatalf("Seitenfeld: %v", err)
	}

	gruppe, err := s.Fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, SnippetID: sn.ID, Key: "zeiten", Label: "Öffnungszeiten",
		Kind: field.KindGroup,
	})
	if err != nil {
		t.Fatalf("Gruppe: %v", err)
	}
	for _, d := range []field.Def{
		{Key: "tag", Label: "Tag", Kind: field.KindText},
		{Key: "von", Label: "Von", Kind: field.KindText},
	} {
		d.WebsiteID, d.SnippetID, d.ParentID = ws.ID, sn.ID, gruppe.ID
		if _, err := s.Fields.Create(ctx, d); err != nil {
			t.Fatalf("Unterfeld %s: %v", d.Key, err)
		}
	}
	for _, d := range []field.Def{
		{Key: "telefon", Label: "Telefon", Kind: field.KindText},
		{Key: "sitzplaetze", Label: "Sitzplätze", Kind: field.KindNumber},
	} {
		d.WebsiteID, d.SnippetID = ws.ID, sn.ID
		if _, err := s.Fields.Create(ctx, d); err != nil {
			t.Fatalf("Feld %s: %v", d.Key, err)
		}
	}

	raw, err := field.Encode(field.Data{
		Values: field.Values{"telefon": "07721 123456", "sitzplaetze": "8"},
		Rows: map[string][]field.Values{"zeiten": {
			{"tag": "Montag", "von": "08:00"},
			{"tag": "Dienstag", "von": "09:00"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Snippets.SetFields(ctx, ws.ID, sn.ID, raw); err != nil {
		t.Fatal(err)
	}

	archive := exportTo(t, s, ws.ID)
	if !strings.Contains(manifestOf(t, archive), `"values"`) {
		t.Fatalf("the manifest carries no values of the snippet:\n%s", manifestOf(t, archive))
	}

	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "Schreinerei (Kopie)")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("Warnungen: %v", report.Warnings)
	}

	kopien, err := s.Snippets.List(ctx, report.WebsiteID)
	if err != nil || len(kopien) != 1 {
		t.Fatalf("List snippets: %v (%d)", err, len(kopien))
	}
	kopie := kopien[0]
	if kopie.ContentMarkdown != "Telefon 07721 123456" {
		t.Errorf("der Rumpf kam anders an: %q", kopie.ContentMarkdown)
	}

	defs, err := s.Fields.OfSnippet(ctx, report.WebsiteID, kopie.ID)
	if err != nil {
		t.Fatalf("OfSnippet: %v", err)
	}
	// Order and field kind, both: a definition that comes back as text although
	// it was a number is a different form.
	wollte := []struct{ key, kind string }{
		{"zeiten", field.KindGroup},
		{"telefon", field.KindText},
		{"sitzplaetze", field.KindNumber},
	}
	if len(defs) != len(wollte) {
		t.Fatalf("after the journey %d definitions, wanted %d: %+v", len(defs), len(wollte), defs)
	}
	for i, w := range wollte {
		if defs[i].Key != w.key || defs[i].Kind != w.kind {
			t.Errorf("Definition %d ist %q/%q, wollte %q/%q", i, defs[i].Key, defs[i].Kind, w.key, w.kind)
		}
	}
	if len(defs[0].Sub) != 2 || defs[0].Sub[0].Key != "tag" || defs[0].Sub[1].Key != "von" {
		t.Errorf("die Unterfelder der Gruppe kamen anders an: %+v", defs[0].Sub)
	}

	daten := field.Decode(kopie.Fields)
	if daten.Values["telefon"] != "07721 123456" {
		t.Errorf("telefon after the journey %q", daten.Values["telefon"])
	}
	if daten.Values["sitzplaetze"] != "8" {
		t.Errorf("sitzplaetze after the journey %q", daten.Values["sitzplaetze"])
	}
	zeilen := daten.Rows["zeiten"]
	if len(zeilen) != 2 {
		t.Fatalf("after the journey %d rows, wanted 2: %+v", len(zeilen), zeilen)
	}
	if zeilen[0]["tag"] != "Montag" || zeilen[1]["von"] != "09:00" {
		t.Errorf("die Zeilen kamen in anderer Gestalt an: %+v", zeilen)
	}

	// The dangerous cut, on the archive path: the page field has the same key
	// and must not receive the value of the snippet.
	seiten, err := s.Fields.List(ctx, report.WebsiteID)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range seiten {
		if d.SnippetID != 0 {
			t.Errorf("a snippet field is in the page list: %+v", d)
		}
	}
}

// A manifest from before this phase: the snippet carries neither fields nor
// values, and it arrives the way it has always arrived.
//
// This is SNIP-05 for the archive. Both keys carry omitempty, so "no key" is
// exactly what an older bundle writes — and the import path must not make half
// a snippet out of it.
func TestAeltererTextbausteinImportiertUnveraendert(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Alte Schreinerei"},
		Snippets: []Snippet{{
			Key: "footer-kontakt", Name: "Kontakt", Markdown: "Telefon 07721 123456",
		}},
	})
	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("Warnungen: %v", report.Warnings)
	}
	if report.Snippets != 1 {
		t.Fatalf("counted %d snippets, wanted 1", report.Snippets)
	}

	kopien, err := s.Snippets.List(ctx, report.WebsiteID)
	if err != nil || len(kopien) != 1 {
		t.Fatalf("List snippets: %v (%d)", err, len(kopien))
	}
	if kopien[0].ContentMarkdown != "Telefon 07721 123456" {
		t.Errorf("der Rumpf kam anders an: %q", kopien[0].ContentMarkdown)
	}
	if kopien[0].ContentHTML == "" {
		t.Error("the body was not rendered")
	}
	if kopien[0].Fields != "" {
		t.Errorf("the snippet carries values although the manifest names none: %q", kopien[0].Fields)
	}
	defs, err := s.Fields.OfSnippet(ctx, report.WebsiteID, kopien[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 0 {
		t.Errorf("the snippet was given definitions out of nowhere: %+v", defs)
	}

	// And the counter-check on the writing side: a snippet without fields
	// writes a manifest that does not name the two new keys.
	roh, err := json.Marshal(Snippet{Key: "k", Name: "n", Markdown: "m"})
	if err != nil {
		t.Fatal(err)
	}
	for _, schluessel := range []string{`"fields"`, `"values"`, `"value_groups"`} {
		if strings.Contains(string(roh), schluessel) {
			t.Errorf("the manifest names %s although nothing was set: %s", schluessel, roh)
		}
	}
}

// A manifest is a file somebody wrote: a definition that validate refuses
// costs its field and not the import, and a value under a key that does not
// exist is removed by field.Clean instead of being stored.
func TestTextbausteinfelderAusDemArchivGehenDurchDieselbePruefung(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Bösartig"},
		Snippets: []Snippet{{
			Key: "footer-kontakt", Name: "Kontakt", Markdown: "x",
			Fields: []Field{
				{Key: "telefon", Label: "Telefon", Kind: field.KindText},
				// A field kind that does not exist: validate refuses it.
				{Key: "kaputt", Label: "Kaputt", Kind: "gibtesnicht"},
			},
			Values: map[string]string{
				"telefon":  "07721 123456",
				"kaputt":   "steht unter keiner Definition",
				"erfunden": "auch nicht",
			},
		}},
	})
	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Snippets != 1 {
		t.Fatalf("counted %d snippets, wanted 1", report.Snippets)
	}
	if len(report.Warnings) == 0 {
		t.Error("die abgewiesene Definition wurde nicht gemeldet — der Betreiber " +
			"muss erfahren, was nicht angekommen ist")
	}

	kopien, err := s.Snippets.List(ctx, report.WebsiteID)
	if err != nil || len(kopien) != 1 {
		t.Fatalf("List snippets: %v (%d)", err, len(kopien))
	}
	defs, err := s.Fields.OfSnippet(ctx, report.WebsiteID, kopien[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 || defs[0].Key != "telefon" {
		t.Fatalf("die abgewiesene Definition wurde trotzdem angelegt: %+v", defs)
	}

	werte := field.Decode(kopien[0].Fields).Values
	if werte["telefon"] != "07721 123456" {
		t.Errorf("the valid value did not arrive: %q", werte["telefon"])
	}
	for _, kennung := range []string{"kaputt", "erfunden"} {
		if _, ok := werte[kennung]; ok {
			t.Errorf("%q was stored although no definition carries it: %+v", kennung, werte)
		}
	}
}

func TestArchivSchluesselMitKlammernWirdAbgelehnt(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Von Hand gebaut"},
		Fields: []Field{
			{Key: "farbe[]", Label: "Farbe", Kind: field.KindChoice, Choices: []string{"rot", "blau"}},
			{Key: "farbe", Label: "Farbe echt", Kind: field.KindChoice, Choices: []string{"rot", "blau"}},
		},
	})

	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	defs, err := s.Fields.List(ctx, report.WebsiteID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(defs) != 1 {
		keys := make([]string, 0, len(defs))
		for _, d := range defs {
			keys = append(keys, d.Key)
		}
		t.Fatalf("angelegte Felder = %v, wollte nur farbe", keys)
	}
	if defs[0].Key != "farbe" {
		t.Errorf("angelegtes Feld = %q, wollte farbe", defs[0].Key)
	}
	// The report says what is missing — otherwise the operator would have to
	// find the gap themselves.
	if !warned(report, "Farbe") {
		t.Errorf("the report does not name the discarded field: %v", report.Warnings)
	}
}

// A manifest that brings values and no definitions is the one shape that is
// written entirely by hand.
//
// Both guards — field.Clean and field.CheckAll — hung off "if len(defs) > 0".
// An archive without fields therefore got past neither of them and went into
// the column unchanged: the one manifest shape for which there is no screen and
// no form was at the same time the only one stored without a check. The double
// comment above cleanSnippetValues says the opposite — it names the hole that
// 07-04 closed on the page, and exactly that one stood open here.
//
// Without a definition a value is displayable by nothing; it belongs in the
// bin, and both carriers — the page and the snippet — have to behave alike in
// this, because it is the same decision.
func TestWerteOhneDefinitionWerdenAufBeidenTraegernVerworfen(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	zuLang := strings.Repeat("x", field.MaxValueBytes+1)
	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Ohne Definitionen"},
		// No Fields on the manifest and none on the snippet — only values.
		Pages: []Page{{
			Title: "Seite", Slug: "seite", Status: "published", Markdown: "x",
			Fields:      map[string]string{"erfunden": zuLang},
			FieldGroups: map[string][]map[string]string{"gibtesnie": {{"tag": "Montag"}}},
		}},
		Snippets: []Snippet{{
			Key: "footer-kontakt", Name: "Kontakt", Markdown: "x",
			Values:      map[string]string{"erfunden": zuLang},
			ValueGroups: map[string][]map[string]string{"gibtesnie": {{"tag": "Montag"}}},
		}},
	})

	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	seiten, _, err := s.Pages.ListPages(ctx, report.WebsiteID, page.ListFilter{Page: 1, PerPage: 10})
	if err != nil || len(seiten) != 1 {
		t.Fatalf("Seiten = %+v, %v", seiten, err)
	}
	seitenwerte := field.Decode(seiten[0].Fields)
	if _, ok := seitenwerte.Values["erfunden"]; ok {
		t.Errorf("page: a value under no definition was stored (%d bytes)",
			len(seitenwerte.Values["erfunden"]))
	}
	if len(seitenwerte.Rows) != 0 {
		t.Errorf("page: group rows under no definition were stored: %+v", seitenwerte.Rows)
	}

	kopien, err := s.Snippets.List(ctx, report.WebsiteID)
	if err != nil || len(kopien) != 1 {
		t.Fatalf("List snippets: %v (%d)", err, len(kopien))
	}
	bausteinwerte := field.Decode(kopien[0].Fields)
	if _, ok := bausteinwerte.Values["erfunden"]; ok {
		t.Errorf("snippet: a value under no definition was stored (%d bytes)",
			len(bausteinwerte.Values["erfunden"]))
	}
	if len(bausteinwerte.Rows) != 0 {
		t.Errorf("Textbaustein: Gruppenzeilen unter keiner Definition wurden abgelegt: %+v",
			bausteinwerte.Rows)
	}

	// And the column stays small. What is measured is the raw value and not
	// just the map: a value can fall through Decode and still stand in the
	// database.
	if n := len(kopien[0].Fields); n > 64 {
		t.Errorf("die fields-Spalte des Textbausteins hält %d Byte — ein von Hand "+
			"geschriebenes Manifest darf nicht ungeprüft in die Spalte laufen", n)
	}
}

// snippet.Store.Create ends on s.Get(ctx, id), and Get hands out (nil, nil)
// when the row is not there (internal/snippet/store.go:92-94). Get reads
// through s.DB.Read — a different pool than the one that wrote.
//
// Before 08-05 the return value was thrown away ("if _, err := …"). Since the
// import pulls the fields of the snippet along, created.ID is read, and
// (nil, nil) is therefore no longer an empty result but a crash that takes the
// whole request with it. internal/admin/snippet.go:307-310 watches over the
// same value — the two call sites disagreed about whether it can be nil.
//
// The situation is reproduced over the read pool and not over a mock: the
// write pool points at the real database, the read pool at a second, empty
// one. Exactly what Get sees when its row is not there.
func TestTextbausteinDerSichNichtZurueckLesenLaesstStuerztNicht(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	leer, err := db.Open(filepath.Join(t.TempDir(), "leer.sqlite"))
	if err != nil {
		t.Fatalf("db.Open (leer): %v", err)
	}
	t.Cleanup(leer.Close)
	if err := db.RunMigrations(leer.Write); err != nil {
		t.Fatalf("RunMigrations (leer): %v", err)
	}
	s.Snippets = &snippet.Store{DB: &db.DB{Write: s.Snippets.DB.Write, Read: leer.Read}}

	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Blindes Lesen"},
		Snippets: []Snippet{{
			Key: "footer-kontakt", Name: "Kontakt", Markdown: "x",
			Fields: []Field{{Key: "telefon", Label: "Telefon", Kind: field.KindText}},
			Values: map[string]string{"telefon": "07721 123456"},
		}},
	})

	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Snippets != 0 {
		t.Errorf("report.Snippets = %d, erwartet 0 — gezählt wird, was zurückgelesen "+
			"werden konnte", report.Snippets)
	}
	if !warned(report, "footer-kontakt") {
		t.Errorf("the report says nothing about the snippet that did not arrive: %v", report.Warnings)
	}
}

// An image field on a snippet does not survive the archive journey, and the
// report says so now.
//
// The value of an image, reference or term field is a number of *this*
// installation. On the page path such numbers are translated into a file name
// and an address on the way out and back again on the way in
// (exportFieldValues/translateIn); the values of a snippet go out raw and come
// in raw. Over there the number belongs to a different website, and
// fieldImages/fieldRefs refuse it — the field arrives, the image does not.
//
// That stays so for now: the translation sits in the page path and prising it
// loose from there is a piece of work of its own (deferred-items.md). What
// changes here is the volume. The admin screen offers these field kinds
// explicitly and field_list.html promises them to the operator; a promise
// broken silently on the way out is the silent data loss this project avoids
// everywhere else.
func TestBildwertEinesTextbausteinsWirdBeimImportGemeldet(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Mit Bild"},
		Snippets: []Snippet{{
			Key: "footer-kontakt", Name: "Kontakt", Markdown: "x",
			Fields: []Field{
				{Key: "logo", Label: "Logo", Kind: field.KindImage},
				{Key: "telefon", Label: "Telefon", Kind: field.KindText},
			},
			Values: map[string]string{"logo": "42", "telefon": "07721 123456"},
		}},
	})

	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if !warned(report, "logo") {
		t.Errorf("der Bericht schweigt über das Bildfeld, dessen Wert hier ins Leere "+
			"zeigt: %v", report.Warnings)
	}

	// And the rest arrives intact: the message replaces no value and throws
	// none away.
	kopien, err := s.Snippets.List(ctx, report.WebsiteID)
	if err != nil || len(kopien) != 1 {
		t.Fatalf("List snippets: %v (%d)", err, len(kopien))
	}
	werte := field.Decode(kopien[0].Fields).Values
	if werte["telefon"] != "07721 123456" {
		t.Errorf("the valid value did not arrive: %q", werte["telefon"])
	}
}
