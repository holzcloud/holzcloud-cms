package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// SetProductStore attaches the catalogue. Nil leaves every shop screen 404ing,
// which is what a build without a shop should do.
func (h *Handler) SetProductStore(s *shop.Store) { h.products = s }

// ProductValues is one product as the form carries it.
//
// Prices travel as the text the operator typed, not as a number: rejecting
// "49,50" because the field is an int would be the CMS being pedantic about a
// notation people use every day. The parsing happens once, in validate.
type ProductValues struct {
	ID           int64
	Slug         string
	Title        string
	Subtitle     string
	Markdown     string
	SKU          string
	Price        string
	TaxBP        int
	StockText    string
	WeightGrams  int
	DeliveryNote string
	Status       string
	FeaturedID   int64
	Terms        string

	// parsed results, filled by validate
	price money.Amount
	stock *int
}

// ProductListData backs the product list screen.
type ProductListData struct {
	web.LayoutData
	web.FormState
	Products []*shop.Product
	Currency money.Currency
	ShopBase string
}

// ProductFormData backs the create/edit screen.
type ProductFormData struct {
	web.LayoutData
	web.FormState
	Values ProductValues
	IsEdit bool
	Media  []media.Media
	Rates  []struct {
		Value int
		Label string
	}
	Website *domain.Website
}

func (v *ProductValues) validate(errs web.FormErrors) {
	v.Title = strings.TrimSpace(v.Title)
	v.Slug = strings.TrimSpace(v.Slug)

	if v.Title == "" {
		errs.Add("title", "Bitte einen Titel angeben.")
	}
	if v.Slug == "" {
		v.Slug = slugify(v.Title)
	}
	if v.Slug == "" {
		errs.Add("slug", "Die Adresse muss Buchstaben oder Ziffern enthalten.")
	}

	amount, err := money.ParseAmount(v.Price)
	if err != nil {
		errs.Add("price", "Preis nicht lesbar. Beispiele: 49.50, 1'234.00")
	} else if amount < 0 {
		errs.Add("price", "Ein Preis kann nicht negativ sein.")
	} else {
		v.price = amount
	}

	if !money.KnownRate(money.TaxRate(v.TaxBP)) {
		errs.Add("tax", "Bitte einen der Schweizer Steuersätze wählen.")
	}

	// Empty is "not tracked", which is a different thing from zero. A joiner
	// who builds to order has no stock level, and forcing a number here would
	// mark every made-to-measure piece as sold out.
	switch text := strings.TrimSpace(v.StockText); text {
	case "":
		v.stock = nil
	default:
		n, err := strconv.Atoi(text)
		if err != nil || n < 0 {
			errs.Add("stock", "Bitte eine Zahl ab 0 angeben, oder das Feld leer lassen.")
		} else {
			v.stock = &n
		}
	}

	if v.Status != shop.StatusDraft && v.Status != shop.StatusPublished {
		v.Status = shop.StatusDraft
	}
}

// HandleProductList shows a website's catalogue.
func (h *Handler) HandleProductList(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.shopWebsite(w, r)
	if err != nil || !ok {
		return err
	}

	products, err := h.products.List(r.Context(), ws.ID)
	if err != nil {
		return err
	}

	data := ProductListData{
		LayoutData: web.NewLayoutData(r, h.sm, "Produkte – "+ws.Name),
		FormState:  web.NewFormState(),
		Products:   products,
		Currency:   money.CurrencyFor(ws.Currency),
		ShopBase:   ws.ShopBase,
	}
	data.ActiveNav = "products"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "product_list", data)
}

// HandleProductForm creates and edits one product.
func (h *Handler) HandleProductForm(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.shopWebsite(w, r)
	if err != nil || !ok {
		return err
	}

	if r.Method == http.MethodPost {
		return h.handleProductSave(w, r, ws)
	}

	values := ProductValues{
		Status: shop.StatusDraft,
		TaxBP:  int(money.RateStandard),
		Price:  "0.00",
	}
	isEdit := false

	if raw := r.PathValue("productID"); raw != "" && raw != "neu" {
		id, _ := strconv.ParseInt(raw, 10, 64)
		p, err := h.products.Get(r.Context(), id)
		if err != nil {
			return err
		}
		if p == nil || p.WebsiteID != ws.ID {
			http.NotFound(w, r)
			return nil
		}
		values = productToValues(p)
		values.Terms = h.productTermNames(r, ws.ID, p.ID)
		isEdit = true
	}

	return h.renderProductForm(w, r, ws, values, isEdit, web.NewFormState())
}

func (h *Handler) renderProductForm(w http.ResponseWriter, r *http.Request,
	ws *domain.Website, values ProductValues, isEdit bool, state web.FormState) error {

	// Only images: a PDF cannot be a product picture, and offering one in the
	// dropdown is an invitation to a broken page.
	items, _, err := h.mediaStore.List(r.Context(), ws.ID,
		media.Filter{MimePrefix: "image/"}, 1, 500)
	if err != nil {
		return err
	}

	rates := make([]struct {
		Value int
		Label string
	}, 0, len(money.SwissRates))
	for _, r := range money.SwissRates {
		rates = append(rates, struct {
			Value int
			Label string
		}{int(r.Rate), r.Label})
	}

	title := "Neues Produkt"
	if isEdit {
		title = "Produkt bearbeiten"
	}
	data := ProductFormData{
		LayoutData: web.NewLayoutData(r, h.sm, title+" – "+ws.Name),
		FormState:  state,
		Values:     values,
		IsEdit:     isEdit,
		Media:      items,
		Rates:      rates,
		Website:    ws,
	}
	data.ActiveNav = "products"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "product_form", data)
}

func (h *Handler) handleProductSave(w http.ResponseWriter, r *http.Request, ws *domain.Website) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	values := ProductValues{
		Title:        r.FormValue("title"),
		Slug:         r.FormValue("slug"),
		Subtitle:     strings.TrimSpace(r.FormValue("subtitle")),
		Markdown:     r.FormValue("description_markdown"),
		SKU:          strings.TrimSpace(r.FormValue("sku")),
		Price:        r.FormValue("price"),
		StockText:    r.FormValue("stock"),
		DeliveryNote: strings.TrimSpace(r.FormValue("delivery_note")),
		Status:       r.FormValue("status"),
		Terms:        r.FormValue("terms"),
	}
	values.TaxBP, _ = strconv.Atoi(r.FormValue("tax_bp"))
	values.WeightGrams, _ = strconv.Atoi(r.FormValue("weight_grams"))
	values.FeaturedID, _ = strconv.ParseInt(r.FormValue("featured_media_id"), 10, 64)
	if raw := r.PathValue("productID"); raw != "" && raw != "neu" {
		values.ID, _ = strconv.ParseInt(raw, 10, 64)
	}

	state := web.NewFormState()
	values.validate(state.Errors)
	if state.Errors.Any() {
		return h.renderProductForm(w, r, ws, values, values.ID != 0, state)
	}

	// The description goes through the same markdown-then-sanitise pipeline as
	// a page. Anything else would make the product the one place where an
	// editor's HTML reaches a visitor unchecked.
	html, err := page.RenderMarkdown(values.Markdown)
	if err != nil {
		return err
	}

	p := &shop.Product{
		ID:                  values.ID,
		WebsiteID:           ws.ID,
		Slug:                values.Slug,
		Title:               values.Title,
		Subtitle:            values.Subtitle,
		DescriptionMarkdown: values.Markdown,
		DescriptionHTML:     html,
		Excerpt:             page.Excerpt(values.Markdown),
		SKU:                 values.SKU,
		PriceGross:          values.price,
		TaxRate:             money.TaxRate(values.TaxBP),
		Stock:               values.stock,
		WeightGrams:         values.WeightGrams,
		DeliveryNote:        values.DeliveryNote,
		Status:              values.Status,
	}
	if values.FeaturedID > 0 {
		p.FeaturedMediaID = &values.FeaturedID
	}

	var saveErr error
	if p.ID == 0 {
		p.ID, saveErr = h.products.Create(r.Context(), p)
	} else {
		saveErr = h.products.Update(r.Context(), p)
	}
	if saveErr == shop.ErrSlugTaken {
		state.Errors.Add("slug", "Diese Adresse ist schon vergeben.")
		return h.renderProductForm(w, r, ws, values, values.ID != 0, state)
	}
	if saveErr != nil {
		return saveErr
	}

	if err := h.setProductTerms(r, ws.ID, p.ID, values.Terms); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Produkt gespeichert")
	return h.redirect(w, r, "/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/produkte")
}

// HandleProductDelete removes a product.
func (h *Handler) HandleProductDelete(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.shopWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("productID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	// Scoped to the website, so a guessed id from another site deletes nothing.
	p, err := h.products.Get(r.Context(), id)
	if err != nil {
		return err
	}
	if p == nil || p.WebsiteID != ws.ID {
		http.NotFound(w, r)
		return nil
	}
	if err := h.products.Delete(r.Context(), id); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Produkt gelöscht")
	return h.redirect(w, r, "/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/produkte")
}

// shopWebsite resolves the website from the path and reports whether the
// request may continue.
func (h *Handler) shopWebsite(w http.ResponseWriter, r *http.Request) (*domain.Website, bool, error) {
	if h.products == nil {
		http.NotFound(w, r)
		return nil, false, nil
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil, false, nil
	}
	ws, err := h.domains.GetWebsite(r.Context(), id)
	if err != nil {
		return nil, false, err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil, false, nil
	}
	return ws, true, nil
}

func productToValues(p *shop.Product) ProductValues {
	v := ProductValues{
		ID:           p.ID,
		Slug:         p.Slug,
		Title:        p.Title,
		Subtitle:     p.Subtitle,
		Markdown:     p.DescriptionMarkdown,
		SKU:          p.SKU,
		Price:        money.Input(p.PriceGross),
		TaxBP:        int(p.TaxRate),
		WeightGrams:  p.WeightGrams,
		DeliveryNote: p.DeliveryNote,
		Status:       p.Status,
	}
	if p.Stock != nil {
		v.StockText = strconv.Itoa(*p.Stock)
	}
	if p.FeaturedMediaID != nil {
		v.FeaturedID = *p.FeaturedMediaID
	}
	return v
}

// productTermNames renders a product's categories as the comma-separated text
// the form edits — the same shape the page editor uses for labels.
func (h *Handler) productTermNames(r *http.Request, websiteID, productID int64) string {
	if h.terms == nil {
		return ""
	}
	ids, err := h.products.TermIDs(r.Context(), productID)
	if err != nil || len(ids) == 0 {
		return ""
	}
	all, err := h.terms.ListAll(r.Context(), websiteID)
	if err != nil {
		return ""
	}
	wanted := make(map[int64]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	var names []string
	for _, t := range all {
		if wanted[t.ID] {
			names = append(names, t.Name)
		}
	}
	return strings.Join(names, ", ")
}

// setProductTerms turns the typed names into label rows, creating the ones that
// do not exist yet — the same behaviour the page editor has, so a shop and a
// blog share one vocabulary instead of drifting into two.
func (h *Handler) setProductTerms(r *http.Request, websiteID, productID int64, raw string) error {
	if h.terms == nil {
		return nil
	}
	var names []string
	for _, part := range strings.Split(raw, ",") {
		if name := strings.TrimSpace(part); name != "" {
			names = append(names, name)
		}
	}

	return h.terms.SetForProduct(r.Context(), websiteID, productID, names)
}
