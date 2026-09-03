package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// Der Bildschirm, auf dem eine Website ihre eigenen Inhaltsarten bekommt.
//
// Wie bei den Feldern: eine Liste und darunter dasselbe Formular, das anlegt
// und ändert. Eine Inhaltsart hat vier Angaben — für vier Angaben auf einen
// zweiten Bildschirm zu schicken ist mehr Navigation, als die Sache wert ist.

// KindRow is one kind with the number of entries that carry it.
type KindRow struct {
	kind.Type
	Entries int
	First   bool
	Last    bool
}

// KindListData is the "Inhaltsarten" screen.
type KindListData struct {
	web.LayoutData
	WebsiteID int64
	Rows      []KindRow
	// Edit is the kind being changed, or nil when the form is for a new one.
	Edit *kind.Type
	// Pages and Posts are how many entries the two built-in kinds hold, so the
	// screen shows the whole picture and not only the new part.
	Pages int
	Posts int
	// BlogBase is the address of the posts' archive, so somebody choosing an
	// address for their own overview can see what is taken.
	BlogBase string
}

// HandleKindList shows a website's own content kinds.
func (h *Handler) HandleKindList(w http.ResponseWriter, r *http.Request) error {
	websiteID, ws, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}

	data, err := h.kindListData(r, websiteID, ws.Name)
	if err != nil {
		return err
	}
	data.BlogBase = ws.BlogBase

	// ?aendern=<id> opens the same form filled in, with the list still visible.
	if id, cerr := strconv.ParseInt(r.URL.Query().Get("aendern"), 10, 64); cerr == nil {
		if t, gerr := h.kinds.Get(r.Context(), websiteID, id); gerr == nil {
			data.Edit = &t
		}
	}
	return web.RenderAdmin(w, h.templates, r, "kind_list", data)
}

func (h *Handler) kindListData(r *http.Request, websiteID int64, websiteName string) (KindListData, error) {
	types, err := h.kinds.List(r.Context(), websiteID)
	if err != nil {
		return KindListData{}, err
	}

	data := KindListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Inhaltsarten – %s", websiteName)),
		WebsiteID:  websiteID,
	}
	data.ActiveNav = "kinds"
	for i, t := range types {
		n, err := h.kinds.Count(r.Context(), websiteID, t.Key)
		if err != nil {
			return KindListData{}, err
		}
		data.Rows = append(data.Rows, KindRow{
			Type: t, Entries: n, First: i == 0, Last: i == len(types)-1,
		})
	}
	if data.Pages, err = h.kinds.Count(r.Context(), websiteID, kind.Page); err != nil {
		return KindListData{}, err
	}
	if data.Posts, err = h.kinds.Count(r.Context(), websiteID, kind.Post); err != nil {
		return KindListData{}, err
	}
	return data, nil
}

// HandleKindSave creates or changes a kind.
func (h *Handler) HandleKindSave(w http.ResponseWriter, r *http.Request) error {
	websiteID, ws, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	back := "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/inhaltsarten"

	t := kind.Type{
		WebsiteID: websiteID,
		Name:      r.FormValue("name"),
		Plural:    r.FormValue("mehrzahl"),
		Archive:   page.Slugify(r.FormValue("archiv")),
		Sort:      r.FormValue("sortierung"),
	}

	// Die Adresse der Übersicht ist reserviert wie die des Archivs: eine Seite
	// mit derselben Adresse käme nie zum Zug, weil die Übersicht zuerst geprüft
	// wird. Lieber hier ablehnen als dort still gewinnen.
	if t.Archive != "" {
		if t.Archive == ws.BlogBase {
			web.SetFlashError(h.sm, r.Context(), "Diese Adresse gehört schon zum Archiv der Beiträge")
			return h.redirect(w, r, back)
		}
		if existing, err := h.pages.GetPageBySlug(r.Context(), websiteID, t.Archive); err == nil && existing != nil {
			web.SetFlashError(h.sm, r.Context(),
				"Unter dieser Adresse liegt schon eine Seite. Wähle eine andere, sonst wäre eine von beiden unerreichbar.")
			return h.redirect(w, r, back)
		}
	}

	if id, cerr := strconv.ParseInt(r.FormValue("id"), 10, 64); cerr == nil && id > 0 {
		t.ID = id
		if err := h.kinds.Update(r.Context(), t); err != nil {
			return h.kindFailed(w, r, back, err)
		}
		web.SetFlashSuccess(h.sm, r.Context(), "Inhaltsart gesichert")
		return h.redirect(w, r, back)
	}

	created, err := h.kinds.Create(r.Context(), t)
	if err != nil {
		return h.kindFailed(w, r, back, err)
	}
	web.SetFlashSuccess(h.sm, r.Context(), web.Titlef(r,
		"Inhaltsart „%s“ angelegt. Sie steht ab sofort im Editor zur Wahl.", created.Name))
	return h.redirect(w, r, back)
}

// kindFailed turns a store error into a sentence for the person at the form.
func (h *Handler) kindFailed(w http.ResponseWriter, r *http.Request, back string, err error) error {
	switch {
	case errors.Is(err, kind.ErrDuplicate):
		web.SetFlashError(h.sm, r.Context(), "Diese Inhaltsart gibt es schon")
	case errors.Is(err, kind.ErrTooMany):
		web.SetFlashError(h.sm, r.Context(), "Mehr Inhaltsarten gehen nicht")
	case errors.Is(err, kind.ErrArchiveTaken):
		web.SetFlashError(h.sm, r.Context(), "Diese Adresse gehört schon zu einer anderen Übersicht")
	case errors.Is(err, kind.ErrNotFound):
		web.SetFlashError(h.sm, r.Context(), "Diese Inhaltsart gibt es nicht")
	default:
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Inhaltsart abgelehnt: %s", err))
	}
	return h.redirect(w, r, back)
}

// HandleKindDelete removes a kind but never its entries.
func (h *Handler) HandleKindDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, _, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("kindID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	back := "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/inhaltsarten"

	t, err := h.kinds.Get(r.Context(), websiteID, id)
	if err != nil {
		return h.kindFailed(w, r, back, err)
	}
	n, err := h.kinds.Count(r.Context(), websiteID, t.Key)
	if err != nil {
		return err
	}
	if err := h.kinds.Delete(r.Context(), websiteID, id); err != nil {
		return err
	}

	// Die Einträge bleiben, und das wird gesagt. Sie behalten ihre Kennung,
	// stehen in der Liste unter ihr und lassen sich im Editor auf eine andere
	// Art umstellen — eine versehentlich gelöschte Art darf nicht hundert
	// Produkte mitnehmen.
	if n > 0 {
		web.SetFlashWarning(h.sm, r.Context(), web.Titlef(r,
			"Inhaltsart entfernt. Die %d Einträge sind noch da und tragen weiter die Kennung „%s“ — stelle sie im Editor auf eine andere Art um oder lege die Art wieder an.", n, t.Key))
	} else {
		web.SetFlashSuccess(h.sm, r.Context(), "Inhaltsart entfernt")
	}
	return h.redirect(w, r, back)
}

// HandleKindMove shifts a kind one place in the order.
func (h *Handler) HandleKindMove(w http.ResponseWriter, r *http.Request) error {
	websiteID, _, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("kindID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	if err := h.kinds.Move(r.Context(), websiteID, id, r.FormValue("richtung") == "hoch"); err != nil {
		return err
	}
	return h.redirect(w, r, "/admin/websites/"+strconv.FormatInt(websiteID, 10)+"/inhaltsarten")
}

// kindsOf are a website's own kinds, or none.
//
// A failure is not worth a failed screen: without the list the editor simply
// offers page and post, which is what every website had before.
func (h *Handler) kindsOf(r *http.Request, websiteID int64) []kind.Type {
	if h.kinds == nil {
		return nil
	}
	types, err := h.kinds.List(r.Context(), websiteID)
	if err != nil {
		return nil
	}
	return types
}

// setKind resolves the "Art" a form sent against this website's own kinds.
//
// Afterwards Kind is the built-in behaviour — an entry of an own kind is a page
// — and TypeKey is the own kind or empty. Anything the website does not have
// becomes a page: a form can be sent with any value in it, and an entry filed
// under a kind that does not exist would be invisible in every list.
func (v *PageValues) setKind(types []kind.Type) {
	switch key := kind.Pick(v.TypeKey, types); key {
	case kind.Post:
		v.Kind, v.TypeKey = page.KindPost, ""
	case kind.Page:
		v.Kind, v.TypeKey = page.KindPage, ""
	default:
		v.Kind, v.TypeKey = page.KindPage, key
	}
}

// ownKindName is what to call an entry of an own kind in a list, empty for the
// built-in two. A kind that was deleted while its entries remain is shown by
// its key, so a row never becomes nameless.
func ownKindName(types []kind.Type, p page.Page) string {
	if p.TypeKey == "" {
		return ""
	}
	return kind.NameOf(types, p.TypeKey, false)
}

// KindChoice is one entry of the "Art" dropdown in the page editor.
type KindChoice struct {
	Key      string
	Name     string
	Selected bool
}

// kindChoices is the dropdown: the two built-in kinds and the website's own.
func kindChoices(types []kind.Type, current string) []KindChoice {
	out := []KindChoice{
		{Key: kind.Page, Name: "Seite", Selected: current != kind.Post && !hasKey(types, current)},
		{Key: kind.Post, Name: "Beitrag", Selected: current == kind.Post},
	}
	for _, t := range types {
		out = append(out, KindChoice{Key: t.Key, Name: t.Name, Selected: current == t.Key})
	}
	// Eine Art, die es nicht mehr gibt, deren Einträge sie aber noch tragen:
	// sie steht mit ihrer Kennung da, damit ein Eintrag beim Speichern nicht
	// still zur Seite wird.
	if current != "" && current != kind.Page && current != kind.Post && !hasKey(types, current) {
		out = append(out, KindChoice{Key: current, Name: current, Selected: true})
	}
	return out
}

func hasKey(types []kind.Type, key string) bool {
	_, ok := kind.Find(types, key)
	return ok
}
