package public

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
	"github.com/holzcloud/holzcloud-cms/internal/value"
)

// pageContent maps a stored page onto what a theme sees, and returns beside it
// when the albums and the values that page names last changed.
//
// Snippet markers are expanded here, at render time. Baking the expansion into
// content_html on save would freeze a copy of the opening hours into every
// page, which is the thing snippets exist to prevent.
//
// The responsive rewrite happens at the same moment and for the same reason:
// regenerating an image's scaled copies, or adding the pipeline to a site that
// already has content, must reach the pages that were written before it.
//
// The second return value is a caching validator and not content, and it comes
// back from here because here is where it is known: the albums are loaded while
// the body is expanded, and asking a second time on the way to serveCached
// would be a second query on every public request. A caller that does not cache
// its answer — the share-link preview, which is no-store — discards it.
func (h *Handler) pageContent(r *http.Request, websiteID int64, pg *page.Page, snippets snippet.Rendered) (tmpl.PageContent, time.Time) {
	updated := pg.UpdatedAt
	body := snippet.Expand(pg.ContentHTML, snippets.HTML)
	// The albums, and all three halves of where this call stands are decisions.
	//
	// AFTER the snippets, because a snippet could itself contain a gallery
	// marker and the expansion should see it.
	//
	// BEFORE the plugins and therefore before the responsive rewrite at the end
	// of this function, because the <img> tags this expansion produces must
	// still be seen by media.MakeResponsive. An expansion placed after that
	// call would serve album pictures with no srcset at all, and nothing
	// anywhere would report it. The plugin filter's own comment below says it
	// runs last so that it sees the page as a visitor would, which is the same
	// argument from the other end.
	//
	// UNCONDITIONALLY, including when the albums could not be loaded and when
	// no album store is wired at all. albumsFor answers the zero Set in both
	// cases and the zero Set expands every marker to nothing, which is what
	// loadSnippets and snippet.Expand have always done between them. The
	// alternative — skip the expansion and serve what is stored — printed
	// [[album:moebel:0]] to the visitor on every request with no recovery.
	// A document naming no album costs nothing here: LoadFor reads the slugs
	// out of the HTML first and Expand returns a marker-free body unchanged.
	set := h.albumsFor(r, websiteID, body)
	albumsAt := set.Latest()
	body = album.Expand(body, set)
	// The words this program mints, in the language of THIS PAGE.
	//
	// After the albums, because an album gallery's wrapper and controls are
	// markers this pass has to see. Before the plugins, for the reason their
	// own comment gives: a filter sees the page as a visitor would, and a
	// visitor sees words.
	//
	// i18n.Lang is the page's language here and not the reader's browser's —
	// LocaleMiddleware put it there, and its comment says why: "the language
	// belongs to the page, not to whoever is reading it".
	body = block.ResolveWords(body, pageWords(r.Context()))
	// The website's values, and again where this stands is the decision.
	//
	// AFTER the snippets and the albums, so a [[wert:…]] a snippet carries —
	// the footer that names the version — is resolved with the page's own.
	//
	// BEFORE the plugins and the responsive rewrite, for the reason both give:
	// a filter sees the page as a visitor would, and media.MakeResponsive must
	// see the final address of a picture, not a marker standing in it.
	//
	// HERE, at delivery, and never at save: VAL-04 is that changing a value
	// changes every page that names it without touching any of them, and a
	// text frozen into content_html would wait for the next save of each.
	//
	// And its time comes back beside the albums', because a browser that holds
	// only a date would otherwise be told 304 about a page whose value moved —
	// contentModTime's own comment tells that story for the albums.
	//
	// The page's summary in the same pass and from the same load: the excerpt
	// is the page's text in short and keeps its markers for this moment
	// (page.Excerpt). It is written back onto pg, which is this request's own
	// row and nobody else's, because everything after this reads it there —
	// the meta description, og:description and the JSON-LD through metaData,
	// the share-link preview's description. One resolution reaches them all,
	// and none of them can be the one that forgot. MetaDescription is not
	// touched: an editor's typed description stays as typed (D-11).
	sum := h.summaries(r.Context(), websiteID)
	if value.Has(body) {
		sum.load()
		body = value.Resolve(body, sum.texts)
	}
	pg.Excerpt = sum.resolve(pg.Excerpt)
	valuesAt := sum.at
	if valuesAt.After(albumsAt) {
		albumsAt = valuesAt
	}
	// The opt-out widget of a website that follows US law, and once more
	// where this stands is the decision.
	//
	// AFTER the values, so the page around the widget is the page a visitor
	// reads. A value cannot carry the marker in through the back door, and
	// not because it is escaped — escaping changes no character of
	// "[[widerspruch]]" — but because a value's text holds no bracket at all:
	// value.CleanText refuses one, and the CHECK of migration 00062 refuses it
	// under that (TestAValueCannotCarryTheWidget).
	//
	// BEFORE the plugins and the responsive rewrite, for the reason both give:
	// a filter sees the page as a visitor would, form included.
	//
	// HERE, at delivery, because its first line answers this request — whether
	// the browser sent the Global Privacy Control signal — and a widget frozen
	// into content_html could only ever give one of the two answers.
	body = h.expandOptOut(r, domain.WebsiteFromContext(r.Context()), pg, body)
	// The request form of any website, in the same slot for the same reasons.
	body = h.expandDataRequest(r, domain.WebsiteFromContext(r.Context()), pg, body)
	// Plugins last: they see the page as a visitor would, with the snippets
	// already in place and the form already drawn. A filter that ran earlier
	// would be filtering markers instead of text.
	body = h.filterByPlugins(r, websiteID, pg, body)
	fields, liste := h.ownFields(r, websiteID, pg)
	return tmpl.PageContent{
		Title:         pg.Title,
		ContentHTML:   template.HTML(h.responsive(r, websiteID, body)),
		Slug:          pg.Slug,
		PublishedAt:   pg.PublishedAt,
		UpdatedAt:     &updated,
		Excerpt:       pg.Excerpt,
		HasOwnHeading: startsWithHeading(pg.ContentHTML),
		Kind:          pg.TypeKey,
		Terms:         termLinksAt(localePrefixOf(r), h.labelsForPage(r, pg.ID)),
		Fields:        fields,
		FieldList:     liste,
	}, albumsAt
}

// resolveValues puts the website's values into a body that names them, and
// returns the newest updated_at among them as a caching validator.
//
// A body without the prefix costs one strings.Contains: no query, no
// allocation, the zero time. That is every page that does not use a value, and
// it is why the texts are loaded here and not before.
//
// Without a store, or when the texts cannot be read, every marker renders as
// nothing — the outcome a key nobody defined has. Serving the body as stored
// would print [[wert:…]] to every visitor until somebody noticed.
func (h *Handler) resolveValues(r *http.Request, websiteID int64, body string) (string, time.Time) {
	if !value.Has(body) {
		return body, time.Time{}
	}
	texts, latest := h.valueTexts(r.Context(), websiteID)
	return value.Resolve(body, texts), latest
}

// summaryValues puts one website's values into page summaries, loading the
// values at most once and only when a summary names one.
//
// A summary is a page's excerpt: derived from its text at save, it keeps a
// value marker as it stands (page.Excerpt), and every place that hands an
// excerpt out — the page's head, an archive, the feed, a plugin, llms.txt —
// asks this to put the value in. A list of twenty excerpts without a marker
// costs twenty strings.Contains and no query.
//
// at is the newest stamp among the values once they were loaded, and the zero
// time when they were not: a caller that caches folds it into its validator,
// because a summary that names a value changes when the value does.
type summaryValues struct {
	h         *Handler
	ctx       context.Context
	websiteID int64
	loaded    bool
	texts     map[string]string
	at        time.Time
}

func (h *Handler) summaries(ctx context.Context, websiteID int64) *summaryValues {
	return &summaryValues{h: h, ctx: ctx, websiteID: websiteID}
}

func (s *summaryValues) load() {
	if s.loaded {
		return
	}
	s.loaded = true
	s.texts, s.at = s.h.valueTexts(s.ctx, s.websiteID)
}

// resolve is the summary with its values in, as plain text for the caller's
// template or encoder to escape. A key the website does not have is nothing.
func (s *summaryValues) resolve(excerpt string) string {
	if !value.Has(excerpt) {
		return excerpt
	}
	s.load()
	return value.ResolveSummary(excerpt, s.texts)
}

// resolveSummaries resolves the excerpt of every page in the list in place and
// returns the values' stamp for the list's validator.
func (h *Handler) resolveSummaries(ctx context.Context, websiteID int64, pages []page.Page) time.Time {
	sum := h.summaries(ctx, websiteID)
	for i := range pages {
		pages[i].Excerpt = sum.resolve(pages[i].Excerpt)
	}
	return sum.at
}

// anySummaryNamesAValue reports whether the excerpt of any page in the list
// carries a value marker — for a caller that loads the values itself.
func anySummaryNamesAValue(pages []page.Page) bool {
	for i := range pages {
		if value.Has(pages[i].Excerpt) {
			return true
		}
	}
	return false
}

// ownFields resolves the website's own fields for a theme.
//
// Resolved on the way out rather than stored resolved: a picture chosen last
// month has to pick up this month's crop, and a field whose definition changed
// has to be read the new way without every page being saved again.
func (h *Handler) ownFields(r *http.Request, websiteID int64, pg *page.Page) (map[string]any, []field.Entry) {
	if h.fieldStore == nil {
		return nil, nil
	}
	defs, err := h.fieldStore.List(r.Context(), websiteID)
	if err != nil {
		slog.Error("load page fields", "err", err, "website", websiteID)
		return nil, nil
	}
	if len(defs) == 0 {
		return nil, nil
	}
	mine := field.For(defs, pg.KindValue())
	data := field.Decode(pg.Fields)
	links := field.Links{
		Image: h.fieldImages(r, websiteID),
		Page:  h.fieldRefs(r, websiteID),
		Term:  h.fieldTerms(r, websiteID),
	}
	return field.Resolve(mine, data, links), field.List(mine, data, links)
}

// fillSnippets writes all three snippet members of SiteData.
//
// The only place in the tree where Snippets, SnippetFields and SnippetList are
// written onto a SiteData, and that is not tidiness: the assignment stood in
// fourteen places in ten files, and a member that is filled in thirteen of them
// is invisible in the fourteenth — the page appears, the theme prints nothing
// in one place, nothing is logged, and nobody learns of it but a visitor. Hence
// one function and fourteen calls, and hence a gate in the plan that shows no
// assignment survived outside this function.
//
// What that gate cannot see is the opposite gap: a route that does neither the
// one nor the other. Three did — renderNotFound, serveShareError and
// HandleMaintenance — and they were noticed nowhere, because index over a nil
// map gives the empty string and range over one takes no turn. Against that
// stands no count but TestSnippetFieldsOnTheRoutesWithoutAPage: it draws the
// three views and reads the value out of the body.
//
// Resolved on the way out rather than stored resolved: a picture chosen last
// month has to pick up this month's crop, and a field whose definition changed
// has to be read the new way without every snippet being saved again.
func (h *Handler) fillSnippets(r *http.Request, site *tmpl.SiteData, websiteID int64, rendered snippet.Rendered) {
	site.Snippets = rendered.HTML
	site.SnippetFields = map[string]map[string]any{}
	site.SnippetList = map[string][]field.Entry{}
	if h.fieldStore == nil {
		return
	}
	// A website without a single snippet pays for no query: it builds its pages
	// as it did before this phase, only with three empty maps instead of three
	// that do not exist.
	if len(rendered.IDs) == 0 {
		return
	}
	// One query for all snippets of this website and not one per snippet: this
	// runs on every public build of a page, and a website with five snippets
	// would otherwise pay five round trips.
	alle, err := h.fieldStore.OfSnippets(r.Context(), websiteID)
	if err != nil {
		slog.Error("load snippet fields", "err", err, "website", websiteID)
		return
	}
	links := field.Links{
		Image: h.fieldImages(r, websiteID),
		Page:  h.fieldRefs(r, websiteID),
		Term:  h.fieldTerms(r, websiteID),
	}
	for key, snippetID := range rendered.IDs {
		defs := alle[snippetID]
		data := field.Decode(rendered.Fields[key])
		// A snippet without a single definition gets its entry too — an empty
		// map and not a missing key: a theme that writes
		// {{ index .Site.SnippetFields "kontakt" "telefon" }} should print
		// nothing, rather than fail, on a website where nobody has created a
		// field yet.
		site.SnippetFields[key] = field.Resolve(defs, data, links)
		// Entered only when something is really filled: field.List hands out no
		// empty entry, and a key behind which an empty list stands would be
		// indistinguishable from a filled one to a theme.
		if liste := field.List(defs, data, links); len(liste) > 0 {
			site.SnippetList[key] = liste
		}
	}
}

// fieldRefs resolves a reference field for the website being rendered.
//
// Three conditions, and the middle one is the one that matters: the page has to
// exist, it has to be **published**, and it has to belong to this website. A
// draft that somebody referenced is not a page a visitor may see, and a
// reference is exactly the kind of side door through which one would otherwise
// slip out — the id is in the database, nobody looks at it again, and the title
// of an unannounced product appears on the public site.
func (h *Handler) fieldRefs(r *http.Request, websiteID int64) field.RefLookup {
	cache := map[int64]field.Ref{}
	return func(id int64) (field.Ref, bool) {
		if id <= 0 || h.pageStore == nil {
			return field.Ref{}, false
		}
		if ref, ok := cache[id]; ok {
			return ref, ref.URL != ""
		}
		target, err := h.pageStore.GetPage(r.Context(), id)
		if err != nil || target == nil || target.WebsiteID != websiteID ||
			!target.PubliclyVisible() || target.Protected() {
			cache[id] = field.Ref{}
			return field.Ref{}, false
		}
		// The target's own language decides the prefix, not the language being
		// rendered: a German page may well point at the French one, and
		// /kontakt under /fr/ would be the wrong address.
		prefix := ""
		if target.Locale != "" {
			prefix = "/" + target.Locale
		}
		ref := field.Ref{
			Title: target.Title,
			URL:   prefix + "/" + target.Slug,
			Kind:  target.Kind,
		}
		cache[id] = ref
		return ref, true
	}
}

// fieldImages resolves a picture field for the website being rendered.
func (h *Handler) fieldImages(r *http.Request, websiteID int64) field.Lookup {
	cache := map[int64]field.Image{}
	return func(id int64) (field.Image, bool) {
		if id <= 0 || h.mediaStore == nil {
			return field.Image{}, false
		}
		if img, ok := cache[id]; ok {
			return img, img.URL != ""
		}
		m, err := h.mediaStore.GetByID(r.Context(), id)
		if err != nil || m == nil || m.WebsiteID != websiteID {
			cache[id] = field.Image{}
			return field.Image{}, false
		}
		img := field.Image{URL: m.URL(), Alt: m.AltText, Width: m.Width, Height: m.Height, Focus: m.FocusCSS()}
		cache[id] = img
		return img, true
	}
}

// fieldTerms resolves a label field for the website being rendered.
//
// The whole map is filled from one query the first time a label is asked for,
// rather than one query per label: a page carrying ten label fields is a page
// with ten values, not ten round trips — the same shape fieldRefs and
// fieldImages already have.
//
// That single ListAll is also the website rule, and the only place it can be.
// A stored slug says nothing about which website it belongs to, so a slug from
// another one is simply not in this map and Resolve yields its typed nil.
//
// The comparison is exact string equality. No case folding and no
// normalisation: a slug is already the normalised spelling of a name, and
// folding a second time here would let two different labels collide.
func (h *Handler) fieldTerms(r *http.Request, websiteID int64) field.TermLookup {
	var bySlug map[string]field.Term
	prefix := localePrefixOf(r)
	return func(slug string) (field.Term, bool) {
		if slug == "" || h.termStore == nil {
			return field.Term{}, false
		}
		if bySlug == nil {
			bySlug = map[string]field.Term{}
			list, err := h.termStore.ListAll(r.Context(), websiteID)
			if err != nil {
				slog.Error("load website terms", "err", err, "website", websiteID)
			}
			for _, t := range list {
				// The same address with the same language prefix the term links
				// get on the rest of the page. The address itself comes from
				// Term.URL and is therefore not written out a second time here
				// — two spellings of the same address drift apart eventually.
				bySlug[t.Slug] = field.Term{Name: t.Name, Slug: t.Slug, URL: prefix + t.URL()}
			}
		}
		t, ok := bySlug[slug]
		return t, ok
	}
}

// labelsForPage loads one page's labels, or none when the store is absent.
func (h *Handler) labelsForPage(r *http.Request, pageID int64) []term.Term {
	if h.termStore == nil {
		return nil
	}
	terms, err := h.termStore.ForPage(r.Context(), pageID)
	if err != nil {
		slog.Error("load page terms", "err", err, "page", pageID)
		return nil
	}
	return terms
}

// filterByPlugins hands the page to the content filters, if there are any.
//
// The check comes first so a site without plugins pays nothing: assembling a
// copy of every page's HTML to hand to nobody is exactly the sort of cost that
// only shows up under load.
func (h *Handler) filterByPlugins(r *http.Request, websiteID int64, pg *page.Page, body string) string {
	if h.plugins == nil || !h.plugins.Active(plugin.HookContent, websiteID) {
		return body
	}
	return h.plugins.FilterContent(withRequest(r.Context(), r), websiteID, plugin.ContentIn{
		WebsiteID: websiteID,
		Slug:      pg.Slug,
		Title:     pg.Title,
		HTML:      body,
		// The query goes along: a filter that draws a form has to be able to
		// show the outcome of the last submission, which comes back in the
		// address and nowhere else.
		Query: r.URL.RawQuery,
		// And the address itself, which is where a form that submits to itself
		// has to point. By this point it is the right one: /home has already
		// been redirected to the root and the language prefix is in it.
		Path: r.URL.Path,
		// And the language the page is in, which is the one a plugin has to
		// answer this visitor in. Not the website's first language: that is
		// how v2.0 came to answer German visitors in English.
		Lang: i18n.Lang(r.Context()),
	})
}

// responsive turns plain <img> tags into a srcset, or leaves the HTML alone.
//
// A failure is logged and ignored: a page that shows full-size images is a
// slower page, while a page that fails to render is no page at all.
func (h *Handler) responsive(r *http.Request, websiteID int64, body string) string {
	if h.mediaStore == nil {
		return body
	}
	idx, err := h.mediaStore.LoadImageSets(r.Context(), websiteID, body)
	if err != nil {
		slog.Error("load image sets", "err", err, "website", websiteID)
		return body
	}
	return media.MakeResponsive(body, idx)
}

// loadSnippets fetches the expansion map for a website.
//
// The three maps are created in both fallback cases as well and are never nil:
// a caller should not have to know whether it holds the path that worked or the
// one that did not.
func (h *Handler) loadSnippets(r *http.Request, websiteID int64) snippet.Rendered {
	if h.snippetStore == nil {
		return emptyBlocks()
	}
	rendered, err := h.snippetStore.LoadRendered(r.Context(), websiteID)
	if err != nil {
		slog.Error("load snippets", "err", err, "website", websiteID)
		return emptyBlocks()
	}
	h.resolveSnippetValues(r, websiteID, &rendered)
	return rendered
}

// resolveSnippetValues puts the website's values into every snippet that names
// one, and folds their newest time into the snippets' own.
//
// Here and not only in pageContent, because a theme prints snippets on every
// route through .Site.Snippets — a footer that names the version is the
// realistic case, and it stands on the 404 page and the maintenance page as
// much as on an article. pageContent's snippet.Expand then inserts snippets
// whose values are already resolved, and its own value pass finds nothing left
// in them.
//
// The time goes into LatestUpdate because that is what every validator built
// from the snippets already reads: contentModTime on the page path, the feed's
// newest. A value change that moved only the text would leave a browser holding
// a date with the old footer.
//
// The same rules as resolveValues: no query when no snippet carries the prefix,
// and without a store or on a load error every marker renders as nothing.
func (h *Handler) resolveSnippetValues(r *http.Request, websiteID int64, rendered *snippet.Rendered) {
	named := false
	for _, html := range rendered.HTML {
		if value.Has(string(html)) {
			named = true
			break
		}
	}
	if !named {
		return
	}
	texts, latest := h.valueTexts(r.Context(), websiteID)
	for key, html := range rendered.HTML {
		if value.Has(string(html)) {
			// The cast keeps the promise LoadRendered's own makes: the input
			// went through goldmark and bluemonday, and value.Resolve escapes
			// every text it puts in.
			rendered.HTML[key] = template.HTML(value.Resolve(string(html), texts))
		}
	}
	if latest.After(rendered.LatestUpdate) {
		rendered.LatestUpdate = latest
	}
}

// valueTexts loads the website's values for one resolution, and never fails.
//
// Without a store, or when the texts cannot be read, it answers the empty map
// and the zero time: every marker then renders as nothing, which is what a key
// nobody defined does, and is better than printing [[wert:…]] to a reader.
func (h *Handler) valueTexts(ctx context.Context, websiteID int64) (map[string]string, time.Time) {
	if h.valueStore == nil {
		return nil, time.Time{}
	}
	texts, latest, err := h.valueStore.Texts(ctx, websiteID)
	if err != nil {
		slog.Error("load values", "err", err, "website", websiteID)
		return nil, time.Time{}
	}
	return texts, latest
}

func emptyBlocks() snippet.Rendered {
	return snippet.Rendered{
		HTML:   map[string]template.HTML{},
		Fields: map[string]string{},
		IDs:    map[string]int64{},
	}
}

// albumsFor is the album expansion set for one document, and it never fails.
//
// All three callers — the page, the feed and the admin preview — want the same
// thing when the albums cannot be loaded, and it is the zero Set: it expands
// every marker to nothing, and a gallery that is not there is better than a
// bracketed token a reader can see. loadSnippets above has had exactly this
// shape since it was written and snippet.Expand runs on its empty map for
// exactly this reason.
//
// # Why not "leave the body as it is"
//
// Because that is not the safe half of the choice, which is how it was written
// until the Phase 11 review argued the other way and was right. Leaving the
// marker costs the PAGE: [[album:<slug>:<n>]] stands in the middle of an
// article, on every gallery the website has, on every request, with no
// recovery — and it leaks the album's internal address besides. Expanding to
// nothing costs the BLOCK, which is one gallery missing from a page that still
// reads. "A failing album must cost its own block and never the page" was the
// right rule pointed the wrong way.
//
// The h.responsive analogy that used to be offered for it does not hold either:
// a body that missed the responsive rewrite is still a page a visitor can read,
// it merely has no srcset. A body that missed this one is not.
//
// Nil-safe on the store, because a build without albums wired must still serve
// a page whose HTML was written when they were.
//
// No translator any more, and that is window 34 closed by subtraction. It used
// to build one from the WEBSITE's locale, matching what the save-time render
// froze, so that an inline gallery and an album gallery on one page could not
// answer in two languages. They could not — and both answered in the website's
// language on a page published in another one. Nothing here translates now:
// every word the renderer mints is a marker, and pageContent resolves all of
// them together in the page's language. See internal/block/words.go.
func (h *Handler) albumsFor(r *http.Request, websiteID int64, body string) album.Set {
	if h.albumStore == nil {
		return album.Set{}
	}
	set, err := h.albumStore.LoadFor(r.Context(), websiteID, body)
	if err != nil {
		slog.Error("load albums for page", "err", err, "website", websiteID)
		return album.Set{}
	}
	return set
}

// expandForFeed expands the markers in feed content, so a subscriber sees the
// same text as a visitor rather than the raw marker.
//
// Snippets AND albums. It used to be snippets alone, and the album marker
// therefore shipped verbatim into every subscriber's reader —
// [[album:moebel:0]], which is both unreadable and a leak of the album's
// internal address. internal/album/expand.go states the rule from the other
// side: "a visitor must not see the internal syntax on a live page", and a feed
// is a live public route.
//
// The order is the page's order and for the page's reason: a snippet may itself
// carry a gallery marker, so the albums are expanded into what the snippets
// left behind.
//
// What the feed still does NOT do, said out loud: it does not run the plugin
// filter or the responsive rewrite. Both predate this phase and neither leaves
// anything unreadable in the output; a feed picture without a srcset is a
// picture.
// words is the feed's language, and a feed has exactly one: HandleFeed lists
// only the pages published in it, because "a reader who subscribed to the
// French feed did not ask for German articles in among them".
//
// texts are the website's values, last and for the page's reason: a subscriber
// reads the version number where a visitor reads it, and the snippets arrive
// here with theirs already resolved by loadSnippets. A nil map renders every
// marker as nothing, never as the marker.
func expandForFeed(html string, snippets snippet.Rendered, albums album.Set, words func(string) string, texts map[string]string) string {
	// The opt-out marker goes without a widget: a feed reader is not a
	// browser that sent a signal, and a form in a feed posts nowhere useful.
	return withoutOptOut(value.Resolve(block.ResolveWords(album.Expand(snippet.Expand(html, snippets.HTML), albums), words), texts))
}

// contentModTime is the validator for conditional requests.
//
// It has to account for the snippets, not just the page: editing the opening
// hours changes what the page renders without touching pages.updated_at, and a
// browser holding an If-Modified-Since would keep the old text indefinitely.
//
// And, since Phase 11, for the albums, which are a second source of exactly the
// same kind and were the same sentence come true. Adding a picture to an album
// changes every page that names it and touches none of them — that is GAL-03 —
// so pages.updated_at says "unchanged" about a page whose gallery is not. A
// browser that had been on the site before was answered 304 and kept the old
// gallery, and every later request repeated the answer.
//
// albums is album.Set.Latest, the newest updated_at among the albums this page
// names, and the zero time for a page that names none.
//
// Where it is still short, said out loud rather than left to be found: an album
// that is DELETED takes its row and its stamp with it, so this value can go
// down. What holds that case is the strong ETag — serveCached now lets a
// present-and-mismatching If-None-Match end the negotiation, per RFC 7232 §3.3,
// which is what every browser sends. This is the validator for the caches that
// send only a date.
func contentModTime(pg *page.Page, snippets snippet.Rendered, albums time.Time) time.Time {
	newest := pg.UpdatedAt
	if snippets.LatestUpdate.After(newest) {
		newest = snippets.LatestUpdate
	}
	if albums.After(newest) {
		newest = albums
	}
	return newest
}

// startsWithHeading reports whether rendered content opens with an <h1>.
//
// The check is on the sanitised HTML rather than the Markdown source, so it also
// catches a page that begins with a raw <h1> block.
func startsWithHeading(html string) bool {
	trimmed := strings.TrimLeftFunc(html, unicode.IsSpace)
	return strings.HasPrefix(strings.ToLower(trimmed), "<h1")
}

// metaData assembles the head metadata for one page.
//
// The description falls back page → excerpt → site, so a theme that emits
// <meta name="description"> always has something to say rather than an empty
// attribute, which search engines treat worse than none.
func metaData(site tmpl.SiteData, pg *page.Page, path string) tmpl.MetaData {
	meta := tmpl.MetaData{
		CanonicalURL: site.URL + path,
		Description:  firstNonEmpty(site.MetaDescription, site.Description),
	}
	// Not every view has a page behind it. The catalogue and a product are
	// their own kind of thing, and passing nil used to panic here — a fault
	// that waits for the first view that is not a page and then takes down the
	// request with a nil dereference rather than saying so.
	if pg == nil {
		return meta
	}
	meta.Description = firstNonEmpty(pg.MetaDescription, pg.Excerpt,
		site.MetaDescription, site.Description)
	meta.NoIndex = pg.NoIndex
	return meta
}

// metaIn is metaData with the language prefix in the canonical address.
//
// The canonical is the address a search engine files the page under. Without
// the prefix the German and the French page would both claim example.de/kontakt
// and one of the two would silently drop out of the index.
func (h *Handler) metaIn(r *http.Request, website *domain.Website, site tmpl.SiteData, pg *page.Page, path string) tmpl.MetaData {
	meta := metaData(site, pg, path)
	meta.CanonicalURL = canonicalWithLocale(site.URL, LocaleFrom(r.Context()), website.Locale, path)
	return meta
}

// WithOGImage fills in the absolute preview-image URL. It is separate from
// metaData because resolving the media row needs the store and the request.
func (h *Handler) withOGImage(r *http.Request, website *domain.Website, meta tmpl.MetaData, pg *page.Page) tmpl.MetaData {
	if pg.FeaturedMediaID == nil {
		return meta
	}
	rel := h.mediaURL(r.Context(), website.ID, pg.FeaturedMediaID)
	if rel == "" {
		return meta
	}
	// og:image is read by machines that will not resolve a relative path.
	meta.OGImage = strings.TrimSuffix(h.canonicalBase(r, website), "/") + rel
	return meta
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// HandleMaintenance renders the maintenance page of a deactivated website.
//
// It answers 503 with Retry-After rather than 404: a 404 tells a search engine
// the pages are permanently gone, and a week of rebuilding would cost the whole
// index. Wired into the resolver, so it covers every path of the site.
func (h *Handler) HandleMaintenance(w http.ResponseWriter, r *http.Request) {
	website := domain.WebsiteFromContext(r.Context())
	if website == nil {
		http.NotFound(w, r)
		return
	}

	site := h.siteData(r, website)
	// Here too, and that is a decision and not levelling: a switched-off
	// website runs one more query less gladly, but the maintenance page is the
	// only page a visitor gets to see at all during that time — and the line
	// they are looking for there is the telephone number. One query per
	// request, on a page nobody fetches often.
	h.fillSnippets(r, &site, website.ID, h.loadSnippets(r, website.ID))
	content, err := h.loader.RenderMaintenance(r.Context(), website.ID, site, website.OfflineMessage)
	if err != nil {
		// The themed page is a courtesy; the status code is the part that
		// matters, so a broken theme must not turn this into a 500.
		http.Error(w, "Diese Website ist vorübergehend nicht erreichbar.", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Retry-After", "3600")
	w.Header().Set("X-Robots-Tag", "noindex")
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write(content)
}
