package web

import (
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// The address is the source. When that goes wrong, the sidebar shows the
// sections of the wrong website — and plausibly enough that somebody notices
// only once they have changed something on the wrong page.
func TestTheWebsiteOutOfTheAddress(t *testing.T) {
	cases := []struct {
		pfad string
		will int64
	}{
		{"/admin/websites/2/pages", 2},
		{"/admin/websites/17", 17},
		{"/admin/websites/2/pages/5/edit", 2},
		{"/admin/websites", 0},
		{"/admin/websites/neu", 0},
		{"/admin/users", 0},
		{"/admin/", 0},
		// No way to smuggle anything but a number in through the address.
		{"/admin/websites/2x/pages", 0},
		{"/admin/websites/-1/pages", 0},
	}
	for _, f := range cases {
		if got := websiteFromPath(f.pfad); got != f.will {
			t.Errorf("websiteFromPath(%q) = %d, want %d", f.pfad, got, f.will)
		}
	}
}

// Without a match the first website: better than no menu at all, and whoever
// has only one website — the usual case — never notices the choice.
func TestAChoiceFallsBackToTheFirst(t *testing.T) {
	liste := websitesMit(3, 7, 9)

	if ws := pick(liste, 7); ws == nil || ws.ID != 7 {
		t.Errorf("pick(7) = %v, want 7", ws)
	}
	if ws := pick(liste, 0); ws == nil || ws.ID != 3 {
		t.Errorf("pick(0) = %v, want 3", ws)
	}
	// A website that no longer exists — just deleted, say, while it still
	// stands in the session.
	if ws := pick(liste, 999); ws == nil || ws.ID != 3 {
		t.Errorf("pick(999) = %v, want 3", ws)
	}
	if ws := pick(nil, 5); ws != nil {
		t.Errorf("pick auf leerer Liste = %v, want nil", ws)
	}
}

// pick hands back a pointer into the list. If every call pointed at the same
// loop variable, every request would get the website seen last.
func TestAChoicePointsAtTheRightWebsite(t *testing.T) {
	liste := websitesMit(1, 2, 3)
	a, b := pick(liste, 1), pick(liste, 3)
	if a.ID != 1 || b.ID != 3 {
		t.Errorf("pick lieferte %d und %d, want 1 und 3", a.ID, b.ID)
	}
}

func websitesMit(ids ...int64) []domain.Website {
	out := make([]domain.Website, 0, len(ids))
	for _, id := range ids {
		out = append(out, domain.Website{ID: id})
	}
	return out
}
