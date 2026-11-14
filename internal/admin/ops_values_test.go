package admin

import (
	"context"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// The value tools of the assistant connection, run against this handler as
// their Ops. Helpers carry the mcpValue prefix: other plans add files to this
// package at the same time.

// mcpValuePage creates a page whose Markdown source names what it is given.
func mcpValuePage(r *structureRig, websiteID int64, title, slug, status, locale, markdown string) *page.Page {
	r.t.Helper()
	p, err := r.h.pages.CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID, Title: title, Slug: slug, Status: status, Locale: locale,
		Markdown: markdown, HTML: "<p>" + markdown + "</p>",
	})
	if err != nil {
		r.t.Fatal(err)
	}
	return p
}

// mcpValueEntry finds the entry with key in a list the tools answered.
func mcpValueEntry(t *testing.T, list any, key string) map[string]any {
	t.Helper()
	items, _ := list.([]any)
	for _, it := range items {
		e := it.(map[string]any)
		if e["key"] == key {
			return e
		}
	}
	t.Fatalf("no entry %q in %v", key, list)
	return nil
}

func TestAnAssistantSetsAValueAndReadsItBack(t *testing.T) {
	r := newStructureRig(t)
	p := mcpValuePage(r, r.site, "Herunterladen", "herunterladen", "published", "",
		"Die aktuelle Version ist [[wert:version-holzcloud-cms]].")

	set := r.must(r.content, "set_value", map[string]any{
		"website": r.site, "key": "version-holzcloud-cms", "value": "0.1.5",
	})
	if set["created"] != true {
		t.Fatalf("created = %v, want true", set["created"])
	}
	if set["marker"] != "[[wert:version-holzcloud-cms]]" {
		t.Fatalf("marker = %v", set["marker"])
	}
	if set["key"] != "version-holzcloud-cms" || set["value"] != "0.1.5" {
		t.Fatalf("answer = %v", set)
	}

	list := r.must(r.content, "list_values", map[string]any{"website": r.site})
	e := mcpValueEntry(t, list["values"], "version-holzcloud-cms")
	if e["value"] != "0.1.5" || e["marker"] != "[[wert:version-holzcloud-cms]]" {
		t.Fatalf("entry = %v", e)
	}
	used, _ := e["used_on"].([]any)
	if len(used) != 1 {
		t.Fatalf("used_on = %v", e["used_on"])
	}
	u := used[0].(map[string]any)
	if numID(u["id"]) != p.ID || u["title"] != "Herunterladen" || u["slug"] != "herunterladen" ||
		u["status"] != "published" {
		t.Fatalf("used_on[0] = %v", u)
	}
	if _, ok := u["language"]; ok {
		t.Fatalf("a page in the main language carries a language: %v", u)
	}
	if e["updated"] == "" || e["updated"] == nil {
		t.Fatalf("no updated in %v", e)
	}

	again := r.must(r.content, "set_value", map[string]any{
		"website": r.site, "key": "version-holzcloud-cms", "value": "0.1.6",
	})
	if again["created"] != false {
		t.Fatalf("created = %v on a change, want false", again["created"])
	}
	list = r.must(r.content, "list_values", map[string]any{"website": r.site})
	if got := mcpValueEntry(t, list["values"], "version-holzcloud-cms")["value"]; got != "0.1.6" {
		t.Fatalf("value after the change = %v", got)
	}
	texts, _, err := r.h.valueStore.Texts(context.Background(), r.site)
	if err != nil {
		t.Fatal(err)
	}
	if texts["version-holzcloud-cms"] != "0.1.6" {
		t.Fatalf("store holds %q", texts["version-holzcloud-cms"])
	}
}

// mcpValueKey issues a key of the given level, for one website or (0) for all.
func mcpValueKey(r *structureRig, websiteID int64, level ai.Level) string {
	r.t.Helper()
	secret, _, err := ai.NewStore(r.h.db).IssueLevel(context.Background(), "werte", websiteID, level, 0)
	if err != nil {
		r.t.Fatal(err)
	}
	return secret
}

// mcpValueTexts reads the texts a website holds straight from the store.
func mcpValueTexts(r *structureRig, websiteID int64) map[string]string {
	r.t.Helper()
	texts, _, err := r.h.valueStore.Texts(context.Background(), websiteID)
	if err != nil {
		r.t.Fatal(err)
	}
	return texts
}

func TestAReadOnlyKeyReadsValuesButCannotWriteThem(t *testing.T) {
	r := newStructureRig(t)
	r.must(r.content, "set_value", map[string]any{"website": r.site, "key": "preis", "value": "12 Franken"})
	read := mcpValueKey(r, 0, ai.LevelRead)

	list := r.must(read, "list_values", map[string]any{"website": r.site})
	mcpValueEntry(t, list["values"], "preis")
	got := r.must(read, "get_value", map[string]any{"website": r.site, "key": "preis"})
	if got["value"] != "12 Franken" || got["marker"] != "[[wert:preis]]" {
		t.Fatalf("get_value = %v", got)
	}

	for name, args := range map[string]map[string]any{
		"set_value":    {"website": r.site, "key": "preis", "value": "13 Franken"},
		"delete_value": {"website": r.site, "key": "preis", "confirm": true},
	} {
		if reason := r.refused(read, name, args); reason != ai.ErrReadOnly.Error() {
			t.Errorf("%s: refused for the wrong reason: %s", name, reason)
		}
	}
	if texts := mcpValueTexts(r, r.site); len(texts) != 1 || texts["preis"] != "12 Franken" {
		t.Fatalf("a read-only key changed the store: %v", texts)
	}
}

func TestValueToolsStayOnTheirWebsite(t *testing.T) {
	r := newStructureRig(t)
	r.must(r.content, "set_value", map[string]any{"website": r.other, "key": "fremd", "value": "B-Text"})
	r.must(r.content, "set_value", map[string]any{"website": r.site, "key": "eigen", "value": "A-Text"})

	limited := mcpValueKey(r, r.site, ai.LevelContent)
	for name, args := range map[string]map[string]any{
		"list_values":  {"website": r.other},
		"get_value":    {"website": r.other, "key": "fremd"},
		"set_value":    {"website": r.other, "key": "fremd", "value": "overwritten"},
		"delete_value": {"website": r.other, "key": "fremd", "confirm": true},
	} {
		r.refused(limited, name, args)
	}
	if texts := mcpValueTexts(r, r.other); texts["fremd"] != "B-Text" {
		t.Fatalf("a key limited to one website reached another: %v", texts)
	}
	// On its own website the limited key works.
	r.must(limited, "get_value", map[string]any{"website": r.site, "key": "eigen"})

	// An all-websites key sees each website's values only on that website.
	list := r.must(r.content, "list_values", map[string]any{"website": r.site})
	for _, it := range list["values"].([]any) {
		e := it.(map[string]any)
		if e["key"] == "fremd" || e["value"] == "B-Text" {
			t.Fatalf("list_values of one website shows another's value: %v", e)
		}
	}
	reason := r.refused(r.content, "get_value", map[string]any{"website": r.site, "key": "fremd"})
	if !strings.Contains(reason, "there is no value with the key") {
		t.Fatalf("get_value of another website's key: %s", reason)
	}
}

func TestDeleteValueNeedsConfirmAndRefusesAValueInUse(t *testing.T) {
	r := newStructureRig(t)
	r.must(r.content, "set_value", map[string]any{"website": r.site, "key": "telefon", "value": "044 000 00 00"})
	r.must(r.content, "set_value", map[string]any{"website": r.site, "key": "alt", "value": "weg damit"})
	mcpValuePage(r, r.site, "Contact", "contact", "draft", "fr", "Appelez le [[wert:telefon]].")

	reason := r.refused(r.content, "delete_value", map[string]any{"website": r.site, "key": "alt"})
	if !strings.Contains(reason, "confirm: true") {
		t.Fatalf("delete without confirm: %s", reason)
	}
	reason = r.refused(r.content, "delete_value", map[string]any{"website": r.site, "key": "telefon", "confirm": true})
	if !strings.Contains(reason, "Contact (fr)") {
		t.Fatalf("the refusal does not name the page: %s", reason)
	}
	if mcpValueTexts(r, r.site)["telefon"] == "" {
		t.Fatal("a value in use was deleted")
	}

	gone := r.must(r.content, "delete_value", map[string]any{"website": r.site, "key": "alt", "confirm": true})
	if gone["deleted"] != "alt" {
		t.Fatalf("delete answer = %v", gone)
	}
	list := r.must(r.content, "list_values", map[string]any{"website": r.site})
	for _, it := range list["values"].([]any) {
		if it.(map[string]any)["key"] == "alt" {
			t.Fatal("a deleted value is still listed")
		}
	}
	reason = r.refused(r.content, "delete_value", map[string]any{"website": r.site, "key": "alt", "confirm": true})
	if !strings.Contains(reason, "there is no value with the key") {
		t.Fatalf("deleting a missing value: %s", reason)
	}
}

func TestSetValueRefusesWhatTheScreenRefuses(t *testing.T) {
	r := newStructureRig(t)
	for name, args := range map[string]map[string]any{
		"a key with a capital":  {"key": "Version", "value": "1"},
		"an empty text":         {"key": "leer", "value": ""},
		"a text of 401 bytes":   {"key": "lang", "value": strings.Repeat("a", 401)},
		"a line break":          {"key": "zeile", "value": "a\nb"},
		"a placeholder opening": {"key": "klammer", "value": "x [[y"},
	} {
		args["website"] = r.site
		reason := r.refused(r.content, "set_value", args)
		if reason == "" || strings.HasPrefix(reason, "value ") {
			t.Errorf("%s: refused without the connection's reason: %q", name, reason)
		}
		t.Logf("%s: %s", name, reason)
	}
	if texts := mcpValueTexts(r, r.site); len(texts) != 0 {
		t.Fatalf("a refused value was stored: %v", texts)
	}
}

func TestListValuesReportsUndefinedAndOutside(t *testing.T) {
	r := newStructureRig(t)
	ctx := context.Background()
	p := mcpValuePage(r, r.site, "Preise", "preise", "published", "", "Ab [[wert:fehlt]] im Monat.")
	titled := mcpValuePage(r, r.site, "Neu in [[wert:version]]", "neu", "published", "", "Text")
	m, err := r.h.menuStore.CreateMenu(ctx, r.site, "Haupt", "main", "")
	if err != nil {
		t.Fatal(err)
	}
	item, err := r.h.menuStore.CreateItem(ctx, m.ID, nil, "Version [[wert:version]]", "url", "https://example.org", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	list := r.must(r.content, "list_values", map[string]any{"website": r.site})
	undef := mcpValueEntry(t, list["undefined"], "fehlt")
	if undef["marker"] != "[[wert:fehlt]]" {
		t.Fatalf("undefined entry = %v", undef)
	}
	used, _ := undef["used_on"].([]any)
	if len(used) != 1 || numID(used[0].(map[string]any)["id"]) != p.ID {
		t.Fatalf("undefined used_on = %v", undef["used_on"])
	}

	var title, menuEntry bool
	outside, _ := list["outside_text"].([]any)
	for _, it := range outside {
		e := it.(map[string]any)
		if e["key"] != "version" {
			continue
		}
		switch e["where"] {
		case "title":
			title = numID(e["id"]) == titled.ID
		case "menu":
			menuEntry = numID(e["id"]) == item.ID
		}
	}
	if !title || !menuEntry {
		t.Fatalf("outside_text = %v (title %v, menu %v)", list["outside_text"], title, menuEntry)
	}
}

// A product's description resolves values in the shop (WR-03 of the phase 33
// review), so the assistant is told about a product that names a key, the way
// it is told about a page or a snippet.
func TestListValuesNamesTheProducts(t *testing.T) {
	r := newStructureRig(t)
	ctx := context.Background()
	md := "Fragen? [[wert:telefon]] [[wert:fehlt]]"
	html, err := page.RenderMarkdown(md)
	if err != nil {
		t.Fatal(err)
	}
	id, err := shop.NewStore(r.h.valueStore.DB).Create(ctx, &shop.Product{
		WebsiteID: r.site, Slug: "tisch", Title: "Tisch", Status: "draft",
		DescriptionMarkdown: md, DescriptionHTML: html, TaxRate: money.RateStandard,
	})
	if err != nil {
		t.Fatal(err)
	}
	r.must(r.content, "set_value", map[string]any{"website": r.site, "key": "telefon", "value": "044 123 45 67"})

	list := r.must(r.content, "list_values", map[string]any{"website": r.site})
	for _, e := range []map[string]any{
		mcpValueEntry(t, list["values"], "telefon"),
		mcpValueEntry(t, list["undefined"], "fehlt"),
	} {
		products, _ := e["products"].([]any)
		if len(products) != 1 {
			t.Fatalf("%v: products = %v; want the one product", e["key"], e["products"])
		}
		p := products[0].(map[string]any)
		if numID(p["id"]) != id || p["title"] != "Tisch" || p["slug"] != "tisch" || p["status"] != "draft" {
			t.Errorf("%v: product = %v", e["key"], p)
		}
		if used, _ := e["used_on"].([]any); len(used) != 0 {
			t.Errorf("%v: the product is listed as a page: %v", e["key"], used)
		}
	}
	got := r.must(r.content, "get_value", map[string]any{"website": r.site, "key": "telefon"})
	if products, _ := got["products"].([]any); len(products) != 1 {
		t.Errorf("get_value products = %v; want the one product", got["products"])
	}
}

// A product's title prints as written (WR-01 of the second phase 33 review),
// so outside_text names it, with the product's id, the way it names a page
// title.
func TestListValuesNamesAMarkerInAProductTitle(t *testing.T) {
	r := newStructureRig(t)
	id, err := shop.NewStore(r.h.valueStore.DB).Create(context.Background(), &shop.Product{
		WebsiteID: r.site, Slug: "tisch", Title: "Tisch [[wert:modell]]", Subtitle: "Ab [[wert:preis]]",
		Status: "draft", TaxRate: money.RateStandard,
	})
	if err != nil {
		t.Fatal(err)
	}
	list := r.must(r.content, "list_values", map[string]any{"website": r.site})
	got := map[string]bool{}
	outside, _ := list["outside_text"].([]any)
	for _, it := range outside {
		e := it.(map[string]any)
		if numID(e["id"]) == id && e["title"] == "Tisch [[wert:modell]]" {
			got[e["key"].(string)+" "+e["where"].(string)] = true
		}
	}
	if !got["modell product-title"] || !got["preis product-subtitle"] {
		t.Fatalf("outside_text = %v; want the product's title and subtitle", list["outside_text"])
	}
}

func TestGetValueOfAnUnknownKey(t *testing.T) {
	r := newStructureRig(t)
	reason := r.refused(r.content, "get_value", map[string]any{"website": r.site, "key": "nirgends"})
	if !strings.Contains(reason, "there is no value with the key") {
		t.Fatalf("get_value of an unknown key: %s", reason)
	}
}
