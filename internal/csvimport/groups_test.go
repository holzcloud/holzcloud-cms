// What this file guards is the answer to the question the roadmap flagged:
// 312 rows with 40 problems have to read as a handful of lines somebody can act
// on. Every property below is one half of that.
//
// No database and no http.Request anywhere: Summarize is a pure function over
// verdicts, and it stays one.
package csvimport_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/csvimport"
)

// skipped is one refused row, at the given number, for the given reason.
func skipped(row int, reason csvimport.Reason, args ...string) csvimport.Verdict {
	return csvimport.Verdict{Row: row, Outcome: csvimport.OutcomeSkip, Reason: reason, Args: args}
}

// created is one row that worked and has nothing to say about itself.
func created(row int) csvimport.Verdict {
	return csvimport.Verdict{Row: row, Outcome: csvimport.OutcomeCreate}
}

// TestGroupSameReasonSameArgumentsIsOneGroup: forty rows refused over one value
// are one line, and the line knows all forty.
func TestGroupSameReasonSameArgumentsIsOneGroup(t *testing.T) {
	var verdicts []csvimport.Verdict
	for i := 0; i < 40; i++ {
		verdicts = append(verdicts, skipped(csvRowNumber(i), csvimport.ReasonFieldRejected, "Sorte", "steht nicht zur Auswahl"))
	}

	report := csvimport.Summarize(verdicts)
	if len(report.Groups) != 1 {
		t.Fatalf("groups = %d; want 1: %+v", len(report.Groups), report.Groups)
	}
	if got := report.Groups[0].Total; got != 40 {
		t.Errorf("group total = %d; want 40", got)
	}
	if report.Skipped != 40 || report.Created != 0 {
		t.Errorf("counters = created %d, skipped %d; want 0 and 40", report.Created, report.Skipped)
	}
}

// TestGroupSameReasonDifferentArgumentsIsTwoGroups: "Apfel" and "Birne" are two
// different things to go and fix, so they are two lines.
func TestGroupSameReasonDifferentArgumentsIsTwoGroups(t *testing.T) {
	report := csvimport.Summarize([]csvimport.Verdict{
		skipped(2, csvimport.ReasonFieldRejected, "Sorte", "Apfel"),
		skipped(3, csvimport.ReasonFieldRejected, "Sorte", "Birne"),
		skipped(4, csvimport.ReasonFieldRejected, "Sorte", "Apfel"),
	})
	if len(report.Groups) != 2 {
		t.Fatalf("groups = %d; want 2: %+v", len(report.Groups), report.Groups)
	}
	// Most rows first: the Apfel group has two, the Birne group one.
	if report.Groups[0].Total != 2 || report.Groups[1].Total != 1 {
		t.Errorf("group sizes = %d and %d; want 2 and 1", report.Groups[0].Total, report.Groups[1].Total)
	}
	if got := report.Groups[0].Args[1]; got != "Apfel" {
		t.Errorf("the larger group carries %q; want Apfel", got)
	}
}

// TestGroupOrderIsStable: the same input twice produces the same screen, so a
// screenshot means something and two runs over one file can be compared.
func TestGroupOrderIsStable(t *testing.T) {
	var verdicts []csvimport.Verdict
	// Three groups of the same size, so the tiebreak is what decides — the case
	// a map's walk order would silently make random.
	for i, value := range []string{"rot", "blau", "gruen"} {
		for n := 0; n < 3; n++ {
			verdicts = append(verdicts, skipped(10*i+n+2, csvimport.ReasonStatusUnknown, value))
		}
	}

	first := csvimport.Summarize(verdicts)
	for i := 0; i < 20; i++ {
		again := csvimport.Summarize(verdicts)
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("run %d differs:\n%+v\n%+v", i, first, again)
		}
	}
}

// TestGroupRowListIsTruncatedAndCounted: a cap is reported, never silently
// applied.
func TestGroupRowListIsTruncatedAndCounted(t *testing.T) {
	var verdicts []csvimport.Verdict
	for i := 0; i < 300; i++ {
		verdicts = append(verdicts, skipped(i+2, csvimport.ReasonNoTitle))
	}

	report := csvimport.Summarize(verdicts)
	if len(report.Groups) != 1 {
		t.Fatalf("groups = %d; want 1", len(report.Groups))
	}
	g := report.Groups[0]
	if len(g.Rows) != 25 {
		t.Errorf("named rows = %d; want 25", len(g.Rows))
	}
	if g.More != 275 {
		t.Errorf("More = %d; want 275", g.More)
	}
	if g.Total != 300 {
		t.Errorf("Total = %d; want 300", g.Total)
	}
	for i := 1; i < len(g.Rows); i++ {
		if g.Rows[i-1] >= g.Rows[i] {
			t.Fatalf("the row list is not ascending: %v", g.Rows)
		}
	}
}

// TestGroupSuccessesAreCountsNotGroups: 272 lines reading "angelegt" are the
// wall this shape exists to avoid.
func TestGroupSuccessesAreCountsNotGroups(t *testing.T) {
	var verdicts []csvimport.Verdict
	for i := 0; i < 272; i++ {
		verdicts = append(verdicts, created(i+2))
	}
	verdicts = append(verdicts, csvimport.Verdict{Row: 300, Outcome: csvimport.OutcomeUpdate})

	report := csvimport.Summarize(verdicts)
	if len(report.Groups) != 0 {
		t.Errorf("groups = %d; want 0 — a success is a counter", len(report.Groups))
	}
	if report.Created != 272 || report.Updated != 1 || report.Total != 273 {
		t.Errorf("created %d, updated %d, total %d; want 272, 1, 273",
			report.Created, report.Updated, report.Total)
	}
}

// TestGroupRenamedCountsAndGroups: the one verdict that is both a success and a
// thing to report.
func TestGroupRenamedCountsAndGroups(t *testing.T) {
	report := csvimport.Summarize([]csvimport.Verdict{
		created(2),
		{Row: 3, Outcome: csvimport.OutcomeCreate, Reason: csvimport.ReasonRenamed,
			Args: []string{"apfel", "apfel-2"}},
	})

	if report.Created != 2 {
		t.Errorf("created = %d; want 2 — a renamed page was still created", report.Created)
	}
	if report.Renamed != 1 {
		t.Errorf("renamed = %d; want 1", report.Renamed)
	}
	if len(report.Groups) != 1 || report.Groups[0].Reason != csvimport.ReasonRenamed {
		t.Fatalf("the rename did not form a group: %+v", report.Groups)
	}
	if got := report.Groups[0].Outcome; got != csvimport.OutcomeCreate {
		t.Errorf("group outcome = %q; want create", got)
	}
}

// TestGroupNoProblemsNoGroups: a clean run is counters and a sentence, not an
// empty table.
func TestGroupNoProblemsNoGroups(t *testing.T) {
	report := csvimport.Summarize([]csvimport.Verdict{created(2), created(3), created(4)})
	if len(report.Groups) != 0 {
		t.Errorf("groups = %d; want 0", len(report.Groups))
	}
	if report.Created != 3 || report.Skipped != 0 {
		t.Errorf("created %d, skipped %d; want 3 and 0", report.Created, report.Skipped)
	}
}

// TestGroupKeyNeverCarriesTheRowNumber: the property the whole shape rests on,
// asserted against the key itself rather than through Summarize.
func TestGroupKeyNeverCarriesTheRowNumber(t *testing.T) {
	a := csvimport.Verdict{Row: 4, Outcome: csvimport.OutcomeSkip,
		Reason: csvimport.ReasonSlugInvalid, Args: []string{"—"}}
	b := a
	b.Row = 4711
	if a.GroupKey() != b.GroupKey() {
		t.Fatalf("two rows with one reason produced two keys: %q and %q", a.GroupKey(), b.GroupKey())
	}
	if got := fmt.Sprint(csvimport.Summarize([]csvimport.Verdict{a, b}).Groups[0].Rows); got != "[4 4711]" {
		t.Errorf("rows = %s; want [4 4711]", got)
	}
}

// csvRowNumber is the number a spreadsheet shows for the i-th data row. Written
// out here rather than imported so this file needs nothing but csvimport.
func csvRowNumber(i int) int { return i + 2 }
