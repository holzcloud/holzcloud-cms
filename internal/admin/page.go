package admin

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	"github.com/holzcloud/holzcloud-cms/internal/term"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// PageListData extends LayoutData for the page list.
type PageListData struct {
	web.LayoutData
	WebsiteID int64
	Rows      []PageRowData
	Pagination
	// Filter and Query are echoed back so the controls keep their state and the
	// pager keeps the filter.
	Filter page.ListFilter
	Query  string
	// ResultCount is the total for a listing and the hit count for a search.
	ResultCount int
	// PendingReview drives the "waiting for review" filter chip.
	PendingReview int
	// Languages drives the language filter. Empty on a website with one
	// language, which hides the control entirely.
	Languages []LanguageChoice
	// MayPublish is false for somebody who writes and submits. Then the bulk
	// menu offers no publishing.
	MayPublish bool
	// Types are the website's own content kinds, for the "Art" filter.
	Types []kind.Type
	// Columns are the chosen columns, Choices the chooser above the table.
	Columns ColumnSet
	Choices []Column
	// Views are this person's remembered filter combinations, and Filterpart
	// is what a new one would remember.
	Views      []SavedView
	Filterpart string
}

// PageFormData extends LayoutData for the page create/edit form.
//
// Values, not Page, is what the template renders. A rejected submit re-renders
// this struct with the values that were posted, so a long article survives a
// missing title instead of being replaced by an empty form and a flash message.
type PageFormData struct {
	web.LayoutData
	web.FormState
	WebsiteID int64
	PageID    int64
	Values    PageValues
	IsEdit    bool

	// PublishedAt is display-only metadata that has no form field.
	PublishedAt *time.Time
	// RevisionCount drives the link to the history, which is hidden when a page
	// has never been edited.
	RevisionCount int
	// PreviewHTML is the initial content of the live preview pane, so the pane
	// is not empty before the first keystroke and stays useful without htmx.
	PreviewHTML template.HTML
	// Media is the image pool the preview image is chosen from.
	Media []media.Media
	// RefPages is the choice a reference field offers: the pages of this
	// website.
	RefPages []PageChoice
	// Videos is the film pool of the video block.
	Videos []media.Media
	// KindChoices is the "Art" dropdown: the two built-in kinds and the
	// website's own.
	KindChoices []KindChoice
	// Snippets lists the reusable blocks, so an editor can see which markers
	// exist without leaving the page.
	Snippets []snippet.Snippet
	// ReviewPending flips the review button between submit and withdraw.
	ReviewPending bool

	// BlockViews is the block editor's markup model, built in Go so a renamed
	// field is a compile error rather than a surprise in a browser.
	BlockViews []BlockView
	// BlockAction is the address the editor's buttons post to. It differs
	// between creating and editing, and the template should not have to know.
	BlockAction string

	// FieldViews are the website's own fields, already narrowed to the ones
	// that apply to this kind of page and divided into blocks by their
	// headings.
	FieldViews []FieldBlock

	// Languages is the language dropdown, empty on a website with one
	// language — which is every website until somebody adds a second.
	Languages []LanguageChoice
	// Translations is the page's translation group, missing languages included.
	Translations []TranslationView
	// MayPublish is false for somebody who writes and submits. Then the status
	// chooser is a plain label and the form says who puts it online.
	MayPublish bool
}

// PageRowData holds data for rendering a single page row partial.
// It is deliberately flat (not embedding LayoutData) so the same value can be
// used both inside the page list and as a standalone htmx fragment.
type PageRowData struct {
	WebsiteID int64
	Page      page.Page
	CSRFToken string
	// Snippet is set on search results: the matching passage with the terms
	// marked. Empty in the ordinary listing.
	Snippet template.HTML
	// Language is the page's language, and empty on a website that has only
	// one — a column that says "Deutsch" on every row of a German site is a
	// column that costs space and says nothing.
	Language string
	// MayPublish is false for somebody who writes and submits. Then the row
	// menu leaves the publish entry out instead of offering a button that
	// answers with a refusal.
	MayPublish bool
	// KindName is the name of an own kind, for the "Art" column. Empty for the
	// built-in two, which the row names itself.
	KindName string
	// Columns are the cells this row draws. Carried on every row rather than
	// read from the page around it, because a row is also an htmx swap target
	// and has to draw the same cells on its own.
	Columns ColumnSet
}

// HandlePageList renders the paginated page list for a website.
func (h *Handler) HandlePageList(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
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

	const perPage = 20
	pageNum := pageParam(r)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	// Every language unless one is asked for. Defaulting to the main language
	// would hide every translation from the list that is supposed to be the
	// place you find your pages.
	loc := "*"
	if r.URL.Query().Has("sprache") {
		loc = r.URL.Query().Get("sprache")
		if loc != "*" {
			loc = locale.Pick(loc, ws.Locales())
		}
	}
	// Die Art aus der Adresse kann eine eingebaute oder eine eigene sein.
	types := h.kindsOf(r, websiteID)
	wantKind := r.URL.Query().Get("kind")
	filter := page.ListFilter{
		Status:  r.URL.Query().Get("status"),
		Review:  r.URL.Query().Get("review"),
		Sort:    r.URL.Query().Get("sort"),
		Locale:  loc,
		Page:    pageNum,
		PerPage: perPage,
	}
	if _, own := kind.Find(types, wantKind); own {
		filter.TypeKey = wantKind
	} else {
		filter.Kind = wantKind
	}

	csrfToken := web.CSRFTokenFromRequest(r)
	data := PageListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Seiten – %s", ws.Name)),
		WebsiteID:  websiteID,
		Filter:     filter,
		Query:      query,
		Languages:  languageChoices(ws, loc),
	}

	darfVeroeffentlichen := h.mayPublish(r)
	data.MayPublish = darfVeroeffentlichen
	data.Types = types

	// Die Spalten dieser Person und ihre gemerkten Ansichten. Beides gehört zu
	// ihr und nicht zur Website — siehe listview.go.
	mehrsprachig := len(ws.Locales()) > 0
	data.Columns = h.pageColumns(r, mehrsprachig)
	data.Choices = columnChoices(data.Columns, mehrsprachig)
	data.Filterpart = filterQuery(r.URL.Query())
	data.Views = h.savedViews(r.Context(), h.sm.GetInt64(r.Context(), auth.SessionKeyUserID),
		websiteID, data.Filterpart)
	if query != "" {
		// A search replaces the listing rather than filtering it: ranking by
		// relevance and paginating by date at the same time would be a lie
		// about the order.
		results, err := h.pages.SearchPages(r.Context(), websiteID, query, true, 50)
		if err != nil {
			return err
		}
		for _, res := range results {
			data.Rows = append(data.Rows, PageRowData{
				MayPublish: darfVeroeffentlichen,
				Columns:    data.Columns,
				KindName:   ownKindName(types, res.Page),
				WebsiteID:  websiteID, Page: res.Page, CSRFToken: csrfToken, Snippet: res.Snippet,
				Language: rowLanguage(ws, res.Page.Locale),
			})
		}
		data.ResultCount = len(results)
	} else {
		pages, total, err := h.pages.ListPages(r.Context(), websiteID, filter)
		if err != nil {
			return err
		}
		for _, p := range pages {
			data.Rows = append(data.Rows, PageRowData{
				MayPublish: darfVeroeffentlichen,
				Columns:    data.Columns,
				KindName:   ownKindName(types, p),
				WebsiteID:  websiteID, Page: p, CSRFToken: csrfToken,
				Language: rowLanguage(ws, p.Locale),
			})
		}
		data.ResultCount = total
		data.Pagination = NewPagination(pageNum, perPage, total).
			WithTarget(fmt.Sprintf("/admin/websites/%d/pages", websiteID), "#page-list")
	}

	pending, err := h.pages.CountPendingReview(r.Context(), websiteID)
	if err != nil {
		return err
	}
	data.PendingReview = pending

	data.ActiveNav = "pages"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "page_list", data)
}

// HandlePageCreate handles GET (form) and POST (submit) for creating a page.
func (h *Handler) HandlePageCreate(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
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
		return h.handlePageCreatePost(w, r, websiteID, ws.Name)
	}

	// Aus der Übersichtsseite kommt man mit Sprache und Vorlage in der
	// Adresse: dort steht die Lücke, die man gerade schliessen will, und das
	// Formular soll sie nicht noch einmal abfragen.
	values := PageValues{Status: "draft"}
	if q := r.URL.Query(); q.Has("sprache") || q.Has("uebersetzung_von") {
		values.Locale = locale.Pick(q.Get("sprache"), ws.Locales())
		values.TranslationOf = strings.TrimSpace(q.Get("uebersetzung_von"))
		if src, err := h.sourcePageFor(r, websiteID, values.TranslationOf); err == nil && src != nil {
			values.Title = src.Title
			values.Slug = src.Slug
		}
	}

	data := h.newPageFormData(r, websiteID, web.Titlef(r, "Neue Seite – %s", ws.Name), values)
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "page_form", data)
}

// sourcePageFor is the page a new translation is made from, or nil.
//
// Title and slug are carried over as a starting point: a translator overwrites
// them, but an empty form makes them go and look the original up, and the
// address is the one thing that ought to stay recognisable across languages.
func (h *Handler) sourcePageFor(r *http.Request, websiteID int64, raw string) (*page.Page, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id == 0 {
		return nil, nil
	}
	p, err := h.pages.GetPage(r.Context(), id)
	if err != nil || p == nil || p.WebsiteID != websiteID {
		return nil, err
	}
	return p, nil
}

// newPageFormData assembles the shared parts of the create and edit form.
func (h *Handler) newPageFormData(r *http.Request, websiteID int64, title string, values PageValues) PageFormData {
	data := PageFormData{
		LayoutData:  web.NewLayoutData(r, h.sm, title),
		FormState:   web.NewFormState(),
		WebsiteID:   websiteID,
		Values:      values,
		PreviewHTML: renderPreview(values.Markdown),
	}
	data.ActiveNav = "pages"
	if h.snippets != nil {
		if list, err := h.snippets.List(r.Context(), websiteID); err == nil {
			data.Snippets = list
		}
	}
	if h.mediaStore != nil {
		if all, _, err := h.mediaStore.List(r.Context(), websiteID, media.Filter{MimePrefix: "image/"}, 1, 200); err == nil {
			data.Media = all
		}
	}
	if h.mediaStore != nil {
		if films, _, err := h.mediaStore.List(r.Context(), websiteID, media.Filter{MimePrefix: "video/"}, 1, 100); err == nil {
			data.Videos = films
		}
	}
	data.RefPages = h.refPages(r.Context(), websiteID)
	data.MayPublish = h.mayPublish(r)
	data.KindChoices = kindChoices(h.kindsOf(r, websiteID), values.KindValue())
	// The language dropdown belongs to every rendering of the form, including
	// the one after a rejected save — a form that comes back having silently
	// dropped the language field would file the page under the main language on
	// the second attempt.
	if ws, err := h.domains.GetWebsite(r.Context(), websiteID); err == nil && ws != nil {
		data.Languages = languageChoices(ws, values.Locale)
	}
	data.BlockViews = blockViews(values.BlockSet, values.Blocks, data.Media, data.Videos, websiteID)
	data.BlockAction = fmt.Sprintf("/admin/websites/%d/pages/new", websiteID)
	defs := h.fieldDefs(r.Context(), websiteID)
	data.FieldViews = fieldViews(field.For(defs, values.KindValue()), values.Fields, data.pool(), nil)
	return data
}

// renderPreview turns Markdown into the HTML shown in the preview pane.
//
// The cast is only safe because RenderMarkdown ends in bluemonday; a render
// failure yields no HTML at all rather than the unsanitised source.
func renderPreview(markdown string) template.HTML {
	if strings.TrimSpace(markdown) == "" {
		return ""
	}
	out, err := page.RenderMarkdown(markdown)
	if err != nil {
		return ""
	}
	return template.HTML(out)
}

// HandlePagePreview renders Markdown for the live preview pane.
//
// It stores nothing: the pane is a rendering of what is currently in the
// textarea, which may never be saved.
func (h *Handler) HandlePagePreview(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
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
	if err := r.ParseForm(); err != nil {
		return err
	}
	return web.RenderPartial(w, h.templates, r, "preview_pane", renderPreview(r.FormValue("content_markdown")))
}

func (h *Handler) handlePageCreatePost(w http.ResponseWriter, r *http.Request, websiteID int64, websiteName string) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	values := pageValuesFromRequest(r)
	values.setKind(h.kindsOf(r, websiteID))
	values.BlockSet = h.blockSet(r.Context(), websiteID)
	if !h.mayPublish(r) {
		// Neu und ohne Veröffentlichungsrecht: ein Entwurf, den jemand ansieht.
		values.Status = "draft"
	}
	// A structural change in the block editor is not a save: apply it and draw
	// the form again, with everything the editor has typed so far still in it.
	if blockAction(r, &values) {
		data := h.newPageFormData(r, websiteID, web.Titlef(r, "Neue Seite – %s", websiteName), values)
		if ws, err := h.domains.GetWebsite(r.Context(), websiteID); err == nil {
			data.CurrentWebsite = ws
		}
		return h.renderBlockList(w, r, data)
	}
	// Adding or removing a row of a group is not a save either.
	if groupAction(r, &values.Fields) {
		data := h.newPageFormData(r, websiteID, web.Titlef(r, "Neue Seite – %s", websiteName), values)
		if ws, err := h.domains.GetWebsite(r.Context(), websiteID); err == nil {
			data.CurrentWebsite = ws
		}
		return web.RenderAdmin(w, h.templates, r, "page_form", data)
	}

	data := h.newPageFormData(r, websiteID, web.Titlef(r, "Neue Seite – %s", websiteName), values)
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	data.CurrentWebsite = ws

	slug := values.validateOn(r, data.Errors, ws.BlogBase, ws.Locales())
	schedule := values.schedule(data.Errors)
	defs := h.fieldDefs(r.Context(), websiteID)
	fieldErrs := checkFields(defs, values.KindValue(), values.Fields)
	if len(fieldErrs) > 0 {
		data.FieldViews = fieldViews(field.For(defs, values.KindValue()), values.Fields, data.pool(), fieldErrs)
		for _, reason := range fieldErrs {
			data.Errors.Add("felder", reason)
		}
	}
	if data.Errors.Any() {
		return web.RenderFormError(w, h.templates, r, "page_form", data)
	}
	storedFields, err := field.Encode(field.Clean(defs, values.Fields))
	if err != nil {
		return err
	}

	markdown, html, blocks, err := h.blockContent(r.Context(), websiteID, values)
	if err != nil {
		return err
	}

	created, err := h.pages.CreatePage(r.Context(), page.PageCreate{
		WebsiteID: websiteID,
		Title:     values.Title,
		Slug:      slug,
		Markdown:  markdown,
		HTML:      html,
		Blocks:    blocks,
		Fields:    storedFields,
		Status:    values.Status,
		Meta:      values.meta(),
		Schedule:  schedule,
		Kind:      values.Kind,
		TypeKey:   values.TypeKey,
		UserID:    h.currentUserID(r),
	})
	if err != nil {
		return err
	}
	h.setLanguage(r, ws, created.ID, values)
	h.recordMediaUsage(r, websiteID, created.ID, html)
	h.recordTerms(r, websiteID, created.ID, values.Tags)
	if msg := h.recordAccess(r, created.ID, values); msg != "" {
		web.SetFlashError(h.sm, r.Context(), msg)
	}

	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionPageCreate,
		EntityType: "page",
		EntityID:   created.ID,
		WebsiteID:  &websiteID,
		Metadata:   map[string]any{"slug": created.Slug, "titel": created.Title},
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Seite erstellt")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages", websiteID))
}

// HandlePageEdit handles GET (form) and POST (submit) for editing a page.
func (h *Handler) HandlePageEdit(w http.ResponseWriter, r *http.Request) error {
	websiteID, pageID, ok := pageIDs(r)
	if !ok {
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

	p, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if p == nil || p.WebsiteID != websiteID || p.InTrash() {
		http.NotFound(w, r)
		return nil
	}

	if r.Method == http.MethodPost {
		return h.handlePageEditPost(w, r, ws.Name, p)
	}

	values := pageValuesFromPage(p, h.blockSet(r.Context(), websiteID))
	values.Tags = h.tagsFor(r, p.ID)
	data, err := h.editFormData(r, ws.Name, p, values)
	if err != nil {
		return err
	}
	data.CurrentWebsite = ws
	data.Translations = h.translationViews(r, ws, p)
	return web.RenderAdmin(w, h.templates, r, "page_form", data)
}

// editFormData builds the edit form for a stored page with the given values.
func (h *Handler) editFormData(r *http.Request, websiteName string, p *page.Page, values PageValues) (PageFormData, error) {
	data := h.newPageFormData(r, p.WebsiteID, web.Titlef(r, "Seite bearbeiten – %s", websiteName), values)
	data.PageID = p.ID
	data.IsEdit = true
	data.BlockAction = fmt.Sprintf("/admin/websites/%d/pages/%d/edit", p.WebsiteID, p.ID)
	data.PublishedAt = p.PublishedAt
	data.ReviewPending = p.ReviewState == "pending"

	revisions, err := h.pages.ListRevisions(r.Context(), p.ID)
	if err != nil {
		return data, err
	}
	data.RevisionCount = len(revisions)
	return data, nil
}

func (h *Handler) handlePageEditPost(w http.ResponseWriter, r *http.Request, websiteName string, existing *page.Page) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	values := pageValuesFromRequest(r)
	values.setKind(h.kindsOf(r, existing.WebsiteID))
	// Wer nicht veröffentlichen darf, ändert den Zustand nicht — weder hinauf
	// noch hinunter. Ein stilles Zurückstufen wäre schlimmer als ein
	// abgelehnter Knopf: eine laufende Seite verschwände beim Korrigieren
	// eines Tippfehlers.
	if !h.mayPublish(r) {
		values.Status = existing.Status
	}

	// Re-rendering the form needs the website for the sidebar, and the stored
	// page for the metadata that has no form field.
	ws, err := h.domains.GetWebsite(r.Context(), existing.WebsiteID)
	if err != nil {
		return err
	}

	rerender := func(data PageFormData) error {
		data.CurrentWebsite = ws
		return web.RenderFormError(w, h.templates, r, "page_form", data)
	}

	data, err := h.editFormData(r, websiteName, existing, values)
	if err != nil {
		return err
	}
	// A structural change in the block editor is not a save.
	if blockAction(r, &values) {
		data.Values = values
		data.BlockViews = blockViews(values.BlockSet, values.Blocks, data.Media, data.Videos, existing.WebsiteID)
		data.CurrentWebsite = ws
		return h.renderBlockList(w, r, data)
	}
	if groupAction(r, &values.Fields) {
		data.Values = values
		data.FieldViews = fieldViews(field.For(h.fieldDefs(r.Context(), existing.WebsiteID), values.KindValue()),
			values.Fields, data.pool(), nil)
		data.CurrentWebsite = ws
		return web.RenderAdmin(w, h.templates, r, "page_form", data)
	}

	slug := values.validateOn(r, data.Errors, ws.BlogBase, ws.Locales())
	schedule := values.schedule(data.Errors)
	defs := h.fieldDefs(r.Context(), existing.WebsiteID)
	fieldErrs := checkFields(defs, values.KindValue(), values.Fields)
	if len(fieldErrs) > 0 {
		data.FieldViews = fieldViews(field.For(defs, values.KindValue()), values.Fields, data.pool(), fieldErrs)
		for _, reason := range fieldErrs {
			data.Errors.Add("felder", reason)
		}
	}
	if data.Errors.Any() {
		return rerender(data)
	}
	storedFields, err := field.Encode(field.Clean(defs, values.Fields))
	if err != nil {
		return err
	}

	markdown, html, blocks, err := h.blockContent(r.Context(), existing.WebsiteID, values)
	if err != nil {
		return err
	}

	err = h.pages.UpdatePage(r.Context(), existing.ID, page.PageUpdate{
		Title:           values.Title,
		Slug:            slug,
		Markdown:        markdown,
		HTML:            html,
		Blocks:          blocks,
		Fields:          storedFields,
		Status:          values.Status,
		Meta:            values.meta(),
		Schedule:        schedule,
		Kind:            values.Kind,
		TypeKey:         values.TypeKey,
		ExpectedVersion: values.Version,
		UserID:          h.currentUserID(r),
	})
	switch {
	case errors.Is(err, page.ErrConflict):
		// The typed Markdown is handed straight back — it is the only copy that
		// exists. The stored version is one click away rather than in its place.
		data.Conflict = "Diese Seite wurde inzwischen von jemand anderem gespeichert. " +
			"Dein Text steht unverändert unten – vergleiche ihn mit der aktuellen Fassung, bevor du erneut speicherst."
		return rerender(data)
	case errors.Is(err, page.ErrSlugTaken):
		data.Errors.Add("slug", "Diese Adresse wird bereits von einer anderen Seite benutzt.")
		return rerender(data)
	case errors.Is(err, page.ErrNotFound):
		http.NotFound(w, r)
		return nil
	case err != nil:
		return err
	}

	h.setLanguage(r, ws, existing.ID, values)
	h.recordMediaUsage(r, existing.WebsiteID, existing.ID, html)
	h.recordTerms(r, existing.WebsiteID, existing.ID, values.Tags)
	if msg := h.recordAccess(r, existing.ID, values); msg != "" {
		web.SetFlashError(h.sm, r.Context(), msg)
	}

	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionPageUpdate,
		EntityType: "page",
		EntityID:   existing.ID,
		WebsiteID:  &existing.WebsiteID,
		Metadata:   map[string]any{"slug": existing.Slug, "titel": values.Title},
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Seite gespeichert")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages/%d/edit", existing.WebsiteID, existing.ID))
}

// HandlePageDelete moves a page to the trash.
func (h *Handler) HandlePageDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, pageID, ok := pageIDs(r)
	if !ok {
		http.NotFound(w, r)
		return nil
	}

	// Verify page belongs to website (T-03-09)
	existing, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if existing == nil || existing.WebsiteID != websiteID {
		http.NotFound(w, r)
		return nil
	}

	if err := h.pages.TrashPage(r.Context(), pageID); err != nil && !errors.Is(err, page.ErrNotFound) {
		return err
	}

	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionPageDelete,
		EntityType: "page",
		EntityID:   pageID,
		WebsiteID:  &websiteID,
		Metadata:   map[string]any{"slug": existing.Slug, "titel": existing.Title},
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Seite in den Papierkorb verschoben")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages", websiteID))
}

// HandlePageStatusToggle toggles a page between draft and published.
func (h *Handler) HandlePageStatusToggle(w http.ResponseWriter, r *http.Request) error {
	websiteID, pageID, ok := pageIDs(r)
	if !ok {
		http.NotFound(w, r)
		return nil
	}

	p, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if p == nil || p.WebsiteID != websiteID || p.InTrash() {
		http.NotFound(w, r)
		return nil
	}

	if !h.mayPublish(r) {
		web.SetFlashError(h.sm, r.Context(),
			"Veröffentlichen gehört nicht zu deinem Zugang. Reiche die Seite zur Prüfung ein.")
		return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages", websiteID))
	}

	newStatus := "published"
	if p.Status == "published" {
		newStatus = "draft"
	}

	if err := h.pages.SetPageStatus(r.Context(), pageID, newStatus, h.currentUserID(r)); err != nil {
		if errors.Is(err, page.ErrNotFound) {
			http.NotFound(w, r)
			return nil
		}
		return err
	}

	// Re-read rather than patching the struct: published_at and version are set
	// by the statement, so a locally edited copy would render stale metadata.
	updated, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if updated == nil {
		http.NotFound(w, r)
		return nil
	}

	action := activity.ActionPageUnpublish
	if newStatus == "published" {
		action = activity.ActionPagePublish
	}
	h.LogActivity(r, activity.Entry{
		Action:     action,
		EntityType: "page",
		EntityID:   pageID,
		WebsiteID:  &websiteID,
		Metadata:   map[string]any{"slug": updated.Slug, "titel": updated.Title},
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Status geändert")

	// htmx: return updated row for in-place swap
	if r.Header.Get("HX-Request") == "true" {
		return h.renderPageRow(w, r, websiteID, *updated)
	}

	// Non-htmx: redirect back to page list
	http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages", websiteID), http.StatusSeeOther)
	return nil
}

// HandlePageInlineEditTitle returns the inline edit form for a page title.
func (h *Handler) HandlePageInlineEditTitle(w http.ResponseWriter, r *http.Request) error {
	websiteID, pageID, ok := pageIDs(r)
	if !ok {
		http.NotFound(w, r)
		return nil
	}

	p, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if p == nil || p.WebsiteID != websiteID || p.InTrash() {
		http.NotFound(w, r)
		return nil
	}

	data := PageRowData{
		MayPublish: h.mayPublish(r),
		WebsiteID:  websiteID,
		Page:       *p,
		CSRFToken:  web.CSRFTokenFromRequest(r),
	}

	return web.RenderPartial(w, h.templates, r, "page_inline_edit", data)
}

// HandlePageInlineEditSave saves the inline edit for a page title.
func (h *Handler) HandlePageInlineEditSave(w http.ResponseWriter, r *http.Request) error {
	websiteID, pageID, ok := pageIDs(r)
	if !ok {
		http.NotFound(w, r)
		return nil
	}

	// Verify page belongs to website (T-03-09)
	p, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if p == nil || p.WebsiteID != websiteID || p.InTrash() {
		http.NotFound(w, r)
		return nil
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = p.Title
	}

	slug := strings.TrimSpace(r.FormValue("slug"))
	if slug == "" {
		slug = page.Slugify(title)
	} else {
		slug = page.Slugify(slug)
	}
	if err := page.ValidateSlug(slug); err != nil {
		// Inline edit has nowhere to show a flash; keep the previous address.
		slug = p.Slug
	}

	switch err := h.pages.UpdatePageTitle(r.Context(), pageID, title, slug, h.currentUserID(r)); {
	case errors.Is(err, page.ErrSlugTaken):
		// Same reasoning as an invalid slug: the row has no place for a message,
		// so the rename is dropped rather than the edit being lost.
		title, slug = p.Title, p.Slug
	case errors.Is(err, page.ErrNotFound):
		http.NotFound(w, r)
		return nil
	case err != nil:
		return err
	}

	updated, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if updated == nil {
		http.NotFound(w, r)
		return nil
	}

	return h.renderPageRow(w, r, websiteID, *updated)
}

// tagsFor renders a page's stored labels back into the editor's field.
func (h *Handler) tagsFor(r *http.Request, pageID int64) string {
	if h.terms == nil {
		return ""
	}
	terms, err := h.terms.ForPage(r.Context(), pageID)
	if err != nil {
		slog.Error("load page terms", "err", err, "page", pageID)
		return ""
	}
	return term.Format(terms)
}

// recordTerms stores the labels typed into the form.
//
// A failure is logged rather than returned: the text is already saved, and
// refusing the whole save over a label would cost the editor their work.
func (h *Handler) recordTerms(r *http.Request, websiteID, pageID int64, raw string) {
	if h.terms == nil {
		return
	}
	if err := h.terms.SetForPage(r.Context(), websiteID, pageID, term.Parse(raw)); err != nil {
		slog.Error("store page terms", "err", err, "page", pageID)
	}
}

// recordAccess stores the page's protection and reports a problem in words the
// editor can act on.
//
// A failure is a flash rather than a rejected save: the text is already stored,
// and losing an article because a password was four characters long would be a
// far worse outcome than a page that is briefly still public.
func (h *Handler) recordAccess(r *http.Request, pageID int64, values PageValues) string {
	err := h.pages.SetAccess(r.Context(), pageID, values.access(), h.argon2Params)
	switch {
	case errors.Is(err, page.ErrPagePasswordTooShort):
		return fmt.Sprintf("Die Seite wurde gespeichert, aber nicht geschützt: "+
			"ein Seitenpasswort braucht mindestens %d Zeichen.", page.MinPagePasswordLength)
	case errors.Is(err, page.ErrNoPagePassword):
		return "Die Seite wurde gespeichert, aber nicht geschützt: bitte noch ein Passwort vergeben."
	case err != nil:
		slog.Error("set page access", "err", err, "page", pageID)
		return "Der Zugriffsschutz konnte nicht gespeichert werden."
	}
	return ""
}

// recordMediaUsage notes which files this page shows, so deleting one can warn
// instead of silently breaking the page.
//
// A failure here is logged rather than returned: the page is already saved, and
// refusing the request would tell the editor their work was lost when it was
// not. The bookkeeping is rebuilt on the next save.
func (h *Handler) recordMediaUsage(r *http.Request, websiteID, pageID int64, html string) {
	if h.mediaStore == nil {
		return
	}
	ids, err := media.ExtractRefs(r.Context(), h.mediaStore, websiteID, html)
	if err != nil {
		slog.Error("extract media references", "err", err, "page", pageID)
		return
	}
	if err := h.mediaStore.ReplaceUsage(r.Context(), pageID, ids); err != nil {
		slog.Error("record media usage", "err", err, "page", pageID)
	}
}

// renderPageRow writes a single page table row as HTML.
//
// It looks the website up for the language marker: a row swapped in by htmx has
// to come back looking exactly like the one it replaced, and a badge that
// disappears the moment somebody toggles the status is worse than no badge.
func (h *Handler) renderPageRow(w http.ResponseWriter, r *http.Request, websiteID int64, p page.Page) error {
	row := PageRowData{
		MayPublish: h.mayPublish(r),
		WebsiteID:  websiteID,
		Page:       p,
		CSRFToken:  web.CSRFTokenFromRequest(r),
	}
	// Dieselben Spalten wie in der Liste, aus der die Zeile kommt: eine Zeile,
	// die htmx nachlädt, muss aussehen wie die, die sie ersetzt — sonst hat die
	// Tabelle nach einem Klick eine Zelle mehr als ihr Kopf.
	mehrsprachig := false
	if ws, err := h.domains.GetWebsite(r.Context(), websiteID); err == nil {
		row.Language = rowLanguage(ws, p.Locale)
		mehrsprachig = len(ws.Locales()) > 0
	}
	row.Columns = h.pageColumns(r, mehrsprachig)
	row.KindName = ownKindName(h.kindsOf(r, websiteID), p)
	return web.RenderPartial(w, h.templates, r, "page_row", row)
}

// pageIDs parses the website and page identifiers out of the route.
func pageIDs(r *http.Request) (websiteID, pageID int64, ok bool) {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, 0, false
	}
	pageID, err = strconv.ParseInt(r.PathValue("pageID"), 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return websiteID, pageID, true
}

// redirect navigates after a successful mutation, using HX-Redirect for htmx
// requests because a 303 would be swapped into the page instead of followed.
func (h *Handler) redirect(w http.ResponseWriter, r *http.Request, to string) error {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", to)
		return nil
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
	return nil
}
