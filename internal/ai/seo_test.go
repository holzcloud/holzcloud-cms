package ai

import (
	"strings"
	"testing"
)

// seo_report is the admin's SEO report as JSON. What is wrong with a page is
// decided in internal/seo and shown the same way by the editor tab; these tests
// are about the tool: that it answers, that it stays with its key, and that it
// changes nothing.

func TestSEOReportListsFindingsOfABadPage(t *testing.T) {
	ts, _, _, first, _, k := siteSetUp(t)
	made, failed := callTool(t, ts, k.content, "create_page", map[string]any{
		"website": first, "title": "Hi", "markdown": "kurz",
	})
	if failed {
		t.Fatalf("create_page: %v", made["text"])
	}

	// A read-only key may ask: the tool changes nothing.
	got, failed := callTool(t, ts, k.read, "seo_report", map[string]any{"website": first})
	if failed {
		t.Fatalf("seo_report: %v", got["text"])
	}
	if got["website"].(float64) != float64(first) || got["pages_checked"].(float64) != 1 {
		t.Errorf("answer %v", got)
	}
	pages := got["pages"].([]any)
	if len(pages) != 1 {
		t.Fatalf("pages %v", pages)
	}
	row := pages[0].(map[string]any)
	if row["title"] != "Hi" || row["worst"] != "warn" {
		t.Errorf("row %v", row)
	}
	codes := map[string]bool{}
	for _, f := range row["findings"].([]any) {
		codes[f.(map[string]any)["code"].(string)] = true
	}
	for _, want := range []string{"title-short", "description-missing", "thin-text", "no-internal-links"} {
		if !codes[want] {
			t.Errorf("no %s in %v", want, codes)
		}
	}
	sum := got["summary"].(map[string]any)
	if sum["warn"].(float64) < 2 || sum["info"].(float64) < 1 || sum["error"].(float64) != 0 {
		t.Errorf("summary %v", sum)
	}
}

func TestSEOReportOnePageAndItsWebsiteFollowsFromIt(t *testing.T) {
	ts, _, _, first, second, k := siteSetUp(t)
	a, _ := callTool(t, ts, k.content, "create_page", map[string]any{"website": first, "title": "Eins", "markdown": "x"})
	b, _ := callTool(t, ts, k.content, "create_page", map[string]any{"website": first, "title": "Zwei", "markdown": "x"})
	idA := int64(a["id"].(float64))
	if idA == 0 || b["id"] == nil {
		t.Fatalf("created %v %v", a, b)
	}

	got, failed := callTool(t, ts, k.read, "seo_report", map[string]any{"page": idA})
	if failed {
		t.Fatalf("seo_report: %v", got["text"])
	}
	pages := got["pages"].([]any)
	if len(pages) != 1 || pages[0].(map[string]any)["id"].(float64) != float64(idA) {
		t.Errorf("one-page mode answered %v", pages)
	}
	if got["pages_checked"].(float64) != 1 {
		t.Errorf("pages_checked %v", got["pages_checked"])
	}

	if _, failed := callTool(t, ts, k.read, "seo_report", map[string]any{"page": idA, "website": second}); !failed {
		t.Error("a page was reported under another website")
	}
	if got, failed := callTool(t, ts, k.read, "seo_report", map[string]any{}); !failed || !strings.Contains(got["text"].(string), "website") {
		t.Errorf("no argument: %v %v", got, failed)
	}
}

func TestSEOReportStaysWithItsKey(t *testing.T) {
	ts, _, _, first, second, k := siteSetUp(t)
	other, failed := callTool(t, ts, k.content, "create_page", map[string]any{"website": second, "title": "Zwei", "markdown": "x"})
	if failed {
		t.Fatalf("create_page: %v", other["text"])
	}
	if _, failed := callTool(t, ts, k.onlyFirst, "seo_report", map[string]any{"website": second}); !failed {
		t.Error("a key for the first website reported on the second")
	}
	if _, failed := callTool(t, ts, k.onlyFirst, "seo_report", map[string]any{"page": int64(other["id"].(float64))}); !failed {
		t.Error("a key for the first website reported on a page of the second")
	}
	if _, failed := callTool(t, ts, k.onlyFirst, "seo_report", map[string]any{"website": first}); failed {
		t.Error("a key was refused its own website")
	}
}

func TestSEOReportIsOfferedAsAReadTool(t *testing.T) {
	ts, _, _, _, _, k := siteSetUp(t)
	res := call(t, ts, k.read, "tools/list", nil)
	for _, raw := range res["result"].(map[string]any)["tools"].([]any) {
		tool := raw.(map[string]any)
		if tool["name"] == "seo_report" {
			return
		}
	}
	t.Error("a read-only key is not offered seo_report")
}
