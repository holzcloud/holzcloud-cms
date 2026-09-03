package page

import (
	"context"
	"errors"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
)

// cheapParams keep the tests fast; the real ones are tuned for the server.
var cheapParams = auth.Argon2Params{
	Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
}

func TestProtectingAPageStoresOnlyAHash(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Preisliste", "preisliste", "Nur für Händler", "published")

	if err := s.SetAccess(ctx, p.ID, AccessUpdate{
		Protected: true, Password: "holz2026", Hint: "Steht im Anschreiben.",
	}, cheapParams); err != nil {
		t.Fatalf("SetAccess: %v", err)
	}

	stored, _ := s.GetPage(ctx, p.ID)
	if !stored.Protected() {
		t.Fatal("the page is not protected")
	}
	// One careless copy of the database file must not expose every protected
	// page at once.
	if stored.AccessPassword == "holz2026" {
		t.Fatal("the password is stored in the clear")
	}
	if !CheckPagePassword(stored, "holz2026") {
		t.Error("the right password does not open the page")
	}
	if CheckPagePassword(stored, "holz2025") {
		t.Error("a wrong password opens the page")
	}
	if stored.AccessHint != "Steht im Anschreiben." {
		t.Errorf("hint = %q", stored.AccessHint)
	}
}

func TestAnEmptyPasswordKeepsTheExistingOne(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Preisliste", "preisliste", "x", "published")

	if err := s.SetAccess(ctx, p.ID, AccessUpdate{Protected: true, Password: "holz2026"},
		cheapParams); err != nil {
		t.Fatalf("SetAccess: %v", err)
	}
	// Changing only the hint must not require retyping the password — and a
	// form that echoed the old one back would put it in the page source.
	if err := s.SetAccess(ctx, p.ID, AccessUpdate{Protected: true, Hint: "Neuer Hinweis"},
		cheapParams); err != nil {
		t.Fatalf("second SetAccess: %v", err)
	}

	stored, _ := s.GetPage(ctx, p.ID)
	if !CheckPagePassword(stored, "holz2026") {
		t.Error("the password was lost when only the hint changed")
	}
	if stored.AccessHint != "Neuer Hinweis" {
		t.Errorf("hint = %q", stored.AccessHint)
	}
}

func TestProtectingWithoutEverSettingAPasswordIsRefused(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Preisliste", "preisliste", "x", "published")

	// Otherwise the page would claim to be protected while letting everyone in.
	err := s.SetAccess(ctx, p.ID, AccessUpdate{Protected: true}, cheapParams)
	if !errors.Is(err, ErrNoPagePassword) {
		t.Fatalf("err = %v, want ErrNoPagePassword", err)
	}
	stored, _ := s.GetPage(ctx, p.ID)
	if stored.Protected() {
		t.Error("the page reports itself protected with no password")
	}
}

func TestAShortPasswordIsRefused(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Preisliste", "preisliste", "x", "published")

	err := s.SetAccess(ctx, p.ID, AccessUpdate{Protected: true, Password: "holz"}, cheapParams)
	if !errors.Is(err, ErrPagePasswordTooShort) {
		t.Fatalf("err = %v, want ErrPagePasswordTooShort", err)
	}
}

func TestUnprotectingClearsTheSecret(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Preisliste", "preisliste", "x", "published")

	if err := s.SetAccess(ctx, p.ID, AccessUpdate{Protected: true, Password: "holz2026"},
		cheapParams); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAccess(ctx, p.ID, AccessUpdate{Protected: false}, cheapParams); err != nil {
		t.Fatalf("SetAccess off: %v", err)
	}

	stored, _ := s.GetPage(ctx, p.ID)
	if stored.Protected() || stored.AccessPassword != "" {
		t.Error("the hash survived switching protection off")
	}
	// A password that guards nothing is a secret with no purpose and a copy of
	// it in every backup from now on.
	if CheckPagePassword(stored, "holz2026") {
		t.Error("the old password still opens the page")
	}
}

func TestAProtectedPageIsServedButNeverListed(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()

	open := seed(t, s, ws, "Über uns", "ueber-uns", "Möbel aus Eiche", "published")
	secret := seed(t, s, ws, "Händlerpreise", "haendlerpreise", "Möbel zum Einkaufspreis", "published")
	if err := s.SetAccess(ctx, secret.ID, AccessUpdate{Protected: true, Password: "holz2026"},
		cheapParams); err != nil {
		t.Fatal(err)
	}

	// Still reachable by address: that is what the gate is in front of.
	got, err := s.GetPublishedPage(ctx, ws, "haendlerpreise")
	if err != nil || got == nil {
		t.Fatalf("the protected page is not reachable at all: %v %v", got, err)
	}

	// But its title and excerpt are exactly what the password holds back, so
	// they must not appear in a sitemap, a listing or a search result.
	entries, err := s.ListPublishedForSitemap(ctx, ws)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Slug == "haendlerpreise" {
			t.Error("a protected page is in the sitemap")
		}
	}

	recent, err := s.ListRecentPublished(ctx, ws, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range recent {
		if p.Slug == "haendlerpreise" {
			t.Error("a protected page is in the feed listing")
		}
	}

	hits, err := s.SearchPages(ctx, ws, "Möbel", false, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if h.Page.Slug == "haendlerpreise" {
			t.Error("a protected page turns up in the search")
		}
	}
	if len(hits) == 0 {
		t.Errorf("the search found nothing at all, so it proves nothing (%q exists)", open.Slug)
	}
}
