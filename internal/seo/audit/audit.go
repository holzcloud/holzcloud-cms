// Package audit turns stored pages into seo.Input and runs seo.Analyze over
// them. It is the one place that knows how, so the editor's SEO tab, the
// report screen and the seo_report tool read a page the same way and cannot
// drift apart.
package audit

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/seo"
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
}

func (s Source) context(ctx context.Context, ws *domain.Website, all []page.Page) siteContext {
	c := siteContext{siteDescription: firstNonEmpty(ws.MetaDescription, ws.Description), set: block.Builtin}
	if s.BlockSet != nil {
		c.set = s.BlockSet(ctx, ws.ID)
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

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
