package user

import (
	"context"
	"errors"
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

// The user form sends the record and the rights together, and they are stored
// together or not at all. Stored one after the other, a refusal of the second
// left the first standing: rights changed by a save the form called failed, or
// a role changed while the websites that belong to it were not.
func TestUpdateWithRightsWritesBothOrNeither(t *testing.T) {
	s, id := newTestStore(t)
	ctx := context.Background()
	if err := websites(ctx, s, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, "Eva", "eva@example.de", "passwort123", RoleEditor); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.SetRights(ctx, id, Rights{Websites: []int64{1}}); err != nil {
		t.Fatalf("limit the editor to website 1: %v", err)
	}

	// unchanged holds the state every refused call has to leave behind.
	unchanged := func(t *testing.T) {
		t.Helper()
		u, err := s.GetByID(ctx, id)
		if err != nil || u == nil {
			t.Fatalf("GetByID: %v", err)
		}
		if u.Name != "Max" || u.Email != "max@example.de" || u.Role != RoleEditor {
			t.Errorf("the record was changed: %q %q %q", u.Name, u.Email, u.Role)
		}
		rights, err := s.Assignment(ctx, id)
		if err != nil {
			t.Fatalf("Assignment: %v", err)
		}
		if rights.MayPublish || !rights.Limited() || len(rights.Websites) != 1 || rights.Websites[0] != 1 {
			t.Errorf("the rights were changed: %+v", rights)
		}
	}

	t.Run("a taken address writes nothing", func(t *testing.T) {
		err := s.UpdateWithRights(ctx, id, "Neu", "eva@example.de", RoleAdmin, Everything())
		if !errors.Is(err, ErrDuplicateEmail) {
			t.Fatalf("got %v, want ErrDuplicateEmail", err)
		}
		unchanged(t)
	})

	t.Run("rights that cannot be stored write no record either", func(t *testing.T) {
		// 99999 names no website, so its INSERT fails on the foreign key —
		// after the record's UPDATE has already run.
		err := s.UpdateWithRights(ctx, id, "Neu", "neu@example.de", RoleAdmin,
			Rights{MayPublish: true, Websites: []int64{99999}})
		if err == nil {
			t.Fatal("a website that does not exist was accepted; the failure this case is about never happened")
		}
		unchanged(t)
	})

	t.Run("somebody who is not there", func(t *testing.T) {
		if err := s.UpdateWithRights(ctx, 4711, "Neu", "neu@example.de", RoleEditor, Nothing()); !errors.Is(err, ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
		unchanged(t)
	})

	t.Run("a valid call writes all of it", func(t *testing.T) {
		if err := s.UpdateWithRights(ctx, id, "  Neu  ", "neu@example.de", RoleEditor,
			Rights{MayPublish: true, Websites: []int64{2}}); err != nil {
			t.Fatalf("UpdateWithRights: %v", err)
		}
		u, err := s.GetByID(ctx, id)
		if err != nil || u == nil {
			t.Fatalf("GetByID: %v", err)
		}
		if u.Name != "Neu" || u.Email != "neu@example.de" || u.Role != RoleEditor {
			t.Errorf("the record was not written: %q %q %q", u.Name, u.Email, u.Role)
		}
		rights, err := s.Assignment(ctx, id)
		if err != nil {
			t.Fatalf("Assignment: %v", err)
		}
		if !rights.MayPublish || len(rights.Websites) != 1 || rights.Websites[0] != 2 {
			t.Errorf("the rights were not written: %+v", rights)
		}
	})
}
