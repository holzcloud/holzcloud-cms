package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/value"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The Values screen: the short texts a website keeps once and its pages name
// as [[wert:key]] (phase 33, D-06). It follows the snippet screen — one list,
// one form that creates or, opened with ?edit=, changes — because an editor
// who knows one finds the other.
//
// A value is online the moment it is stored: the next response of every page
// that names it carries the new text. So the publishing-right rule of phase 31
// holds here as on menus and snippets (D-01). It is asked in these handlers and
// nowhere below them; internal/value and the MCP tools never ask it, because an
// AI key belongs to no person and follows its own level (Phase 31 D-11).

// screenValueListData is what the Values screen renders.
type screenValueListData struct {
	web.LayoutData
	web.FormState
	WebsiteID int64
	Rows      []screenValueRow
	// Form is the create/edit form, re-rendered with what was submitted when
	// a save is refused.
	Form   screenValueForm
	IsEdit bool
	// ReadOnly is somebody without the publishing right: they see the values
	// and may open one, and have nothing to send.
	ReadOnly bool
	// MaxText is the longest text in bytes, for the input's maxlength and the
	// hint beside it.
	MaxText int
}

// screenValueRow is one value with how often it is named.
type screenValueRow struct {
	value.Value
	// Pages and Snippets count the pages (any language, drafts included, not
	// the wastebasket) and the snippets that name the key — the same reading
	// the delete refusal makes.
	Pages    int
	Snippets int
}

// screenValueForm is exactly what the form submitted, or the value opened.
type screenValueForm struct {
	ID   int64
	Key  string
	Text string
}

func screenValueFormFromRequest(r *http.Request) screenValueForm {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	return screenValueForm{
		ID:   id,
		Key:  strings.TrimSpace(r.FormValue("key")),
		Text: r.FormValue("text"),
	}
}

func screenValueListAddress(websiteID int64) string {
	return fmt.Sprintf("/admin/websites/%d/werte", websiteID)
}

// HandleValueList renders the values of a website with the form, and saves
// the form on POST.
func (h *Handler) HandleValueList(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || h.valueStore == nil {
		http.NotFound(w, r)
		return nil
	}
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			return err
		}
		form := screenValueFormFromRequest(r)
		// An edit names a value by id, and that value must be this website's
		// before anybody is asked about rights: a foreign id is not this
		// person's business and answers 404, as on every scoped screen (D-03).
		if form.ID != 0 {
			existing, err := h.valueStore.Get(r.Context(), websiteID, form.ID)
			if err != nil {
				return err
			}
			if existing == nil {
				http.NotFound(w, r)
				return nil
			}
			// The key of an existing value is never taken from the form: a
			// renamed key would empty every sentence that names the old one
			// (D-13, A5).
			form.Key = existing.Key
		}
		if h.refuseReadOnly(w, r, screenValueListAddress(websiteID)) {
			return nil
		}
		return h.screenValueSave(w, r, websiteID, ws.Name, form)
	}

	form := screenValueForm{}
	isEdit := false
	if raw := r.URL.Query().Get("edit"); raw != "" {
		id, _ := strconv.ParseInt(raw, 10, 64)
		v, err := h.valueStore.Get(r.Context(), websiteID, id)
		if err != nil {
			return err
		}
		if v != nil {
			form = screenValueForm{ID: v.ID, Key: v.Key, Text: v.Text}
			isEdit = true
		}
	}

	data, err := h.screenValueListData(r, websiteID, ws.Name, form)
	if err != nil {
		return err
	}
	data.IsEdit = isEdit
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "value_list", data)
}

// screenValueSave stores the form, or re-renders it with the reason at the
// field. The store checks the key and the text itself; this only says which
// field a refusal belongs to, in the operator's words.
func (h *Handler) screenValueSave(w http.ResponseWriter, r *http.Request, websiteID int64, websiteName string, form screenValueForm) error {
	var err error
	if form.ID == 0 {
		_, err = h.valueStore.Create(r.Context(), websiteID, form.Key, form.Text)
	} else {
		err = h.valueStore.UpdateText(r.Context(), websiteID, form.ID, form.Text)
	}
	if errors.Is(err, value.ErrNotFound) {
		http.NotFound(w, r)
		return nil
	}
	if err == nil {
		web.SetFlashSuccess(h.sm, r.Context(), "Value saved")
		return h.redirect(w, r, screenValueListAddress(websiteID))
	}

	data, dataErr := h.screenValueListData(r, websiteID, websiteName, form)
	if dataErr != nil {
		return dataErr
	}
	data.IsEdit = form.ID != 0
	if ws, wsErr := h.domains.GetWebsite(r.Context(), websiteID); wsErr == nil {
		data.CurrentWebsite = ws
	}
	// Each sentence is written out at its own Add, so the catalogue tool sees
	// every one of them.
	switch {
	case errors.Is(err, value.ErrBadKey):
		data.Errors.Add("key", "Lower-case letters, digits, hyphen and underscore only.")
	case errors.Is(err, value.ErrKeyTaken):
		data.Errors.Add("key", "That key is already used by another value.")
	case errors.Is(err, value.ErrTextEmpty):
		data.Errors.Add("text", "Please enter a text.")
	case errors.Is(err, value.ErrTextTooLong):
		data.Errors.Add("text", "The text is longer than 400 bytes.")
	case errors.Is(err, value.ErrTextInvalid):
		data.Errors.Add("text", "The text must be one line, without line breaks or control characters.")
	case errors.Is(err, value.ErrTextMarker):
		data.Errors.Add("text", "A value cannot contain [[.")
	default:
		return err
	}
	return web.RenderFormError(w, h.templates, r, "value_list", data)
}

// screenValueListData loads the list and how often each value is named.
func (h *Handler) screenValueListData(r *http.Request, websiteID int64, websiteName string, form screenValueForm) (screenValueListData, error) {
	data := screenValueListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Values – %s", websiteName)),
		FormState:  web.NewFormState(),
		WebsiteID:  websiteID,
		Form:       form,
		ReadOnly:   h.readOnly(r),
		MaxText:    value.MaxTextBytes,
	}
	data.ActiveNav = "values"

	list, err := h.valueStore.List(r.Context(), websiteID)
	if err != nil {
		return data, err
	}
	report, err := h.valueStore.Report(r.Context(), websiteID)
	if err != nil {
		return data, err
	}
	for _, v := range list {
		row := screenValueRow{Value: v}
		for _, u := range report.Used[v.Key] {
			if u.Kind == "snippet" {
				row.Snippets++
			} else {
				row.Pages++
			}
		}
		data.Rows = append(data.Rows, row)
	}
	return data, nil
}
