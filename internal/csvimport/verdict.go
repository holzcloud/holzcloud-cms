// This file mints codes and never sentences, and that is the whole of it.
//
// The measurement it rests on: tools/i18n reads {{t}}, {{th}} and {{tf}}
// literals out of the templates (main.go:47) and the first string argument of
// exactly eight named Go functions — SetFlashError, SetFlashSuccess,
// SetFlashWarning, Add, NewLayoutData, Titlef, T and N (main.go:53-65). A
// string built with fmt.Sprintf and appended to a report is collected by
// neither. internal/admin/wordpress.go:66-84 is the proof standing in the tree
// today: those warnings are hard-coded German, and go run ./tools/i18n reports
// nothing missing and nothing orphaned anyway.
//
// This phase's report is entirely such strings — one per row, several hundred
// of them on a real file. Built in Go they would leave the translation gate
// green while the largest screen of the feature was German-only, which is the
// same hole as internal/field's untranslated rejection reasons. Twice is a
// pattern; a third time would be a decision. So a row's outcome travels as a
// code plus its arguments, and the sentence a person reads is a {{tf}} literal
// in the template, where the tool can see it.
//
// What is kept from internal/admin/wordpress.go:107 is the discipline and not
// the type: an empty Reason means the row worked.
package csvimport

import "strings"

// Outcome is what happens to one row.
//
// These three values are this program's own and are never written to a
// database. The staged upload's collision setting carries the German words
// 'uebergehen' and 'aktualisieren', pinned by migration 00049's CHECK
// constraint — those are data, these are not, and the two must not be confused
// for one another in either direction.
type Outcome string

const (
	// OutcomeCreate: the row becomes a new page.
	OutcomeCreate Outcome = "create"
	// OutcomeUpdate: a page with this address exists and the operator chose to
	// have it rewritten.
	OutcomeUpdate Outcome = "update"
	// OutcomeSkip: nothing is written for this row. Every refusal ends here,
	// and so does an existing page the operator chose to leave alone.
	OutcomeSkip Outcome = "skip"
)

// Reason is a short code naming why a row ended the way it did.
//
// Lower-case ASCII with no spaces: it is an identifier a template switches on
// and never a word a translator sees. The empty Reason means the row worked,
// which is wordpress.go:107's discipline kept.
type Reason string

// The reasons one row can carry. Each one names its arguments, because the
// {{tf}} that renders it has to take exactly that many and nothing else
// enforces the count.
const (
	// ReasonRowUnreadable: encoding/csv could not read the row.
	// Argument: the stdlib's own ParseError text.
	//
	// That argument is English prose inside a German sentence, and it is the
	// one place in this phase where that is a DECISION rather than the D-32
	// hole. It is encoding/csv's wording, not this phase's, and rewriting the
	// standard library's parse errors as codes would mean tracking the set of
	// them across Go releases. Every reason this phase mints itself carries
	// numbers and names as arguments instead — ReasonRowTooWide below is the
	// case that used to be a sentence here and is now one of them.
	ReasonRowUnreadable Reason = "row_unreadable"
	// ReasonRowTooWide: the row brought more cells than the header has
	// columns. Arguments: the row's cell count, the header's.
	//
	// No cell is dropped for it — Reader.fill keeps them all — but a row the
	// header cannot account for is a row whose values may have shifted, so it
	// is refused and both numbers are named, which is what lets the operator
	// find the stray separator.
	ReasonRowTooWide Reason = "row_too_wide"
	// ReasonCellTooLong: a cell is over csv.MaxCellBytes.
	// Arguments: the column's position, the cell's size in bytes.
	//
	// The position and not the heading: the row function is given a row and a
	// mapping and not the header, and passing the header in only so a message
	// could name it would widen the one decision of this phase for the sake of
	// one sentence. The screen has the header and can print it beside the
	// number.
	ReasonCellTooLong Reason = "cell_too_long"
	// ReasonNoTitle: no column is pointed at the title, or the title cell is
	// empty and no default was given. No arguments. A page with no title is a
	// page nobody finds again.
	ReasonNoTitle Reason = "no_title"
	// ReasonSlugInvalid: page.ValidateSlug refused the address.
	// Argument: the offending address.
	ReasonSlugInvalid Reason = "slug_invalid"
	// ReasonStatusUnknown: the status cell is outside the closed vocabulary.
	// Argument: the cell.
	ReasonStatusUnknown Reason = "status_unknown"
	// ReasonBoolUnreadable: a janein cell says something the closed vocabulary
	// does not contain. Arguments: the field's label, the cell.
	ReasonBoolUnreadable Reason = "bool_unreadable"
	// ReasonFieldRejected: field.CheckAll refused a value.
	// Arguments: the field's label, Check's own reason.
	//
	// That second argument is prose, and prose this phase did not write:
	// field.Check's reasons (field.go:740-800) are hard-coded German and
	// invisible to tools/i18n today. They are carried here as an argument and
	// deliberately not fixed — internal/field is on this phase's list of what
	// must not change — so a later reader does not mistake them for debt this
	// phase created.
	ReasonFieldRejected Reason = "field_rejected"
	// ReasonFieldMissing: the mapping names a field definition that no longer
	// exists. Argument: the field's key.
	ReasonFieldMissing Reason = "field_missing"
	// ReasonTermsTruncated: the cell named more terms than term.MaxPerPage.
	// Argument: how many were kept. A cap is reported, never silently applied.
	ReasonTermsTruncated Reason = "terms_truncated"
	// ReasonRenamed: the address was taken and the database gave the page the
	// next free one. Arguments: the wanted address, the address it got.
	ReasonRenamed Reason = "renamed"
	// ReasonExistingSkipped: a page with this address exists and the operator
	// chose to leave it alone. Argument: the address.
	ReasonExistingSkipped Reason = "existing_skipped"
	// ReasonNotRolledBack: the row is half written and was not taken back.
	// Arguments: the page's title, what went wrong.
	//
	// Two cases reach it. On the create path the page was made, a later step
	// failed and undoing it failed as well. On the update path the page was
	// rewritten and its terms were not — and there undoing is not on the table
	// at all, because a page that was already there when the import started is
	// not this import's to delete. An ugly truth either way, because a lie
	// about the row would be worse.
	ReasonNotRolledBack Reason = "not_rolled_back"
	// ReasonNotWritten: the store refused the row, or the row was taken back
	// cleanly after a later step failed. Arguments: the page's title, what the
	// store said. The second argument is a Go error meant for the log; the
	// screen shows the title and the code's own sentence.
	ReasonNotWritten Reason = "not_written"
	// ReasonBodyUnreadable: the Markdown of the body could not be rendered.
	// Argument: what the renderer said.
	ReasonBodyUnreadable Reason = "body_unreadable"
	// ReasonColumnTaken: another column already took this heading's target.
	// Argument: the winning column's position.
	ReasonColumnTaken Reason = "column_taken"
	// ReasonEmptyHeader: the heading folds to nothing, so there is nothing to
	// match it by. No arguments.
	ReasonEmptyHeader Reason = "empty_header"
)

// Verdict is what happened, or would happen, to one row.
type Verdict struct {
	// Row is the number the operator's spreadsheet shows, minted by
	// csv.RowNumber and never anything else.
	//
	// A member of its own and never an entry in Args, which is what keeps the
	// grouping key below usable: two rows failing for the same reason have to
	// produce one key, and a row number inside Args would give them two.
	Row int
	// Outcome is what happens. Set even when Reason is empty.
	Outcome Outcome
	// Reason is empty when the row worked.
	Reason Reason
	// Args are what the template substitutes into the sentence, in the order
	// the Reason's comment names them.
	Args []string
}

// Written says whether this verdict touches the database.
//
// The dry run and the write call the same decision, and this is the predicate
// the write suppresses on: nothing is skipped by remembering not to write it,
// it is skipped by this returning false.
func (v Verdict) Written() bool { return v.Outcome != OutcomeSkip }

// GroupKey is how the report puts rows that say the same thing into one line.
//
// The report is grouped by reason and not listed by row, because "17 rows:
// this value is not one of the options (rows 4, 9, 12, ...)" is one line
// somebody can act on and seventeen separate lines saying it are a wall.
//
// Which arguments belong in the key: all of them. Every entry of Args is the
// reason's — the field that was refused, the value that was refused, the
// address that was taken — and none of them is the row's. What identifies the
// row is Row, and Row is deliberately not an argument, so two rows failing on
// the same value collapse into one group instead of into two groups of one.
//
// The outcome is deliberately not part of the key. A report groups by what went
// wrong and counts by what happened, and those are two different questions. A
// verdict with no reason produces the empty key, which is the group the report
// renders as a count and a link to the page list rather than as several hundred
// lines.
func (v Verdict) GroupKey() string {
	if v.Reason == "" && len(v.Args) == 0 {
		return ""
	}
	// The unit separator, because it cannot occur in a cell that got this far:
	// csv.CheckBytes refuses a file carrying a NUL and every other control
	// character would have to survive a spreadsheet's export to be here.
	return string(v.Reason) + "\x1f" + strings.Join(v.Args, "\x1f")
}
