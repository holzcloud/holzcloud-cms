package main

import (
	"archive/zip"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Registered for image.Decode below. The example only holds JPEGs today,
	// but a PNG placeholder is a legitimate thing to add, and a test that
	// failed on the day somebody did would teach the wrong lesson.
	_ "image/jpeg"
	_ "image/png"

	"github.com/holzcloud/holzcloud-cms/internal/bundle"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// beispiel is the example website kept in the repository as source.
func beispiel() string { return filepath.Join("..", "..", "sites", "beispiel") }

// TestTheExampleStillPacks is the reason the example does not quietly rot.
//
// The manifest is written by hand, and readManifest refuses a field that
// internal/bundle does not know. So a field that gets renamed or dropped there
// fails here, on the next `go test ./...`, rather than waiting for the next
// person who happens to run mkbundle by hand — which, for an example nobody
// deploys, could be months.
//
// It packs into t.TempDir(): the archive is a build artefact, and a test that
// left one beside the source would put an ignored file into everybody's
// `git status` forever.
func TestTheExampleStillPacks(t *testing.T) {
	out := filepath.Join(t.TempDir(), "beispiel.zip")
	if err := pack(beispiel(), out); err != nil {
		t.Fatalf("pack: %v", err)
	}

	m, err := readManifest(filepath.Join(beispiel(), bundle.ManifestName))
	if err != nil {
		t.Fatalf("readManifest: %v", err)
	}

	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatalf("the archive is not readable: %v", err)
	}
	defer zr.Close()

	inArchive := map[string]bool{}
	for _, f := range zr.File {
		inArchive[f.Name] = true
	}

	if !inArchive[bundle.ManifestName] {
		t.Errorf("the archive has no %s", bundle.ManifestName)
	}
	// Both directions, for the same reason checkMedia insists on both: a file
	// that travels unlisted is invisible, and a listed file that does not
	// travel renders as a hole on the target machine.
	for _, med := range m.Media {
		if !inArchive[bundle.MediaDir+med.Filename] {
			t.Errorf("%s is in the manifest but not in the archive", med.Filename)
		}
	}
	if got, want := len(zr.File), len(m.Media)+1; got != want {
		t.Errorf("the archive holds %d entries; want %d (the manifest and %d pictures)",
			got, want, len(m.Media))
	}
}

// TestEveryPictureIsTheTypeItClaims closes the one gap mkbundle leaves open.
//
// mkbundle takes mime_type at its word and never looks at the bytes. A
// manifest claiming image/jpeg over PNG bytes imports without a complaint and
// renders as a broken picture on a machine where nobody who could recognise
// the mistake is looking. The example is what people copy, so a lie about the
// format here would be copied too.
func TestEveryPictureIsTheTypeItClaims(t *testing.T) {
	m, err := readManifest(filepath.Join(beispiel(), bundle.ManifestName))
	if err != nil {
		t.Fatalf("readManifest: %v", err)
	}
	if len(m.Media) == 0 {
		t.Fatal("the example lists no pictures, so this test proves nothing")
	}

	byFormat := map[string]string{"jpeg": "image/jpeg", "png": "image/png"}

	for _, med := range m.Media {
		f, err := os.Open(filepath.Join(beispiel(), "media", med.Filename))
		if err != nil {
			t.Errorf("%s: %v", med.Filename, err)
			continue
		}
		// Decoded whole rather than only the header: a truncated file has a
		// perfectly good header and is still a broken picture.
		_, format, err := image.Decode(f)
		f.Close()
		if err != nil {
			t.Errorf("%s does not decode as an image: %v", med.Filename, err)
			continue
		}
		if got := byFormat[format]; got != med.MimeType {
			t.Errorf("%s is %s data but the manifest declares %s",
				med.Filename, format, med.MimeType)
		}
	}
}

// labelManifest is the smallest manifest that carries a label whose name is not
// its own slug — the shape that could not be packed at all until 2026-09-03.
//
// "Laufräder" slugs to "laufraeder" because internal/page/slug.go folds ä to
// ae, so name and slug genuinely differ. That difference is the whole point:
// a manifest whose labels happen to be lowercase ASCII passes either reading of
// the check and proves nothing about which one is in force.
func labelManifest(pageTerm string) *bundle.Manifest {
	return &bundle.Manifest{
		Site:  bundle.Site{Name: "Werkstatt"},
		Terms: []bundle.Term{{Slug: "laufraeder", Name: "Laufräder"}},
		Pages: []bundle.Page{{
			Title: "Die Werkstatt",
			Slug:  "werkstatt",
			Terms: []string{pageTerm},
		}},
	}
}

// TestAPageMayCarryALabelSpelledAsItsName is the regression guard for the
// defect this file's neighbours were written after.
//
// Export writes t.Name into a page's terms and the import reads a name back, so
// the name is what a page's terms hold. The check used to compare those entries
// against the declared slugs instead, which meant a bundle exported from a real
// site was refused the moment somebody had typed a label with a capital or an
// umlaut — the ordinary case, not the exotic one.
func TestAPageMayCarryALabelSpelledAsItsName(t *testing.T) {
	if err := checkReferences(labelManifest("Laufräder")); err != nil {
		t.Fatalf("a page naming a declared label was refused: %v", err)
	}
}

// TestALabelSpelledAsASlugSaysWhatToWriteInstead keeps the refusal useful.
//
// A manifest hand-written against the old check spells its page terms as slugs.
// It is now refused, correctly — it would have imported a label literally named
// "laufraeder" — but "has no entry in terms" would send its author looking for
// a declaration they can see sitting right there. So the message names the
// spelling to use.
func TestALabelSpelledAsASlugSaysWhatToWriteInstead(t *testing.T) {
	err := checkReferences(labelManifest("laufraeder"))
	if err == nil {
		t.Fatal("a page spelling a label as its slug was accepted")
	}
	if !strings.Contains(err.Error(), `"Laufräder"`) {
		t.Errorf("the message does not name the spelling to use: %v", err)
	}
}

// TestALabelThatIsNeitherNameNorSlugIsStillRefused keeps the check a check.
//
// Moving the lookup from the slugs to the names would be trivially satisfiable
// by widening it to both and calling everything valid. This is the guard that
// says a genuine typo still fails.
func TestALabelThatIsNeitherNameNorSlugIsStillRefused(t *testing.T) {
	err := checkReferences(labelManifest("Möbel"))
	if err == nil {
		t.Fatal("a page naming an undeclared label was accepted")
	}
	if !strings.Contains(err.Error(), "has no entry in terms") {
		t.Errorf("an undeclared label lost its own message: %v", err)
	}
}

// TestTheExampleStillCarriesALabelWorthShowing keeps the shipped example the
// regression guard it was added to be.
//
// The example was without labels for as long as mkbundle could not pack one
// whose name differs from its slug. Now that it can, the example is where the
// format is demonstrated, and "Laufräder" under "laufraeder" is deliberately
// the awkward case rather than a tidy lowercase one. Without this test the next
// person tidying the manifest could drop the umlaut in good faith and quietly
// remove the only shipped case that exercises the fix.
//
// The last check is a rule the example holds itself to and mkbundle does not:
// every declared slug must be the one the import would derive from the name.
// A genuine export from a renamed label legitimately breaks that equality —
// term.Store.Rename changes the name and leaves the slug — so the tool must not
// refuse it. An example declaring a slug the import would never produce would
// teach a fiction, which is a different standard.
func TestTheExampleStillCarriesALabelWorthShowing(t *testing.T) {
	m, err := readManifest(filepath.Join(beispiel(), bundle.ManifestName))
	if err != nil {
		t.Fatalf("readManifest: %v", err)
	}

	awkward := false
	declared := map[string]bool{}
	for _, term := range m.Terms {
		declared[term.Name] = true
		if term.Name != term.Slug {
			awkward = true
		}
		if got := page.Slugify(term.Name); got != term.Slug {
			t.Errorf("the label %q declares the slug %q, but an import would create %q",
				term.Name, term.Slug, got)
		}
	}
	if !awkward {
		t.Error("no label in the example has a name that differs from its slug, so the example no longer shows the case that used to be unpackable")
	}

	carried := 0
	for _, p := range m.Pages {
		for _, name := range p.Terms {
			carried++
			if !declared[name] {
				t.Errorf("page %q carries the label %q, which is not declared", p.Slug, name)
			}
		}
	}
	if carried == 0 {
		t.Error("no page in the example carries a label, so nothing exercises the check")
	}
}
