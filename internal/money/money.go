// Package money is the shop's arithmetic: amounts in the smallest unit, tax in
// basis points, and one place where rounding happens.
//
// Every amount in this package is an integer of the currency's smallest unit —
// rappen for CHF, cents for EUR. Nothing here is ever a float. A price held in
// a floating-point number is a price that eventually appears on an invoice as
// 19.999999, and no amount of formatting hides that the sum no longer adds up.
package money

import (
	"fmt"
	"strconv"
	"strings"
)

// Amount is a value in the smallest unit of its currency.
type Amount int64

// TaxRate is a VAT rate in basis points: 810 is 8.1 %.
//
// Basis points rather than whole percent because Switzerland's rates all have a
// decimal place — 8.1 %, 2.6 %, 3.8 %. An integer percent could not hold any of
// them, and a float would reintroduce exactly the rounding this package exists
// to prevent.
type TaxRate int

// The Swiss rates, current since 1 January 2024.
const (
	RateStandard  TaxRate = 810 // Normalsatz
	RateReduced   TaxRate = 260 // Reduzierter Satz: Lebensmittel, Bücher, Medikamente
	RateLodging   TaxRate = 380 // Sondersatz Beherbergung
	RateExempt    TaxRate = 0   // von der Steuer ausgenommen oder befreit
	basisPointsIn         = 10000
)

// SwissRates are the rates offered in the admin, in the order they are shown.
var SwissRates = []struct {
	Rate  TaxRate
	Label string
}{
	{RateStandard, "8.1 % (Normalsatz)"},
	{RateReduced, "2.6 % (reduziert: Lebensmittel, Bücher, Medikamente)"},
	{RateLodging, "3.8 % (Beherbergung)"},
	{RateExempt, "0 % (ausgenommen oder befreit)"},
}

// KnownRate reports whether a value is one of the offered rates.
func KnownRate(r TaxRate) bool {
	for _, known := range SwissRates {
		if known.Rate == r {
			return true
		}
	}
	return false
}

// String renders a rate the way it is printed on an invoice: "8.1 %".
func (r TaxRate) String() string {
	whole := int(r) / 100
	frac := int(r) % 100
	if frac == 0 {
		return strconv.Itoa(whole) + " %"
	}
	if frac%10 == 0 {
		return fmt.Sprintf("%d.%d %%", whole, frac/10)
	}
	return fmt.Sprintf("%d.%02d %%", whole, frac)
}

// NetFromGross removes the tax from a gross amount.
//
// Rounding is half away from zero, which is what a Swiss invoice does and what
// every accountant checking the figures expects. The tax is then the remainder
// rather than a second rounded calculation — that is what guarantees
// net + tax == gross exactly, on every line, without a correction row.
func NetFromGross(gross Amount, rate TaxRate) Amount {
	if rate <= 0 {
		return gross
	}
	return divRound(int64(gross)*basisPointsIn, int64(basisPointsIn+int(rate)))
}

// TaxFromGross is the tax contained in a gross amount.
func TaxFromGross(gross Amount, rate TaxRate) Amount {
	return gross - NetFromGross(gross, rate)
}

// GrossFromNet adds the tax to a net amount.
func GrossFromNet(net Amount, rate TaxRate) Amount {
	if rate <= 0 {
		return net
	}
	return net + divRound(int64(net)*int64(rate), basisPointsIn)
}

// divRound divides and rounds half away from zero.
//
// Go's integer division truncates towards zero, so 155/10 is 15 and -155/10 is
// -15; both are wrong for money by half a unit. Adding half the divisor with
// the sign of the numerator fixes both directions.
func divRound(numerator, denominator int64) Amount {
	if denominator == 0 {
		return 0
	}
	half := denominator / 2
	if (numerator < 0) != (denominator < 0) {
		half = -half
	}
	return Amount((numerator + half) / denominator)
}

// Line is one row of a basket or an invoice, already resolved to whole amounts.
//
// Net, Tax and Gross always satisfy Net + Tax == Gross. Which of the three is
// computed first depends on who is looking — see LineForPrivate and
// LineForBusiness — but the identity holds either way, because whichever two
// are computed, the third is their difference.
type Line struct {
	Quantity int
	Rate     TaxRate
	// UnitGross is the price as stored on the product.
	UnitGross Amount
	// UnitNet is what a business customer is quoted per piece.
	UnitNet Amount
	Net     Amount
	Tax     Amount
	Gross   Amount
}

// LineForPrivate totals a line the way a consumer shop must.
//
// The advertised gross price is multiplied by the quantity and the tax is
// carved out of the result. Three items at 49.00 cost exactly 147.00 — which is
// what the customer counted on, and what the Preisbekanntgabeverordnung means
// by the price actually payable.
func LineForPrivate(unitGross Amount, quantity int, rate TaxRate) Line {
	gross := unitGross * Amount(quantity)
	net := NetFromGross(gross, rate)
	return Line{
		Quantity:  quantity,
		Rate:      rate,
		UnitGross: unitGross,
		UnitNet:   NetFromGross(unitGross, rate),
		Net:       net,
		Tax:       gross - net,
		Gross:     gross,
	}
}

// LineForBusiness totals a line the way a trade shop must.
//
// The net unit price is what was quoted, so it is multiplied first and the tax
// added to the sum. The gross total can then differ by a rappen or two from the
// consumer calculation of the same product — that is not a defect. Each side
// gets a total that is an exact multiple of the price it was shown, which is
// the figure its own bookkeeping will check.
func LineForBusiness(unitGross Amount, quantity int, rate TaxRate) Line {
	unitNet := NetFromGross(unitGross, rate)
	net := unitNet * Amount(quantity)
	gross := GrossFromNet(net, rate)
	return Line{
		Quantity:  quantity,
		Rate:      rate,
		UnitGross: unitGross,
		UnitNet:   unitNet,
		Net:       net,
		Tax:       gross - net,
		Gross:     gross,
	}
}

// TaxBreakdown is the per-rate summary an invoice has to show.
type TaxBreakdown struct {
	Rate TaxRate
	Net  Amount
	Tax  Amount
}

// Summarize groups lines by rate.
//
// An invoice may not show one lump of tax when several rates are involved: the
// recipient has to be able to see which part was taxed at which rate. Summing
// the already-rounded line figures — rather than recomputing from the total —
// is what keeps the summary equal to the sum of the lines.
func Summarize(lines []Line) []TaxBreakdown {
	var out []TaxBreakdown
	for _, l := range lines {
		found := false
		for i := range out {
			if out[i].Rate == l.Rate {
				out[i].Net += l.Net
				out[i].Tax += l.Tax
				found = true
				break
			}
		}
		if !found {
			out = append(out, TaxBreakdown{Rate: l.Rate, Net: l.Net, Tax: l.Tax})
		}
	}
	return out
}

// Currency is how a currency is written.
type Currency struct {
	Code string
	// Thousands and Decimal are the separators. Switzerland writes
	// CHF 1'234.55; the euro area writes 1.234,55 €.
	Thousands string
	Decimal   string
	// SymbolFirst puts the code before the number, which is what CHF does.
	SymbolFirst bool
}

var currencies = map[string]Currency{
	"CHF": {Code: "CHF", Thousands: "’", Decimal: ".", SymbolFirst: true},
	"EUR": {Code: "€", Thousands: ".", Decimal: ",", SymbolFirst: false},
}

// CurrencyFor returns the writing rules for a code, defaulting to Swiss francs.
func CurrencyFor(code string) Currency {
	if c, ok := currencies[strings.ToUpper(strings.TrimSpace(code))]; ok {
		return c
	}
	return currencies["CHF"]
}

// Format writes an amount the way the currency is written.
//
// The Swiss thousands separator is the typographic apostrophe U+2019, not the
// ASCII quote: CHF 1’234.55. The ASCII one is what a keyboard produces and what
// makes a price list look like it was typed in a terminal.
func (c Currency) Format(a Amount) string {
	negative := a < 0
	if negative {
		a = -a
	}

	whole := int64(a) / 100
	frac := int64(a) % 100

	var b strings.Builder
	digits := strconv.FormatInt(whole, 10)
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteString(c.Thousands)
		}
		b.WriteRune(r)
	}
	number := b.String() + c.Decimal + fmt.Sprintf("%02d", frac)
	if negative {
		number = "−" + number
	}

	if c.SymbolFirst {
		return c.Code + " " + number
	}
	return number + " " + c.Code
}

// ParseAmount reads what an operator typed into a price field.
//
// Both separators are accepted in either role, because the same person writes
// 1'234.50, 1234.5 and 1234,50 on different days and all three mean the same
// thing. An apostrophe or a space is always a grouping mark; the last dot or
// comma is the decimal point.
func ParseAmount(input string) (Amount, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return 0, nil
	}
	// Currency codes and symbols people paste in along with the number.
	for _, drop := range []string{"CHF", "chf", "Fr.", "€", "EUR"} {
		s = strings.ReplaceAll(s, drop, "")
	}
	// Grouping marks carry no meaning.
	for _, drop := range []string{"’", "'", " ", " ", " "} {
		s = strings.ReplaceAll(s, drop, "")
	}
	s = strings.TrimSpace(s)

	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")

	// What is left may only be digits and separators, and no two separators may
	// stand together. Without this "1..2" was read as 1.20: the whole part is
	// stripped of separators further down, so the second dot simply vanished
	// and a typo became a plausible price.
	prevSeparator := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			prevSeparator = false
		case r == '.' || r == ',':
			if prevSeparator {
				return 0, fmt.Errorf("zwei Trennzeichen nebeneinander: %q", input)
			}
			prevSeparator = true
		default:
			return 0, fmt.Errorf("keine Zahl: %q", input)
		}
	}

	// Whichever separator comes last is the decimal point; anything before it
	// was grouping that survived the pass above.
	cut := strings.LastIndexAny(s, ".,")
	whole, frac := s, ""
	if cut >= 0 {
		whole, frac = s[:cut], s[cut+1:]
		whole = strings.NewReplacer(".", "", ",", "").Replace(whole)
	}
	if whole == "" {
		whole = "0"
	}

	switch len(frac) {
	case 0:
		frac = "00"
	case 1:
		frac += "0"
	case 2:
	default:
		return 0, fmt.Errorf("mehr als zwei Nachkommastellen: %q", input)
	}

	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("keine Zahl: %q", input)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("keine Zahl: %q", input)
	}

	total := Amount(w*100 + f)
	if negative {
		total = -total
	}
	return total, nil
}

// Input renders an amount for an edit field: plain, no grouping, a dot.
//
// The formatted form must not go back into a form field. It contains a
// non-breaking space and a typographic apostrophe, and the round trip through a
// browser turns a price into a parse error.
func Input(a Amount) string {
	return fmt.Sprintf("%d.%02d", int64(a)/100, abs(int64(a)%100))
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
