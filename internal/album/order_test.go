package album

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// Where the next sort_order is decided.
//
// AddItem used to ask the READ pool how many pictures there are and where the
// last one sits, and then insert lastOrder+1 on the WRITE pool. Under WAL those
// are two connections and two snapshots, so two adds to one album could both
// read MAX = 5 and both insert 6.
//
// A duplicate sort_order is not itself visible: Pictures orders by
// (sort_order, id), which is still deterministic. What it costs is the arrow
// button. SwapSortOrder exchanges two rows' sort_order values, and exchanging
// two equal values writes nothing — so the operator presses "up", reads
// "Reihenfolge geändert", and the list does not move. Whatever they try after
// that has the same answer, and nothing anywhere says why. The same window lets
// MaxItems be exceeded by one.
//
// The fix is the second of the two the review offered: the count and the
// maximum are read INSIDE the write transaction, on the write pool. It is
// preferred over folding the arithmetic into the INSERT because the INSERT
// already reports "not this website's album" through RowsAffected() == 0, and
// adding the cap to the same WHERE would make one answer stand for two
// questions — ErrTooManyItems and ErrNotFound are different sentences to an
// operator. The write pool holds a single connection (db.Open sets
// SetMaxOpenConns(1)), so two AddItem transactions cannot overlap at all, and
// the read sees the previous one committed.

// TestConcurrentAddsGetDistinctSortOrders is the property, and it is the
// arithmetic rather than the timing that it asserts: however the eight adds
// interleave, the eight rows must carry eight different places in the order.
func TestConcurrentAddsGetDistinctSortOrders(t *testing.T) {
	s, database, owner, _ := newTestStore(t)
	ctx := context.Background()
	a := mustCreate(t, s, owner, "Werkstatt")

	const adds = 8
	media := make([]int64, adds)
	for i := range media {
		media[i] = seedMedia(t, database, owner, fmt.Sprintf("bild-%d.jpg", i))
	}

	// All eight ask at once. Without the barrier they would queue up behind one
	// another and the two snapshots would never disagree.
	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)
	errs := make([]error, adds)
	for i := 0; i < adds; i++ {
		done.Add(1)
		go func(i int) {
			defer done.Done()
			start.Wait()
			_, errs[i] = s.AddItem(ctx, owner, a.ID, media[i], fmt.Sprintf("Bild %d", i), "")
		}(i)
	}
	start.Done()
	done.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("AddItem %d: %v", i, err)
		}
	}

	pics, err := s.Pictures(ctx, owner, a.ID)
	if err != nil {
		t.Fatalf("Pictures: %v", err)
	}
	if len(pics) != adds {
		t.Fatalf("%d pictures, want %d", len(pics), adds)
	}
	seen := map[int]int64{}
	for _, p := range pics {
		if other, clash := seen[p.SortOrder]; clash {
			t.Errorf("pictures %d and %d both sit at sort_order %d — the next place was read on a different snapshot than the one it was written on, and the arrow button between them will now report success and move nothing",
				other, p.ID, p.SortOrder)
		}
		seen[p.SortOrder] = p.ID
	}
}

// TestASwapBetweenTwoEqualOrdersIsWhatTheDuplicateCosts makes the damage
// visible rather than inferred: two rows sharing a sort_order, swapped, and the
// list does not move although the call reports success.
//
// The duplicate is written here with raw SQL, because the point is what the
// state costs and not how it arises — and it must keep costing that, so a later
// change to AddItem that reintroduced duplicates would still be caught by the
// test above.
func TestASwapBetweenTwoEqualOrdersIsWhatTheDuplicateCosts(t *testing.T) {
	s, database, owner, _ := newTestStore(t)
	ctx := context.Background()
	a := mustCreate(t, s, owner, "Werkstatt")

	one := mustAdd(t, s, owner, a.ID, seedMedia(t, database, owner, "eins.jpg"), "Eins")
	two := mustAdd(t, s, owner, a.ID, seedMedia(t, database, owner, "zwei.jpg"), "Zwei")
	if _, err := database.Write.Exec(
		`UPDATE album_items SET sort_order = 0 WHERE album_id = $1`, a.ID); err != nil {
		t.Fatalf("force the duplicate: %v", err)
	}

	if err := s.SwapSortOrder(ctx, owner, a.ID, one, two); err != nil {
		t.Fatalf("SwapSortOrder: %v", err)
	}

	pics, err := s.Pictures(ctx, owner, a.ID)
	if err != nil || len(pics) != 2 {
		t.Fatalf("Pictures: %v, %v", pics, err)
	}
	if pics[0].ID != one || pics[1].ID != two {
		t.Fatalf("the fixture is not the state under test: %d, %d", pics[0].ID, pics[1].ID)
	}
	// This is an assertion about the COST, and it is expected to hold: the swap
	// really does nothing here. It is written down so the reason the test above
	// matters is in the file rather than in a commit message.
	if pics[0].SortOrder != pics[1].SortOrder {
		t.Errorf("the forced duplicate did not survive the swap: %d, %d",
			pics[0].SortOrder, pics[1].SortOrder)
	}
}

// TestTheItemCapIsNotExceededByConcurrentAdds is the second thing the window
// bought: adds that all read count == MaxItems-1 and then all insert.
//
// Twenty attempts rather than one, and that is not padding. This race is much
// narrower than the sort_order one — a single attempt tripped it about one time
// in four against the unfixed store — so a one-shot test would be a guard that
// passes for the wrong reason three runs out of four. Twenty attempts made it
// red every time it was run against the pre-fix code, and the fix makes it
// green by construction rather than by luck: the count is read inside the same
// write transaction as the insert, and the write pool holds one connection.
func TestTheItemCapIsNotExceededByConcurrentAdds(t *testing.T) {
	s, database, owner, _ := newTestStore(t)
	ctx := context.Background()

	const attempts = 20
	const tries = 12

	for attempt := 0; attempt < attempts; attempt++ {
		a := mustCreate(t, s, owner, fmt.Sprintf("Werkstatt %d", attempt))

		// Filled with raw SQL rather than through AddItem: 119 write
		// transactions of our own before the race warm the write connection and
		// narrow the very window under test, which made this test pass for a
		// reason that has nothing to do with the property.
		for i := 0; i < MaxItems-1; i++ {
			m := seedMedia(t, database, owner, fmt.Sprintf("voll-%d-%d.jpg", attempt, i))
			if _, err := database.Write.Exec(
				`INSERT INTO album_items (album_id, media_id, alt, caption, sort_order, updated_at)
				 VALUES ($1, $2, 'Voll', '', $3, '2020-01-01T00:00:00Z')`, a.ID, m, i); err != nil {
				t.Fatalf("fill album %d: %v", attempt, err)
			}
		}

		media := make([]int64, tries)
		for i := range media {
			media[i] = seedMedia(t, database, owner, fmt.Sprintf("dazu-%d-%d.jpg", attempt, i))
		}

		var start sync.WaitGroup
		var done sync.WaitGroup
		start.Add(1)
		for i := 0; i < tries; i++ {
			done.Add(1)
			go func(i int) {
				defer done.Done()
				start.Wait()
				// The error is discarded on purpose: all but one of these are
				// SUPPOSED to be refused with ErrTooManyItems, and which ones
				// is exactly what must not depend on timing.
				_, _ = s.AddItem(ctx, owner, a.ID, media[i], "Dazu", "")
			}(i)
		}
		start.Done()
		done.Wait()

		pics, err := s.Pictures(ctx, owner, a.ID)
		if err != nil {
			t.Fatalf("Pictures: %v", err)
		}
		if len(pics) > MaxItems {
			t.Fatalf("attempt %d: the album holds %d pictures and the cap is %d — the count was read on a snapshot older than the insert that acted on it",
				attempt, len(pics), MaxItems)
		}
	}
}
