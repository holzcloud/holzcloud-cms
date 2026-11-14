package admin

import (
	"context"
	"errors"

	"github.com/holzcloud/holzcloud-cms/internal/value"
)

// The values of a website — the one-line texts a page names as [[wert:key]] —
// as the assistant connection reaches them.
//
// Each Op… method below is the one store call the Values screen makes too,
// reached through here because the store is the handler's and not the
// connection's. The screen and the connection share the store and its checks:
// the shape of a key and the cleaning of a text run inside the store, on every
// write, so the two cannot drift apart. Every method takes the website and
// hands it to the store, which puts it into every statement — a key an
// assistant names on one website never reaches another's value (D-03).
//
// No publishing right is asked here: the level of the assistant's key decides
// whether it may write at all, and a value is not a publication (Phase 31
// D-11). The store's sentinel errors come back unchanged, so the connection can
// say in its own words which rule a text broke.

// errNoValueStore answers when the installation was built without the store.
var errNoValueStore = errors.New("values are not available in this installation")

// OpValues lists the values of a website, by key, with the report of what its
// pages, snippets and menus do with values.
func (h *Handler) OpValues(ctx context.Context, websiteID int64) ([]value.Value, value.Report, error) {
	if h.valueStore == nil {
		return nil, value.Report{}, errNoValueStore
	}
	list, err := h.valueStore.List(ctx, websiteID)
	if err != nil {
		return nil, value.Report{}, err
	}
	rep, err := h.valueStore.Report(ctx, websiteID)
	if err != nil {
		return nil, value.Report{}, err
	}
	return list, rep, nil
}

// OpValue returns one value of a website by key, nil when the website has none
// by that key, with the report its users are read from.
func (h *Handler) OpValue(ctx context.Context, websiteID int64, key string) (*value.Value, value.Report, error) {
	if h.valueStore == nil {
		return nil, value.Report{}, errNoValueStore
	}
	v, err := h.valueStore.GetByKey(ctx, websiteID, key)
	if err != nil || v == nil {
		return nil, value.Report{}, err
	}
	rep, err := h.valueStore.Report(ctx, websiteID)
	if err != nil {
		return nil, value.Report{}, err
	}
	return v, rep, nil
}

// OpSetValue creates the value of a key or changes its text, and reports
// whether it created it.
func (h *Handler) OpSetValue(ctx context.Context, websiteID int64, key, text string) (*value.Value, bool, error) {
	if h.valueStore == nil {
		return nil, false, errNoValueStore
	}
	return h.valueStore.Set(ctx, websiteID, key, text)
}

// OpDeleteValue removes a value nobody names any more. A value a page or a
// snippet still names is refused with the store's *value.InUseError.
func (h *Handler) OpDeleteValue(ctx context.Context, websiteID int64, key string) error {
	if h.valueStore == nil {
		return errNoValueStore
	}
	return h.valueStore.Delete(ctx, websiteID, key)
}
