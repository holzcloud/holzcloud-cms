package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// Erasing a person's data on one website (Phase 33.2, DSR-03, D-03, D-07,
// D-08, D-09).
//
// The confirmation is a page, not an htmx question: the preview lists every
// record that will go, every record that stays reduced or kept — with its
// fields, its reason and the date it is deleted — and the button under it
// posts. Both the page and the POST stand behind the password asked again
// (requireFresh in main.go), so the page works without JavaScript and an
// unattended session reaches neither. There is no hx-confirm: the page is the
// second thought, and the count of internal/web/nojs_test.go stays 22.
//
// Who may: anybody with access to the website may look at the preview; the
// erasure needs the publishing right (refuseReadOnly, asked first), because
// what it deletes does not come back, and the staff account part needs an
// administrator — the account belongs to the installation, not to this
// website. A box an editor ticks for the account is ignored and the result
// says so.
//
// The POST sends the address, the request it answers and the ticked groups,
// and nothing else is read from it: no record key, id or media id. dsr.Erase
// searches the website of the path again and erases what that search finds.
//
// The result is shown once. The POST leaves it in the session, without the
// address, and the result page takes it out; reloading the result page leads
// back to the Privacy screen. Nothing personal is written to the journal or
// the activity log.

// The groups an operator can tick, in the order the page shows them, with
// their names: the core headings of the Privacy screen, and one for every
// plugin.
var eraseGroups = []string{dsr.EraseOrders, dsr.EraseFormMessages, dsr.EraseMail, dsr.ErasePlugins, dsr.EraseAccounts}

var eraseGroupLabels = map[string]string{
	dsr.EraseOrders:       privacySourceHeadings["orders"],
	dsr.EraseFormMessages: privacySourceHeadings["form_messages"],
	dsr.EraseMail:         privacySourceHeadings["mail"],
	dsr.ErasePlugins:      i18n.N("Plugins"),
	dsr.EraseAccounts:     privacySourceHeadings["accounts"],
}

// sessionKeyErased holds the result of the last erasure until the result
// page shows it.
const sessionKeyErased = "privacy_erased"

// privacyEraseData backs the erasure page.
type privacyEraseData struct {
	web.LayoutData
	web.FormState
	WebsiteID int64
	// Email is the address as searched, sent back with the form.
	Email string
	// RequestID is the request the erasure answers, 0 for none.
	RequestID int64
	// Action is the page's own address, where the form posts.
	Action string
	// Back is the Privacy screen with the address searched.
	Back string
	// Delete, Reduce and Keep are the records by what the erasure does with
	// them; Accounts are the staff accounts, which only an administrator
	// erases.
	Delete, Reduce, Keep, Accounts []erasureView
	// Boxes are the groups the operator ticks.
	Boxes []erasureBox
	// NotCovered are the plugins that keep data and cannot be asked.
	NotCovered []dsr.Holder
	// Failed are the stores that could not be searched, by heading.
	Failed []string
	// ReadOnly is true for somebody who may look but not erase: the page
	// shows no button and ReadOnlyNotice says why.
	ReadOnly       bool
	ReadOnlyNotice string
	// IsAdmin offers the account box.
	IsAdmin bool
	// Answers is true when the erasure will record the answer of the
	// request it was opened from.
	Answers bool
	// Nothing is true when the address finds nothing to erase.
	Nothing bool
	// Snapshots are the pre-upgrade snapshots in the data directory, which
	// still hold the data (D-08).
	Snapshots []erasureSnapshot
}

// erasureView is one record as the erasure page and the result show it. It
// travels through the session as JSON, so it carries text only, and no
// address: the key of an order is its number.
type erasureView struct {
	// Heading names the store, a catalogue key or a plugin's name.
	Heading string `json:"heading"`
	// Label and Key name the record.
	Label string `json:"label"`
	Key   string `json:"key"`
	// Gone and Kept are what goes and stays of a reduced record.
	Gone []string `json:"gone,omitempty"`
	Kept []string `json:"kept,omitempty"`
	// Reason and Basis are catalogue keys, "" when there is none.
	Reason string `json:"reason,omitempty"`
	Basis  string `json:"basis,omitempty"`
	// Until is the date the record is deleted, "" when it goes now.
	Until string `json:"until,omitempty"`
}

// erasureBox is one group the operator can tick.
type erasureBox struct {
	Name    string
	Label   string
	Checked bool
}

// erasedReport is what the result page shows, as the POST left it in the
// session.
type erasedReport struct {
	WebsiteID int64         `json:"website"`
	Orders    int           `json:"orders"`
	Deleted   []erasedCount `json:"deleted,omitempty"`
	Reduced   []erasureView `json:"reduced,omitempty"`
	Kept      []erasureView `json:"kept,omitempty"`
	// Failed are the sentences of what could not be erased, each with the
	// heading of its store.
	Failed []erasedFailure `json:"failed,omitempty"`
	// FileErrors name the files that stayed on the disk.
	FileErrors []string `json:"file_errors,omitempty"`
	// NotCovered are the names of the plugins that keep data for the
	// website and could not be asked: what they keep was not erased.
	NotCovered []string `json:"not_covered,omitempty"`
	// RequestAnswered is true when the erasure answered the request it was
	// made for.
	RequestAnswered bool `json:"request_answered,omitempty"`
}

// erasedCount is how many records of one group went.
type erasedCount struct {
	Heading string `json:"heading"`
	Count   int    `json:"count"`
}

// erasedFailure is one thing that could not be erased.
type erasedFailure struct {
	Heading  string `json:"heading"`
	Sentence string `json:"sentence"`
}

// privacyErasedData backs the result page.
type privacyErasedData struct {
	web.LayoutData
	WebsiteID int64
	Report    erasedReport
	// Snapshots are the pre-upgrade snapshots that still hold the data.
	Snapshots []erasureSnapshot
}

// What backups and snapshots still hold (D-08).
//
// An erasure does not rewrite them, and that is the decision, not an
// oversight: a pre-upgrade snapshot is the way back after a failed update,
// and a backup is the copy a restore needs; an erasure that edited them would
// make exactly those unreliable. What the operator is told instead is which
// ones still hold the data and when they age out — the pre-upgrade snapshots
// in the data directory by name and date, the nightly backups of
// deploy/backup.sh by their period — and that an erasure made after a backup
// has to be repeated when that backup is restored. The ICO asks for backups
// to be kept "beyond use" and for the person to be told, which is what these
// sentences let the operator do.
//
// The snapshots are listed from the data directory as the service writes
// them (pre-upgrade-<version>-<YYYYMMDD-HHMMSS>.sqlite, the newest five kept,
// cmd/holzcloud's snapshotBeforeMigrating); their date is the stamp in the
// name, as db.PruneSnapshots reads it, and the modification time for a name
// without one.

// snapshotStampLayout is the UTC time at the end of a snapshot's name.
const snapshotStampLayout = "20060102-150405"

// erasureSnapshot is one pre-upgrade snapshot as the pages list it.
type erasureSnapshot struct {
	Name string
	// Date is when it was written, on the UTC clock the name carries.
	Date string
	at   time.Time
}

// erasureSnapshots lists the pre-upgrade snapshots in the data directory,
// newest first. A data directory that cannot be read lists none.
func (h *Handler) erasureSnapshots() []erasureSnapshot {
	if h.cfg == nil || h.cfg.DataDir == "" {
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(h.cfg.DataDir, "pre-upgrade-*.sqlite"))
	out := make([]erasureSnapshot, 0, len(matches))
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		name := filepath.Base(path)
		at := info.ModTime().UTC()
		if base := strings.TrimSuffix(name, ".sqlite"); len(base) > len(snapshotStampLayout) {
			if t, err := time.Parse(snapshotStampLayout, base[len(base)-len(snapshotStampLayout):]); err == nil {
				at = t
			}
		}
		out = append(out, erasureSnapshot{Name: name, Date: at.Format("02.01.2006 15:04 MST"), at: at})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].at.After(out[j].at) })
	return out
}

// privacyEraseAddress is the erasure page of one website.
func privacyEraseAddress(websiteID int64) string { return privacyAddress(websiteID) + "/loeschen" }

// privacyErasedAddress is the result page of one website.
func privacyErasedAddress(websiteID int64) string { return privacyAddress(websiteID) + "/geloescht" }

// eraseQuery is the address and the request as a query string.
func eraseQuery(email string, requestID int64) string {
	q := url.Values{"email": {email}}
	if requestID > 0 {
		q.Set("anfrage", strconv.FormatInt(requestID, 10))
	}
	return q.Encode()
}

// eraseGroupOf is the group a record's source belongs to.
func eraseGroupOf(source string) string {
	if strings.HasPrefix(source, "plugin:") {
		return dsr.ErasePlugins
	}
	return source
}

// erasureDate is the date a record is deleted, as the screens print it. The
// date is 1 January in UTC; it is printed in UTC so that no clock west of
// Greenwich turns it into 31 December.
func erasureDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("02.01.2006")
}

// erasureViewOf is one item of the erasure as the screens show it.
func erasureViewOf(it dsr.ErasureItem, holders []dsr.Holder) erasureView {
	return erasureView{
		Heading: privacyHeading(it.Source, holders),
		Label:   it.Label,
		Key:     it.Key,
		Gone:    it.Gone,
		Kept:    it.Kept,
		Reason:  it.Reason,
		Basis:   it.Basis,
		Until:   erasureDate(it.Until),
	}
}

// HandlePrivacyErase shows what erasing ?email= would do (GET) and does it
// (POST). Both stand behind requireFresh in main.go.
func (h *Handler) HandlePrivacyErase(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.privacyWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	if r.Method == http.MethodPost {
		return h.erasePrivacy(w, r, ws)
	}
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	requestID, _ := strconv.ParseInt(r.URL.Query().Get("anfrage"), 10, 64)
	prev, err := h.dataRights.Preview(r.Context(), ws.ID, ws.Country, email, time.Now())
	if errors.Is(err, dsr.ErrBadEmail) {
		web.SetFlashError(h.sm, r.Context(), "This is not one e-mail address.")
		return h.redirect(w, r, privacyAddress(ws.ID))
	}
	if err != nil {
		return err
	}

	data := privacyEraseData{
		LayoutData:     web.NewLayoutData(r, h.sm, web.Titlef(r, "Erase personal data – %s", ws.Name)),
		FormState:      web.NewFormState(),
		WebsiteID:      ws.ID,
		Email:          email,
		RequestID:      max(requestID, 0),
		Action:         privacyEraseAddress(ws.ID),
		Back:           privacyAddress(ws.ID) + "?" + eraseQuery(email, max(requestID, 0)),
		NotCovered:     prev.NotCovered,
		ReadOnly:       h.readOnly(r),
		ReadOnlyNotice: readOnlySentence(),
		IsAdmin:        h.isAdmin(r),
	}
	data.ActiveNav = "privacy"
	data.CurrentWebsite = ws
	if requestID > 0 && h.requests != nil {
		req, err := h.requests.Get(r.Context(), ws.ID, requestID)
		if err != nil {
			return err
		}
		data.Answers = req != nil && req.Kind == dsr.KindErasure && req.Outcome == ""
	}
	for _, f := range prev.Failed {
		data.Failed = append(data.Failed, privacyHeading(f.Source, prev.Holders))
	}
	present := map[string]bool{}
	for _, it := range prev.Items {
		v := erasureViewOf(it, prev.Holders)
		group := eraseGroupOf(it.Source)
		switch {
		case group == dsr.EraseAccounts:
			data.Accounts = append(data.Accounts, v)
		case it.Action == dsr.ActionReduce:
			data.Reduce = append(data.Reduce, v)
		case it.Action == dsr.ActionKeep:
			data.Keep = append(data.Keep, v)
		default:
			data.Delete = append(data.Delete, v)
		}
		if it.Action != dsr.ActionKeep {
			present[group] = true
		}
	}
	for _, g := range eraseGroups {
		if !present[g] {
			continue
		}
		if g == dsr.EraseAccounts && !data.IsAdmin {
			continue
		}
		// Ticked by default, the staff account not: deleting a colleague's
		// account is a decision of its own.
		data.Boxes = append(data.Boxes, erasureBox{Name: g, Label: eraseGroupLabels[g], Checked: g != dsr.EraseAccounts})
	}
	data.Nothing = len(data.Boxes) == 0 && len(data.Accounts) == 0
	data.Snapshots = h.erasureSnapshots()
	return web.RenderAdmin(w, h.templates, r, "privacy_erase", data)
}

// erasePrivacy erases what the form asks for, leaves the result in the
// session and goes to the result page.
func (h *Handler) erasePrivacy(w http.ResponseWriter, r *http.Request, ws *domain.Website) error {
	if h.refuseReadOnly(w, r, privacyAddress(ws.ID)) {
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	ctx := r.Context()
	email := strings.TrimSpace(r.PostForm.Get("email"))
	requestID, _ := strconv.ParseInt(r.PostForm.Get("anfrage"), 10, 64)
	sources := map[string]bool{}
	for _, v := range r.PostForm["quelle"] {
		for _, g := range eraseGroups {
			if v == g {
				sources[g] = true
			}
		}
	}
	opt := dsr.EraseOptions{
		Email:       email,
		Sources:     sources,
		Accounts:    sources[dsr.EraseAccounts] && h.isAdmin(r),
		ActorUserID: h.sm.GetInt64(ctx, auth.SessionKeyUserID),
		ActorEmail:  h.sm.GetString(ctx, auth.SessionKeyUserEmail),
		RequestID:   max(requestID, 0),
		Now:         time.Now(),
	}
	rep, err := h.dataRights.Erase(ctx, ws.ID, ws.Country, opt)
	if errors.Is(err, dsr.ErrBadEmail) {
		web.SetFlashError(h.sm, ctx, "This is not one e-mail address.")
		return h.redirect(w, r, privacyAddress(ws.ID))
	}
	if err != nil {
		return err
	}
	body, err := json.Marshal(erasedReportOf(ws.ID, rep))
	if err != nil {
		return err
	}
	h.sm.Put(ctx, sessionKeyErased, string(body))
	return h.redirect(w, r, privacyErasedAddress(ws.ID))
}

// erasedReportOf is the result as the result page shows it.
func erasedReportOf(websiteID int64, rep *dsr.ErasureReport) erasedReport {
	out := erasedReport{WebsiteID: websiteID}
	for _, g := range eraseGroups {
		if n := rep.Deleted[g]; n > 0 {
			out.Deleted = append(out.Deleted, erasedCount{Heading: eraseGroupLabels[g], Count: n})
		}
	}
	for _, it := range rep.Reduced {
		if it.Source == dsr.EraseOrders {
			out.Orders++
		}
		out.Reduced = append(out.Reduced, erasureViewOf(it, rep.Holders))
	}
	for _, it := range rep.Kept {
		out.Kept = append(out.Kept, erasureViewOf(it, rep.Holders))
	}
	for _, f := range rep.Failed {
		out.Failed = append(out.Failed, erasedFailure{
			Heading:  privacyHeading(f.Source, rep.Holders),
			Sentence: dsr.ErasureFailure(f.Err),
		})
	}
	out.FileErrors = rep.FileErrors
	for _, hd := range rep.NotCovered {
		out.NotCovered = append(out.NotCovered, hd.Name)
	}
	out.RequestAnswered = rep.RequestAnswered
	return out
}

// HandlePrivacyErased shows the result of the last erasure, once. Without
// one it goes to the Privacy screen.
func (h *Handler) HandlePrivacyErased(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.privacyWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	var rep erasedReport
	raw := h.sm.PopString(r.Context(), sessionKeyErased)
	if raw == "" || json.Unmarshal([]byte(raw), &rep) != nil || rep.WebsiteID != ws.ID {
		return h.redirect(w, r, privacyAddress(ws.ID))
	}
	data := privacyErasedData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Erasure done – %s", ws.Name)),
		WebsiteID:  ws.ID,
		Report:     rep,
		Snapshots:  h.erasureSnapshots(),
	}
	data.ActiveNav = "privacy"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "privacy_erased", data)
}
