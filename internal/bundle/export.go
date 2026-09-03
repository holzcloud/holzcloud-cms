package bundle

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	"github.com/holzcloud/holzcloud-cms/internal/term"
)

// Stores are the readers an export needs. Passing them in rather than reaching
// for a global keeps this package testable without an HTTP server.
type Stores struct {
	Domains  *domain.Store
	Pages    *page.Store
	Menus    *menu.Store
	Snippets *snippet.Store
	Terms    *term.Store
	Media    *media.Store
	// Fields are the website's own page fields. Nil means a bundle carries
	// none, which is right for a build without them.
	Fields *field.Store
	// Kinds are the website's own content kinds. Nil means a bundle carries
	// none, which is what a build without them looks like.
	Kinds *kind.Store
	// BlockTypes are the website's own block kinds. Nil means a bundle carries
	// none.
	BlockTypes *block.Store
	DataDir    string
}

// maxExportPages bounds an export.
//
// A site with more than this has outgrown a single archive, and streaming a
// gigabyte out of a small server over one request would be worse than saying
// so.
const maxExportPages = 5000

// Export writes a website as a zip archive.
//
// Streaming straight into w rather than building the file first: on a node
// with half a gigabyte of RAM, a site with two hundred photos does not fit in
// memory twice.
func Export(ctx context.Context, s Stores, websiteID int64, version string, w io.Writer) error {
	ws, err := s.Domains.GetWebsite(ctx, websiteID)
	if err != nil {
		return fmt.Errorf("load website: %w", err)
	}
	if ws == nil {
		return fmt.Errorf("website %d does not exist", websiteID)
	}

	manifest, err := buildManifest(ctx, s, ws, version)
	if err != nil {
		return err
	}

	zw := zip.NewWriter(w)
	// The manifest goes in first so a reader can learn the version without
	// scanning to the end of a large archive.
	entry, err := zw.Create(ManifestName)
	if err != nil {
		return fmt.Errorf("create manifest: %w", err)
	}
	enc := json.NewEncoder(entry)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifest); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	dir := filepath.Join(s.DataDir, "media", strconv.FormatInt(websiteID, 10))
	for _, m := range manifest.Media {
		if err := copyMedia(zw, dir, m.Filename); err != nil {
			// A missing file must not lose the whole export: the manifest still
			// describes it and the import will say which files did not arrive.
			continue
		}
	}
	return zw.Close()
}

// copyMedia streams one file into the archive.
func copyMedia(zw *zip.Writer, dir, filename string) error {
	src, err := os.Open(filepath.Join(dir, filename))
	if err != nil {
		return err
	}
	defer src.Close()

	// Store rather than deflate: these are JPEGs and PNGs, already compressed,
	// and re-compressing them costs CPU to save nothing.
	entry, err := zw.CreateHeader(&zip.FileHeader{
		Name:   MediaDir + filename,
		Method: zip.Store,
	})
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, src)
	return err
}

func buildManifest(ctx context.Context, s Stores, ws *domain.Website, version string) (*Manifest, error) {
	m := &Manifest{
		Version:     Version,
		ExportedAt:  time.Now().UTC().Format(time.RFC3339),
		GeneratedBy: "holzcloud " + version,
		Site: Site{
			Name: ws.Name, Description: ws.Description,
			Locale: ws.Locale, ExtraLocales: ws.ExtraLocales, TimeZone: ws.TimeZone,
			MetaDescription: ws.MetaDescription,
			BlogBase:        ws.BlogBase, PostsPerPage: ws.PostsPerPage,
			ContactEmail: ws.ContactEmail,
			OrgType:      ws.OrgType, Street: ws.Street, PostalCode: ws.PostalCode,
			City: ws.City, Country: ws.Country, Phone: ws.Phone,
			OpeningHours: ws.OpeningHours,
			Design: Design{
				Ink: ws.TokenInk, Paper: ws.TokenPaper, Brand: ws.TokenBrand,
				Font: ws.TokenFont, Measure: ws.TokenMeasure, Radius: ws.TokenRadius,
			},
		},
	}

	mediaByID, err := exportMedia(ctx, s, ws.ID, m)
	if err != nil {
		return nil, err
	}
	if err := exportPages(ctx, s, ws.ID, m, mediaByID); err != nil {
		return nil, err
	}
	if err := exportMenus(ctx, s, ws.ID, m); err != nil {
		return nil, err
	}
	if err := exportSnippets(ctx, s, ws.ID, m); err != nil {
		return nil, err
	}
	if err := exportTerms(ctx, s, ws.ID, m); err != nil {
		return nil, err
	}
	if err := exportFields(ctx, s, ws.ID, m); err != nil {
		return nil, err
	}
	if err := exportTypes(ctx, s, ws.ID, m); err != nil {
		return nil, err
	}
	return m, exportBlockTypes(ctx, s, ws.ID, m)
}

func exportMedia(ctx context.Context, s Stores, websiteID int64, m *Manifest) (map[int64]string, error) {
	byID := map[int64]string{}
	if s.Media == nil {
		return byID, nil
	}
	items, _, err := s.Media.List(ctx, websiteID, media.Filter{}, 1, 10000)
	if err != nil {
		return nil, fmt.Errorf("list media: %w", err)
	}
	for _, item := range items {
		byID[item.ID] = item.Filename
		m.Media = append(m.Media, Media{
			Filename: item.Filename, OriginalName: item.OriginalName,
			MimeType: item.MimeType, AltText: item.AltText, Caption: item.Caption,
			SHA256: item.ContentHash,
		})
	}
	return byID, nil
}

func exportPages(ctx context.Context, s Stores, websiteID int64, m *Manifest, mediaByID map[int64]string) error {
	// "*" is every language: without it an export of a multilingual site would
	// quietly carry only the main language and the import would look like it
	// had worked.
	pages, _, err := s.Pages.ListPages(ctx, websiteID,
		page.ListFilter{Locale: "*", Page: 1, PerPage: maxExportPages})
	if err != nil {
		return fmt.Errorf("list pages: %w", err)
	}

	labels := map[int64][]term.Term{}
	if s.Terms != nil && len(pages) > 0 {
		ids := make([]int64, len(pages))
		for i, p := range pages {
			ids[i] = p.ID
		}
		if labels, err = s.Terms.ForPages(ctx, ids); err != nil {
			return err
		}
	}

	// Which fields are pictures has to be known before the values are written:
	// a picture is stored as an id, and an id is worthless on another machine.
	fieldKinds := map[string]string{}
	if s.Fields != nil {
		if defs, err := s.Fields.List(ctx, websiteID); err == nil {
			for _, d := range defs {
				fieldKinds[d.Key] = d.Kind
				for _, sub := range d.Sub {
					fieldKinds[d.Key+"."+sub.Key] = sub.Kind
				}
			}
		}
	}

	// The website's own block kinds. Needed here and not only in the block-type
	// list: an own kind's picture field holds an id, and telling which value is
	// one takes the kind's field definitions.
	set := block.Builtin
	if s.BlockTypes != nil {
		set = s.BlockTypes.Set(ctx, websiteID)
	}

	// The translation links travel as addresses, so the group has to be
	// resolvable from ids that only exist on this machine.
	slugByID := make(map[int64]string, len(pages))
	for _, p := range pages {
		slugByID[p.ID] = p.Slug
	}

	for _, p := range pages {
		out := Page{
			Title: p.Title, Slug: p.Slug, Markdown: p.ContentMarkdown,
			Status: p.Status, Kind: p.Kind, TypeKey: p.TypeKey,
			Excerpt: p.Excerpt, MetaDescription: p.MetaDescription, NoIndex: p.NoIndex,
			PublishedAt: p.PublishedAt, PublishAt: p.PublishAt, UnpublishAt: p.UnpublishAt,
			Access: p.Access, AccessHint: p.AccessHint,
			Locale: p.Locale,
		}
		if p.TranslationOf != 0 {
			out.TranslationOf = slugByID[p.TranslationOf]
		}
		if p.FeaturedMediaID != nil {
			out.FeaturedImage = mediaByID[*p.FeaturedMediaID]
		}
		if blocks, err := block.Decode(p.Blocks, set); err == nil && len(blocks) > 0 {
			out.Blocks = exportBlocks(blocks, set, mediaByID)
		}
		out.Fields, out.FieldGroups = exportFieldValues(fieldKinds, p.Fields, mediaByID, slugByID)
		for _, t := range labels[p.ID] {
			out.Terms = append(out.Terms, t.Name)
		}
		m.Pages = append(m.Pages, out)
	}
	return nil
}

func exportMenus(ctx context.Context, s Stores, websiteID int64, m *Manifest) error {
	if s.Menus == nil {
		return nil
	}
	menus, err := s.Menus.ListMenus(ctx, websiteID)
	if err != nil {
		return fmt.Errorf("list menus: %w", err)
	}
	for _, mn := range menus {
		tree, err := s.Menus.GetMenuTreeIn(ctx, websiteID, mn.LocationKey, mn.Locale)
		if err != nil {
			return fmt.Errorf("load menu %q: %w", mn.LocationKey, err)
		}
		m.Menus = append(m.Menus, Menu{
			Name: mn.Name, LocationKey: mn.LocationKey, Locale: mn.Locale,
			Items: exportNodes(tree),
		})
	}
	return nil
}

// exportNodes flattens the tree into the nested form the manifest uses.
func exportNodes(nodes []menu.MenuNode) []MenuItem {
	var out []MenuItem
	for _, n := range nodes {
		out = append(out, MenuItem{
			Title: n.Title, Type: n.ItemType, PageSlug: n.PageSlug, URL: n.URL,
			Children: exportNodes(n.Children),
		})
	}
	return out
}

func exportSnippets(ctx context.Context, s Stores, websiteID int64, m *Manifest) error {
	if s.Snippets == nil {
		return nil
	}
	items, err := s.Snippets.List(ctx, websiteID)
	if err != nil {
		return fmt.Errorf("list snippets: %w", err)
	}
	for _, sn := range items {
		m.Snippets = append(m.Snippets, Snippet{
			Key: sn.Key, Name: sn.Name, Markdown: sn.ContentMarkdown,
		})
	}
	return nil
}

func exportTerms(ctx context.Context, s Stores, websiteID int64, m *Manifest) error {
	if s.Terms == nil {
		return nil
	}
	terms, err := s.Terms.ListAll(ctx, websiteID)
	if err != nil {
		return fmt.Errorf("list terms: %w", err)
	}
	for _, t := range terms {
		m.Terms = append(m.Terms, Term{Slug: t.Slug, Name: t.Name})
	}
	return nil
}

// exportTypes writes the website's own content kinds.
func exportTypes(ctx context.Context, s Stores, websiteID int64, m *Manifest) error {
	if s.Kinds == nil {
		return nil
	}
	types, err := s.Kinds.List(ctx, websiteID)
	if err != nil {
		return fmt.Errorf("list content types: %w", err)
	}
	for _, t := range types {
		m.Types = append(m.Types, ContentType{
			Key: t.Key, Name: t.Name, Plural: t.Plural, Archive: t.Archive, Sort: t.Sort,
		})
	}
	return nil
}

// exportFields writes the website's own field definitions.
// exportBlockTypes writes the website's own block kinds and what they are made
// of.
func exportBlockTypes(ctx context.Context, s Stores, websiteID int64, m *Manifest) error {
	if s.BlockTypes == nil {
		return nil
	}
	types, err := s.BlockTypes.List(ctx, websiteID)
	if err != nil {
		return fmt.Errorf("list block types: %w", err)
	}
	for _, t := range types {
		out := BlockType{Key: t.Key, Name: t.Name, Hint: t.Hint}
		for _, d := range t.Fields {
			out.Fields = append(out.Fields, Field{
				Key: d.Key, Label: d.Label, Kind: d.Kind,
				Hint: d.Hint, Choices: d.Choices,
			})
		}
		m.BlockTypes = append(m.BlockTypes, out)
	}
	return nil
}

func exportFields(ctx context.Context, s Stores, websiteID int64, m *Manifest) error {
	if s.Fields == nil {
		return nil
	}
	defs, err := s.Fields.List(ctx, websiteID)
	if err != nil {
		return fmt.Errorf("list fields: %w", err)
	}
	for _, d := range defs {
		f := Field{
			Key: d.Key, Label: d.Label, Kind: d.Kind, Required: d.Required,
			Hint: d.Hint, Choices: d.Choices, AppliesTo: d.AppliesTo,
			Condition: d.Condition,
		}
		for _, sub := range d.Sub {
			f.Sub = append(f.Sub, Field{
				Key: sub.Key, Label: sub.Label, Kind: sub.Kind, Required: sub.Required,
				Hint: sub.Hint, Choices: sub.Choices,
			})
		}
		m.Fields = append(m.Fields, f)
	}
	return nil
}

// hashBytes is the checksum an import compares against.
func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// exportFieldValues turns stored values into what a bundle carries.
//
// A picture field holds a media id and a reference field a page id — numbers
// that mean something only in this database. They travel as the file name and
// as the address, the same way the featured image and the translation links
// do, and come back as ids on the other side.
func exportFieldValues(kinds map[string]string, raw string, mediaByID, slugByID map[int64]string) (map[string]string, map[string][]map[string]string) {
	data := field.Decode(raw)
	if data.Empty() {
		return nil, nil
	}
	values := translateOut(kinds, data.Values, mediaByID, slugByID)

	var groups map[string][]map[string]string
	for key, rows := range data.Rows {
		out := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			// The sub-fields are keyed by "group.sub" in the kind map, so a
			// picture inside a group is recognised the same way.
			out = append(out, translateOut(subKinds(kinds, key), row, mediaByID, slugByID))
		}
		if len(out) > 0 {
			if groups == nil {
				groups = map[string][]map[string]string{}
			}
			groups[key] = out
		}
	}
	return values, groups
}

// translateOut turns stored values into what a bundle carries: a picture's id
// becomes its file name.
func translateOut(kinds map[string]string, values field.Values, mediaByID, slugByID map[int64]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, val := range values {
		// A reference is a page id, which means nothing on the machine the
		// bundle lands on. It travels as the address, the same way a
		// translation link does, and becomes an id again on the other side.
		if kinds[key] == field.KindRef {
			id, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				continue
			}
			slug := slugByID[id]
			if slug == "" {
				// The target is not in this bundle — a reference to a page
				// that was not exported. Dropped rather than carried as a
				// number that would point at whatever happens to have that id.
				continue
			}
			out[key] = slug
			continue
		}
		if kinds[key] == field.KindImage {
			id, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				continue
			}
			name := mediaByID[id]
			if name == "" {
				continue
			}
			out[key] = name
			continue
		}
		out[key] = val
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// subKinds narrows the kind map to one group's sub-fields, keyed by their own
// names.
func subKinds(kinds map[string]string, group string) map[string]string {
	prefix := group + "."
	out := map[string]string{}
	for key, kind := range kinds {
		if rest, ok := strings.CutPrefix(key, prefix); ok {
			out[rest] = kind
		}
	}
	return out
}
