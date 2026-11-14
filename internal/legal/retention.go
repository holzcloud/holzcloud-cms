package legal

// How long a seller keeps an order (Phase 33.2, D-06).
//
// The period follows the seller's country — the country of the website's own
// address — and not the frameworks the website's visitors are protected by:
// bookkeeping and tax law bind the business where it sits, whoever its
// customers are. That is the owner's decision of 2026-11-14 (D-06), replacing
// the earlier "longest of the chosen frameworks" rule.
//
// The periods are the ones 33.2-RESEARCH.md §B.2 established:
//
//   - Switzerland, 10 years: business books and booking vouchers, Art. 958f
//     CO; VAT records until the absolute limitation, Art. 70(2) with
//     Art. 42(6) VAT Act.
//   - Germany, 8 years: booking vouchers and copies of invoices, § 147(3) AO,
//     § 257(4) HGB and § 14b UStG, as amended for 2025 (BEG IV; ten years for
//     the books themselves, which this CMS does not keep).
//   - United Kingdom, 6 years: HMRC VAT Notice 700/21 and the GOV.UK guidance
//     on company records.
//   - United States, 3 years: the federal period of limitations the IRS names
//     as the default; state sales-tax rules were not researched.
//
// Any other country gets the longest of these, ten years, and the screen says
// that no period was researched for it, so the operator can set a longer one.
//
// The period starts at the end of the calendar year of the order (§B.3 Rule
// 2): an order may be deleted from 1 January of its year plus the period plus
// one. That is exact for Germany; for a business year that is not the calendar
// year it can end up to a year early, and the screen says so.

import (
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// OrderPeriod is how long a seller in one country keeps its orders.
type OrderPeriod struct {
	// Country is the code as looked up: trimmed, upper case.
	Country string
	Years   int
	// Known is false for a country no period was researched for.
	Known bool
}

// fallbackYears is the period of a country nobody researched: the longest of
// the researched ones.
const fallbackYears = 10

// The sentences that name each period's law, catalogue keys. One whole
// sentence per country with its citation inside, so a translator sees the
// citation where it stands.
var (
	basisCH   = i18n.N("Switzerland: business records and booking vouchers are kept for ten years (Art. 958f CO; Art. 70 VAT Act).")
	basisDE   = i18n.N("Germany: booking vouchers and copies of invoices are kept for eight years (§ 147 AO, § 257 HGB, § 14b UStG).")
	basisUK   = i18n.N("United Kingdom: VAT and company records are kept for six years (HMRC VAT Notice 700/21).")
	basisUS   = i18n.N("United States: tax records are kept for three years, the federal period of limitations (IRS).")
	basisNone = i18n.N("No retention period was researched for the country in this website's address, so the longest researched one, ten years, applies. Set a longer period if your law requires one.")
)

// OrderPeriodFor is the order period of a seller in the country, an ISO
// 3166 code in any letter case. GB and UK are the same country.
func OrderPeriodFor(country string) OrderPeriod {
	c := strings.ToUpper(strings.TrimSpace(country))
	p := OrderPeriod{Country: c, Known: true}
	switch c {
	case "CH":
		p.Years = 10
	case "DE":
		p.Years = 8
	case "GB", "UK":
		p.Years = 6
	case "US":
		p.Years = 3
	default:
		p.Years, p.Known = fallbackYears, false
	}
	return p
}

// Basis is the sentence that names the period's law, a catalogue key.
func (p OrderPeriod) Basis() string {
	switch p.Country {
	case "CH":
		return basisCH
	case "DE":
		return basisDE
	case "GB", "UK":
		return basisUK
	case "US":
		return basisUS
	}
	return basisNone
}

// OrderCutoff is the moment before which an order placed has expired at now:
// 1 January of now's year, UTC, minus the years. An order placed before it
// is past the end of the calendar year of the order plus the period.
func OrderCutoff(now time.Time, years int) time.Time {
	return time.Date(now.UTC().Year()-years, time.January, 1, 0, 0, 0, 0, time.UTC)
}

// OrderDeletableFrom is the first moment an order placed at created may be
// deleted: 1 January, UTC, of its year plus the years plus one. It agrees
// with OrderCutoff: created is before OrderCutoff(now) exactly when now is
// not before this.
func OrderDeletableFrom(created time.Time, years int) time.Time {
	return time.Date(created.UTC().Year()+years+1, time.January, 1, 0, 0, 0, 0, time.UTC)
}
