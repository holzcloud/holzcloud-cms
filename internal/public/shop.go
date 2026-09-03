package public

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"html/template"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// productsPerPage is how many articles one catalogue page shows.
//
// Not a setting: the number of columns is decided by the theme's grid, and an
// operator who sets 7 gets a ragged last row on every width. Twelve divides by
// two, three and four.
const productsPerPage = 12

// SetProductStore attaches the catalogue. It may be nil, in which case every
// shop route 404s and no shop data is rendered.
func (h *Handler) SetProductStore(s *shop.Store) { h.products = s }

// shopSettings resolves a website's shop configuration.
func shopSettings(w *domain.Website) shop.Settings {
	s := shop.Settings{
		Base:            w.ShopBase,
		Currency:        money.CurrencyFor(w.Currency),
		Display:         w.PriceDisplay,
		VATExempt:       w.VATExempt,
		VATNumber:       w.VATNumber,
		ReturnPolicy:    w.ReturnPolicy,
		ShippingGross:   money.Amount(w.ShippingGross),
		ShippingTaxRate: money.TaxRate(w.ShippingTaxBP),
	}
	if w.ShippingFreeFrom != nil {
		v := money.Amount(*w.ShippingFreeFrom)
		s.ShippingFreeAt = &v
	}
	return s
}

// shopData is what every page of a selling website knows about its shop.
//
// It is filled on every page, not only in the catalogue, so a layout can put a
// basket or the price switch in its header. On a website that sells nothing it
// is the zero value, which is what a theme has to survive.
func (h *Handler) shopData(r *http.Request, website *domain.Website) tmpl.ShopData {
	set := shopSettings(website)
	if !set.Enabled() || h.products == nil {
		return tmpl.ShopData{}
	}

	audience := set.AudienceFor(r)
	sample := set.PriceFor(0, money.RateStandard, audience)

	return tmpl.ShopData{
		Enabled:  true,
		URL:      website.ShopURL(),
		Audience: string(audience),
		// Only where both are served. A consumer shop must not offer to show
		// net prices — see internal/shop/pricing.go.
		CanSwitchAudience: set.Display == shop.DisplayBoth,
		TaxNote:           sample.Note,
		ShippingNote:      set.FreeShippingNote(),
		Categories:        h.shopCategories(r, website),
	}
}

// shopCategories are the labels that actually have published products behind
// them. A category that leads to an empty page is worse than no category.
func (h *Handler) shopCategories(r *http.Request, website *domain.Website) []tmpl.TermLink {
	if h.termStore == nil || h.products == nil {
		return nil
	}
	terms, err := h.termStore.ListAll(r.Context(), website.ID)
	if err != nil {
		return nil
	}

	var out []tmpl.TermLink
	for _, t := range terms {
		n, err := h.products.CountPublishedByTerm(r.Context(), website.ID, t.ID)
		if err != nil || n == 0 {
			continue
		}
		out = append(out, tmpl.TermLink{
			Name:  t.Name,
			URL:   website.ShopURL() + "/kategorie/" + t.Slug,
			Count: n,
		})
	}
	return out
}

// HandleShop renders the catalogue, optionally filtered by a category.
func (h *Handler) HandleShop(w http.ResponseWriter, r *http.Request, website *domain.Website, categorySlug string) error {
	set := shopSettings(website)
	if !set.Enabled() || h.products == nil {
		return h.serve404(w, r, website)
	}

	audience := set.AudienceFor(r)
	page := pageNumber(r)
	offset := (page - 1) * productsPerPage

	var (
		products []*shop.Product
		total    int
		err      error
		termName string
		basePath = website.ShopURL()
	)

	if categorySlug != "" {
		if h.termStore == nil {
			return h.serve404(w, r, website)
		}
		t, err := h.termStore.GetBySlug(r.Context(), website.ID, categorySlug)
		if err != nil || t == nil {
			return h.serve404(w, r, website)
		}
		termName = t.Name
		basePath = website.ShopURL() + "/kategorie/" + t.Slug
		products, err = h.products.ListPublishedByTerm(r.Context(), website.ID, t.ID, productsPerPage, offset)
		if err != nil {
			return fmt.Errorf("list products by term: %w", err)
		}
		total, _ = h.products.CountPublishedByTerm(r.Context(), website.ID, t.ID)
	} else {
		products, err = h.products.ListPublished(r.Context(), website.ID, productsPerPage, offset)
		if err != nil {
			return fmt.Errorf("list products: %w", err)
		}
		total, _ = h.products.CountPublished(r.Context(), website.ID)
	}

	totalPages := (total + productsPerPage - 1) / productsPerPage
	if totalPages < 1 {
		totalPages = 1
	}
	// A page beyond the end is a wrong address, not an empty catalogue.
	if page > totalPages && total > 0 {
		return h.serve404(w, r, website)
	}

	title := "Shop"
	if termName != "" {
		title = termName
	}

	site := h.siteData(r, website)
	site.Snippets = h.loadSnippets(r, website.ID).HTML

	data := tmpl.PageData{
		Site:  site,
		Page:  tmpl.PageContent{Title: title, Slug: website.ShopBase},
		Menus: h.loadMenus(r, website.ID),
		Meta:  metaData(site, nil, basePath),
		Shop:  h.shopData(r, website),
		Cart:  h.cartData(r, website, set),
		Catalogue: tmpl.CatalogueData{
			Products:   h.productEntries(website, set, audience, products),
			Page:       page,
			TotalPages: totalPages,
			Total:      total,
			PrevURL:    pageURL(basePath, page-1, page > 1),
			NextURL:    pageURL(basePath, page+1, page < totalPages),
			Term:       termName,
		},
	}

	content, err := h.loader.RenderPage(r.Context(), website.ID, "shop.html", data)
	if err != nil {
		return fmt.Errorf("render shop: %w", err)
	}
	h.serveCached(w, r, content, time.Time{})
	return nil
}

// HandleProduct renders one product.
func (h *Handler) HandleProduct(w http.ResponseWriter, r *http.Request, website *domain.Website, slug string) error {
	set := shopSettings(website)
	if !set.Enabled() || h.products == nil {
		return h.serve404(w, r, website)
	}

	p, err := h.products.GetPublished(r.Context(), website.ID, slug)
	if err != nil {
		return fmt.Errorf("get product: %w", err)
	}
	if p == nil {
		return h.serve404(w, r, website)
	}

	audience := set.AudienceFor(r)
	price := set.PriceFor(p.PriceGross, p.TaxRate, audience)

	// The other audience's figure, so a trade shop still states the gross
	// amount and a consumer shop can show the net one on request.
	other := ""
	if set.Display == shop.DisplayBoth && !set.VATExempt {
		switch audience {
		case shop.Business:
			other = set.Currency.Format(p.PriceGross) + " inkl. MWST"
		default:
			other = set.Currency.Format(price.Net) + " zzgl. MWST"
		}
	}

	site := h.siteData(r, website)
	site.Snippets = h.loadSnippets(r, website.ID).HTML

	productURL := website.ShopURL() + "/" + p.Slug
	meta := metaData(site, nil, productURL)
	meta.Description = firstNonEmpty(p.Excerpt, p.Subtitle, site.MetaDescription)

	data := tmpl.PageData{
		Site:  site,
		Page:  tmpl.PageContent{Title: p.Title, Slug: p.Slug},
		Menus: h.loadMenus(r, website.ID),
		Meta:  meta,
		Shop:  h.shopData(r, website),
		Cart:  h.cartData(r, website, set),
		Product: tmpl.ProductData{
			Title:           p.Title,
			Subtitle:        p.Subtitle,
			Slug:            p.Slug,
			DescriptionHTML: template.HTML(p.DescriptionHTML),
			SKU:             p.SKU,
			Price:           price.Main,
			PriceNote:       price.Note,
			PriceOther:      other,
			ImageURL:        h.mediaURL(r.Context(), website.ID, p.FeaturedMediaID),
			Gallery:         h.galleryURLs(r, website.ID, p.ID),
			Available:       p.Orderable(),
			StockNote:       stockNote(p),
			DeliveryNote:    p.DeliveryNote,
			Terms:           h.productTerms(r, website, p.ID),
			AddURL:          "/warenkorb/hinzufuegen",
		},
	}

	content, err := h.loader.RenderPage(r.Context(), website.ID, "product.html", data)
	if err != nil {
		return fmt.Errorf("render product: %w", err)
	}
	h.serveCached(w, r, content, time.Time{})
	return nil
}

// HandlePriceSwitch stores the visitor's choice of price mode.
//
// A POST rather than a link, because it changes stored state: a crawler
// following every link on the page must not be able to flip a shop into trade
// prices for the visitor whose cache it shares.
func (h *Handler) HandlePriceSwitch(w http.ResponseWriter, r *http.Request) error {
	website := domain.WebsiteFromContext(r.Context())
	if website == nil {
		http.NotFound(w, r)
		return nil
	}
	set := shopSettings(website)
	// Only where both are offered. Anywhere else the operator's setting decides
	// and this route must not appear to work.
	if !set.Enabled() || set.Display != shop.DisplayBoth {
		return h.serve404(w, r, website)
	}
	if err := r.ParseForm(); err != nil {
		return err
	}

	shop.SetAudienceCookie(w, r.TLS != nil, shop.ParseAudience(r.FormValue("ansicht")))

	back := r.FormValue("zurueck")
	// Only a path on this site. An open redirect out of a shop is how a
	// phishing page gets a genuine domain in front of it.
	if !strings.HasPrefix(back, "/") || strings.HasPrefix(back, "//") {
		back = website.ShopURL()
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}

// productEntries maps products onto what a listing renders.
func (h *Handler) productEntries(website *domain.Website, set shop.Settings,
	audience shop.Audience, products []*shop.Product) []tmpl.ProductEntry {

	out := make([]tmpl.ProductEntry, 0, len(products))
	for _, p := range products {
		price := set.PriceFor(p.PriceGross, p.TaxRate, audience)
		entry := tmpl.ProductEntry{
			Title:     p.Title,
			Subtitle:  p.Subtitle,
			URL:       website.ShopURL() + "/" + p.Slug,
			Excerpt:   p.Excerpt,
			Price:     price.Main,
			PriceNote: price.Note,
			Available: p.Orderable(),
		}
		if !entry.Available {
			entry.SoldOutLabel = "Ausverkauft"
		}
		if p.FeaturedMediaID != nil {
			entry.ImageURL = h.mediaURL(context.Background(), website.ID, p.FeaturedMediaID)
		}
		out = append(out, entry)
	}
	return out
}

// stockNote is the sentence about availability, or empty when the operator
// does not track quantities.
func stockNote(p *shop.Product) string {
	if p.Stock == nil {
		return ""
	}
	switch {
	case *p.Stock <= 0:
		return "Ausverkauft"
	case *p.Stock == 1:
		return "Noch 1 an Lager"
	default:
		return "Noch " + strconv.Itoa(*p.Stock) + " an Lager"
	}
}

// pageNumber reads ?seite=, defaulting to the first.
func pageNumber(r *http.Request) int {
	n, err := strconv.Atoi(r.URL.Query().Get("seite"))
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// pageURL builds a pager link, or empty when there is nowhere to go.
func pageURL(base string, page int, exists bool) string {
	if !exists {
		return ""
	}
	if page <= 1 {
		return base
	}
	return base + "?seite=" + strconv.Itoa(page)
}

// productTerms are the labels on one product, as links into the shop.
func (h *Handler) productTerms(r *http.Request, website *domain.Website, productID int64) []tmpl.TermLink {
	if h.termStore == nil || h.products == nil {
		return nil
	}
	ids, err := h.products.TermIDs(r.Context(), productID)
	if err != nil || len(ids) == 0 {
		return nil
	}
	terms, err := h.termStore.ListAll(r.Context(), website.ID)
	if err != nil {
		return nil
	}

	wanted := make(map[int64]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}

	var out []tmpl.TermLink
	for _, t := range terms {
		if wanted[t.ID] {
			out = append(out, tmpl.TermLink{
				Name: t.Name,
				URL:  website.ShopURL() + "/kategorie/" + t.Slug,
			})
		}
	}
	return out
}

// galleryURLs are the product's further pictures.
func (h *Handler) galleryURLs(r *http.Request, websiteID, productID int64) []string {
	if h.products == nil {
		return nil
	}
	ids, err := h.products.GalleryIDs(r.Context(), productID)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if url := h.mediaURL(r.Context(), websiteID, &id); url != "" {
			out = append(out, url)
		}
	}
	return out
}

// ShopRoutes dispatches the catalogue's deeper paths before the route table.
//
// The shop lives under a per-website path, so its routes cannot be registered
// as patterns: "/{base}/{slug}" and "/t/{path...}" both match /t/etwas, neither
// is more specific than the other, and Go's mux panics at startup rather than
// guess. Matching the first segment against this website's own setting is what
// the request actually needs, and by the time this runs the resolver has put
// the website in the context.
//
// Anything that is not this website's shop falls straight through to next, so
// a site without a shop is routed exactly as before.
func (h *Handler) ShopRoutes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		website := domain.WebsiteFromContext(r.Context())
		if website == nil || !website.HasShop() || r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		rest, ok := strings.CutPrefix(r.URL.Path, "/"+website.ShopBase+"/")
		if !ok || rest == "" {
			next.ServeHTTP(w, r)
			return
		}

		// /{shop}/kategorie/{slug} and /{shop}/{product}. Anything deeper is
		// not an address this shop has.
		handle := h.ErrHandler(func(w http.ResponseWriter, r *http.Request) error {
			if slug, ok := strings.CutPrefix(rest, "kategorie/"); ok {
				if slug == "" || strings.Contains(slug, "/") {
					return h.serve404(w, r, website)
				}
				return h.HandleShop(w, r, website, slug)
			}
			if strings.Contains(rest, "/") {
				return h.serve404(w, r, website)
			}
			return h.HandleProduct(w, r, website, rest)
		})
		handle(w, r)
	})
}
