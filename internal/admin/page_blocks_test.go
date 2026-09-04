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
func TestBausteineWerdenGespeichertUndAusgegeben(t *testing.T) {
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
		t.Fatalf("die Seite wurde nicht angelegt: %v", err)
	}

	// Die Bausteine liegen als solche in der Datenbank.
	blocks, err := block.Decode(p.Blocks, block.Builtin)
	if err != nil || len(blocks) != 2 {
		t.Fatalf("Bausteine = %+v, %v", blocks, err)
	}

	// Das ausgegebene HTML trägt den Rahmen und den Text.
	for _, wollte := range []string{
		`class="hc-block hc-text"`, "Willkommen", `class="hc-block hc-zitat"`, "Eine Kundin",
	} {
		if !strings.Contains(p.ContentHTML, wollte) {
			t.Errorf("%q fehlt im HTML:\n%s", wollte, p.ContentHTML)
		}
	}

	// Und der reine Text ist da, sonst wäre die Seite für die eigene Suche
	// unsichtbar.
	for _, wollte := range []string{"Willkommen", "Schöne Tiere", "Eine Kundin"} {
		if !strings.Contains(p.ContentMarkdown, wollte) {
			t.Errorf("%q fehlt im reinen Text:\n%s", wollte, p.ContentMarkdown)
		}
	}
}

// Ein Knopf im Editor ist kein Speichern. Er darf die Seite nicht anlegen und
// muss zurückgeben, was schon getippt wurde.
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
		t.Errorf("der getippte Text kam nicht zurück:\n%s", body)
	}
	// Und der neue Baustein steht jetzt drin.
	if !strings.Contains(body, `name="b1.typ" value="bild"`) {
		t.Errorf("der neue Baustein fehlt:\n%s", body)
	}
}

// Mit htmx kommt nur die Liste zurück, ohne htmx das ganze Formular. Beides
// muss den Zustand tragen, sonst verliert einer der beiden Wege den Text.
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
		t.Errorf("die Liste ist unvollständig:\n%s", teil)
	}

	req = postForm("/admin/websites/1/pages/new", values,
		map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	ganz := serve(t, h, sm, h.HandlePageCreate, req).Body.String()
	if !strings.Contains(ganz, `name="title"`) {
		t.Errorf("ohne htmx fehlt das Formular drumherum:\n%s", ganz)
	}
	if !strings.Contains(ganz, "Bleibt stehen") {
		t.Errorf("ohne htmx ging der Text verloren:\n%s", ganz)
	}
}

// Der Weg in die Bausteine macht aus dem Text einen Baustein, ohne ihn zu
// zerlegen — und ohne dabei zu speichern.
func TestWechselInDieBausteine(t *testing.T) {
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
		t.Errorf("aus dem Text wurde kein Baustein:\n%s", body)
	}
	if !strings.Contains(body, "Ein langer Artikel.") {
		t.Errorf("der Text ging beim Wechsel verloren:\n%s", body)
	}

	// Gespeichert wurde nichts: die Seite ist unverändert Markdown.
	nach, _ := page.NewStore(database).GetPage(context.Background(), p.ID)
	if nach.Blocks != "" {
		t.Errorf("der Wechsel hat gespeichert: %q", nach.Blocks)
	}
}

// Ein Bild aus der Mediathek einer anderen Website darf nicht durch eine
// getippte Nummer erreichbar sein.
func TestBildEinerFremdenWebsiteWirdNichtAusgegeben(t *testing.T) {
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
		t.Errorf("das Bild der anderen Website wurde ausgegeben:\n%s", html)
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

// Eine Seite aus Bausteinen behält sie über das Speichern und erneute Öffnen
// hinweg — der häufigste Weg, auf dem so etwas Inhalt verliert.
func TestBausteineUeberlebenDasErneuteBearbeiten(t *testing.T) {
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
		t.Fatal("die Seite wurde nicht angelegt")
	}

	get := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/edit", nil)
	get.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	get.SetPathValue("pageID", strconv.FormatInt(p.ID, 10))
	body := serve(t, h, sm, h.HandlePageEdit, get).Body.String()

	for _, wollte := range []string{`value="karten"`, "Rohwolle", "Gekardet"} {
		if !strings.Contains(body, wollte) {
			t.Errorf("%q fehlt im wieder geöffneten Editor:\n%s", wollte, body)
		}
	}
	// Und der einfache Editor wird nicht angeboten, weil dabei die Karten
	// verloren gingen.
	if strings.Contains(body, "zu-markdown") {
		t.Error("der Weg zurück wird angeboten, obwohl die Karten verloren gingen")
	}
}

// Eine eigene Bausteinart überlebt das Speichern einer bestehenden Seite.
//
// Sie tat es lange nicht: handlePageCreatePost setzte values.BlockSet,
// handlePageEditPost nicht — und ein leeres Set kennt nur die neun eingebauten
// Arten. Clean verwarf daraufhin beim Speichern jeden Baustein einer eigenen
// Art, ohne Meldung, und Apply legte keinen neuen an. Eine Seite mit sechs
// Merkmalen kam als Fliesstext zurück, und der Editor bot die Art nicht mehr
// an, mit der man sie hätte wiederherstellen können.
func TestEigeneBausteinartUeberlebtDasBearbeiten(t *testing.T) {
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
		t.Fatalf("die Seite wurde nicht angelegt: %v", err)
	}

	// Bearbeiten, ohne irgendetwas zu ändern.
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
		t.Fatalf("die Seite ist nach dem Bearbeiten weg: %v", err)
	}
	set := block.Set{Own: []block.Own{{ID: art.ID, Key: art.Key, Name: art.Name,
		Fields: []field.Def{{Key: "begriff"}}}}}
	blocks, err := block.Decode(p.Blocks, set)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("nach dem Bearbeiten %d Bausteine, wollte 2: %+v", len(blocks), blocks)
	}
	if blocks[1].Type != art.Key {
		t.Errorf("Baustein 1 ist %q, wollte %q", blocks[1].Type, art.Key)
	}
	if got := blocks[1].Fields["begriff"]; got != "Eine Installation" {
		t.Errorf("Feldwert %q, wollte %q", got, "Eine Installation")
	}
}

// Und die Art lässt sich beim Bearbeiten auch hinzufügen.
func TestEigeneBausteinartLaesstSichBeimBearbeitenAnlegen(t *testing.T) {
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

	// Zwei Bausteine im zurückgegebenen Teil, nicht einer.
	if got := strings.Count(rec.Body.String(), `name="b1.typ"`); got != 1 {
		t.Errorf("der neue Baustein fehlt im Formular (b1.typ %dmal):\n%s", got, rec.Body.String())
	}
}
