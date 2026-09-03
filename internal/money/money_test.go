package money

import "testing"

// The Swiss rates all have a decimal place. A percent held as a whole number
// cannot express any of them, which is the reason this package counts in basis
// points at all — so that is the first thing worth proving.
func TestSwissRatesSurviveTheRoundTrip(t *testing.T) {
	cases := map[TaxRate]string{
		RateStandard: "8.1 %",
		RateReduced:  "2.6 %",
		RateLodging:  "3.8 %",
		RateExempt:   "0 %",
		775:          "7.75 %",
	}
	for rate, want := range cases {
		if got := rate.String(); got != want {
			t.Errorf("TaxRate(%d) = %q, want %q", rate, got, want)
		}
	}
}

// net + tax == gross has to hold for every line, or an invoice needs a
// correction row to add up. It is guaranteed by taking the tax as the
// remainder rather than rounding it separately — so the property is what the
// test checks, across a wide spread of values.
func TestTaxIsAlwaysTheRemainder(t *testing.T) {
	for _, rate := range []TaxRate{RateStandard, RateReduced, RateLodging, RateExempt} {
		for gross := Amount(0); gross <= 20000; gross += 7 {
			net := NetFromGross(gross, rate)
			tax := TaxFromGross(gross, rate)
			if net+tax != gross {
				t.Fatalf("rate %s, gross %d: net %d + tax %d = %d", rate, gross, net, tax, net+tax)
			}
		}
	}
}

// Rounding is half away from zero. Truncation would quietly favour one side by
// up to a rappen on every single line.
func TestRoundingIsHalfAwayFromZero(t *testing.T) {
	cases := []struct {
		numerator, denominator int64
		want                   Amount
	}{
		{15, 10, 2},   // 1.5 up
		{25, 10, 3},   // 2.5 up
		{14, 10, 1},   // 1.4 down
		{-15, 10, -2}, // and the same in the other direction
		{-14, 10, -1},
		{0, 10, 0},
		{5, 0, 0}, // no division by zero
	}
	for _, c := range cases {
		if got := divRound(c.numerator, c.denominator); got != c.want {
			t.Errorf("divRound(%d, %d) = %d, want %d", c.numerator, c.denominator, got, c.want)
		}
	}
}

// A consumer buying three items at 49.00 pays 147.00 — not 146.99, and not
// 147.01. The advertised price is the one that is multiplied.
func TestPrivateLineIsAMultipleOfTheAdvertisedPrice(t *testing.T) {
	l := LineForPrivate(4900, 3, RateStandard)

	if l.Gross != 14700 {
		t.Errorf("gross = %d, want 14700 — the advertised price was not what got multiplied", l.Gross)
	}
	if l.Net+l.Tax != l.Gross {
		t.Errorf("net %d + tax %d != gross %d", l.Net, l.Tax, l.Gross)
	}
	// 14700 / 1.081 = 13598.5...  → 13599 rounded, tax 1101.
	if l.Net != 13599 || l.Tax != 1101 {
		t.Errorf("net/tax = %d/%d, want 13599/1101", l.Net, l.Tax)
	}
}

// A business is quoted a net unit price, so that is what gets multiplied. The
// gross total may then differ by a rappen from the consumer calculation of the
// same product — deliberately, because each side's total must be an exact
// multiple of the figure it was shown.
func TestBusinessLineIsAMultipleOfTheQuotedNetPrice(t *testing.T) {
	b := LineForBusiness(4900, 3, RateStandard)
	p := LineForPrivate(4900, 3, RateStandard)

	unitNet := NetFromGross(4900, RateStandard) // 4533
	if b.Net != unitNet*3 {
		t.Errorf("business net = %d, want %d — the quoted net price was not what got multiplied",
			b.Net, unitNet*3)
	}
	if b.Net+b.Tax != b.Gross {
		t.Errorf("net %d + tax %d != gross %d", b.Net, b.Tax, b.Gross)
	}
	// The two must not be assumed equal anywhere in the shop.
	if b.Gross == p.Gross && b.Net == p.Net {
		t.Log("both calculations agree on this value; that is allowed, not required")
	}
}

// Zero rate is not a special case in the formulas, but a shop below the VAT
// threshold shows nothing but the plain price and must not lose a rappen to it.
func TestExemptRateLeavesTheAmountUntouched(t *testing.T) {
	l := LineForPrivate(4900, 3, RateExempt)
	if l.Gross != 14700 || l.Net != 14700 || l.Tax != 0 {
		t.Errorf("exempt line = net %d, tax %d, gross %d; want 14700/0/14700", l.Net, l.Tax, l.Gross)
	}
}

// An invoice may not show one lump of tax when several rates are involved, and
// the summary has to equal the sum of the lines it summarises.
func TestSummarizeGroupsByRateAndKeepsTheSum(t *testing.T) {
	lines := []Line{
		LineForPrivate(4900, 3, RateStandard),
		LineForPrivate(1250, 2, RateReduced),
		LineForPrivate(999, 1, RateStandard),
	}
	groups := Summarize(lines)

	if len(groups) != 2 {
		t.Fatalf("got %d rate groups, want 2", len(groups))
	}

	var lineNet, lineTax, groupNet, groupTax Amount
	for _, l := range lines {
		lineNet += l.Net
		lineTax += l.Tax
	}
	for _, g := range groups {
		groupNet += g.Net
		groupTax += g.Tax
	}
	if groupNet != lineNet || groupTax != lineTax {
		t.Errorf("summary %d/%d does not equal the lines %d/%d", groupNet, groupTax, lineNet, lineTax)
	}
}

// Swiss francs are written CHF 1’234.55, with the typographic apostrophe. The
// ASCII quote is what a keyboard produces and what makes a price list look
// like terminal output.
//
// The space between the code and the number is a non-breaking one, deliberately:
// a price that wraps after "CHF" at the end of a line is a price that has to be
// read twice. The literals below carry U+00A0 for that reason.
func TestFormatFollowsTheCurrency(t *testing.T) {
	chf := CurrencyFor("CHF")
	eur := CurrencyFor("EUR")

	cases := []struct {
		currency Currency
		amount   Amount
		want     string
	}{
		{chf, 123455, "CHF\u00a01’234.55"},
		{chf, 4900, "CHF\u00a049.00"},
		{chf, 5, "CHF\u00a00.05"},
		{chf, 0, "CHF\u00a00.00"},
		{chf, 123456789, "CHF\u00a01’234’567.89"},
		{chf, -4900, "CHF\u00a0−49.00"},
		{eur, 123455, "1.234,55\u00a0€"},
	}
	for _, c := range cases {
		if got := c.currency.Format(c.amount); got != c.want {
			t.Errorf("Format(%d) = %q, want %q", c.amount, got, c.want)
		}
	}

	// An unknown code must not produce an empty currency on a price tag.
	if got := CurrencyFor("XYZ").Format(4900); got != "CHF\u00a049.00" {
		t.Errorf("unknown currency fell back to %q", got)
	}
}

// The same person types 1'234.50, 1234.5 and 1234,50 on different days.
func TestParseAcceptsWhatPeopleType(t *testing.T) {
	cases := map[string]Amount{
		"49":          4900,
		"49.":         4900,
		"49.5":        4950,
		"49.50":       4950,
		"49,50":       4950,
		"1'234.50":    123450,
		"1’234.50":    123450,
		"1 234.50":    123450,
		"1.234,50":    123450,
		"CHF 49.50":   4950,
		"49.50 CHF":   4950,
		"  49.50  ":   4950,
		"-49.50":      -4950,
		"0":           0,
		"":            0,
		"0.05":        5,
		"1'000'000":   100000000,
		"Fr. 1'234.5": 123450,
	}
	for input, want := range cases {
		got, err := ParseAmount(input)
		if err != nil {
			t.Errorf("ParseAmount(%q): %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("ParseAmount(%q) = %d, want %d", input, got, want)
		}
	}

	for _, bad := range []string{"abc", "49.505", "1..2"} {
		if _, err := ParseAmount(bad); err == nil {
			t.Errorf("ParseAmount(%q) was accepted", bad)
		}
	}
}

// The formatted price must never go back into a form field: it carries a
// typographic apostrophe, and the round trip through a browser would turn a
// price into a parse error.
func TestInputRoundTripsThroughParse(t *testing.T) {
	for _, a := range []Amount{0, 5, 4900, 123455, -4950} {
		got, err := ParseAmount(Input(a))
		if err != nil {
			t.Errorf("Input(%d) = %q, which does not parse: %v", a, Input(a), err)
			continue
		}
		if got != a {
			t.Errorf("round trip of %d gave %d (via %q)", a, got, Input(a))
		}
	}
}
