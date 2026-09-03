package shop

// Die Preisliterale hier tragen U+00A0 zwischen Währung und Zahl — sichtbar
// als \u00a0 geschrieben, weil ein rohes geschütztes Leerzeichen im Quelltext
// nicht von einem gewöhnlichen zu unterscheiden ist.
import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/money"
)

func chfSettings(display string) Settings {
	return Settings{
		Base:            "shop",
		Currency:        money.CurrencyFor("CHF"),
		Display:         display,
		ShippingGross:   900,
		ShippingTaxRate: money.RateStandard,
	}
}

// The price shown to a consumer is regulated: it must be the amount actually
// payable. A cookie must not be able to talk a consumer shop into showing net
// prices — only a shop that offers both may follow the visitor's choice.
func TestTheOperatorsSettingWinsOverTheVisitorsCookie(t *testing.T) {
	cases := []struct {
		display string
		cookie  string
		want    Audience
	}{
		{DisplayPrivate, "business", Private}, // the cookie must not win here
		{DisplayPrivate, "", Private},
		{DisplayBusiness, "private", Business}, // nor here
		{DisplayBusiness, "", Business},
		{DisplayBoth, "business", Business},
		{DisplayBoth, "private", Private},
		{DisplayBoth, "", Private}, // consumers are the safe default
		{DisplayBoth, "unsinn", Private},
	}

	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/shop", nil)
		if c.cookie != "" {
			r.AddCookie(&http.Cookie{Name: audienceCookie, Value: c.cookie})
		}
		if got := chfSettings(c.display).AudienceFor(r); got != c.want {
			t.Errorf("display=%s cookie=%q: got %s, want %s", c.display, c.cookie, got, c.want)
		}
	}
}

// Consumers see the gross price and "inkl.", businesses the net and "zzgl.".
// Getting the word wrong is a price statement that is simply untrue.
func TestPriceNoteMatchesTheAudience(t *testing.T) {
	s := chfSettings(DisplayBoth)

	priv := s.PriceFor(4900, money.RateStandard, Private)
	if priv.Main != "CHF\u00a049.00" {
		t.Errorf("consumer price = %q, want the gross amount", priv.Main)
	}
	if !strings.HasPrefix(priv.Note, "inkl.") || !strings.Contains(priv.Note, "8.1 %") {
		t.Errorf("consumer note = %q, want inkl. 8.1 %% MWST", priv.Note)
	}

	biz := s.PriceFor(4900, money.RateStandard, Business)
	if biz.Main != "CHF\u00a045.33" {
		t.Errorf("business price = %q, want the net amount", biz.Main)
	}
	if !strings.HasPrefix(biz.Note, "zzgl.") {
		t.Errorf("business note = %q, want zzgl. …", biz.Note)
	}

	// Whatever is shown, the raw figures must still add up.
	for _, p := range []Price{priv, biz} {
		if p.Net+money.TaxFromGross(p.Gross, p.Rate) != p.Gross {
			t.Errorf("net %d and gross %d do not agree", p.Net, p.Gross)
		}
	}
}

// A shop below CHF 100'000 of turnover must not show anything that looks like
// tax — not "inkl. 0 % MWST" either, which reads as a rate rather than as an
// exemption.
func TestExemptShopShowsNoTaxAtAll(t *testing.T) {
	s := chfSettings(DisplayBoth)
	s.VATExempt = true

	for _, a := range []Audience{Private, Business} {
		p := s.PriceFor(4900, money.RateStandard, a)
		if p.Main != "CHF\u00a049.00" {
			t.Errorf("%s: price = %q, want the plain amount", a, p.Main)
		}
		if strings.Contains(p.Note, "inkl.") || strings.Contains(p.Note, "zzgl.") ||
			strings.Contains(p.Note, "8.1") {
			t.Errorf("%s: note = %q, which still talks about a rate", a, p.Note)
		}
		if p.Rate != money.RateExempt {
			t.Errorf("%s: rate = %s, want 0 for an exempt shop", a, p.Rate)
		}
	}

	// The product keeps its own rate: the day the shop crosses the threshold
	// the operator flips one setting and the prices already carry the rate.
	if s.EffectiveRate(money.RateStandard) != money.RateExempt {
		t.Error("an exempt shop still applied a rate")
	}
	s.VATExempt = false
	if s.EffectiveRate(money.RateStandard) != money.RateStandard {
		t.Error("switching the exemption off did not restore the product's rate")
	}
}

// "Versandkostenfrei ab CHF 200" is a promise phrased in what the customer
// pays, so the threshold is compared against the gross total.
func TestFreeShippingThresholdUsesTheGrossTotal(t *testing.T) {
	s := chfSettings(DisplayPrivate)
	limit := money.Amount(20000)
	s.ShippingFreeAt = &limit

	cases := map[money.Amount]money.Amount{
		0:     900,
		19999: 900,
		20000: 0, // exactly at the threshold is free — "ab" includes the value
		25000: 0,
	}
	for basket, want := range cases {
		if got := s.ShippingFor(basket); got != want {
			t.Errorf("basket %d: shipping %d, want %d", basket, got, want)
		}
	}

	if note := s.FreeShippingNote(); !strings.Contains(note, "CHF\u00a0200.00") {
		t.Errorf("note = %q, want the threshold named", note)
	}

	// Without a threshold there is nothing to promise.
	s.ShippingFreeAt = nil
	if note := s.FreeShippingNote(); note != "" {
		t.Errorf("note = %q, want none when no threshold is set", note)
	}
}

// A website with no shop path sells nothing, and every shop route has to be
// able to see that in one call.
func TestShopIsOffUntilAPathIsSet(t *testing.T) {
	if (Settings{}).Enabled() {
		t.Error("a website with no shop path reported an enabled shop")
	}
	if !chfSettings(DisplayPrivate).Enabled() {
		t.Error("a website with a shop path reported no shop")
	}
}
