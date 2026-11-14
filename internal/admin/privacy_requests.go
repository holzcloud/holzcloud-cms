package admin

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The Requests screen under Settings → Privacy (Phase 33.2, D-01, DSR-01..03).
//
// A person asks a website for a copy of their data, for it to be corrected,
// or for it to be erased — through the [[datenanfrage]] form of the website
// (internal/public/datarequest.go), or by e-mail, letter or telephone. Every
// such request is a row of privacy_requests, the table the opt-outs are in
// (33.2 D-09), and this screen lists the website's data requests. The
// opt-outs keep their own list.
//
// From each request the operator opens the person search with the request's
// address: the request says what was asked, the search what the website
// holds. Checking that the request comes from the person, and answering it,
// stay with the operator.
//
// There is no delete. The list is the record CCPA regs § 7101 ask a business
// to keep for 24 months, and deciding when a row may go is the retention of
// this phase; dsr.Store has no way to delete a request either. Data requests
// are not offered over MCP, like the opt-outs (33.1 D-16).
//
// The rights are the opt-out list's: anybody with access to the website.

// privacyRequestsData backs the screen.
type privacyRequestsData struct {
	web.LayoutData
	web.FormState
	WebsiteID int64
	Requests  []privacyRequestView
}

// privacyRequestView is one request as the screen shows it.
type privacyRequestView struct {
	dsr.Request
	// KindName is the name of the kind, a catalogue key.
	KindName string
	// Language is the name of the page's language, a catalogue key.
	Language string
	// MailTo is the mailto: link to the address, empty when the address is
	// not one bare address (optoutMailTo).
	MailTo string
	// SearchURL opens the person search for the request's address, empty
	// when the address is not one the search takes.
	SearchURL string
}

// privacyRequestSearch is the person search for a request: the address, and
// the request's id so that the search can come back to it. Written out
// rather than through url.Values, whose sorted keys would put the id first.
func privacyRequestSearch(websiteID int64, req dsr.Request) string {
	if !dsr.ValidEmail(req.Email) {
		return ""
	}
	return privacyAddress(websiteID) + "?email=" + url.QueryEscape(req.Email) +
		"&anfrage=" + strconv.FormatInt(req.ID, 10)
}

// HandlePrivacyRequests lists a website's data requests, newest first.
func (h *Handler) HandlePrivacyRequests(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.optoutWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	list, err := h.requests.ListData(r.Context(), ws.ID)
	if err != nil {
		return err
	}
	// The store keeps UTC; the screen shows the website's own clock and
	// names the zone, as the opt-out list does, because this is a record.
	loc := tmpl.LoadLocation(ws.TimeZone)
	views := make([]privacyRequestView, 0, len(list))
	for _, req := range list {
		req.CreatedAt = req.CreatedAt.In(loc)
		if req.HandledAt != nil {
			handled := req.HandledAt.In(loc)
			req.HandledAt = &handled
		}
		views = append(views, privacyRequestView{
			Request:   req,
			KindName:  dsr.KindName(req.Kind),
			Language:  locale.Name(req.Locale),
			MailTo:    optoutMailTo(req.Email),
			SearchURL: privacyRequestSearch(ws.ID, req),
		})
	}

	data := privacyRequestsData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Requests about personal data – %s", ws.Name)),
		FormState:  web.NewFormState(),
		WebsiteID:  ws.ID,
		Requests:   views,
	}
	data.ActiveNav = "privacy-requests"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "privacy_requests", data)
}
