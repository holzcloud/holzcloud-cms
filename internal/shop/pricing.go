package shop

import (
	"net/http"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/money"
)

// Audience is who the prices are being shown to.
type Audience string

const (
	// Private sees gross prices — the amount actually payable, which is what
	// the Preisbekanntgabeverordnung requires of an offer to consumers.
	Private Audience = "private"
	// Business sees net prices with the tax added at the end.
	Business Audience = "business"
)

// Display values of websites.price_display.
const (
	DisplayPrivate  = "private"
	DisplayBusiness = "business"
	DisplayBoth     = "both"
)

// audienceCookie remembers a visitor's choice.
//
// A cookie rather than a session: the choice is not personal data, it survives
// a restart, and a visitor who never touches it costs no storage at all.
const audienceCookie = "hc_preise"

// Settings are a website's shop configuration, resolved once per request.
type Settings struct {
	Base     string // path the catalogue lives under; empty switches the shop off
	Currency money.Currency
	// Display is what the operator allows: private, business or both.
	Display string
	// VATExempt is the small-business rule: below CHF 100'000 of turnover no
	// tax is shown or charged, and the invoice has to say why.
	VATExempt bool
	VATNumber string
	// ReturnPolicy is copied onto every order placed while it says this. A
	// voluntary promise that changes retroactively is not a promise.
	ReturnPolicy string

	ShippingGross   money.Amount
	ShippingFreeAt  *money.Amount
	ShippingTaxRate money.TaxRate
}

// Enabled reports whether this website sells anything.
func (s Settings) Enabled() bool { return s.Base != "" }

// EffectiveRate is the rate to apply to a product.
//
// A shop below the VAT threshold charges nothing regardless of what its
// products say. Keeping the product's own rate untouched is deliberate: the
// day the shop crosses CHF 100'000 the operator flips one setting and every
// price is already carrying the rate it will need.
func (s Settings) EffectiveRate(r money.TaxRate) money.TaxRate {
	if s.VATExempt {
		return money.RateExempt
	}
	return r
}

// AudienceFor decides which prices this visitor sees.
//
// The operator's setting wins: a consumer shop must not be talked into showing
// net prices by a cookie, because the price a consumer is shown is regulated.
// Only where both are offered does the visitor's own choice count.
func (s Settings) AudienceFor(r *http.Request) Audience {
	switch s.Display {
	case DisplayBusiness:
		return Business
	case DisplayBoth:
		if c, err := r.Cookie(audienceCookie); err == nil && c.Value == string(Business) {
			return Business
		}
	}
	return Private
}

// SetAudienceCookie remembers the visitor's choice.
func SetAudienceCookie(w http.ResponseWriter, secure bool, a Audience) {
	http.SetCookie(w, &http.Cookie{
		Name:     audienceCookie,
		Value:    string(a),
		Path:     "/",
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ParseAudience reads a submitted choice, defaulting to the consumer view.
func ParseAudience(v string) Audience {
	if strings.TrimSpace(strings.ToLower(v)) == string(Business) {
		return Business
	}
	return Private
}

// Price is one price, ready to be printed.
//
// Both figures are carried even though only one is shown prominently: a
// business shop still has to state the gross total somewhere, and a consumer
// shop's invoice has to break out the tax. Computing them here rather than in
// the template is what keeps the template from doing arithmetic on money.
type Price struct {
	// Main is what the visitor reads as "the price".
	Main string
	// Note is the line under it — "inkl. 8.1 % MWST" or "zzgl. 8.1 % MWST".
	Note string
	// Gross and Net are the raw amounts, for a caller that has to add them up.
	Gross money.Amount
	Net   money.Amount
	Rate  money.TaxRate
	// Audience is who this was rendered for.
	Audience Audience
}

// PriceFor renders one product's price for one audience.
func (s Settings) PriceFor(gross money.Amount, rate money.TaxRate, a Audience) Price {
	effective := s.EffectiveRate(rate)
	net := money.NetFromGross(gross, effective)

	p := Price{
		Gross:    gross,
		Net:      net,
		Rate:     effective,
		Audience: a,
	}

	switch {
	case s.VATExempt:
		// Nothing may be shown that looks like tax. The note explains why, as
		// the invoice will have to.
		p.Main = s.Currency.Format(gross)
		p.Note = "keine MWST (Kleinunternehmen)"
	case a == Business:
		p.Main = s.Currency.Format(net)
		p.Note = "zzgl. " + effective.String() + " MWST"
	default:
		p.Main = s.Currency.Format(gross)
		p.Note = "inkl. " + effective.String() + " MWST"
	}
	return p
}

// ShippingFor is the delivery charge for a basket of a given value.
//
// The threshold is compared against the gross total, because that is the number
// the offer is phrased in — "Versandkostenfrei ab CHF 200" means what the
// customer pays, not what remains after tax.
func (s Settings) ShippingFor(itemsGross money.Amount) money.Amount {
	if s.ShippingFreeAt != nil && itemsGross >= *s.ShippingFreeAt {
		return 0
	}
	return s.ShippingGross
}

// FreeShippingNote is the sentence that names the threshold, or empty.
func (s Settings) FreeShippingNote() string {
	if s.ShippingFreeAt == nil || s.ShippingGross == 0 {
		return ""
	}
	return "Versandkostenfrei ab " + s.Currency.Format(*s.ShippingFreeAt)
}
