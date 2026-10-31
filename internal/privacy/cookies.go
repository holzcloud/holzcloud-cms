// Package privacy holds what the public side of a website stores in a visitor's
// browser, and the privacy text that says so.
//
// The register in this file is the one list of public cookies. Their names and
// lifetimes are not typed here: they are read from the exported constants of
// the packages that set them, so a renamed cookie or a changed lifetime changes
// the register and with it the published text. A test walks the source and
// fails when any file other than the known ones writes a cookie, so a new
// cookie cannot go out without being listed.
//
// This file is language-neutral. The German text is in text.go.
package privacy

import (
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// Features is what a website uses, and therefore which blocks and which
// cookies its privacy text needs. The admin computes it from the website's
// settings.
//
// PriceChoice and Payment only mean something when Shop is true; Compose and
// Cookies treat them as false otherwise.
type Features struct {
	Shop           bool
	PriceChoice    bool
	Payment        bool
	ProtectedPages bool
	Forms          bool
}

// normalised returns f with the shop-only features switched off when there is
// no shop.
func (f Features) normalised() Features {
	if !f.Shop {
		f.PriceChoice = false
		f.Payment = false
	}
	return f
}

// Trigger names what the visitor does that makes the program set a cookie. No
// public cookie is ever set without one of these.
type Trigger string

const (
	// TriggerBasket is the visitor putting something in the basket.
	TriggerBasket Trigger = "basket"
	// TriggerPriceView is the visitor switching the price view.
	TriggerPriceView Trigger = "price-view"
	// TriggerUnlock is the visitor entering a page password.
	TriggerUnlock Trigger = "unlock"
)

// Cookie is one entry of the register.
type Cookie struct {
	// Name is the cookie name. For the unlock cookie it is the prefix plus the
	// letter N, standing for the number of the protected page.
	Name     string
	Lifetime time.Duration
	Trigger  Trigger
}

// Cookies returns the cookies a website with these features can store in a
// visitor's browser, in a fixed order: basket, price view, unlock.
func Cookies(f Features) []Cookie {
	f = f.normalised()
	var out []Cookie
	if f.Shop {
		out = append(out, Cookie{Name: shop.CartCookieName, Lifetime: shop.CartLifetime, Trigger: TriggerBasket})
	}
	if f.PriceChoice {
		out = append(out, Cookie{Name: shop.AudienceCookieName, Lifetime: shop.AudienceLifetime, Trigger: TriggerPriceView})
	}
	if f.ProtectedPages {
		out = append(out, Cookie{Name: page.UnlockCookiePrefix + "N", Lifetime: page.UnlockLifetime, Trigger: TriggerUnlock})
	}
	return out
}
