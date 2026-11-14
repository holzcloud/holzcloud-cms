package privacy

// The "Do Not Sell or Share My Personal Information" page (LEG-03, D-10, D-11).
//
// What it is for. A website whose set has US tells a California visitor, on
// the page the statute names (Cal. Civ. Code § 1798.135(a)(1)), that it sells
// no personal information and shares none, which processors it involves, how
// it treats the Global Privacy Control signal, and how to opt out anyway. The
// status line and the form are not part of this text: the page carries the
// marker [[widerspruch]], and internal/public's expandOptOut puts the widget
// there at delivery, because whether this browser sent Sec-GPC: 1 is a fact of
// the request and not of the page (CCPA regs § 7025(c)(6)).
//
// Why it exists at all. A business that neither sells nor shares need not
// offer the page (CCPA regs § 7010(e)). The owner keeps it nevertheless (D-10):
// a visitor who looks for it finds it, and the opt-out it takes is recorded.
// The text says both things, so the page claims no duty it does not have.
//
// Why a marker and not a template type. A page type of its own would need a
// template in all eight shipped themes and a new chapter of TEMPLATE-SPEC, and
// every theme somebody uploads would lack it. A marker in an ordinary page
// renders in every theme, through the contact-form classes they all style.
//
// Why the title is English in every language. It is the wording of the
// statute and the words a California visitor looks for, whatever language the
// rest of the website speaks. The page itself, below the title, speaks the
// page's language. The slug is the same in every language too; slugs are
// unique per language, so the same address in several languages is allowed.

func init() {
	register(kindSpec{
		kind:       KindDNS,
		minFillins: 1,
		blocks: []block{
			{name: "glance"},
			{name: "nothing-sold"},
			{name: "processors"},
			{name: "payment", on: func(r Request) bool { return r.Features.Shop && r.Features.Payment }},
			{name: "gpc"},
			{name: "form"},
			{name: "agents"},
			{name: "closing"},
		},
	})
}

// ComposeDNS writes the Markdown of the "Do Not Sell or Share My Personal
// Information" page. The same request always gives the same text, and every
// text carries the widget marker exactly once, on a line of its own.
func ComposeDNS(r Request) string { return compose(KindDNS, r) }
