package wording

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

func newStore(t *testing.T) (*Store, int64) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	ws, err := domain.NewStore(database).CreateWebsite(context.Background(), "Hof", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	return NewStore(database), ws.ID
}

// A word is written, read back, and reworded.
func TestAWordIsWrittenReadAndReworded(t *testing.T) {
	s, id := newStore(t)
	ctx := context.Background()

	if err := s.Set(ctx, id, "de", "Cart", "Korb"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	words, err := s.ThemeWords(ctx, id, "de")
	if err != nil {
		t.Fatalf("ThemeWords: %v", err)
	}
	if words["Cart"] != "Korb" {
		t.Fatalf("= %q, want \"Korb\"", words["Cart"])
	}

	if err := s.Set(ctx, id, "de", "Cart", "Merkliste"); err != nil {
		t.Fatalf("Set again: %v", err)
	}
	words, _ = s.ThemeWords(ctx, id, "de")
	if words["Cart"] != "Merkliste" {
		t.Errorf("rewording left %q", words["Cart"])
	}
	if n, _ := s.Count(ctx, id); n != 1 {
		t.Errorf("rewording made %d rows, want 1", n)
	}
}

// Emptying a box gives the theme's word back.
//
// The alternative — storing "" — would put a blank label on the page and leave
// the operator no way back except deleting a row they cannot see.
func TestAnEmptiedBoxRemovesTheWordRatherThanStoringNothing(t *testing.T) {
	s, id := newStore(t)
	ctx := context.Background()

	if err := s.Set(ctx, id, "de", "Cart", "Korb"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, id, "de", "Cart", "   "); err != nil {
		t.Fatalf("emptying: %v", err)
	}
	words, _ := s.ThemeWords(ctx, id, "de")
	if _, ok := words["Cart"]; ok {
		t.Errorf("an emptied box left %q behind", words["Cart"])
	}
	if n, _ := s.Count(ctx, id); n != 0 {
		t.Errorf("%d rows left, want 0", n)
	}
}

// Languages do not bleed into one another, and de-CH is not de.
//
// The theme's own catalogue falls back from de-CH to de, because a theme author
// writes one German file. An operator who overrides a word under "de" and
// publishes under "de-CH" has said which language they meant, and applying it
// to the other would put a word on a page they did not choose it for.
func TestALanguageIsExactlyTheLanguageItSays(t *testing.T) {
	s, id := newStore(t)
	ctx := context.Background()

	for _, c := range []struct{ locale, value string }{
		{"de", "Korb"}, {"fr", "Panier"},
	} {
		if err := s.Set(ctx, id, c.locale, "Cart", c.value); err != nil {
			t.Fatal(err)
		}
	}

	for _, c := range []struct{ locale, want string }{
		{"de", "Korb"}, {"fr", "Panier"}, {"de-CH", ""}, {"it", ""}, {"", ""},
	} {
		words, err := s.ThemeWords(ctx, id, c.locale)
		if err != nil {
			t.Fatalf("ThemeWords(%q): %v", c.locale, err)
		}
		if words["Cart"] != c.want {
			t.Errorf("locale %q = %q, want %q", c.locale, words["Cart"], c.want)
		}
	}
}

// The bound stops new words and never stops a correction.
//
// An operator who reaches the limit must still be able to fix their own typo;
// a limit that locks the screen is worse than no limit.
func TestTheBoundStopsNewWordsAndNeverACorrection(t *testing.T) {
	s, id := newStore(t)
	ctx := context.Background()

	for i := 0; i < MaxKeys; i++ {
		if err := s.Set(ctx, id, "de", fmt.Sprintf("Word %d", i), "x"); err != nil {
			t.Fatalf("filling up at %d: %v", i, err)
		}
	}
	if err := s.Set(ctx, id, "de", "One too many", "x"); err != ErrTooMany {
		t.Errorf("the %dth word was accepted: %v", MaxKeys+1, err)
	}
	if err := s.Set(ctx, id, "de", "Word 0", "corrected"); err != nil {
		t.Errorf("a correction at the limit was refused: %v", err)
	}
	words, _ := s.ThemeWords(ctx, id, "de")
	if words["Word 0"] != "corrected" {
		t.Errorf("the correction did not land: %q", words["Word 0"])
	}
	// And removing one makes room again.
	if err := s.Set(ctx, id, "de", "Word 0", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, id, "de", "One too many", "x"); err != nil {
		t.Errorf("after removing one there is still no room: %v", err)
	}
}

// A very long word is cut rather than refused.
func TestAnOverlongWordIsCutRatherThanRefused(t *testing.T) {
	s, id := newStore(t)
	ctx := context.Background()
	long := ""
	for i := 0; i < MaxValue+50; i++ {
		long += "ü"
	}
	if err := s.Set(ctx, id, "de", "Cart", long); err != nil {
		t.Fatalf("Set: %v", err)
	}
	words, _ := s.ThemeWords(ctx, id, "de")
	// Runes, not bytes: cutting an umlaut in half would store invalid UTF-8.
	if n := len([]rune(words["Cart"])); n != MaxValue {
		t.Errorf("stored %d runes, want %d", n, MaxValue)
	}
}

// All lists every language at once, for the screen.
func TestAllListsEveryLanguageOrdered(t *testing.T) {
	s, id := newStore(t)
	ctx := context.Background()
	for _, c := range []struct{ locale, key, value string }{
		{"fr", "Search", "Recherche"},
		{"de", "Cart", "Korb"},
		{"de", "Search", "Suchen"},
	} {
		if err := s.Set(ctx, id, c.locale, c.key, c.value); err != nil {
			t.Fatal(err)
		}
	}
	all, err := s.All(ctx, id)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	want := []Entry{
		{"de", "Cart", "Korb"}, {"de", "Search", "Suchen"}, {"fr", "Search", "Recherche"},
	}
	if len(all) != len(want) {
		t.Fatalf("%d entries, want %d: %v", len(all), len(want), all)
	}
	for i := range want {
		if all[i] != want[i] {
			t.Errorf("entry %d = %v, want %v", i, all[i], want[i])
		}
	}
}

// A deleted website takes its wording with it.
func TestADeletedWebsiteTakesItsWordsWithIt(t *testing.T) {
	s, id := newStore(t)
	ctx := context.Background()
	if err := s.Set(ctx, id, "de", "Cart", "Korb"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Write.ExecContext(ctx, `DELETE FROM websites WHERE id = ?`, id); err != nil {
		t.Fatalf("deleting the website: %v", err)
	}
	if n, _ := s.Count(ctx, id); n != 0 {
		t.Errorf("%d rows outlived their website", n)
	}
}
