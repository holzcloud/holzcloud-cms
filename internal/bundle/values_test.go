package bundle

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/value"
)

// newValueStores is newStores with the value store set, on the same database.
func newValueStores(t *testing.T) Stores {
	t.Helper()
	s := newStores(t)
	s.Values = value.NewStore(s.Domains.DB)
	return s
}

// A page's sentence names a value by key. A bundle that left the values behind
// would arrive with every such sentence missing its word, and nothing on the
// new website would say why.
func TestRoundTripKeepsTheValues(t *testing.T) {
	s := newValueStores(t)
	ctx := context.Background()

	ws, err := s.Domains.CreateWebsite(ctx, "Holzcloud", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	for key, text := range map[string]string{
		"version-holzcloud-cms": "0.1.5",
		"preis":                 "CHF 12.–",
	} {
		if _, _, err := s.Values.Set(ctx, ws.ID, key, text); err != nil {
			t.Fatalf("Set(%s): %v", key, err)
		}
	}
	if _, err := s.Pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "Start", Slug: "start", Status: "published",
		Markdown: "Alpha [[wert:version-holzcloud-cms]]: neu", HTML: "<p>x</p>",
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	// Another website with a value of its own, which must stay where it is.
	other, err := s.Domains.CreateWebsite(ctx, "Nachbar", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	if _, _, err := s.Values.Set(ctx, other.ID, "fremd", "nicht mitnehmen"); err != nil {
		t.Fatalf("Set(fremd): %v", err)
	}

	archive := exportTo(t, s, ws.ID)
	m := decodedManifest(t, archive)
	got := map[string]string{}
	for _, v := range m.Values {
		got[v.Key] = v.Value
	}
	want := map[string]string{"version-holzcloud-cms": "0.1.5", "preis": "CHF 12.–"}
	if len(m.Values) != 2 || got["version-holzcloud-cms"] != want["version-holzcloud-cms"] || got["preis"] != want["preis"] {
		t.Errorf("manifest values = %+v, want exactly %v", m.Values, want)
	}
	if strings.Contains(manifestOf(t, archive), "fremd") {
		t.Error("the export carries another website's value")
	}

	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "Holzcloud (Kopie)")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Values != 2 {
		t.Errorf("report.Values = %d, want 2", report.Values)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("a clean round trip warned: %v", report.Warnings)
	}
	list, err := s.Values.List(ctx, report.WebsiteID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	copied := map[string]string{}
	for _, v := range list {
		copied[v.Key] = v.Text
	}
	if len(list) != 2 || copied["version-holzcloud-cms"] != "0.1.5" || copied["preis"] != "CHF 12.–" {
		t.Errorf("imported values = %v, want %v", copied, want)
	}
	start, err := s.Pages.GetPageBySlug(ctx, report.WebsiteID, "start")
	if err != nil || start == nil {
		t.Fatalf("the page did not arrive: %v", err)
	}
	if !strings.Contains(start.ContentMarkdown, "[[wert:version-holzcloud-cms]]") {
		t.Errorf("the page lost its placeholder: %q", start.ContentMarkdown)
	}

	// The neighbour keeps its value and gains none.
	theirs, _ := s.Values.List(ctx, other.ID)
	if len(theirs) != 1 || theirs[0].Key != "fremd" {
		t.Errorf("the other website's values changed: %+v", theirs)
	}
}

// The field is optional: a bundle written before values existed, or by a
// website that has none, imports exactly as it did, and a website without
// values writes no values key.
func TestABundleWithoutValuesImportsAsBefore(t *testing.T) {
	s := newValueStores(t)
	ctx := context.Background()

	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Ohne Werte"},
		Pages: []Page{{
			Title: "Start", Slug: "start", Status: "published", Markdown: "Hallo.",
		}},
	})
	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Values != 0 {
		t.Errorf("report.Values = %d for a manifest with no values key", report.Values)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("a manifest with no values key warned: %v", report.Warnings)
	}
	if report.Pages != 1 {
		t.Errorf("report.Pages = %d, want 1", report.Pages)
	}
	list, _ := s.Values.List(ctx, report.WebsiteID)
	if len(list) != 0 {
		t.Errorf("values appeared from nowhere: %+v", list)
	}

	written := manifestOf(t, exportTo(t, s, report.WebsiteID))
	if strings.Contains(written, `"values"`) {
		t.Errorf("a website without values wrote a values key:\n%s", written)
	}
}

// A bundle is a file anyone can edit, so its values are exactly as untrusted
// as a form field. Each refused entry is named in the report, and the rest of
// the import goes on.
func TestAnImportNamesTheValuesItRefused(t *testing.T) {
	s := newValueStores(t)
	ctx := context.Background()

	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Von Hand"},
		Pages: []Page{{
			Title: "Start", Slug: "start", Status: "published", Markdown: "Preis [[wert:preis]].",
		}},
		Values: []SiteValue{
			{Key: "Version", Value: "0.1.5"},
			{Key: "leer", Value: ""},
			{Key: "zeile", Value: "a\nb"},
			{Key: "marke", Value: "x [[y"},
			{Key: "lang", Value: strings.Repeat("x", 401)},
			{Key: "preis", Value: "CHF 12.–"},
		},
	})
	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Values != 1 {
		t.Errorf("report.Values = %d, want 1", report.Values)
	}
	if report.Pages != 1 {
		t.Errorf("report.Pages = %d, want 1: a refused value must not stop the pages", report.Pages)
	}
	list, err := s.Values.List(ctx, report.WebsiteID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Key != "preis" || list[0].Text != "CHF 12.–" {
		t.Errorf("stored values = %+v, want only preis", list)
	}
	for _, key := range []string{"Version", "leer", "zeile", "marke", "lang"} {
		n := 0
		for _, w := range report.Warnings {
			if strings.Contains(w, `"`+key+`"`) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%d warnings name %q, want one; warnings: %v", n, key, report.Warnings)
		}
	}
	if warned(report, `"preis"`) {
		t.Errorf("the stored value was warned about: %v", report.Warnings)
	}
}

// A hand-edited bundle may name one key twice. The first text is kept and the
// second is named, rather than one silently replacing the other.
func TestADuplicateValueKeyIsNamedNotOverwritten(t *testing.T) {
	s := newValueStores(t)
	ctx := context.Background()

	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Doppelt"},
		Values: []SiteValue{
			{Key: "preis", Value: "CHF 12.–"},
			{Key: "preis", Value: "CHF 99.–"},
		},
	})
	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Values != 1 {
		t.Errorf("report.Values = %d, want 1", report.Values)
	}
	list, _ := s.Values.List(ctx, report.WebsiteID)
	if len(list) != 1 || list[0].Text != "CHF 12.–" {
		t.Errorf("stored values = %+v, want the first text", list)
	}
	if len(report.Warnings) != 1 || !warned(report, `"preis"`) {
		t.Errorf("warnings = %v, want one naming preis", report.Warnings)
	}
}

// A build without the value store exports and imports as before: the values
// of a manifest are not stored and nothing breaks.
func TestWithoutAValueStoreTheValuesStayBehind(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	archive := archiveWith(t, Manifest{
		Version: Version,
		Site:    Site{Name: "Ohne Speicher"},
		Values:  []SiteValue{{Key: "preis", Value: "CHF 12.–"}},
	})
	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Values != 0 {
		t.Errorf("report.Values = %d without a value store", report.Values)
	}
	written := manifestOf(t, exportTo(t, s, report.WebsiteID))
	if strings.Contains(written, `"values"`) {
		t.Errorf("an export without a value store wrote a values key:\n%s", written)
	}
}
