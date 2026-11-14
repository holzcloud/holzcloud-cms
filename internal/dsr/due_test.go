package dsr

import (
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
)

func TestDueDatesFollowTheFrameworks(t *testing.T) {
	// A Saturday, and the last day of a month whose next month is shorter:
	// time.AddDate(0, 1, 0) would land on 3 March.
	received := time.Date(2026, 1, 31, 15, 30, 0, 0, time.UTC)
	day := func(t time.Time) string { return t.Format("2006-01-02") }

	for _, tc := range []struct {
		set    string
		answer string
		by     legal.Framework
		ack    string
	}{
		{"eu", "2026-02-28", legal.EU, ""},
		{"uk", "2026-02-28", legal.UK, ""},
		{"ch", "2026-03-02", legal.CH, ""},
		{"us", "2026-03-17", legal.US, "2026-02-13"},
		{"eu,us", "2026-02-28", legal.EU, "2026-02-13"},
		{"eu,ch", "2026-02-28", legal.EU, ""},
		{"ch,us", "2026-03-02", legal.CH, "2026-02-13"},
	} {
		due, ok := DueFor(received, legal.ParseSet(tc.set))
		if !ok {
			t.Errorf("%s: no due date", tc.set)
			continue
		}
		if day(due.Answer) != tc.answer || due.By != tc.by {
			t.Errorf("%s: answer by %s (%s); want %s (%s)", tc.set, day(due.Answer), due.By, tc.answer, tc.by)
		}
		gotAck := ""
		if !due.Acknowledge.IsZero() {
			gotAck = day(due.Acknowledge)
		}
		if gotAck != tc.ack {
			t.Errorf("%s: acknowledge by %q; want %q", tc.set, gotAck, tc.ack)
		}
	}

	// In a month with the day, the same day of the next month.
	if due, _ := DueFor(time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC), legal.ParseSet("eu")); day(due.Answer) != "2026-04-15" {
		t.Errorf("15 March + one month = %s; want 2026-04-15", day(due.Answer))
	}
	// Across the year, and into a leap February.
	if due, _ := DueFor(time.Date(2027, 12, 31, 9, 0, 0, 0, time.UTC), legal.ParseSet("eu")); day(due.Answer) != "2028-01-31" {
		t.Errorf("31 December + one month = %s; want 2028-01-31", day(due.Answer))
	}
	if due, _ := DueFor(time.Date(2028, 1, 30, 9, 0, 0, 0, time.UTC), legal.ParseSet("eu")); day(due.Answer) != "2028-02-29" {
		t.Errorf("30 January 2028 + one month = %s; want 2028-02-29", day(due.Answer))
	}
	// Ten business days from a Wednesday: two weekends skipped.
	if due, _ := DueFor(time.Date(2026, 2, 4, 9, 0, 0, 0, time.UTC), legal.ParseSet("us")); day(due.Acknowledge) != "2026-02-18" {
		t.Errorf("10 business days from Wednesday 4 February = %s; want 2026-02-18", day(due.Acknowledge))
	}

	if _, ok := DueFor(received, legal.Set{}); ok {
		t.Error("an empty set has a due date")
	}
}
