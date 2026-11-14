package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The Opt-outs screen (phase 33.1 D-11 and D-14). Its German name comes from
// the catalogue, under the key "Opt-outs".
//
// A website that follows US law offers a "Do Not Sell or Share" page, and the
// form on it ([[widerspruch]], internal/public/optout.go) stores what a visitor
// sends in privacy_requests. This screen is where the operator reads those
// requests — per website, because a request belongs to the website it was
// made on and to no other — and records that they answered one.
//
// There is no delete, on purpose. CCPA regs § 7101 ask a business to keep the
// record of a consumer request for at least 24 months, and deciding when a
// row may go is Phase 33.2's retention, which has to honour that floor (D-14).
// A delete button here would be the one way to break it by accident, and it
// would be the twenty-third hx-confirm. dsr.Store has no delete method either.
//
// The rights are the order book's: anybody with access to the website may
// list the requests and mark one handled. Marking records an answer; it
// publishes nothing, so the publishing right is not the question here, just as
// it is not for an order's status.
//
// Opt-outs are not exposed over MCP (D-16): a visitor's name, address and
// note stay in the admin.

// optoutListData backs the Opt-outs screen.
type optoutListData struct {
	web.LayoutData
	web.FormState
	WebsiteID int64
	Requests  []optoutView
	// HasUS is false when US law is no longer chosen for the website: the
	// form is gone from its pages, the requests it took stay listed.
	HasUS bool
}

// optoutView is one request as the screen shows it.
type optoutView struct {
	dsr.Request
	// Language is the name of the page's language, a catalogue key.
	Language string
}

// optoutListAddress is the screen of one website.
func optoutListAddress(websiteID int64) string {
	return "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/widersprueche"
}

// optoutWebsite resolves the website of the path. ok is false when the answer
// has been written already: a malformed or unknown id, or a build without the
// request store, is a 404.
func (h *Handler) optoutWebsite(w http.ResponseWriter, r *http.Request) (*domain.Website, bool, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || h.requests == nil {
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

// HandleOptOutList shows a website's opt-out requests, newest first.
func (h *Handler) HandleOptOutList(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.optoutWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	list, err := h.requests.List(r.Context(), ws.ID, dsr.KindOptOut)
	if err != nil {
		return err
	}
	views := make([]optoutView, 0, len(list))
	for _, req := range list {
		views = append(views, optoutView{Request: req, Language: locale.Name(req.Locale)})
	}

	data := optoutListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Opt-outs – %s", ws.Name)),
		FormState:  web.NewFormState(),
		WebsiteID:  ws.ID,
		Requests:   views,
		HasUS:      ws.HasUSFramework(),
	}
	data.ActiveNav = "optouts"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "optout_list", data)
}

// HandleOptOutHandled records that the operator answered an opt-out.
//
// The request is looked up through the website of the path before anything
// is written, so an id from another website is a 404 and changes nothing
// (T-33.1-28). The first answer is the one the record keeps: marking a
// request a second time leaves its date where it was (dsr.Store.MarkHandled).
// No confirmation step: marking costs nothing and removes nothing.
func (h *Handler) HandleOptOutHandled(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.optoutWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("requestID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	req, err := h.requests.Get(r.Context(), ws.ID, id)
	if err != nil {
		return err
	}
	if req == nil || req.Kind != dsr.KindOptOut {
		http.NotFound(w, r)
		return nil
	}
	if err := h.requests.MarkHandled(r.Context(), ws.ID, id); err != nil {
		if errors.Is(err, dsr.ErrNotFound) {
			http.NotFound(w, r)
			return nil
		}
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Opt-out marked as handled")
	return h.redirect(w, r, optoutListAddress(ws.ID))
}
