package admin

import (
	"net/http"
	"strings"

	"golang.org/x/net/html"

	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// BrokenLink is an internal link on a page that resolves to nothing.
type BrokenLink struct {
	// OnPage is where the link sits, which is the part that makes it fixable.
	OnPage     string
	OnPageID   int64
	Target     string
	OnPageSlug string
}

// checkInternalLinks walks every live page and reports links that go nowhere.
//
// It is a pure read pass with no schema and no egress: the stored HTML is
// parsed, same-origin hrefs are collected and resolved against the pages and
// redirects of this website.
//
// This stays in the core while the 404 list moved to a plugin, and the two are
// not the same thing: a broken link is a fact about the site's own content,
// visible without a single visitor, and fixing it is editing a page. The 404
// list is a record of what visitors did, which is exactly the kind of thing an
// operator should be able to decline to keep.
func (h *Handler) checkInternalLinks(r *http.Request, websiteID int64) ([]BrokenLink, error) {
	pages, _, err := h.pages.ListPages(r.Context(), websiteID,
		page.ListFilter{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, err
	}

	var broken []BrokenLink
	seen := map[string]bool{}
	for _, p := range pages {
		for _, target := range internalLinks(p.ContentHTML) {
			key := p.Slug + " " + target
			if seen[key] {
				continue
			}
			seen[key] = true

			ok, err := h.linkResolves(r, websiteID, target)
			if err != nil {
				return nil, err
			}
			if !ok {
				broken = append(broken, BrokenLink{
					OnPage: p.Title, OnPageID: p.ID, OnPageSlug: p.Slug, Target: target,
				})
			}
		}
	}
	return broken, nil
}

// linkResolves reports whether an internal path leads somewhere.
func (h *Handler) linkResolves(r *http.Request, websiteID int64, target string) (bool, error) {
	// The home page and the built-in routes always exist.
	switch target {
	case "/", "/sitemap.xml", "/robots.txt", "/feed.xml":
		return true, nil
	}
	// A path a plugin claims exists as long as the plugin is on. The search is
	// the usual case: it is a plugin now, so a link to it is only sound while
	// that plugin is installed and switched on for this site.
	if h.plugins != nil {
		if _, ok := h.plugins.RouteOwner(target, websiteID); ok {
			return true, nil
		}
	}
	// Media and template assets are checked by their own paths, not here.
	if strings.HasPrefix(target, "/media/") || strings.HasPrefix(target, "/t/") {
		return true, nil
	}

	slug := strings.TrimPrefix(target, "/")
	pg, err := h.pages.GetPageBySlug(r.Context(), websiteID, slug)
	if err != nil {
		return false, err
	}
	if pg != nil {
		return true, nil
	}
	// A redirect is a working link too.
	redirect, err := h.pages.LookupRedirect(r.Context(), websiteID, target)
	if err != nil {
		return false, err
	}
	return redirect != nil, nil
}

// internalLinks collects the same-origin <a href> targets of a document.
//
// A parser, not a regexp, for the same reason the media scan uses one: an
// attribute can be quoted three ways or spread over lines.
func internalLinks(pageHTML string) []string {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return nil
	}

	var out []string
	seen := map[string]bool{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key != "href" {
					continue
				}
				target := strings.TrimSpace(a.Val)
				// Only root-relative paths are ours to check. An absolute URL
				// points at someone else's site, and an anchor or a mailto: is
				// not a page at all.
				if !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") {
					continue
				}
				if i := strings.IndexAny(target, "?#"); i >= 0 {
					target = target[:i]
				}
				if target == "" || seen[target] {
					continue
				}
				seen[target] = true
				out = append(out, target)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}
