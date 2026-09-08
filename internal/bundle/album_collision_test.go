package bundle

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// Two albums with one name, which is a state the store used to produce.
//
// A manifest carries an album by its NAME and by nothing else — a slug is not
// transferable across machines, which is exportAlbums' whole argument. That is
// sound only for as long as a name identifies an album, and until this fix
// album.Store.Rename said in its own doc comment that it does not: "two albums
// may therefore end up with the same visible name and different addresses".
//
// A rename is the operation GAL-04 is built around, so this was not an exotic
// state. What it cost, end to end: two manifest entries that nothing tells
// apart; the second Create refused by UNIQUE (website_id, slug); an album and
// all of its pictures lost; and — because the check was made against what the
// MANIFEST declared rather than against what was CREATED — every gallery that
// pointed at the lost album silently bound to the survivor and showed a
// stranger's photographs, with missingAlbum reporting nothing at all, because
// the name had indeed been declared.

// TestRenameRefusesANameAnotherAlbumAlreadyHas closes the source.
//
// Create has refused a duplicate since 00050, through the UNIQUE constraint on
// the slug. Rename could not see that case, because the slug does not move —
// and the slug not moving is exactly what GAL-04 needs. So the check is on the
// name, in the same transaction as the write.
func TestRenameRefusesANameAnotherAlbumAlreadyHas(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()
	ws, first, _ := seedAlbumSite(t, s, "Werkstatt", "Werkstatt 2024")

	second, err := s.Albums.Create(ctx, ws, "Werkstatt 2025")
	if err != nil {
		t.Fatalf("Albums.Create: %v", err)
	}

	err = s.Albums.Rename(ctx, ws, first, "Werkstatt 2025")
	if err == nil {
		t.Fatal("Rename produced a second album called \"Werkstatt 2025\" — an archive of this website can no longer say which of them a gallery means")
	}
	if !strings.Contains(err.Error(), album.ErrDuplicateName.Error()) {
		t.Errorf("Rename refused with %v; want the named ErrDuplicateName, which is what albumSaid turns into a sentence", err)
	}

	// The negative controls, or a Rename that refused everything would pass.
	unchanged, err := s.Albums.Get(ctx, ws, first)
	if err != nil || unchanged == nil {
		t.Fatalf("Albums.Get: %v", err)
	}
	if unchanged.Name != "Werkstatt 2024" {
		t.Errorf("the refused rename still landed: name is %q", unchanged.Name)
	}
	if err := s.Albums.Rename(ctx, ws, first, "Werkstatt, alt"); err != nil {
		t.Fatalf("an ordinary rename was refused too: %v", err)
	}
	if err := s.Albums.Rename(ctx, ws, second.ID, "Werkstatt 2025"); err != nil {
		t.Fatalf("renaming an album to the name it already has was refused: %v", err)
	}
	// And the name is free on another website: albums are scoped, and two
	// websites may both have a "Referenzen".
	other, err := s.Domains.CreateWebsite(ctx, "Andere", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	third, err := s.Albums.Create(ctx, other.ID, "Etwas")
	if err != nil {
		t.Fatalf("Albums.Create on the other website: %v", err)
	}
	if err := s.Albums.Rename(ctx, other.ID, third.ID, "Werkstatt 2025"); err != nil {
		t.Errorf("a name taken on ANOTHER website was refused here: %v", err)
	}
}

// TestCreateRefusesANameARenamedAlbumAlreadyHas closes the other door, and it
// is the door the browser pass of 11-07 walked through.
//
// The comment above says "Create has refused a duplicate since 00050, through
// the UNIQUE constraint on the slug". True — while a name still derives its
// album's slug. A rename breaks that on purpose (GAL-04 needs the address to
// stand still), and from then on the OLD name is free under a new slug:
//
//	create "Werkstatt 2024"           -> slug werkstatt-2024
//	rename it to "Werkstatt 2025"     -> slug is STILL werkstatt-2024
//	create "Werkstatt 2025"           -> slug werkstatt-2025, no constraint hit
//
// Two albums, one name, no error — the exact state this file forbids, reached
// without ever calling Rename twice. Found in a browser on 2026-09-08, not in a
// test, which is why the browser gate exists.
func TestCreateRefusesANameARenamedAlbumAlreadyHas(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()
	ws, first, _ := seedAlbumSite(t, s, "Werkstatt", "Werkstatt 2024")

	if err := s.Albums.Rename(ctx, ws, first, "Werkstatt 2025"); err != nil {
		t.Fatalf("Albums.Rename: %v", err)
	}

	_, err := s.Albums.Create(ctx, ws, "Werkstatt 2025")
	if err == nil {
		t.Fatal("Create produced a second album called \"Werkstatt 2025\" — an archive of this website can no longer say which of them a gallery means")
	}
	if !strings.Contains(err.Error(), album.ErrDuplicateName.Error()) {
		t.Errorf("Create refused with %v; want the named ErrDuplicateName, which is what albumSaid turns into a sentence", err)
	}

	// The negative control: an ordinary create is still an ordinary create, or
	// a Create that refused everything would pass this test.
	if _, err := s.Albums.Create(ctx, ws, "Werkstatt 2026"); err != nil {
		t.Fatalf("an ordinary create was refused too: %v", err)
	}

	// And the slug collision keeps its OWN sentence. "Werkstatt 2024" is the
	// address the renamed album still carries while no album is called that
	// any more, so answering it with ErrDuplicateName would send an operator
	// to a list in which that name does not appear.
	_, err = s.Albums.Create(ctx, ws, "Werkstatt 2024")
	if err == nil {
		t.Fatal("Create took an address another album still holds")
	}
	if !strings.Contains(err.Error(), album.ErrDuplicateSlug.Error()) {
		t.Errorf("a slug collision refused with %v; want the named ErrDuplicateSlug", err)
	}
	if strings.Contains(err.Error(), album.ErrDuplicateName.Error()) {
		t.Error("a slug collision was reported as a name collision — the list shows no such name")
	}
}

// TestTwoAlbumsWithOneNameDoNotCollapseIntoOne is the import half, driven from
// a manifest rather than through a rename — because a manifest is a file
// anybody can edit (T-11-27), so the fix above closes the source and this one
// has to hold when the source is bypassed.
//
// The requirement is NOT that both albums arrive: a name-keyed manifest cannot
// express two albums with one name, and pretending otherwise would be inventing
// data. The requirement is that no gallery is bound to an album that is not the
// one it meant, and that the operator is told.
func TestTwoAlbumsWithOneNameDoNotCollapseIntoOne(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	// A manifest is built by hand here, and it is the same manifest an export
	// of a database in this state writes: two entries, one name, nothing to
	// tell them apart.
	const name = "Werkstatt 2025"
	set := block.Builtin

	m := &Manifest{
		Version: Version,
		Site:    Site{Name: "Werkstatt", Locale: "de"},
		Albums: []Album{
			{Name: name, Items: []AlbumItem{{Media: "hobel.jpg", Alt: "Ein Hobel"}}},
			{Name: name},
		},
		Pages: []Page{{
			Title: "Arbeiten", Slug: "arbeiten", Status: "published",
			Blocks: []Block{
				{Type: block.TypeGallery, Album: name},
				{Type: block.TypeGallery, Album: name},
			},
		}},
	}

	ws, err := s.Domains.CreateWebsite(ctx, "Ziel", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	report := &Report{WebsiteID: ws.ID}
	albumSlugs := importAlbums(ctx, s, ws.ID, m, map[string]int64{}, report)
	importPages(ctx, s, ws.ID, m, map[string]int64{}, albumSlugs, map[string]string{}, set, report)

	// 1. The ambiguous name binds nothing. Anything else is a gallery showing
	//    the wrong album's pictures.
	if slug := albumSlugs[name]; slug != "" {
		t.Errorf("the ambiguous name resolved to %q — a gallery has just been bound to one of two albums, chosen by which came first", slug)
	}

	pg, err := s.Pages.GetPageBySlug(ctx, ws.ID, "arbeiten")
	if err != nil || pg == nil {
		t.Fatalf("GetPageBySlug: %v — %v", err, report.Warnings)
	}
	arrived, err := block.Decode(pg.Blocks, set)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	for i, b := range arrived {
		if b.AlbumSlug != "" {
			t.Errorf("block %d points at album %q although the archive named two albums %q and only one of them can exist",
				i, b.AlbumSlug, name)
		}
	}

	// 2. And the operator is told, by name. The old code was silent here for
	//    the exact reason that made it wrong: the name HAD been declared.
	joined := strings.Join(report.Warnings, "\n")
	if !strings.Contains(joined, "zwei Alben") {
		t.Errorf("nothing in the report says the archive named one album twice:\n%s", joined)
	}
	if !strings.Contains(joined, "Arbeiten") || !strings.Contains(joined, name) {
		t.Errorf("no warning connects the loss to the page whose galleries it emptied:\n%s", joined)
	}
}

// TestAnAlbumThatCouldNotBeCreatedBindsNoGallery is the sibling case, and it is
// the one that shows the check was in the wrong place rather than merely
// incomplete: any failure of Create at all — not just a duplicate name — used
// to leave every gallery naming that album bound to whatever else answered.
func TestAnAlbumThatCouldNotBeCreatedBindsNoGallery(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	// A name of nothing but spaces is not a name; album.Store.Create refuses it
	// with ErrNoName. The manifest declares it all the same, which is what a
	// hand-edited archive looks like.
	m := &Manifest{
		Version: Version,
		Site:    Site{Name: "Werkstatt", Locale: "de"},
		Albums:  []Album{{Name: "   "}},
		Pages: []Page{{
			Title: "Arbeiten", Slug: "arbeiten", Status: "published",
			Blocks: []Block{{Type: block.TypeGallery, Album: "   "}},
		}},
	}

	ws, err := s.Domains.CreateWebsite(ctx, "Ziel", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	report := &Report{WebsiteID: ws.ID}
	albumSlugs := importAlbums(ctx, s, ws.ID, m, map[string]int64{}, report)
	importPages(ctx, s, ws.ID, m, map[string]int64{}, albumSlugs, map[string]string{}, block.Builtin, report)

	if report.Albums != 0 {
		t.Errorf("report.Albums = %d; the archive's claim was counted rather than what was made", report.Albums)
	}
	if list, err := s.Albums.List(ctx, ws.ID); err != nil || len(list) != 0 {
		t.Fatalf("Albums.List = %v, %v; the fixture is meant to have no album at all", list, err)
	}

	// The effect, which is the assertion that matters: the gallery is bound to
	// nothing. page.Slugify("   ") is "untitled", so a derivation would have
	// produced a perfectly plausible address pointing at no album on earth.
	pg, err := s.Pages.GetPageBySlug(ctx, ws.ID, "arbeiten")
	if err != nil || pg == nil {
		t.Fatalf("GetPageBySlug: %v — %v", err, report.Warnings)
	}
	arrived, err := block.Decode(pg.Blocks, block.Builtin)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	for i, b := range arrived {
		if b.AlbumSlug != "" {
			t.Errorf("block %d points at album %q although no album was created at all", i, b.AlbumSlug)
		}
	}
	if !strings.Contains(strings.Join(report.Warnings, "\n"), "Arbeiten") {
		t.Errorf("the page whose gallery lost its album is not in the report:\n%v", report.Warnings)
	}
}

// TestAnOverlongAlbumNameLandsWhereTheStorePutIt is WR-04: the second
// derivation of one key, removed.
//
// album.Store.Create slugifies the NORMALISED name — normalizeName cuts it at
// album.MaxNameLength runes — and importBlocks used to slugify the RAW one. For
// any manifest name longer than the cap the two disagreed, so Create made an
// album at one address and every gallery reference was written to another. The
// gallery rendered nothing, and missingAlbum reported nothing, because the name
// had been declared. internal/album's package comment forbids exactly this in
// as many words; internal/term/store.go warns about it at length.
func TestAnOverlongAlbumNameLandsWhereTheStorePutIt(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()

	long := strings.Repeat("Werkstatt ", 12) // 120 runes, twice the cap
	if len([]rune(long)) <= album.MaxNameLength {
		t.Fatalf("the fixture name is %d runes, which is not over the cap of %d — the test would prove nothing",
			len([]rune(long)), album.MaxNameLength)
	}
	if page.Slugify(long) == page.Slugify(strings.TrimSpace(string([]rune(long)[:album.MaxNameLength]))) {
		t.Fatal("the raw and the normalised name slugify the same; the test would prove nothing")
	}

	m := &Manifest{
		Version: Version,
		Site:    Site{Name: "Werkstatt", Locale: "de"},
		Albums:  []Album{{Name: long}},
		Pages: []Page{{
			Title: "Arbeiten", Slug: "arbeiten", Status: "published",
			Blocks: []Block{{Type: block.TypeGallery, Album: long}},
		}},
	}

	ws, err := s.Domains.CreateWebsite(ctx, "Ziel", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	report := &Report{WebsiteID: ws.ID}
	albumSlugs := importAlbums(ctx, s, ws.ID, m, map[string]int64{}, report)
	importPages(ctx, s, ws.ID, m, map[string]int64{}, albumSlugs, map[string]string{}, block.Builtin, report)

	list, err := s.Albums.List(ctx, ws.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("Albums.List = %v, %v", list, err)
	}
	pg, err := s.Pages.GetPageBySlug(ctx, ws.ID, "arbeiten")
	if err != nil || pg == nil {
		t.Fatalf("GetPageBySlug: %v", err)
	}
	arrived, err := block.Decode(pg.Blocks, block.Builtin)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(arrived) != 1 {
		t.Fatalf("%d blocks instead of 1: %+v", len(arrived), arrived)
	}
	if arrived[0].AlbumSlug != list[0].Slug {
		t.Errorf("the gallery points at %q and the album the store made is at %q — two derivations of one key, and nothing anywhere reports the empty gallery",
			arrived[0].AlbumSlug, list[0].Slug)
	}
}

// TestAnHonestExportStillCarriesEveryAlbumByName is the guard on the export
// side: the fix must not have made an archive quieter than the website.
func TestAnHonestExportStillCarriesEveryAlbumByName(t *testing.T) {
	s := newStores(t)
	ctx := context.Background()
	ws, _, _ := seedAlbumSite(t, s, "Werkstatt", "Referenzen")
	if _, err := s.Albums.Create(ctx, ws, "Werkzeug"); err != nil {
		t.Fatalf("Albums.Create: %v", err)
	}

	var m Manifest
	if err := json.Unmarshal([]byte(manifestOf(t, exportTo(t, s, ws))), &m); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	names := map[string]bool{}
	for _, a := range m.Albums {
		names[a.Name] = true
	}
	for _, want := range []string{"Referenzen", "Werkzeug"} {
		if !names[want] {
			t.Errorf("the manifest lost the album %q: %+v", want, m.Albums)
		}
	}

	// And it still comes back whole.
	archive := exportTo(t, s, ws)
	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "Werkstatt (Kopie)")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Albums != 2 {
		t.Errorf("report.Albums = %d; want 2 — %v", report.Albums, report.Warnings)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("a clean round trip warned: %v", report.Warnings)
	}
}
