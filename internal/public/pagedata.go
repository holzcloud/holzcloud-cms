package public

import (
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
)

// pageContent maps a stored page onto what a theme sees.
//
// Snippet markers are expanded here, at render time. Baking the expansion into
// content_html on save would freeze a copy of the opening hours into every
// page, which is the thing snippets exist to prevent.
//
// The responsive rewrite happens at the same moment and for the same reason:
// regenerating an image's scaled copies, or adding the pipeline to a site that
// already has content, must reach the pages that were written before it.
func (h *Handler) pageContent(r *http.Request, websiteID int64, pg *page.Page, snippets snippet.Rendered) tmpl.PageContent {
	updated := pg.UpdatedAt
	body := snippet.Expand(pg.ContentHTML, snippets.HTML)
	// The albums, and both halves of where this call stands are decisions.
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
	if h.albumStore != nil {
		if set, ok := h.albumSet(r, websiteID, body); ok {
			body = album.Expand(body, set)
		}
	}
	// Plugins last: they see the page as a visitor would, with the snippets
	// already in place and the form already drawn. A filter that ran earlier
	// would be filtering markers instead of text.
	body = h.filterByPlugins(r, websiteID, pg, body)
	felder, liste := h.ownFields(r, websiteID, pg)
	return tmpl.PageContent{
		Title:         pg.Title,
		ContentHTML:   template.HTML(h.responsive(r, websiteID, body)),
		Slug:          pg.Slug,
		PublishedAt:   pg.PublishedAt,
		UpdatedAt:     &updated,
		Excerpt:       pg.Excerpt,
		HasOwnHeading: startsWithHeading(pg.ContentHTML),
		Art:           pg.TypeKey,
		Terms:         termLinksAt(localePrefixOf(r), h.labelsForPage(r, pg.ID)),
		Felder:        felder,
		Feldliste:     liste,
	}
}

// albumSet loads the albums this page's HTML names, in the website's language.
//
// The translator is built from the website's locale — the same one
// internal/admin/page_blocks.go's blockSet uses for the save-time render — so a
// page carrying an inline gallery and an album gallery does not end up with its
// two sets of lightbox controls in two languages.
//
// A failing query is logged and reported as not-loaded, and the caller then
// leaves the body unexpanded rather than replacing every marker with nothing. A
// failing album must cost its own block and never the page, which is the rule
// h.responsive below already follows and the one block.Render states for a
// picture that was deleted.
func (h *Handler) albumSet(r *http.Request, websiteID int64, body string) (album.Set, bool) {
	var t func(string) string
	if ws := domain.WebsiteFromContext(r.Context()); ws != nil {
		locale := ws.Locale
		t = func(word string) string { return i18n.T(locale, word) }
	}
	set, err := h.albumStore.LoadFor(r.Context(), websiteID, body, t)
	if err != nil {
		slog.Error("load albums for page", "err", err, "website", websiteID)
		return album.Set{}, false
	}
	return set, true
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
	daten := field.Decode(pg.Fields)
	links := field.Links{
		Image: h.fieldImages(r, websiteID),
		Page:  h.fieldRefs(r, websiteID),
		Term:  h.fieldTerms(r, websiteID),
	}
	return field.Resolve(mine, daten, links), field.List(mine, daten, links)
}

// fillSnippets writes all three snippet members of SiteData.
//
// Der einzige Ort im Baum, an dem Snippets, Bausteinfelder und Bausteinliste
// an eine SiteData geschrieben werden, und das ist keine Ordnungsliebe: die
// Zuweisung stand an vierzehn Stellen in zehn Dateien, und ein Mitglied, das
// an dreizehn davon gefüllt wird, ist auf der vierzehnten unsichtbar — die
// Seite erscheint, das Theme druckt an einer Stelle nichts, nichts wird
// protokolliert, und niemand erfährt davon ausser einem Besucher. Deshalb eine
// Funktion und vierzehn Aufrufe, und deshalb ein Gatter im Plan, das
// nachweist, dass ausserhalb dieser Funktion keine Zuweisung überlebt hat.
//
// Was jenes Gatter nicht sehen kann, ist die entgegengesetzte Lücke: eine
// Route, die weder das eine noch das andere tut. Drei taten es —
// renderNotFound, serveShareError und HandleMaintenance —, und sie fielen
// nirgends auf, weil index über eine nil-Karte die leere Zeichenkette gibt und
// range über eine keine Runde dreht. Dagegen steht keine Zählung, sondern
// TestBausteinfelderAufDenRoutenOhneSeite: es zeichnet die drei Ansichten und
// liest den Wert aus dem Körper.
//
// Resolved on the way out rather than stored resolved: a picture chosen last
// month has to pick up this month's crop, and a field whose definition changed
// has to be read the new way without every snippet being saved again.
func (h *Handler) fillSnippets(r *http.Request, site *tmpl.SiteData, websiteID int64, rendered snippet.Rendered) {
	site.Snippets = rendered.HTML
	site.Bausteinfelder = map[string]map[string]any{}
	site.Bausteinliste = map[string][]field.Entry{}
	if h.fieldStore == nil {
		return
	}
	// Eine Website ohne einen einzigen Textbaustein zahlt keine Abfrage: sie
	// baut ihre Seiten auf wie vor dieser Phase, nur eben mit drei leeren
	// Karten statt mit dreien, die es nicht gibt.
	if len(rendered.IDs) == 0 {
		return
	}
	// Eine Abfrage für alle Textbausteine dieser Website und keine je
	// Textbaustein: das hier läuft auf jedem öffentlichen Aufbau einer Seite,
	// und eine Website mit fünf Textbausteinen zahlte sonst fünf Umläufe.
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
		daten := field.Decode(rendered.Fields[key])
		// Auch ein Textbaustein ohne eine einzige Definition bekommt seinen
		// Eintrag — eine leere Karte und keinen fehlenden Schlüssel: ein Theme,
		// das {{ index .Site.Bausteinfelder "kontakt" "telefon" }} schreibt,
		// soll auf einer Website, auf der noch niemand ein Feld angelegt hat,
		// nichts drucken statt zu scheitern.
		site.Bausteinfelder[key] = field.Resolve(defs, daten, links)
		// Nur eingetragen, wenn wirklich etwas gefüllt ist: field.List gibt
		// keinen leeren Eintrag heraus, und ein Schlüssel, hinter dem eine
		// leere Liste steht, wäre für ein Theme nicht von einem gefüllten zu
		// unterscheiden.
		if liste := field.List(defs, daten, links); len(liste) > 0 {
			site.Bausteinliste[key] = liste
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
				// Dieselbe Adresse mit demselben Sprachpräfix, das die
				// Schlagwortlinks auf dem Rest der Seite bekommen. Die
				// Adresse selbst kommt von Term.URL und steht hier deshalb
				// nicht ein zweites Mal ausgeschrieben — zwei Schreibweisen
				// derselben Adresse laufen irgendwann auseinander.
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
// Die drei Karten sind auch in den beiden Ausweichfällen angelegt und nie nil:
// ein Aufrufer soll nicht wissen müssen, ob er den geglückten oder den
// missglückten Weg in der Hand hält.
func (h *Handler) loadSnippets(r *http.Request, websiteID int64) snippet.Rendered {
	if h.snippetStore == nil {
		return leereBausteine()
	}
	rendered, err := h.snippetStore.LoadRendered(r.Context(), websiteID)
	if err != nil {
		slog.Error("load snippets", "err", err, "website", websiteID)
		return leereBausteine()
	}
	return rendered
}

func leereBausteine() snippet.Rendered {
	return snippet.Rendered{
		HTML:   map[string]template.HTML{},
		Fields: map[string]string{},
		IDs:    map[string]int64{},
	}
}

// expandForFeed expands snippet markers in feed content, so a subscriber sees
// the same text as a visitor rather than the raw marker.
func expandForFeed(html string, snippets snippet.Rendered) string {
	return snippet.Expand(html, snippets.HTML)
}

// contentModTime is the validator for conditional requests.
//
// It has to account for the snippets, not just the page: editing the opening
// hours changes what the page renders without touching pages.updated_at, and a
// browser holding an If-Modified-Since would keep the old text indefinitely.
func contentModTime(pg *page.Page, snippets snippet.Rendered) time.Time {
	if snippets.LatestUpdate.After(pg.UpdatedAt) {
		return snippets.LatestUpdate
	}
	return pg.UpdatedAt
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
	// Auch hier, und das ist eine Entscheidung und keine Gleichmacherei: eine
	// abgeschaltete Website führt eine Abfrage weniger gern aus, aber die
	// Wartungsseite ist die einzige Seite, die ein Besucher in dieser Zeit
	// überhaupt zu sehen bekommt — und die Zeile, die er dort sucht, ist die
	// Telefonnummer. Eine Abfrage je Anfrage, auf einer Seite, die niemand
	// oft abruft.
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
