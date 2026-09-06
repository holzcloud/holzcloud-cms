// The shape of the report is the decision, and it is the one the roadmap
// flagged as this phase's research question in as many words: 312 rows with 40
// problems is an information-design problem, not a layout question.
//
// Two shapes were available and both are wrong on their own. A list by row is
// honest and unusable: forty lines saying the same thing are a wall an operator
// scrolls past, and the one line that says something else is lost in it. A
// count with no rows is usable and useless: "40 rows were skipped" leaves the
// operator to find those forty rows in a spreadsheet of three hundred.
//
// One line per reason, with the rows named inside it, is the shape that lets a
// person open their file and fix something:
//
//	17 rows: "Sorte": "Apfel" steht nicht zur Auswahl (Zeilen 4, 9, 12, ...)
//
// Rows that worked are a count and a link to the page list, never a list of
// their own: "272 Seiten angelegt" beside the way to go and look at them.
//
// Nothing in this file formats a sentence and it imports no fmt. A group
// carries a reason CODE and the reason's arguments; the sentence is a {{tf}}
// literal in the template, where tools/i18n can see it (D-32).
package csvimport

import "sort"

// maxNamedRows is how many row numbers one group prints before it counts the
// rest.
//
// Twenty-five, which is what internal/admin/wordpress.go:73 already uses for
// the same reason and on the same kind of list. The cap is reported through
// Group.More and never silently applied — a truncation nobody is told about is
// the report lying about the file.
const maxNamedRows = 25

// Group is every row that said the same thing, on one line.
type Group struct {
	// Outcome is what happened to the rows in this group. It is taken from the
	// first verdict that opened the group and is not part of the key: a report
	// groups by what went wrong and counts by what happened, and those are two
	// different questions.
	Outcome Outcome
	// Reason is the code the template switches on.
	Reason Reason
	// Args are what the template substitutes into the sentence, in the order
	// the Reason's comment in verdict.go names them.
	Args []string
	// Rows are the numbers the operator's spreadsheet shows, ascending, at most
	// maxNamedRows of them.
	Rows []int
	// More is how many row numbers were not listed. Zero when all of them are.
	More int
	// Total is how many rows are in this group altogether, listed or not. The
	// headline number of the line, and the thing the operator reads first.
	Total int
}

// Report is the whole of one dry run or one write, ready to render.
type Report struct {
	// Created, Updated and Skipped count the outcomes. Together they are Total.
	Created int
	Updated int
	Skipped int
	// Renamed counts the rows the database gave a different address to.
	//
	// A renamed row is the one verdict that is BOTH a success and a thing to
	// report, and a reader will otherwise assume the two are exclusive: it
	// increments Created and Renamed, and it also forms a group, because
	// criterion 4 asks for every renamed address by name (D-23). Renamed is
	// therefore not a fourth outcome and does not belong beside the other three
	// in a sum.
	Renamed int
	// Groups are the lines of the report, most rows first.
	Groups []Group
	// Truncated says the file held more rows than csv.MaxRows. Set by the
	// caller, which is the one that holds the reader.
	Truncated bool
	// Total is how many rows were decided.
	Total int
}

// Summarize turns one pass of verdicts into the report a screen renders.
//
// It is called with the dry run's verdicts and with the write's verdicts, and
// it cannot tell the two apart — which is the point: the operator sees the same
// screen shape before and after, built by the same function from verdicts the
// same CheckRow produced (D-22).
//
// The grouping key is Verdict.GroupKey, which is the reason plus the reason's
// own arguments and never the row number. That is why Row is a member of
// Verdict and not Args[0]: two rows refused over the same value have to collapse
// into one group instead of into two groups of one.
//
// The order is most rows first, ties broken by the group's lowest row number
// and then by the key itself. Deterministic on purpose: two runs over one file
// produce the same screen, so a screenshot means something and a test can
// compare two runs byte for byte.
func Summarize(verdicts []Verdict) Report {
	report := Report{Total: len(verdicts)}

	// index maps a group key to its position in report.Groups, so the groups
	// come out in the order the file first mentioned each — which is what the
	// sort below then breaks ties on, deterministically.
	index := make(map[string]int, len(verdicts))

	for _, v := range verdicts {
		switch v.Outcome {
		case OutcomeCreate:
			report.Created++
		case OutcomeUpdate:
			report.Updated++
		case OutcomeSkip:
			report.Skipped++
		}
		if v.Reason == ReasonRenamed {
			report.Renamed++
		}

		key := v.GroupKey()
		if key == "" {
			// The row worked and has nothing to say. Successful rows are
			// counters and never groups: 272 lines reading "angelegt" are the
			// wall this shape exists to avoid.
			continue
		}

		at, seen := index[key]
		if !seen {
			at = len(report.Groups)
			index[key] = at
			report.Groups = append(report.Groups, Group{
				Outcome: v.Outcome,
				Reason:  v.Reason,
				Args:    v.Args,
			})
		}
		g := &report.Groups[at]
		g.Total++
		// Ascending by construction: the pass runs in file order, so the row
		// numbers arrive in order and nothing has to sort them afterwards.
		if len(g.Rows) < maxNamedRows {
			g.Rows = append(g.Rows, v.Row)
			continue
		}
		g.More++
	}

	sort.SliceStable(report.Groups, func(i, j int) bool {
		a, b := report.Groups[i], report.Groups[j]
		if a.Total != b.Total {
			return a.Total > b.Total
		}
		if len(a.Rows) > 0 && len(b.Rows) > 0 && a.Rows[0] != b.Rows[0] {
			return a.Rows[0] < b.Rows[0]
		}
		// The last tiebreak, so the order cannot depend on how the map was
		// walked. Two groups of the same size starting on the same row is not a
		// case a real file produces, and an order that is undefined for it
		// would still be an order two runs could disagree on.
		return groupKeyOf(a) < groupKeyOf(b)
	})

	return report
}

// groupKeyOf rebuilds a group's key from what the group kept, so the sort's
// last tiebreak uses the same string the grouping did.
func groupKeyOf(g Group) string {
	return Verdict{Reason: g.Reason, Args: g.Args}.GroupKey()
}
