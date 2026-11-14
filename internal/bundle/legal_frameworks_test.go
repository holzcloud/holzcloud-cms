package bundle

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"testing"
)

// The legal frameworks travel with the website (LEG-01). A bundle written
// before they existed has no key for them and arrives with the default EU + CH
// (D-05); a website that chose none exports that choice and arrives with none
// (D-16); and a bundle is a file anyone can edit, so what it says goes through
// the same parser as the settings form.

// legalBundleRaw builds an archive around a manifest written by hand, so a
// test can leave a key out or misspell a value the way an old or edited file
// would — the structs of this package could not write either.
func legalBundleRaw(t *testing.T, site string) []byte {
	t.Helper()
	return legalBundleRawWith(t, site, "")
}

// legalBundleRawWith is legalBundleRaw with further top-level members, written
// as JSON without the braces.
func legalBundleRawWith(t *testing.T, site, more string) []byte {
	t.Helper()
	manifest := `{"version": ` + strconv.Itoa(Version) + `, "site": ` + site
	if more != "" {
		manifest += ", " + more
	}
	manifest += `}`
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(ManifestName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(manifest)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// legalBundleImport imports an archive and answers the new website's set.
func legalBundleImport(t *testing.T, s Stores, archive []byte) string {
	t.Helper()
	ctx := context.Background()
	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	ws, err := s.Domains.GetWebsite(ctx, report.WebsiteID)
	if err != nil || ws == nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	return ws.LegalFrameworks
}

// legalBundleSet writes a website's set past every Go rule.
func legalBundleSet(t *testing.T, s Stores, websiteID int64, set string) {
	t.Helper()
	if _, err := s.Domains.DB.Write.ExecContext(context.Background(),
		`UPDATE websites SET legal_frameworks = $1 WHERE id = $2`, set, websiteID); err != nil {
		t.Fatalf("set legal frameworks: %v", err)
	}
}

func TestTheSetTravelsInTheBundle(t *testing.T) {
	s := newStores(t)
	ws := seedSite(t, s)
	legalBundleSet(t, s, ws, "ch,uk,us")

	if got := legalBundleImport(t, s, exportTo(t, s, ws)); got != "ch,uk,us" {
		t.Errorf("the imported website follows %q, want ch,uk,us", got)
	}
}

// Pitfall 3: the import writes the whole settings row, so a missing key that
// read as "" would quietly drop every framework of every older bundle.
func TestAnOldBundleImportsWithTheDefault(t *testing.T) {
	s := newStores(t)
	archive := legalBundleRaw(t, `{"name": "Aus alter Zeit", "locale": "de"}`)

	if got := legalBundleImport(t, s, archive); got != "eu,ch" {
		t.Errorf("a bundle without the key imported as %q, want eu,ch", got)
	}
}

func TestAnEmptySetSurvivesTheBundle(t *testing.T) {
	s := newStores(t)
	ws := seedSite(t, s)
	legalBundleSet(t, s, ws, "")

	archive := exportTo(t, s, ws)
	var m struct {
		Site map[string]any `json:"site"`
	}
	if err := json.Unmarshal([]byte(manifestOf(t, archive)), &m); err != nil {
		t.Fatalf("the manifest is not readable: %v", err)
	}
	v, ok := m.Site["legal_frameworks"]
	if !ok {
		t.Fatal("an empty set wrote no key — the import could not tell it from an old bundle")
	}
	if v != "" {
		t.Errorf("legal_frameworks = %#v, want \"\"", v)
	}
	if got := legalBundleImport(t, s, archive); got != "" {
		t.Errorf("an exported empty set imported as %q", got)
	}
}

func TestABundleCannotSmuggleASpelling(t *testing.T) {
	s := newStores(t)
	archive := legalBundleRaw(t, `{"name": "Bearbeitet", "legal_frameworks": "EU, mars"}`)

	if got := legalBundleImport(t, s, archive); got != "eu" {
		t.Errorf("an edited bundle stored %q, want eu", got)
	}
}
