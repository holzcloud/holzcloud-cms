package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// blockForm baut die Felder, die der Editor absendet.
func blockForm(ws int64, extra url.Values) url.Values {
	v := url.Values{
		"title":     {"Startseite"},
		"slug":      {"start"},
		"status":    {"published"},
		"kind":      {"page"},
		"bausteine": {"1"},
	}
	for k, vals := range extra {
		v[k] = vals
	}
	return v
}

// Der ganze Weg: Formular, Speichern, ausgegebenes HTML.
func TestBlocksAreStoredAndRendered(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	req := postForm("/admin/websites/1/pages/new", blockForm(ws.ID, url.Values{
		"b0.typ":      {"text"},
		"b0.markdown": {"## Willkommen\n\nIn der Velowerkstatt."},
		"b1.typ":      {"zitat"},
		"b1.text":     {"Schöne Tiere."},
		"b1.quelle":   {"Eine Kundin"},
	}), map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	serve(t, h, sm, h.HandlePageCreate, req)

	p, err := page.NewStore(database).GetPageBySlug(context.Background(), ws.ID, "start")
	if err != nil || p == nil {
		t.Fatalf("the page was not created: %v", err)
	}

	// Die Bausteine liegen als solche in der Datenbank.
	blocks, err := block.Decode(p.Blocks, block.Builtin)
	if err != nil || len(blocks) != 2 {
		t.Fatalf("Bausteine = %+v, %v", blocks, err)
	}

	// The delivered HTML carries the frame and the text.
	for _, wollte := range []string{
		`class="hc-block hc-text"`, "Willkommen", `class="hc-block hc-zitat"`, "Eine Kundin",
	} {
		if !strings.Contains(p.ContentHTML, wollte) {
			t.Errorf("%q fehlt im HTML:\n%s", wollte, p.ContentHTML)
		}
	}

	// And the plain text is there, or the page would be invisible to the
	// website's own search.
	for _, wollte := range []string{"Willkommen", "Schöne Tiere", "Eine Kundin"} {
		if !strings.Contains(p.ContentMarkdown, wollte) {
			t.Errorf("%q fehlt im reinen Text:\n%s", wollte, p.ContentMarkdown)
		}
	}
}

// A button in the editor is not a save. It must not create the page and has to
// hand back what has already been typed.
func TestEineEditoraktionSpeichertNicht(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	req := postForm("/admin/websites/1/pages/new", blockForm(ws.ID, url.Values{
		"b0.typ":         {"text"},
		"b0.markdown":    {"Ein angefangener Satz"},
		"bausteinaktion": {"neu:bild"},
	}), map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	body := serve(t, h, sm, h.HandlePageCreate, req).Body.String()

	p, _ := page.NewStore(database).GetPageBySlug(context.Background(), ws.ID, "start")
	if p != nil {
		t.Error("die Aktion hat die Seite angelegt")
	}
	if !strings.Contains(body, "Ein angefangener Satz") {
		t.Errorf("the typed text did not come back:\n%s", body)
	}
	// And the new block now stands in it.
	if !strings.Contains(body, `name="b1.typ" value="bild"`) {
		t.Errorf("the new block is missing:\n%s", body)
	}
}

// With htmx only the list comes back, without htmx the whole form. Both have to
// carry the state, or one of the two paths loses the text.
func TestEditoraktionMitUndOhneHtmx(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)

	values := blockForm(ws.ID, url.Values{
		"b0.typ":         {"text"},
		"b0.markdown":    {"Bleibt stehen"},
		"bausteinaktion": {"neu:trenner"},
	})

	req := postForm("/admin/websites/1/pages/new", values,
		map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	req.Header.Set("HX-Request", "true")
	teil := serve(t, h, sm, h.HandlePageCreate, req).Body.String()

	if strings.Contains(teil, "<html") || strings.Contains(teil, `name="title"`) {
		t.Errorf("htmx bekam mehr als die Liste:\n%s", teil)
	}
	if !strings.Contains(teil, "Bleibt stehen") || !strings.Contains(teil, `value="trenner"`) {
		t.Errorf("the list is incomplete:\n%s", teil)
	}

	req = postForm("/admin/websites/1/pages/new", values,
		map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	ganz := serve(t, h, sm, h.HandlePageCreate, req).Body.String()
	if !strings.Contains(ganz, `name="title"`) {
		t.Errorf("without htmx the surrounding form is missing:\n%s", ganz)
	}
	if !strings.Contains(ganz, "Bleibt stehen") {
		t.Errorf("without htmx the text was lost:\n%s", ganz)
	}
}

// The way into the blocks turns the text into a block without taking it apart —
// and without saving in the process.
func TestSwitchingIntoTheBlocks(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "Ein langer Artikel.", "published")

	req := postForm("/admin/websites/1/pages/1/edit", url.Values{
		"title":            {"Über uns"},
		"slug":             {"ueber-uns"},
		"status":           {"published"},
		"kind":             {"page"},
		"version":          {strconv.FormatInt(p.Version, 10)},
		"content_markdown": {"Ein langer Artikel."},
		"editorwechsel":    {"zu-bausteinen"},
	}, map[string]string{"id": "1", "pageID": "1"})
	body := serve(t, h, sm, h.HandlePageEdit, req).Body.String()

	if !strings.Contains(body, `name="b0.typ" value="text"`) {
		t.Errorf("the text did not become a block:\n%s", body)
	}
	if !strings.Contains(body, "Ein langer Artikel.") {
		t.Errorf("the text was lost in the switch:\n%s", body)
	}

	// Nothing was saved: the page is markdown, unchanged.
	nach, _ := page.NewStore(database).GetPage(context.Background(), p.ID)
	if nach.Blocks != "" {
		t.Errorf("der Wechsel hat gespeichert: %q", nach.Blocks)
	}
}

// An image from another website's media library must not be reachable through a
// typed-in number.
func TestAnImageOfAForeignWebsiteIsNotRendered(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	fremd, err := h.domains.CreateWebsite(context.Background(), "Andere", "")
	if err != nil {
		t.Fatal(err)
	}
	res, err := database.Write.Exec(
		`INSERT INTO media (website_id, filename, original_name, mime_type, size_bytes, content_hash, alt_text, width, height)
		 VALUES ($1, 'fremd.jpg', 'fremd.jpg', 'image/jpeg', 100, 'abc', 'Fremdes Bild', 800, 600)`,
		fremd.ID)
	if err != nil {
		t.Fatalf("Medium anlegen: %v", err)
	}
	id, _ := res.LastInsertId()

	html := h.renderBlocks(context.Background(), ws.ID, block.Builtin, []block.Block{{
		Type: block.TypeImage, MediaID: id,
	}})
	if strings.Contains(html, "fremd.jpg") {
		t.Errorf("the other website's image was rendered:\n%s", html)
	}

	// Auf der eigenen Website erscheint dasselbe Bild sehr wohl.
	res, _ = database.Write.Exec(
		`INSERT INTO media (website_id, filename, original_name, mime_type, size_bytes, content_hash, alt_text, width, height)
		 VALUES ($1, 'eigen.jpg', 'eigen.jpg', 'image/jpeg', 100, 'def', 'Eigenes Bild', 800, 600)`,
		ws.ID)
	eigen, _ := res.LastInsertId()
	html = h.renderBlocks(context.Background(), ws.ID, block.Builtin, []block.Block{{
		Type: block.TypeImage, MediaID: eigen,
	}})
	if !strings.Contains(html, "eigen.jpg") || !strings.Contains(html, `alt="Eigenes Bild"`) {
		t.Errorf("das eigene Bild fehlt:\n%s", html)
	}
}

// A page made of blocks keeps them across saving and opening again — the most
// common way for something like this to lose content.
func TestBlocksSurviveBeingEditedAgain(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	req := postForm("/admin/websites/1/pages/new", blockForm(ws.ID, url.Values{
		"b0.typ":      {"karten"},
		"b0.variante": {"2"},
		"b0.e0.titel": {"Rohwolle"},
		"b0.e1.titel": {"Gekardet"},
	}), map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	serve(t, h, sm, h.HandlePageCreate, req)

	p, _ := page.NewStore(database).GetPageBySlug(context.Background(), ws.ID, "start")
	if p == nil {
		t.Fatal("the page was not created")
	}

	get := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/edit", nil)
	get.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	get.SetPathValue("pageID", strconv.FormatInt(p.ID, 10))
	body := serve(t, h, sm, h.HandlePageEdit, get).Body.String()

	for _, wollte := range []string{`value="karten"`, "Rohwolle", "Gekardet"} {
		if !strings.Contains(body, wollte) {
			t.Errorf("%q is missing from the reopened editor:\n%s", wollte, body)
		}
	}
	// And the simple editor is not offered, because the cards would be lost in
	// the process.
	if strings.Contains(body, "zu-markdown") {
		t.Error("the way back is offered although the cards were lost")
	}
}

// A block kind of one's own survives the saving of an existing page.
//
// For a long time it did not: handlePageCreatePost set values.BlockSet,
// handlePageEditPost did not — and an empty set knows only the nine built-in
// kinds. Clean thereupon discarded every block of a kind of one's own on
// saving, without a message, and Apply created no new one. A page with six
// features came back as running text, and the editor no longer offered the kind
// it could have been restored with.
func TestAnOwnBlockKindSurvivesEditing(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	fields := field.NewStore(database)
	art, err := block.NewStore(database, fields).Create(ctx, ws.ID, "Merkmal", "")
	if err != nil {
		t.Fatalf("Bausteinart anlegen: %v", err)
	}
	if _, err := fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "begriff", Label: "Begriff",
		Kind: field.KindText, BlockTypeID: art.ID,
	}); err != nil {
		t.Fatalf("Feld anlegen: %v", err)
	}

	// Anlegen — dieser Weg war immer richtig.
	req := postForm("/admin/websites/1/pages/new", blockForm(ws.ID, url.Values{
		"b0.typ":       {"text"},
		"b0.markdown":  {"Ein Absatz."},
		"b1.typ":       {art.Key},
		"b1.f.begriff": {"Eine Installation"},
	}), map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	serve(t, h, sm, h.HandlePageCreate, req)

	pages := page.NewStore(database)
	p, err := pages.GetPageBySlug(ctx, ws.ID, "start")
	if err != nil || p == nil {
		t.Fatalf("the page was not created: %v", err)
	}

	// Editing without changing anything at all.
	req = postForm("/admin/websites/1/pages/1/edit", blockForm(ws.ID, url.Values{
		"b0.typ":       {"text"},
		"b0.markdown":  {"Ein Absatz."},
		"b1.typ":       {art.Key},
		"b1.f.begriff": {"Eine Installation"},
		"version":      {strconv.FormatInt(p.Version, 10)},
	}), map[string]string{
		"id": strconv.FormatInt(ws.ID, 10), "pageID": strconv.FormatInt(p.ID, 10),
	})
	serve(t, h, sm, h.HandlePageEdit, req)

	p, err = pages.GetPageBySlug(ctx, ws.ID, "start")
	if err != nil || p == nil {
		t.Fatalf("the page is gone after editing: %v", err)
	}
	set := block.Set{Own: []block.Own{{ID: art.ID, Key: art.Key, Name: art.Name,
		Fields: []field.Def{{Key: "begriff"}}}}}
	blocks, err := block.Decode(p.Blocks, set)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("after editing %d blocks, wanted 2: %+v", len(blocks), blocks)
	}
	if blocks[1].Type != art.Key {
		t.Errorf("block 1 is %q, wanted %q", blocks[1].Type, art.Key)
	}
	if got := blocks[1].Fields["begriff"]; got != "Eine Installation" {
		t.Errorf("Feldwert %q, wollte %q", got, "Eine Installation")
	}
}

// And the kind can be added while editing, too.
func TestAnOwnBlockKindCanBeAddedWhileEditing(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	fields := field.NewStore(database)
	art, err := block.NewStore(database, fields).Create(ctx, ws.ID, "Merkmal", "")
	if err != nil {
		t.Fatalf("Bausteinart anlegen: %v", err)
	}
	p := seedPage(t, database, ws.ID, "Titel", "titel", "Ein Absatz.", "draft")

	req := postForm("/admin/websites/1/pages/1/edit", blockForm(ws.ID, url.Values{
		"title":           {"Titel"},
		"slug":            {"titel"},
		"b0.typ":          {"text"},
		"b0.markdown":     {"Ein Absatz."},
		"version":         {strconv.FormatInt(p.Version, 10)},
		block.ActionField: {block.ActionAdd + ":" + art.Key},
	}), map[string]string{
		"id": strconv.FormatInt(ws.ID, 10), "pageID": strconv.FormatInt(p.ID, 10),
	})
	req.Header.Set("HX-Request", "true")
	rec := serve(t, h, sm, h.HandlePageEdit, req)

	// Two blocks in the returned fragment, not one.
	if got := strings.Count(rec.Body.String(), `name="b1.typ"`); got != 1 {
		t.Errorf("the new block is missing from the form (b1.typ %d times):\n%s", got, rec.Body.String())
	}
}

// The third place where field names come about. On the page itself
// Def.FieldName mints the marker, in a group row groupView appends NameSuffix —
// and in the block editor the same source has to hold. Without it the form
// draws a checkbox group without "[]", the parser keeps values[0], and because
// the guard stands before the group that is the empty string: every checkbox
// would be lost on every save, without a message.
func TestAMultipleChoiceInAnOwnBlockKindSurvivesSaving(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	fields := field.NewStore(database)
	art, err := block.NewStore(database, fields).Create(ctx, ws.ID, "Merkmal", "")
	if err != nil {
		t.Fatalf("Bausteinart anlegen: %v", err)
	}
	if _, err := fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "hoelzer", Label: "Hölzer", Kind: field.KindMulti,
		Choices: []string{"Eiche", "Buche", "Esche"}, BlockTypeID: art.ID,
	}); err != nil {
		t.Fatalf("Feld anlegen: %v", err)
	}

	p := seedPage(t, database, ws.ID, "Titel", "titel", "Ein Absatz.", "draft")

	// The form first: the editor has to draw the marker itself, or no browser
	// ever sends it along.
	req := postForm("/admin/websites/1/pages/1/edit", blockForm(ws.ID, url.Values{
		"title":           {"Titel"},
		"slug":            {"titel"},
		"b0.typ":          {"text"},
		"b0.markdown":     {"Ein Absatz."},
		"version":         {strconv.FormatInt(p.Version, 10)},
		block.ActionField: {block.ActionAdd + ":" + art.Key},
	}), map[string]string{
		"id": strconv.FormatInt(ws.ID, 10), "pageID": strconv.FormatInt(p.ID, 10),
	})
	req.Header.Set("HX-Request", "true")
	rec := serve(t, h, sm, h.HandlePageEdit, req)
	body := rec.Body.String()
	if !strings.Contains(body, `name="b1.f.hoelzer[]"`) {
		t.Errorf("the checkbox group does not carry the marking:\n%s", body)
	}
	if strings.Contains(body, `name="b1.f.hoelzer"`) {
		t.Errorf("the group is in the form without a marking:\n%s", body)
	}
	if !strings.Contains(body, `<input type="hidden" name="b1.f.hoelzer[]" value="">`) {
		t.Errorf("the sentinel is missing before the group:\n%s", body)
	}

	// And then what this form submits: the guard in front, two ticks.
	req = postForm("/admin/websites/1/pages/1/edit", blockForm(ws.ID, url.Values{
		"title":          {"Titel"},
		"slug":           {"titel"},
		"b0.typ":         {"text"},
		"b0.markdown":    {"Ein Absatz."},
		"b1.typ":         {art.Key},
		"b1.f.hoelzer[]": {"", "Eiche", "Buche"},
		"version":        {strconv.FormatInt(p.Version, 10)},
	}), map[string]string{
		"id": strconv.FormatInt(ws.ID, 10), "pageID": strconv.FormatInt(p.ID, 10),
	})
	serve(t, h, sm, h.HandlePageEdit, req)

	saved, err := page.NewStore(database).GetPageBySlug(ctx, ws.ID, "titel")
	if err != nil || saved == nil {
		t.Fatalf("the page is gone after saving: %v", err)
	}
	// Read back through the website's own set, the one every production reader
	// gets from the store. This used to be a hand-built set whose field had no
	// choices; since Clean holds an own kind's values to field.Check (v1.6
	// audit), such a set refuses "Eiche" on the way out, and the test measured
	// its own stand-in instead of what was stored.
	set := block.NewStore(database, fields).Set(ctx, ws.ID)
	blocks, err := block.Decode(saved.Blocks, set)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("%d Bausteine, wollte 2: %+v", len(blocks), blocks)
	}
	if got, will := blocks[1].Fields["hoelzer"], "Eiche\nBuche"; got != will {
		t.Errorf("gespeichert wurde %q, wollte %q", got, will)
	}
}
