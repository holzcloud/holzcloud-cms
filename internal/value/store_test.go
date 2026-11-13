package value

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

func newTestStore(t *testing.T) (*Store, *db.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return NewStore(database), database
}

func website(t *testing.T, database *db.DB, name string) int64 {
	t.Helper()
	res, err := database.Write.Exec(`INSERT INTO websites (name, description) VALUES ($1, '')`, name)
	if err != nil {
		t.Fatalf("insert website: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// pageWith inserts a page directly, the way the stored rows look; the store
// reads the source columns and nothing the page package computes.
func pageWith(t *testing.T, database *db.DB, websiteID int64, title, slug, locale, status, markdown, blocks string) int64 {
	t.Helper()
	res, err := database.Write.Exec(
		`INSERT INTO pages (website_id, title, slug, locale, status, content_markdown, content_html, blocks)
		 VALUES ($1, $2, $3, $4, $5, $6, '', $7)`,
		websiteID, title, slug, locale, status, markdown, blocks)
	if err != nil {
		t.Fatalf("insert page %q: %v", title, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func snippetWith(t *testing.T, database *db.DB, websiteID int64, key, name, markdown string) {
	t.Helper()
	if _, err := database.Write.Exec(
		`INSERT INTO snippets (website_id, key, name, content_markdown) VALUES ($1, $2, $3, $4)`,
		websiteID, key, name, markdown); err != nil {
		t.Fatalf("insert snippet: %v", err)
	}
}

func mustCreate(t *testing.T, s *Store, websiteID int64, key, text string) *Value {
	t.Helper()
	v, err := s.Create(context.Background(), websiteID, key, text)
	if err != nil {
		t.Fatalf("Create(%q): %v", key, err)
	}
	return v
}

func TestValuesStayOnTheirWebsite(t *testing.T) {
	ctx := context.Background()
	s, database := newTestStore(t)
	a := website(t, database, "A")
	b := website(t, database, "B")
	va := mustCreate(t, s, a, "geheim", "von A")
	pageWith(t, database, a, "Seite", "seite", "", "published", "[[wert:geheim]]", "")

	list, err := s.List(ctx, b)
	if err != nil || len(list) != 0 {
		t.Errorf("List(B) = %v, %v; want nothing", list, err)
	}
	if got, err := s.Get(ctx, b, va.ID); got != nil || err != nil {
		t.Errorf("Get(B, id of A) = %v, %v; want nil, nil", got, err)
	}
	if got, err := s.GetByKey(ctx, b, "geheim"); got != nil || err != nil {
		t.Errorf("GetByKey(B, key of A) = %v, %v; want nil, nil", got, err)
	}
	texts, _, err := s.Texts(ctx, b)
	if err != nil || len(texts) != 0 {
		t.Errorf("Texts(B) = %v, %v; want nothing", texts, err)
	}
	rep, err := s.Report(ctx, b)
	if err != nil || len(rep.Used) != 0 || len(rep.Undefined) != 0 {
		t.Errorf("Report(B) = %+v, %v; want nothing of A", rep, err)
	}
	if err := s.Delete(ctx, b, "geheim"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete(B, key of A) = %v; want ErrNotFound", err)
	}
	if err := s.UpdateText(ctx, b, va.ID, "von B"); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateText(B, id of A) = %v; want ErrNotFound", err)
	}
	if got, _ := s.Get(ctx, a, va.ID); got == nil || got.Text != "von A" {
		t.Errorf("A's value changed: %+v", got)
	}
}

func TestCreateRefusesATakenKey(t *testing.T) {
	ctx := context.Background()
	s, database := newTestStore(t)
	ws := website(t, database, "A")
	first := mustCreate(t, s, ws, "v", "eins")
	if _, err := s.Create(ctx, ws, "v", "zwei"); !errors.Is(err, ErrKeyTaken) {
		t.Fatalf("second Create = %v; want ErrKeyTaken", err)
	}
	if got, _ := s.Get(ctx, ws, first.ID); got == nil || got.Text != "eins" {
		t.Errorf("the first text changed: %+v", got)
	}
}

func TestTheStoreRefusesWhatTheScreenWouldRefuse(t *testing.T) {
	ctx := context.Background()
	s, database := newTestStore(t)
	ws := website(t, database, "A")
	existing := mustCreate(t, s, ws, "da", "x")

	for _, key := range []string{"Version", "-x", "x-", strings.Repeat("k", 61), ""} {
		if _, err := s.Create(ctx, ws, key, "x"); !errors.Is(err, ErrBadKey) {
			t.Errorf("Create(key %q) = %v; want ErrBadKey", key, err)
		}
		if _, _, err := s.Set(ctx, ws, key, "x"); !errors.Is(err, ErrBadKey) {
			t.Errorf("Set(key %q) = %v; want ErrBadKey", key, err)
		}
	}

	texts := []struct {
		text string
		want error
	}{
		{"", ErrTextEmpty},
		{"   ", ErrTextEmpty},
		{strings.Repeat("a", 401), ErrTextTooLong},
		{strings.Repeat("ä", 201), ErrTextTooLong},
		{"a\nb", ErrTextInvalid},
		{"a\rb", ErrTextInvalid},
		{"a\tb", ErrTextInvalid},
		{"a\xffb", ErrTextInvalid},
		// Line and paragraph separators are line breaks; bidi controls,
		// zero-width characters and the BOM are invisible and change how the
		// sentence around the value reads or compares (WR-04).
		{"a\u2028b", ErrTextInvalid},
		{"a\u2029b", ErrTextInvalid},
		{"0.1.5\u202e", ErrTextInvalid},
		{"\u202ax", ErrTextInvalid},
		{"x\u2066y", ErrTextInvalid},
		{"x\u2069", ErrTextInvalid},
		{"x\u200by", ErrTextInvalid},
		{"x\u200cy", ErrTextInvalid},
		{"x\u200dy", ErrTextInvalid},
		{"x\u2060y", ErrTextInvalid},
		{"\ufeff1", ErrTextInvalid},
		{"x [[wert:y]]", ErrTextMarker},
		{"a [[b", ErrTextMarker},
		// A single bracket either way: a value that holds one can complete a
		// marker with the bracket beside it in the page or in the next value
		// (WR-01), so neither half is allowed.
		{"[", ErrTextMarker},
		{"[wert:x]]", ErrTextMarker},
		{"wert:x]]", ErrTextMarker},
		{"a]]b", ErrTextMarker},
		{"a]b", ErrTextMarker},
	}
	for i, c := range texts {
		key := "k" + string(rune('a'+i/26)) + string(rune('a'+i%26))
		if _, err := s.Create(ctx, ws, key, c.text); !errors.Is(err, c.want) {
			t.Errorf("Create(text %q) = %v; want %v", c.text, err, c.want)
		}
		if _, _, err := s.Set(ctx, ws, key, c.text); !errors.Is(err, c.want) {
			t.Errorf("Set(text %q) = %v; want %v", c.text, err, c.want)
		}
		if err := s.UpdateText(ctx, ws, existing.ID, c.text); !errors.Is(err, c.want) {
			t.Errorf("UpdateText(text %q) = %v; want %v", c.text, err, c.want)
		}
	}

	exact := strings.Repeat("a", MaxTextBytes)
	if _, err := s.Create(ctx, ws, "genau", exact); err != nil {
		t.Errorf("Create(400 bytes) = %v; want accepted", err)
	}
	if err := s.UpdateText(ctx, ws, existing.ID, exact); err != nil {
		t.Errorf("UpdateText(400 bytes) = %v; want accepted", err)
	}
	if _, _, err := s.Set(ctx, ws, "genau-set", exact); err != nil {
		t.Errorf("Set(400 bytes) = %v; want accepted", err)
	}

	v, err := s.Create(ctx, ws, "getrimmt", "  0.1.5  ")
	if err != nil || v.Text != "0.1.5" {
		t.Errorf("Create(\"  0.1.5  \") = %+v, %v; want the text trimmed", v, err)
	}
	if err := s.UpdateText(ctx, ws, existing.ID, "  0.1.6  "); err != nil {
		t.Fatalf("UpdateText: %v", err)
	}
	if got, _ := s.Get(ctx, ws, existing.ID); got.Text != "0.1.6" {
		t.Errorf("UpdateText stored %q; want trimmed", got.Text)
	}
	if got, _, err := s.Set(ctx, ws, "getrimmt-set", "  0.1.7  "); err != nil || got.Text != "0.1.7" {
		t.Errorf("Set stored %+v, %v; want trimmed", got, err)
	}
}

func TestUpdateTextKeepsTheStampWhenNothingChanged(t *testing.T) {
	ctx := context.Background()
	s, database := newTestStore(t)
	ws := website(t, database, "A")
	v := mustCreate(t, s, ws, "v", "0.1.5")
	const past = "2020-01-02T03:04:05Z"
	if _, err := database.Write.Exec(`UPDATE site_values SET updated_at = $1 WHERE id = $2`, past, v.ID); err != nil {
		t.Fatalf("set the stamp: %v", err)
	}

	if err := s.UpdateText(ctx, ws, v.ID, "0.1.5"); err != nil {
		t.Fatalf("UpdateText(same): %v", err)
	}
	got, _ := s.Get(ctx, ws, v.ID)
	if got.UpdatedAt.Format(timeLayout) != past {
		t.Errorf("the same text moved updated_at to %v", got.UpdatedAt)
	}

	if err := s.UpdateText(ctx, ws, v.ID, "0.1.6"); err != nil {
		t.Fatalf("UpdateText(new): %v", err)
	}
	got, _ = s.Get(ctx, ws, v.ID)
	if got.UpdatedAt.Format(timeLayout) == past {
		t.Errorf("a new text left updated_at at %v", got.UpdatedAt)
	}
}

func TestSetCreatesOrChanges(t *testing.T) {
	ctx := context.Background()
	s, database := newTestStore(t)
	ws := website(t, database, "A")
	v, created, err := s.Set(ctx, ws, "version-holzcloud-cms", "0.1.5")
	if err != nil || !created || v.Text != "0.1.5" {
		t.Fatalf("first Set = %+v, %v, %v; want created", v, created, err)
	}
	v, created, err = s.Set(ctx, ws, "version-holzcloud-cms", "0.1.6")
	if err != nil || created || v.Text != "0.1.6" {
		t.Fatalf("second Set = %+v, %v, %v; want changed, not created", v, created, err)
	}
	if list, _ := s.List(ctx, ws); len(list) != 1 {
		t.Errorf("List = %v; want one value", list)
	}
}

func TestDeleteRefusesAValueInUseAndNamesTheUsers(t *testing.T) {
	ctx := context.Background()
	s, database := newTestStore(t)
	ws := website(t, database, "A")
	mustCreate(t, s, ws, "v", "0.1.5")
	pageWith(t, database, ws, "Page B", "page-b", "fr", "draft", "Version [[wert:v]]", "")
	pageWith(t, database, ws, "Seite A", "seite-a", "", "published", "Version [[wert:v]]", "")
	snippetWith(t, database, ws, "fuss", "Fuss", "Stand [[wert:v]]")

	err := s.Delete(ctx, ws, "v")
	var inUse *InUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("Delete = %v; want *InUseError", err)
	}
	want := []string{"Seite A", "Page B (fr)", "Fuss [[snippet:fuss]]"}
	if !reflect.DeepEqual(inUse.Users, want) {
		t.Errorf("Users = %q; want %q", inUse.Users, want)
	}
	if got, _ := s.GetByKey(ctx, ws, "v"); got == nil {
		t.Error("the value is gone although it was in use")
	}
}

func TestATrashedPageDoesNotKeepAValue(t *testing.T) {
	ctx := context.Background()
	s, database := newTestStore(t)
	ws := website(t, database, "A")
	mustCreate(t, s, ws, "v", "0.1.5")
	id := pageWith(t, database, ws, "Alt", "alt", "", "published", "[[wert:v]]", "")
	if _, err := database.Write.Exec(`UPDATE pages SET deleted_at = '2026-01-01T00:00:00Z' WHERE id = $1`, id); err != nil {
		t.Fatalf("trash: %v", err)
	}
	if err := s.Delete(ctx, ws, "v"); err != nil {
		t.Fatalf("Delete = %v; a trashed page must not keep a value", err)
	}
	if got, _ := s.GetByKey(ctx, ws, "v"); got != nil {
		t.Error("the value is still there")
	}
}

func TestAKeyWithAnUnderscoreIsNotAWildcard(t *testing.T) {
	ctx := context.Background()
	s, database := newTestStore(t)
	ws := website(t, database, "A")
	mustCreate(t, s, ws, "a_b", "x")
	pageWith(t, database, ws, "Andere", "andere", "", "published", "[[wert:axb]]", "")

	rep, err := s.Report(ctx, ws)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if len(rep.Used["a_b"]) != 0 {
		t.Errorf("Report counts %v as users of a_b", rep.Used["a_b"])
	}
	if err := s.Delete(ctx, ws, "a_b"); err != nil {
		t.Errorf("Delete(a_b) = %v; a page naming only axb does not use it", err)
	}
}

// TestTheScanSeesWhatDeliveryResolves is WR-02 of the phase 33 review. Delivery
// resolves the stored HTML and the page's excerpt; the scan read only the
// Markdown and the blocks. A typed excerpt, and a marker the Markdown wrote
// with escaped brackets (\[\[wert:k\]\], literal in the HTML), are both
// resolved for the visitor, so both keep a value alive and both are reported.
func TestTheScanSeesWhatDeliveryResolves(t *testing.T) {
	ctx := context.Background()
	s, database := newTestStore(t)
	ws := website(t, database, "A")
	for _, key := range []string{"im-auszug", "im-html", "im-baustein"} {
		mustCreate(t, s, ws, key, "0.1.5")
	}
	insert := func(title, slug, markdown, html, excerpt string) {
		t.Helper()
		if _, err := database.Write.Exec(
			`INSERT INTO pages (website_id, title, slug, locale, status, content_markdown, content_html, excerpt)
			 VALUES ($1, $2, $3, '', 'published', $4, $5, $6)`,
			ws, title, slug, markdown, html, excerpt); err != nil {
			t.Fatalf("insert page %q: %v", title, err)
		}
	}
	insert("Auszug", "auszug", "Kein Wert im Text.", "<p>Kein Wert im Text.</p>", "Version [[wert:im-auszug]] und [[wert:fehlt-auszug]]")
	insert("Maskiert", "maskiert", `Version \[\[wert:im-html\]\]`, "<p>Version [[wert:im-html]] [[wert:fehlt-html]]</p>", "")
	if _, err := database.Write.Exec(
		`INSERT INTO snippets (website_id, key, name, content_markdown, content_html) VALUES ($1, 'fuss', 'Fuss', $2, $3)`,
		ws, `Stand \[\[wert:im-baustein\]\]`, "<p>Stand [[wert:im-baustein]]</p>"); err != nil {
		t.Fatalf("insert snippet: %v", err)
	}

	for key, user := range map[string]string{
		"im-auszug": "Auszug", "im-html": "Maskiert", "im-baustein": "Fuss [[snippet:fuss]]",
	} {
		var inUse *InUseError
		if err := s.Delete(ctx, ws, key); !errors.As(err, &inUse) {
			t.Errorf("Delete(%s) = %v; delivery resolves it, so it is in use", key, err)
			continue
		}
		if !reflect.DeepEqual(inUse.Users, []string{user}) {
			t.Errorf("Delete(%s) names %q; want [%s]", key, inUse.Users, user)
		}
	}
	rep, err := s.Report(ctx, ws)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if want := []string{"fehlt-auszug", "fehlt-html"}; !reflect.DeepEqual(rep.Undefined, want) {
		t.Errorf("Undefined = %q; want %q", rep.Undefined, want)
	}
	// Each page once per key, although the key stands in two of its columns.
	insert("Doppelt", "doppelt", "[[wert:im-html]]", "<p>[[wert:im-html]]</p>", "[[wert:im-html]]")
	rep, _ = s.Report(ctx, ws)
	if n := len(rep.Used["im-html"]); n != 2 {
		t.Errorf("Used[im-html] has %d entries; want 2 (each page once)", n)
	}
}

// TestAProductKeepsAValueAlive is the other half of WR-03 of the phase 33
// review: the shop resolves values in a product's description, so a product
// that names a key keeps it from being deleted, counts as a user, and an
// unknown key in it is reported — drafts included, like a page.
func TestAProductKeepsAValueAlive(t *testing.T) {
	ctx := context.Background()
	s, database := newTestStore(t)
	ws := website(t, database, "A")
	other := website(t, database, "B")
	mustCreate(t, s, ws, "telefon", "044 123 45 67")
	insert := func(websiteID int64, slug, title, status, markdown, html string) {
		t.Helper()
		if _, err := database.Write.Exec(
			`INSERT INTO products (website_id, slug, title, status, description_markdown, description_html, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
			websiteID, slug, title, status, markdown, html); err != nil {
			t.Fatalf("insert product: %v", err)
		}
	}
	insert(ws, "tisch", "Tisch", "draft", "Fragen? [[wert:telefon]] [[wert:fehlt]]", "")
	insert(ws, "stuhl", "Stuhl", "published", "", `<p>[[wert:telefon]]</p>`)
	// Another website's product names the same key and is nobody's user here.
	insert(other, "bank", "Bank", "published", "[[wert:telefon]]", "")

	var inUse *InUseError
	if err := s.Delete(ctx, ws, "telefon"); !errors.As(err, &inUse) {
		t.Fatalf("Delete = %v; a product names the value", err)
	}
	if want := []string{"Stuhl", "Tisch"}; !reflect.DeepEqual(inUse.Users, want) {
		t.Errorf("Users = %q; want %q", inUse.Users, want)
	}
	rep, err := s.Report(ctx, ws)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if len(rep.Used["telefon"]) != 2 || rep.Used["telefon"][0].Kind != "product" {
		t.Errorf("Used[telefon] = %+v; want the two products", rep.Used["telefon"])
	}
	if !reflect.DeepEqual(rep.Undefined, []string{"fehlt"}) {
		t.Errorf("Undefined = %q; want [fehlt]", rep.Undefined)
	}
}

// TestTheTableRefusesWhatTheStoreRefuses is IN-04 of the phase 33 review: the
// table holds the shape of a key and the length and the brackets of a text
// itself, so a writer that does not go through the store cannot store what
// the store refuses.
func TestTheTableRefusesWhatTheStoreRefuses(t *testing.T) {
	_, database := newTestStore(t)
	ws := website(t, database, "A")
	insert := func(key, text string) error {
		_, err := database.Write.Exec(`INSERT INTO site_values (website_id, key, value) VALUES ($1, $2, $3)`, ws, key, text)
		return err
	}
	for _, c := range []struct{ key, text string }{
		{"", "x"},
		{strings.Repeat("k", MaxKeyBytes+1), "x"},
		{"Gross", "x"},
		{"-x", "x"},
		{"x-", "x"},
		{"x_", "x"},
		{"a b", "x"},
		// A key of its own for every text, so that UNIQUE is never the
		// reason an insert fails.
		{"t1", ""},
		{"t2", strings.Repeat("ä", MaxTextBytes/2+1)},
		{"t3", "a[b"},
		{"t4", "a]b"},
		{"t5", "a\nb"},
		{"t6", "a\rb"},
		{"t7", "a\tb"},
	} {
		if err := insert(c.key, c.text); err == nil {
			t.Errorf("INSERT (%q, %q) was accepted", c.key, c.text)
		}
	}
	for i, c := range []struct{ key, text string }{
		{"version-holzcloud-cms", "0.1.5"},
		{"a_b", strings.Repeat("ä", MaxTextBytes/2)},
		{strings.Repeat("k", MaxKeyBytes), "x"},
		{"7", "Grüezi"},
	} {
		if err := insert(c.key, c.text); err != nil {
			t.Errorf("INSERT %d (%q) refused: %v", i, c.key, err)
		}
	}
}

// TestADeleteRefusalAlwaysNamesSomebody is IN-03 of the phase 33 review. The
// refusal and the list of users are two reads; when the last page naming the
// key is edited in between, the list was empty and the flash read "still used
// on: .". Delete now tries again, and the value goes.
func TestADeleteRefusalAlwaysNamesSomebody(t *testing.T) {
	ctx := context.Background()
	s, database := newTestStore(t)
	ws := website(t, database, "A")
	mustCreate(t, s, ws, "v", "0.1.5")
	id := pageWith(t, database, ws, "Seite", "seite", "", "published", "Version [[wert:v]]", "")
	calls := 0
	s.afterRefusal = func() {
		calls++
		// The editor removes the placeholder between the two reads.
		if _, err := database.Write.Exec(`UPDATE pages SET content_markdown = 'Version' WHERE id = $1`, id); err != nil {
			t.Fatalf("edit page: %v", err)
		}
	}
	if err := s.Delete(ctx, ws, "v"); err != nil {
		t.Fatalf("Delete = %v; nobody names the value any more, so it goes", err)
	}
	if calls != 1 {
		t.Errorf("the refusal was read %d times; want 1", calls)
	}
	if got, _ := s.GetByKey(ctx, ws, "v"); got != nil {
		t.Error("the value is still there")
	}
}

func TestDeleteSeesAMarkerInABlockLink(t *testing.T) {
	ctx := context.Background()
	s, database := newTestStore(t)
	ws := website(t, database, "A")
	mustCreate(t, s, ws, "v", "0.1.5")
	pageWith(t, database, ws, "Block", "block", "", "published", "",
		`[{"typ":"aufruf","linkziel":"https://x.ch/v[[wert:v]]"}]`)

	var inUse *InUseError
	if err := s.Delete(ctx, ws, "v"); !errors.As(err, &inUse) {
		t.Fatalf("Delete = %v; a marker in a block link keeps the value", err)
	}
	if !reflect.DeepEqual(inUse.Users, []string{"Block"}) {
		t.Errorf("Users = %q; want [Block]", inUse.Users)
	}
}

func TestTheReportListsUseUndefinedAndOutside(t *testing.T) {
	ctx := context.Background()
	s, database := newTestStore(t)
	ws := website(t, database, "A")
	other := website(t, database, "B")
	mustCreate(t, s, ws, "v", "0.1.5")
	mustCreate(t, s, ws, "frei", "ungenutzt")

	pageWith(t, database, ws, "Stand", "stand", "", "published", "[[wert:v]] und [[wert:zzz]] und [[wert:aaa]]", "")
	snippetWith(t, database, ws, "fuss", "Fuss", "[[wert:v]]")
	titled := pageWith(t, database, ws, "Titel [[wert:v]]", "titel", "fr", "draft", "", "")
	described := pageWith(t, database, ws, "Beschrieben", "beschrieben", "", "published", "", "")
	if _, err := database.Write.Exec(`UPDATE pages SET meta_description = 'Jetzt [[wert:v]]' WHERE id = $1`, described); err != nil {
		t.Fatalf("meta: %v", err)
	}
	trashed := pageWith(t, database, ws, "Weg [[wert:v]]", "weg", "", "published", "[[wert:geloescht]]", "")
	if _, err := database.Write.Exec(`UPDATE pages SET deleted_at = '2026-01-01T00:00:00Z' WHERE id = $1`, trashed); err != nil {
		t.Fatalf("trash: %v", err)
	}
	pageWith(t, database, other, "Fremd [[wert:v]]", "fremd", "", "published", "[[wert:fremd]]", "")

	res, err := database.Write.Exec(`INSERT INTO menus (website_id, name, location_key) VALUES ($1, 'Haupt', 'main')`, ws)
	if err != nil {
		t.Fatalf("menu: %v", err)
	}
	menuID, _ := res.LastInsertId()
	res, err = database.Write.Exec(`INSERT INTO menu_items (menu_id, title, url) VALUES ($1, 'Version [[wert:v]]', '/x')`, menuID)
	if err != nil {
		t.Fatalf("menu item: %v", err)
	}
	itemID, _ := res.LastInsertId()
	res, err = database.Write.Exec(`INSERT INTO menus (website_id, name, location_key) VALUES ($1, 'Fremd', 'main')`, other)
	if err != nil {
		t.Fatalf("other menu: %v", err)
	}
	otherMenu, _ := res.LastInsertId()
	if _, err := database.Write.Exec(`INSERT INTO menu_items (menu_id, title, url) VALUES ($1, 'x', '/[[wert:v]]')`, otherMenu); err != nil {
		t.Fatalf("other menu item: %v", err)
	}

	rep, err := s.Report(ctx, ws)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}

	var users []string
	for _, u := range rep.Used["v"] {
		users = append(users, u.Name())
	}
	if !reflect.DeepEqual(users, []string{"Stand", "Fuss [[snippet:fuss]]"}) {
		t.Errorf("Used[v] = %q; want [Stand, Fuss [[snippet:fuss]]]", users)
	}
	if len(rep.Used["zzz"]) != 1 || rep.Used["zzz"][0].Title != "Stand" {
		t.Errorf("Used[zzz] = %+v; an undefined key is listed with its users", rep.Used["zzz"])
	}
	if _, ok := rep.Used["frei"]; ok {
		t.Errorf("Used lists a value nobody names: %+v", rep.Used["frei"])
	}
	if !reflect.DeepEqual(rep.Undefined, []string{"aaa", "zzz"}) {
		t.Errorf("Undefined = %q; want [aaa zzz]", rep.Undefined)
	}
	for _, k := range []string{"geloescht", "fremd"} {
		if _, ok := rep.Used[k]; ok {
			t.Errorf("Used lists %q from a trashed page or another website", k)
		}
	}

	want := map[string]Outside{
		"title":       {Key: "v", Where: "title", ID: titled, Title: "Titel [[wert:v]]", Locale: "fr"},
		"description": {Key: "v", Where: "description", ID: described, Title: "Beschrieben"},
		"menu":        {Key: "v", Where: "menu", ID: itemID, Title: "Version [[wert:v]]"},
	}
	if len(rep.Outside) != len(want) {
		t.Errorf("Outside = %+v; want exactly %d entries", rep.Outside, len(want))
	}
	for _, o := range rep.Outside {
		w, ok := want[o.Where]
		if !ok || o != w {
			t.Errorf("Outside entry %+v; want %+v", o, w)
		}
	}
}

func TestUseNamesItself(t *testing.T) {
	cases := []struct {
		u    Use
		want string
	}{
		{Use{Kind: "page", Title: "Stand"}, "Stand"},
		{Use{Kind: "page", Title: "Stand", Locale: "fr"}, "Stand (fr)"},
		{Use{Kind: "snippet", Title: "Fuss", Slug: "fuss"}, "Fuss [[snippet:fuss]]"},
	}
	for _, c := range cases {
		if got := c.u.Name(); got != c.want {
			t.Errorf("%+v.Name() = %q; want %q", c.u, got, c.want)
		}
	}
}
