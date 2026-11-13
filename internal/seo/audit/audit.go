// Package audit turns stored pages into seo.Input and runs seo.Analyze over
// them. It is the one place that knows how, so the editor's SEO tab, the
// report screen and the seo_report tool read a page the same way and cannot
// drift apart.
package audit

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/seo"
	"github.com/holzcloud/holzcloud-cms/internal/value"
)

// Source is what an audit reads from.
type Source struct {
	Pages   *page.Store
	Media   *media.Store
	Domains *domain.Store
	// BlockSet says which block kinds a website has. Nil means the built-in
	// ones, which is enough for everything this package reads: an own kind's
	// fields are text and are taken as they stand.
	BlockSet func(ctx context.Context, websiteID int64) block.Set
	// Values says which value keys a website defines. Nil, or an error, means
	// not checked: a page's [[wert:key]] is then not reported as unknown,
	// because nothing is known about the keys.
	Values func(ctx context.Context, websiteID int64) (map[string]bool, error)
}

// PageReport is one page and what is wrong with it.
type PageReport struct {
	Page     page.Page
	Findings []seo.Finding
	Worst    seo.Severity
}

// pageBatch is how many pages are read per query while collecting a website.
const pageBatch = 200

// Site analyses every live page of a website, in the order the store lists
// them. The whole website is read once because duplicate detection needs every
// other page's title anyway.
func (s Source) Site(ctx context.Context, ws *domain.Website) ([]PageReport, error) {
	all, err := s.live(ctx, ws.ID)
	if err != nil {
		return nil, err
	}
	ctxIn := s.context(ctx, ws, all)
	out := make([]PageReport, 0, len(all))
	for _, p := range all {
		out = append(out, s.report(ctx, ctxIn, p))
	}
	return out, nil
}

// Page analyses one stored page against the rest of its website.
func (s Source) Page(ctx context.Context, ws *domain.Website, p *page.Page) (PageReport, error) {
	if p == nil {
		return PageReport{}, errors.New("no page")
	}
	all, err := s.live(ctx, ws.ID)
	if err != nil {
		return PageReport{}, err
	}
	return s.report(ctx, s.context(ctx, ws, all), *p), nil
}

func (s Source) live(ctx context.Context, websiteID int64) ([]page.Page, error) {
	var all []page.Page
	for n := 1; ; n++ {
		batch, total, err := s.Pages.ListPages(ctx, websiteID, page.ListFilter{
			Locale: "*", Sort: "created_at", Page: n, PerPage: pageBatch,
		})
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) == 0 || len(all) >= total {
			return all, nil
		}
	}
}

// siteContext is what is the same for every page of one website.
type siteContext struct {
	host, siteDescription string
	others                []seo.Other
	set                   block.Set
	// values is the set of keys the website defines, nil when not checked.
	values map[string]bool
}

func (s Source) context(ctx context.Context, ws *domain.Website, all []page.Page) siteContext {
	c := siteContext{siteDescription: firstNonEmpty(ws.MetaDescription, ws.Description), set: block.Builtin}
	if s.BlockSet != nil {
		c.set = s.BlockSet(ctx, ws.ID)
	}
	if s.Values != nil {
		// An error is not a finding about any page: the map stays nil and the
		// keys go unchecked.
		if known, err := s.Values(ctx, ws.ID); err == nil {
			c.values = known
		}
	}
	if s.Domains != nil {
		if ds, err := s.Domains.ListDomains(ctx, ws.ID); err == nil {
			for _, d := range ds {
				if d.IsPrimary || c.host == "" {
					c.host = d.Domain
				}
			}
		}
	}
	c.others = make([]seo.Other, 0, len(all))
	for _, p := range all {
		c.others = append(c.others, seo.Other{
			ID: p.ID, Title: p.Title, Description: p.MetaDescription,
			Locale: p.Locale, Status: p.Status, Deleted: p.InTrash(),
		})
	}
	return c
}

func (s Source) report(ctx context.Context, c siteContext, p page.Page) PageReport {
	in := s.input(ctx, c, p)
	fs := seo.Analyze(in)
	return PageReport{Page: p, Findings: fs, Worst: seo.Worst(fs)}
}

func (s Source) input(ctx context.Context, c siteContext, p page.Page) seo.Input {
	in := seo.Input{
		ID: p.ID, Title: p.Title, Slug: p.Slug, Description: p.MetaDescription,
		Locale: p.Locale, Status: p.Status, Kind: p.Kind, TypeKey: p.TypeKey,
		Markdown: p.ContentMarkdown,
		NoIndex:  p.NoIndex, Protected: p.Protected(), HasFeatured: p.FeaturedMediaID != nil,
		Host: c.host, SiteDescription: c.siteDescription, Others: c.others,
		// The blocks are read as stored: a key is a-z, 0-9, - and _, which
		// JSON leaves as it is, so a marker in any block field is found. The
		// stored HTML and the excerpt are read too, because delivery resolves
		// both: a typed excerpt need not repeat the text, and a marker the
		// Markdown wrote with escaped brackets is literal only in the HTML
		// (WR-02 of the phase 33 review; value.Store's usage reads the same).
		UsedValues: value.Keys(p.ContentMarkdown + "\n" + p.Blocks + "\n" +
			p.ContentHTML + "\n" + p.Excerpt),
		KnownValues:        c.values,
		ValuesOutsideText:  value.Keys(p.Title + "\n" + p.MetaDescription),
		ValuesBeforeParen:  keysBeforeParen(p.ContentMarkdown + "\n" + p.Blocks),
		ValuesStartingLink: keysStartingLink(p.ContentMarkdown + "\n" + p.Blocks),
	}
	if strings.TrimSpace(p.Blocks) != "" {
		if blocks, err := block.Decode(p.Blocks, c.set); err == nil {
			in.BlocksText = FlattenBlocks(blocks)
			in.ImagesWithoutAlt = s.blocksWithoutAlt(ctx, p.WebsiteID, blocks)
		}
	}
	return in
}

// FlattenBlocks joins the prose and the links of a block list into one
// Markdown string. Pictures are not included: they have their own count.
func FlattenBlocks(blocks []block.Block) string {
	var parts []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			parts = append(parts, s)
		}
	}
	link := func(text, url string) {
		if strings.TrimSpace(url) != "" {
			add("[" + strings.TrimSpace(strings.NewReplacer("[", "", "]", "").Replace(text)) + "](" + strings.TrimSpace(url) + ")")
		}
	}
	for _, b := range blocks {
		add(b.Title)
		add(b.Markdown)
		add(b.Text)
		link(b.LinkText, b.LinkURL)
		for _, it := range b.Items {
			add(it.Title)
			add(it.Markdown)
			link(it.Title, it.LinkURL)
		}
		// An own kind's fields are free text, possibly Markdown. Sorted so the
		// output does not depend on map order.
		keys := make([]string, 0, len(b.Fields))
		for k := range b.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			add(b.Fields[k])
		}
	}
	return strings.Join(parts, "\n\n")
}

// blocksWithoutAlt counts the pictures of a block list that have no
// description: neither on the block itself nor in the media library.
func (s Source) blocksWithoutAlt(ctx context.Context, websiteID int64, blocks []block.Block) int {
	seen := map[int64]bool{}
	n := 0
	check := func(id int64, alt string) {
		if id <= 0 || strings.TrimSpace(alt) != "" || seen[id] {
			return
		}
		seen[id] = true
		if s.Media == nil {
			return
		}
		m, err := s.Media.GetByID(ctx, id)
		if err != nil || m == nil || m.WebsiteID != websiteID || !m.IsImage() {
			return
		}
		if strings.TrimSpace(m.AltText) == "" {
			n++
		}
	}
	for _, b := range blocks {
		check(b.MediaID, b.Alt)
		for _, it := range b.Items {
			check(it.MediaID, it.Alt)
		}
	}
	return n
}

// beforeParen is a value placeholder directly followed by an opening bracket.
// The key alphabet is the one internal/value matches.
var beforeParen = regexp.MustCompile(`\[\[wert:([a-z0-9][a-z0-9_-]*)\]\]\(`)

// keysBeforeParen lists, once each and in order, the keys whose placeholder
// Markdown would read as the text of a link.
func keysBeforeParen(s string) []string {
	if !value.Has(s) {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range beforeParen.FindAllStringSubmatch(s, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// linkDestination is the address of a Markdown link or picture, up to the
// closing bracket or the first space (a title may follow it).
var linkDestination = regexp.MustCompile(`\]\(([^)\s]*)`)

// keysStartingLink lists, once each and in order, the keys whose placeholder
// stands in the first segment of a relative link address (IN-06 of the phase
// 33 review).
//
// In [Release](v[[wert:k]]) the colon of "wert:" is in the first segment,
// before any "/", "?" or "#". url.Parse then reads "v[[wert" as a scheme, the
// sanitiser refuses the address, and the link is stored without one — the
// editor sees plain text and is told nothing. An address that starts with
// "/", ".", "#" or "?", that puts a folder first (releases/v[[wert:k]]), or
// that has a scheme of its own before the placeholder (https://…, mailto:…)
// keeps its address.
func keysStartingLink(s string) []string {
	if !value.Has(s) {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range linkDestination.FindAllStringSubmatch(s, -1) {
		dest := m[1]
		if dest == "" || strings.ContainsAny(dest[:1], "/.#?") {
			continue
		}
		first := dest
		if i := strings.IndexAny(dest, "/?#"); i >= 0 {
			first = dest[:i]
		}
		at := strings.Index(first, value.Prefix)
		if at < 0 || strings.Contains(first[:at], ":") {
			continue
		}
		for _, key := range value.Keys(first) {
			if !seen[key] {
				seen[key] = true
				out = append(out, key)
			}
		}
	}
	return out
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
