package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// ShopSettingsValues is the shop configuration as the form carries it.
type ShopSettingsValues struct {
	ShopBase       string
	Currency       string
	ShippingGross  string
	ShippingFreeAt string
	ShippingTaxBP  int
	PriceDisplay   string
	VATExempt      bool
	VATNumber      string
	ReturnPolicy   string
	OrderEmail     string
	PaymentDetails string

	shipping money.Amount
	freeAt   *money.Amount
}

// ShopSettingsData backs the settings screen.
type ShopSettingsData struct {
	web.LayoutData
	web.FormState
	Values ShopSettingsValues
	Rates  []struct {
		Value int
		Label string
	}
}

func (v *ShopSettingsValues) validate(errs web.FormErrors) {
	// The path is a single segment, like blog_base. A slash here would produce
	// routes nobody can reach.
	v.ShopBase = strings.Trim(strings.TrimSpace(v.ShopBase), "/")
	if strings.ContainsAny(v.ShopBase, "/ ?#") {
		errs.Add("shop_base", "Nur ein Pfadabschnitt, ohne Schrägstrich — zum Beispiel: shop")
	}

	if amount, err := money.ParseAmount(v.ShippingGross); err != nil || amount < 0 {
		errs.Add("shipping", "Betrag nicht lesbar.")
	} else {
		v.shipping = amount
	}

	// Empty means there is no free-shipping threshold at all. Zero would mean
	// "free from nothing", which is a promise of always free — a different
	// offer, and one an operator should have to type on purpose.
	if text := strings.TrimSpace(v.ShippingFreeAt); text != "" {
		amount, err := money.ParseAmount(text)
		if err != nil || amount < 0 {
			errs.Add("free_from", "Betrag nicht lesbar.")
		} else {
			v.freeAt = &amount
		}
	}

	if !money.KnownRate(money.TaxRate(v.ShippingTaxBP)) {
		errs.Add("shipping_tax", "Bitte einen der Schweizer Steuersätze wählen.")
	}

	// Leer ist erlaubt und heisst: es geht keine Meldung raus. Eine Adresse
	// ohne @ ist dagegen ein Tippfehler, und der fällt sonst erst bei der
	// ersten Bestellung auf, die niemand bemerkt.
	if v.OrderEmail != "" && !strings.Contains(v.OrderEmail, "@") {
		errs.Add("order_email", "Das sieht nicht nach einer E-Mail-Adresse aus.")
	}

	switch v.PriceDisplay {
	case shop.DisplayPrivate, shop.DisplayBusiness, shop.DisplayBoth:
	default:
		v.PriceDisplay = shop.DisplayPrivate
	}
	if v.Currency == "" {
		v.Currency = "CHF"
	}
}

// HandleShopSettings shows and stores a website's shop configuration.
func (h *Handler) HandleShopSettings(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.shopWebsite(w, r)
	if err != nil || !ok {
		return err
	}

	if r.Method == http.MethodPost {
		return h.handleShopSettingsSave(w, r, ws)
	}

	values := ShopSettingsValues{
		ShopBase:       ws.ShopBase,
		Currency:       ws.Currency,
		ShippingGross:  money.Input(money.Amount(ws.ShippingGross)),
		ShippingTaxBP:  ws.ShippingTaxBP,
		PriceDisplay:   ws.PriceDisplay,
		VATExempt:      ws.VATExempt,
		VATNumber:      ws.VATNumber,
		ReturnPolicy:   ws.ReturnPolicy,
		OrderEmail:     ws.OrderEmail,
		PaymentDetails: ws.PaymentDetails,
	}
	if ws.ShippingFreeFrom != nil {
		values.ShippingFreeAt = money.Input(money.Amount(*ws.ShippingFreeFrom))
	}
	return h.renderShopSettings(w, r, ws, values, web.NewFormState())
}

func (h *Handler) renderShopSettings(w http.ResponseWriter, r *http.Request,
	ws *domain.Website, values ShopSettingsValues, state web.FormState) error {

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

	data := ShopSettingsData{
		LayoutData: web.NewLayoutData(r, h.sm, "Shop – "+ws.Name),
		FormState:  state,
		Values:     values,
		Rates:      rates,
	}
	data.ActiveNav = "products"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "shop_settings", data)
}

func (h *Handler) handleShopSettingsSave(w http.ResponseWriter, r *http.Request, ws *domain.Website) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	values := ShopSettingsValues{
		ShopBase:       r.FormValue("shop_base"),
		Currency:       strings.ToUpper(strings.TrimSpace(r.FormValue("currency"))),
		ShippingGross:  r.FormValue("shipping_gross"),
		ShippingFreeAt: r.FormValue("shipping_free_from"),
		PriceDisplay:   r.FormValue("price_display"),
		VATExempt:      r.FormValue("vat_exempt") != "",
		VATNumber:      strings.TrimSpace(r.FormValue("vat_number")),
		ReturnPolicy:   strings.TrimSpace(r.FormValue("return_policy")),
		OrderEmail:     strings.TrimSpace(r.FormValue("order_email")),
		PaymentDetails: strings.TrimSpace(r.FormValue("payment_details")),
	}
	values.ShippingTaxBP, _ = strconv.Atoi(r.FormValue("shipping_tax_bp"))

	state := web.NewFormState()
	values.validate(state.Errors)
	if state.Errors.Any() {
		return h.renderShopSettings(w, r, ws, values, state)
	}

	var freeFrom any
	if values.freeAt != nil {
		freeFrom = int64(*values.freeAt)
	}
	exempt := 0
	if values.VATExempt {
		exempt = 1
	}

	if _, err := h.db.Write.ExecContext(r.Context(),
		`UPDATE websites SET shop_base=$1, currency=$2, shipping_gross=$3,
		 shipping_free_from=$4, shipping_tax_bp=$5, price_display=$6,
		 vat_exempt=$7, vat_number=$8, return_policy=$9,
		 order_email=$10, payment_details=$11 WHERE id=$12`,
		values.ShopBase, values.Currency, int64(values.shipping), freeFrom,
		values.ShippingTaxBP, values.PriceDisplay, exempt, values.VATNumber,
		values.ReturnPolicy, values.OrderEmail, values.PaymentDetails, ws.ID); err != nil {
		return err
	}

	// The resolver caches the whole website row and the loader caches the
	// parsed theme. Without both, a changed shop path stays invisible until a
	// restart — the same trap the design tokens fell into.
	h.invalidateWebsiteCaches(ws.ID)

	web.SetFlashSuccess(h.sm, r.Context(), "Shop-Einstellungen gespeichert")
	return h.redirect(w, r, "/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/shop")
}
