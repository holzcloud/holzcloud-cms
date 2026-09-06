// What this file guards is that a verdict stays a code and its arguments.
//
// The plan gives this type no test file of its own, on the ground that the row
// function's tests exercise it. They exercise the codes; they do not exercise
// the two properties the type itself promises — that an empty reason means the
// row worked, and that two rows saying the same thing produce one grouping key
// — and a stated guarantee with no gate is not a guarantee.
package csvimport

import "testing"

// TestCleanRowCarriesNoReason: the discipline kept from
// internal/admin/wordpress.go:107.
func TestCleanRowCarriesNoReason(t *testing.T) {
	v := Verdict{Row: 2, Outcome: OutcomeCreate}
	if v.Reason != "" {
		t.Errorf("a clean verdict carries the reason %q, want none", v.Reason)
	}
	if !v.Written() {
		t.Error("a created row reports as not written")
	}
	if v.GroupKey() != "" {
		t.Errorf("a clean verdict groups under %q, want the empty key", v.GroupKey())
	}
}

// TestRenamedRowKeepsBothAddresses: an outcome and a reason together, which is
// the case that would be lost if the reason were only ever a refusal.
func TestRenamedRowKeepsBothAddresses(t *testing.T) {
	v := Verdict{Row: 4, Outcome: OutcomeCreate, Reason: ReasonRenamed, Args: []string{"apfel", "apfel-2"}}
	if !v.Written() {
		t.Error("a renamed row reports as not written — it was created, under another address")
	}
	if len(v.Args) != 2 {
		t.Fatalf("the verdict carries %d arguments, want the wanted and the actual address", len(v.Args))
	}
}

// TestSameReasonAndArgsGroupTogether: and a differing row number does not
// split the group, which is why Row is a member and never an argument.
func TestSameReasonAndArgsGroupTogether(t *testing.T) {
	a := Verdict{Row: 4, Outcome: OutcomeSkip, Reason: ReasonFieldRejected, Args: []string{"Sorte", "not one of the options"}}
	b := Verdict{Row: 9, Outcome: OutcomeSkip, Reason: ReasonFieldRejected, Args: []string{"Sorte", "not one of the options"}}
	c := Verdict{Row: 12, Outcome: OutcomeSkip, Reason: ReasonFieldRejected, Args: []string{"Farbe", "not one of the options"}}

	if a.GroupKey() != b.GroupKey() {
		t.Errorf("two rows failing on the same value group under %q and %q, want one key", a.GroupKey(), b.GroupKey())
	}
	if a.GroupKey() == c.GroupKey() {
		t.Errorf("two rows failing on different fields share the key %q, want two", a.GroupKey())
	}
	if a.Written() {
		t.Error("a skipped row reports as written")
	}
}

// TestEveryReasonIsAnAsciiCode: no reason is a word a translator would have to
// see, and none of them carries a space that would end up in a URL or a form
// value.
func TestEveryReasonIsAnAsciiCode(t *testing.T) {
	all := []Reason{
		ReasonRowUnreadable, ReasonCellTooLong, ReasonNoTitle, ReasonSlugInvalid,
		ReasonStatusUnknown, ReasonBoolUnreadable, ReasonFieldRejected, ReasonFieldMissing,
		ReasonTermsTruncated, ReasonRenamed, ReasonExistingSkipped, ReasonNotRolledBack,
		ReasonColumnTaken, ReasonEmptyHeader,
	}
	seen := map[Reason]bool{}
	for _, r := range all {
		if seen[r] {
			t.Errorf("the code %q is used twice, so two different things would group as one", r)
		}
		seen[r] = true
		if r == "" {
			t.Error("a reason is the empty string, which already means the row worked")
		}
		for _, c := range r {
			if !(c >= 'a' && c <= 'z') && c != '_' {
				t.Errorf("the code %q carries %q — a reason is an identifier, not a sentence", r, string(c))
			}
		}
	}
}
