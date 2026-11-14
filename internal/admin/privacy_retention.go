package admin

import (
	"errors"
	"net/http"
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

// The Retention screen under Settings → Privacy (Phase 33.2, DSR-04, D-04,
// D-05, D-09).
//
// It shows how long the website keeps each kind of record, lets the operator
// change it, and shows what the retention job did the last time it ran. The
// server log is shown with its period and cannot be changed here: journald
// keeps it for the whole server.
//
// Reading is open to everybody with access to the website. Changing a period
// is not: a shortened period deletes data at the next run of the job, with no
// further question, so it needs the publishing right (refuseReadOnly, asked
// first) and the password again (requireFresh in front of the route in
// main.go). There is no htmx confirmation: the password is the step in front.
// The form is a plain POST, because a refused period comes back as the page
// with the error, which htmx would not show for a 422.

// The labels and units of the kinds, catalogue keys.
var (
	retentionLabels = map[string]string{
		dsr.RetentionFormSubmission: i18n.N("Form submissions"),
		dsr.RetentionRequest:        i18n.N("Privacy requests"),
		dsr.RetentionOrder:          i18n.N("Orders"),
	}
	retentionMonths     = i18n.N("months")
	retentionYears      = i18n.N("years")
	retentionBadPeriod  = i18n.N("A period must be between 1 and 600 months.")
	retentionStatusName = map[string]string{
		dsr.RunRunning: i18n.N("Running"),
		dsr.RunOK:      i18n.N("Successful"),
		dsr.RunFailed:  i18n.N("Failed"),
	}
)

// privacyRetentionData backs the screen.
type privacyRetentionData struct {
	web.LayoutData
	web.FormState
	WebsiteID int64
	Periods   []retentionPeriodView
	// LogDays is the period of the server log, which journald enforces.
	LogDays int
	// Run is the last run that reached the website, nil before the first.
	Run *retentionRunView
	// ReadOnly is true for somebody who may not change the periods; the
	// form is shown disabled with ReadOnlyNotice above it.
	ReadOnly       bool
	ReadOnlyNotice string
}

// retentionPeriodView is one kind as the form shows it.
type retentionPeriodView struct {
	Kind string
	// Label and Unit are catalogue keys.
	Label, Unit string
	// Value is what the field shows: the period in its unit, or what was
	// typed when the save was refused.
	Value string
	// Default, Min, Max and Step are in the unit.
	Default, Min, Max, Step int
	// IsDefault is true when the website has not chosen its own period.
	IsDefault bool
	// Error is the refusal, a catalogue key, with ErrorArg its number when
	// the sentence carries one.
	Error    string
	ErrorArg int
}

// retentionRunView is the last run as the screen shows it.
type retentionRunView struct {
	// Started is on the website's clock.
	Started time.Time
	Status  string
	// StatusName is a catalogue key.
	StatusName string
	// Deleted is this website's count.
	Deleted int
}

// privacyRetentionAddress is the screen of one website.
func privacyRetentionAddress(websiteID int64) string {
	return privacyAddress(websiteID) + "/aufbewahrung"
}

// HandlePrivacyRetention shows the retention periods of a website (GET) and
// stores the ones the form sends (POST).
func (h *Handler) HandlePrivacyRetention(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.privacyWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	if r.Method == http.MethodPost {
		return h.savePrivacyRetention(w, r, ws)
	}
	data, err := h.privacyRetentionData(r, ws, nil)
	if err != nil {
		return err
	}
	return web.RenderAdmin(w, h.templates, r, "privacy_retention", data)
}

// savePrivacyRetention stores the periods of the form.
//
// A field the form did not send keeps its period. Every value is checked
// before any is written (dsr.Service.SetPeriods), so a refused save changes
// nothing and comes back as the page with what was typed.
func (h *Handler) savePrivacyRetention(w http.ResponseWriter, r *http.Request, ws *domain.Website) error {
	back := privacyRetentionAddress(ws.ID)
	if h.refuseReadOnly(w, r, back) {
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	periods, err := h.dataRights.Periods(r.Context(), ws.ID, ws.Country)
	if err != nil {
		return err
	}
	typed := map[string]string{}
	months := map[string]int{}
	errs := web.FormErrors{}
	for _, p := range periods {
		if !r.PostForm.Has(p.Kind) {
			continue
		}
		raw := strings.TrimSpace(r.PostForm.Get(p.Kind))
		typed[p.Kind] = raw
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 600 {
			errs.Add(p.Kind, retentionBadPeriod)
			continue
		}
		months[p.Kind] = n * retentionUnitMonths(p.Kind)
	}
	if !errs.Any() {
		err := h.dataRights.SetPeriods(r.Context(), ws.ID, ws.Country, months)
		var pe *dsr.PeriodError
		switch {
		case errors.As(err, &pe):
			errs.Add(pe.Kind, retentionBadPeriod)
		case err != nil:
			return err
		}
	}
	if errs.Any() {
		data, err := h.privacyRetentionData(r, ws, typed)
		if err != nil {
			return err
		}
		data.Errors = errs
		for i := range data.Periods {
			data.Periods[i].Error = errs[data.Periods[i].Kind]
		}
		return web.RenderFormError(w, h.templates, r, "privacy_retention", data)
	}

	websiteID := ws.ID
	meta := map[string]any{}
	for kind, m := range months {
		meta[kind] = m
	}
	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionPrivacyPeriods,
		EntityType: "website",
		EntityID:   ws.ID,
		WebsiteID:  &websiteID,
		Metadata:   meta,
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Retention periods saved.")
	return h.redirect(w, r, back)
}

// retentionUnitMonths is how many months one unit of the kind's field is.
func retentionUnitMonths(kind string) int {
	if kind == dsr.RetentionOrder {
		return 12
	}
	return 1
}

// privacyRetentionData builds the screen. typed are the values of a refused
// save, by kind.
func (h *Handler) privacyRetentionData(r *http.Request, ws *domain.Website, typed map[string]string) (*privacyRetentionData, error) {
	ctx := r.Context()
	periods, err := h.dataRights.Periods(ctx, ws.ID, ws.Country)
	if err != nil {
		return nil, err
	}
	last, err := h.dataRights.LastRun(ctx, ws.ID)
	if err != nil {
		return nil, err
	}
	loc := tmpl.LoadLocation(ws.TimeZone)

	data := &privacyRetentionData{
		LayoutData:     web.NewLayoutData(r, h.sm, web.Titlef(r, "Retention – %s", ws.Name)),
		FormState:      web.NewFormState(),
		WebsiteID:      ws.ID,
		LogDays:        web.LogRetentionDays,
		ReadOnly:       h.readOnly(r),
		ReadOnlyNotice: readOnlySentence(),
	}
	for _, p := range periods {
		unit := retentionUnitMonths(p.Kind)
		v := retentionPeriodView{
			Kind:      p.Kind,
			Label:     retentionLabels[p.Kind],
			Unit:      retentionMonths,
			Value:     strconv.Itoa(p.Months / unit),
			Default:   p.Default / unit,
			Min:       p.Floor / unit,
			Max:       600 / unit,
			Step:      p.Step / unit,
			IsDefault: !p.Stored,
		}
		if unit == 12 {
			v.Unit = retentionYears
		}
		if s, ok := typed[p.Kind]; ok {
			v.Value = s
		}
		data.Periods = append(data.Periods, v)
	}
	if last != nil {
		data.Run = &retentionRunView{
			Started:    last.Started.In(loc),
			Status:     last.Status,
			StatusName: retentionStatusName[last.Status],
			Deleted:    last.Deleted,
		}
	}
	data.ActiveNav = "privacy-retention"
	data.CurrentWebsite = ws
	return data, nil
}
