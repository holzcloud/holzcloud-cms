// What this file guards are three sentences a person is entitled to believe.
//
// The dry run writes nothing. It is the same function the write calls, and the
// write is suppressed by not being reached rather than by a second decision
// somewhere that could drift from the first — and the one that drifts is always
// the one the operator was not shown.
//
// A cell reading "nein" does not import as true. field.Check has no case for
// the janein kind at all, so any non-empty value passes it, and both readers of
// a stored boolean treat anything that is neither empty nor "0" as true. Left
// to the validator, a column of noes would import as a column of yeses through
// a check that says the row is fine.
//
// A row that fails halfway leaves no page behind. The terms of a page are
// written after the page is, in their own transaction, and a failure there is
// undone through the ordinary delete path.
//
// A real migrated database and no http.Request anywhere: the row function is
// build-order step 3, which is still before the first screen.
package csvimport_test

import (
	"context"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/csv"
	"github.com/holzcloud/holzcloud-cms/internal/csvimport"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/term"
)

// reader parses a whole file the way the importer will, so a test's input is a
// CSV file and not a hand-built Row: a quoted cell holding a real line break is
// one of the cases under test and cannot be written any other way.
func reader(t *testing.T, text string) ([]string, []csv.Row) {
	t.Helper()
	r, err := csv.New(strings.NewReader(text))
	if err != nil {
		t.Fatalf("csv.New: %v", err)
	}
	var rows []csv.Row
	for {
		row, ok := r.Next()
		if !ok {
			return r.Header(), rows
		}
		rows = append(rows, row)
	}
}

func fieldDef(id int64, position int, key, label, kind string, choices ...string) field.Def {
	return field.Def{ID: id, Position: position, Key: key, Label: label, Kind: kind, Choices: choices}
}

func writer(database *db.DB) csvimport.Writer {
	return csvimport.Writer{Pages: page.NewStore(database), Terms: term.NewStore(database)}
}

func countPages(t *testing.T, database *db.DB) int {
	t.Helper()
	var n int
	if err := database.Read.QueryRow(`SELECT COUNT(*) FROM pages`).Scan(&n); err != nil {
		t.Fatalf("count pages: %v", err)
	}
	return n
}

// TestDryRunWritesNothing (IMP-05): CheckRow decides and touches no store.
func TestDryRunWritesNothing(t *testing.T) {
	_, database, _, websiteID := setup(t)
	before := countPages(t, database)

	head, rows := reader(t, "Titel,Text\nApfel,Ein Baum\nBirne,Noch einer\n")
	m := csvimport.AutoMap(head, nil)
	for _, row := range rows {
		v, create, _ := csvimport.CheckRow(nil, row, m, nil, csvimport.CollisionSkip)
		if v.Outcome != csvimport.OutcomeCreate || v.Reason != "" {
			t.Errorf("row %d: %s / %q, want a clean create", v.Row, v.Outcome, v.Reason)
		}
		if create.Title == "" {
			t.Errorf("row %d: the create carries no title", v.Row)
		}
		if create.WebsiteID != 0 {
			t.Errorf("row %d: CheckRow filled in a website id, which only the writer knows", v.Row)
		}
	}
	if after := countPages(t, database); after != before {
		t.Errorf("the dry run wrote %d pages, want 0", after-before)
	}
	_ = websiteID
}

// TestBoolKnowsAClosedVocabulary (D-19): the trap, asserted from the failing
// side and not only from the happy one.
func TestBoolKnowsAClosedVocabulary(t *testing.T) {
	defs := []field.Def{fieldDef(1, 1, "verfuegbar", "Verfuegbar", field.KindBool)}

	cases := []struct {
		cell   string
		stored string
	}{
		{"ja", "1"}, {"Ja", "1"}, {"yes", "1"}, {"wahr", "1"}, {"true", "1"}, {"1", "1"}, {"x", "1"}, {"X", "1"},
		{"nein", ""}, {"Nein", ""}, {"no", ""}, {"falsch", ""}, {"false", ""}, {"0", ""}, {"", ""},
	}
	for _, c := range cases {
		head, rows := reader(t, "Titel,Verfuegbar\nApfel,"+c.cell+"\n")
		m := csvimport.AutoMap(head, defs)
		v, _, data := csvimport.CheckRow(defs, rows[0], m, nil, csvimport.CollisionSkip)
		if v.Outcome != csvimport.OutcomeCreate {
			t.Fatalf("cell %q: %s / %q, want a create", c.cell, v.Outcome, v.Reason)
		}
		if got := data.Values["verfuegbar"]; got != c.stored {
			t.Errorf("cell %q stored %q, want %q", c.cell, got, c.stored)
		}
	}

	// Anything outside the vocabulary is a reported row and never a guess.
	head, rows := reader(t, "Titel,Verfuegbar\nApfel,vielleicht\n")
	m := csvimport.AutoMap(head, defs)
	v, _, _ := csvimport.CheckRow(defs, rows[0], m, nil, csvimport.CollisionSkip)
	if v.Outcome != csvimport.OutcomeSkip || v.Reason != csvimport.ReasonBoolUnreadable {
		t.Errorf("an unreadable boolean gave %s / %q, want a skip with %q", v.Outcome, v.Reason, csvimport.ReasonBoolUnreadable)
	}
}

// TestMultiChoiceTravelsOnThePipe (D-21): the pipe is the wire format, and the
// value reaches the slot through field.JoinValues and never through a newline
// the importer builds itself.
func TestMultiChoiceTravelsOnThePipe(t *testing.T) {
	defs := []field.Def{fieldDef(1, 1, "farben", "Farben", field.KindMulti, "rot", "blau")}

	head, rows := reader(t, "Titel,Farben\nApfel,rot|blau\n")
	m := csvimport.AutoMap(head, defs)
	_, _, data := csvimport.CheckRow(defs, rows[0], m, nil, csvimport.CollisionSkip)
	if got := field.SplitValues(data.Values["farben"]); len(got) != 2 || got[0] != "rot" || got[1] != "blau" {
		t.Errorf("rot|blau read back as %v, want two values", got)
	}

	// An empty entry falls away and a duplicate is kept: JoinValues sorts
	// nothing and removes nothing, so the same file imported twice produces the
	// same string byte for byte.
	head, rows = reader(t, "Titel,Farben\nApfel,rot|rot|\n")
	m = csvimport.AutoMap(head, defs)
	_, _, data = csvimport.CheckRow(defs, rows[0], m, nil, csvimport.CollisionSkip)
	if got := field.SplitValues(data.Values["farben"]); len(got) != 2 {
		t.Errorf("rot|rot| read back as %v, want two values", got)
	}

	// A real line break inside a quoted cell is legal RFC 4180 and invisible in
	// a spreadsheet. It stays ONE value, because field.JoinValues folds it to a
	// space — the Phase 7 hardening whose doc comment names this caller,
	// asserted here rather than trusted. The option list is written to contain
	// the folded spelling, so what is measured is the number of values and not
	// the validator's opinion of them.
	folded := []field.Def{fieldDef(1, 1, "farben", "Farben", field.KindMulti, "rot blau")}
	head, rows = reader(t, "Titel,Farben\nApfel,\"rot\nblau\"\n")
	m = csvimport.AutoMap(head, folded)
	_, _, data = csvimport.CheckRow(folded, rows[0], m, nil, csvimport.CollisionSkip)
	if got := field.SplitValues(data.Values["farben"]); len(got) != 1 {
		t.Errorf("a cell with an embedded line break read back as %v, want one value", got)
	}

	// And against the real option list the row is refused rather than importing
	// a colour nobody chose: one value that is not on the list, never two that
	// are.
	head, rows = reader(t, "Titel,Farben\nApfel,\"rot\nblau\"\n")
	m = csvimport.AutoMap(head, defs)
	v, _, data := csvimport.CheckRow(defs, rows[0], m, nil, csvimport.CollisionSkip)
	if v.Outcome != csvimport.OutcomeSkip || v.Reason != csvimport.ReasonFieldRejected {
		t.Errorf("the folded cell gave %s / %q, want a skip with %q", v.Outcome, v.Reason, csvimport.ReasonFieldRejected)
	}
	if got := field.SplitValues(data.Values["farben"]); len(got) > 1 {
		t.Errorf("the folded cell minted %v, want no more than one value", got)
	}
}

// TestTermNameBecomesASlug (D-20): the cell holds a name, the slot holds a
// slug, and the two derivations agree by construction.
func TestTermNameBecomesASlug(t *testing.T) {
	_, database, _, websiteID := setup(t)
	defs := []field.Def{fieldDef(1, 1, "kategorie", "Kategorie", field.KindTerm)}
	terms := term.NewStore(database)

	// The name is written with escapes so this file carries no literal umlaut:
	// the first row spells it composed, the second decomposed and in lower
	// case, and the two must reach one term.
	head, rows := reader(t, "Titel,Kategorie\nApfel,M\u00F6bel\nBirne,mo\u0308bel\n")
	m := csvimport.AutoMap(head, defs)

	names := csvimport.TermNames(defs, m, rows)
	if len(names) != 1 {
		t.Fatalf("TermNames returned %v, want one name for two spellings of it", names)
	}
	if _, err := terms.EnsureNames(context.Background(), websiteID, names); err != nil {
		t.Fatalf("EnsureNames: %v", err)
	}

	for _, row := range rows {
		_, _, data := csvimport.CheckRow(defs, row, m, nil, csvimport.CollisionSkip)
		if got := data.Values["kategorie"]; got != "moebel" {
			t.Errorf("row %d stored %q, want %q", row.Number, got, "moebel")
		}
	}

	all, err := terms.ListAll(context.Background(), websiteID)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("the website carries %d terms, want 1", len(all))
	}
	if all[0].Name != "M\u00F6bel" {
		t.Errorf("the term is called %q, want the spelling the first row used", all[0].Name)
	}
}

// TestTermNamesOncePerFile (D-20): the caller runs EnsureNames once, before the
// row loop, which is internal/bundle/import.go:289-329's order.
func TestTermNamesOncePerFile(t *testing.T) {
	defs := []field.Def{fieldDef(1, 1, "kategorie", "Kategorie", field.KindTerm)}
	head, rows := reader(t, "Titel,Kategorie,Schlagworte\nApfel,Obst,gruen|rot\nBirne,Obst,gruen\n")
	m := csvimport.AutoMap(head, defs)

	names := csvimport.TermNames(defs, m, rows)
	want := []string{"Obst", "gruen", "rot"}
	if len(names) != len(want) {
		t.Fatalf("TermNames returned %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("name %d is %q, want %q — first appearance decides the order", i, names[i], want[i])
		}
	}
}

// TestRenamedSlugIsReported (D-23, IMP-02 idempotency): CreatePage renames on
// collision and returns the page it made; created.Slug != wanted is the whole
// test.
func TestRenamedSlugIsReported(t *testing.T) {
	_, database, userID, websiteID := setup(t)
	w := writer(database)
	ctx := context.Background()

	head, rows := reader(t, "Titel\nApfel\nApfel\n")
	m := csvimport.AutoMap(head, nil)

	first := w.WriteRow(ctx, websiteID, nil, rows[0], m, nil, csvimport.CollisionSkip, &userID)
	if first.Outcome != csvimport.OutcomeCreate || first.Reason != "" {
		t.Fatalf("the first row gave %s / %q, want a clean create", first.Outcome, first.Reason)
	}

	// The second row is written WITHOUT being told the page exists, which is
	// what happens when the collision choice is not the lookup key: the
	// database renames rather than refusing.
	second := w.WriteRow(ctx, websiteID, nil, rows[1], m, nil, csvimport.CollisionSkip, &userID)
	if second.Outcome != csvimport.OutcomeCreate {
		t.Fatalf("the second row gave %s / %q, want a create under another address", second.Outcome, second.Reason)
	}
	if second.Reason != csvimport.ReasonRenamed {
		t.Fatalf("the second row carries %q, want %q", second.Reason, csvimport.ReasonRenamed)
	}
	if len(second.Args) != 2 || second.Args[0] != "apfel" || second.Args[1] != "apfel-2" {
		t.Errorf("the rename is reported as %v, want [apfel apfel-2]", second.Args)
	}

	p, err := page.NewStore(database).GetPageBySlug(ctx, websiteID, "apfel-2")
	if err != nil || p == nil {
		t.Fatalf("the renamed page is not in the database: %v", err)
	}
}

// TestExistingPageIsUpdatedOrSkipped (IMP-04 adjacency): both arms, and the
// absence of a third.
func TestExistingPageIsUpdatedOrSkipped(t *testing.T) {
	_, database, userID, websiteID := setup(t)
	w := writer(database)
	pages := page.NewStore(database)
	ctx := context.Background()

	head, rows := reader(t, "Titel,Text\nApfel,Zweiter Text\n")
	m := csvimport.AutoMap(head, nil)

	existing, err := pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: websiteID, Title: "Apfel", Slug: "apfel",
		Markdown: "Erster Text", HTML: "<p>Erster Text</p>", Status: "draft",
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	skipped := w.WriteRow(ctx, websiteID, nil, rows[0], m, existing, csvimport.CollisionSkip, &userID)
	if skipped.Outcome != csvimport.OutcomeSkip || skipped.Reason != csvimport.ReasonExistingSkipped {
		t.Errorf("the skip arm gave %s / %q, want a skip with %q", skipped.Outcome, skipped.Reason, csvimport.ReasonExistingSkipped)
	}
	after, _ := pages.GetPage(ctx, existing.ID)
	if after.ContentMarkdown != "Erster Text" {
		t.Errorf("the skipped page now reads %q, want it untouched", after.ContentMarkdown)
	}

	updated := w.WriteRow(ctx, websiteID, nil, rows[0], m, after, csvimport.CollisionUpdate, &userID)
	if updated.Outcome != csvimport.OutcomeUpdate {
		t.Fatalf("the update arm gave %s / %q, want an update", updated.Outcome, updated.Reason)
	}
	after, _ = pages.GetPage(ctx, existing.ID)
	if after.ContentMarkdown != "Zweiter Text" {
		t.Errorf("the updated page reads %q, want the file's text", after.ContentMarkdown)
	}
	if after.Slug != "apfel" {
		t.Errorf("the update moved the page to %q, want its own address kept", after.Slug)
	}
	if countPages(t, database) != 1 {
		t.Errorf("the update created a second page — there is no third behaviour")
	}
}

// TestUnmappedColumnsAreLeftAlone (IMP-04): an import that maps two columns
// must not blank the other twenty of a page it updates.
func TestUnmappedColumnsAreLeftAlone(t *testing.T) {
	_, database, userID, websiteID := setup(t)
	w := writer(database)
	pages := page.NewStore(database)
	ctx := context.Background()

	existing, err := pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: websiteID, Title: "Apfel", Slug: "apfel",
		Markdown: "Erster Text", HTML: "<p>Erster Text</p>", Status: "published",
		Kind: page.KindPost, Meta: page.PageMeta{Excerpt: "Kurz"},
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	// A file that carries a title and nothing else.
	head, rows := reader(t, "Titel\nApfelbaum\n")
	m := csvimport.AutoMap(head, nil)
	if v := w.WriteRow(ctx, websiteID, nil, rows[0], m, existing, csvimport.CollisionUpdate, &userID); v.Outcome != csvimport.OutcomeUpdate {
		t.Fatalf("the update gave %s / %q", v.Outcome, v.Reason)
	}

	after, _ := pages.GetPage(ctx, existing.ID)
	if after.Title != "Apfelbaum" {
		t.Errorf("the title is %q, want the file's", after.Title)
	}
	if after.ContentMarkdown != "Erster Text" {
		t.Errorf("an unmapped body was overwritten with %q", after.ContentMarkdown)
	}
	if after.Status != "published" {
		t.Errorf("an unmapped status became %q", after.Status)
	}
	if after.Kind != page.KindPost {
		t.Errorf("the update reclassified the page to %q", after.Kind)
	}
	if after.Excerpt != "Kurz" {
		t.Errorf("an unmapped excerpt became %q", after.Excerpt)
	}
}

// TestRowIsRolledBack (D-02): a row that fails after its page was created
// leaves no page behind, and it is undone through the ordinary delete path.
func TestRowIsRolledBack(t *testing.T) {
	_, database, userID, websiteID := setup(t)
	ctx := context.Background()

	// A term store over a database that is closed: SetForPage fails on its very
	// first statement, which is the residual window D-02 describes — the page
	// is already in, the terms are not, and nothing wraps the two.
	broken, err := db.Open(t.TempDir() + "/broken.sqlite")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := db.RunMigrations(broken.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	broken.Close()

	w := csvimport.Writer{Pages: page.NewStore(database), Terms: term.NewStore(broken)}

	head, rows := reader(t, "Titel,Schlagworte\nApfel,Obst\n")
	m := csvimport.AutoMap(head, nil)

	v := w.WriteRow(ctx, websiteID, nil, rows[0], m, nil, csvimport.CollisionSkip, &userID)
	if v.Outcome != csvimport.OutcomeSkip {
		t.Errorf("the failed row gave %s / %q, want a skip", v.Outcome, v.Reason)
	}
	if v.Reason == csvimport.ReasonNotRolledBack {
		t.Errorf("the compensation itself failed: %v", v.Args)
	}
	if n := countPages(t, database); n != 0 {
		t.Errorf("%d pages left behind, want none", n)
	}
}

// TestUpdateArmIsNotRolledBack (D-02, the branch the create path must NOT get):
// an update whose terms fail is reported and NOTHING is undone.
//
// Why this test exists at all, in the words of the failure it prevents:
// TrashPage then PurgePage undoes a row whose page was created here and whose
// terms then failed, and that is right, because the page came into being in
// this import. Applied to the update arm the same two calls would delete a page
// the operator ALREADY HAD — a page that existed before the import started,
// carrying content this file never supplied. That is not compensation, that is
// data loss caused by the recovery path.
//
// A missing branch is invisible to a reviewer: the code that must not run
// leaves no trace of not running. So it is asserted, and the assertion is that
// the page is still there afterwards with the update it did get.
func TestUpdateArmIsNotRolledBack(t *testing.T) {
	_, database, userID, websiteID := setup(t)
	ctx := context.Background()
	pages := page.NewStore(database)

	existing, err := pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: websiteID, Title: "Alt", Slug: "apfel",
		Markdown: "Ein Satz", HTML: "<p>Ein Satz</p>", Status: "draft", Kind: page.KindPage,
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	w := writer(database)
	head, rows := reader(t, "Titel,Schlagworte\nNeu,Obst\n")
	m := csvimport.AutoMap(head, nil)

	// A website id no row of websites carries. UpdatePage does not use it — the
	// page is addressed by its own id — but SetForPage inserts into terms,
	// whose website_id is a foreign key, so the terms step and only the terms
	// step fails. That is the residual window on the update arm, exactly.
	v := w.WriteRow(ctx, 4711, nil, rows[0], m, existing, csvimport.CollisionUpdate, &userID)

	if v.Outcome != csvimport.OutcomeUpdate {
		t.Errorf("the row gave %s, want an update: %v", v.Outcome, v.Args)
	}
	if v.Reason != csvimport.ReasonNotRolledBack {
		t.Fatalf("the row carries %q, want %q — the half-written state must be reported",
			v.Reason, csvimport.ReasonNotRolledBack)
	}

	after, err := pages.GetPage(ctx, existing.ID)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if after == nil {
		t.Fatal("the operator's own page was DELETED by the recovery path — that is the bug this test exists for")
	}
	if after.Title != "Neu" {
		t.Errorf("title = %q, want Neu: the update that did go through must stand", after.Title)
	}
	if n := countPages(t, database); n != 1 {
		t.Errorf("%d pages, want 1", n)
	}
}

// TestFileReadTwice (IMP-02 idempotency): the create path renames the second
// time, the update path rewrites the same values, and the verdict says which.
func TestFileReadTwice(t *testing.T) {
	_, database, userID, websiteID := setup(t)
	w := writer(database)
	pages := page.NewStore(database)
	ctx := context.Background()

	head, rows := reader(t, "Titel,Text\nApfel,Ein Baum\n")
	m := csvimport.AutoMap(head, nil)

	first := w.WriteRow(ctx, websiteID, nil, rows[0], m, nil, csvimport.CollisionSkip, &userID)
	if first.Outcome != csvimport.OutcomeCreate || first.Reason != "" {
		t.Fatalf("first pass: %s / %q", first.Outcome, first.Reason)
	}

	existing, _ := pages.GetPageBySlug(ctx, websiteID, "apfel")
	second := w.WriteRow(ctx, websiteID, nil, rows[0], m, existing, csvimport.CollisionUpdate, &userID)
	if second.Outcome != csvimport.OutcomeUpdate {
		t.Fatalf("second pass: %s / %q, want an update", second.Outcome, second.Reason)
	}
	if countPages(t, database) != 1 {
		t.Errorf("the second pass created a page instead of updating one")
	}
}

// TestImageColumnIsNotWritten (D-18): a refused kind reaches no slot even when
// a mapping names it, which is what a mapping arriving from a form can do.
func TestImageColumnIsNotWritten(t *testing.T) {
	defs := []field.Def{fieldDef(1, 1, "bild", "Bild", field.KindImage)}
	head, rows := reader(t, "Titel,Bild\nApfel,apfel.jpg\n")

	m := csvimport.AutoMap(head, defs)
	// Point the column at the picture field by hand, the way a tampered form
	// would.
	m.Targets[1] = csvimport.Target{Kind: csvimport.TargetField, Key: "bild"}

	v, _, data := csvimport.CheckRow(defs, rows[0], m, nil, csvimport.CollisionSkip)
	if v.Outcome != csvimport.OutcomeCreate {
		t.Fatalf("the row gave %s / %q", v.Outcome, v.Reason)
	}
	if got, ok := data.Values["bild"]; ok {
		t.Errorf("the picture field was written as %q, want nothing at all", got)
	}
}

// TestMissingDefinitionIsReported (D-29, IMP-04 concurrency): the field a
// mapping names can be deleted while the wizard is open.
func TestMissingDefinitionIsReported(t *testing.T) {
	head, rows := reader(t, "Titel,Sorte\nApfel,Boskoop\n")
	m := csvimport.AutoMap(head, []field.Def{fieldDef(1, 1, "sorte", "Sorte", field.KindText)})

	// The same mapping, re-checked against a website whose field is gone.
	v, create, data := csvimport.CheckRow(nil, rows[0], m, nil, csvimport.CollisionSkip)
	if v.Outcome != csvimport.OutcomeCreate {
		t.Fatalf("the row gave %s / %q, want the row to survive the loss of one column", v.Outcome, v.Reason)
	}
	if v.Reason != csvimport.ReasonFieldMissing || len(v.Args) != 1 || v.Args[0] != "sorte" {
		t.Errorf("the loss is reported as %q %v, want %q [sorte]", v.Reason, v.Args, csvimport.ReasonFieldMissing)
	}
	if create.Title != "Apfel" {
		t.Errorf("the title is %q, want the row to be importable without the lost column", create.Title)
	}
	if _, ok := data.Values["sorte"]; ok {
		t.Error("a value was written for a field that no longer exists")
	}
}

// TestStatusKnowsAClosedVocabulary: draft is the default and the safe
// direction, and anything outside the vocabulary is a reported row.
func TestStatusKnowsAClosedVocabulary(t *testing.T) {
	for cell, want := range map[string]string{
		"": "draft", "Entwurf": "draft", "draft": "draft", "offline": "draft",
		"published": "published", "online": "published",
		// Composed and decomposed, both written as escapes.
		"Ver\u00F6ffentlicht": "published", "Vero\u0308ffentlicht": "published",
	} {
		head, rows := reader(t, "Titel,Zustand\nApfel,"+cell+"\n")
		m := csvimport.AutoMap(head, nil)
		v, create, _ := csvimport.CheckRow(nil, rows[0], m, nil, csvimport.CollisionSkip)
		if v.Outcome != csvimport.OutcomeCreate {
			t.Fatalf("cell %q gave %s / %q", cell, v.Outcome, v.Reason)
		}
		if create.Status != want {
			t.Errorf("cell %q became %q, want %q", cell, create.Status, want)
		}
	}

	head, rows := reader(t, "Titel,Zustand\nApfel,halbfertig\n")
	m := csvimport.AutoMap(head, nil)
	v, _, _ := csvimport.CheckRow(nil, rows[0], m, nil, csvimport.CollisionSkip)
	if v.Outcome != csvimport.OutcomeSkip || v.Reason != csvimport.ReasonStatusUnknown {
		t.Errorf("an unknown status gave %s / %q, want a skip with %q", v.Outcome, v.Reason, csvimport.ReasonStatusUnknown)
	}
}

// TestRowWithoutATitleIsSkipped: a page with no title is a page nobody finds
// again, and a default fills in for an empty cell (IMP-08).
func TestRowWithoutATitleIsSkipped(t *testing.T) {
	head, rows := reader(t, "Titel,Text\n,Ein Baum\n")
	m := csvimport.AutoMap(head, nil)

	v, _, _ := csvimport.CheckRow(nil, rows[0], m, nil, csvimport.CollisionSkip)
	if v.Outcome != csvimport.OutcomeSkip || v.Reason != csvimport.ReasonNoTitle {
		t.Errorf("a row with no title gave %s / %q, want a skip with %q", v.Outcome, v.Reason, csvimport.ReasonNoTitle)
	}

	m.Defaults[csvimport.Target{Kind: csvimport.TargetTitle}.String()] = "Ohne Namen"
	v, create, _ := csvimport.CheckRow(nil, rows[0], m, nil, csvimport.CollisionSkip)
	if v.Outcome != csvimport.OutcomeCreate || create.Title != "Ohne Namen" {
		t.Errorf("the default gave %s / %q with the title %q", v.Outcome, v.Reason, create.Title)
	}
}

// TestUnreadableRowIsReported: the reader's own refusal travels as an argument
// rather than being re-decided here.
func TestUnreadableRowIsReported(t *testing.T) {
	row := csv.Row{Number: 4, Cells: []string{"Apfel"}, Error: "cell in column 1 is too big"}
	m := csvimport.AutoMap([]string{"Titel"}, nil)

	v, _, _ := csvimport.CheckRow(nil, row, m, nil, csvimport.CollisionSkip)
	if v.Outcome != csvimport.OutcomeSkip || v.Reason != csvimport.ReasonRowUnreadable {
		t.Fatalf("gave %s / %q, want a skip with %q", v.Outcome, v.Reason, csvimport.ReasonRowUnreadable)
	}
	if v.Row != 4 {
		t.Errorf("the verdict names row %d, want the number the reader minted", v.Row)
	}
	if len(v.Args) != 1 || v.Args[0] != row.Error {
		t.Errorf("the verdict carries %v, want the reader's own reason", v.Args)
	}
}

// TestRejectedFieldValueIsReported: field.CheckAll is the one validator, and
// its reason is carried rather than re-worded.
func TestRejectedFieldValueIsReported(t *testing.T) {
	defs := []field.Def{fieldDef(1, 1, "sorte", "Sorte", field.KindChoice, "Boskoop", "Gravensteiner")}
	head, rows := reader(t, "Titel,Sorte\nApfel,Klarapfel\n")
	m := csvimport.AutoMap(head, defs)

	v, _, _ := csvimport.CheckRow(defs, rows[0], m, nil, csvimport.CollisionSkip)
	if v.Outcome != csvimport.OutcomeSkip || v.Reason != csvimport.ReasonFieldRejected {
		t.Fatalf("gave %s / %q, want a skip with %q", v.Outcome, v.Reason, csvimport.ReasonFieldRejected)
	}
	if len(v.Args) != 2 || v.Args[0] != "Sorte" {
		t.Errorf("the verdict carries %v, want the field's label and Check's own reason", v.Args)
	}
}

// TestRowsAreDecidedInFileOrder (IMP-04 ordering): row 40 duplicating row 4
// gets the rename in that order, and not the other way round.
func TestRowsAreDecidedInFileOrder(t *testing.T) {
	_, database, userID, websiteID := setup(t)
	w := writer(database)
	ctx := context.Background()

	head, rows := reader(t, "Titel,Text\nApfel,Erster\nBirne,Zweiter\nApfel,Dritter\n")
	m := csvimport.AutoMap(head, nil)

	var verdicts []csvimport.Verdict
	for _, row := range rows {
		verdicts = append(verdicts, w.WriteRow(ctx, websiteID, nil, row, m, nil, csvimport.CollisionSkip, &userID))
	}
	if verdicts[0].Reason != "" {
		t.Errorf("row 2 carries %q, want nothing", verdicts[0].Reason)
	}
	if verdicts[2].Reason != csvimport.ReasonRenamed {
		t.Errorf("row 4 carries %q, want %q", verdicts[2].Reason, csvimport.ReasonRenamed)
	}
	p, _ := page.NewStore(database).GetPageBySlug(ctx, websiteID, "apfel")
	if p == nil || p.ContentMarkdown != "Erster" {
		t.Error("the first occurrence did not keep the address")
	}
}
