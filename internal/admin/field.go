package admin

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The screen where a website gets its own fields.
//
// One list, one form beneath it, no separate page for editing — a field has
// five properties, and sending someone to another screen for five properties
// is more navigation than the thing is worth.

// FieldListData is the field screen.
type FieldListData struct {
	web.LayoutData
	WebsiteID int64
	Fields    []field.Def
	Kinds     []field.Kind
	// Edit is the field being changed, or nil when the form is for a new one.
	Edit *field.Def
	// EditChoices is the choice list of the field being changed, as text.
	EditChoices string
	// Group is the group whose fields are being edited, or nil for the top
	// level. When it is set, the screen shows that group's fields and the form
	// adds to it.
	Group *field.Def
	// Types are the website's own content kinds, so a field can belong to one
	// of them rather than to every page.
	Types []kind.Type
	// Controls are the fields a condition may hang on. Not every field can be
	// one — see field.Def.MayControl — and the list is the whole explanation:
	// what is not offered cannot be chosen wrong.
	Controls []field.Def
	// BlockType is the block kind whose fields are being edited, or nil for the
	// page's own fields. The third mode of this screen, after the top level and
	// a group.
	BlockType *block.Own
	// Snippet is the text snippet whose fields are being edited, or nil. The
	// fourth mode of this screen, after the top level, a group and a block
	// kind — a snippet stops being one Markdown box and becomes a small
	// content model of its own.
	Snippet *snippet.Snippet
}

// TypeName is the plural of one own content kind, for the "gilt für" column.
//
// Falls back to the key: a field can be tied to a kind that was removed later,
// and "nur produkt" is still more use than an empty cell — it says which kind
// to define again to see the field.
func (d FieldListData) TypeName(key string) string {
	for _, t := range d.Types {
		if t.Key == key {
			return t.Plural
		}
	}
	return key
}

// Simple reports whether this is the website's own top-level field list — the
// one place where a field can say which pages it belongs to and hang on
// another. Inside a group, inside a block kind and on a text snippet none of
// those questions applies: „gilt für" names a kind of page, and a snippet is
// not a page.
func (d FieldListData) Simple() bool {
	return d.Group == nil && d.BlockType == nil && d.Snippet == nil
}

// snippetOf returns the snippet with this id if it belongs to this website, and
// nil otherwise.
//
// The website belongs to the lookup and is not merely checked afterwards — the
// same reason field.Store.Get carries: the id comes out of an address, and
// without the check an editor reaches another site's snippet by typing its
// number.
//
// Why this exists here and not for block kinds: blockTypes.Get takes the
// website id and simply does not find a block kind of another site, while
// snippets.Get takes only an id. That asymmetry is the whole reason this is one
// function rather than two inline comparisons that can drift apart — the GET
// that opens the mode and the POST that creates a field must ask the same
// question.
func (h *Handler) snippetOf(ctx context.Context, websiteID, id int64) *snippet.Snippet {
	// The comparison this function was written to hold in one place has moved
	// into the store, which is where the comment above always said it belonged.
	// The function stays: it is the one shape the two callers want — nil for
	// "not yours and not here", without either of them writing an err check.
	sn, err := h.snippets.Get(ctx, websiteID, id)
	if err != nil {
		return nil
	}
	return sn
}

// HandleFieldList shows a website's fields.
func (h *Handler) HandleFieldList(w http.ResponseWriter, r *http.Request) error {
	websiteID, ws, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	// ?gruppe=<id> narrows the screen to one group's own fields. A query
	// parameter and not a second screen: it is the same list and the same form,
	// one level down.
	var group *field.Def
	if id, gerr := strconv.ParseInt(r.URL.Query().Get("gruppe"), 10, 64); gerr == nil {
		if def, err := h.fields.Get(r.Context(), websiteID, id); err == nil && def.IsGroup() {
			group = def
		}
	}
	// ?baustein=<id> narrows it to one block kind's fields, the same way.
	var blockType *block.Own
	if id, berr := strconv.ParseInt(r.URL.Query().Get("baustein"), 10, 64); berr == nil {
		if t, gerr := h.blockTypes.Get(r.Context(), websiteID, id); gerr == nil {
			blockType = t
			group = nil
		}
	}
	// ?textbaustein=<id> narrows it to one text snippet's fields. The name is
	// not „baustein": that one is taken by block kinds, and the two German
	// words differ by one prefix — a reader who has to guess which is which
	// will eventually guess wrong.
	var snip *snippet.Snippet
	if id, serr := strconv.ParseInt(r.URL.Query().Get("textbaustein"), 10, 64); serr == nil {
		if sn := h.snippetOf(r.Context(), websiteID, id); sn != nil {
			snip = sn
			group = nil
		}
	}

	data, err := h.fieldListData(r, websiteID, ws.Name, group, blockType, snip)
	if err != nil {
		return err
	}

	// ?aendern=<id> opens the same form filled in. The list stays visible, so
	// one can see what the other fields are called while renaming one.
	if id, cerr := strconv.ParseInt(r.URL.Query().Get("aendern"), 10, 64); cerr == nil {
		if def, gerr := h.fields.Get(r.Context(), websiteID, id); gerr == nil {
			data.Edit = def
			data.EditChoices = field.JoinChoices(def.Choices)
		}
	}
	return web.RenderAdmin(w, h.templates, r, "field_list", data)
}

// HandleFieldSave creates or changes a field.
func (h *Handler) HandleFieldSave(w http.ResponseWriter, r *http.Request) error {
	websiteID, _, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	if err := r.ParseForm(); err != nil {
		return err
	}

	parentID, _ := strconv.ParseInt(r.FormValue("gruppe"), 10, 64)
	blockTypeID, _ := strconv.ParseInt(r.FormValue("baustein"), 10, 64)
	snippetID, _ := strconv.ParseInt(r.FormValue("textbaustein"), 10, 64)
	def := field.Def{
		WebsiteID:   websiteID,
		ParentID:    parentID,
		BlockTypeID: blockTypeID,
		SnippetID:   snippetID,
		Label:       r.FormValue("beschriftung"),
		Kind:        r.FormValue("art"),
		Required:    r.FormValue("pflicht") == "1",
		Hint:        r.FormValue("hinweis"),
		Choices:     field.SplitChoices(r.FormValue("auswahl")),
		AppliesTo:   r.FormValue("gilt_fuer"),
		Condition:   r.FormValue("bedingung"),
		Display:     r.FormValue("darstellung"),
		RangeMin:    r.FormValue("min_wert"),
		RangeMax:    r.FormValue("max_wert"),
	}
	// Ein leeres Kästchen heisst keine Obergrenze, und das ist genau die Null,
	// die Atoi bei einem Fehler ohnehin zurückgibt — deshalb bleibt der Fehler
	// hier liegen. Eine ausgeschriebene Zahl unter null lehnt validate ab; sie
	// wird hier nicht stillschweigend zurechtgebogen.
	def.MaxValues, _ = strconv.Atoi(r.FormValue("max_werte"))

	// Ein Unterfeld erbt seinen Träger von der Gruppe, in der es steht, und
	// nicht aus dem Formular. Der Gruppenbildschirm ist eine Ebene tiefer und
	// weiss von keinem Textbaustein — ohne diese Zeilen bekäme das Unterfeld
	// einer Gruppe an einem Textbaustein snippet_id NULL, und OfSnippet, das
	// „WHERE snippet_id = $2" fragt, gäbe die Gruppe ohne ihre Unterfelder
	// heraus: eine Gruppe, die auf dem Formular des Textbausteins keine einzige
	// Zeile zeichnet. Aus dem Gespeicherten gepinnt und nicht aus dem Körper
	// gelesen, aus demselben Grund, aus dem Update seinen Träger pinnt — was
	// nicht aus dem Formular kommt, kann auch nicht gefälscht werden.
	if def.ParentID > 0 {
		parent, gerr := h.fields.Get(r.Context(), websiteID, def.ParentID)
		if gerr != nil || parent == nil || !parent.IsGroup() {
			http.NotFound(w, r)
			return nil
		}
		def.SnippetID = parent.SnippetID
		def.BlockTypeID = parent.BlockTypeID
	}

	// The hidden input is a courtesy; the body is not. A snippet id that names
	// another website's snippet must never reach the store, so it is refused
	// here — before anything is written.
	if def.SnippetID > 0 && h.snippetOf(r.Context(), websiteID, def.SnippetID) == nil {
		http.NotFound(w, r)
		return nil
	}

	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if id > 0 {
		err = h.fields.Update(r.Context(), websiteID, id, def)
	} else {
		_, err = h.fields.Create(r.Context(), def)
	}
	switch {
	case errors.Is(err, field.ErrNested):
		web.SetFlashError(h.sm, r.Context(),
			"Eine Gruppe in einer Gruppe gibt es nicht — eine Ebene, damit sich im Formular noch jemand zurechtfindet.")
	case errors.Is(err, field.ErrKindFixed):
		web.SetFlashError(h.sm, r.Context(),
			"Aus einer Gruppe wird kein einfaches Feld und umgekehrt: die Zeilen hätten nirgends hin.")
	case errors.Is(err, field.ErrDuplicateKey):
		web.SetFlashError(h.sm, r.Context(),
			"Ein Feld mit dieser Kennung gibt es schon. Wähle eine andere Beschriftung.")
	case errors.Is(err, field.ErrNotInBlock):
		web.SetFlashError(h.sm, r.Context(),
			"Diese Art von Feld gibt es in einem Baustein nicht. Ein Verweis folgt einer Umbenennung, und der Baustein wird beim Speichern der Seite ein für alle Mal in HTML verwandelt — er könnte dieses Versprechen nicht halten. Nimm einen Link.")
	case errors.Is(err, field.ErrNoCondition):
		web.SetFlashError(h.sm, r.Context(),
			"An dieses Feld lässt sich keine Bedingung hängen. Möglich sind die Felder, bei denen der Browser sehen kann, ob sie ausgefüllt sind — ein Datum und eine Gruppe gehören nicht dazu.")
	case errors.Is(err, field.ErrConditionLoop):
		web.SetFlashError(h.sm, r.Context(),
			"Die Bedingungen drehen sich im Kreis: keines der beteiligten Felder wäre je zu sehen.")
	case errors.Is(err, field.ErrRangeInverted):
		web.SetFlashError(h.sm, r.Context(),
			"Die untere Grenze liegt über der oberen — so gäbe es keine Zahl, die dazwischenpasst. Vertausche die beiden Werte.")
	case errors.Is(err, field.ErrTooMany):
		web.SetFlashError(h.sm, r.Context(),
			"Mehr Felder werden nicht angelegt — ein Formular, das so lang ist, füllt niemand richtig aus.")
	// The three carriers a field can hang off, answered by name.
	//
	// They were missing from this switch, so they fell through to err.Error()
	// below — and the store composes them with fmt.Errorf("%w: %s", …,
	// "gehört zu einer anderen Website"), which no catalogue can hold whichever
	// half is marked. Reachable by posting a baustein= id that belongs to
	// another website, which this handler does not pre-check (it pre-checks
	// only snippetID, above).
	case errors.Is(err, field.ErrNoGroup):
		web.SetFlashError(h.sm, r.Context(),
			"Diese Gruppe gibt es nicht, oder sie gehört zu einer anderen Website.")
	case errors.Is(err, field.ErrNoSnippet):
		web.SetFlashError(h.sm, r.Context(),
			"Diesen Textbaustein gibt es nicht, oder er gehört zu einer anderen Website.")
	case errors.Is(err, field.ErrNoBlockType):
		web.SetFlashError(h.sm, r.Context(),
			"Diese Bausteinart gibt es nicht, oder sie gehört zu einer anderen Website.")
	case err != nil:
		// What is left is a database failure, and its text is a German
		// fmt.Errorf wrap around a driver message ("feld anlegen: database is
		// locked"). Putting that on a screen answered an English admin in two
		// languages at once and told the operator nothing they could act on.
		// The sentence they read is a collected one; the wrap goes to the log,
		// where its detail is worth something.
		slog.Error("save field", "err", err, "website", websiteID)
		web.SetFlashError(h.sm, r.Context(), "Speichern fehlgeschlagen.")
	case id > 0:
		web.SetFlashSuccess(h.sm, r.Context(), "Feld geändert.")
	default:
		web.SetFlashSuccess(h.sm, r.Context(),
			"Feld angelegt. Es steht ab sofort im Editor und im Theme.")
	}
	return h.redirect(w, r, fieldPath(websiteID, parentID, blockTypeID, snippetID))
}

// HandleFieldDelete removes a field definition.
func (h *Handler) HandleFieldDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, _, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("fieldID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	parentID, blockTypeID, snippetID := int64(0), int64(0), int64(0)
	if def, gerr := h.fields.Get(r.Context(), websiteID, id); gerr == nil {
		parentID, blockTypeID, snippetID = def.ParentID, def.BlockTypeID, def.SnippetID
	}
	if err := h.fields.Delete(r.Context(), websiteID, id); err != nil {
		return err
	}
	// Deliberately says what did not happen: the values are still on the pages,
	// and someone who deleted the wrong field should know they can get it back.
	web.SetFlashSuccess(h.sm, r.Context(),
		"Feld entfernt. Das Ausgefüllte bleibt an den Seiten stehen, bis sie das nächste Mal gespeichert werden — "+
			"wer sich vertan hat, legt das Feld einfach wieder an.")
	return h.redirect(w, r, fieldPath(websiteID, parentID, blockTypeID, snippetID))
}

// HandleFieldMove shifts a field one place.
func (h *Handler) HandleFieldMove(w http.ResponseWriter, r *http.Request) error {
	websiteID, _, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("fieldID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	parentID, blockTypeID, snippetID := int64(0), int64(0), int64(0)
	if def, gerr := h.fields.Get(r.Context(), websiteID, id); gerr == nil {
		parentID, blockTypeID, snippetID = def.ParentID, def.BlockTypeID, def.SnippetID
	}
	if err := h.fields.Move(r.Context(), websiteID, id, r.URL.Query().Get("richtung") == "hoch"); err != nil {
		return err
	}
	return h.redirect(w, r, fieldPath(websiteID, parentID, blockTypeID, snippetID))
}

func (h *Handler) fieldListData(r *http.Request, websiteID int64, websiteName string,
	group *field.Def, blockType *block.Own, snip *snippet.Snippet) (FieldListData, error) {

	// Through web.Titlef and not by concatenation, which is what this screen
	// did until 2026-09-08. A concatenated title has no string literal at the
	// argument index tools/i18n reads, so it was neither offen nor verwaist —
	// the gate did not know it existed, and the screen printed German to a
	// reader whose admin was English. Found by switching the language and
	// looking, because "Felder – " carries no umlaut either and no gate hunting
	// German-looking literals could see it. The pattern is album.go's.
	var (
		defs  []field.Def
		err   error
		title = web.Titlef(r, "Felder – %s", websiteName)
		kinds = field.Kinds
	)
	switch {
	case blockType != nil:
		defs, err = h.fields.OfBlockType(r.Context(), websiteID, blockType.ID)
		title = web.Titlef(r, "Baustein „%s“ – %s", blockType.Name, websiteName)
		kinds = field.BlockKinds()
	case snip != nil:
		defs, err = h.fields.OfSnippet(r.Context(), websiteID, snip.ID)
		title = web.Titlef(r, "Textbaustein „%s“ – %s", snip.Name, websiteName)
		// The full palette, not BlockKinds(): its four exclusions exist because
		// a block freezes to HTML when the page is saved, while a snippet's
		// values are resolved on the way out. A reference and a label field can
		// keep their promise here, so they are offered.
		kinds = field.Kinds
	case group != nil:
		defs, err = h.fields.Sub(r.Context(), websiteID, group.ID)
		title = web.Titlef(r, "Gruppe „%s“ – %s", group.Label, websiteName)
		kinds = field.SubKinds()
	default:
		defs, err = h.fields.List(r.Context(), websiteID)
	}
	if err != nil {
		return FieldListData{}, err
	}

	data := FieldListData{
		LayoutData: web.NewLayoutData(r, h.sm, title),
		WebsiteID:  websiteID,
		Fields:     defs,
		Kinds:      kinds,
		Group:      group,
		BlockType:  blockType,
		Snippet:    snip,
		Types:      h.kindsOf(r, websiteID),
	}
	if group == nil && blockType == nil && snip == nil {
		for _, d := range defs {
			if d.MayControl() {
				data.Controls = append(data.Controls, d)
			}
		}
	}
	data.ActiveNav = "fields"
	return data, nil
}

// fieldPath is the screen a change returns to: the group's own list, the block
// kind's own list, the text snippet's own list, or the website's.
//
// A definition never carries two carriers at once, so the order between the
// block kind and the snippet is a readability choice and not a behaviour.
func fieldPath(websiteID, parentID, blockTypeID, snippetID int64) string {
	path := "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/felder"
	switch {
	case blockTypeID > 0:
		path += "?baustein=" + strconv.FormatInt(blockTypeID, 10)
	case snippetID > 0:
		path += "?textbaustein=" + strconv.FormatInt(snippetID, 10)
	case parentID > 0:
		path += "?gruppe=" + strconv.FormatInt(parentID, 10)
	}
	return path
}
