package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The Privacy screen of a website (Phase 33.2, DSR-01, D-01 and D-02).
//
// A person asks what a website holds about them. The operator types the
// person's e-mail address here and sees everything the website keeps under
// it — orders, messages, staff accounts that work on the website, privacy
// requests, queued mail — and downloads it as one JSON file to hand over.
// The search is dsr.Service: one list of sources, the same list erasure,
// correction and retention walk later.
//
// What the screen could not search comes first: a plugin that keeps data and
// cannot be asked, or a store that failed. An answer that leaves something
// out has to say so before it shows what it found.
//
// The rights are the order book's and the opt-out list's: anybody with access
// to the website may search and download, and RequireWebsiteAccess guards the
// path. Nothing here changes anything, so the search is a GET form and needs
// no confirmation. It is not offered over MCP: personal data stays in the
// admin, as 33.1 D-16 decided for opt-outs.
//
// The address travels in the query string. The request log writes the path
// only (internal/web/logging.go) and Referrer-Policy is same-origin, so the
// address reaches neither the journal nor another site.

// privacySourceOrder is the order the screen shows the core sources in;
// plugins follow, by name.
var privacySourceOrder = []string{"orders", "form_messages", "accounts", "requests", "mail"}

// privacySourceHeadings are the headings of the core sources, catalogue keys.
var privacySourceHeadings = map[string]string{
	"orders":        i18n.N("Orders"),
	"form_messages": i18n.N("Form messages (before the contact-form plugin)"),
	"accounts":      i18n.N("Staff accounts"),
	"requests":      i18n.N("Privacy requests"),
	"mail":          i18n.N("Queued mails"),
}

// privacyPersonData backs the screen.
type privacyPersonData struct {
	web.LayoutData
	web.FormState
	WebsiteID int64
	// Email is the address as typed, shown back in the field.
	Email string
	// Searched is true when a valid address was searched, even if nothing
	// was found.
	Searched bool
	// Count is the number of records found.
	Count  int
	Groups []privacyGroup
	// NotCovered are the plugins that keep data and were not asked.
	NotCovered []dsr.Holder
	// Failed are the stores that could not be searched, by heading.
	Failed []string
	// ExportURL is the download of what was found.
	ExportURL string
	// EraseURL is the erasure page of the address, with the request the
	// search came from.
	EraseURL string
}

// privacyGroup is the records of one source.
type privacyGroup struct {
	Source string
	// Heading is a catalogue key for a core source, the plugin's name for a
	// plugin.
	Heading string
	Records []privacyRecordView
}

// privacyRecordView is one record as the screen shows it.
type privacyRecordView struct {
	dsr.Record
	// Shown are the fields that hold a value: an empty company is not
	// information the operator needs to read. The export carries them all.
	Shown []dsr.Field
}

// privacyAddress is the screen of one website.
func privacyAddress(websiteID int64) string {
	return "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/datenschutz"
}

// privacyWebsite resolves the website of the path. ok is false when the
// answer has been written already: a malformed or unknown id, or a build
// without the search, is a 404.
func (h *Handler) privacyWebsite(w http.ResponseWriter, r *http.Request) (*domain.Website, bool, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || h.dataRights == nil {
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

// privacyHeading is the heading of a source: its catalogue key, the plugin's
// name for a plugin, or the source name itself.
func privacyHeading(source string, holders []dsr.Holder) string {
	if h, ok := privacySourceHeadings[source]; ok {
		return h
	}
	if id, ok := strings.CutPrefix(source, "plugin:"); ok {
		for _, h := range holders {
			if h.PluginID == id {
				return h.Name
			}
		}
		return id
	}
	return source
}

// privacyGroups puts the records in the screen's order and its clock.
func privacyGroups(res *dsr.Result, loc *time.Location) []privacyGroup {
	by := res.BySource()
	order := append([]string(nil), privacySourceOrder...)
	var plugins []string
	for source := range by {
		if strings.HasPrefix(source, "plugin:") {
			plugins = append(plugins, source)
		}
	}
	sort.Strings(plugins)
	order = append(order, plugins...)

	var groups []privacyGroup
	for _, source := range order {
		recs := by[source]
		if len(recs) == 0 {
			continue
		}
		g := privacyGroup{Source: source, Heading: privacyHeading(source, res.Holders)}
		for _, rec := range recs {
			if !rec.Time.IsZero() {
				rec.Time = rec.Time.In(loc)
			}
			view := privacyRecordView{Record: rec}
			for _, f := range rec.Fields {
				if f.Value == "" {
					continue
				}
				// A stored time is UTC; the screen reads it on the
				// website's clock, as the record's own time above it. The
				// export keeps the stored value.
				if strings.HasSuffix(f.Name, "_at") {
					if t, err := time.Parse(time.RFC3339, f.Value); err == nil {
						f.Value = t.In(loc).Format("02.01.2006 15:04 MST")
					}
				}
				view.Shown = append(view.Shown, f)
			}
			g.Records = append(g.Records, view)
		}
		groups = append(groups, g)
	}
	return groups
}

// HandlePrivacyPerson shows the search form and, with ?email=, what the
// website holds under that address.
func (h *Handler) HandlePrivacyPerson(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.privacyWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	data := privacyPersonData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Privacy – %s", ws.Name)),
		FormState:  web.NewFormState(),
		WebsiteID:  ws.ID,
	}
	data.ActiveNav = "privacy"
	data.CurrentWebsite = ws

	_, asked := r.URL.Query()["email"]
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	data.Email = email
	if asked {
		res, err := h.dataRights.Find(r.Context(), ws.ID, email)
		switch {
		case errors.Is(err, dsr.ErrBadEmail):
			data.Errors.Add("email", "This is not one e-mail address.")
			return web.RenderFormError(w, h.templates, r, "privacy_person", data)
		case err != nil:
			return err
		}
		data.Searched = true
		data.Count = len(res.Records)
		data.Groups = privacyGroups(res, tmpl.LoadLocation(ws.TimeZone))
		data.NotCovered = res.NotCovered()
		for _, f := range res.Failed {
			data.Failed = append(data.Failed, privacyHeading(f.Source, res.Holders))
		}
		data.ExportURL = privacyAddress(ws.ID) + "/export.json?email=" + url.QueryEscape(res.Email)
		requestID, _ := strconv.ParseInt(r.URL.Query().Get("anfrage"), 10, 64)
		data.EraseURL = privacyEraseAddress(ws.ID) + "?" + eraseQuery(res.Email, max(requestID, 0))
	}
	return web.RenderAdmin(w, h.templates, r, "privacy_person", data)
}

// HandlePrivacyExport answers with the JSON file of what the website holds
// under ?email=.
//
// A missing or invalid address is a 400 with no body: there is nothing to
// show, and a page would only be a second place to say what the screen says.
// The file name carries the website and the date, never the address — a
// download folder is not the place for somebody's address either. Every
// download writes one privacy.export entry with the number of records, and
// never the address: the log outlives an erasure, and must not keep what it
// erased.
func (h *Handler) HandlePrivacyExport(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.privacyWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	res, err := h.dataRights.Find(r.Context(), ws.ID, r.URL.Query().Get("email"))
	if errors.Is(err, dsr.ErrBadEmail) {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	if err != nil {
		return err
	}
	now := time.Now()
	body, err := json.MarshalIndent(dsr.BuildExport(res, ws.ID, ws.Name, now), "", "  ")
	if err != nil {
		return err
	}
	name := "personal-data-" + strconv.FormatInt(ws.ID, 10) + "-" + now.UTC().Format("2006-01-02") + ".json"
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	// Set here and not left to the admin's headers: the file is a person's
	// data, and that it is never cached must not depend on a middleware
	// staying in front of this handler.
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write(append(body, '\n')); err != nil {
		return nil
	}
	websiteID := ws.ID
	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionPrivacyExport,
		EntityType: "website",
		EntityID:   ws.ID,
		WebsiteID:  &websiteID,
		Metadata:   map[string]any{"records": len(res.Records)},
	})
	return nil
}
