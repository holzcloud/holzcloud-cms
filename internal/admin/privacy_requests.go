package admin

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
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
// (33.2 D-09), and this screen is where the operator works through them:
//
//   - when each request is due, per the frameworks the website follows
//     (dsr.DueFor) — shown, never enforced;
//   - the answer, recorded once with its outcome and basis: together with the
//     arrival, the kind and the manner it is the record CCPA regs § 7101 ask
//     a business to keep;
//   - a request that came by e-mail, letter or telephone, entered by hand
//     with the day it arrived;
//   - from each request, the person search with the request's address: the
//     request says what was asked, the search what the website holds.
//
// Checking that a request comes from the person stays with the operator.
//
// There is no delete. The list is the record, and deciding when a row may go
// is the retention of this phase; dsr.Store has no way to delete a request
// either. Data requests are not offered over MCP, like the opt-outs (33.1
// D-16).
//
// The rights are the opt-out list's: anybody with access to the website may
// read the requests, record an answer and enter a request. Recording publishes
// nothing — the answer itself goes to the person by the operator's own mail —
// so the publishing right is not the question here, as it is not for marking
// an opt-out handled. Both forms are plain POSTs: an answer refused for its
// missing basis comes back as the page with the error, which htmx would not
// show for a 422.

// privacyRequestsData backs the screen.
type privacyRequestsData struct {
	web.LayoutData
	web.FormState
	WebsiteID int64
	Requests  []privacyRequestView
	// NoFramework is true when the website follows no framework, so no
	// request has a deadline to show.
	NoFramework bool
	// Entry is what the entry form shows: empty, or what was typed when it
	// was refused. EntryOpen keeps its <details> open after a refusal.
	Entry     privacyEntry
	EntryOpen bool
	// AnswerFor is the request whose answer form the errors belong to, and
	// Answer what was typed there.
	AnswerFor int64
	Answer    privacyAnswer
	// The choices of the two forms.
	Kinds    []privacyChoice
	Manners  []privacyChoice
	Outcomes []privacyChoice
	// Today is the latest day the entry form accepts, in the website's zone.
	Today string
}

// privacyChoice is one option of a select: its value and its catalogue key.
type privacyChoice struct {
	Value, Name string
}

// privacyEntry is the entry form of a request that came by hand.
type privacyEntry struct {
	Kind, Manner, Received, Name, Email, Note string
}

// privacyAnswer is what was typed into an answer form.
type privacyAnswer struct {
	Outcome, Basis string
}

// privacyRequestView is one request as the screen shows it.
type privacyRequestView struct {
	dsr.Request
	// KindName, MannerName and OutcomeName are catalogue keys.
	KindName    string
	MannerName  string
	OutcomeName string
	// Language is the name of the page's language, a catalogue key.
	Language string
	// MailTo is the mailto: link to the address, empty when the address is
	// not one bare address (optoutMailTo).
	MailTo string
	// SearchURL opens the person search for the request's address, empty
	// when the address is not one the search takes.
	SearchURL string
	// Due is when the request has to be answered, and HasDue false when the
	// website follows no framework. DueBy is the framework's id, upper case.
	Due    dsr.Due
	HasDue bool
	DueBy  string
	// Overdue is an open request past its answer date.
	Overdue bool
}

// privacyRequestsAddress is the screen of one website.
func privacyRequestsAddress(websiteID int64) string {
	return privacyAddress(websiteID) + "/anfragen"
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
	data, err := h.privacyRequestsData(r, ws)
	if err != nil {
		return err
	}
	return web.RenderAdmin(w, h.templates, r, "privacy_requests", data)
}

// privacyRequestsData builds the screen of one website.
func (h *Handler) privacyRequestsData(r *http.Request, ws *domain.Website) (*privacyRequestsData, error) {
	list, err := h.requests.ListData(r.Context(), ws.ID)
	if err != nil {
		return nil, err
	}
	// The store keeps UTC; the screen shows the website's own clock and
	// names the zone, as the opt-out list does, because this is a record.
	// The due dates are counted in the same zone, so a request that arrived
	// late in the evening is due on the website's day and not on London's.
	loc := tmpl.LoadLocation(ws.TimeZone)
	set := ws.Frameworks()
	now := time.Now().In(loc)
	views := make([]privacyRequestView, 0, len(list))
	for _, req := range list {
		req.CreatedAt = req.CreatedAt.In(loc)
		if req.HandledAt != nil {
			handled := req.HandledAt.In(loc)
			req.HandledAt = &handled
		}
		v := privacyRequestView{
			Request:     req,
			KindName:    dsr.KindName(req.Kind),
			MannerName:  dsr.MannerName(req.Manner),
			OutcomeName: dsr.OutcomeName(req.Outcome),
			Language:    locale.Name(req.Locale),
			MailTo:      optoutMailTo(req.Email),
			SearchURL:   privacyRequestSearch(ws.ID, req),
		}
		v.Due, v.HasDue = dsr.DueFor(req.CreatedAt, set)
		if v.HasDue {
			v.DueBy = strings.ToUpper(string(v.Due.By))
			// Overdue once the whole answer day has passed.
			v.Overdue = req.Outcome == "" && now.After(endOfDay(v.Due.Answer))
		}
		views = append(views, v)
	}

	data := &privacyRequestsData{
		LayoutData:  web.NewLayoutData(r, h.sm, web.Titlef(r, "Requests about personal data – %s", ws.Name)),
		FormState:   web.NewFormState(),
		WebsiteID:   ws.ID,
		Requests:    views,
		NoFramework: set.Empty(),
		Entry:       privacyEntry{Kind: dsr.KindAccess, Manner: dsr.MannerEmail, Received: now.Format("2006-01-02")},
		Today:       now.Format("2006-01-02"),
	}
	for _, k := range dsr.DataKinds {
		data.Kinds = append(data.Kinds, privacyChoice{k, dsr.KindName(k)})
	}
	for _, m := range dsr.ReceivedManners {
		data.Manners = append(data.Manners, privacyChoice{m, dsr.MannerName(m)})
	}
	for _, o := range dsr.Outcomes {
		data.Outcomes = append(data.Outcomes, privacyChoice{o, dsr.OutcomeName(o)})
	}
	data.ActiveNav = "privacy-requests"
	data.CurrentWebsite = ws
	return data, nil
}

// endOfDay is the last moment of t's day.
func endOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, t.Location()).Add(-time.Nanosecond)
}

// privacyRequestOf resolves the request of the path on the website of the
// path. ok is false when the answer has been written already: a malformed id,
// a request of another website, or an opt-out, which is marked handled on its
// own list and not answered here, is a 404.
func (h *Handler) privacyRequestOf(w http.ResponseWriter, r *http.Request, ws *domain.Website) (*dsr.Request, bool, error) {
	id, err := strconv.ParseInt(r.PathValue("requestID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil, false, nil
	}
	req, err := h.requests.Get(r.Context(), ws.ID, id)
	if err != nil {
		return nil, false, err
	}
	if req == nil || !dsr.IsDataKind(req.Kind) {
		http.NotFound(w, r)
		return nil, false, nil
	}
	return req, true, nil
}

// HandlePrivacyRequestAnswer records how a data request was answered.
//
// The request is looked up through the website of the path before anything
// is written, so an id of another website is a 404 and changes nothing. A
// refusal and a partial answer need their basis — § 7101 asks for "the basis
// for the denial" — and are sent back with the error without one. The first
// answer is the record: a second is refused with a flash and changes nothing
// (dsr.Store.Answer). No confirmation step: recording removes nothing.
func (h *Handler) HandlePrivacyRequestAnswer(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.optoutWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	req, ok, err := h.privacyRequestOf(w, r, ws)
	if err != nil || !ok {
		return err
	}
	if req.Outcome != "" {
		web.SetFlashError(h.sm, r.Context(), "This request was already answered.")
		return h.redirect(w, r, privacyRequestsAddress(ws.ID))
	}
	answer := privacyAnswer{
		Outcome: r.PostFormValue("outcome"),
		Basis:   strings.TrimSpace(r.PostFormValue("basis")),
	}
	errs := web.FormErrors{}
	switch answer.Outcome {
	case dsr.OutcomeGranted:
	case dsr.OutcomePartly, dsr.OutcomeRefused:
		if answer.Basis == "" {
			errs.Add("basis", "Please give the basis for the refusal.")
		}
	default:
		errs.Add("outcome", "Please choose how the request was answered.")
	}
	if utf8.RuneCountInString(answer.Basis) > dsr.MaxBasis {
		errs.Add("basis", "An entry is longer than the form allows.")
	}
	if !errs.Any() {
		err = h.requests.Answer(r.Context(), ws.ID, req.ID, answer.Outcome, answer.Basis)
		switch {
		case errors.Is(err, dsr.ErrAnswered):
			web.SetFlashError(h.sm, r.Context(), "This request was already answered.")
			return h.redirect(w, r, privacyRequestsAddress(ws.ID))
		case errors.Is(err, dsr.ErrNotFound), errors.Is(err, dsr.ErrBadKind):
			http.NotFound(w, r)
			return nil
		case err != nil:
			return err
		}
	}
	if errs.Any() {
		data, err := h.privacyRequestsData(r, ws)
		if err != nil {
			return err
		}
		data.Errors = errs
		data.AnswerFor, data.Answer = req.ID, answer
		return web.RenderFormError(w, h.templates, r, "privacy_requests", data)
	}

	websiteID := ws.ID
	// The request and how it was answered, never the person: the log
	// outlives an erasure (activity/actions_privacy.go).
	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionPrivacyAnswer,
		EntityType: "privacy_request",
		EntityID:   req.ID,
		WebsiteID:  &websiteID,
		Metadata:   map[string]any{"request": req.ID, "outcome": answer.Outcome},
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Answer recorded")
	return h.redirect(w, r, privacyRequestsAddress(ws.ID))
}

// HandlePrivacyRequestNew enters a data request that came by e-mail, letter
// or telephone, with the day it arrived in the website's zone. The day starts
// the deadline, so it is the day the request arrived and not the day it is
// typed in, and it cannot lie ahead (dsr.Store.AddReceived).
func (h *Handler) HandlePrivacyRequestNew(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.optoutWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	entry := privacyEntry{
		Kind:     r.PostFormValue("kind"),
		Manner:   r.PostFormValue("manner"),
		Received: strings.TrimSpace(r.PostFormValue("received")),
		Name:     strings.TrimSpace(r.PostFormValue("name")),
		Email:    strings.TrimSpace(r.PostFormValue("email")),
		Note:     strings.TrimSpace(r.PostFormValue("note")),
	}
	errs := web.FormErrors{}
	loc := tmpl.LoadLocation(ws.TimeZone)
	received, perr := time.ParseInLocation("2006-01-02", entry.Received, loc)
	if perr != nil {
		errs.Add("received", "Please give the day the request arrived.")
	}
	if !dsr.ValidEmail(entry.Email) {
		errs.Add("email", "Please give the e-mail address the request is about.")
	}
	var id int64
	if !errs.Any() {
		id, err = h.requests.AddReceived(r.Context(), dsr.Request{
			WebsiteID: ws.ID, Kind: entry.Kind, Manner: entry.Manner,
			Name: entry.Name, Email: entry.Email, Note: entry.Note, CreatedAt: received,
		})
		switch {
		case errors.Is(err, dsr.ErrBadKind):
			errs.Add("kind", "Please choose what the person asks for.")
		case errors.Is(err, dsr.ErrBadManner):
			errs.Add("manner", "Please choose how the request arrived.")
		case errors.Is(err, dsr.ErrFutureTime), errors.Is(err, dsr.ErrNoTime):
			errs.Add("received", "Please give the day the request arrived.")
		case errors.Is(err, dsr.ErrNeedsEmail), errors.Is(err, dsr.ErrBadEmail):
			errs.Add("email", "Please give the e-mail address the request is about.")
		case errors.Is(err, dsr.ErrTooLong):
			errs.Add("note", "An entry is longer than the form allows.")
		case err != nil:
			return err
		}
	}
	if errs.Any() {
		data, err := h.privacyRequestsData(r, ws)
		if err != nil {
			return err
		}
		data.Errors = errs
		data.Entry, data.EntryOpen = entry, true
		return web.RenderFormError(w, h.templates, r, "privacy_requests", data)
	}

	websiteID := ws.ID
	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionPrivacyRequestAdd,
		EntityType: "privacy_request",
		EntityID:   id,
		WebsiteID:  &websiteID,
		Metadata:   map[string]any{"request": id, "kind": entry.Kind, "manner": entry.Manner},
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Request entered")
	return h.redirect(w, r, privacyRequestsAddress(ws.ID))
}
