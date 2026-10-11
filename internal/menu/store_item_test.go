package menu

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

func TestUpdateItemFullChangesEveryField(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := database.Write.ExecContext(ctx, `INSERT INTO websites (name) VALUES ('W')`); err != nil {
		t.Skipf("cannot seed a website: %v", err)
	}
	s := NewStore(database)
	m, err := s.CreateMenu(ctx, 1, "M", "main", "")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := s.CreateItem(ctx, m.ID, nil, "A", "custom", "", nil, 0)
	b, _ := s.CreateItem(ctx, m.ID, nil, "B", "custom", "", nil, 1)
	if err := s.UpdateItemFull(ctx, b.ID, &a.ID, "B2", "url", "https://x.org/", nil, 4); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetItem(ctx, b.ID)
	if got.Title != "B2" || got.ItemType != "url" || got.URL != "https://x.org/" || got.SortOrder != 4 ||
		got.ParentID == nil || *got.ParentID != a.ID {
		t.Errorf("stored %+v", got)
	}
}
