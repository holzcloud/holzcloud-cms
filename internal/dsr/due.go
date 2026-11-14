package dsr

import (
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
)

// When a request about personal data has to be answered (Phase 33.2, D-01;
// research §B.1).
//
//   - EU, GDPR Art. 12(3): "without undue delay and in any event within one
//     month of receipt of the request".
//   - United Kingdom, UK GDPR Art. 12(3): the same month.
//   - Switzerland, DSG Art. 25(7) and DSV Art. 18: as a rule within 30 days.
//   - United States, CCPA regs § 7021: the answer "no later than 45 calendar
//     days after receipt", and an acknowledgement "no later than 10 business
//     days after receiving" a request to know, delete or correct.
//
// Each law allows an extension when the person is told in time; that is the
// operator's to decide and is not computed here. The Requests screen shows
// these dates and enforces nothing: a date is a reminder, not a gate.
//
// "One month" is the same day of the next month, and the last day of that
// month when it has no such day: 31 January is due on 28 February. Written out
// rather than left to time.AddDate, which normalises 31 February to 3 March
// and would put the deadline after the one the law sets.
//
// Business days are Monday to Friday. Public holidays are not counted — they
// differ by state and year — so the screen says "about" before the
// acknowledgement date, and a holiday only ever moves the true date later.

// Due is when a request has to be answered.
type Due struct {
	// Answer is the earliest answer date of the website's frameworks.
	Answer time.Time
	// Acknowledge is the US acknowledgement date, zero when the website does
	// not follow US law.
	Acknowledge time.Time
	// By is the framework whose deadline Answer is.
	By legal.Framework
}

// DueFor is the due dates of a request received at received on a website that
// follows set, and false when the set is empty: then no law names a deadline.
// With several frameworks the earliest answer date wins, because answering
// by it meets all of them; on a tie the first in legal.All's order names it.
// The dates keep received's location, so a caller passes the time in the
// website's zone and gets that zone's days back.
func DueFor(received time.Time, set legal.Set) (Due, bool) {
	if set.Empty() {
		return Due{}, false
	}
	var due Due
	for _, f := range set.List() {
		var at time.Time
		switch f {
		case legal.EU, legal.UK:
			at = oneMonthAfter(received)
		case legal.CH:
			at = received.AddDate(0, 0, 30)
		case legal.US:
			at = received.AddDate(0, 0, 45)
			due.Acknowledge = businessDaysAfter(received, 10)
		default:
			continue
		}
		if due.Answer.IsZero() || dayBefore(at, due.Answer) {
			due.Answer, due.By = at, f
		}
	}
	return due, !due.Answer.IsZero()
}

// oneMonthAfter is the same day of the next month, or that month's last day.
func oneMonthAfter(t time.Time) time.Time {
	y, m, d := t.Date()
	// Day 0 of the month after next is the last day of the next month.
	last := time.Date(y, m+2, 0, 0, 0, 0, 0, t.Location()).Day()
	if d > last {
		d = last
	}
	return time.Date(y, m+1, d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

// businessDaysAfter is the n-th Monday-to-Friday after t.
func businessDaysAfter(t time.Time, n int) time.Time {
	for n > 0 {
		t = t.AddDate(0, 0, 1)
		if wd := t.Weekday(); wd != time.Saturday && wd != time.Sunday {
			n--
		}
	}
	return t
}

// dayBefore reports whether a falls on an earlier calendar day than b.
func dayBefore(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return time.Date(ay, am, ad, 0, 0, 0, 0, time.UTC).Before(time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC))
}
