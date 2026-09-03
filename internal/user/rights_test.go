package user

import (
	"context"
	"testing"
)

// Die Regel, an der alles hängt: keine Zuordnung heisst alle Websites. Sonst
// wäre die Migration selbst eine Aussperrung.
func TestOhneZuordnungAlleWebsites(t *testing.T) {
	s, id := newTestStore(t)
	rights, err := s.Rights(context.Background(), id)
	if err != nil {
		t.Fatalf("Rights: %v", err)
	}
	if rights.Limited() {
		t.Error("ein frischer Zugang ist eingeschränkt, sollte er nicht sein")
	}
	if !rights.MayUse(1) || !rights.MayUse(999) {
		t.Error("ohne Zuordnung muss jede Website erlaubt sein")
	}
	if !rights.MayPublish {
		t.Error("ein frischer Zugang darf nicht veröffentlichen — die Migration nimmt etwas weg")
	}
}

func TestZuordnungGrenztEin(t *testing.T) {
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
		t.Error("das Veröffentlichungsrecht wurde nicht entzogen")
	}

	// Und wieder aufheben: keine Zeile heisst wieder alle.
	if err := s.SetRights(ctx, id, Everything()); err != nil {
		t.Fatalf("SetRights: %v", err)
	}
	rights, _ = s.Rights(ctx, id)
	if rights.Limited() || !rights.MayPublish {
		t.Errorf("nach dem Aufheben = %+v", rights)
	}
}

// Ein Administrator führt die Anlage. Eine Website, die er nicht betreten darf,
// wäre eine, die niemand reparieren kann — die Zuordnung gilt für ihn nicht.
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
		t.Errorf("ein Administrator ist eingeschränkt: %+v", rights)
	}
}

// websites legt n Websites an, damit die Fremdschlüssel der Zuordnung halten.
func websites(ctx context.Context, s *Store, n int) error {
	for i := 0; i < n; i++ {
		if _, err := s.DB.Write.ExecContext(ctx,
			`INSERT INTO websites (name) VALUES ($1)`, "Website"); err != nil {
			return err
		}
	}
	return nil
}
