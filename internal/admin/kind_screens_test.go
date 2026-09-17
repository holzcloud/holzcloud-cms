package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// The two screens where a website says what kinds of thing it holds: its own
// content kinds, and its own kinds of block.
//
// Both are small screens with one property that matters more than the rest:
// removing a kind must never remove the things that carry it. A kind deleted by
// accident taking a hundred products with it is not a mistake anybody recovers
// from by clicking.
//
// That property held. What did not was the sentence about it: kind.Store.Count
// matched the `kind` column alone, and an entry of a website's own kind is
// stored as a PAGE with the key beside it. So three products counted as zero
// products and as three pages, the delete screen's warning never fired, and
// removing a kind with a hundred entries said "Content kind removed" and
// nothing else — the exact silence the branch above it exists to break.
// TestAnEntryCountsUnderItsOwnKindAndNotAlsoUnderPages is that, measured.
//
// Driven red by thirteen mutations, all thirteen caught. Three had to be
// chased, and two of them are the same shape as ever: the handler's last arm
// answers "Saving failed." for anything it does not recognise, so a taken key
// and a reserved key both still produced SOME error with their own branch
// removed — and the operator read a shrug instead of the one sentence that
// says what to change.

func kindRoute(websiteID int64, extra ...string) map[string]string {
	return websiteRoute(websiteID, extra...)
}

func TestAContentKindIsCreatedChangedAndOrdered(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	ctx := context.Background()

	rec, bad, good := albumFlash(t, h, sm, h.HandleKindSave, postForm(
		"/admin/websites/1/inhaltsarten", url.Values{
			"name": {"Produkt"}, "mehrzahl": {"Produkte"},
			"archiv": {"produkte"}, "sortierung": {"titel"},
		}, kindRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}
	// The message names the kind, because the next thing the operator does is
	// look for it in the editor.
	if !strings.Contains(good, "Produkt") {
		t.Errorf("the message does not name the kind: %q", good)
	}

	types, err := h.kinds.List(ctx, ws.ID)
	if err != nil || len(types) != 1 {
		t.Fatalf("List: %v (%d)", err, len(types))
	}
	made := types[0]
	if made.Name != "Produkt" || made.Plural != "Produkte" {
		t.Errorf("singular and plural: %q / %q", made.Name, made.Plural)
	}
	if made.Archive != "produkte" {
		t.Errorf("archive = %q", made.Archive)
	}

	// Changed, by id, rather than created a second time.
	rec, bad, _ = albumFlash(t, h, sm, h.HandleKindSave, postForm(
		"/admin/websites/1/inhaltsarten", url.Values{
			"id":   {strconv.FormatInt(made.ID, 10)},
			"name": {"Möbelstück"}, "mehrzahl": {"Möbelstücke"},
			"archiv": {"moebel"}, "sortierung": {"neueste"},
		}, kindRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("the change: status %d — %q", rec.Code, bad)
	}
	types, _ = h.kinds.List(ctx, ws.ID)
	if len(types) != 1 {
		t.Fatalf("%d kinds after a change, want 1", len(types))
	}
	if types[0].Name != "Möbelstück" || types[0].Archive != "moebel" {
		t.Errorf("the change did not arrive: %+v", types[0])
	}

	// A second kind, then moved up, so the editor's menu has an order the
	// operator chose.
	serve(t, h, sm, h.HandleKindSave, postForm("/admin/websites/1/inhaltsarten",
		url.Values{"name": {"Rezept"}, "mehrzahl": {"Rezepte"}}, kindRoute(ws.ID)))
	types, _ = h.kinds.List(ctx, ws.ID)
	if len(types) != 2 {
		t.Fatalf("%d kinds", len(types))
	}
	second := types[1]
	rec = serve(t, h, sm, h.HandleKindMove, postForm("/admin/websites/1/inhaltsarten/1/move",
		url.Values{"richtung": {"hoch"}},
		kindRoute(ws.ID, "kindID", strconv.FormatInt(second.ID, 10))))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("move: status %d", rec.Code)
	}
	types, _ = h.kinds.List(ctx, ws.ID)
	if types[0].ID != second.ID {
		t.Errorf("the kind did not move up: %v", []int64{types[0].ID, types[1].ID})
	}
}

// An overview's address is reserved the way the post archive's is: a page at
// the same address would never get its turn, because the overview is checked
// first. Refusing here beats winning silently there.
func TestAnOverviewAddressThatIsTakenIsRefused(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	if err := domain.NewStore(database).UpdateSettings(ctx, ws.ID, domain.Settings{
		Locale: "de", TimeZone: "Europe/Zurich", OfflineMode: "notfound",
		BlogBase: "aktuelles",
	}); err != nil {
		t.Fatal(err)
	}
	seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "x", "published")

	for _, c := range []struct{ what, archive string }{
		{"the address of the post archive", "aktuelles"},
		{"the address of a page that exists", "ueber-uns"},
	} {
		rec, bad, _ := albumFlash(t, h, sm, h.HandleKindSave, postForm(
			"/admin/websites/1/inhaltsarten",
			url.Values{"name": {"Produkt"}, "mehrzahl": {"Produkte"}, "archiv": {c.archive}},
			kindRoute(ws.ID)))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status %d", c.what, rec.Code)
		}
		if bad == "" {
			t.Errorf("%s: accepted in silence", c.what)
		}
	}
	if types, _ := h.kinds.List(ctx, ws.ID); len(types) != 0 {
		t.Errorf("%d kinds were created on addresses that are already taken", len(types))
	}

	// A free address goes through, or this would be a screen that refuses every
	// overview. And it is an ADDRESS by the time it is stored: what an operator
	// types is a heading, and an overview at /Möbel%20&%20Stühle is one nobody //nolint:german — the example address, which is the point
	// can link to.
	rec := serve(t, h, sm, h.HandleKindSave, postForm("/admin/websites/1/inhaltsarten",
		url.Values{"name": {"Möbelstück"}, "mehrzahl": {"Möbelstücke"},
			"archiv": {"  Möbel & Stühle  "}},
		kindRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("a free address: status %d", rec.Code)
	}
	types, _ := h.kinds.List(ctx, ws.ID)
	if len(types) != 1 {
		t.Fatalf("a free address was refused too")
	}
	if got := types[0].Archive; got != page.Slugify("Möbel & Stühle") {
		t.Errorf("the overview address was stored as %q, not as an address", got)
	}
}

// The property that matters: a kind is removed, its entries are not, and the
// screen says so with the number in it.
func TestRemovingAContentKindKeepsItsEntriesAndSaysHowMany(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	serve(t, h, sm, h.HandleKindSave, postForm("/admin/websites/1/inhaltsarten",
		url.Values{"name": {"Produkt"}, "mehrzahl": {"Produkte"}}, kindRoute(ws.ID)))
	types, _ := h.kinds.List(ctx, ws.ID)
	if len(types) != 1 {
		t.Fatalf("%d kinds", len(types))
	}
	made := types[0]

	// Three entries of that kind.
	for i, title := range []string{"Tisch", "Stuhl", "Bank"} {
		if _, err := h.pages.CreatePage(ctx, pageOfKind(ws.ID, title, made.Key, i)); err != nil {
			t.Fatalf("create %s: %v", title, err)
		}
	}
	before, err := h.kinds.Count(ctx, ws.ID, made.Key)
	if err != nil || before != 3 {
		t.Fatalf("Count before = %d (%v)", before, err)
	}

	rec, bad, _ := albumFlash(t, h, sm, h.HandleKindDelete,
		postForm("/admin/websites/1/inhaltsarten/1/delete", nil,
			kindRoute(ws.ID, "kindID", strconv.FormatInt(made.ID, 10))))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}
	if types, _ := h.kinds.List(ctx, ws.ID); len(types) != 0 {
		t.Error("the kind is still there")
	}

	// The three entries are still there, still carrying the key.
	after, err := h.kinds.Count(ctx, ws.ID, made.Key)
	if err != nil {
		t.Fatal(err)
	}
	if after != 3 {
		t.Fatalf("%d entries left of 3 — a kind removed by accident took them with it", after)
	}
	_ = database
}

// A kind nothing carries goes without a warning, and one that carries entries
// goes with the number in it. The two messages are different on purpose.
func TestTheDeleteMessageCountsWhatIsLeftBehind(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	ctx := context.Background()

	// Empty first.
	serve(t, h, sm, h.HandleKindSave, postForm("/admin/websites/1/inhaltsarten",
		url.Values{"name": {"Leer"}, "mehrzahl": {"Leere"}}, kindRoute(ws.ID)))
	types, _ := h.kinds.List(ctx, ws.ID)
	empty := types[0]
	_, _, good := albumFlashWarn(t, h, sm, h.HandleKindDelete,
		postForm("/admin/websites/1/inhaltsarten/1/delete", nil,
			kindRoute(ws.ID, "kindID", strconv.FormatInt(empty.ID, 10))))
	if good.warning != "" {
		t.Errorf("an empty kind warned: %q", good.warning)
	}
	if good.success == "" {
		t.Error("an empty kind said nothing at all")
	}

	// Now one with two entries.
	serve(t, h, sm, h.HandleKindSave, postForm("/admin/websites/1/inhaltsarten",
		url.Values{"name": {"Voll"}, "mehrzahl": {"Volle"}}, kindRoute(ws.ID)))
	types, _ = h.kinds.List(ctx, ws.ID)
	full := types[0]
	for i, title := range []string{"Eins", "Zwei"} {
		if _, err := h.pages.CreatePage(ctx, pageOfKind(ws.ID, title, full.Key, i)); err != nil {
			t.Fatal(err)
		}
	}
	_, _, flashes := albumFlashWarn(t, h, sm, h.HandleKindDelete,
		postForm("/admin/websites/1/inhaltsarten/1/delete", nil,
			kindRoute(ws.ID, "kindID", strconv.FormatInt(full.ID, 10))))
	if flashes.warning == "" {
		t.Fatal("a kind with entries went without a warning")
	}
	if !strings.Contains(flashes.warning, "2") {
		t.Errorf("the warning does not say how many were left behind: %q", flashes.warning)
	}
	if !strings.Contains(flashes.warning, full.Key) {
		t.Errorf("the warning does not name the key the entries still carry: %q", flashes.warning)
	}
}

// What counts as what, asserted on its own because two screens read it and
// both read it wrongly before.
//
// An entry of a website's own kind is stored as a PAGE with the key beside it
// (setKind says why: an own kind is a page as far as routing and rendering go).
// So a count that matched the column alone answered two questions wrongly at
// once — measured with one page, one post and three products:
//
//	Count("produkt") = 0   three products counted as none
//	Count("page")    = 4   and counted as pages as well
//
// The first number is what the delete screen uses to decide whether to warn
// that the entries stay behind, so removing a kind with a hundred products said
// "Content kind removed" and nothing more. The second is the "Pages" figure on
// the same screen, inflated by every entry of every own kind.
func TestAnEntryCountsUnderItsOwnKindAndNotAlsoUnderPages(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	serve(t, h, sm, h.HandleKindSave, postForm("/admin/websites/1/inhaltsarten",
		url.Values{"name": {"Produkt"}, "mehrzahl": {"Produkte"}}, kindRoute(ws.ID)))
	types, _ := h.kinds.List(ctx, ws.ID)
	if len(types) != 1 {
		t.Fatalf("%d kinds", len(types))
	}
	made := types[0]

	seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "x", "published")
	if _, err := h.pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "Neue Werkbank", Slug: "neue-werkbank",
		Markdown: "x", Status: "published", Kind: page.KindPost,
	}); err != nil {
		t.Fatal(err)
	}
	for i, title := range []string{"Tisch", "Stuhl", "Bank"} {
		if _, err := h.pages.CreatePage(ctx, pageOfKind(ws.ID, title, made.Key, i)); err != nil {
			t.Fatal(err)
		}
	}

	for _, c := range []struct {
		what, key string
		want      int
	}{
		{"the products", made.Key, 3},
		{"the pages, and only the pages", kind.Page, 1},
		{"the posts", kind.Post, 1},
	} {
		got, err := h.kinds.Count(ctx, ws.ID, c.key)
		if err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		if got != c.want {
			t.Errorf("%s: %d, want %d", c.what, got, c.want)
		}
	}

	// A page in the trash is not content the operator has, on this screen as on
	// every other.
	all, _, err := h.pages.ListPages(ctx, ws.ID, page.ListFilter{Locale: "*", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range all {
		if p.TypeKey == made.Key {
			if err := h.pages.TrashPage(ctx, p.ID); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if got, _ := h.kinds.Count(ctx, ws.ID, made.Key); got != 2 {
		t.Errorf("after trashing one product the count is %d, want 2", got)
	}
}

// A block kind is created and the operator is taken straight to its fields: a
// kind without any is a block that renders to nothing.
func TestANewBlockKindLeadsStraightToItsFields(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	ctx := context.Background()

	rec, bad, _ := albumFlash(t, h, sm, h.HandleBlockTypeSave, postForm(
		"/admin/websites/1/bausteinarten",
		url.Values{"name": {"Merkkasten"}, "hinweis": {"Ein Kasten mit einem Hinweis"}}, kindRoute(ws.ID)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}
	types, err := h.blockTypes.List(ctx, ws.ID)
	if err != nil || len(types) != 1 {
		t.Fatalf("List: %v (%d)", err, len(types))
	}
	if got := rec.Header().Get("Location"); !strings.Contains(got, "feld") {
		t.Errorf("a new block kind did not lead to its fields: %q", got)
	}
}

// The keys of the built-in block kinds are not available, and neither is a key
// already taken.
func TestABlockKindDoesNotTakeAKeyThatIsSpokenFor(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	ctx := context.Background()

	serve(t, h, sm, h.HandleBlockTypeSave, postForm("/admin/websites/1/bausteinarten",
		url.Values{"name": {"Merkkasten"}}, kindRoute(ws.ID)))

	// Each refusal is asserted on ITS OWN sentence, not on "an error came back".
	// The handler's last arm answers "Saving failed." for anything it does not
	// recognise, so a taken key and a reserved key both still produce SOME
	// error with their own branch removed — and the operator reads a shrug
	// instead of the one thing that would tell them what to change.
	cases := []struct{ what, name, want string }{
		{"a key already taken", "Merkkasten",
			"A block kind with this key already exists. Choose another name."},
	}
	// Every built-in kind, by its own key rather than by a list written out
	// here — the two must not drift. "Zitat" is one of them, which is how this
	// test found out that its first draft was picking a reserved name.
	for _, b := range block.Kinds {
		cases = append(cases, struct{ what, name, want string }{
			"the built-in " + b.Type, b.Type,
			"This key belongs to a built-in block kind. Choose another name."})
	}

	for _, c := range cases {
		rec, bad, _ := albumFlash(t, h, sm, h.HandleBlockTypeSave,
			postForm("/admin/websites/1/bausteinarten", url.Values{"name": {c.name}},
				kindRoute(ws.ID)))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status %d", c.what, rec.Code)
		}
		if bad != c.want {
			t.Errorf("%s: answered %q, want %q", c.what, bad, c.want)
		}
	}
	if types, _ := h.blockTypes.List(ctx, ws.ID); len(types) != 1 {
		t.Errorf("%d block kinds after the refusals, want 1", len(types))
	}
}

// Removing a block kind says what did NOT happen: the blocks are still on the
// pages, invisible, until each page is next saved.
func TestRemovingABlockKindSaysWhatStaysOnThePages(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	ctx := context.Background()

	serve(t, h, sm, h.HandleBlockTypeSave, postForm("/admin/websites/1/bausteinarten",
		url.Values{"name": {"Merkkasten"}}, kindRoute(ws.ID)))
	types, _ := h.blockTypes.List(ctx, ws.ID)
	if len(types) != 1 {
		t.Fatalf("%d block kinds", len(types))
	}

	rec, bad, good := albumFlash(t, h, sm, h.HandleBlockTypeDelete,
		postForm("/admin/websites/1/bausteinarten/1/delete", nil,
			kindRoute(ws.ID, "typeID", strconv.FormatInt(types[0].ID, 10))))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}
	if !strings.Contains(good, "saved") && !strings.Contains(good, "gespeichert") {
		t.Errorf("the message does not say when the blocks go: %q", good)
	}
	if types, _ := h.blockTypes.List(ctx, ws.ID); len(types) != 0 {
		t.Error("the block kind is still there")
	}
}

// Both list screens render, and both answer 404 for a website that is not
// there rather than 500.
func TestTheKindListsShowWhatIsThere(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)

	serve(t, h, sm, h.HandleKindSave, postForm("/admin/websites/1/inhaltsarten",
		url.Values{"name": {"Produkt"}, "mehrzahl": {"Produkte"}}, kindRoute(ws.ID)))
	serve(t, h, sm, h.HandleBlockTypeSave, postForm("/admin/websites/1/bausteinarten",
		url.Values{"name": {"Merkkasten"}}, kindRoute(ws.ID)))

	for _, c := range []struct {
		what, want string
		fn         func(http.ResponseWriter, *http.Request) error
	}{
		{"the content kinds", "Produkt", h.HandleKindList},
		{"the block kinds", "Merkkasten", h.HandleBlockTypeList},
	} {
		req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/x", nil)
		req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
		rec := serve(t, h, sm, c.fn, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", c.what, rec.Code)
			continue
		}
		if !strings.Contains(rec.Body.String(), c.want) {
			t.Errorf("%s does not show %q", c.what, c.want)
		}

		unknown := httptest.NewRequest(http.MethodGet, "/admin/websites/999/x", nil)
		unknown.SetPathValue("id", "999")
		if rec := serve(t, h, sm, c.fn, unknown); rec.Code != http.StatusNotFound {
			t.Errorf("%s for an unknown website: status %d, want 404", c.what, rec.Code)
		}
	}
	_ = kind.MaxTypes
}

// pageOfKind is one entry of a website's own kind.
func pageOfKind(websiteID int64, title, key string, i int) page.PageCreate {
	return page.PageCreate{
		WebsiteID: websiteID, Title: title,
		Slug:     page.Slugify(title) + "-" + strconv.Itoa(i),
		Markdown: "x", HTML: "<p>x</p>", Status: "published",
		// Stored the way the editor stores it, which setKind spells out: an
		// entry of a website's own kind IS a page as far as routing and
		// rendering go, and the key lives in TypeKey beside it. Writing the key
		// into Kind instead would be a fixture that no screen produces —
		// NormalizeKind turns anything that is not "post" into "page" anyway.
		Kind: page.KindPage, TypeKey: key,
	}
}

// flashes is all three, because these screens use the warning as a distinct
// answer: "removed, and here is what stayed behind" is neither a success nor an
// error and must not be flattened into one.
type flashes struct{ err, success, warning string }

func albumFlashWarn(t *testing.T, h *Handler, sm *scs.SessionManager,
	fn func(http.ResponseWriter, *http.Request) error, req *http.Request,
) (*httptest.ResponseRecorder, error, flashes) {
	t.Helper()
	rec := httptest.NewRecorder()
	var handlerErr error
	var f flashes
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerErr = fn(w, r)
		f.err = sm.GetString(r.Context(), auth.SessionKeyFlashError)
		f.success = sm.GetString(r.Context(), auth.SessionKeyFlashSuccess)
		f.warning = sm.GetString(r.Context(), auth.SessionKeyFlashWarning)
	})).ServeHTTP(rec, req)
	if handlerErr != nil {
		t.Fatalf("handler: %v", handlerErr)
	}
	return rec, nil, f
}
