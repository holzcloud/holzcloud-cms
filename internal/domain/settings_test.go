package domain

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return NewStore(database)
}

// A fresh website has to come out of the database usable, not with an empty
// language and no time zone.
func TestNewWebsiteHasGermanDefaults(t *testing.T) {
	s := newTestStore(t)
	ws, err := s.CreateWebsite(context.Background(), "Test", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	if ws.Locale != "de" {
		t.Errorf("locale = %q, want de", ws.Locale)
	}
	if ws.TimeZone != "Europe/Berlin" {
		t.Errorf("timezone = %q, want Europe/Berlin", ws.TimeZone)
	}
	// A fresh install often runs on a bare IP, where redirecting to a primary
	// domain that does not exist yet would take the site down.
	if ws.CanonicalRedirect {
		t.Error("canonical redirect is on by default")
	}
	if ws.OfflineMode != "notfound" {
		t.Errorf("offline mode = %q, want notfound", ws.OfflineMode)
	}
}

func TestUpdateSettingsRoundTrips(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ws, _ := s.CreateWebsite(ctx, "Test", "")

	err := s.UpdateSettings(ctx, ws.ID, Settings{
		Locale:            "en",
		TimeZone:          "Europe/London",
		MetaDescription:   "Furniture from the Black Forest",
		CanonicalRedirect: true,
		OfflineMode:       "maintenance",
		OfflineMessage:    "Back on Monday",
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	after, err := s.GetWebsite(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	if after.Locale != "en" || after.TimeZone != "Europe/London" ||
		!after.CanonicalRedirect || after.OfflineMode != "maintenance" ||
		after.OfflineMessage != "Back on Monday" {
		t.Errorf("settings did not round-trip: %+v", after)
	}
}

// The CHECK constraint would turn a bad mode into an opaque SQL error on the
// operator's screen.
func TestUpdateSettingsRejectsAnUnknownOfflineMode(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ws, _ := s.CreateWebsite(ctx, "Test", "")

	if err := s.UpdateSettings(ctx, ws.ID, Settings{OfflineMode: "explode"}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	after, _ := s.GetWebsite(ctx, ws.ID)
	if after.OfflineMode != "notfound" {
		t.Errorf("offline mode = %q, want the safe default", after.OfflineMode)
	}
}

// is_primary had a CHECK but nothing stopped five primaries per website, and
// the canonical URL then depended on which row a query happened to return.
func TestOnlyOneDomainCanBePrimary(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ws, _ := s.CreateWebsite(ctx, "Test", "")

	if _, err := s.AddDomain(ctx, ws.ID, "erste.test", true); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if _, err := s.AddDomain(ctx, ws.ID, "zweite.test", true); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}

	domains, err := s.ListDomains(ctx, ws.ID)
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	primaries := 0
	for _, d := range domains {
		if d.IsPrimary {
			primaries++
		}
	}
	if primaries != 1 {
		t.Fatalf("%d primary domains, want exactly 1", primaries)
	}

	host, err := s.PrimaryDomain(ctx, ws.ID)
	if err != nil {
		t.Fatalf("PrimaryDomain: %v", err)
	}
	if host != "zweite.test" {
		t.Errorf("primary = %q, want the most recently marked one", host)
	}
}

func TestSetPrimaryDomainMovesTheFlag(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ws, _ := s.CreateWebsite(ctx, "Test", "")
	first, _ := s.AddDomain(ctx, ws.ID, "erste.test", true)
	second, _ := s.AddDomain(ctx, ws.ID, "zweite.test", false)

	if err := s.SetPrimaryDomain(ctx, ws.ID, second.ID); err != nil {
		t.Fatalf("SetPrimaryDomain: %v", err)
	}
	host, _ := s.PrimaryDomain(ctx, ws.ID)
	if host != "zweite.test" {
		t.Errorf("primary = %q, want zweite.test", host)
	}

	// A domain of another website must not be reachable through this call.
	other, _ := s.CreateWebsite(ctx, "Andere", "")
	if err := s.SetPrimaryDomain(ctx, other.ID, first.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("cross-website set returned %v, want sql.ErrNoRows", err)
	}
	host, _ = s.PrimaryDomain(ctx, ws.ID)
	if host != "zweite.test" {
		t.Errorf("primary changed to %q through another website", host)
	}
}
