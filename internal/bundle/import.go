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
	"sort"
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
	// Die Schlagwörter vor den Seiten: eines, das nur ein Schlagwortfeld
	// nennt, gibt es sonst nirgends — SetForPage legt nur an, was in der
	// Schlagwortliste einer Seite steht.
	importTerms(ctx, s, websiteID, manifest, report)
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

// importTerms creates every label the manifest declares.
//
// Until now a label was only created as a side effect of a page carrying it, so
// one reachable solely through a label field arrived nowhere — counted on the
// way in and not created, exactly as format.go:274-284 says. Every name goes
// through term.Normalize first, the same normalisation every other label gets,
// so whitespace and an over-long name are spelled identically whichever path
// creates the label.
//
// A store error is a warning naming the labels and never a failed import: an
// import that stops halfway is worse than one that says what is missing.
func importTerms(ctx context.Context, s Stores, websiteID int64, m *Manifest, report *Report) {
	if len(m.Terms) == 0 {
		return
	}
	if s.Terms == nil {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("%d Schlagwörter konnten nicht angelegt werden.", len(m.Terms)))
		return
	}
	// term.Normalize und nicht term.Parse: Parse ist der Leser des Feldes,
	// das ein Redaktor tippt. Es trennt an Kommas und hört bei
	// term.MaxPerPage auf — beides gilt für einen Eintrag und für nichts
	// sonst. Die ganze Liste einer Website durch diesen Leser zu schicken
	// liess ab dem dreizehnten Schlagwort alles fallen, still, und riss
	// einen Namen mit einem Komma darin entzwei. Jeder Name für sich, mit
	// derselben Normalisierung, die jedes andere Schlagwort auch bekommt.
	names := make([]string, 0, len(m.Terms))
	for _, t := range m.Terms {
		if name := term.Normalize(t.Name); name != "" {
			names = append(names, name)
		}
	}
	n, err := s.Terms.EnsureNames(ctx, websiteID, names)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Schlagwörter: %v", err))
		return
	}
	// Was angelegt wurde, nicht was das Archiv behauptet: eine Zahl, die ein
	// Bericht nennt, soll geglaubt werden können.
	report.Terms = n
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
			Display: f.Display, MaxValues: f.MaxValues,
			RangeMin: f.Min, RangeMax: f.Max,
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
				Display: sub.Display, MaxValues: sub.MaxValues,
				RangeMin: sub.Min, RangeMax: sub.Max,
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
				Display: f.Display, MaxValues: f.MaxValues,
				RangeMin: f.Min, RangeMax: f.Max,
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
// addressLookup finds a page of this bundle by its address, in one language.
type addressLookup func(slug string) (int64, bool)

// pageIndex remembers which id each address got, language by language.
//
// An address is unique within a language and not across the website: a product
// name is not translated, so /holzcloud-cms exists five times over and is five
// pages. A single map keyed by the address alone would keep only the language
// imported last, and every link resolved through it would point into that one.
type pageIndex map[string]int64

func addressKey(loc, slug string) string { return loc + "\x00" + slug }

func (x pageIndex) add(loc, slug string, id int64) { x[addressKey(loc, slug)] = id }

// at finds the page with this address in exactly this language.
func (x pageIndex) at(loc, slug string) (int64, bool) {
	id, ok := x[addressKey(loc, slug)]
	return id, ok
}

// in is the lookup for one language. What it does not find there it looks for
// in the main language: a translated page may well point at something that
// exists only once, an imprint for instance, and that link should land.
func (x pageIndex) in(loc string) addressLookup {
	return func(slug string) (int64, bool) {
		if id, ok := x.at(loc, slug); ok {
			return id, true
		}
		if loc == "" {
			return 0, false
		}
		return x.at("", slug)
	}
}

// importFieldValues turns a bundle's field values into what is stored, and
// returns the keys it had to drop on the way.
//
// The two guards are the reason defs is passed beside kinds. A manifest is a
// file somebody uploaded: nothing about it was typed into this program's
// forms, so nothing about it has been through the checks every other write
// path applies. field.Clean drops values for fields this website does not have
// and bounds a group's rows; field.CheckAll is where the byte budget and every
// per-kind rule live since 07-04 stopped trimTo from silently truncating. Both
// are what internal/admin/page.go and internal/ai/tools.go already do before
// they write, and this path was the one hole left (STATE.md blocker, 07-04).
//
// A rejected value is dropped and named, never a failed import: an archive
// with one over-long value in it is still worth having, and the operator has
// to be told which value did not arrive rather than left to find the gap.
//
// Beide laufen ohne Vorbedingung, und das ist eine Berichtigung: sie hingen an
// „if len(defs) > 0". Ein Manifest, das Werte mitbringt und keine
// Definitionen, kam damit an keinem der beiden vorbei — der eine Zuschnitt,
// für den es kein Formular und keinen Bildschirm gibt, war zugleich der
// einzige, der ungeprüft in die Spalte lief. Ohne Definition wirft Clean
// richtigerweise alles weg: ein Wert, den keine Definition trägt, ist von
// nichts darstellbar.
func importFieldValues(defs []field.Def, kinds map[string]string, p Page,
	mediaByName map[string]int64, byAddress addressLookup) (string, []string) {

	data := field.Data{
		Values: translateIn(kinds, p.Fields, mediaByName, byAddress),
		Rows:   map[string][]field.Values{},
	}
	for key, rows := range p.FieldGroups {
		sub := subKinds(kinds, key)
		out := make([]field.Values, 0, len(rows))
		for _, row := range rows {
			if translated := translateIn(sub, row, mediaByName, byAddress); len(translated) > 0 {
				out = append(out, translated)
			}
		}
		if len(out) > 0 {
			data.Rows[key] = out
		}
	}

	var dropped []string
	data = field.Clean(defs, data)
	for key := range field.CheckAll(defs, data) {
		// Ein Pflichtfeld ohne Wert wird hier auch gemeldet, und dort ist
		// nichts wegzunehmen — die Seite kommt eben ohne an, so wie sie
		// abgereist ist. Weggenommen wird nur, was tatsächlich dasteht.
		if _, ok := data.Values[key]; ok {
			delete(data.Values, key)
			dropped = append(dropped, key)
			continue
		}
		if group, i, sub, ok := splitRowKey(key); ok {
			if rows := data.Rows[group]; i < len(rows) {
				if _, ok := rows[i][sub]; ok {
					delete(rows[i], sub)
					dropped = append(dropped, key)
				}
			}
		}
	}
	sort.Strings(dropped)

	raw, err := field.Encode(data)
	if err != nil {
		return "", dropped
	}
	return raw, dropped
}

// splitRowKey reads back what field.RowKey wrote: "gruppe.3.kennung".
//
// The number is in the middle and a key may not contain a dot, so the split is
// unambiguous — cutting at the first and last dot would not be.
func splitRowKey(key string) (group string, index int, sub string, ok bool) {
	parts := strings.Split(key, ".")
	if len(parts) != 3 {
		return "", 0, "", false
	}
	i, err := strconv.Atoi(parts[1])
	if err != nil || i < 0 {
		return "", 0, "", false
	}
	return parts[0], i, parts[2], true
}

// translateIn turns a bundle's values back into what is stored: a picture's
// file name becomes the id it has on this machine.
func translateIn(kinds map[string]string, values map[string]string, mediaByName map[string]int64, byAddress addressLookup) field.Values {
	if len(values) == 0 {
		return nil
	}
	out := field.Values{}
	for key, val := range values {
		if kinds[key] == field.KindRef {
			// A page that has not been created yet is not an error here: the
			// caller runs this a second time when every page exists. What is
			// still missing then was never in the bundle.
			id, ok := byAddress(page.Slugify(val))
			if !ok {
				continue
			}
			out[key] = strconv.FormatInt(id, 10)
			continue
		}
		if kinds[key] == field.KindTerm {
			// Der Wert im Archiv ist ein Name, gespeichert wird ein Kürzel.
			// page.Slugify ist die eine Regel, die auch der Schlagwortspeicher
			// anwendet — die beiden stimmen dadurch von Bauart wegen überein
			// und nicht durch Zufall.
			//
			// Nicht nachgeschlagen und nicht fallengelassen: importTerms ist
			// schon gelaufen, und ein Wert, dessen Schlagwort das Archiv nie
			// genannt hat, löst sich beim Rendern zu nichts auf — genau das
			// leere Verhalten, das die Art zusagt.
			out[key] = page.Slugify(val)
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

	// Die Felddefinitionen, wie sie tatsächlich angelegt wurden — gelesen und
	// nicht aus dem Archiv nachgebaut, damit die Prüfung gegen das läuft, was
	// diese Website hat, samt allem, was validate beim Anlegen geleert hat.
	var defs []field.Def
	if s.Fields != nil {
		defs, _ = s.Fields.List(ctx, websiteID)
	}

	// The languages the target website has. A page filed under a language the
	// site does not serve would be unreachable, so it arrives in the main
	// language and the operator is told.
	var extras []string
	if ws, err := s.Domains.GetWebsite(ctx, websiteID); err == nil && ws != nil {
		extras = ws.Locales()
	}
	// The translation links are addresses; they can only be resolved once every
	// page of the bundle exists, so they are collected and applied at the end.
	//
	// The key carries the language, because an address is only unique within
	// one: /holzcloud-cms exists in German and in French, and they are two
	// pages. Keyed by the address alone, the last language imported would win
	// and every translation link would point into it.
	pages := pageIndex{}
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
		loc  string
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

		// The language is settled before the page is written, not after: the
		// address is unique per language, so a page created in the main
		// language and moved afterwards would already have collided with its
		// own original and been renamed to "-2".
		loc := ""
		if p.Locale != "" {
			loc = locale.Pick(p.Locale, extras)
			if loc == "" {
				report.Warnings = append(report.Warnings, fmt.Sprintf(
					"Seite %q ist in der Sprache %q verfasst, die diese Website nicht hat – "+
						"sie liegt jetzt in der Hauptsprache.", p.Title, p.Locale))
			}
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
			for _, name := range missingMedia(p.Blocks, set, mediaByName) {
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

		felder, verworfen := importFieldValues(defs, fieldKinds, p, mediaByName, pages.in(loc))
		if len(verworfen) > 0 {
			report.Warnings = append(report.Warnings, fmt.Sprintf(
				"Seite %q: die Werte von %s wurden nicht übernommen, sie halten die Regeln ihrer Felder nicht ein.",
				p.Title, strings.Join(verworfen, ", ")))
		}

		created, err := s.Pages.CreatePage(ctx, page.PageCreate{
			WebsiteID: websiteID, Title: p.Title, Slug: slug, Locale: loc,
			Markdown: markdown, HTML: html, Blocks: encodedBlocks, Status: p.Status,
			Fields: felder,
			Meta:   meta, Kind: p.Kind, TypeKey: p.TypeKey,
			Schedule: page.PageSchedule{PublishAt: p.PublishAt, UnpublishAt: p.UnpublishAt},
		})
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Seite %q konnte nicht angelegt werden: %v", p.Title, err))
			continue
		}
		report.Pages++
		pages.add(loc, created.Slug, created.ID)
		if hasRef(fieldKinds, p) {
			refs = append(refs, refPage{id: created.ID, page: p, loc: loc})
		}

		if loc != "" {
			links = append(links, link{id: created.ID, loc: loc, ofSlug: page.Slugify(p.TranslationOf)})
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
		// translation_of always names a page in the main language, so it is
		// looked up there and nowhere else.
		of, _ := pages.at("", l.ofSlug)
		if err := s.Pages.SetTranslation(ctx, l.id, l.loc, of); err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Sprache konnte nicht gesetzt werden: %v", err))
		}
	}

	// The references, once. A page may point at one that is created after it,
	// so the first pass could only resolve what happened to exist already; now
	// every address in the bundle has an id. Only the pages that actually carry
	// a reference are written again.
	for _, rp := range refs {
		// Was hier verworfen wird, wurde im ersten Durchgang schon gemeldet —
		// dieselbe Seite, dieselben Werte, dieselben Regeln.
		raw, _ := importFieldValues(defs, fieldKinds, rp.page, mediaByName, pages.in(rp.loc))
		if err := s.Pages.SetFields(ctx, rp.id, raw); err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Verweise von %q konnten nicht gesetzt werden: %v", rp.page.Title, err))
		}
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
		created, err := s.Snippets.Create(ctx, websiteID, sn.Key, sn.Name, markdown, html)
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Textbaustein %q konnte nicht angelegt werden: %v", sn.Key, err))
			continue
		}
		// Create endet auf Get, und Get gibt (nil, nil) heraus, wenn die Zeile
		// nicht dasteht — durch den Lesepool, einen anderen als den, der eben
		// geschrieben hat. Vor 08-05 wurde der Rückgabewert weggeworfen; seit
		// die Felder nachgezogen werden, wäre nil ein Absturz, der die ganze
		// Anfrage mitnimmt. internal/admin/snippet.go wacht über denselben
		// Wert, und die zwei Aufrufstellen sagen jetzt dasselbe über ihn.
		if created == nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf(
				"Textbaustein %q konnte nach dem Anlegen nicht zurückgelesen werden.", sn.Key))
			continue
		}
		report.Snippets++
		importSnippetFields(ctx, s, websiteID, created.ID, sn, report)
	}
}

// importSnippetFields recreates one snippet's own definitions and its values.
//
// Every definition goes through field.Store.Create and never through an INSERT
// of its own. That is the whole point of doing it here rather than in SQL: a
// manifest is a file somebody handed the program, and Create is where validate
// runs — the key, the kind, the bounds and the per-carrier MaxFields count.
// A definition it refuses produces a line in the report and the loop goes on,
// so a malformed archive costs one field and not the import, and never writes a
// row nothing can render.
//
// The SnippetID handed to Create is the id of the snippet this import has just
// created on the website it is importing into. The manifest never supplies an
// id, so a bundle cannot name another website's snippet, and Create's own
// website scoping is the second layer under that.
//
// The conditions are not carried and there is no second pass for them:
// validate empties the condition of every snippet field (08-02), so there would
// be nothing to hang on afterwards even if the manifest named one.
func importSnippetFields(ctx context.Context, s Stores, websiteID, snippetID int64,
	sn Snippet, report *Report) {

	if s.Fields == nil {
		if len(sn.Fields) > 0 {
			report.Warnings = append(report.Warnings, fmt.Sprintf(
				"Die eigenen Felder des Textbausteins %q konnten nicht angelegt werden.", sn.Key))
		}
		return
	}

	for _, f := range sn.Fields {
		created, err := s.Fields.Create(ctx, field.Def{
			WebsiteID: websiteID, SnippetID: snippetID,
			Key: f.Key, Label: f.Label, Kind: f.Kind, Required: f.Required,
			Hint: f.Hint, Choices: f.Choices,
			Display: f.Display, MaxValues: f.MaxValues,
			RangeMin: f.Min, RangeMax: f.Max,
		})
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Feld %q konnte nicht angelegt werden: %v", f.Label, err))
			continue
		}
		for _, sub := range f.Sub {
			// A group's sub-fields carry the parent id AND the snippet id: the
			// group is a row of this snippet's form, and field.Store.Sub reads
			// them by the parent alone.
			if _, err := s.Fields.Create(ctx, field.Def{
				WebsiteID: websiteID, ParentID: created.ID, SnippetID: snippetID,
				Key: sub.Key, Label: sub.Label, Kind: sub.Kind, Required: sub.Required,
				Hint: sub.Hint, Choices: sub.Choices,
				Display: sub.Display, MaxValues: sub.MaxValues,
				RangeMin: sub.Min, RangeMax: sub.Max,
			}); err != nil {
				report.Warnings = append(report.Warnings, fmt.Sprintf(
					"Feld %q in der Gruppe %q konnte nicht angelegt werden: %v", sub.Label, f.Label, err))
			}
		}
	}

	if len(sn.Values) == 0 && len(sn.ValueGroups) == 0 {
		return
	}
	// Read the definitions back rather than collecting them above: what Create
	// accepted is what the values are cleaned against, positions, groups and
	// all, and OfSnippet is the same reader the snippet's own form uses.
	defs, err := s.Fields.OfSnippet(ctx, websiteID, snippetID)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"Die Werte des Textbausteins %q konnten nicht übernommen werden: %v", sn.Key, err))
		return
	}
	raw, verworfen, err := cleanSnippetValues(defs, sn)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"Die Werte des Textbausteins %q konnten nicht übernommen werden: %v", sn.Key, err))
		return
	}
	if len(verworfen) > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"Textbaustein %q: die Werte von %s wurden nicht übernommen, sie halten die Regeln ihrer Felder nicht ein.",
			sn.Key, strings.Join(verworfen, ", ")))
	}
	if err := s.Snippets.SetFields(ctx, snippetID, raw); err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"Die Werte des Textbausteins %q konnten nicht gespeichert werden: %v", sn.Key, err))
	}
}

// cleanSnippetValues turns a manifest's snippet values into what is stored, and
// returns the keys it had to drop on the way.
//
// The same two guards importFieldValues applies to a page, and for the same
// reason: nothing in a manifest has been through the form that every other
// write path goes through. field.Clean drops a value naming a key no definition
// carries and bounds a group's rows — by hand rather than by Clean the archive
// path and the form path would drift apart. field.CheckAll is where the byte
// budget and the per-kind rules live; leaving it out here would re-open on the
// snippet exactly the hole 07-04 closed on the page.
//
// Ohne Vorbedingung, aus demselben Grund wie in importFieldValues und als
// dieselbe Entscheidung: „if len(defs) > 0" liess genau das Manifest durch,
// das Werte ohne Definitionen mitbringt — den Zuschnitt, der zur Gänze von Hand
// geschrieben ist. Der Satz oben nannte damit ein Loch, das darunter offen
// stand.
func cleanSnippetValues(defs []field.Def, sn Snippet) (string, []string, error) {
	data := field.Data{Values: field.Values{}, Rows: map[string][]field.Values{}}
	for key, val := range sn.Values {
		data.Values[key] = val
	}
	for key, rows := range sn.ValueGroups {
		out := make([]field.Values, 0, len(rows))
		for _, row := range rows {
			if len(row) == 0 {
				continue
			}
			values := field.Values{}
			for k, v := range row {
				values[k] = v
			}
			out = append(out, values)
		}
		if len(out) > 0 {
			data.Rows[key] = out
		}
	}

	var dropped []string
	data = field.Clean(defs, data)
	for key := range field.CheckAll(defs, data) {
		// A required field left empty is reported here too, and there is
		// nothing to take away in that case — the snippet arrives as it
		// left. Only what is actually there is dropped.
		if _, ok := data.Values[key]; ok {
			delete(data.Values, key)
			dropped = append(dropped, key)
			continue
		}
		if group, i, sub, ok := splitRowKey(key); ok {
			if rows := data.Rows[group]; i < len(rows) {
				if _, ok := rows[i][sub]; ok {
					delete(rows[i], sub)
					dropped = append(dropped, key)
				}
			}
		}
	}
	sort.Strings(dropped)

	raw, err := field.Encode(data)
	if err != nil {
		return "", dropped, err
	}
	return raw, dropped, nil
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
		loc := locale.Pick(mn.Locale, extras)
		created, err := s.Menus.CreateMenu(ctx, websiteID, mn.Name, mn.LocationKey, loc)
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Menü %q konnte nicht angelegt werden: %v", mn.Name, err))
			continue
		}
		importItems(ctx, s, websiteID, created.ID, nil, mn.Items, loc, report)
		report.Menus++
	}
}

// importItems writes one level of a menu. loc is the menu's language, and it
// decides which page an entry points at: the French menu links the French
// pages. Without it every language's menu pointed at the same set — whichever
// the address lookup happened to return — and four of five menus led to pages
// in a language nobody had asked for.
func importItems(ctx context.Context, s Stores, websiteID, menuID int64,
	parent *int64, items []MenuItem, loc string, report *Report) {

	for i, item := range items {
		var pageID *int64
		if item.PageSlug != "" {
			pg, err := s.Pages.GetPageBySlugIn(ctx, websiteID, loc, item.PageSlug)
			// A menu entry pointing at a page that this language does not have
			// falls back to the main language rather than losing its target.
			if (err != nil || pg == nil) && loc != "" {
				pg, err = s.Pages.GetPageBySlugIn(ctx, websiteID, "", item.PageSlug)
			}
			if err == nil && pg != nil {
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
			importItems(ctx, s, websiteID, menuID, &id, item.Children, loc, report)
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
