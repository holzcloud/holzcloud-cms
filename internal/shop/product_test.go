package shop

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/money"
)

func newTestStore(t *testing.T) (*Store, int64) {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	res, err := database.Write.Exec(
		`INSERT INTO websites (name, description) VALUES ('Test', '')`)
	if err != nil {
		t.Fatalf("insert website: %v", err)
	}
	websiteID, _ := res.LastInsertId()

	return NewStore(database), websiteID
}

func seedProduct(t *testing.T, s *Store, websiteID int64, slug, status string, stock *int) *Product {
	t.Helper()
	p := &Product{
		WebsiteID:  websiteID,
		Slug:       slug,
		Title:      slug,
		PriceGross: 4900,
		TaxRate:    money.RateStandard,
		Stock:      stock,
		Status:     status,
	}
	id, err := s.Create(context.Background(), p)
	if err != nil {
		t.Fatalf("Create(%s): %v", slug, err)
	}
	p.ID = id
	return p
}

func intp(v int) *int { return &v }

// A draft must never reach a visitor. The status condition lives in the query
// rather than in a check afterwards, and that is what this asserts — a filter
// applied in Go is one a second call site forgets.
func TestDraftsNeverReachThePublicQueries(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()

	seedProduct(t, s, ws, "sichtbar", StatusPublished, nil)
	seedProduct(t, s, ws, "entwurf", StatusDraft, nil)

	got, err := s.GetPublished(ctx, ws, "entwurf")
	if err != nil {
		t.Fatalf("GetPublished: %v", err)
	}
	if got != nil {
		t.Error("a draft product was served to a visitor")
	}

	list, err := s.ListPublished(ctx, ws, 50, 0)
	if err != nil {
		t.Fatalf("ListPublished: %v", err)
	}
	for _, p := range list {
		if p.Slug == "entwurf" {
			t.Error("a draft product appeared in the catalogue")
		}
	}
	if n, _ := s.CountPublished(ctx, ws); n != 1 {
		t.Errorf("CountPublished = %d, want 1 — the pager counted a draft", n)
	}
}

// One website's catalogue must not show another's, even at the same path.
func TestProductsAreScopedToTheirWebsite(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()

	res, err := s.DB.Write.Exec(`INSERT INTO websites (name, description) VALUES ('Zweite', '')`)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := res.LastInsertId()

	seedProduct(t, s, ws, "tisch", StatusPublished, nil)
	seedProduct(t, s, other, "tisch", StatusPublished, nil)

	got, err := s.GetPublished(ctx, other, "tisch")
	if err != nil {
		t.Fatalf("GetPublished: %v", err)
	}
	if got == nil || got.WebsiteID != other {
		t.Fatalf("got the wrong website's product: %+v", got)
	}
}

// The same path twice on one website would make one of the two unreachable.
func TestDuplicateSlugIsRefusedWithAUsableError(t *testing.T) {
	s, ws := newTestStore(t)
	seedProduct(t, s, ws, "tisch", StatusPublished, nil)

	_, err := s.Create(context.Background(), &Product{
		WebsiteID: ws, Slug: "tisch", Title: "Noch ein Tisch", Status: StatusDraft,
	})
	if err != ErrSlugTaken {
		t.Errorf("second product at the same path: got %v, want ErrSlugTaken", err)
	}
}

// Untracked stock is not "sold out". A workshop that builds to order has no
// quantity, and refusing the sale because a column is NULL would be the CMS
// inventing a rule nobody set.
func TestOrderableDistinguishesUntrackedStockFromNone(t *testing.T) {
	cases := []struct {
		name   string
		status string
		stock  *int
		want   bool
	}{
		{"veröffentlicht, ohne Mengenverwaltung", StatusPublished, nil, true},
		{"veröffentlicht, drei auf Lager", StatusPublished, intp(3), true},
		{"veröffentlicht, ausverkauft", StatusPublished, intp(0), false},
		{"Entwurf", StatusDraft, intp(3), false},
	}
	for _, c := range cases {
		p := &Product{Status: c.status, Stock: c.stock}
		if got := p.Orderable(); got != c.want {
			t.Errorf("%s: Orderable() = %v, want %v", c.name, got, c.want)
		}
	}
}

// Two orders for the last item must not both succeed. The guard is in the
// WHERE clause; in Go both would read stock = 1 and both write 0, and the
// second sale would be one nobody can ship.
//
// The buyers are released from a barrier rather than started in a loop.
// Spawning them one after another was not enough: each goroutine finished
// before the next was scheduled, so the overlap the test is about never
// happened, and a deliberately broken read-then-write implementation passed it.
// A test that a bug walks through is worse than no test — with the barrier the
// broken version loses the last item twice within five rounds.
func TestStockCannotGoNegativeUnderConcurrency(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()

	// Several rounds, because a race that needs a particular interleaving does
	// not show up on the first try.
	for round := 0; round < 25; round++ {
		p := seedProduct(t, s, ws, fmt.Sprintf("letztes-stueck-%d", round), StatusPublished, intp(1))

		const buyers = 8
		start := make(chan struct{})
		var ready, done sync.WaitGroup
		ready.Add(buyers)
		done.Add(buyers)

		results := make([]bool, buyers)
		errs := make([]error, buyers)
		for i := 0; i < buyers; i++ {
			go func(i int) {
				defer done.Done()
				ready.Done()
				<-start // everyone waits here, then they all go at once
				results[i], errs[i] = s.AdjustStock(ctx, p.ID, -1)
			}(i)
		}
		ready.Wait()
		close(start)
		done.Wait()

		won := 0
		for i, ok := range results {
			if errs[i] != nil {
				t.Fatalf("AdjustStock: %v", errs[i])
			}
			if ok {
				won++
			}
		}
		if won != 1 {
			t.Fatalf("round %d: %d of %d buyers got the last item; exactly one may",
				round, won, buyers)
		}

		after, _ := s.Get(ctx, p.ID)
		if after.Stock == nil || *after.Stock != 0 {
			t.Fatalf("round %d: stock ended at %v, want 0", round, after.Stock)
		}
	}
}

// A product without stock tracking has nothing to decrement, and saying so is
// different from failing.
func TestAdjustStockLeavesUntrackedProductsAlone(t *testing.T) {
	s, ws := newTestStore(t)
	p := seedProduct(t, s, ws, "auf-mass", StatusPublished, nil)

	ok, err := s.AdjustStock(context.Background(), p.ID, -1)
	if err != nil {
		t.Fatalf("AdjustStock: %v", err)
	}
	if ok {
		t.Error("an untracked product reported a stock change")
	}
	after, _ := s.Get(context.Background(), p.ID)
	if after.Stock != nil {
		t.Errorf("stock became %v; it must stay untracked", *after.Stock)
	}
}

// Price and rate have to survive the round trip through the database exactly —
// this is the value an invoice is built from.
func TestPriceAndRateRoundTripExactly(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()

	for _, rate := range []money.TaxRate{money.RateStandard, money.RateReduced, money.RateLodging, money.RateExempt} {
		p := &Product{
			WebsiteID: ws, Slug: "p" + rate.String(), Title: "T",
			PriceGross: 123455, TaxRate: rate, Status: StatusPublished,
		}
		id, err := s.Create(ctx, p)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := s.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.PriceGross != 123455 || got.TaxRate != rate {
			t.Errorf("round trip gave %d/%s, want 123455/%s", got.PriceGross, got.TaxRate, rate)
		}
	}
}

// Categories reuse the site's existing labels; a product listing by label is
// what makes them worth having.
func TestProductsCanBeListedByTheirLabel(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()

	res, err := s.DB.Write.Exec(
		`INSERT INTO terms (website_id, name, slug) VALUES ($1, 'Eiche', 'eiche')`, ws)
	if err != nil {
		t.Fatal(err)
	}
	termID, _ := res.LastInsertId()

	tisch := seedProduct(t, s, ws, "tisch", StatusPublished, nil)
	seedProduct(t, s, ws, "stuhl", StatusPublished, nil)
	entwurf := seedProduct(t, s, ws, "bank", StatusDraft, nil)

	for _, id := range []int64{tisch.ID, entwurf.ID} {
		if err := s.SetTerms(ctx, id, []int64{termID}); err != nil {
			t.Fatalf("SetTerms: %v", err)
		}
	}

	list, err := s.ListPublishedByTerm(ctx, ws, termID, 50, 0)
	if err != nil {
		t.Fatalf("ListPublishedByTerm: %v", err)
	}
	if len(list) != 1 || list[0].Slug != "tisch" {
		t.Errorf("label listing = %v, want just the published tisch", slugs(list))
	}
	if n, _ := s.CountPublishedByTerm(ctx, ws, termID); n != 1 {
		t.Errorf("CountPublishedByTerm = %d, want 1", n)
	}

	// Replacing the labels must not leave the old ones behind.
	if err := s.SetTerms(ctx, tisch.ID, nil); err != nil {
		t.Fatalf("SetTerms(nil): %v", err)
	}
	if n, _ := s.CountPublishedByTerm(ctx, ws, termID); n != 0 {
		t.Errorf("after clearing the labels the product is still listed under one")
	}
}

func slugs(ps []*Product) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Slug)
	}
	return out
}
