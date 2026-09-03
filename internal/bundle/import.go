// Package bundle exports and imports a whole website as one archive.
//
// It stays in the core. Doing this as a plugin would need write access to
// pages, media, labels, menus, snippets and settings all at once — which is the
// whole database, and a sandbox that hands over the whole database is a sandbox
// in name only. It is also a maintenance tool for whoever runs the server, not
// a feature of the website: nothing a visitor ever meets.
package bundle

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/design"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
)

// Report says what an import did, in the words the operator needs.
//
// An import that only says "done" leaves someone to check two hundred pages by
// hand to find out what did not arrive.
type Report struct {
	WebsiteID int64
	Pages     int
	Media     int
	Menus     int
	Snippets  int
	Terms     int
	// Warnings are the things that did not come through. Each one names what
	// was lost and why; none of them stops the import.
	Warnings []string
}

// MaxMediaBytes bounds one file inside an archive.
const MaxMediaBytes = 64 << 20

// MaxManifestBytes bounds the manifest.
//
// A zip entry can claim any uncompressed size, so every read is limited: a
// hundred-kilobyte archive that expands to four gigabytes is the oldest trick
// there is against a machine with half a gigabyte of memory.
const MaxManifestBytes = 32 << 20

// Import reads an archive and creates a website from it.
//
// It always creates rather than merges. Merging would need an answer for every
// collision — same slug, different text — and the honest answer for a CMS this
// size is a second website the operator can compare and then delete.
func Import(ctx context.Context, s Stores, r io.ReaderAt, size int64, name string) (*Report, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("die Datei ist kein gültiges Archiv: %w", err)
	}

	manifest, err := readManifest(zr)
	if err != nil {
		return nil, err
	}
	if manifest.Version > Version {
		return nil, fmt.Errorf(
			"das Archiv wurde mit einer neueren Fassung erstellt (Format %d, diese Fassung kennt %d)",
			manifest.Version, Version)
	}

	siteName := strings.TrimSpace(name)
	if siteName == "" {
		siteName = manifest.Site.Name
	}
	if siteName == "" {
		return nil, fmt.Errorf("das Archiv nennt keinen Namen für die Website")
	}

	created, err := s.Domains.CreateWebsite(ctx, siteName, manifest.Site.Description)
	if err != nil {
		return nil, fmt.Errorf("Website anlegen: %w", err)
	}
	websiteID := created.ID
	report := &Report{WebsiteID: websiteID}

	if err := applySettings(ctx, s, websiteID, manifest.Site); err != nil {
		report.Warnings = append(report.Warnings, "Einstellungen: "+err.Error())
	}
	mediaByName := importMedia(ctx, s, websiteID, zr, manifest, report)
	// The field definitions go in before the pages: a value without its
	// definition would be dropped by the first save of that page.
	// Die Inhaltsarten zuerst: ein Eintrag, dessen Art es noch nicht gibt,
	// stünde in der Liste unter einer Kennung ohne Namen.
	importTypes(ctx, s, websiteID, manifest, report)
	fieldKinds := importFields(ctx, s, websiteID, manifest, report)
	importBlockTypes(ctx, s, websiteID, manifest, report)
	// The block kinds have to exist before the pages: a block of a kind this
	// website does not have is dropped on the way in, and a page of recipe
	// steps would arrive empty.
	set := block.Builtin
	if s.BlockTypes != nil {
		set = s.BlockTypes.Set(ctx, websiteID)
	}
	set.Date = func(t time.Time) string {
		return tmpl.DateText(manifest.Site.Locale, manifest.Site.TimeZone, t)
	}
	importPages(ctx, s, websiteID, manifest, mediaByName, fieldKinds, set, report)
	importSnippets(ctx, s, websiteID, manifest, report)
	importMenus(ctx, s, websiteID, manifest, report)

	return report, nil
}

func readManifest(zr *zip.Reader) (*Manifest, error) {
	f, err := zr.Open(ManifestName)
	if err != nil {
		return nil, fmt.Errorf("im Archiv fehlt %s", ManifestName)
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, MaxManifestBytes))
	if err != nil {
		return nil, fmt.Errorf("%s konnte nicht gelesen werden: %w", ManifestName, err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s ist beschädigt: %w", ManifestName, err)
	}
	return &m, nil
}

func applySettings(ctx context.Context, s Stores, websiteID int64, site Site) error {
	set := domain.Settings{
		Locale: site.Locale, ExtraLocales: site.ExtraLocales, TimeZone: site.TimeZone,
		MetaDescription: site.MetaDescription,
		OfflineMode:     "notfound",
		BlogBase:        site.BlogBase, PostsPerPage: site.PostsPerPage,
		ContactEmail: site.ContactEmail,
		OrgType:      site.OrgType, Street: site.Street, PostalCode: site.PostalCode,
		City: site.City, Country: site.Country, Phone: site.Phone,
		OpeningHours: site.OpeningHours,
	}
	if err := s.Domains.UpdateSettings(ctx, websiteID, set); err != nil {
		return err
	}

	// Through the same validator as the form: a bundle is a file anyone can
	// edit, so it is exactly as untrusted as a text field.
	tokens := design.Sanitize(design.Tokens{
		Ink: site.Design.Ink, Paper: site.Design.Paper, Brand: site.Design.Brand,
		Font: site.Design.Font, Measure: site.Design.Measure, Radius: site.Design.Radius,
	})
	return s.Domains.UpdateDesignTokens(ctx, websiteID, domain.DesignTokens{
		Ink: tokens.Ink, Paper: tokens.Paper, Brand: tokens.Brand,
		Font: tokens.Font, Measure: tokens.Measure, Radius: tokens.Radius,
	})
}

// importMedia writes the files and returns the new id of each by file name.
func importMedia(ctx context.Context, s Stores, websiteID int64, zr *zip.Reader,
	m *Manifest, report *Report) map[string]int64 {

	byName := map[string]int64{}
	if s.Media == nil || len(m.Media) == 0 {
		return byName
	}

	dir := filepath.Join(s.DataDir, "media", strconv.FormatInt(websiteID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		report.Warnings = append(report.Warnings, "Medienordner: "+err.Error())
		return byName
	}

	for _, entry := range m.Media {
		name := safeName(entry.Filename)
		if name == "" {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Datei %q hat einen unzulässigen Namen und wurde übersprungen", entry.Filename))
			continue
		}

		data, err := readEntry(zr, MediaDir+name)
		if err != nil {
			// Absent and damaged are different problems with different answers:
			// one means the archive was assembled wrong, the other that it did
			// not survive the journey.
			why := "fehlt im Archiv"
			if !errors.Is(err, fs.ErrNotExist) {
				why = "ist beschädigt (Prüfsumme des Archivs stimmt nicht)"
			}
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Datei %q %s", entry.Filename, why))
			continue
		}
		// The checksum is the difference between storing a corrupted file and
		// finding out when a page renders a grey box.
		if entry.SHA256 != "" && hashBytes(data) != entry.SHA256 {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Datei %q ist beschädigt und wurde übersprungen", entry.Filename))
			continue
		}

		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Datei %q konnte nicht gespeichert werden: %v", entry.Filename, err))
			continue
		}

		created, err := s.Media.Create(ctx, websiteID, name, entry.OriginalName,
			entry.MimeType, int64(len(data)), hashBytes(data))
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Datei %q konnte nicht eingetragen werden: %v", entry.Filename, err))
			continue
		}
		if entry.AltText != "" || entry.Caption != "" {
			_ = s.Media.UpdateMeta(ctx, created.ID, entry.AltText, entry.Caption)
		}
		byName[entry.Filename] = created.ID
		report.Media++
	}
	return byName
}

// readEntry reads one archive member under a hard size limit.
func readEntry(zr *zip.Reader, name string) ([]byte, error) {
	f, err := zr.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, MaxMediaBytes))
}

// safeName rejects anything that is not a plain file name.
//
// Zip slip: an entry called "../../etc/holzcloud.conf" is how an archive writes
// outside the directory it was unpacked into. The template upload already
// guards against this; an import is the same hazard through a different door.
func safeName(name string) string {
	if name == "" || name != path.Base(name) || name == "." || name == ".." {
		return ""
	}
	if strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return ""
	}
	return name
}

// importTypes recreates the website's own content kinds.
func importTypes(ctx context.Context, s Stores, websiteID int64, m *Manifest, report *Report) {
	if s.Kinds == nil {
		if len(m.Types) > 0 {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("%d Inhaltsarten konnten nicht angelegt werden.", len(m.Types)))
		}
		return
	}
	for _, t := range m.Types {
		if _, err := s.Kinds.Create(ctx, kind.Type{
			WebsiteID: websiteID, Key: t.Key, Name: t.Name, Plural: t.Plural,
			Archive: t.Archive, Sort: t.Sort,
		}); err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Inhaltsart %q: %v", t.Name, err))
		}
	}
}

// importFields recreates the website's own field definitions and returns which
// of them are pictures, so the values can be translated back to ids.
func importFields(ctx context.Context, s Stores, websiteID int64, m *Manifest, report *Report) map[string]string {
	kinds := map[string]string{}
	if s.Fields == nil {
		if len(m.Fields) > 0 {
			report.Warnings = append(report.Warnings,
				"Die eigenen Felder der Website konnten nicht angelegt werden.")
		}
		return kinds
	}
	// The conditions are hung on afterwards. A field may hang on one that comes
	// later in the file, and a condition pointing at a field that does not exist
	// yet is refused — which would cost the whole field, not just its condition.
	conditions := map[int64]field.Def{}

	for _, f := range m.Fields {
		kinds[f.Key] = f.Kind
		def := field.Def{
			WebsiteID: websiteID, Key: f.Key, Label: f.Label, Kind: f.Kind,
			Required: f.Required, Hint: f.Hint, Choices: f.Choices, AppliesTo: f.AppliesTo,
		}
		created, err := s.Fields.Create(ctx, def)
		if err == nil && f.Condition != "" {
			def.Condition = f.Condition
			conditions[created.ID] = def
		}
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Feld %q konnte nicht angelegt werden: %v", f.Label, err))
			continue
		}
		for _, sub := range f.Sub {
			kinds[f.Key+"."+sub.Key] = sub.Kind
			if _, err := s.Fields.Create(ctx, field.Def{
				WebsiteID: websiteID, ParentID: created.ID, Key: sub.Key, Label: sub.Label,
				Kind: sub.Kind, Required: sub.Required, Hint: sub.Hint, Choices: sub.Choices,
			}); err != nil {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("Feld %q in der Gruppe %q konnte nicht angelegt werden: %v", sub.Label, f.Label, err))
			}
		}
	}

	for id, def := range conditions {
		if err := s.Fields.Update(ctx, websiteID, id, def); err != nil {
			// The field is there and works; only the condition is missing, so it
			// is always shown. Worth a line, not worth failing the import.
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Die Bedingung von Feld %q konnte nicht übernommen werden: %v", def.Label, err))
		}
	}
	return kinds
}

// importBlockTypes recreates the website's own block kinds and their fields.
//
// The blocks on the pages do not travel (see the note on bundle.Page), so this
// is the definition alone: after an import the kinds are in the editor's menu
// and ready to be used, even though the pages arrive as text.
func importBlockTypes(ctx context.Context, s Stores, websiteID int64, m *Manifest, report *Report) {
	if s.BlockTypes == nil || len(m.BlockTypes) == 0 {
		if len(m.BlockTypes) > 0 {
			report.Warnings = append(report.Warnings,
				"Die eigenen Bausteinarten der Website konnten nicht angelegt werden.")
		}
		return
	}
	for _, t := range m.BlockTypes {
		created, err := s.BlockTypes.Create(ctx, websiteID, t.Name, t.Hint)
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Bausteinart %q konnte nicht angelegt werden: %v", t.Name, err))
			continue
		}
		for _, f := range t.Fields {
			if _, err := s.Fields.Create(ctx, field.Def{
				WebsiteID: websiteID, BlockTypeID: created.ID,
				Key: f.Key, Label: f.Label, Kind: f.Kind,
				Hint: f.Hint, Choices: f.Choices,
			}); err != nil {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("Feld %q der Bausteinart %q konnte nicht angelegt werden: %v",
						f.Label, t.Name, err))
			}
		}
	}
}

// importFieldValues turns a bundle's values back into what is stored: a
// picture's file name becomes the id it has on this machine, and a reference's
// address the id of the page that now carries it.
func importFieldValues(kinds map[string]string, p Page, mediaByName, idBySlug map[string]int64) string {
	data := field.Data{
		Values: translateIn(kinds, p.Fields, mediaByName, idBySlug),
		Rows:   map[string][]field.Values{},
	}
	for key, rows := range p.FieldGroups {
		sub := subKinds(kinds, key)
		out := make([]field.Values, 0, len(rows))
		for _, row := range rows {
			if translated := translateIn(sub, row, mediaByName, idBySlug); len(translated) > 0 {
				out = append(out, translated)
			}
		}
		if len(out) > 0 {
			data.Rows[key] = out
		}
	}
	raw, err := field.Encode(data)
	if err != nil {
		return ""
	}
	return raw
}

// translateIn turns a bundle's values back into what is stored: a picture's
// file name becomes the id it has on this machine.
func translateIn(kinds map[string]string, values map[string]string, mediaByName, idBySlug map[string]int64) field.Values {
	if len(values) == 0 {
		return nil
	}
	out := field.Values{}
	for key, val := range values {
		if kinds[key] == field.KindRef {
			// A page that has not been created yet is not an error here: the
			// caller runs this a second time when every page exists. What is
			// still missing then was never in the bundle.
			id, ok := idBySlug[page.Slugify(val)]
			if !ok {
				continue
			}
			out[key] = strconv.FormatInt(id, 10)
			continue
		}
		if kinds[key] == field.KindImage {
			id, ok := mediaByName[val]
			if !ok {
				continue
			}
			out[key] = strconv.FormatInt(id, 10)
			continue
		}
		out[key] = val
	}
	return out
}

func importPages(ctx context.Context, s Stores, websiteID int64, m *Manifest,
	mediaByName map[string]int64, fieldKinds map[string]string, set block.Set, report *Report) {

	// One lookup for the whole import, so a picture used on twenty pages is
	// read once.
	look := blockImages(ctx, s, websiteID)

	// The languages the target website has. A page filed under a language the
	// site does not serve would be unreachable, so it arrives in the main
	// language and the operator is told.
	var extras []string
	if ws, err := s.Domains.GetWebsite(ctx, websiteID); err == nil && ws != nil {
		extras = ws.Locales()
	}
	// The translation links are addresses; they can only be resolved once every
	// page of the bundle exists, so they are collected and applied at the end.
	idBySlug := map[string]int64{}
	type link struct {
		id     int64
		loc    string
		ofSlug string
	}
	var links []link
	// Pages carrying a reference, for the second pass below.
	type refPage struct {
		id   int64
		page Page
	}
	var refs []refPage

	for _, p := range m.Pages {
		slug := page.Slugify(p.Slug)
		if slug == "" {
			slug = page.Slugify(p.Title)
		}
		if err := page.ValidateSlug(slug); err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Seite %q: Adresse %q ist nicht zulässig", p.Title, p.Slug))
			continue
		}

		markdown := rewriteMediaPaths(p.Markdown, websiteID, m.Media)

		// Re-rendered here rather than carried in the archive: a bundle written
		// by an older renderer would otherwise import HTML that no longer
		// matches what this version produces.
		html, err := page.RenderMarkdown(markdown)
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Seite %q konnte nicht gesetzt werden: %v", p.Title, err))
			continue
		}

		// A page built with the block editor: the blocks are what it is made of,
		// and the markdown column holds the plain text derived from them — which
		// is what the search index and the excerpt read.
		encodedBlocks := ""
		if len(p.Blocks) > 0 {
			blocks := set.Clean(importBlocks(p.Blocks, set, mediaByName))
			if len(blocks) > 0 {
				encoded, eerr := block.Encode(blocks, set)
				if eerr != nil {
					report.Warnings = append(report.Warnings,
						fmt.Sprintf("Seite %q: die Bausteine konnten nicht gesichert werden: %v", p.Title, eerr))
				} else {
					encodedBlocks = encoded
					markdown = block.PlainText(blocks, set)
					html = block.Render(blocks, set, look, page.RenderMarkdown)
				}
			}
			for _, name := range missingMedia(p.Blocks, mediaByName) {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("Seite %q: die Datei %q fehlt, der Baustein bleibt ohne Bild", p.Title, name))
			}
		}

		meta := page.PageMeta{
			Excerpt: p.Excerpt, MetaDescription: p.MetaDescription, NoIndex: p.NoIndex,
		}
		if p.FeaturedImage != "" {
			if id, ok := mediaByName[p.FeaturedImage]; ok {
				meta.FeaturedMediaID = &id
			}
		}

		created, err := s.Pages.CreatePage(ctx, page.PageCreate{
			WebsiteID: websiteID, Title: p.Title, Slug: slug,
			Markdown: markdown, HTML: html, Blocks: encodedBlocks, Status: p.Status,
			Fields: importFieldValues(fieldKinds, p, mediaByName, idBySlug),
			Meta:   meta, Kind: p.Kind, TypeKey: p.TypeKey,
			Schedule: page.PageSchedule{PublishAt: p.PublishAt, UnpublishAt: p.UnpublishAt},
		})
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Seite %q konnte nicht angelegt werden: %v", p.Title, err))
			continue
		}
		report.Pages++
		idBySlug[slug] = created.ID
		if hasRef(fieldKinds, p) {
			refs = append(refs, refPage{id: created.ID, page: p})
		}

		if p.Locale != "" {
			loc := locale.Pick(p.Locale, extras)
			if loc == "" {
				report.Warnings = append(report.Warnings, fmt.Sprintf(
					"Seite %q ist in der Sprache %q verfasst, die diese Website nicht hat – "+
						"sie liegt jetzt in der Hauptsprache.", p.Title, p.Locale))
			} else {
				links = append(links, link{id: created.ID, loc: loc, ofSlug: page.Slugify(p.TranslationOf)})
			}
		}

		if len(p.Terms) > 0 && s.Terms != nil {
			if err := s.Terms.SetForPage(ctx, websiteID, created.ID, term.Parse(strings.Join(p.Terms, ", "))); err != nil {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("Schlagwörter von %q: %v", p.Title, err))
			}
		}
		// The password never travels, so a protected page arrives unprotected
		// and the operator has to be told rather than left to discover it.
		if p.Protected() {
			report.Warnings = append(report.Warnings, fmt.Sprintf(
				"Seite %q war mit einem Passwort geschützt. Passwörter werden nie exportiert – "+
					"bitte ein neues vergeben, die Seite ist bis dahin öffentlich.", p.Title))
		}
	}

	for _, l := range links {
		// A missing counterpart is not an error: the page is still in its
		// language, it just stands on its own.
		if err := s.Pages.SetTranslation(ctx, l.id, l.loc, idBySlug[l.ofSlug]); err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Sprache konnte nicht gesetzt werden: %v", err))
		}
	}

	// The references, once. A page may point at one that is created after it,
	// so the first pass could only resolve what happened to exist already; now
	// every address in the bundle has an id. Only the pages that actually carry
	// a reference are written again.
	for _, rp := range refs {
		raw := importFieldValues(fieldKinds, rp.page, mediaByName, idBySlug)
		if err := s.Pages.SetFields(ctx, rp.id, raw); err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Verweise von %q konnten nicht gesetzt werden: %v", rp.page.Title, err))
		}
	}

	if s.Terms != nil {
		report.Terms = len(m.Terms)
	}
}

func importSnippets(ctx context.Context, s Stores, websiteID int64, m *Manifest, report *Report) {
	if s.Snippets == nil {
		return
	}
	for _, sn := range m.Snippets {
		markdown := rewriteMediaPaths(sn.Markdown, websiteID, m.Media)

		// The same renderer as a page: a snippet is Markdown too, and rendering
		// it any other way would make the same text look different in the two
		// places it can appear.
		html, err := page.RenderMarkdown(markdown)
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Textbaustein %q: %v", sn.Key, err))
			continue
		}
		if _, err := s.Snippets.Create(ctx, websiteID, sn.Key, sn.Name, markdown, html); err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Textbaustein %q konnte nicht angelegt werden: %v", sn.Key, err))
			continue
		}
		report.Snippets++
	}
}

// mediaPathPattern matches the /media/<website id>/<file name> links that a
// page uses to point at its own pictures.
//
// The file name stops at the first character that cannot be part of one, which
// is what keeps the match inside the link and out of the sentence around it:
// the closing bracket of `![Alt](/media/3/hof.jpg)`, the quote of an `<img
// src="…">`, or simply the next space.
var mediaPathPattern = regexp.MustCompile(`/media/[0-9]+/([^)"'\s<>]+)`)

// rewriteMediaPaths points a page's picture links at the website this import
// just created.
//
// Without this an import is quietly broken in the one way nobody checks for.
// The archive carries the Markdown as written, and that Markdown says
// /media/3/hof.jpg because the site it came from was website 3. An import
// always creates a new website, which will be number 7 or 12 — so every
// picture on every page resolves to a file belonging to a different site, or
// to nothing at all. The pages import, the report says so, the media count is
// right, and the site is full of broken images.
//
// Only names the archive actually carries are rewritten. A link to a file this
// bundle does not contain is left exactly as it was: it is either a mistake
// that the operator needs to see, or a deliberate link to something else on the
// machine that this import has no business redirecting.
//
// It also gives a hand-written bundle a way in. There is no id to write down
// before the import — the website does not exist yet — so an author writes
// /media/0/hof.jpg and gets the right number here.
func rewriteMediaPaths(markdown string, websiteID int64, media []Media) string {
	if markdown == "" || len(media) == 0 {
		return markdown
	}
	known := make(map[string]bool, len(media))
	for _, m := range media {
		known[m.Filename] = true
	}
	prefix := "/media/" + strconv.FormatInt(websiteID, 10) + "/"

	return mediaPathPattern.ReplaceAllStringFunc(markdown, func(match string) string {
		name := mediaPathPattern.FindStringSubmatch(match)[1]
		if !known[name] {
			return match
		}
		return prefix + name
	})
}

func importMenus(ctx context.Context, s Stores, websiteID int64, m *Manifest, report *Report) {
	if s.Menus == nil {
		return
	}
	// Menus come last: their items point at pages by slug, and the pages have
	// to exist before the link can be resolved.
	var extras []string
	if ws, err := s.Domains.GetWebsite(ctx, websiteID); err == nil && ws != nil {
		extras = ws.Locales()
	}
	for _, mn := range m.Menus {
		// Same rule as for a page: a menu in a language the site does not have
		// would never be rendered anywhere.
		created, err := s.Menus.CreateMenu(ctx, websiteID, mn.Name, mn.LocationKey, locale.Pick(mn.Locale, extras))
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Menü %q konnte nicht angelegt werden: %v", mn.Name, err))
			continue
		}
		importItems(ctx, s, websiteID, created.ID, nil, mn.Items, report)
		report.Menus++
	}
}

func importItems(ctx context.Context, s Stores, websiteID, menuID int64,
	parent *int64, items []MenuItem, report *Report) {

	for i, item := range items {
		var pageID *int64
		if item.PageSlug != "" {
			if pg, err := s.Pages.GetPageBySlug(ctx, websiteID, item.PageSlug); err == nil && pg != nil {
				pageID = &pg.ID
			} else {
				report.Warnings = append(report.Warnings, fmt.Sprintf(
					"Menüpunkt %q zeigt auf die Seite %q, die es nicht gibt", item.Title, item.PageSlug))
			}
		}
		created, err := s.Menus.CreateItem(ctx, menuID, parent, item.Title, item.Type, item.URL, pageID, i)
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Menüpunkt %q: %v", item.Title, err))
			continue
		}
		if len(item.Children) > 0 {
			id := created.ID
			importItems(ctx, s, websiteID, menuID, &id, item.Children, report)
		}
	}
}

// hasRef reports whether a page of a bundle carries a reference field, so only
// those are written a second time.
func hasRef(kinds map[string]string, p Page) bool {
	for key := range p.Fields {
		if kinds[key] == field.KindRef {
			return true
		}
	}
	for group, rows := range p.FieldGroups {
		sub := subKinds(kinds, group)
		for _, row := range rows {
			for key := range row {
				if sub[key] == field.KindRef {
					return true
				}
			}
		}
	}
	return false
}
