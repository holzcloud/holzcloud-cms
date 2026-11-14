package privacy

import "github.com/holzcloud/holzcloud-cms/internal/legal"

// The imprint: who offers the website and how to reach them.
//
// Every website gets one, whatever its frameworks (D-09). That is the owner's
// choice and not a reading of any one law: a visitor who wants to know who is
// behind a website should find out, and a website that starts under the US
// set and later adds the EU is better off with the page already in its footer.
//
// The lean form, the overview, the identity and the contact blocks and the
// closing, is what a website gets with only the US set or none. US law has no
// general imprint duty, so nothing is added for it, and an empty set adds
// nothing either. Each of the other frameworks adds the items its law
// requires: the EU block those of Art. 5 of Directive 2000/31/EC as § 5 DDG
// writes them for a provider in Germany, the Swiss block those of the Federal
// Act against Unfair Competition (UWG, SR 241), the UK block those of the
// Electronic Commerce Regulations 2002 and, for a company, the Trading
// Disclosures Regulations 2015. internal/privacy/sources/imprint.md records
// which item comes from where.
//
// There is no line about the EU platform for online dispute resolution. Its
// regulation was repealed with effect from 20 July 2025 (D-16), and a link to
// it now sends a consumer to a page that is gone; a test keeps it out.
//
// Every fact only the operator knows is a fill-in marker: the name, the
// address, the e-mail, the register and the tax numbers. The settings do
// hold an address and a tax number, and the admin shows them next to the
// draft button, but they are never written into the page. Markdown an
// operator typed into a settings field would otherwise reach a template that
// is published after editing, which is the property Phase 30 held for the
// privacy page: the inputs are the language, the spelling and the set, and
// nothing a person typed.

func init() {
	register(kindSpec{kind: KindImprint, blocks: imprintBlocks, minFillins: 2})
}

// imprintBlocks is the structure of the imprint: the lean form always, one
// block per framework that has an imprint duty, and the closing.
var imprintBlocks = []block{
	{name: "glance"},
	{name: "identity"},
	{name: "contact"},
	{name: "eu", on: imprintHas(legal.EU)},
	{name: "ch", on: imprintHas(legal.CH)},
	{name: "uk", on: imprintHas(legal.UK)},
	{name: "closing"},
}

// imprintHas is the condition of a framework block.
func imprintHas(f legal.Framework) func(Request) bool {
	return func(r Request) bool { return r.Set.Has(f) }
}

// ComposeImprint writes the Markdown of an imprint. The same request always
// gives the same text, and only the language, the spelling and the framework
// set change it.
//
// Like the privacy page it is a template: it opens by saying it is not
// finished and closes by saying it is no legal advice.
func ComposeImprint(r Request) string { return compose(KindImprint, r) }
