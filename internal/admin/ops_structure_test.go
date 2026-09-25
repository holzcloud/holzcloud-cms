package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// The structure tools of the assistant connection, run against this handler as
// their Ops — the way main.go wires them. internal/ai holds who may call what;
// this file holds that each call does what the screen does.

type structureRig struct {
	t       *testing.T
	h       *Handler
	ts      *httptest.Server
	site    int64
	other   int64
	admin   string
	content string
}

func newStructureRig(t *testing.T) *structureRig {
	t.Helper()
	h, _, database, ws := newTestAdmin(t)
	other, err := h.domains.CreateWebsite(context.Background(), "Andere", "")
	if err != nil {
		t.Fatal(err)
	}
	tokens := ai.NewStore(database)
	admin, _, err := tokens.IssueLevel(context.Background(), "verwaltung", 0, ai.LevelAdmin, 0)
	if err != nil {
		t.Fatal(err)
	}
	content, _, err := tokens.IssueLevel(context.Background(), "inhalt", 0, ai.LevelContent, 0)
	if err != nil {
		t.Fatal(err)
	}
	srv := ai.NewServer(tokens, "Test", slog.New(slog.DiscardHandler), ai.Tools(ai.Deps{
		Domains: h.domains, Pages: h.pages, Media: h.mediaStore, Fields: h.fields, Ops: h,
	}))
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return &structureRig{t: t, h: h, ts: ts, site: ws.ID, other: other.ID, admin: admin, content: content}
}

// call runs one tool and hands back its answer and whether it failed.
func (r *structureRig) call(key, name string, args map[string]any) (map[string]any, bool) {
	r.t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args}})
	req, _ := http.NewRequest(http.MethodPost, r.ts.URL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := r.ts.Client().Do(req)
	if err != nil {
		r.t.Fatalf("%s: %v", name, err)
	}
	defer res.Body.Close()
	var envelope struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil || len(envelope.Result.Content) == 0 {
		r.t.Fatalf("%s: unreadable answer (%v)", name, err)
	}
	text := envelope.Result.Content[0].Text
	if envelope.Result.IsError {
		return map[string]any{"text": text}, true
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		r.t.Fatalf("%s: %q is not JSON", name, text)
	}
	return out, false
}

// must runs one tool that has to succeed.
func (r *structureRig) must(key, name string, args map[string]any) map[string]any {
	r.t.Helper()
	out, failed := r.call(key, name, args)
	if failed {
		r.t.Fatalf("%s: %v", name, out["text"])
	}
	return out
}

// refused runs one tool that has to fail, and hands back the reason.
func (r *structureRig) refused(key, name string, args map[string]any) string {
	r.t.Helper()
	out, failed := r.call(key, name, args)
	if !failed {
		r.t.Fatalf("%s was accepted: %v", name, out)
	}
	return out["text"].(string)
}

func (r *structureRig) page(websiteID int64, title, slug string) *page.Page {
	r.t.Helper()
	p, err := r.h.pages.CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID, Title: title, Slug: slug, Status: "published", HTML: "<p>x</p>",
	})
	if err != nil {
		r.t.Fatal(err)
	}
	return p
}

func numID(v any) int64 { return int64(v.(float64)) }

func TestAMenuIsBuiltChangedAndDeletedThroughTheTools(t *testing.T) {
	r := newStructureRig(t)
	about := r.page(r.site, "Über uns", "ueber-uns")

	made := r.must(r.content, "create_menu", map[string]any{
		"website": r.site, "name": "Hauptmenü", "key": "main",
		"items": []any{
			map[string]any{"label": "Über uns", "page": about.ID},
			map[string]any{"label": "Mehr", "children": []any{
				map[string]any{"label": "Beispiel", "url": "https://example.org"},
			}},
		},
	})
	menuID := numID(made["id"])
	items := made["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items = %v", items)
	}
	first := items[0].(map[string]any)
	if first["type"] != "page" || first["page_slug"] != "ueber-uns" {
		t.Errorf("page entry = %v", first)
	}
	child := items[1].(map[string]any)["children"].([]any)[0].(map[string]any)
	if child["type"] != "url" || child["url"] != "https://example.org" {
		t.Errorf("child entry = %v", child)
	}

	// The key rule of the screen holds here too.
	r.refused(r.content, "create_menu", map[string]any{"website": r.site, "name": "X", "key": "Haupt Menü"})
	r.refused(r.content, "update_menu", map[string]any{"website": r.site, "menu": menuID, "key": "Nicht Gültig"})

	// A page of another website is not a target, and the menu stays as it was.
	foreign := r.page(r.other, "Fremd", "fremd")
	r.refused(r.content, "update_menu", map[string]any{"website": r.site, "menu": menuID,
		"items": []any{map[string]any{"label": "Fremd", "page": foreign.ID}}})
	if got := r.must(r.content, "get_menu", map[string]any{"website": r.site, "menu": menuID}); len(got["items"].([]any)) != 2 {
		t.Errorf("a refused change touched the menu: %v", got["items"])
	}

	changed := r.must(r.content, "update_menu", map[string]any{"website": r.site, "menu": menuID,
		"name": "Oben", "items": []any{map[string]any{"label": "Nur eins", "url": "/kontakt"}}})
	if changed["name"] != "Oben" || changed["key"] != "main" || len(changed["items"].([]any)) != 1 {
		t.Errorf("after update = %v", changed)
	}

	// A menu of this website named under another is not found there.
	r.refused(r.content, "get_menu", map[string]any{"website": r.other, "menu": menuID})
	r.refused(r.content, "delete_menu", map[string]any{"website": r.other, "menu": menuID, "confirm": true})

	r.refused(r.content, "delete_menu", map[string]any{"website": r.site, "menu": menuID})
	r.must(r.content, "delete_menu", map[string]any{"website": r.site, "menu": menuID, "confirm": true})
	if list := r.must(r.content, "list_menus", map[string]any{"website": r.site}); len(list["menus"].([]any)) != 0 {
		t.Errorf("the menu is still there: %v", list)
	}
}

func TestTermsAreRenamedAndDeletedThroughTheTools(t *testing.T) {
	r := newStructureRig(t)
	p := r.page(r.site, "Tisch", "tisch")
	if err := r.h.terms.SetForPage(context.Background(), r.site, p.ID, []string{"Möbel"}); err != nil {
		t.Fatal(err)
	}
	list := r.must(r.content, "list_terms", map[string]any{"website": r.site})["terms"].([]any)
	if len(list) != 1 {
		t.Fatalf("terms = %v", list)
	}
	term := list[0].(map[string]any)
	termID := numID(term["id"])

	// Another website's route changes nothing and says so.
	r.refused(r.content, "rename_term", map[string]any{"website": r.other, "term": termID, "name": "Fremd"})

	r.must(r.content, "rename_term", map[string]any{"website": r.site, "term": termID, "name": "Möbelbau"})
	after := r.must(r.content, "list_terms", map[string]any{"website": r.site})["terms"].([]any)[0].(map[string]any)
	if after["name"] != "Möbelbau" || after["slug"] != term["slug"] {
		t.Errorf("after rename = %v, before %v", after, term)
	}

	r.must(r.content, "delete_term", map[string]any{"website": r.site, "term": termID, "confirm": true})
	if list := r.must(r.content, "list_terms", map[string]any{"website": r.site})["terms"].([]any); len(list) != 0 {
		t.Errorf("the term is still there: %v", list)
	}
	if still, _ := r.h.pages.GetPage(context.Background(), p.ID); still == nil {
		t.Error("deleting the term took the page with it")
	}
}

func TestASnippetWithItsOwnFieldsGoesThroughTheTools(t *testing.T) {
	r := newStructureRig(t)
	made := r.must(r.content, "create_snippet", map[string]any{
		"website": r.site, "key": "oeffnungszeiten", "name": "Öffnungszeiten", "markdown": "Mo–Fr",
	})
	snipID := numID(made["id"])
	if made["marker"] != "[[snippet:oeffnungszeiten]]" {
		t.Errorf("marker = %v", made["marker"])
	}
	r.refused(r.content, "create_snippet", map[string]any{"website": r.site, "key": "oeffnungszeiten", "name": "Doppelt"})
	r.refused(r.content, "create_snippet", map[string]any{"website": r.site, "key": "Nicht gültig", "name": "X"})

	// Its own field is an admin's to define, and then an editor's to fill in.
	r.refused(r.content, "create_field", map[string]any{"website": r.site, "snippet": snipID, "label": "Telefon", "kind": "text"})
	r.must(r.admin, "create_field", map[string]any{"website": r.site, "snippet": snipID, "label": "Telefon", "kind": "text", "required": true})

	r.must(r.content, "update_snippet", map[string]any{"website": r.site, "snippet": snipID,
		"fields": map[string]any{"telefon": "0123"}})
	// The markdown changes and the field value stays: what is not given is
	// carried forward.
	r.must(r.content, "update_snippet", map[string]any{"website": r.site, "snippet": snipID, "markdown": "Sa geschlossen"})
	got := r.must(r.content, "get_snippet", map[string]any{"website": r.site, "snippet": snipID})
	if got["markdown"] != "Sa geschlossen" {
		t.Errorf("markdown = %v", got["markdown"])
	}
	if fields, _ := got["fields"].(map[string]any); fields["telefon"] != "0123" {
		t.Errorf("fields = %v", got["fields"])
	}
	if defs := got["field_definitions"].([]any); len(defs) != 1 {
		t.Errorf("field definitions = %v", defs)
	}
	// A required field emptied is refused, as on the screen.
	r.refused(r.content, "update_snippet", map[string]any{"website": r.site, "snippet": snipID,
		"fields": map[string]any{"telefon": ""}})

	r.refused(r.content, "get_snippet", map[string]any{"website": r.other, "snippet": snipID})
	r.must(r.content, "delete_snippet", map[string]any{"website": r.site, "snippet": snipID, "confirm": true})
	if list := r.must(r.content, "list_snippets", map[string]any{"website": r.site}); len(list["snippets"].([]any)) != 0 {
		t.Errorf("the snippet is still there: %v", list)
	}
}

func TestRedirectsAndTheLinkCheckThroughTheTools(t *testing.T) {
	r := newStructureRig(t)
	made := r.must(r.content, "create_redirect", map[string]any{
		"website": r.site, "from": "https://alt.example/kontakt.html", "to": "/kontakt",
	})
	if made["from"] != "/kontakt.html" || made["code"] != float64(301) {
		t.Errorf("created = %v", made)
	}
	r.refused(r.content, "create_redirect", map[string]any{"website": r.site, "from": "/a", "to": "/a"})

	list := r.must(r.content, "list_redirects", map[string]any{"website": r.site})["redirects"].([]any)
	if len(list) != 1 {
		t.Fatalf("redirects = %v", list)
	}
	redirectID := numID(list[0].(map[string]any)["id"])
	r.refused(r.content, "delete_redirect", map[string]any{"website": r.other, "redirect": redirectID, "confirm": true})
	r.must(r.content, "delete_redirect", map[string]any{"website": r.site, "redirect": redirectID, "confirm": true})

	_, err := r.h.pages.CreatePage(context.Background(), page.PageCreate{
		WebsiteID: r.site, Title: "Mit Loch", Slug: "mit-loch", Status: "published",
		HTML: `<p><a href="/gibt-es-nicht">weg</a></p>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	broken := r.must(r.content, "check_links", map[string]any{"website": r.site})["broken_links"].([]any)
	if len(broken) != 1 || broken[0].(map[string]any)["target"] != "/gibt-es-nicht" {
		t.Errorf("broken links = %v", broken)
	}
}

func TestContentKindsThroughTheTools(t *testing.T) {
	r := newStructureRig(t)
	made := r.must(r.admin, "create_kind", map[string]any{
		"website": r.site, "name": "Produkt", "plural": "Produkte", "archive": "produkte", "sort": "title",
	})
	if made["key"] != "produkt" || made["archive"] != "/produkte" || made["sort"] != "title" {
		t.Errorf("created = %v", made)
	}
	kindID := numID(made["id"])

	// Without an overview there is none — not one at /untitled.
	second := r.must(r.admin, "create_kind", map[string]any{"website": r.site, "name": "Termin", "plural": "Termine"})
	if _, has := second["archive"]; has {
		t.Errorf("a kind without an overview got one: %v", second)
	}

	// The page check of the screen: an overview may not shadow a page.
	r.page(r.site, "Veranstaltungen", "veranstaltungen")
	r.refused(r.admin, "update_kind", map[string]any{"website": r.site, "kind": kindID, "archive": "veranstaltungen"})

	changed := r.must(r.admin, "update_kind", map[string]any{"website": r.site, "kind": kindID, "plural": "Waren"})
	if changed["plural"] != "Waren" || changed["name"] != "Produkt" || changed["archive"] != "/produkte" {
		t.Errorf("after update = %v", changed)
	}
	r.must(r.admin, "move_kind", map[string]any{"website": r.site, "kind": numID(second["id"]), "direction": "up"})
	kinds := r.must(r.admin, "list_kinds", map[string]any{"website": r.site})["kinds"].([]any)
	if kinds[0].(map[string]any)["key"] != "termin" {
		t.Errorf("order after moving = %v", kinds)
	}

	r.refused(r.admin, "delete_kind", map[string]any{"website": r.other, "kind": kindID, "confirm": true})
	r.must(r.admin, "delete_kind", map[string]any{"website": r.site, "kind": kindID, "confirm": true})
	if kinds := r.must(r.admin, "list_kinds", map[string]any{"website": r.site})["kinds"].([]any); len(kinds) != 1 {
		t.Errorf("kinds after delete = %v", kinds)
	}
}

func TestFieldsAndGroupsThroughTheTools(t *testing.T) {
	r := newStructureRig(t)
	price := r.must(r.admin, "create_field", map[string]any{"website": r.site, "label": "Preis", "kind": "zahl"})
	group := r.must(r.admin, "create_field", map[string]any{"website": r.site, "label": "Zeiten", "kind": "gruppe"})
	sub := r.must(r.admin, "create_field", map[string]any{"website": r.site, "group": numID(group["id"]),
		"label": "Tag", "kind": "auswahl", "choices": []any{"Mo", "Di"}})
	if sub["key"] != "tag" {
		t.Errorf("subfield = %v", sub)
	}
	// No group inside a group, as on the screen.
	r.refused(r.admin, "create_field", map[string]any{"website": r.site, "group": numID(group["id"]), "label": "Innen", "kind": "gruppe"})
	// A group of another website is not a carrier.
	r.refused(r.admin, "create_field", map[string]any{"website": r.other, "group": numID(group["id"]), "label": "X", "kind": "text"})

	changed := r.must(r.admin, "update_field", map[string]any{"website": r.site, "field": numID(price["id"]),
		"label": "Preis in Euro", "required": true})
	if changed["label"] != "Preis in Euro" || changed["key"] != "preis" || changed["required"] != true || changed["kind"] != "zahl" {
		t.Errorf("after update = %v", changed)
	}
	r.refused(r.admin, "update_field", map[string]any{"website": r.site, "field": numID(group["id"]), "kind": "text"})

	r.must(r.admin, "move_field", map[string]any{"website": r.site, "field": numID(group["id"]), "direction": "up"})
	fields := r.must(r.content, "list_fields", map[string]any{"website": r.site})["fields"].([]any)
	if fields[0].(map[string]any)["key"] != "zeiten" {
		t.Errorf("order after moving = %v", fields)
	}
	if fields[0].(map[string]any)["id"] == nil {
		t.Error("list_fields does not report the id the writing tools need")
	}

	r.refused(r.admin, "delete_field", map[string]any{"website": r.other, "field": numID(price["id"]), "confirm": true})
	r.must(r.admin, "delete_field", map[string]any{"website": r.site, "field": numID(price["id"]), "confirm": true})
	if fields := r.must(r.content, "list_fields", map[string]any{"website": r.site})["fields"].([]any); len(fields) != 1 {
		t.Errorf("fields after delete = %v", fields)
	}
}

func TestBlockKindsThroughTheTools(t *testing.T) {
	r := newStructureRig(t)
	made := r.must(r.admin, "create_block_kind", map[string]any{"website": r.site, "name": "Rezeptschritt", "hint": "Ein Schritt"})
	typeID := numID(made["id"])
	r.must(r.admin, "create_field", map[string]any{"website": r.site, "block_kind": typeID, "label": "Dauer", "kind": "text"})
	// A reference cannot live in a block, as the screen says.
	if reason := r.refused(r.admin, "create_field", map[string]any{"website": r.site, "block_kind": typeID, "label": "Seite", "kind": "verweis"}); reason == "" {
		t.Error("no reason given")
	}
	r.refused(r.admin, "create_block_kind", map[string]any{"website": r.site, "name": "Text"})

	r.must(r.admin, "update_block_kind", map[string]any{"website": r.site, "block_kind": typeID, "name": "Schritt"})
	list := r.must(r.admin, "list_block_kinds", map[string]any{"website": r.site})
	own := list["own"].([]any)
	if len(own) != 1 {
		t.Fatalf("own = %v", own)
	}
	entry := own[0].(map[string]any)
	if entry["name"] != "Schritt" || entry["hint"] != "Ein Schritt" || len(entry["fields"].([]any)) != 1 {
		t.Errorf("block kind = %v", entry)
	}
	if len(list["built_in"].([]any)) == 0 {
		t.Error("the built-in kinds are missing")
	}

	r.refused(r.admin, "delete_block_kind", map[string]any{"website": r.other, "block_kind": typeID, "confirm": true})
	r.must(r.admin, "delete_block_kind", map[string]any{"website": r.site, "block_kind": typeID, "confirm": true})
	if own := r.must(r.admin, "list_block_kinds", map[string]any{"website": r.site})["own"].([]any); len(own) != 0 {
		t.Errorf("own after delete = %v", own)
	}
}
