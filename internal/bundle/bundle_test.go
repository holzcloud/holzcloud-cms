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
	if !warned(report, "Passwort") {
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
	if !warned(report, "unzulässigen Namen") {
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
	if !warned(report, "beschädigt") {
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

// Ein Verweis ist eine Seiten-Nummer, und eine Nummer bedeutet auf der anderen
// Maschine nichts. Er reist als Adresse — und zwar auch dann, wenn er auf eine
// Seite zeigt, die im Archiv erst später kommt.
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

	// Die Zielseite wird nach der verweisenden angelegt, damit der Import sie
	// beim ersten Durchgang nicht kennen kann.
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
		t.Error("das Archiv trägt die Nummer der Seite statt ihrer Adresse")
	}
	if !strings.Contains(geschrieben, `"gehoert_zu": "schafe"`) {
		t.Errorf("das Archiv trägt die Adresse der Zielseite nicht:\n%s", geschrieben)
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

// Eine mit Bausteinen gebaute Seite reiste bis hierher als reiner Text: die
// Bausteine standen nie im Archiv. Dieser Test ist die Zusage, dass sie es tun
// — samt der eigenen Bausteinart und samt dem Bild darin, das als Dateiname
// reist und auf der anderen Seite eine neue Nummer bekommt.
func TestRoundTripKeepsBlocks(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()
	ws := seedSite(t, s)

	// Ein Bild, das der Export auch wirklich mitnehmen kann.
	dir := filepath.Join(s.DataDir, "media", strconv.FormatInt(ws, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	foto := []byte("ein foto")
	if err := os.WriteFile(filepath.Join(dir, "teig.jpg"), foto, 0o644); err != nil {
		t.Fatal(err)
	}
	// Die Prüfsumme muss zur Datei passen: der Import wirft eine Datei weg,
	// deren Summe nicht stimmt, und das wäre hier kein Fehler des Bündels.
	bild, err := s.Media.Create(ctx, ws, "teig.jpg", "teig.jpg", "image/jpeg",
		int64(len(foto)), hashBytes(foto))
	if err != nil {
		t.Fatalf("Media.Create: %v", err)
	}

	// Eine eigene Bausteinart mit einem Text- und einem Bildfeld.
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
		// The album slug rides beside the display for the same reason and it
		// is a pass-through in this version: the manifest has no albums entry
		// until plan 11-06, so there is no name to translate the slug into
		// yet. Carrying it now means it is never silently lost.
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
		t.Fatal("die Seite kam ohne Bausteine an")
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

	// Die Bildnummer muss eine neue sein — die der Kopie, nicht die des
	// Originals. Genau hier ginge ein Bündel sonst still auf die Bibliothek
	// der falschen Website.
	neuesBild, err := s.Media.GetByID(ctx, angekommen[1].MediaID)
	if err != nil || neuesBild == nil {
		t.Fatalf("das Bild des Bausteins gibt es nicht: %v", err)
	}
	if neuesBild.WebsiteID != report.WebsiteID {
		t.Errorf("das Bild gehört zu Website %d statt %d", neuesBild.WebsiteID, report.WebsiteID)
	}
	if angekommen[2].Fields["bild"] != strconv.FormatInt(neuesBild.ID, 10) {
		t.Errorf("das Bild im eigenen Baustein zeigt auf %q statt auf %d",
			angekommen[2].Fields["bild"], neuesBild.ID)
	}

	// Und die Seite ist gesetzt, nicht leer: das HTML entsteht beim Import neu.
	if !strings.Contains(pg.ContentHTML, "hc-eigen--rezeptschritt") {
		t.Errorf("die Seite wurde nicht neu gesetzt:\n%s", pg.ContentHTML)
	}
	if !strings.Contains(pg.ContentHTML, neuesBild.URL()) {
		t.Errorf("das Bild fehlt in der gesetzten Seite:\n%s", pg.ContentHTML)
	}
	if !strings.Contains(pg.ContentMarkdown, "Schritt 1") {
		t.Errorf("der reine Text für Suche und Anriss fehlt: %q", pg.ContentMarkdown)
	}
}

// Die weiteren Sprachen einer Website reisten nicht mit. Die Folgen waren
// still und teuer: jede übersetzte Seite kam unter der Hauptsprache an, und
// zwei Menüs, die sich nur in der Sprache unterscheiden, stiessen beim Anlegen
// zusammen — die Kopie stand ohne halbe Navigation da.
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
	// Eine französische Seite und zwei Menüs am selben Ort, eines je Sprache.
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
		t.Fatalf("die französische Seite fehlt: %v", err)
	}
	if fr.Locale != "fr" {
		t.Errorf("die französische Seite kam als %q an", fr.Locale)
	}
	// Die drei aus seedSite und diesem Test, keines davon verloren.
	menus, err := s.Menus.ListMenus(ctx, report.WebsiteID)
	if err != nil {
		t.Fatalf("List menus: %v", err)
	}
	if len(menus) != 3 {
		t.Errorf("%d Menüs statt 3 — %v", len(menus), report.Warnings)
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
	importPages(ctx, s, ws, m, map[string]int64{}, map[string]string{}, block.Set{}, report)
	importMenus(ctx, s, ws, m, report)

	// Drei Seiten, alle drei unter derselben Adresse.
	for _, loc := range []string{"", "fr", "it"} {
		pg, err := s.Pages.GetPageBySlugIn(ctx, ws, loc, "produkt")
		if err != nil || pg == nil {
			t.Fatalf("die Seite in der Sprache %q fehlt: %v — %v", loc, err, report.Warnings)
		}
		if pg.Slug != "produkt" {
			t.Errorf("die Sprache %q bekam die Adresse %q statt produkt", loc, pg.Slug)
		}
	}

	de, _ := s.Pages.GetPageBySlugIn(ctx, ws, "", "produkt")
	for _, loc := range []string{"fr", "it"} {
		pg, _ := s.Pages.GetPageBySlugIn(ctx, ws, loc, "produkt")
		if pg.TranslationOf == 0 {
			t.Fatalf("die Sprache %q hängt an keinem Original", loc)
		}
		if pg.TranslationOf != de.ID {
			t.Errorf("die Sprache %q übersetzt Seite %d statt %d", loc, pg.TranslationOf, de.ID)
		}
	}

	// Und das französische Menü zeigt auf die französische Seite.
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
				t.Fatal("der französische Menüpunkt zeigt auf gar nichts")
			}
			if *it.PageID != fr.ID {
				t.Errorf("der französische Menüpunkt zeigt auf Seite %d statt %d", *it.PageID, fr.ID)
			}
			checked = true
		}
	}
	if !checked {
		t.Fatal("kein französisches Menü gefunden")
	}
}

// Ein mehrwertiger Wert reist als das, was er ist: eine Zeichenkette mit einer
// Zeile je Wert. Übersetzt werden nur Bildnummern und Seitennummern, alles
// andere trägt das Archiv unverändert — und genau das muss nachweisbar bleiben,
// sonst zerlegt eine spätere Übersetzung still die Kodierung, auf der Phase 9
// aufbaut.
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
	// Doppelte und Reihenfolge sind Teil des Wertes: beide müssen die Reise
	// unverändert überstehen.
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
		t.Fatalf("die Seite fehlt in der Kopie: %+v", kopien)
	}
	got := field.Decode(kopie.Fields).Values["sorten"]
	if got != wert {
		t.Errorf("nach der Reise %q, wollte %q — Zeichen für Zeichen dasselbe", got, wert)
	}
	if werte := field.SplitValues(got); len(werte) != 3 {
		t.Errorf("nach der Reise %d Werte, wollte 3: %#v", len(werte), werte)
	}
}

// Die vier Eigenschaften aus Wanderung 00046 müssen die Archivreise überstehen:
// Ein Bundle ist die Übergabe einer Website, und eine Auswahl, die drüben
// wieder als Klappliste erscheint, ist nicht dieselbe Website.
//
// Drei Stellen bauen ein Manifest-Feld und drei bauen daraus wieder eine
// Definition — das Seitenfeld, das Feld in einer Gruppe und das Feld einer
// Bausteinart. Alle drei Paare werden hier gelesen.
func TestNeueFeldeigenschaftenUeberlebenDieArchivreise(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	ws, err := s.Domains.CreateWebsite(ctx, "Schreinerei", "")
	if err != nil {
		t.Fatal(err)
	}

	// Ein Seitenfeld als Auswahl: trägt die Darstellung.
	if _, err := s.Fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "farbe", Label: "Farbe", Kind: field.KindChoice,
		Choices: []string{"hell", "dunkel"},
		Display: field.DisplayButtons,
	}); err != nil {
		t.Fatalf("Auswahlfeld anlegen: %v", err)
	}
	// Ein Seitenfeld als Mehrfachauswahl: trägt die Höchstzahl. Und eines als
	// Bereich: trägt die beiden Grenzen. Drei Felder, weil validate leert, was
	// zur Art nicht passt — kein einziges Feld kann alle vier tragen.
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
	// Dasselbe noch einmal in einer Gruppe.
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
		t.Fatalf("Feld in der Gruppe anlegen: %v", err)
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
		t.Fatalf("Feld %q fehlt in der Kopie", key)
		return field.Def{}
	}

	farbe := nimm(kopien, "farbe")
	if farbe.Display != field.DisplayButtons {
		t.Errorf("Darstellung nach der Reise = %q, wollte %q", farbe.Display, field.DisplayButtons)
	}
	if !farbe.IsButtonRow() {
		t.Error("die Auswahl ist nach der Reise keine Knopfreihe mehr")
	}

	menge := nimm(kopien, "menge")
	if menge.RangeMin != "1" || menge.RangeMax != "9" {
		t.Errorf("Grenzen nach der Reise = %q/%q, wollte \"1\"/\"9\"", menge.RangeMin, menge.RangeMax)
	}

	hoelzer := nimm(kopien, "hoelzer")
	if hoelzer.MaxValues != 2 {
		t.Errorf("Höchstzahl nach der Reise = %d, wollte 2", hoelzer.MaxValues)
	}

	inGruppe := nimm(nimm(kopien, "zeiten").Sub, "gfarbe")
	if inGruppe.Display != field.DisplayButtons {
		t.Errorf("Darstellung in der Gruppe nach der Reise = %q, wollte %q",
			inGruppe.Display, field.DisplayButtons)
	}
	inGruppeMenge := nimm(nimm(kopien, "zeiten").Sub, "gmenge")
	if inGruppeMenge.RangeMin != "3" || inGruppeMenge.RangeMax != "7" {
		t.Errorf("Grenzen in der Gruppe nach der Reise = %q/%q, wollte \"3\"/\"7\"",
			inGruppeMenge.RangeMin, inGruppeMenge.RangeMax)
	}
}

// Ein Manifest einer Website, die keine der vier neuen Eigenschaften benutzt,
// darf keine der vier Schlüssel tragen — omitempty ist das Versprechen, dass
// ein Archiv von vor dieser Phase Byte für Byte gleich aussieht. Ein Archiv
// ist dazu da, von Hand gelesen und geflickt zu werden.
func TestManifestSchweigtUeberUngenutzteEigenschaften(t *testing.T) {
	roh, err := json.Marshal(Field{Key: "preis", Label: "Preis", Kind: field.KindNumber})
	if err != nil {
		t.Fatal(err)
	}
	for _, schluessel := range []string{"display", "max_values", `"min"`, `"max"`} {
		if strings.Contains(string(roh), schluessel) {
			t.Errorf("das Manifest nennt %s, obwohl nichts gesetzt war: %s", schluessel, roh)
		}
	}
}

// Die Rundreise eines Schlagwortfeldes — und der eine Fall, der sie heute
// zerbricht.
//
// Rename behält absichtlich das Kürzel: bestehende Links sollen nicht
// zerbrechen. Eine Seite trägt danach das *alte* Kürzel, während das
// Schlagwort einen *neuen* Namen zeigt. Reist der Wert als Kürzel, leitet die
// andere Maschine aus dem Namen ein anderes Kürzel ab, und der Wert zeigt auf
// nichts — lautlos. Deshalb wird hier vor dem Export umbenannt: ein
// Schlagwort, dessen Name noch zu seinem Kürzel passt, reist auch ohne die
// Übersetzung heil und bewiese gar nichts.
//
// Das Schlagwort hängt ausserdem an keiner Seite. Ein Schlagwort, das kein
// Seiten-Schlagwortfeld trägt, wurde beim Import bisher gezählt und nicht
// angelegt — die zweite Hälfte desselben Fehlers, in derselben Seite.
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

		// Anlegen, umbenennen, wieder abhängen: übrig bleibt ein Schlagwort
		// mit dem Kürzel "moebel" und dem Namen "Möbelbau", das keine Seite
		// trägt.
		if err := s.Terms.SetForPage(ctx, ws.ID, seite.ID, []string{"Möbel"}); err != nil {
			t.Fatal(err)
		}
		alle, err := s.Terms.ListAll(ctx, ws.ID)
		if err != nil || len(alle) != 1 {
			t.Fatalf("ListAll = %v, %v", alle, err)
		}
		if alle[0].Slug != "moebel" {
			t.Fatalf("Kürzel = %q, wollte moebel", alle[0].Slug)
		}
		if err := s.Terms.Rename(ctx, ws.ID, alle[0].ID, "Möbelbau"); err != nil {
			t.Fatal(err)
		}
		if err := s.Terms.SetForPage(ctx, ws.ID, seite.ID, nil); err != nil {
			t.Fatal(err)
		}

		// Der gespeicherte Wert ist das *alte* Kürzel — genau das, was Rename
		// hinterlässt.
		raw, err := field.Encode(field.Data{Values: field.Values{"thema": "moebel"}})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Pages.SetFields(ctx, seite.ID, raw); err != nil {
			t.Fatal(err)
		}

		archive := exportTo(t, s, ws.ID)
		geschrieben := manifestOf(t, archive)
		// Das Archiv trägt den Namen, so wie eine Schlagwortliste einer Seite
		// ihn immer schon getragen hat — nicht das Kürzel.
		if !strings.Contains(geschrieben, `"thema": "Möbelbau"`) {
			t.Errorf("das Archiv trägt den Namen des Schlagworts nicht:\n%s", geschrieben)
		}
		if strings.Contains(geschrieben, `"thema": "moebel"`) {
			t.Errorf("das Archiv trägt das Kürzel statt des Namens:\n%s", geschrieben)
		}

		report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "Werkstatt (Kopie)")
		if err != nil {
			t.Fatalf("Import: %v", err)
		}
		if len(report.Warnings) != 0 {
			t.Errorf("Warnungen: %v", report.Warnings)
		}

		// Das Schlagwort ist auf der neuen Website angelegt, obwohl keine
		// Seite es trägt.
		neue, err := s.Terms.ListAll(ctx, report.WebsiteID)
		if err != nil {
			t.Fatal(err)
		}
		if len(neue) != 1 || neue[0].Name != "Möbelbau" {
			t.Fatalf("Schlagwörter der Kopie = %+v, wollte genau „Möbelbau“", neue)
		}
		if report.Terms != 1 {
			t.Errorf("report.Terms = %d, wollte 1 angelegtes Schlagwort", report.Terms)
		}

		// Und der Wert der Seite zeigt auf *dieses* Schlagwort. Die Adresse
		// ist eine andere als auf der Quellwebsite — dort "moebel", hier
		// "moebelbau" — und das ist kein Fehler: das Format leitet die
		// Adresse eines Schlagworts aus seinem Namen ab (format.go:274-284),
		// also darf sie sich über eine Rundreise bewegen. Was sich nicht
		// bewegen darf, ist, auf welches Schlagwort das Feld zeigt.
		kopien, _, err := s.Pages.ListPages(ctx, report.WebsiteID, page.ListFilter{Page: 1, PerPage: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(kopien) != 1 {
			t.Fatalf("Seiten der Kopie = %+v", kopien)
		}
		got := field.Decode(kopien[0].Fields).Values["thema"]
		if got != neue[0].Slug {
			t.Errorf("das Feld zeigt auf %q, das Schlagwort dieser Website hat %q", got, neue[0].Slug)
		}
		if got == "" {
			t.Error("der Wert ist unterwegs verlorengegangen")
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
		// Einmal von importTerms, einmal von der Schlagwortliste der Seite —
		// dieselbe Kürzelableitung, also dieselbe Zeile.
		if len(neue) != 1 {
			t.Errorf("Schlagwörter der Kopie = %+v, wollte genau eines", neue)
		}
		// Und die Seite trägt es weiterhin als eigenes Schlagwort.
		kopien, _, err := s.Pages.ListPages(ctx, report.WebsiteID, page.ListFilter{Page: 1, PerPage: 10})
		if err != nil || len(kopien) != 1 {
			t.Fatalf("Seiten der Kopie = %+v, %v", kopien, err)
		}
		haengt, err := s.Terms.ForPage(ctx, kopien[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(haengt) != 1 || haengt[0].Name != "Eiche" {
			t.Errorf("die Seite trägt %+v, wollte „Eiche“", haengt)
		}
	})

	// term.MaxPerPage ist eine redaktionelle Grenze für einen Eintrag, nicht
	// für ein Archiv. Ging die ganze Liste des Manifests durch term.Parse,
	// hörte der Import beim zwölften Schlagwort auf — und gerade die, die
	// keine Seite trägt, sind der Grund, warum importTerms überhaupt
	// existiert. Der Bericht nannte die gekürzte Zahl ohne ein Wort dazu.
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

		// Fünfzehn Schlagwörter, an keiner Seite. Das letzte trägt ein Komma
		// im Namen — ein Manifest ist eine Datei von Hand, und der Leser für
		// ein Formularfeld zerrisse ihn in zwei.
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
			t.Fatalf("ListAll = %d Schlagwörter, %v", len(alle), err)
		}

		// Das Feld zeigt auf das letzte — das, das ohne den Fehler nie
		// angelegt würde.
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
			t.Fatalf("die Kopie hat %d Schlagwörter, wollte %d", len(neue), len(namen))
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

		// Und das Feld findet sein Schlagwort auf der Kopie wieder.
		kopien, _, err := s.Pages.ListPages(ctx, report.WebsiteID, page.ListFilter{Page: 1, PerPage: 10})
		if err != nil || len(kopien) != 1 {
			t.Fatalf("Seiten der Kopie = %+v, %v", kopien, err)
		}
		got := field.Decode(kopien[0].Fields).Values["thema"]
		if got == "" {
			t.Fatal("der Wert des Schlagwortfeldes ging verloren")
		}
		gefunden := false
		for _, tt := range neue {
			if tt.Slug == got && tt.Name == "Möbel, Bau" {
				gefunden = true
			}
		}
		if !gefunden {
			t.Errorf("das Feld zeigt auf %q, das auf der Kopie kein „Möbel, Bau“ ist: %+v", got, neue)
		}
	})
}

// Ein Archiv ist eine Datei, die jeder bearbeiten kann — genauso unvertraut
// wie ein Formularfeld, und deshalb durch dieselben Prüfungen.
//
// Bis hierher war der Importweg der eine, der ohne sie schrieb: field.Encode
// legte ab, was im Manifest stand, ohne field.Clean und ohne field.CheckAll.
// Alle anderen Schreibwege sind gedeckt (internal/admin/page.go, die Werkzeuge
// in internal/ai). Seit 07-04 kürzt trimTo nichts mehr, also ist CheckAll auch
// die einzige Stelle, an der das Bytebudget überhaupt noch gilt.
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
		t.Errorf("ein Wert über dem Bytebudget wurde abgelegt (%d Byte)", len(werte["notiz"]))
	}
	if _, da := werte["art"]; da {
		t.Errorf("„Zement“ steht nicht zur Auswahl und wurde trotzdem abgelegt: %q", werte["art"])
	}
	// field.Clean nimmt weg, was zu keinem Feld dieser Website gehört.
	if _, da := werte["gibtesnie"]; da {
		t.Error("ein Wert ohne Felddefinition wurde abgelegt")
	}
	// Und der gültige Wert kommt an: die Wache wirft nicht die ganze Seite weg.
	if werte["gut"] != "das hier bleibt" {
		t.Errorf("der gültige Wert = %q, wollte „das hier bleibt“", werte["gut"])
	}
	// Der Bericht sagt, was fehlt — sonst müsste der Betreiber die Lücke
	// selbst finden.
	if !warned(report, "notiz") || !warned(report, "art") {
		t.Errorf("der Bericht nennt die verworfenen Werte nicht: %v", report.Warnings)
	}
}

// Der Archivweg ist der einzige, auf dem ein Feldschlüssel mitgebracht statt
// abgeleitet wird: importFields übergibt Key: f.Key wörtlich aus dem Manifest
// (internal/bundle/import.go:351), und ein Manifest ist eine Datei, die jeder
// von Hand schreiben kann.
//
// Ein Schlüssel wie farbe[] wäre das Formularpräfix der Mehrwertigkeit als
// Felddefinition getarnt (D-03: die Mehrwertigkeit steht im Namen des
// Formularfeldes). Er wird abgelehnt — aber als Warnung im Bericht und nicht
// als Abbruch des Imports, dieselbe Härte wie bei jedem anderen verworfenen
// Wert: das echte Feld daneben kommt trotzdem an.
// Die Rundreise der Felder eines Textbausteins.
//
// Ein Archiv, das die Definitionen eines Textbausteins ausführt und die Werte
// beim Import verliert, ist der lautlose Datenverlust, den dieses Projekt
// überall sonst vermeidet: der Rumpf kommt an, die Website sieht heil aus, und
// die Hälfte, die jemand eingetippt hat, ist weg. Deshalb wird hier die
// schwierige Reise gefahren und nicht die leichte — ein Textfeld, ein Zahlfeld
// und eine Gruppe mit zwei Zeilen, samt Reihenfolge und Feldarten auf der
// anderen Seite.
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

	// Ein Seitenfeld mit derselben Kennung steht danebem: es darf weder in den
	// Definitionen des Textbausteins landen noch dessen Wert bekommen.
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
	if err := s.Snippets.SetFields(ctx, sn.ID, raw); err != nil {
		t.Fatal(err)
	}

	archive := exportTo(t, s, ws.ID)
	if !strings.Contains(manifestOf(t, archive), `"values"`) {
		t.Fatalf("das Manifest trägt keine Werte des Textbausteins:\n%s", manifestOf(t, archive))
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
	// Reihenfolge und Feldart, beides: eine Definition, die als Text
	// zurückkommt, obwohl sie eine Zahl war, ist ein anderes Formular.
	wollte := []struct{ key, kind string }{
		{"zeiten", field.KindGroup},
		{"telefon", field.KindText},
		{"sitzplaetze", field.KindNumber},
	}
	if len(defs) != len(wollte) {
		t.Fatalf("nach der Reise %d Definitionen, wollte %d: %+v", len(defs), len(wollte), defs)
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
		t.Errorf("telefon nach der Reise %q", daten.Values["telefon"])
	}
	if daten.Values["sitzplaetze"] != "8" {
		t.Errorf("sitzplaetze nach der Reise %q", daten.Values["sitzplaetze"])
	}
	zeilen := daten.Rows["zeiten"]
	if len(zeilen) != 2 {
		t.Fatalf("nach der Reise %d Zeilen, wollte 2: %+v", len(zeilen), zeilen)
	}
	if zeilen[0]["tag"] != "Montag" || zeilen[1]["von"] != "09:00" {
		t.Errorf("die Zeilen kamen in anderer Gestalt an: %+v", zeilen)
	}

	// Der gefährliche Schnitt, auf dem Archivweg: das Seitenfeld hat dieselbe
	// Kennung und darf den Wert des Textbausteins nicht bekommen.
	seiten, err := s.Fields.List(ctx, report.WebsiteID)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range seiten {
		if d.SnippetID != 0 {
			t.Errorf("ein Textbausteinfeld steht in der Seitenliste: %+v", d)
		}
	}
}

// Ein Manifest von vor dieser Phase: der Textbaustein trägt weder Felder noch
// Werte, und er kommt an, wie er immer angekommen ist.
//
// Das ist SNIP-05 für das Archiv. Beide Schlüssel tragen omitempty, also ist
// „kein Schlüssel“ genau das, was ein älteres Bündel schreibt — und der
// Importweg darf daraus keinen halben Textbaustein machen.
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
		t.Fatalf("%d Textbausteine gezählt, wollte 1", report.Snippets)
	}

	kopien, err := s.Snippets.List(ctx, report.WebsiteID)
	if err != nil || len(kopien) != 1 {
		t.Fatalf("List snippets: %v (%d)", err, len(kopien))
	}
	if kopien[0].ContentMarkdown != "Telefon 07721 123456" {
		t.Errorf("der Rumpf kam anders an: %q", kopien[0].ContentMarkdown)
	}
	if kopien[0].ContentHTML == "" {
		t.Error("der Rumpf wurde nicht gerendert")
	}
	if kopien[0].Fields != "" {
		t.Errorf("der Textbaustein trägt Werte, obwohl das Manifest keine nennt: %q", kopien[0].Fields)
	}
	defs, err := s.Fields.OfSnippet(ctx, report.WebsiteID, kopien[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 0 {
		t.Errorf("der Textbaustein bekam Definitionen aus dem Nichts: %+v", defs)
	}

	// Und die Gegenprobe an der Schreibweise: ein Textbaustein ohne Felder
	// schreibt ein Manifest, das die zwei neuen Schlüssel nicht nennt.
	roh, err := json.Marshal(Snippet{Key: "k", Name: "n", Markdown: "m"})
	if err != nil {
		t.Fatal(err)
	}
	for _, schluessel := range []string{`"fields"`, `"values"`, `"value_groups"`} {
		if strings.Contains(string(roh), schluessel) {
			t.Errorf("das Manifest nennt %s, obwohl nichts gesetzt war: %s", schluessel, roh)
		}
	}
}

// Ein Manifest ist eine Datei, die jemand geschrieben hat: eine Definition,
// die validate ablehnt, kostet ihr Feld und nicht den Import, und ein Wert
// unter einer Kennung, die es nicht gibt, wird von field.Clean weggenommen
// statt gespeichert.
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
				// Eine Feldart, die es nicht gibt: validate weist sie ab.
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
		t.Fatalf("%d Textbausteine gezählt, wollte 1", report.Snippets)
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
		t.Errorf("der gültige Wert kam nicht an: %q", werte["telefon"])
	}
	for _, kennung := range []string{"kaputt", "erfunden"} {
		if _, ok := werte[kennung]; ok {
			t.Errorf("%q wurde gespeichert, obwohl keine Definition ihn trägt: %+v", kennung, werte)
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
	// Der Bericht sagt, was fehlt — sonst müsste der Betreiber die Lücke
	// selbst finden.
	if !warned(report, "Farbe") {
		t.Errorf("der Bericht nennt das verworfene Feld nicht: %v", report.Warnings)
	}
}

// Ein Manifest, das Werte mitbringt und keine Definitionen, ist die eine Form,
// die vollständig von Hand geschrieben ist.
//
// Beide Wächter — field.Clean und field.CheckAll — hingen an „if len(defs) > 0".
// Ein Archiv ohne fields kam damit an keinem von beiden vorbei und ging
// unverändert in die Spalte: der eine Manifestzuschnitt, für den es keinen
// Bildschirm und kein Formular gibt, war zugleich der einzige, der ohne Prüfung
// gespeichert wurde. Der Doppelkommentar über cleanSnippetValues sagt dabei das
// Gegenteil — er nennt das Loch, das 07-04 auf der Seite geschlossen hat, und
// genau dieses stand hier offen.
//
// Ohne Definition ist ein Wert von nichts darstellbar; er gehört weggeworfen,
// und beide Träger — die Seite und der Textbaustein — müssen sich darin gleich
// verhalten, weil es dieselbe Entscheidung ist.
func TestWerteOhneDefinitionWerdenAufBeidenTraegernVerworfen(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	zuLang := strings.Repeat("x", field.MaxValueBytes+1)
	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Ohne Definitionen"},
		// Kein Fields am Manifest und keines am Textbaustein — nur Werte.
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
		t.Errorf("Seite: ein Wert unter keiner Definition wurde abgelegt (%d Byte)",
			len(seitenwerte.Values["erfunden"]))
	}
	if len(seitenwerte.Rows) != 0 {
		t.Errorf("Seite: Gruppenzeilen unter keiner Definition wurden abgelegt: %+v", seitenwerte.Rows)
	}

	kopien, err := s.Snippets.List(ctx, report.WebsiteID)
	if err != nil || len(kopien) != 1 {
		t.Fatalf("List snippets: %v (%d)", err, len(kopien))
	}
	bausteinwerte := field.Decode(kopien[0].Fields)
	if _, ok := bausteinwerte.Values["erfunden"]; ok {
		t.Errorf("Textbaustein: ein Wert unter keiner Definition wurde abgelegt (%d Byte)",
			len(bausteinwerte.Values["erfunden"]))
	}
	if len(bausteinwerte.Rows) != 0 {
		t.Errorf("Textbaustein: Gruppenzeilen unter keiner Definition wurden abgelegt: %+v",
			bausteinwerte.Rows)
	}

	// Und die Spalte bleibt klein. Gemessen wird das Rohe und nicht nur die
	// Karte: ein Wert kann durch Decode fallen und trotzdem in der Datenbank
	// stehen.
	if n := len(kopien[0].Fields); n > 64 {
		t.Errorf("die fields-Spalte des Textbausteins hält %d Byte — ein von Hand "+
			"geschriebenes Manifest darf nicht ungeprüft in die Spalte laufen", n)
	}
}

// snippet.Store.Create endet auf s.Get(ctx, id), und Get gibt (nil, nil)
// heraus, wenn die Zeile nicht dasteht (internal/snippet/store.go:92-94). Get
// liest dabei durch s.DB.Read — einen anderen Pool als den, der geschrieben
// hat.
//
// Vor 08-05 wurde der Rückgabewert weggeworfen („if _, err := …"). Seit der
// Import die Felder des Textbausteins nachzieht, wird created.ID gelesen, und
// (nil, nil) ist damit kein leeres Ergebnis mehr, sondern ein Absturz, der die
// ganze Anfrage mitnimmt. internal/admin/snippet.go:307-310 wacht über
// denselben Wert — die zwei Aufrufstellen waren sich uneins darüber, ob er
// nil sein kann.
//
// Nachgestellt wird die Lage über den Lesepool und nicht über eine Attrappe:
// der Schreibpool zeigt auf die echte Datenbank, der Lesepool auf eine zweite,
// leere. Genau das, was Get sieht, wenn seine Zeile nicht dasteht.
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
		t.Errorf("der Bericht schweigt über den Textbaustein, der nicht ankam: %v", report.Warnings)
	}
}

// Ein Bildfeld an einem Textbaustein überlebt die Archivreise nicht, und der
// Bericht sagt es jetzt.
//
// Der Wert eines Bild-, Verweis- oder Schlagwortfeldes ist eine Nummer *dieser*
// Anlage. Auf dem Seitenweg werden solche Nummern beim Ausfahren in einen
// Dateinamen und eine Adresse übersetzt und beim Einfahren zurück
// (exportFieldValues/translateIn); die Werte eines Textbausteins gehen roh
// hinaus und roh hinein. Drüben gehört die Nummer einer anderen Website, und
// fieldImages/fieldRefs weisen sie zurück — das Feld kommt an, das Bild nicht.
//
// Das bleibt vorerst so: die Übersetzung sitzt im Seitenweg und sie von dort zu
// lösen ist eine eigene Arbeit (deferred-items.md). Was sich hier ändert, ist
// die Lautstärke. Der Verwaltungsbildschirm bietet diese Feldarten ausdrücklich
// an und field_list.html verspricht sie dem Betreiber; ein Versprechen, das beim
// Ausfahren stillschweigend gebrochen wird, ist der lautlose Datenverlust, den
// dieses Projekt sonst überall vermeidet.
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

	// Und der Rest kommt heil an: die Meldung ersetzt keinen Wert und wirft
	// keinen weg.
	kopien, err := s.Snippets.List(ctx, report.WebsiteID)
	if err != nil || len(kopien) != 1 {
		t.Fatalf("List snippets: %v (%d)", err, len(kopien))
	}
	werte := field.Decode(kopien[0].Fields).Values
	if werte["telefon"] != "07721 123456" {
		t.Errorf("der gültige Wert kam nicht an: %q", werte["telefon"])
	}
}
