package public

import (
	"context"
	"errors"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// The host operations a public plugin may call.
//
// They live here rather than in internal/plugin because they need the page
// store and the theme loader, and the plugin package must not depend on either:
// a sandbox that imported half the application would be a sandbox in name only.
// The runtime takes them as functions, so what a plugin can reach is exactly
// what is handed over here and nothing that is added elsewhere later.

// requestKey carries the request into a plugin call.
//
// The theme needs it: which host the site answers on decides the canonical URL,
// and that is a property of the request, not of the website row.
type requestKey struct{}

func withRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, requestKey{}, r)
}

func requestFrom(ctx context.Context) *http.Request {
	r, _ := ctx.Value(requestKey{}).(*http.Request)
	return r
}

// SearchPath is the address the search answers on. A theme links to it and the
// search plugin claims it; the constant is here so the two cannot drift apart.
const SearchPath = "/suche"

// hasSearch reports whether anything answers the search address on this site.
func (h *Handler) hasSearch(websiteID int64) bool {
	if h.plugins == nil {
		return false
	}
	_, ok := h.plugins.RouteOwner(SearchPath, websiteID)
	return ok
}

// expandForPlugin resolves the markers a plugin must never see.
//
// It takes a context rather than a request because that is what the plugin
// host is handed; the request is fetched back out of it for the website's
// locale, exactly as RenderForPlugin does, and its absence is survivable — a
// gallery whose lightbox controls fall back to the default language is a page,
// where a raw marker is not.
func (h *Handler) expandForPlugin(ctx context.Context, websiteID int64, body string) string {
	if !strings.Contains(body, "[[") {
		return body
	}
	r := requestFrom(ctx)
	if r == nil {
		// No request means no locale and no album lookup. Expanding the
		// snippets alone is still strictly better than handing both markers
		// over, and album.Expand on a zero Set removes what it cannot resolve.
		if h.snippetStore != nil {
			if rendered, err := h.snippetStore.LoadRendered(ctx, websiteID); err == nil {
				body = snippet.Expand(body, rendered.HTML)
			}
		}
		return album.Expand(body, album.Set{})
	}
	body = snippet.Expand(body, h.loadSnippets(r, websiteID).HTML)
	return album.Expand(body, h.albumsFor(r, websiteID, body))
}

// PagesForPlugin answers the page operations.
//
// Published pages only, in every case. A plugin asking for a draft gets the
// same answer as a plugin asking for a page that does not exist, because the
// rule that an unfinished page is not public has to hold in the host: a module
// is written by someone else, and a check inside it is a promise, not a
// guarantee.
func (h *Handler) PagesForPlugin(ctx context.Context, websiteID int64, q plugin.PagesQuery) (plugin.PagesResult, error) {
	if h.pageStore == nil {
		return plugin.PagesResult{}, errors.New("die Seiten sind nicht verfügbar")
	}

	switch q.Op {
	case plugin.OpPagesGet:
		p, err := h.pageStore.GetPublishedPage(ctx, websiteID, strings.TrimPrefix(q.Slug, "/"))
		if err != nil {
			return plugin.PagesResult{}, err
		}
		// A protected page gets the same answer as one that does not exist.
		// GetPublishedPage is right for the page's own route, where the gate
		// stands in front of the body and the unlock cookie decides — and this
		// path has neither. Asking the plugin to check would be the promise the
		// doc comment above says is not a guarantee.
		if p == nil || p.Protected() {
			return plugin.PagesResult{}, nil
		}
		info := pageInfo(*p)
		// Expanded, not handed over as it stands. `content_html` is the
		// FROZEN body and not what a visitor is served: a snippet marker has
		// survived into it since Phase 8 and an album marker since Phase 11,
		// and both are resolved per request. A plugin that printed this column
		// would show `[[album:sommer:0]]` in the middle of an article — and
		// leak the album's internal address with it.
		//
		// The same order and the same reason as pageContent and the feed:
		// snippets first, because a snippet's body may itself carry a gallery
		// marker; then the albums over the result.
		//
		// The feed's own comment already made this argument for a subscriber —
		// "so a subscriber sees the same text as a visitor rather than the raw
		// marker". A plugin is a reader too, and it was the last one left.
		info.HTML = h.expandForPlugin(ctx, websiteID, p.ContentHTML)
		h.addFields(&info, *p, q.WithFields)
		return plugin.PagesResult{Pages: []plugin.PageInfo{info}, Total: 1}, nil

	case plugin.OpPagesSearch:
		results, err := h.pageStore.SearchPages(ctx, websiteID, q.Query, false, q.Limit)
		if err != nil {
			return plugin.PagesResult{}, err
		}
		out := plugin.PagesResult{Pages: make([]plugin.PageInfo, 0, len(results))}
		for _, res := range results {
			info := pageInfo(res.Page)
			// The snippet is built from escaped text in internal/page, with
			// <mark> around the terms. It is passed on as it is; what the
			// plugin does with it goes through the sanitiser on the way back.
			info.Snippet = string(res.Snippet)
			out.Pages = append(out.Pages, info)
		}
		out.Total = len(out.Pages)
		return out, nil

	case plugin.OpPagesList:
		// ListPublic and not ListPages: the admin list filters on the status
		// column and on nothing else, so a page whose publication date is still
		// ahead of it, whose window has closed, or which sits behind a password
		// came back as published. See page.ListPublic.
		pages, total, err := h.pageStore.ListPublic(ctx, websiteID, q.PostsOnly, q.Limit, q.Offset)
		if err != nil {
			return plugin.PagesResult{}, err
		}
		out := plugin.PagesResult{Pages: make([]plugin.PageInfo, 0, len(pages)), Total: total}
		for _, p := range pages {
			info := pageInfo(p)
			h.addFields(&info, p, q.WithFields)
			out.Pages = append(out.Pages, info)
		}
		return out, nil
	}
	return plugin.PagesResult{}, fmt.Errorf("unbekannte Operation %q", q.Op)
}

// pageInfo is the part of a page a plugin sees. The body is left out on
// purpose: a list of a hundred pages with their HTML would be the whole site in
// one payload, copied into a sandbox that has sixteen megabytes in total.
func pageInfo(p page.Page) plugin.PageInfo {
	info := plugin.PageInfo{
		ID: p.ID, Slug: p.Slug, Title: p.Title,
		Excerpt: p.Excerpt, IsPost: p.IsPost(),
	}
	if p.PublishedAt != nil {
		info.PublishedAt = p.PublishedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return info
}

// addFields attaches the website's own fields, when they were asked for.
//
// As stored, not resolved: a plugin gets the media id and the typed number,
// because turning them into pictures and floats is what a theme does and a
// plugin has no theme.
func (h *Handler) addFields(info *plugin.PageInfo, p page.Page, want bool) {
	if !want || p.Fields == "" {
		return
	}
	data := field.Decode(p.Fields)
	if len(data.Values) > 0 {
		info.Fields = data.Values
	}
	if len(data.Rows) > 0 {
		info.Groups = make(map[string][]map[string]string, len(data.Rows))
		for key, rows := range data.Rows {
			out := make([]map[string]string, 0, len(rows))
			for _, row := range rows {
				out = append(out, row)
			}
			info.Groups[key] = out
		}
	}
}

// RenderForPlugin draws a plugin's page in the website's own theme.
//
// This is what keeps a plugin from looking like a foreign body: the plugin
// supplies a title and a piece of markup, and gets back a page with the site's
// header, menus, stylesheet and footer. It never sees the theme, so a plugin
// cannot break one, and a theme needs to know nothing about plugins.
func (h *Handler) RenderForPlugin(ctx context.Context, websiteID int64, a plugin.RenderArg) (string, error) {
	r := requestFrom(ctx)
	if r == nil {
		// Reachable from the admin hook, where there is no visitor and no site
		// to draw. Saying so beats returning a page nobody asked for.
		return "", errors.New("es wird gerade keine öffentliche Seite ausgeliefert")
	}
	website := domain.WebsiteFromContext(r.Context())
	if website == nil || website.ID != websiteID {
		return "", errors.New("die Website ist nicht bekannt")
	}

	site := h.siteData(r, website)
	snippets := h.loadSnippets(r, websiteID)
	h.fillSnippets(r, &site, websiteID, snippets)

	data := tmpl.PageData{
		Site: site,
		Page: tmpl.PageContent{
			Title: a.Title,
			Slug:  strings.TrimPrefix(a.Slug, "/"),
			// Sanitised with the same policy as page content. A plugin runs on
			// the operator's server, but its output is read by visitors, and a
			// script there would run in the site's own origin.
			ContentHTML: template.HTML(page.SanitizeHTML(a.HTML)),
		},
		Menus: h.loadMenus(r, websiteID),
		Meta: tmpl.MetaData{
			CanonicalURL: site.URL + r.URL.Path,
			Description:  a.Description,
			NoIndex:      a.NoIndex,
		},
	}
	if a.View == plugin.ViewSearch && a.Search != nil {
		data.Search = searchData(*a.Search)
	}

	view := a.View
	if view == "" {
		view = plugin.ViewPage
	}
	out, err := h.loader.RenderPage(ctx, websiteID, view, data)
	if err != nil {
		return "", fmt.Errorf("the view %q cannot be rendered: %w", view, err)
	}
	return string(out), nil
}

// NotifyForPlugin queues one notification to the operator of a website.
//
// The recipient comes from the website's settings and never from the plugin. A
// plugin that could name an address would be a mail relay reachable from the
// public internet, and the first thing that found it would use it to send other
// people's spam.
//
// Nowhere to send is not a failure. An installation with no mail server, or a
// website whose operator left the notification field empty, has said what it
// wants; a plugin that treated that as an error would fill the log with a
// complaint about a decision somebody made on purpose.
func (h *Handler) NotifyForPlugin(ctx context.Context, websiteID int64, a plugin.NotifyArg) (bool, string, error) {
	if h.mail == nil || !h.mail.Enabled() {
		return false, "es ist kein Mailserver eingerichtet", nil
	}
	if h.domains == nil {
		return false, "die Website ist nicht bekannt", nil
	}
	ws, err := h.domains.GetWebsite(ctx, websiteID)
	if err != nil {
		return false, "", err
	}
	if ws == nil {
		return false, "die Website ist nicht bekannt", nil
	}
	if ws.NotifyEmail == "" {
		return false, "für diese Website ist keine Benachrichtigungsadresse hinterlegt", nil
	}

	subject := strings.TrimSpace(a.Subject)
	if subject == "" {
		subject = "Neue Meldung von " + ws.Name
	}
	msg := mail.Message{
		To: ws.NotifyEmail,
		// The site's name goes in front, because an operator running four
		// websites gets four kinds of notification into one inbox.
		Subject: "[" + ws.Name + "] " + subject,
		Body:    a.Body,
		ReplyTo: a.ReplyTo,
	}
	if err := h.mail.Enqueue(ctx, websiteID, msg); err != nil {
		return false, "", err
	}
	return true, "", nil
}

// searchData turns a plugin's result list into the theme's.
func searchData(s plugin.RenderSearch) tmpl.SearchData {
	out := tmpl.SearchData{Query: s.Query, Submitted: s.Submitted}
	for _, hit := range s.Results {
		out.Results = append(out.Results, tmpl.SearchHit{
			Title:   hit.Title,
			URL:     hit.URL,
			Snippet: markOnly(hit.Snippet),
		})
	}
	return out
}

// markOnly escapes a snippet and puts back only <mark>.
//
// A result row is the one place a plugin hands over a fragment of somebody
// else's text, and the only markup it needs there is the highlight. Escaping
// everything and restoring one element is the narrow way round: an allow list
// of one, rather than a sanitiser that has to be right about everything.
func markOnly(s string) template.HTML {
	escaped := html.EscapeString(s)
	escaped = strings.ReplaceAll(escaped, "&lt;mark&gt;", "<mark>")
	escaped = strings.ReplaceAll(escaped, "&lt;/mark&gt;", "</mark>")
	return template.HTML(escaped)
}
