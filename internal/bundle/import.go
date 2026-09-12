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
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
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
	Albums    int
	// Warnings are the things that did not come through. Each one names what
	// was lost and why; none of them stops the import.
	//
	// Finished sentences and not codes, unlike csvimport's Verdict — an import
	// report is read once and thrown away, and there is nothing here to group
	// two rows by. They are assembled by warnf in the OPERATOR's language.
	Warnings []string

	// lang is the language of whoever started the import, taken from the
	// request's context by Import and used by nothing but warnf.
	//
	// It is here rather than being passed down through fifteen functions
	// because every one of those would carry it only to hand it on. Broken
	// window 6 held that threading a locale through bundle.Import was the
	// reason these sentences had to stay raw German, and that the report would
	// otherwise appear in the language of the IMPORTED website: neither is so.
	// Import already takes a context, and the language in it is the reader's —
	// an archive carries no language of its own that a report could pick up.
	lang string
}

// warnf records one warning, in the language of whoever is reading the report.
//
// The format has to be written as i18n.N("…") at the call site so that
// tools/i18n collects it. Everything filled into it is operator data — a page
// title, a filename, an error — and is never translated.
func (r *Report) warnf(format string, args ...any) {
	r.Warnings = append(r.Warnings, i18n.Tf(r.lang, format, args...))
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
		return nil, fmt.Errorf("the file is not a valid archive: %w", err)
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
		return nil, fmt.Errorf("the archive names no name for the website")
	}

	created, err := s.Domains.CreateWebsite(ctx, siteName, manifest.Site.Description)
	if err != nil {
		return nil, fmt.Errorf("create website: %w", err)
	}
	websiteID := created.ID
	report := &Report{WebsiteID: websiteID, lang: i18n.Lang(ctx)}

	if err := applySettings(ctx, s, websiteID, manifest.Site); err != nil {
		report.warnf(i18n.N("Settings: %v"), err)
	}
	mediaByName := importMedia(ctx, s, websiteID, zr, manifest, report)
	// The field definitions go in before the pages: a value without its
	// definition would be dropped by the first save of that page.
	// The content kinds first: an entry whose kind does not exist yet would
	// stand in the list under a key with no name.
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
	// The words the renderer writes itself follow the same locale as its dates:
	// a re-rendered lightbox on an imported page speaks the language of the
	// site in the manifest, not the language of whoever ran the import.
	set.T = func(word string) string { return i18n.T(manifest.Site.Locale, word) }
	// The terms before the pages: one that only a term field names exists
	// nowhere else — SetForPage creates only what stands in a page's term
	// list.
	importTerms(ctx, s, websiteID, manifest, report)
	// The albums before the pages, because a gallery block names one and the
	// report should be able to say which album a block did not find. After the
	// images, because an album's pictures are filenames and mediaByName is
	// where those become ids. Deliberately not last like the menus, whose
	// entries point at pages by address and therefore need the pages
	// (import.go:1087).
	albumSlugs := importAlbums(ctx, s, websiteID, manifest, mediaByName, report)
	importPages(ctx, s, websiteID, manifest, mediaByName, albumSlugs, fieldKinds, set, report)
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
		return nil, fmt.Errorf("%s could not be read: %w", ManifestName, err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s is corrupt: %w", ManifestName, err)
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
		report.warnf(i18n.N("Media folder: %v"), err)
		return byName
	}

	for _, entry := range m.Media {
		name := safeName(entry.Filename)
		if name == "" {
			report.warnf(i18n.N("file %q has a name that is not allowed and was skipped"), entry.Filename)
			continue
		}

		data, err := readEntry(zr, MediaDir+name)
		if err != nil {
			// Absent and damaged are different problems with different answers:
			// one means the archive was assembled wrong, the other that it did
			// not survive the journey.
			why := i18n.N("is missing from the archive")
			if !errors.Is(err, fs.ErrNotExist) {
				why = i18n.N("is damaged (the archive's checksum does not match)")
			}
			report.warnf(i18n.N("file %q %s"), entry.Filename, i18n.T(report.lang, why))
			continue
		}
		// The checksum is the difference between storing a corrupted file and
		// finding out when a page renders a grey box.
		if entry.SHA256 != "" && hashBytes(data) != entry.SHA256 {
			report.warnf(i18n.N("file %q is corrupt and was skipped"), entry.Filename)
			continue
		}

		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			report.warnf(i18n.N("file %q could not be stored: %v"), entry.Filename, err)
			continue
		}

		created, err := s.Media.Create(ctx, websiteID, name, entry.OriginalName,
			entry.MimeType, int64(len(data)), hashBytes(data))
		if err != nil {
			report.warnf(i18n.N("file %q could not be recorded: %v"), entry.Filename, err)
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
			report.warnf(i18n.N("%d content kinds could not be created."), len(m.Types))
		}
		return
	}
	for _, t := range m.Types {
		if _, err := s.Kinds.Create(ctx, kind.Type{
			WebsiteID: websiteID, Key: t.Key, Name: t.Name, Plural: t.Plural,
			Archive: t.Archive, Sort: t.Sort,
		}); err != nil {
			report.warnf(i18n.N("content kind %q: %v"), t.Name, err)
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
		report.warnf(i18n.N("%d terms could not be created."), len(m.Terms))
		return
	}
	// term.Normalize and not term.Parse: Parse is the reader of the field an
	// editor types. It splits on commas and stops at term.MaxPerPage — both of
	// which hold for one ENTRY and for nothing else. Sending a whole website's
	// list through that reader dropped everything from the thirteenth term on,
	// silently, and tore a name with a comma in it in two. Each name on its
	// own, with the same normalisation every other term gets.
	names := make([]string, 0, len(m.Terms))
	for _, t := range m.Terms {
		if name := term.Normalize(t.Name); name != "" {
			names = append(names, name)
		}
	}
	n, err := s.Terms.EnsureNames(ctx, websiteID, names)
	if err != nil {
		report.warnf(i18n.N("terms: %v"), err)
		return
	}
	// What was created, not what the archive claims: a number a report names
	// should be believable.
	report.Terms = n
}

// importAlbums recreates the website's albums, with their pictures in order.
//
// Every album is made through album.Store.Create and never through an INSERT
// of this package's own. That is not tidiness: Create holds the one call to
// page.Slugify that derives an album's address, and importBlocks derives the
// same address from the same name. A second INSERT here would be a second
// derivation of one key, which internal/term/store.go:318-328 is a long
// warning about — and the two would agree until the day one of them changed.
//
// A name the store refuses is a warning that names the album and never a
// failed import: an import that stops halfway is worse than one that says what
// is missing. The album is named by its position as well as its text, because
// the name that gets refused most often is the one that is blank.
//
// A picture whose file did not arrive is dropped and reported rather than
// guessed at. mediaByName holds the media this import created for this
// website, so a name that is not in it resolves to nothing — never to a
// number, which is blocks.go's rule one level up.
func importAlbums(ctx context.Context, s Stores, websiteID int64, m *Manifest,
	mediaByName map[string]int64, report *Report) map[string]string {
	if len(m.Albums) == 0 {
		return nil
	}
	if s.Albums == nil {
		report.warnf(i18n.N("%d albums could not be created."), len(m.Albums))
		return nil
	}
	// Which names the archive uses twice. A block naming such a name names two
	// albums, and there is nothing in the archive that would say which — so
	// the mapping is not hard to make, it is absent. It is therefore dropped
	// and reported rather than guessed: binding it to the first album would
	// mean a gallery shows another album's pictures and nobody ever finds out.
	twice := map[string]bool{}
	seen := map[string]bool{}
	for _, a := range m.Albums {
		if seen[a.Name] {
			twice[a.Name] = true
		}
		seen[a.Name] = true
	}

	slugs := map[string]string{}
	created := 0
	for i, a := range m.Albums {
		row, err := s.Albums.Create(ctx, websiteID, a.Name)
		if err != nil {
			report.warnf(i18n.N("album %d %q could not be created: %v"), i+1, a.Name, err)
			continue
		}
		created++
		// The address this album was given on THIS machine, asked of the store
		// and not derived a second time. blocks.go reads it back here rather
		// than calling page.Slugify itself: two derivations of the same key are
		// the danger internal/album's package comment warns about explicitly.
		if !twice[a.Name] {
			slugs[a.Name] = row.Slug
		}
		for _, it := range a.Items {
			id, ok := mediaByName[it.Media]
			if !ok {
				report.warnf(i18n.N("album %q: the image %q is not in the archive."), row.Name, it.Media)
				continue
			}
			if _, err := s.Albums.AddItem(ctx, websiteID, row.ID, id, it.Alt, it.Caption); err != nil {
				report.warnf(i18n.N("album %q: the image %q did not arrive: %v"), row.Name, it.Media, err)
			}
		}
	}
	for name := range twice {
		report.warnf(i18n.N("Das Archiv nennt zwei Alben %q. Es kann nur eines davon geben, und "+
			"keine Galerie wird daran gebunden — sonst zeigte sie die Bilder des "+
			"falschen. Die betroffenen Seiten stehen unten einzeln."), name)
	}
	// What was created, not what the archive claims — the same rule as for the
	// terms above: a number a report names should be believable.
	report.Albums = created
	return slugs
}

// importFields recreates the website's own field definitions and returns which
// of them are pictures, so the values can be translated back to ids.
func importFields(ctx context.Context, s Stores, websiteID int64, m *Manifest, report *Report) map[string]string {
	kinds := map[string]string{}
	if s.Fields == nil {
		if len(m.Fields) > 0 {
			report.warnf(i18n.N("the website's own fields could not be created."))
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
			report.warnf(i18n.N("field %q could not be created: %v"), f.Label, err)
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
				report.warnf(i18n.N("field %q in the group %q could not be created: %v"), sub.Label, f.Label, err)
			}
		}
	}

	for id, def := range conditions {
		if err := s.Fields.Update(ctx, websiteID, id, def); err != nil {
			// The field is there and works; only the condition is missing, so it
			// is always shown. Worth a line, not worth failing the import.
			report.warnf(i18n.N("the condition of field %q could not be taken over: %v"), def.Label, err)
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
			report.warnf(i18n.N("the website's own block kinds could not be created."))
		}
		return
	}
	for _, t := range m.BlockTypes {
		created, err := s.BlockTypes.Create(ctx, websiteID, t.Name, t.Hint)
		if err != nil {
			report.warnf(i18n.N("block kind %q could not be created: %v"), t.Name, err)
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
				report.warnf(i18n.N("field %q of block kind %q could not be created: %v"),
					f.Label, t.Name, err)
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
// Both run unconditionally, and that is a correction: they used to hang off
// `if len(defs) > 0`. A manifest that brought values and no definitions got
// past neither of them — the one shape for which there is no form and no
// screen was at the same time the only one that ran into the column unchecked.
// Without definitions Clean rightly throws everything away: a value no
// definition carries is displayable by nothing.
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
		// A required field with no value is reported here too, and there is
		// nothing to take away there — the page simply arrives without it, just
		// as it departed. What is taken away is only what actually stands there.
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
			// The value in the archive is a name; what is stored is a slug.
			// page.Slugify is the one rule the term store applies as well — so
			// the two agree by construction and not by accident.
			//
			// Neither looked up nor dropped: importTerms has already run, and a
			// value whose term the archive never named resolves to nothing when
			// rendered — exactly the empty behaviour the kind promises.
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
	mediaByName map[string]int64, albumSlugs map[string]string,
	fieldKinds map[string]string, set block.Set, report *Report) {

	// One lookup for the whole import, so a picture used on twenty pages is
	// read once.
	look := blockImages(ctx, s, websiteID)

	// The field definitions as they were actually created — read back rather
	// than rebuilt from the archive, so that the check runs against what this
	// website has, including everything validate emptied on the way in.
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
			report.warnf(i18n.N("page %q: the address %q is not allowed"), p.Title, p.Slug)
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
				report.warnf(i18n.N("Seite %q ist in der Sprache %q verfasst, die diese Website nicht hat – "+
					"sie liegt jetzt in der Hauptsprache."), p.Title, p.Locale)
			}
		}

		markdown := rewriteMediaPaths(p.Markdown, websiteID, m.Media)

		// Re-rendered here rather than carried in the archive: a bundle written
		// by an older renderer would otherwise import HTML that no longer
		// matches what this version produces.
		html, err := page.RenderMarkdown(markdown)
		if err != nil {
			report.warnf(i18n.N("page %q could not be set: %v"), p.Title, err)
			continue
		}

		// A page built with the block editor: the blocks are what it is made of,
		// and the markdown column holds the plain text derived from them — which
		// is what the search index and the excerpt read.
		encodedBlocks := ""
		if len(p.Blocks) > 0 {
			blocks := set.Clean(importBlocks(p.Blocks, set, mediaByName, albumSlugs))
			if len(blocks) > 0 {
				encoded, eerr := block.Encode(blocks, set)
				if eerr != nil {
					report.warnf(i18n.N("page %q: the blocks could not be stored: %v"), p.Title, eerr)
				} else {
					encodedBlocks = encoded
					markdown = block.PlainText(blocks, set)
					html = block.Render(blocks, set, look, page.RenderMarkdown)
				}
			}
			for _, name := range missingMedia(p.Blocks, set, mediaByName) {
				report.warnf(i18n.N("page %q: the file %q is missing, the block stays without an image"), p.Title, name)
			}
			// The same thing one level up: a block naming an album that is not
			// in the archive gets a line in the report rather than an empty
			// gallery with no explanation.
			for _, name := range missingAlbum(p.Blocks, albumSlugs) {
				report.warnf(i18n.N("page %q: the album %q is not in the archive, the gallery stays empty"), p.Title, name)
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

		fields, verworfen := importFieldValues(defs, fieldKinds, p, mediaByName, pages.in(loc))
		if len(verworfen) > 0 {
			report.warnf(i18n.N("page %q: the values of %s were not taken over, they do not keep the rules of their fields."),
				p.Title, strings.Join(verworfen, ", "))
		}

		created, err := s.Pages.CreatePage(ctx, page.PageCreate{
			WebsiteID: websiteID, Title: p.Title, Slug: slug, Locale: loc,
			Markdown: markdown, HTML: html, Blocks: encodedBlocks, Status: p.Status,
			Fields: fields,
			Meta:   meta, Kind: p.Kind, TypeKey: p.TypeKey,
			Schedule: page.PageSchedule{PublishAt: p.PublishAt, UnpublishAt: p.UnpublishAt},
		})
		if err != nil {
			report.warnf(i18n.N("page %q could not be created: %v"), p.Title, err)
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
				report.warnf(i18n.N("terms of %q: %v"), p.Title, err)
			}
		}
		// The password never travels, so a protected page arrives unprotected
		// and the operator has to be told rather than left to discover it.
		if p.Protected() {
			report.warnf(i18n.N("page %q was protected with a password. Passwords are never exported – "+
				"please set a new one; until then the page is public."), p.Title)
		}
	}

	for _, l := range links {
		// A missing counterpart is not an error: the page is still in its
		// language, it just stands on its own.
		// translation_of always names a page in the main language, so it is
		// looked up there and nowhere else.
		of, _ := pages.at("", l.ofSlug)
		if err := s.Pages.SetTranslation(ctx, websiteID, l.id, l.loc, of); err != nil {
			report.warnf(i18n.N("the language could not be set: %v"), err)
		}
	}

	// The references, once. A page may point at one that is created after it,
	// so the first pass could only resolve what happened to exist already; now
	// every address in the bundle has an id. Only the pages that actually carry
	// a reference are written again.
	for _, rp := range refs {
		// Whatever is discarded here was already reported on the first pass —
		// same page, same values, same rules.
		raw, _ := importFieldValues(defs, fieldKinds, rp.page, mediaByName, pages.in(rp.loc))
		if err := s.Pages.SetFields(ctx, rp.id, raw); err != nil {
			report.warnf(i18n.N("references of %q could not be set: %v"), rp.page.Title, err)
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
			report.warnf(i18n.N("snippet %q: %v"), sn.Key, err)
			continue
		}
		created, err := s.Snippets.Create(ctx, websiteID, sn.Key, sn.Name, markdown, html)
		if err != nil {
			report.warnf(i18n.N("snippet %q could not be created: %v"), sn.Key, err)
			continue
		}
		// Create ends in Get, and Get hands out (nil, nil) when the row is not
		// there — through the read pool, a different one from the one that has
		// just written. Before 08-05 the return value was thrown away; since
		// the fields are fetched afterwards, nil would be a crash that takes
		// the whole request with it. internal/admin/snippet.go guards the same
		// value, and the two call sites now say the same thing about it.
		if created == nil {
			report.warnf(i18n.N("snippet %q could not be read back after being created."), sn.Key)
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
// website scoping is the second layer under that — since the review of phase 8
// it checks that the id belongs to d.WebsiteID (ErrNoSnippet). That sentence
// stood here before and was not yet true: REFERENCES proves only that the row
// exists.
//
// The conditions are not carried and there is no second pass for them:
// validate empties the condition of every snippet field (08-02), so there would
// be nothing to hang on afterwards even if the manifest named one.
func importSnippetFields(ctx context.Context, s Stores, websiteID, snippetID int64,
	sn Snippet, report *Report) {

	if s.Fields == nil {
		if len(sn.Fields) > 0 {
			report.warnf(i18n.N("the snippet %q's own fields could not be created."), sn.Key)
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
			report.warnf(i18n.N("field %q could not be created: %v"), f.Label, err)
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
				report.warnf(i18n.N("field %q in the group %q could not be created: %v"), sub.Label, f.Label, err)
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
		report.warnf(i18n.N("the values of snippet %q could not be taken over: %v"), sn.Key, err)
		return
	}
	raw, verworfen, err := cleanSnippetValues(defs, sn)
	if err != nil {
		report.warnf(i18n.N("the values of snippet %q could not be taken over: %v"), sn.Key, err)
		return
	}
	if len(verworfen) > 0 {
		report.warnf(i18n.N("snippet %q: the values of %s were not taken over, they do not keep the rules of their fields."),
			sn.Key, strings.Join(verworfen, ", "))
	}
	if bound := installationBoundValues(defs, raw); len(bound) > 0 {
		report.warnf(i18n.N("snippet %q: the values of %s point at ids of the installation "+
			"the archive came from, and have to be chosen again here."),
			sn.Key, strings.Join(bound, ", "))
	}
	if err := s.Snippets.SetFields(ctx, websiteID, snippetID, raw); err != nil {
		report.warnf(i18n.N("the values of snippet %q could not be stored: %v"), sn.Key, err)
	}
}

// installationBoundValues names the fields whose value is an id of the
// installation the archive came from.
//
// The value of an image, reference or term field is not a string that means the
// same thing everywhere — it is an id. On the page path such ids are translated
// into a filename and an address on the way out and back on the way in
// (exportFieldValues/translateIn); a snippet's values go out raw and come in
// raw. On the other side the id belongs to a different website, fieldImages and
// fieldRefs refuse it, and the field arrives while the picture is missing.
//
// That stays so for now — the translation sits on the page path, and prising it
// out of there is work of its own (deferred-items.md). What stands here is the
// volume: the admin screen offers these field kinds explicitly and promises
// them to the operator, so the promise must not break silently. The value
// travels along regardless, so that nothing disappears and the choice only has
// to be repeated on the other side.
func installationBoundValues(defs []field.Def, raw string) []string {
	data := field.Decode(raw)
	ortsgebunden := func(kind string) bool {
		switch kind {
		case field.KindImage, field.KindRef, field.KindTerm:
			return true
		}
		return false
	}

	var out []string
	for _, d := range defs {
		if d.IsGroup() {
			for _, sub := range d.Sub {
				if !ortsgebunden(sub.Kind) {
					continue
				}
				for i, row := range data.Rows[d.Key] {
					if row[sub.Key] != "" {
						out = append(out, field.RowKey(d.Key, i, sub.Key))
					}
				}
			}
			continue
		}
		if ortsgebunden(d.Kind) && data.Values[d.Key] != "" {
			out = append(out, d.Key)
		}
	}
	sort.Strings(out)
	return out
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
// Unconditional, for the same reason as in importFieldValues and as the same
// decision: `if len(defs) > 0` let through exactly the manifest that brings
// values without definitions — the shape that is written entirely by hand. The
// sentence above therefore named a hole that stood open underneath it.
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
			report.warnf(i18n.N("menu %q could not be created: %v"), mn.Name, err)
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
				report.warnf(i18n.N("menu item %q points at the page %q, which does not exist"), item.Title, item.PageSlug)
			}
		}
		created, err := s.Menus.CreateItem(ctx, menuID, parent, item.Title, item.Type, item.URL, pageID, i)
		if err != nil {
			report.warnf(i18n.N("menu item %q: %v"), item.Title, err)
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
