package user

import (
	"context"
	"testing"
)

// A SetRights that fails must leave the old rights standing, not half of the
// new ones.
//
// SetRights runs three statements without a transaction: the publishing right,
// DELETE of every assignment, then one INSERT per website. When an INSERT fails
// the DELETE is already written, and an editor with no rows is an editor of
// every website. Before Phase 10 only a person at the user form could reach
// that; since Phase 10 the forward-auth sync calls SetRights unattended at every
// sign-in, and a website group pointing at a deleted website fails exactly this
// INSERT, on the foreign key.
func TestAFailedSetRightsLeavesTheOldRights(t *testing.T) {
	s, id := newTestStore(t)
	ctx := context.Background()
	if err := websites(ctx, s, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRights(ctx, id, Rights{MayPublish: true, Websites: []int64{1}}); err != nil {
		t.Fatalf("limit the editor to website 1: %v", err)
	}

	// 99999 names no website, so its INSERT fails on the foreign key.
	if err := s.SetRights(ctx, id, Rights{MayPublish: false, Websites: []int64{99999}}); err == nil {
		t.Fatal("SetRights accepted a website that does not exist; the failure this test is " +
			"about never happened")
	}

	rights, err := s.Rights(ctx, id)
	if err != nil {
		t.Fatalf("Rights: %v", err)
	}
	if rights.MayUse(2) {
		t.Errorf("after a failed SetRights the editor reaches website 2, which was never theirs — "+
			"the DELETE was written, the INSERT failed, and no rows read as every website (%+v)", rights)
	}
	if !rights.MayUse(1) {
		t.Errorf("a failed SetRights took website 1 away (%+v)", rights)
	}
	if !rights.MayPublish {
		t.Error("a failed SetRights still withdrew the publishing right — half of the change was written")
	}
}
