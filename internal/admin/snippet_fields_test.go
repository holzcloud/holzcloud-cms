package admin

import (
	"context"
	stdhtml "html"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
)

// The fourth mode of the field screen, checked where the permission lives.
//
// `?textbaustein=<id>` is the one place in this phase where a number out of the
// address names a carrier that `snippets.Get` looks up **without** a website
// number. For a block kind the query itself takes care of that; here it is the
// handler's business, and that is why the check stands in `internal/admin` and
// not in the store. It is checked against the real templates from disk, because
// a template that no longer fits its data structure should fail here and not in
// the browser.

// fieldScreen calls GET …/felder with the query it is given.
func fieldScreen(t *testing.T, h *Handler, sm *scs.SessionManager, websiteID int64, query string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/felder"
	if query != "" {
		target += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	return serve(t, h, sm, h.HandleFieldList, req)
}

// createField submits the form of the field screen.
func createField(t *testing.T, h *Handler, sm *scs.SessionManager, websiteID int64, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, h, sm, h.HandleFieldSave, postForm(
		"/admin/websites/"+strconv.FormatInt(websiteID, 10)+"/felder",
		values, map[string]string{"id": strconv.FormatInt(websiteID, 10)}))
}

// secondWebsite creates a second website with a snippet of its own — the other
// side of every permission check in this file.
func zweiteWebsite(t *testing.T, database *db.DB, name, key, snippetName string) (*domain.Website, *snippet.Snippet) {
	t.Helper()
	ctx := context.Background()
	ws, err := domain.NewStore(database).CreateWebsite(ctx, name, "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	sn, err := snippet.NewStore(database).Create(ctx, ws.ID, key, snippetName, "Inhalt", "<p>Inhalt</p>")
	if err != nil {
		t.Fatalf("snippet.Create: %v", err)
	}
	return ws, sn
}

// The mode opens for a snippet of one's own, carries its name and creates a
// field that comes back through OfSnippet.
// A group on a snippet draws its rows — the fault that only the browser pass
// brought to light.
//
// The group screen is one level down and carries "?gruppe=<id>" without
// "textbaustein". Whoever submitted its form therefore created a subfield with
// snippet_id NULL — while its group carries a snippet_id. OfSnippet asks
// "WHERE snippet_id = $2" and handed the group out afterwards without a single
// subfield: a group that can draw no row on the snippet's form and looks
// different in the archive than on the screen, because the import path sets
// both.
//
// The subfield therefore inherits its carrier from the stored group.
func TestAGroupOnASnippetCarriesItsSubfields(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	sn, err := snippet.NewStore(database).Create(ctx, ws.ID, "kontakt", "Kontaktblock",
		"Adresse", "<p>Adresse</p>")
	if err != nil {
		t.Fatalf("snippet.Create: %v", err)
	}

	if rec := createField(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Öffnungszeiten"},
		"art":          {field.KindGroup},
		"textbaustein": {strconv.FormatInt(sn.ID, 10)},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("Gruppe anlegen: Status %d, wollte 303", rec.Code)
	}

	defs, err := h.fields.OfSnippet(ctx, ws.ID, sn.ID)
	if err != nil || len(defs) != 1 {
		t.Fatalf("OfSnippet: %v (%d)", err, len(defs))
	}
	group := defs[0]

	// And now the subfield, the way the screen submits it: with "gruppe" and
	// without "textbaustein", because one level down nobody knows any more
	// whom the group hangs on.
	if rec := createField(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Tag"},
		"art":          {field.KindText},
		"gruppe":       {strconv.FormatInt(group.ID, 10)},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("Unterfeld anlegen: Status %d, wollte 303", rec.Code)
	}

	defs, err = h.fields.OfSnippet(ctx, ws.ID, sn.ID)
	if err != nil || len(defs) != 1 {
		t.Fatalf("OfSnippet after the sub-field: %v (%d)", err, len(defs))
	}
	if len(defs[0].Sub) != 1 || defs[0].Sub[0].Key != "tag" {
		t.Fatalf("the group comes back without its sub-field: %+v", defs[0].Sub)
	}
	if defs[0].Sub[0].SnippetID != sn.ID {
		t.Errorf("das Unterfeld trägt snippet_id %d, wollte %d — es erbt seinen "+
			"Träger aus der gespeicherten Gruppe", defs[0].Sub[0].SnippetID, sn.ID)
	}

	// The counter-check on the dangerous cut: the subfield must not show up on
	// the page screen because of this.
	pages, err := h.fields.List(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range pages {
		if d.Key == "tag" || d.Key == "oeffnungszeiten" {
			t.Errorf("a snippet's field is in the page list: %+v", d)
		}
	}
}

func TestTheSnippetModeOpensForOnesOwn(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	sn, err := snippet.NewStore(database).Create(ctx, ws.ID, "kontakt", "Kontaktblock",
		"Adresse", "<p>Adresse</p>")
	if err != nil {
		t.Fatalf("snippet.Create: %v", err)
	}

	// The empty case first: the same screen, an empty list and the sentence
	// that says so.
	rec := fieldScreen(t, h, sm, ws.ID, "textbaustein="+strconv.FormatInt(sn.ID, 10))
	if rec.Code != http.StatusOK {
		t.Fatalf("Status %d, wollte 200", rec.Code)
	}
	leer := rec.Body.String()
	if !strings.Contains(leer, "Kontaktblock") {
		t.Error("the screen does not name the snippet")
	}
	if !strings.Contains(leer, "This snippet has no fields yet") {
		t.Error("the empty case does not show its sentence")
	}
	// A snippet is not "simple": "Applies to" has no meaning on it. "Required"
	// does, and the checkbox has to be there.
	if !strings.Contains(leer, `name="pflicht"`) {
		t.Error("the required box is missing — a field on a snippet may be demanded")
	}
	if strings.Contains(leer, `name="gilt_fuer"`) {
		t.Error(`"gilt für" steht auf dem Textbaustein-Bildschirm, wo es nichts bedeutet`) //nolint:german — the message quotes the German fixture it is about
	}
	// The choice of field kind is the full one: a group belongs to it, unlike
	// on a block kind.
	if !strings.Contains(leer, `value="`+field.KindGroup+`"`) {
		t.Error("the list of field kinds knows no group — it is not field.Kinds")
	}

	// Und jetzt eines anlegen.
	createField(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Telefonnummer"},
		"art":          {field.KindText},
		"textbaustein": {strconv.FormatInt(sn.ID, 10)},
	})

	defs, err := field.NewStore(database).OfSnippet(ctx, ws.ID, sn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 {
		t.Fatalf("%d Felder am Textbaustein, wollte 1", len(defs))
	}
	if defs[0].SnippetID != sn.ID {
		t.Errorf("SnippetID = %d, wollte %d", defs[0].SnippetID, sn.ID)
	}

	rec = fieldScreen(t, h, sm, ws.ID, "textbaustein="+strconv.FormatInt(sn.ID, 10))
	if !strings.Contains(rec.Body.String(), "Telefonnummer") {
		t.Error("the created field is not in its snippet's list")
	}
}

// A snippet of another website does not open the mode — and nothing of that
// website appears. The absence is the assurance: a mode opened silently would
// answer 200 as well.
func TestASnippetOfAForeignWebsiteDoesNotOpenTheMode(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	// A page field of one's own, so that the fallback to the page screen shows
	// something demonstrable.
	createField(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Seitenpreis"},
		"art":          {field.KindNumber},
		"gilt_fuer":    {"beides"},
	})

	_, fremd := zweiteWebsite(t, database, "Zweite Seite", "fremd", "Fremder Kontaktblock")

	rec := fieldScreen(t, h, sm, ws.ID, "textbaustein="+strconv.FormatInt(fremd.ID, 10))
	if rec.Code != http.StatusOK {
		t.Fatalf("Status %d, wollte 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "Fremder Kontaktblock") {
		t.Error("another website's snippet name is on the screen")
	}
	if !strings.Contains(body, "Seitenpreis") {
		t.Error("the fallback to the page's own fields did not happen")
	}
}

// The same attempt as a POST. The status alone proves nothing: a handler that
// first creates and then says 404 would get through on it. That is why the
// store is consulted.
func TestASnippetOfAForeignWebsiteIsRefusedOnSaving(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	fremdeWS, fremd := zweiteWebsite(t, database, "Zweite Seite", "fremd", "Fremder Kontaktblock")

	rec := createField(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Eingeschmuggelt"},
		"art":          {field.KindText},
		"textbaustein": {strconv.FormatInt(fremd.ID, 10)},
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("Status %d, wollte 404", rec.Code)
	}

	fields := field.NewStore(database)
	// Look under both website numbers: the definition would have been created
	// with the number out of the address, and it should be found under neither
	// of the two.
	for _, id := range []int64{ws.ID, fremdeWS.ID} {
		defs, err := fields.OfSnippet(ctx, id, fremd.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(defs) != 0 {
			t.Errorf("website %d carries %d definitions on the foreign snippet, wanted 0", id, len(defs))
		}
	}
}

// The page screen stays clean: with a snippet field in the database, GET
// …/felder without a query parameter lists exactly the page's own fields. That
// is ROADMAP criterion 3 as a check that runs on every commit.
func TestASnippetFieldDoesNotAppearOnThePageScreen(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	sn, err := snippet.NewStore(database).Create(ctx, ws.ID, "kontakt", "Kontaktblock",
		"Adresse", "<p>Adresse</p>")
	if err != nil {
		t.Fatalf("snippet.Create: %v", err)
	}

	createField(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Seitenpreis"},
		"art":          {field.KindNumber},
		"gilt_fuer":    {"beides"},
	})
	createField(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Telefonnummer"},
		"art":          {field.KindText},
		"textbaustein": {strconv.FormatInt(sn.ID, 10)},
	})

	rec := fieldScreen(t, h, sm, ws.ID, "")
	body := rec.Body.String()
	if !strings.Contains(body, "Seitenpreis") {
		t.Error("the page's own field is missing from the page screen")
	}
	if strings.Contains(body, "Telefonnummer") {
		t.Error("a snippet field is on the page screen — the dangerous cut of this phase")
	}
}

// The value half: what the editors type into the fields of a snippet.
//
// The four cases here drive over the real templates from disk and over the real
// storage path, because both ends together carry the promise: a form that mints
// its own names and a parser that expects different ones are green separately
// and silently broken together. That is why every case builds its form keys
// with field.Def.FieldName — the same function the template draws them from —
// and never by hand.
//
// What an overlooked place would look like if only the counting gates ran:
// every grep -c of plans 08-01 through 08-04 reports green, the snippet field
// additionally appears in the page editor under a name nobody chose, somebody
// fills it in, and the value lands in the page's fields column, where no theme
// reads it. Nothing is logged, nothing fails, and the first report is a
// screenshot. The fourth case below is written against exactly that.

// textbausteinSpeichern schickt das Wertformular eines Textbausteins ab.
func saveSnippet(t *testing.T, h *Handler, sm *scs.SessionManager, websiteID int64, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, h, sm, h.HandleSnippetList, postForm(
		"/admin/websites/"+strconv.FormatInt(websiteID, 10)+"/snippets",
		values, map[string]string{"id": strconv.FormatInt(websiteID, 10)}))
}

// snippetScreen calls GET …/snippets with the query it is given.
func snippetScreen(t *testing.T, h *Handler, sm *scs.SessionManager, websiteID int64, query string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/snippets"
	if query != "" {
		target += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	return serve(t, h, sm, h.HandleSnippetList, req)
}

// snippetField creates a field definition on a snippet and hands it back, so
// that the caller can fetch its form key from FieldName instead of typing it.
func snippetField(t *testing.T, database *db.DB, websiteID, snippetID int64, def field.Def) field.Def {
	t.Helper()
	def.WebsiteID = websiteID
	def.SnippetID = snippetID
	angelegt, err := field.NewStore(database).Create(context.Background(), def)
	if err != nil {
		t.Fatalf("Textbausteinfeld %q anlegen: %v", def.Key, err)
	}
	return *angelegt
}

// snippetWithFields creates a snippet and hands it back.
func blockWithFields(t *testing.T, database *db.DB, websiteID int64, key, name string) *snippet.Snippet {
	t.Helper()
	sn, err := snippet.NewStore(database).Create(context.Background(), websiteID, key, name,
		"Wir sind **da**.", "<p>Wir sind <strong>da</strong>.</p>")
	if err != nil {
		t.Fatalf("snippet.Create: %v", err)
	}
	return sn
}

// gespeicherteFelder liest die fields-Spalte eines Textbausteins back.
func storedFields(t *testing.T, database *db.DB, websiteID, id int64) field.Data {
	t.Helper()
	sn, err := snippet.NewStore(database).Get(context.Background(), websiteID, id)
	if err != nil || sn == nil {
		t.Fatalf("read the snippet back: %v", err)
	}
	return field.Decode(sn.Fields)
}

// The round trip: typed, stored, opened again, and the same values stand there
// — in the form and in the column.
func TestASnippetFieldRoundTrip(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	sn := blockWithFields(t, database, ws.ID, "kontakt", "Kontaktblock")
	kurz := snippetField(t, database, ws.ID, sn.ID, field.Def{
		Key: "telefon", Label: "Telefon", Kind: field.KindText})
	lang := snippetField(t, database, ws.ID, sn.ID, field.Def{
		Key: "hinweis", Label: "Hinweis", Kind: field.KindLong})

	rec := saveSnippet(t, h, sm, ws.ID, url.Values{
		"id":               {strconv.FormatInt(sn.ID, 10)},
		"key":              {"kontakt"},
		"name":             {"Kontaktblock"},
		"content_markdown": {"Wir sind **da**."},
		kurz.FieldName():   {"07721 123456"},
		lang.FieldName():   {"Nur vormittags erreichbar."},
	})
	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("status %d, wanted a redirect after saving", rec.Code)
	}

	data := storedFields(t, database, ws.ID, sn.ID)
	if got := data.Values["telefon"]; got != "07721 123456" {
		t.Errorf("telefon = %q, wollte %q", got, "07721 123456")
	}
	if got := data.Values["hinweis"]; got != "Nur vormittags erreichbar." {
		t.Errorf("hinweis = %q, wollte %q", got, "Nur vormittags erreichbar.")
	}

	// And the same in the form, under the same names.
	body := snippetScreen(t, h, sm, ws.ID, "edit="+strconv.FormatInt(sn.ID, 10)).Body.String()
	for _, wollte := range []string{
		`name="` + kurz.FieldName() + `"`,
		`name="` + lang.FieldName() + `"`,
		"07721 123456",
		"Nur vormittags erreichbar.",
	} {
		if !strings.Contains(body, wollte) {
			t.Errorf("the reopened form does not carry %q", wollte)
		}
	}

	// The body and the key came through the pass intact.
	sn2, err := snippet.NewStore(database).Get(context.Background(), ws.ID, sn.ID)
	if err != nil || sn2 == nil {
		t.Fatalf("read back: %v", err)
	}
	if sn2.Key != "kontakt" || sn2.ContentMarkdown != "Wir sind **da**." {
		t.Errorf("key or body has changed: %q / %q", sn2.Key, sn2.ContentMarkdown)
	}
}

// A refused save writes nothing — not even half.
//
// The proof stands in the store and not only in the body of the answer: a
// handler that first writes and then refuses would otherwise get through.
func TestARequiredSnippetFieldIsRefused(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	sn := blockWithFields(t, database, ws.ID, "kontakt", "Kontaktblock")
	required := snippetField(t, database, ws.ID, sn.ID, field.Def{
		Key: "telefon", Label: "Telefon", Kind: field.KindText, Required: true})
	frei := snippetField(t, database, ws.ID, sn.ID, field.Def{
		Key: "hinweis", Label: "Hinweis", Kind: field.KindLong})

	gemeinsam := func(telefon, note string) url.Values {
		return url.Values{
			"id":                 {strconv.FormatInt(sn.ID, 10)},
			"key":                {"kontakt"},
			"name":               {"Kontaktblock"},
			"content_markdown":   {"Wir sind **da**."},
			required.FieldName(): {telefon},
			frei.FieldName():     {note},
		}
	}

	saveSnippet(t, h, sm, ws.ID, gemeinsam("07721 123456", "Erster Hinweis"))
	if got := storedFields(t, database, ws.ID, sn.ID).Values["telefon"]; got != "07721 123456" {
		t.Fatalf("the first pass stored nothing: telefon = %q", got)
	}

	rec := saveSnippet(t, h, sm, ws.ID, gemeinsam("", "Zweiter Hinweis"))
	if rec.Code == http.StatusSeeOther || rec.Code == http.StatusFound {
		t.Fatalf("status %d — the empty required field was accepted", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Zweiter Hinweis") {
		t.Error("the refused form lost the typed value of the other field")
	}
	if !strings.Contains(body, "has to be filled in") {
		t.Error("there is no reason beside the field")
	}

	data := storedFields(t, database, ws.ID, sn.ID)
	if got := data.Values["telefon"]; got != "07721 123456" {
		t.Errorf("telefon = %q — the refusal touched the previous value", got)
	}
	if got := data.Values["hinweis"]; got != "Erster Hinweis" {
		t.Errorf("hinweis = %q — die Ablehnung hat halb geschrieben", got)
	}
}

// TestSnippetFieldSanitising: the same protection on both carriers, and the
// proof is that the two agree.
//
// What protects a langtext field value is **not** goldmark and not bluemonday.
// field.Resolve has no arm of its own for KindLong; the value falls into the
// default: arm and comes out as a plain Go string, and html/template escapes it
// context-sensitively where the theme prints it. Exactly the same happens with
// the same value on a page — which is why "the two agree" is a statement about
// a shared mechanism and not a coincidence.
//
// The chain goldmark → bluemonday belongs to the **body** of the snippet. That
// is a different value on a different path, and it is named here only to draw
// the line.
//
// What this case is not: it does not re-check html/template's escaper, which
// has checks of its own, and it does not re-check goldmark. It shows that the
// snippet's field path reaches the same escaping as the page's field path —
// "one chain, not a second" is a property of the call graph, and that is how it
// is asserted from outside.
func TestSnippetFieldSanitising(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	const boshaft = `<script>alert(1)</script>`

	sn := blockWithFields(t, database, ws.ID, "kontakt", "Kontaktblock")
	onBlock := snippetField(t, database, ws.ID, sn.ID, field.Def{
		Key: "hinweis", Label: "Hinweis", Kind: field.KindLong})

	// The same key, the same field kind, the other carrier.
	onPage, err := field.NewStore(database).Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "hinweis", Label: "Hinweis", Kind: field.KindLong})
	if err != nil {
		t.Fatalf("Seitenfeld anlegen: %v", err)
	}

	saveSnippet(t, h, sm, ws.ID, url.Values{
		"id":                {strconv.FormatInt(sn.ID, 10)},
		"key":               {"kontakt"},
		"name":              {"Kontaktblock"},
		"content_markdown":  {"Wir sind **da**."},
		onBlock.FieldName(): {boshaft},
	})
	serve(t, h, sm, h.HandlePageCreate, postForm("/admin/websites/1/pages/new", url.Values{
		"title":            {"Startseite"},
		"slug":             {"start"},
		"status":           {"published"},
		"kind":             {"page"},
		onPage.FieldName(): {boshaft},
	}, map[string]string{"id": strconv.FormatInt(ws.ID, 10)}))

	p, err := page.NewStore(database).GetPageBySlug(ctx, ws.ID, "start")
	if err != nil || p == nil {
		t.Fatalf("the page was not created: %v", err)
	}

	// Both carriers through the same resolver, with their own definitions.
	onBlockResolved := field.Resolve([]field.Def{onBlock},
		storedFields(t, database, ws.ID, sn.ID), field.Links{})
	onPageResolved := field.Resolve([]field.Def{*onPage},
		field.Decode(p.Fields), field.Links{})

	if onBlockResolved["hinweis"] != onPageResolved["hinweis"] {
		t.Fatalf("snippet and page resolve the same value differently:\n  block: %#v\n  page:  %#v",
			onBlockResolved["hinweis"], onPageResolved["hinweis"])
	}

	// And the way a theme prints them: the same template over both.
	wieEinTheme := template.Must(template.New("theme").Parse(`<p class="hinweis">{{.}}</p>`))
	druck := func(value any) string {
		var aus strings.Builder
		if err := wieEinTheme.Execute(&aus, value); err != nil {
			t.Fatalf("drucken: %v", err)
		}
		return aus.String()
	}
	fromBlock := druck(onBlockResolved["hinweis"])
	fromPage := druck(onPageResolved["hinweis"])

	for name, aus := range map[string]string{"Textbaustein": fromBlock, "Page": fromPage} {
		if strings.Contains(aus, "<script") {
			t.Errorf("%s: a live token survived: %s", name, aus)
		}
	}
	if fromBlock != fromPage {
		t.Errorf("the two carriers print the same value differently:\n  block: %s\n  page:  %s",
			fromBlock, fromPage)
	}

	// The body keeps its own, different chain — untouched by this phase.
	sn2, err := snippet.NewStore(database).Get(ctx, ws.ID, sn.ID)
	if err != nil || sn2 == nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(sn2.ContentHTML, "<strong>da</strong>") {
		t.Errorf("the Markdown body is no longer what the chain made of it: %q", sn2.ContentHTML)
	}
}

// The page form stays clean, seen in the browser.
//
// The storage half of this promise is held by 08-01's TestBausteinNamensraum.
// The other half stands here: what an editor *sees* is a drawn template and not
// a query result, and that is why it is drawn.
func TestASnippetFieldDoesNotStandInThePageForm(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	sn := blockWithFields(t, database, ws.ID, "kontakt", "Kontaktblock")
	onBlock := snippetField(t, database, ws.ID, sn.ID, field.Def{
		Key: "telefonnummer", Label: "Telefonnummer", Kind: field.KindText})

	saveSnippet(t, h, sm, ws.ID, url.Values{
		"id":                {strconv.FormatInt(sn.ID, 10)},
		"key":               {"kontakt"},
		"name":              {"Kontaktblock"},
		"content_markdown":  {"Wir sind **da**."},
		onBlock.FieldName(): {"07721 123456"},
	})

	req := httptest.NewRequest(http.MethodGet,
		"/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/pages/new", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	body := serve(t, h, sm, h.HandlePageCreate, req).Body.String()

	if strings.Contains(body, "Telefonnummer") {
		t.Error("a snippet field's label is in the page editor")
	}
	// The one hand-written field prefix in this file, and it stands inside an
	// assertion about an absence.
	if strings.Contains(body, `name="feld_telefonnummer"`) {
		t.Error("the form name of a snippet field is in the page editor")
	}
}

// codeExpression fetches the first <code>…</code> out of a rendered page and
// turns the entities back into characters.
//
// The screen writes the curly braces as &#123;, or the admin template would try
// to carry out the advice itself. For the check it has to be again what the
// operator copies out.
func codeAusdruck(t *testing.T, koerper string) string {
	t.Helper()
	auf := strings.Index(koerper, "<code>")
	if auf < 0 {
		t.Fatal("no <code> on the screen")
	}
	rest := koerper[auf+len("<code>"):]
	zu := strings.Index(rest, "</code>")
	if zu < 0 {
		t.Fatal("<code> ohne Ende")
	}
	return stdhtml.UnescapeString(rest[:zu])
}

// The advice on the screen has to be an expression that parses.
//
// validKey allows the hyphen expressly (internal/admin/snippet.go), and this
// project's sample key is called "footer-kontakt"
// (internal/template/sample.go). The field suffix form
// {{.Site.SnippetFields.footer-kontakt.telefon}} is no expression to Go but a
// parse error — "bad character U+002D". Whoever copies the advice gets their
// theme refused by template.Check, with a message that names a character and no
// cause.
//
// What is checked is therefore not the wording but the property: what stands
// there goes through the parser. TEMPLATE-SPEC.md uses index throughout; the
// screen was the one place that contradicted the specification.
func TestTheFieldScreensAdviceParses(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	sn, err := snippet.NewStore(database).Create(ctx, ws.ID, "footer-kontakt", "Kontaktblock",
		"Adresse", "<p>Adresse</p>")
	if err != nil {
		t.Fatalf("snippet.Create: %v", err)
	}

	rec := fieldScreen(t, h, sm, ws.ID, "textbaustein="+strconv.FormatInt(sn.ID, 10))
	if rec.Code != http.StatusOK {
		t.Fatalf("Status %d, wollte 200", rec.Code)
	}
	rat := codeAusdruck(t, rec.Body.String())
	if !strings.Contains(rat, "footer-kontakt") {
		t.Fatalf("the advice does not name the key: %q", rat)
	}
	if _, err := template.New("rat").Parse(rat); err != nil {
		t.Errorf("the advice on the screen is not a translatable expression:\n  %s\n  %v",
			rat, err)
	}
}
