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
	// compose, when set, writes the Markdown from what the website uses and
	// the frameworks it follows at the moment it is created, and markdown
	// stays empty.
	compose func(privacy.Request) string
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
// Title and slug stay German, because a new website is German until its
// settings change; drafts in other languages are offered later. Only a
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
		title:   "Impressum",
		slug:    "impressum",
		status:  "draft",
		compose: privacy.ComposeImprint,
	},
	{
		title:   "Datenschutzerklärung",
		slug:    "datenschutz",
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
	created := make(map[string]int64, len(starterPages))

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

	for _, sp := range starterPages {
		markdown := sp.markdown
		if sp.compose != nil {
			markdown = sp.compose(req)
		}
		html, err := page.RenderMarkdown(markdown)
		if err != nil {
			slog.Error("render starter page", "err", err, "slug", sp.slug)
			continue
		}
		p, err := h.pages.CreatePage(ctx, page.PageCreate{
			WebsiteID: websiteID,
			Title:     sp.title,
			Slug:      sp.slug,
			Markdown:  markdown,
			HTML:      html,
			Status:    sp.status,
			Meta:      page.PageMeta{Excerpt: page.Excerpt(markdown)},
			UserID:    userID,
		})
		if err != nil {
			slog.Error("create starter page", "err", err, "slug", sp.slug)
			continue
		}
		created[sp.slug] = p.ID
	}

	h.createStarterMenu(ctx, websiteID, "Hauptmenü", "main", created, []string{"home"})
	// § 5 DDG wants the imprint reachable from every page; a footer menu is how
	// a theme does that.
	h.createStarterMenu(ctx, websiteID, "Fußzeile", "footer", created, []string{"impressum", "datenschutz"})
}

func (h *Handler) createStarterMenu(ctx context.Context, websiteID int64, name, location string, pages map[string]int64, slugs []string) {
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
	for i, slug := range slugs {
		id, ok := pages[slug]
		if !ok {
			continue
		}
		title := slug
		for _, sp := range starterPages {
			if sp.slug == slug {
				title = sp.title
			}
		}
		if _, err := h.menuStore.CreateItem(ctx, m.ID, nil, title, "page", "", &id, i); err != nil {
			slog.Error("create starter menu item", "err", err, "slug", slug)
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
