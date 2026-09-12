package user

import (
	"context"
	"testing"
)

// The rule everything hangs on: no assignment means every website. Otherwise
// the migration itself would be a lockout.
func TestWithoutAnAssignmentEveryWebsite(t *testing.T) {
	s, id := newTestStore(t)
	rights, err := s.Rights(context.Background(), id)
	if err != nil {
		t.Fatalf("Rights: %v", err)
	}
	if rights.Limited() {
		t.Error("a fresh account is limited and should not be")
	}
	if !rights.MayUse(1) || !rights.MayUse(999) {
		t.Error("with no mapping every website has to be allowed")
	}
	if !rights.MayPublish {
		t.Error("a fresh account must not publish — the migration takes something away")
	}
}

func TestTheAssignmentNarrows(t *testing.T) {
	s, id := newTestStore(t)
	ctx := context.Background()
	if err := websites(ctx, s, 3); err != nil {
		t.Fatal(err)
	}

	if err := s.SetRights(ctx, id, Rights{MayPublish: false, Websites: []int64{2}}); err != nil {
		t.Fatalf("SetRights: %v", err)
	}
	rights, err := s.Rights(ctx, id)
	if err != nil {
		t.Fatalf("Rights: %v", err)
	}
	if !rights.Limited() || rights.MayUse(1) || !rights.MayUse(2) || rights.MayUse(3) {
		t.Errorf("Zuordnung = %+v", rights)
	}
	if rights.MayPublish {
		t.Error("the right to publish was not withdrawn")
	}

	// And undone again: no row means everything again.
	if err := s.SetRights(ctx, id, Everything()); err != nil {
		t.Fatalf("SetRights: %v", err)
	}
	rights, _ = s.Rights(ctx, id)
	if rights.Limited() || !rights.MayPublish {
		t.Errorf("after lifting it = %+v", rights)
	}
}

// An administrator runs the installation. A website they may not enter would be
// one nobody can repair — the assignment does not apply to them.
func TestAdministratorKennnKeineGrenze(t *testing.T) {
	s, id := newTestStore(t)
	ctx := context.Background()
	if err := websites(ctx, s, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRights(ctx, id, Rights{Websites: []int64{1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Write.ExecContext(ctx,
		`UPDATE users SET role = 'admin' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}

	rights, err := s.Rights(ctx, id)
	if err != nil {
		t.Fatalf("Rights: %v", err)
	}
	if rights.Limited() || !rights.MayPublish || !rights.MayUse(2) {
		t.Errorf("an administrator is limited: %+v", rights)
	}
}

// websites creates n websites, so that the assignment's foreign keys hold.
func websites(ctx context.Context, s *Store, n int) error {
	for i := 0; i < n; i++ {
		if _, err := s.DB.Write.ExecContext(ctx,
			`INSERT INTO websites (name) VALUES ($1)`, "Website"); err != nil {
			return err
		}
	}
	return nil
}
