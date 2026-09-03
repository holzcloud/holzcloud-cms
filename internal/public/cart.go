package public

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// Fixed public routes of the basket. They are literal rather than derived from
// the shop path: there is one basket per website, not one per catalogue, and a
// fixed address is one a theme can link to without knowing the setting.
const (
	cartPath     = "/warenkorb"
	cartAddPath  = "/warenkorb/hinzufuegen"
	cartSetPath  = "/warenkorb/menge"
	cartDropPath = "/warenkorb/entfernen"
)

// SetCartStore attaches the basket. Nil leaves every basket route 404ing.
func (h *Handler) SetCartStore(s *shop.CartStore) { h.carts = s }

// cartData is what every page knows about the basket.
//
// Only the count and the total, because that is what a header shows. Reading
// the lines on every page of a website would be a join per request to render a
// number.
func (h *Handler) cartData(r *http.Request, website *domain.Website, set shop.Settings) tmpl.CartData {
	if h.carts == nil || !set.Enabled() {
		return tmpl.CartData{}
	}
	cart, err := h.carts.Get(r.Context(), website.ID, shop.TokenFrom(r))
	if err != nil || cart == nil {
		return tmpl.CartData{URL: cartPath}
	}

	totals := cart.Total(set, set.AudienceFor(r))
	return tmpl.CartData{
		Count: cart.Count(),
		URL:   cartPath,
		Total: set.Currency.Format(totals.TotalGross),
	}
}

// HandleCart renders the basket.
func (h *Handler) HandleCart(w http.ResponseWriter, r *http.Request) error {
	website, set, ok := h.shopRequest(w, r)
	if !ok {
		return nil
	}

	cart, err := h.carts.Get(r.Context(), website.ID, shop.TokenFrom(r))
	if err != nil {
		return fmt.Errorf("read cart: %w", err)
	}
	if cart == nil {
		cart = &shop.Cart{}
	}

	audience := set.AudienceFor(r)
	totals := cart.Total(set, audience)

	data := tmpl.PageData{
		Site:  h.siteData(r, website),
		Page:  tmpl.PageContent{Title: "Warenkorb", Slug: "warenkorb"},
		Menus: h.loadMenus(r, website.ID),
		Shop:  h.shopData(r, website),
		Cart:  h.cartView(r.Context(), website, set, audience, cart, totals),
	}
	data.Site.Snippets = h.loadSnippets(r, website.ID).HTML
	data.Meta = metaData(data.Site, nil, cartPath)
	// A basket is different for every visitor and worth nothing to a search
	// engine, so it says so rather than relying on the crawler to work it out.
	data.Meta.NoIndex = true

	content, err := h.loader.RenderPage(r.Context(), website.ID, "cart.html", data)
	if err != nil {
		return fmt.Errorf("render cart: %w", err)
	}
	// Never cached: two visitors share a proxy, and a cached basket is someone
	// else's shopping shown to a stranger.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = w.Write(content)
	return err
}

// cartView maps the basket onto what a template renders.
//
// The audience is passed in rather than read from the request again: the totals
// were computed for one of the two price bases, and reading it a second time is
// how a line ends up priced net under a gross total.
func (h *Handler) cartView(ctx context.Context, website *domain.Website, set shop.Settings,
	audience shop.Audience, cart *shop.Cart, totals shop.Totals) tmpl.CartData {

	view := tmpl.CartData{
		Count:     cart.Count(),
		URL:       cartPath,
		Total:     set.Currency.Format(totals.TotalGross),
		UpdateURL: cartSetPath,
		RemoveURL: cartDropPath,
	}

	blocked := ""
	for _, it := range cart.Items {
		line := tmpl.CartLine{
			Title:     it.Product.Title,
			Subtitle:  it.Product.Subtitle,
			URL:       website.ShopURL() + "/" + it.Product.Slug,
			Slug:      it.Product.Slug,
			Quantity:  it.Quantity,
			UnitPrice: set.Currency.Format(it.Line.UnitGross),
			LinePrice: set.Currency.Format(it.Line.Gross),
			Available: it.Product.Orderable(),
		}
		if audience == shop.Business {
			line.UnitPrice = set.Currency.Format(it.Line.UnitNet)
		}
		if it.Product.FeaturedMediaID != nil {
			line.ImageURL = h.mediaURL(ctx, website.ID, it.Product.FeaturedMediaID)
		}
		if !line.Available {
			blocked = "Ein Artikel im Warenkorb ist nicht mehr verfügbar. " +
				"Bitte entfernen Sie ihn, um fortzufahren."
		}
		view.Lines = append(view.Lines, line)
	}

	view.Totals = tmpl.CartTotals{
		Items:        set.Currency.Format(totals.ItemsGross),
		Shipping:     set.Currency.Format(totals.ShippingGross),
		ShippingFree: totals.ShippingGross == 0,
		Total:        set.Currency.Format(totals.TotalGross),
	}
	for _, b := range totals.Breakdown {
		view.Totals.TaxLines = append(view.Totals.TaxLines, tmpl.CartTaxLine{
			Label: "MWST " + b.Rate.String(),
			Net:   set.Currency.Format(b.Net),
			Tax:   set.Currency.Format(b.Tax),
		})
	}
	if set.VATExempt {
		view.Totals.TaxNote = "keine MWST (Kleinunternehmen)"
	}

	view.Blocked = blocked
	if !cart.Empty() && blocked == "" {
		view.CheckoutURL = "/kasse"
	}
	return view
}

// HandleCartAdd puts an article into the basket.
func (h *Handler) HandleCartAdd(w http.ResponseWriter, r *http.Request) error {
	website, _, ok := h.shopRequest(w, r)
	if !ok {
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}

	product, err := h.products.GetPublished(r.Context(), website.ID, r.FormValue("artikel"))
	if err != nil {
		return err
	}
	if product == nil {
		return h.serve404(w, r, website)
	}

	cart, token, err := h.carts.Ensure(r.Context(), website.ID, shop.TokenFrom(r))
	if err != nil {
		return err
	}
	shop.SetCartCookie(w, h.secure, token)

	quantity, _ := strconv.Atoi(r.FormValue("menge"))
	switch err := h.carts.Add(r.Context(), cart.ID, product.ID, quantity); err {
	case nil:
	case shop.ErrNotOrderable:
		// Back to the product, where the page already says it is unavailable.
		http.Redirect(w, r, website.ShopURL()+"/"+product.Slug, http.StatusSeeOther)
		return nil
	default:
		return err
	}

	http.Redirect(w, r, cartPath, http.StatusSeeOther)
	return nil
}

// HandleCartUpdate changes a quantity, removing the line at zero.
func (h *Handler) HandleCartUpdate(w http.ResponseWriter, r *http.Request) error {
	return h.cartLineChange(w, r, func(cartID, productID int64, r *http.Request) error {
		quantity, _ := strconv.Atoi(r.FormValue("menge"))
		return h.carts.SetQuantity(r.Context(), cartID, productID, quantity)
	})
}

// HandleCartRemove drops a line.
func (h *Handler) HandleCartRemove(w http.ResponseWriter, r *http.Request) error {
	return h.cartLineChange(w, r, func(cartID, productID int64, r *http.Request) error {
		return h.carts.Remove(r.Context(), cartID, productID)
	})
}

// cartLineChange is the shared body of the two line operations.
func (h *Handler) cartLineChange(w http.ResponseWriter, r *http.Request,
	apply func(cartID, productID int64, r *http.Request) error) error {

	website, _, ok := h.shopRequest(w, r)
	if !ok {
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}

	cart, err := h.carts.Get(r.Context(), website.ID, shop.TokenFrom(r))
	if err != nil {
		return err
	}
	if cart == nil {
		http.Redirect(w, r, cartPath, http.StatusSeeOther)
		return nil
	}

	// The article is named by its slug, and it is looked up inside this
	// website. A numeric id from the form would let a guessed number reach
	// another site's catalogue.
	product, err := h.products.GetPublished(r.Context(), website.ID, r.FormValue("artikel"))
	if err != nil {
		return err
	}
	if product != nil {
		if err := apply(cart.ID, product.ID, r); err != nil {
			return err
		}
	}

	http.Redirect(w, r, cartPath, http.StatusSeeOther)
	return nil
}

// shopRequest resolves the website and its shop settings, answering 404 when
// this website sells nothing.
func (h *Handler) shopRequest(w http.ResponseWriter, r *http.Request) (*domain.Website, shop.Settings, bool) {
	website := domain.WebsiteFromContext(r.Context())
	if website == nil {
		http.NotFound(w, r)
		return nil, shop.Settings{}, false
	}
	set := shopSettings(website)
	if !set.Enabled() || h.products == nil || h.carts == nil {
		_ = h.serve404(w, r, website)
		return nil, shop.Settings{}, false
	}
	return website, set, true
}
