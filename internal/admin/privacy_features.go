package admin

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/privacy"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// formPluginIDs are the plugins that store what a visitor typed.
//
// Both count for the privacy text: the contact form keeps every enquiry in the
// administration, and the order form keeps the order with the address that came
// with it. Either one on a website means the text has to describe what happens
// to a message.
var formPluginIDs = []string{"kontaktformular", "bestellung"}

// privacyFeatures answers which of the functions the privacy text tells apart
// a website uses.
//
// Each answer is the condition under which the function sets a cookie or
// stores something a visitor gave. The method only reads.
func (h *Handler) privacyFeatures(ctx context.Context, ws *domain.Website) (privacy.Features, error) {
	var f privacy.Features
	if ws == nil {
		return f, errors.New("no website")
	}

	f.Shop = ws.HasShop()
	if f.Shop {
		f.PriceChoice = ws.PriceDisplay == shop.DisplayBoth
		// The same condition cmd/holzcloud/main.go uses to allow the payment
		// provider as a form target: both halves of the credentials.
		f.Payment = h.cfg != nil && h.cfg.PayrexxInstance != "" && h.cfg.PayrexxSecret != ""
	}

	protected, err := h.pages.HasProtected(ctx, ws.ID)
	if err != nil {
		return privacy.Features{}, fmt.Errorf("privacy features: %w", err)
	}
	f.ProtectedPages = protected

	forms, err := h.formsEnabled(ctx, ws.ID)
	if err != nil {
		return privacy.Features{}, fmt.Errorf("privacy features: %w", err)
	}
	f.Forms = forms

	return f, nil
}

// legalRequest is everything a legal page of this website depends on: the
// functions it uses, the frameworks it follows and the language of the page.
//
// It reads the website it is given and nothing else, so one website's choices
// can never shape another's text. When the functions cannot be read, the
// request still carries the language and the stored set, with the zero
// Features beside the error; a caller that cannot stop, like the starter
// content, uses it as it is.
func (h *Handler) legalRequest(ctx context.Context, ws *domain.Website, tag string) (privacy.Request, error) {
	if ws == nil {
		return privacy.Request{Lang: tag, Swiss: i18n.Region(tag) == "CH"}, errors.New("no website")
	}
	// Swiss spelling (D-15) for a website in Switzerland, and for a page whose
	// tag says Swiss German even when the website is elsewhere.
	swiss := ws.Country == "CH" || i18n.Region(tag) == "CH"
	r := privacy.Request{Lang: tag, Swiss: swiss, Set: ws.Frameworks()}
	f, err := h.privacyFeatures(ctx, ws)
	if err != nil {
		return r, err
	}
	r.Features = f
	return r, nil
}

// formsEnabled reports whether one of the form plugins is installed, switched
// on, and assigned to the website.
func (h *Handler) formsEnabled(ctx context.Context, websiteID int64) (bool, error) {
	if h.plugins == nil {
		return false, nil
	}
	for _, id := range formPluginIDs {
		st, err := h.plugins.Get(ctx, id)
		if errors.Is(err, plugin.ErrNotFound) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("look up plugin %s: %w", id, err)
		}
		if st.Enabled && slices.Contains(st.Websites, websiteID) {
			return true, nil
		}
	}
	return false, nil
}
