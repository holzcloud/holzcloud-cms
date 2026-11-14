package legal

import (
	"testing"
	"time"
)

func TestOrderPeriodFollowsTheSellersCountry(t *testing.T) {
	for _, c := range []struct {
		country string
		years   int
		known   bool
	}{
		{"CH", 10, true},
		{"ch ", 10, true},
		{"DE", 8, true},
		{"GB", 6, true},
		{"UK", 6, true},
		{"US", 3, true},
		{"AT", 10, false},
		{"", 10, false},
		{"XX", 10, false},
	} {
		p := OrderPeriodFor(c.country)
		if p.Years != c.years || p.Known != c.known {
			t.Errorf("OrderPeriodFor(%q) = %d years, known %v; want %d, %v", c.country, p.Years, p.Known, c.years, c.known)
		}
	}

	keys := map[string]string{}
	for _, country := range []string{"CH", "DE", "GB", "US"} {
		b := OrderPeriodFor(country).Basis()
		if b == "" {
			t.Errorf("%s has no basis", country)
		}
		if was, dup := keys[b]; dup {
			t.Errorf("%s and %s share the basis %q", was, country, b)
		}
		keys[b] = country
	}
	if OrderPeriodFor("UK").Basis() != OrderPeriodFor("GB").Basis() {
		t.Error("UK and GB are one country and must name one law")
	}
	fallback := OrderPeriodFor("AT").Basis()
	if fallback == "" {
		t.Error("the fallback has no sentence")
	}
	if was, dup := keys[fallback]; dup {
		t.Errorf("the fallback says the same as %s: %q", was, fallback)
	}
	if OrderPeriodFor("XX").Basis() != fallback {
		t.Error("every unresearched country gets the same sentence")
	}
}

func TestAnOrderIsDeletableFromTheFirstOfJanuary(t *testing.T) {
	for _, c := range []struct {
		created time.Time
		years   int
		want    time.Time
	}{
		{time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC), 10, time.Date(2037, 1, 1, 0, 0, 0, 0, time.UTC)},
		{time.Date(2026, 12, 31, 23, 30, 0, 0, time.UTC), 8, time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC)},
		// Counted in UTC, the clock the order was stored on.
		{time.Date(2027, 1, 1, 0, 30, 0, 0, time.FixedZone("CET", 3600)), 3, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)},
	} {
		if got := OrderDeletableFrom(c.created, c.years); !got.Equal(c.want) {
			t.Errorf("OrderDeletableFrom(%v, %d) = %v, want %v", c.created, c.years, got, c.want)
		}
	}
}

func TestTheCutoffAgreesWithTheDeletableDate(t *testing.T) {
	var created, nows []time.Time
	for y := 2010; y <= 2030; y++ {
		for _, md := range [][2]int{{1, 1}, {6, 15}, {12, 31}} {
			created = append(created, time.Date(y, time.Month(md[0]), md[1], 12, 0, 0, 0, time.UTC))
			nows = append(nows, time.Date(y, time.Month(md[0]), md[1], 0, 0, 0, 0, time.UTC))
		}
	}
	for _, y := range []int{3, 6, 8, 10} {
		for _, c := range created {
			for _, now := range nows {
				expired := c.Before(OrderCutoff(now, y))
				deletable := !now.Before(OrderDeletableFrom(c, y))
				if expired != deletable {
					t.Fatalf("created %v, now %v, %d years: the cut-off says %v, the deletable date %v",
						c, now, y, expired, deletable)
				}
			}
		}
	}
}
