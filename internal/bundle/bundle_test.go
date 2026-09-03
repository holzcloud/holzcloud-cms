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
	if len(angekommen) != 3 {
		t.Fatalf("%d Bausteine statt 3: %+v", len(angekommen), angekommen)
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
