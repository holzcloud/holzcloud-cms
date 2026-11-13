package admin

import (
	"context"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/page"
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
