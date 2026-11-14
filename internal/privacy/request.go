package privacy

import "github.com/holzcloud/holzcloud-cms/internal/legal"

// The "request about my data" page (Phase 33.2, D-01, DSR-01..DSR-03).
//
// What it is for. It is the place a person asks what this website holds about
// them, or asks for it to be corrected or erased. The page explains the
// rights, says by when an answer is owed, and carries the form. Nothing about
// any person is ever shown on it: the form takes the request and the visitor
// is told the same sentence whatever address was typed. Finding the data,
// checking that the request comes from the person it is about, and sending
// the answer stay with the operator (D-01).
//
// Why a marker and not a template type. As with the Do-Not-Sell page, a page
// type of its own would need a template in all eight shipped themes and a new
// chapter of TEMPLATE-SPEC, and every theme somebody uploads would lack it.
// The page carries [[datenanfrage]] on a line of its own, and internal/public's
// expandDataRequest puts the form there at delivery, through the contact-form
// classes every theme styles. The form, its fields and its words are
// internal/public's and the catalogue's, not this package's.
//
// Why the deadlines follow the chosen frameworks. Each framework owes the
// answer by its own clock: one month under the GDPR and the UK GDPR, as a rule
// 30 days under the Swiss DSG, 45 calendar days with an acknowledgement within
// 10 business days under the CCPA regulations. A page that names a deadline
// the website does not owe promises what the operator need not keep; a page
// that leaves out one it owes withholds a right. So each deadline is a block of
// its own, shown when its framework is chosen and only then, and an empty set
// shows none of them (the launch checklist reports the empty set).
//
// Why every website gets it. The right of access is in all four frameworks,
// and the form is offered on every website (internal/public's
// dataRequestOffered). The legal pages card offers this page whatever the
// set.

func init() {
	register(kindSpec{
		kind:       KindRequest,
		minFillins: 1,
		blocks: []block{
			{name: "glance"},
			{name: "rights"},
			{name: "deadline-eu", on: requestUnder(legal.EU)},
			{name: "deadline-ch", on: requestUnder(legal.CH)},
			{name: "deadline-uk", on: requestUnder(legal.UK)},
			{name: "deadline-us", on: requestUnder(legal.US)},
			{name: "form"},
			{name: "identity"},
			{name: "kept", on: func(r Request) bool { return r.Features.Shop }},
			{name: "closing"},
		},
	})
}

// requestUnder is the condition of one framework's deadline block.
func requestUnder(f legal.Framework) func(Request) bool {
	return func(r Request) bool { return r.Set.Has(f) }
}

// ComposeRequest writes the Markdown of the "request about my data" page. The
// same request always gives the same text, and every text carries the form
// marker exactly once, on a line of its own.
func ComposeRequest(r Request) string { return compose(KindRequest, r) }
