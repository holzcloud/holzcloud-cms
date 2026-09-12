package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// SnippetListData is the snippet overview.
type SnippetListData struct {
	web.LayoutData
	web.FormState
	WebsiteID int64
	Snippets  []SnippetRow
	// Values is the create/edit form, re-rendered with what was submitted when
	// a save is rejected.
	Values SnippetValues
	IsEdit bool

	// FieldViews sind die eigenen Felder dieses Textbausteins, als Modell des
	// Formulars. Gebaut wie die der Seite, von derselben Funktion, aus
	// demselben Grund: welche Eingabe eine Feldart braucht, ist eine
	// Entscheidung mit acht Zweigen, und acht Zweige in einer Vorlage sind der
	// Ort, an dem ein fehlendes name-Attribut sich versteckt.
	FieldViews []FieldBlock
	// Media ist der Bildvorrat eines Bildfeldes.
	Media []media.Media
	// RefPages ist die Auswahl eines Verweisfeldes: die Seiten dieser Website.
	RefPages []PageChoice
	// RefTerms ist die Auswahl eines Bezeichnungsfeldes: die Bezeichnungen
	// dieser Website.
	RefTerms []TermChoice
}

// pool gathers the choices this form already loaded, exactly as the page form
// does — one value rather than three parameters, so a fourth kind of chooser
// does not mean touching every call site again.
func (d SnippetListData) pool() pool {
	return pool{media: d.Media, pages: d.RefPages, terms: d.RefTerms}
}

// SnippetRow pairs a snippet with how often it is used.
type SnippetRow struct {
	snippet.Snippet
	// UsedOn is the number of live pages carrying the marker, which is what
	// makes it safe to know whether deleting one matters.
	UsedOn int
}

// SnippetValues is exactly what the form submitted.
type SnippetValues struct {
	ID       int64
	Key      string
	Name     string
	Markdown string

	// Fields sind die Antworten auf die eigenen Felder dieses Textbausteins,
	// so getippt wie abgeschickt — dieselbe Rolle, die PageValues.Fields auf
	// einer Seite spielt, damit ein abgewiesenes Formular zurückgibt, was
	// dastand.
	Fields field.Data
}

func snippetValuesFromRequest(r *http.Request) SnippetValues {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	return SnippetValues{
		ID:       id,
		Key:      strings.TrimSpace(strings.ToLower(r.FormValue("key"))),
		Name:     strings.TrimSpace(r.FormValue("name")),
		Markdown: r.FormValue("content_markdown"),
		// Der Parser des Seiteneditors, gerufen und nicht abgeschrieben. Sein
		// Doc-Kommentar erklärt, warum ein leerer Wert und ein fehlender
		// Schlüssel dasselbe JSON ergeben; genau diese Eigenschaft macht
		// snippets.fields DEFAULT '' unbedenklich.
		//
		// Der Vorbehalt, den derselbe Kommentar erhebt, trifft diesen Handler
		// nicht: sein Speicherweg ist ein vollständiges Ersetzen, ein UPDATE
		// über die ganze Spalte durch SetFields. Ein Aufrufer, der je nur
		// einen Teil der Felder fortschreibt, müsste die Anwesenheit auf
		// seiner eigenen Ebene tragen — der nächste weiss das vielleicht
		// nicht.
		Fields: fieldsFromRequest(r),
	}
}

// validKey is the same shape as a slug: a marker has to be typeable inside page
// text without ambiguity.
func validKey(key string) bool {
	if key == "" || len(key) > 60 {
		return false
	}
	for i, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case (r == '-' || r == '_') && i > 0 && i < len(key)-1:
		default:
			return false
		}
	}
	return true
}

func (v SnippetValues) validate(errs web.FormErrors) {
	if v.Name == "" {
		errs.Add("name", "Please give a name.")
	}
	if !validKey(v.Key) {
		errs.Add("key", "Lower-case letters, digits, hyphen and underscore only.")
	}
}

// HandleSnippetList renders the snippets of a website together with the form.
func (h *Handler) HandleSnippetList(w http.ResponseWriter, r *http.Request) error {
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
		return h.handleSnippetSave(w, r, websiteID, ws.Name)
	}

	// Opening one for editing prefills the same form rather than showing a
	// second screen. Gelesen wird vor snippetListData und nicht danach, weil
	// die Feldeingaben aus genau diesen Werten gebaut werden: ein Formular, das
	// die Werte erst hinterher bekommt, zeigt leere Kästchen.
	values := SnippetValues{}
	isEdit := false
	if raw := r.URL.Query().Get("edit"); raw != "" {
		id, _ := strconv.ParseInt(raw, 10, 64)
		sn, err := h.snippets.Get(r.Context(), websiteID, id)
		if err != nil {
			return err
		}
		if sn != nil {
			values = SnippetValues{
				ID: sn.ID, Key: sn.Key, Name: sn.Name, Markdown: sn.ContentMarkdown,
				Fields: field.Decode(sn.Fields),
			}
			isEdit = true
		}
	}

	data, err := h.snippetListData(r, websiteID, ws.Name, values)
	if err != nil {
		return err
	}
	data.IsEdit = isEdit
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "snippet_list", data)
}

// snippetFieldDefs loads the own fields of one snippet, or none.
//
// Ein Textbaustein, den es noch nicht gibt, hat keine Felder und keine Nummer,
// nach der sich fragen liesse — der Bildschirm für einen neuen trägt darum kein
// Feld, und das ist keine Einschränkung, sondern die Reihenfolge der Sache.
//
// Wie bei fieldDefs ist ein Fehler hier keinen gescheiterten Aufruf wert: ohne
// die Definitionen werden die Felder schlicht nicht angeboten, und alles andere
// am Bildschirm bleibt bedienbar.
func (h *Handler) snippetFieldDefs(ctx context.Context, websiteID, snippetID int64) []field.Def {
	if h.fields == nil || snippetID == 0 {
		return nil
	}
	defs, err := h.fields.OfSnippet(ctx, websiteID, snippetID)
	if err != nil {
		return nil
	}
	return defs
}

func (h *Handler) snippetListData(r *http.Request, websiteID int64, websiteName string, values SnippetValues) (SnippetListData, error) {
	data := SnippetListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Snippets – %s", websiteName)),
		FormState:  web.NewFormState(),
		WebsiteID:  websiteID,
		Values:     values,
	}
	data.ActiveNav = "snippets"

	list, err := h.snippets.List(r.Context(), websiteID)
	if err != nil {
		return data, err
	}
	for _, sn := range list {
		used, err := h.snippets.CountUsage(r.Context(), websiteID, sn.Key)
		if err != nil {
			return data, err
		}
		data.Snippets = append(data.Snippets, SnippetRow{Snippet: sn, UsedOn: used})
	}

	// Dieselben drei Vorräte, die der Seiteneditor lädt, mit denselben
	// Wächtern: ein Bildfeld, ein Verweisfeld und ein Bezeichnungsfeld
	// brauchen eine Auswahl, und die Auswahl gehört dieser Website.
	if h.mediaStore != nil {
		if all, _, err := h.mediaStore.List(r.Context(), websiteID, media.Filter{MimePrefix: "image/"}, 1, 200); err == nil {
			data.Media = all
		}
	}
	data.RefPages = h.refPages(r.Context(), websiteID)
	data.RefTerms = h.siteTerms(r.Context(), websiteID)

	// Die Definitionen gehen unverengt hinein. Kein `field.For`: jener Filter
	// verengt nach „gilt für", was auf einer Seite etwas bedeutet und an einem
	// Textbaustein von validate ohnehin auf „beides" gestellt wird (Plan
	// 08-02). Gefiltert würde hier also nichts gewonnen und im Zweifel ein Feld
	// verloren.
	defs := h.snippetFieldDefs(r.Context(), websiteID, values.ID)
	data.FieldViews = fieldViews(defs, values.Fields, data.pool(), nil)
	return data, nil
}

func (h *Handler) handleSnippetSave(w http.ResponseWriter, r *http.Request, websiteID int64, websiteName string) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	values := snippetValuesFromRequest(r)

	// Eine Zeile einer Gruppe hinzuzufügen oder wegzunehmen ist kein Speichern:
	// der Knopf ist ein gewöhnliches Absenden, der Server baut das Formular neu
	// auf, und der ganze Bildschirm kommt ohne eine Zeile JavaScript aus.
	if groupAction(r, &values.Fields) {
		data, err := h.snippetListData(r, websiteID, websiteName, values)
		if err != nil {
			return err
		}
		data.IsEdit = values.ID != 0
		if ws, err := h.domains.GetWebsite(r.Context(), websiteID); err == nil {
			data.CurrentWebsite = ws
		}
		return web.RenderFormError(w, h.templates, r, "snippet_list", data)
	}

	data, err := h.snippetListData(r, websiteID, websiteName, values)
	if err != nil {
		return err
	}
	data.IsEdit = values.ID != 0
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	data.CurrentWebsite = ws

	values.validate(data.Errors)

	// Geprüft wird gegen die Definitionen, die der **Server** geladen hat, nie
	// gegen das, was das Formular behauptet: ein Wert, der keine Definition
	// benennt, wird von field.Clean verworfen und nie gespeichert. Nicht
	// checkFields, denn das legt `field.For` mit einer Seitenart darum, und
	// ein Textbaustein hat keine.
	defs := h.snippetFieldDefs(r.Context(), websiteID, values.ID)
	fieldErrs := reasonTexts(r.Context(), field.CheckAll(defs, values.Fields))
	if len(fieldErrs) > 0 {
		data.FieldViews = fieldViews(defs, values.Fields, data.pool(), fieldErrs)
		for _, reason := range fieldErrs {
			data.Errors.Add("felder", reason)
		}
	}
	// Kein Flash: ein Flash überlebt eine Umleitung und nicht den Inhalt des
	// Formulars. Das Wertformular eines Textbausteins ist lang, und ein
	// abgewiesenes Speichern muss zurückgeben, was dastand — mit dem Grund
	// neben dem Feld, das ihn ausgelöst hat.
	if data.Errors.Any() {
		return web.RenderFormError(w, h.templates, r, "snippet_list", data)
	}

	// Dieselbe Auswahl wie in CheckAll eine Zeile darüber: geprüft und
	// gesäubert wird gegen dieselben Definitionen, sonst bliebe ein Wert, den
	// niemand geprüft hat, ungeprüft liegen.
	storedFields, err := field.Encode(field.Clean(defs, values.Fields))
	if err != nil {
		return err
	}

	// The same pipeline page content goes through, so the stored HTML carries
	// the same guarantee and can be cast in a template.
	html, err := page.RenderMarkdown(values.Markdown)
	if err != nil {
		return err
	}

	id := values.ID
	if values.ID == 0 {
		var created *snippet.Snippet
		created, err = h.snippets.Create(r.Context(), websiteID, values.Key, values.Name, values.Markdown, html)
		if created != nil {
			id = created.ID
		}
	} else {
		existing, getErr := h.snippets.Get(r.Context(), websiteID, values.ID)
		if getErr != nil {
			return getErr
		}
		if existing == nil {
			http.NotFound(w, r)
			return nil
		}
		err = h.snippets.Update(r.Context(), websiteID, values.ID, values.Key, values.Name, values.Markdown, html)
	}
	if errors.Is(err, snippet.ErrKeyTaken) {
		data.Errors.Add("key", "That key is already used by another snippet.")
		return web.RenderFormError(w, h.templates, r, "snippet_list", data)
	}
	if err != nil {
		return err
	}

	// Erst jetzt, denn ein neuer Textbaustein hat seine Nummer bis hierher
	// nicht. Die Spalte wird ganz geschrieben und nicht verschmolzen — das ist
	// die Zusage, auf der der Parser oben ruht.
	if err := h.snippets.SetFields(r.Context(), websiteID, id, storedFields); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Snippet saved")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/snippets", websiteID))
}

// HandleSnippetDelete removes a snippet.
func (h *Handler) HandleSnippetDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	id, err := strconv.ParseInt(r.PathValue("snippetID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	sn, err := h.snippets.Get(r.Context(), websiteID, id)
	if err != nil {
		return err
	}
	if sn == nil {
		http.NotFound(w, r)
		return nil
	}

	if err := h.snippets.Delete(r.Context(), websiteID, id); err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Snippet deleted")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/snippets", websiteID))
}
