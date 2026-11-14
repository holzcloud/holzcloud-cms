package admin

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/privacy"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// starterPage is one of the pages a new website gets.
type starterPage struct {
	title    string
	slug     string
	status   string
	markdown string
	// kind, when set, makes the page a legal page: its title and slug are
	// those of that kind in the website's main language (privacy.Title,
	// privacy.Slug), and title and slug above stay empty.
	kind privacy.Kind
	// compose, when set, writes the Markdown from what the website uses and
	// the frameworks it follows at the moment it is created, and markdown
	// stays empty.
	compose func(privacy.Request) string
}

// titleSlug is the title and the slug the page gets on a website whose main
// language is tag.
func (sp starterPage) titleSlug(tag string) (title, slug string) {
	if sp.kind == "" {
		return sp.title, sp.slug
	}
	return privacy.Title(sp.kind, tag), privacy.Slug(sp.kind, tag)
}

// starterPages are what a brand-new website starts with.
//
// A site with no pages answers its own domain with "page not found",
// which is the first thing the owner sees after pointing DNS at it. The two
// legal pages are drafts: they are scaffolding to fill in, and publishing an
// empty imprint would be worse than having none.
//
// Both legal pages are composed when the website is created, from the same
// request: the website's main language, its spelling and the legal frameworks
// it follows (internal/privacy). The privacy page also follows the functions
// the website uses, so it never says the site sets no cookies while a shop on
// it does. The imprint follows only the frameworks (D-09): every website gets
// one, with the items of the EU, Swiss and UK rules when those are chosen and
// just who and how to reach otherwise. Both stay drafts for the reason above:
// the responsible body, the address and the processors are fill-in markers.
// Title and slug are those of the website's main language, the same ones the
// Legal pages card looks for (privacy.Title, privacy.Slug): "Impressum" at
// /impressum on a German website, "Mentions légales" at /mentions-legales on
// a French one. Drafts in the further languages are offered later. Only a
// brand-new website gets these pages; an existing imprint or privacy page is
// never touched (D-01).
var starterPages = []starterPage{
	{
		title:  "Startseite",
		slug:   "home",
		status: "published",
		markdown: `Willkommen!

Diese Startseite wurde automatisch angelegt. Klicke auf **Bearbeiten**, um sie
durch deinen eigenen Text zu ersetzen.
`,
	},
	{
		kind:    privacy.KindImprint,
		status:  "draft",
		compose: privacy.ComposeImprint,
	},
	{
		kind:    privacy.KindPrivacy,
		status:  "draft",
		compose: privacy.ComposePrivacy,
	},
}

// createStarterContent fills a new website with a published start page, the two
// legal drafts, a main menu and a footer menu linking them.
//
// Errors are logged rather than returned: the website itself is already
// created, and failing the request would leave the operator staring at an error
// for a site that does exist.
func (h *Handler) createStarterContent(ctx context.Context, websiteID int64, userID *int64) {
	// created holds the menu entry of each page by its place in starterPages.
	created := make(map[int]starterMenuEntry, len(starterPages))

	// A brand-new website uses none of the functions, so the zero Features are
	// the true answer when they cannot be read; the log says they could not.
	// The language and the frameworks are the website's own; a website that
	// cannot be read at all gets the German text and the default set, which is
	// what every new website starts with.
	req := privacy.Request{Lang: "de", Set: legal.Default()}
	if ws, err := h.domains.GetWebsite(ctx, websiteID); err != nil || ws == nil {
		slog.Error("read website for starter legal pages", "err", err, "website", websiteID)
	} else if req, err = h.legalRequest(ctx, ws, ws.Locale); err != nil {
		slog.Error("detect website functions for starter legal pages", "err", err, "website", websiteID)
	}

	for i, sp := range starterPages {
		title, slug := sp.titleSlug(req.Lang)
		markdown := sp.markdown
		if sp.compose != nil {
			markdown = sp.compose(req)
		}
		html, err := page.RenderMarkdown(markdown)
		if err != nil {
			slog.Error("render starter page", "err", err, "slug", slug)
			continue
		}
		p, err := h.pages.CreatePage(ctx, page.PageCreate{
			WebsiteID: websiteID,
			Title:     title,
			Slug:      slug,
			Markdown:  markdown,
			HTML:      html,
			Status:    sp.status,
			Meta:      page.PageMeta{Excerpt: page.Excerpt(markdown)},
			UserID:    userID,
		})
		if err != nil {
			slog.Error("create starter page", "err", err, "slug", slug)
			continue
		}
		created[i] = starterMenuEntry{id: p.ID, title: title}
	}

	// The start page goes into the main menu, the legal pages (the ones with
	// a kind) into the footer, in the order of starterPages.
	var main, footer []starterMenuEntry
	for i, sp := range starterPages {
		e, ok := created[i]
		if !ok {
			continue
		}
		if sp.kind == "" {
			main = append(main, e)
		} else {
			footer = append(footer, e)
		}
	}
	h.createStarterMenu(ctx, websiteID, "Hauptmenü", "main", main)
	// § 5 DDG wants the imprint reachable from every page; a footer menu is how
	// a theme does that.
	h.createStarterMenu(ctx, websiteID, "Fußzeile", "footer", footer)
}

// starterMenuEntry is a page a starter menu links, under the page's title.
type starterMenuEntry struct {
	id    int64
	title string
}

func (h *Handler) createStarterMenu(ctx context.Context, websiteID int64, name, location string, entries []starterMenuEntry) {
	if h.menuStore == nil {
		return
	}
	// The starter menus are in the website's main language: a brand-new website
	// has no other one yet.
	m, err := h.menuStore.CreateMenu(ctx, websiteID, name, location, "")
	if err != nil {
		slog.Error("create starter menu", "err", err, "location", location)
		return
	}
	for i, e := range entries {
		id := e.id
		if _, err := h.menuStore.CreateItem(ctx, m.ID, nil, e.title, "page", "", &id, i); err != nil {
			slog.Error("create starter menu item", "err", err, "page", id)
		}
	}
}

// starterContentSummary is what the flash message says after creating a site.
//
// The one sentence in this file addressed to the operator rather than written
// into a page, so the one that goes through the catalogue.
func starterContentSummary(r *http.Request) string {
	return web.Titlef(r, "Website created – with a start page plus an imprint and a privacy statement as drafts (%d pages)", len(starterPages))
}
