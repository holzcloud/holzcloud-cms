package csvimport

import (
	"context"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"sort"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/csv"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/term"
)

// The two collision settings, as migration 00049's CHECK constraint pins them
// on csv_imports.kollision.
//
// German, and staying German. These are values that stand in a database row,
// not identifiers: a released migration refuses anything else, so translating
// one here would compile cleanly and then have SQLite turn the row away at
// runtime, on a path no check of this phase travels.
const (
	// CollisionSkip: a page with this address already exists, leave it alone.
	CollisionSkip = "uebergehen"
	// CollisionUpdate: rewrite the page that is already there.
	CollisionUpdate = "aktualisieren"
)

// foldCell folds a cell the way the operator's spreadsheet may have spelled it.
//
// The same settling of combining marks a heading gets, then
// page.Transliterate, which lowercases and writes the accented letters out in
// ASCII. Transliterate and not field.SlugifyKey, because SlugifyKey is a key
// derivation: it prefixes a leading digit with 'f', so it would fold the cell
// "0" to "f0" and a column of noughts would stop being recognisable as a
// vocabulary word. A cell is not a key.
func foldCell(cell string) string {
	return page.Transliterate(settleMarks(cell))
}

// statusVocabulary is the closed set of things a status cell may say.
//
// Closed on purpose, and reported when a cell falls outside it. The direction
// of the default matters more than its size: an empty cell is a draft, so a
// file that says nothing about publication does not publish two hundred pages
// on a live website.
//
// The German spellings are folded already — foldCell has written "oe" for the
// umlaut by the time a lookup reaches this table — and they are words an
// operator types into a spreadsheet, which is data and not an identifier. The
// two stored values, "draft" and "published", are the ones the pages table
// already carries.
var statusVocabulary = map[string]string{
	"draft":           "draft",
	"entwurf":         "draft",
	"offline":         "draft",
	"published":       "published",
	"veroeffentlicht": "published",
	"oeffentlich":     "published",
	"online":          "published",
}

// parseStatus reads a status cell. An empty cell is a draft.
//
// That default is the CREATE arm's, and only the create arm's. The update arm
// asks whether the cell said anything at all before it writes a status, so this
// substitution never reaches a page that already exists — see update(), which
// carries the argument.
func parseStatus(cell string) (string, bool) {
	folded := foldCell(cell)
	if folded == "" {
		return "draft", true
	}
	status, ok := statusVocabulary[folded]
	return status, ok
}

// parseBool reads a janein cell against a closed vocabulary.
//
// This function exists because field.Check has NO case for the janein kind:
// any non-empty value passes it. And both readers of a stored boolean —
// internal/admin/page_fields.go:281 and internal/field/render.go:126 — take
// truth to be `value != "" && value != "0"`. Put together, a cell reading
// "nein" would be stored as the four letters n-e-i-n and read back as TRUE, by
// a validator that reported the row as fine. A whole column of noes would
// import as a column of yeses and nothing anywhere would say so.
//
// So the vocabulary is closed and a cell outside it is a reported row, never a
// guess. The stored spelling is "1" and the empty string, which is what the
// page form writes.
func parseBool(cell string) (string, bool) {
	// field.NormalizeBool and not a copy of its four words. This function used
	// to be the only reading of a boolean in the tree, which is why the gap it
	// covered was invisible everywhere else: the archive import and the
	// assistant stored what they were handed, and Resolve read "nein" as yes.
	// Now there is one reading, and it cannot drift from the one Check applies.
	//
	// foldCell stays in front of it: it transliterates and settles combining
	// marks, which nothing in this vocabulary needs but a cell out of a
	// spreadsheet might carry. NormalizeBool lowercases and trims on its own,
	// so the two overlap rather than fight.
	return field.NormalizeBool(foldCell(cell))
}

// termNames reads the names out of one multi-name cell.
//
// The pipe and not the newline: a newline inside a cell is legal RFC 4180 but
// has to be quoted and is invisible in a spreadsheet, so it is not a wire
// format an operator can see they have got wrong.
//
// Every name goes through settleMarks and then term.Normalize, and never
// through term.Parse. Parse is the reader of the field an editor types: it
// splits on commas and stops at term.MaxPerPage, which would tear a name with
// a comma in it in two. internal/bundle/import.go:305-318 writes that reasoning
// out for the same reason and this carries it.
//
// Duplicates fold together on the slug, because that is what decides whether
// two spellings are one term, and the first spelling wins — which is the same
// answer term.EnsureNames gives with its ON CONFLICT DO NOTHING.
func termNames(cell string) []string {
	var names []string
	seen := map[string]bool{}
	for _, part := range strings.Split(cell, "|") {
		name := term.Normalize(settleMarks(part))
		if name == "" {
			continue
		}
		slug := page.Slugify(name)
		if seen[slug] {
			continue
		}
		seen[slug] = true
		names = append(names, name)
	}
	return names
}

// RowTerms are the page's own terms for one row, and whether the cap cut them.
//
// The cap is term.MaxPerPage, which is an editorial limit and not a database
// one, and it is reported rather than silently applied.
//
// **This is the only place the cap is applied, and TermNames applies it by
// calling this function.** The two once carried the cap separately — this one
// cut at term.MaxPerPage, the pre-pass did not cut at all — and the gap between
// them was labels no row could point at, created inside one transaction on the
// single write connection. Written twice they drift again; written once they
// cannot.
func RowTerms(row csv.Row, m Mapping) ([]string, bool) {
	names := termNames(cellFor(row, m, Target{Kind: TargetTerms}))
	if len(names) > term.MaxPerPage {
		return names[:term.MaxPerPage], true
	}
	return names, false
}

// TermNames is every term name the rows of a file can actually store, once, in
// the order the file first mentions each.
//
// The caller runs term.EnsureNames with this ONCE, before the row loop, which
// is internal/bundle/import.go:289-329's order and not an optimisation: a term
// field's stored value is a slug, so the term it names has to exist before the
// value referring to it is written, or the page would carry an address that
// resolves to nothing.
//
// Both sources are read: the page's own terms column, whose cell holds several
// names separated by pipes, and every column pointed at a field of the
// schlagwort kind, whose cell holds one name standing on its own.
//
// **The cap here and the cap in RowTerms must agree, and they agree by calling
// the same function rather than by both naming term.MaxPerPage.** This is
// exactly where they drifted: RowTerms cut a row at term.MaxPerPage while this
// function read the whole cell, so a file could mint labels no row was able to
// point at — 1.17 million of them from one 10 MB upload, measured, every one of
// them an orphan and every one of them inside the single transaction
// term.EnsureNames opens. A second cap written out as a number would drift
// again the day the first one moves; a call to RowTerms cannot.
//
// A target is harvested ONCE even when two columns are pointed at it, for the
// same reason: cellFor reads the first column a target has (mapping.go:328), so
// the second one contributes nothing to the row and must contribute nothing
// here either.
//
// And a target with NO column is harvested too, when the operator typed a
// default for it. That is the same rule read from the other end — cellFor falls
// back to the default whether a column exists or not, so the row stores the
// name and the pre-pass owes it. Walking the columns alone had a file with no
// Schlagwörter column and two names typed into the Vorgaben card create four
// labels while the dry run promised two.
func TermNames(defs []field.Def, m Mapping, rows []csv.Row) []string {
	byKey := definitionsByKey(defs)

	// The targets a term can come from, in column order, each of them once.
	var sources []Target
	taken := map[string]bool{}
	remember := func(t Target) {
		if taken[t.String()] {
			return
		}
		taken[t.String()] = true
		sources = append(sources, t)
	}
	for _, t := range m.Targets {
		carries := t.Kind == TargetTerms ||
			(t.Kind == TargetField && byKey[t.Key].Kind == field.KindTerm)
		if carries {
			remember(t)
		}
	}
	// Then the ones the mapping screen alone supplies, after the columns so the
	// file's own order still leads.
	if terms := (Target{Kind: TargetTerms}); m.Defaults[terms.String()] != "" {
		remember(terms)
	}
	for _, d := range defs {
		if d.Kind != field.KindTerm {
			continue
		}
		if t := (Target{Kind: TargetField, Key: d.Key}); m.Defaults[t.String()] != "" {
			remember(t)
		}
	}

	var out []string
	seen := map[string]bool{}
	add := func(names []string) {
		for _, name := range names {
			slug := page.Slugify(name)
			if seen[slug] {
				continue
			}
			seen[slug] = true
			out = append(out, name)
		}
	}

	for _, row := range rows {
		// Left to right, so the order of the file is the order of the list.
		for _, t := range sources {
			if t.Kind == TargetTerms {
				// The page's own terms, through the very function that decides
				// what the page keeps. Whether the cap cut them is the row's
				// business to report (ReasonTermsTruncated) and not this
				// function's; what matters here is that the cut names are the
				// ones created.
				names, _ := RowTerms(row, m)
				add(names)
				continue
			}
			// One name standing on its own. Only the first is taken, because
			// the slot holds exactly one slug and inventing terms a row cannot
			// store would create labels nothing points at.
			if names := termNames(cellFor(row, m, t)); len(names) > 0 {
				add(names[:1])
			}
		}
	}
	return out
}

// definitionsByKey indexes the definitions a mapping may name.
// applicableFields is the definitions that belong on the page this row writes.
//
// The importer is the one reader of a website's own fields that never asked
// the question. Nine other call sites wrap the list in field.For, and the one
// that omits it deliberately writes down why; internal/csvimport said nothing
// about gilt_fuer at all. Two things went wrong at once: a REQUIRED field
// scoped to Beiträge refused every row of every import, naming a field the
// page form never asks for — and a column pointed at such a field stored a
// value the theme filters out and the next save from the page form drops
// without a word.
//
// The kind is the EXISTING page's on an update and page.KindPage on a create,
// which is the whole reason this takes `existing` rather than a constant.
// CheckRow builds a PageCreate with Kind: page.KindPage and no content-type
// key, so a created page has exactly the built-in page's fields; but a row
// UPDATING a post has to be checked and cleaned against the post's, or the
// cleaning would drop every post-only value the page already carries. That is
// the opposite mistake and the worse one, because it destroys data instead of
// refusing to write it.
func applicableFields(defs []field.Def, existing *page.Page) []field.Def {
	kind := page.KindPage
	if existing != nil {
		kind = existing.Kind
		if existing.TypeKey != "" {
			kind = existing.TypeKey
		}
	}
	return field.For(defs, kind)
}

func definitionsByKey(defs []field.Def) map[string]field.Def {
	byKey := make(map[string]field.Def, len(defs))
	for _, d := range defs {
		byKey[d.Key] = d
	}
	return byKey
}

// cellFor reads the cell a target is pointed at, falling back to the target's
// default where the target may carry one.
//
// The default belongs to the target and not to the column (IMP-08), which is
// why it is looked up by Target.String() and why two columns pointed at one
// field cannot carry two defaults for one slot.
//
// TakesDefault is asked rather than assumed, so that this reader and the
// mapping screen answer the same question from the same rule. It also keeps
// TargetNone out: a column pointed at nothing has no slot to fill, and
// m.Defaults["none"] would be a key nothing could ever read back.
func cellFor(row csv.Row, m Mapping, t Target) string {
	if i := m.ColumnFor(t.Kind, t.Key); i >= 0 && i < len(row.Cells) {
		if cell := strings.TrimSpace(row.Cells[i]); cell != "" {
			return cell
		}
	}
	if !TakesDefault(t) {
		return ""
	}
	return strings.TrimSpace(m.Defaults[t.String()])
}

// RowSlug is the address one row wants, before the database has had a say.
//
// Exported because of an ordering the caller cannot get around: CheckRow is
// told whether a page with this address already exists, so the caller has to
// look that page up FIRST — and it has to look it up under the very address
// CheckRow is about to derive. A second derivation written in the handler would
// be a second answer to "which page does this row mean", and the two would
// drift on the first title carrying an umlaut. So there is one derivation, and
// the caller asks it rather than copying it.
//
// The address as the file gave it, or derived from the title. Deliberately NOT
// de-duplicated here: page.CreatePage appends -2, -3 and hands back the page it
// made, and reporting that rename is what criterion 4 asks for by name (D-23).
func RowSlug(row csv.Row, m Mapping) string {
	if slug := cellFor(row, m, Target{Kind: TargetSlug}); slug != "" {
		return slug
	}
	return page.Slugify(cellFor(row, m, Target{Kind: TargetTitle}))
}

// CheckRow decides what happens to one row, and touches no store at all.
//
// It is the one decision. The dry run calls it and shows the operator what it
// says; the write calls it and then writes. The write is suppressed by not
// being reached — Verdict.Written() is false — and never by a second decision
// somewhere else, because two functions deciding the same thing drift, and the
// one that drifts is always the one the operator was not shown (D-22).
//
// Everything it needs is passed in: the definitions as the caller has just
// re-read them, the row, the mapping, and the page that already holds this
// address, or nil. Nothing is looked up, so the dry run costs one read of the
// page list and not one query per row.
//
// It returns the verdict together with everything a write would need — a
// page.PageCreate lacking only the website and the user, which are the writer's
// to know, and the field data as field.Clean left it.
// lang is the operator's language. It is here, and not further out, because
// field.CheckAll now hands back a Reason — a format plus its arguments — rather
// than a finished German sentence, and a Verdict's Args are strings by design:
// GroupKey folds two rows that say the same thing into one line of the report,
// and it can only do that if the thing they say is a string. So the sentence is
// rendered exactly once, here, in the language of the person who will read the
// report. One run is one language, so the folding is unaffected.
func CheckRow(defs []field.Def, row csv.Row, m Mapping, existing *page.Page, collision, lang string) (Verdict, page.PageCreate, field.Data) {
	skip := func(reason Reason, args ...string) (Verdict, page.PageCreate, field.Data) {
		return Verdict{Row: row.Number, Outcome: OutcomeSkip, Reason: reason, Args: args}, page.PageCreate{}, field.Data{}
	}

	// A cell over the reader's limit is named with its column and its size
	// before the reader's own technical sentence is fallen back on: the report
	// can say which column and how big, and both of those are arguments a
	// template substitutes rather than words this file would have to write.
	for i, cell := range row.Cells {
		if len(cell) > csv.MaxCellBytes {
			return skip(ReasonCellTooLong, strconv.Itoa(i+1), strconv.Itoa(len(cell)))
		}
	}
	// More cells than the header has columns, as two numbers rather than as a
	// sentence built in Go: the reader carries the header's width on the row
	// so this can be asked without the header, and the sentence the operator
	// reads is a {{tf}} literal in csv_reason.html (D-32). HeaderWidth is zero
	// on a row no reader produced, and zero means "not known" rather than
	// "no columns".
	if row.HeaderWidth > 0 && len(row.Cells) > row.HeaderWidth {
		return skip(ReasonRowTooWide, strconv.Itoa(len(row.Cells)), strconv.Itoa(row.HeaderWidth))
	}
	if row.Error != "" {
		return skip(ReasonRowUnreadable, row.Error)
	}

	// note carries the first thing worth saying about a row that still went
	// through. A verdict says one thing, so the first is kept and a harder
	// refusal below replaces it by returning outright.
	var note Note
	remember := func(reason Reason, args ...string) {
		if note.Reason == "" {
			note = Note{Reason: reason, Args: args}
		}
	}

	title := cellFor(row, m, Target{Kind: TargetTitle})
	if title == "" {
		// A page with no title is a page nobody finds again, and the admin
		// form refuses one for the same reason.
		return skip(ReasonNoTitle)
	}

	// The same derivation the caller used to find `existing`, and the one place
	// it is written. Carrying wordpress.go:113-126's `seen` map into this
	// importer would hide exactly what criterion 4 asks to be told.
	slug := RowSlug(row, m)
	if err := page.ValidateSlug(slug); err != nil {
		return skip(ReasonSlugInvalid, slug)
	}

	rawStatus := cellFor(row, m, Target{Kind: TargetStatus})
	status, ok := parseStatus(rawStatus)
	if !ok {
		return skip(ReasonStatusUnknown, rawStatus)
	}

	// The body is the page's Markdown source and goes through the one render
	// path — goldmark, then bluemonday — exactly as wordpress.go:128 does it.
	// Never a second pipeline, and no template.HTML cast anywhere on this road.
	markdown := cellFor(row, m, Target{Kind: TargetBody})
	html, err := page.RenderMarkdown(markdown)
	if err != nil {
		return skip(ReasonBodyUnreadable, err.Error())
	}

	// Which fields belong on the page this row writes. Everything below —
	// the loop, field.CheckAll and field.Clean — reads this list and not the
	// website's whole one.
	mine := applicableFields(defs, existing)

	// D-29 first, and against the UNFILTERED list. The wizard spans four
	// requests and an unknown amount of wall time, so the field a mapping names
	// can be deleted while it is open: the column falls back to unmapped and
	// the row says so, which is a reported row instead of a nil dereference.
	//
	// Against the unfiltered list, because "this field no longer exists" and
	// "this field does not belong on this page" are two different answers. The
	// first is worth a line in the report; the second is the mapping screen
	// offering a target it cannot know is inapplicable to this particular row,
	// and it passes over in silence.
	all := definitionsByKey(defs)
	done := map[string]bool{}
	for _, t := range m.Targets {
		if t.Kind != TargetField || done[t.Key] {
			continue
		}
		done[t.Key] = true
		if _, exists := all[t.Key]; !exists {
			remember(ReasonFieldMissing, t.Key)
		}
	}

	// Over the DEFINITIONS and not over the columns.
	//
	// m.Targets holds exactly one entry per column, so a loop over it can only
	// visit a field some column points at — and the Vorgaben card promises the
	// opposite in as many words: "Was eingetragen wird, wenn die Datei für
	// dieses Ziel keine Spalte hat." cellFor already falls back to the target's
	// default, which is how the five fixed targets have always kept that
	// promise. Walking the definitions is what extends it to the website's own
	// fields, where the box was offered, accepted, carried through to the
	// commit, and never written.
	data := field.Data{Values: field.Values{}, Rows: map[string][]field.Values{}}
	for _, d := range mine {
		if !Mappable(d.Kind) {
			// A mapping arriving from a form can name a kind no column may
			// feed. Refused here as well as on the screen, because the screen
			// is not what makes it true.
			continue
		}

		cell := cellFor(row, m, Target{Kind: TargetField, Key: d.Key})
		switch d.Kind {
		case field.KindBool:
			stored, readable := parseBool(cell)
			if !readable {
				return skip(ReasonBoolUnreadable, d.Label, cell)
			}
			data.Values[d.Key] = stored
		case field.KindMulti:
			// Through field.JoinValues and never through a newline built here.
			// JoinValues was hardened for this caller in Phase 7 and its doc
			// comment says so in as many words: it is the boundary behind
			// which a caller cannot mint an additional value. Building the
			// string directly is going around the fix.
			data.Values[d.Key] = field.JoinValues(strings.Split(cell, "|"))
		case field.KindTerm:
			// The cell holds a NAME and the slot holds a slug. term.Normalize
			// then page.Slugify, which is internal/bundle/import.go:573-583
			// exactly: the two derivations agree by construction and not by
			// luck, because term.EnsureNames derives its own slug the same way.
			// EnsureNames itself runs once for the whole file, before the row
			// loop, and not here.
			if names := termNames(cell); len(names) > 0 {
				data.Values[d.Key] = page.Slugify(names[0])
			}
		default:
			data.Values[d.Key] = cell
		}
	}

	// The same validator the admin form, the bundle import and the assistant
	// use. Three copies of these rules would be three chances for one of them
	// to be laxer than the others, and the lax one is the one that gets used —
	// and it is what makes the dry run trustworthy at no cost at all.
	//
	// The reason it hands back used to be an untranslated German string built
	// by concatenation, carried here as an argument because Phase 9 was not
	// allowed to change internal/field. v2.0 changed it: CheckAll returns a
	// field.Reason — a collected format plus its arguments — and the sentence
	// is assembled below in the operator's language.
	if errs := field.CheckAll(mine, data); len(errs) > 0 {
		for _, d := range mine {
			if reason := errs[d.Key]; !reason.Empty() {
				return skip(ReasonFieldRejected, d.Label, reason.Text(lang))
			}
		}
		// A key belonging to a row of a group. Sorted, so a row that is refused
		// is refused for the same reason every time it is checked.
		keys := make([]string, 0, len(errs))
		for key := range errs {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return skip(ReasonFieldRejected, keys[0], errs[keys[0]].Text(lang))
	}

	if _, cut := RowTerms(row, m); cut {
		remember(ReasonTermsTruncated, strconv.Itoa(term.MaxPerPage))
	}

	// Clean before encoding, the same as the form path.
	data = field.Clean(mine, data)
	fields, err := field.Encode(data)
	if err != nil {
		return skip(ReasonNotWritten, title, err.Error())
	}

	create := page.PageCreate{
		Title:    title,
		Slug:     slug,
		Markdown: markdown,
		HTML:     html,
		Status:   status,
		Fields:   fields,
		Kind:     page.KindPage,
	}

	// There is no third behaviour. Either the address is free and the row
	// creates, or it is taken and the choice made on screen 1 decides between
	// rewriting the page and leaving it alone.
	outcome := OutcomeCreate
	if existing != nil {
		if collision != CollisionUpdate {
			return skip(ReasonExistingSkipped, existing.Slug)
		}
		outcome = OutcomeUpdate
	}

	return Verdict{Row: row.Number, Outcome: outcome, Reason: note.Reason, Args: note.Args}, create, data
}

// Writer writes the rows CheckRow admitted.
//
// Named Writer and not Store because this package already has a Store, which
// is the staging table. This one owns no table of its own: it holds the two
// ordinary stores and calls them the way any other part of the admin does.
type Writer struct {
	Pages *page.Store
	Terms *term.Store
}

// WriteRow decides one row and, if the decision says to, writes it.
//
// No transaction is opened here and nothing this calls holds one across two
// rows. That is the guarantee this phase ships, and it is the one that matters:
// the write pool admits a single connection (db.go:28, _txlock=immediate), so a
// transaction spanning a file would block every other request on the machine,
// admin and public alike. Between two rows, other requests interleave.
//
// Per-row atomicity is deliberately NOT delivered, and the requirement carries
// an amendment stamp saying so. One imported row already spans two or three
// transactions inside the ordinary stores — page.CreatePage is a single
// autocommitted INSERT, page.UpdatePage opens its own, term.SetForPage opens
// its own at store.go:120 and term.EnsureNames its own at :330 — and merging
// them would mean threading a *sql.Tx through page.Store and term.Store, which
// is a second creation path beside the ordinary one and precisely what IMP-02
// and criterion 4 forbid in as many words.
//
// "Leaves nothing half-written" is therefore delivered twice over. The whole
// row is validated by field.CheckAll before the first write, so a row that
// cannot become a page never reaches CreatePage. And if a later step of the row
// still fails on a database error, the row is undone through the ORDINARY path:
// page.Store.TrashPage then PurgePage, the same two steps an admin takes to
// delete a page and empty the trash. Both are single statements, neither does
// redirect bookkeeping, and no new store method is added, so IMP-02 is not
// bent. If the compensation itself fails, the verdict says so and names the
// page — a lie about the row would be worse than an ugly truth.
//
// The compensation runs on the create path ONLY. A page that was already there
// when the import started is not this import's to delete: TrashPage then
// PurgePage applied to the update arm would destroy a page the operator already
// had, carrying content this file never supplied. That is not compensation,
// that is data loss caused by the recovery path. So an update whose terms fail
// is reported and left standing — the page keeps whatever the update wrote, the
// row is named in the report, and the operator decides.
//
// A branch that must not run leaves no trace of not running, so it is asserted
// rather than argued: row_test.go's TestUpdateArmIsNotRolledBack fails the
// terms step of an update and looks for the page afterwards.
func (w Writer) WriteRow(ctx context.Context, websiteID int64, defs []field.Def,
	row csv.Row, m Mapping, existing *page.Page, collision string, userID *int64) Verdict {

	v, create, data := CheckRow(defs, row, m, existing, collision, i18n.Lang(ctx))
	if !v.Written() {
		return v
	}
	create.WebsiteID = websiteID
	create.UserID = userID

	names, _ := RowTerms(row, m)

	if v.Outcome == OutcomeUpdate {
		u, err := w.update(existing, create, data, defs, row, m, userID)
		if err != nil {
			// Reported and not written. The create arm answers the same
			// failure the same way at field.Encode below, and an update that
			// quietly kept the old values and reported success is the one
			// answer the operator has no way to check.
			return Verdict{Row: v.Row, Outcome: OutcomeSkip, Reason: ReasonNotWritten, Args: []string{create.Title, err.Error()}}
		}
		if err := w.Pages.UpdatePage(ctx, existing.ID, u); err != nil {
			return Verdict{Row: v.Row, Outcome: OutcomeSkip, Reason: ReasonNotWritten, Args: []string{create.Title, err.Error()}}
		}
		if err := w.setTerms(ctx, websiteID, existing.ID, row, m, names); err != nil {
			return Verdict{Row: v.Row, Outcome: OutcomeUpdate, Reason: ReasonNotRolledBack, Args: []string{create.Title, err.Error()}}
		}
		return v
	}

	created, err := w.Pages.CreatePage(ctx, create)
	if err != nil {
		return Verdict{Row: v.Row, Outcome: OutcomeSkip, Reason: ReasonNotWritten, Args: []string{create.Title, err.Error()}}
	}
	if created.Slug != create.Slug {
		// The one place the rename is visible. CreatePage retries -2, -3 and so
		// on silently and hands back the page it made, so this comparison is
		// the whole of D-23. It replaces a softer note, because criterion 4
		// asks for every renamed address by name.
		v.Reason, v.Args = ReasonRenamed, []string{create.Slug, created.Slug}
	}

	if err := w.setTerms(ctx, websiteID, created.ID, row, m, names); err != nil {
		if terr := w.Pages.TrashPage(ctx, created.ID); terr != nil {
			return Verdict{Row: v.Row, Outcome: OutcomeSkip, Reason: ReasonNotRolledBack, Args: []string{create.Title, terr.Error()}}
		}
		if perr := w.Pages.PurgePage(ctx, created.ID); perr != nil {
			return Verdict{Row: v.Row, Outcome: OutcomeSkip, Reason: ReasonNotRolledBack, Args: []string{create.Title, perr.Error()}}
		}
		return Verdict{Row: v.Row, Outcome: OutcomeSkip, Reason: ReasonNotWritten, Args: []string{create.Title, err.Error()}}
	}
	return v
}

// update builds the edit for a page that already exists.
//
// It starts from the page as it stands and replaces only what the mapping
// actually points at. An import that maps three columns must leave the other
// twenty alone: a file carrying titles is not a statement that every page's
// body is now empty. Kind is left empty on purpose, which page.UpdatePage reads
// as "leave the kind alone", so importing into a website of posts does not
// silently turn every one of them into a page.
//
// What counts as "points at" is the whole of this function, and it is the CELL
// and not the COLUMN. A mapped column whose cell is empty, and whose target
// carries no default, says NOTHING about that slot, and the slot is left
// exactly as it stands — for the body, for the status and for every field
// alike. A default set on the mapping screen does apply, because a default is
// the operator stating a value rather than the file failing to.
//
// The consequence, said plainly rather than left to be discovered: CLEARING A
// VALUE IS NOT EXPRESSIBLE FROM A CSV UPDATE. A file cannot ask for a page's
// body to become empty, and it cannot ask for a field to be emptied. Emptying
// is done on the page form, one page at a time, where the person doing it can
// see what they are emptying.
//
// This reverses a decision, and the record should show that rather than hide
// it. The rule here used to be that a mapped column whose cell is empty CLEARS
// its slot, "which is the difference between 'the file says this is empty now'
// and 'the file says nothing about this'". That distinction cannot be drawn
// from a CSV file: a blank cell is exactly as much "empty now" as it is "not
// filled in", the data does not carry the difference, and code that claims to
// read it is reading something that is not there. What the old rule actually
// bought was three ways to destroy content nobody asked to destroy — forty
// blank Zustand cells demoting forty live pages to drafts with no line in the
// report and no revision recorded, a blank Text cell wiping a page's Markdown,
// a blank field cell emptying a field — for a capability a CSV cannot express
// in the first place.
//
// Matching on an arbitrary key column, merging rather than replacing a value,
// and a per-column "only fill if this is empty" are all deferred ideas and none
// of them is built here; they are named so they are not smuggled in.
func (w Writer) update(existing *page.Page, create page.PageCreate, data field.Data,
	defs []field.Def, row csv.Row, m Mapping, userID *int64) (page.PageUpdate, error) {

	u := page.PageUpdate{
		Title:    create.Title, // always mapped: CheckRow refuses a row without one
		Slug:     existing.Slug,
		Markdown: existing.ContentMarkdown,
		HTML:     existing.ContentHTML,
		Status:   existing.Status,
		Blocks:   existing.Blocks,
		Fields:   existing.Fields,
		Meta: page.PageMeta{
			Excerpt:         existing.Excerpt,
			MetaDescription: existing.MetaDescription,
			FeaturedMediaID: existing.FeaturedMediaID,
			NoIndex:         existing.NoIndex,
		},
		Schedule:        page.PageSchedule{PublishAt: existing.PublishAt, UnpublishAt: existing.UnpublishAt},
		ExpectedVersion: existing.Version,
		UserID:          userID,
	}

	// stated is the one predicate this function decides on, and it asks the
	// question the file can actually answer: did this row put anything in this
	// slot? cellFor already falls the cell back to the target's default, so a
	// default the operator typed on the mapping screen counts as stated and an
	// empty cell without one does not.
	stated := func(t Target) bool { return cellFor(row, m, t) != "" }

	if stated(Target{Kind: TargetBody}) {
		u.Markdown, u.HTML = create.Markdown, create.HTML
	}
	if stated(Target{Kind: TargetStatus}) {
		u.Status = create.Status
	}

	// The field slots this row stated something for, and only those. A stated
	// cell may still store the empty string — parseBool turns "nein" into it —
	// and that is a value the file gave, not a slot it was silent about.
	byKey := definitionsByKey(applicableFields(defs, existing))
	merged := field.Decode(existing.Fields)
	for _, t := range m.Targets {
		if t.Kind != TargetField {
			continue
		}
		if _, exists := byKey[t.Key]; !exists {
			continue
		}
		if !stated(t) {
			continue
		}
		merged.Values[t.Key] = data.Values[t.Key]
	}
	// The error is handed out and never swallowed. Keeping the page's OLD
	// fields here while UpdatePage went on to succeed would discard every
	// mapped value of the row and report the row as a clean update — the
	// operator would read that it went through with the values they mapped,
	// and it did not. json.Marshal over a map of strings does not fail today,
	// so this is close to unreachable; a swallowed error on the write path, on
	// the one arm that touches pages the operator already had, is not
	// something to leave standing for that reason.
	encoded, err := field.Encode(field.Clean(applicableFields(defs, existing), merged))
	if err != nil {
		return u, err
	}
	u.Fields = encoded
	return u, nil
}

// setTerms writes the page's own terms, and does nothing at all when this row
// said nothing about them.
//
// The distinction is the whole of it, and it is drawn per ROW: a cell that says
// something replaces the page's labels, and a blank cell with no default says
// nothing about them, so the page keeps what it has.
//
// A DEFAULT counts as the file speaking, and that is the same rule the status,
// the body and every field already follow (row_test.go's
// TestBlankCellSaysNothingOnTheUpdateArm states it in as many words: a default
// is the operator stating a value on the mapping screen, so a default does
// apply). Asking about the column alone silently discarded what the operator
// typed into the Vorgaben card — and, once TermNames learned to harvest it, it
// discarded it AFTER the label had been created, which is an orphan made on
// purpose.
func (w Writer) setTerms(ctx context.Context, websiteID, pageID int64, row csv.Row, m Mapping, names []string) error {
	// The CELL and not the COLUMN, which is the same predicate update() uses
	// and for the same reason. This function used to ask whether the file had a
	// terms column at all, and argue for it: "an empty list from a mapped
	// column means this page has no terms and clears them". That is the rule
	// update() reversed — a blank cell is exactly as much "empty now" as it is
	// "not filled in", and a CSV cannot carry the difference — and the reversal
	// reached the body, the status and every field while this slot was left
	// behind on the old one.
	//
	// So a file of five hundred rows with a Schlagwörter column, three hundred
	// of whose cells are blank, stripped the labels off three hundred existing
	// pages with no line in the report. cellFor already falls back to the
	// target's default, so a default typed on the mapping screen still counts
	// as the operator stating a value.
	if w.Terms == nil || cellFor(row, m, Target{Kind: TargetTerms}) == "" {
		return nil
	}
	return w.Terms.SetForPage(ctx, websiteID, pageID, names)
}
