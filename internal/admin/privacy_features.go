package admin

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
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
