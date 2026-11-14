package domain

import (
	"context"
	"errors"
	"testing"
)

// A website that received a privacy request in the last 24 months cannot be
// deleted (WR-06 of the 33.1 review). Its requests went with it through ON
// DELETE CASCADE, and CCPA regs § 7101 ask for the record of a request to be
// kept for 24 months — the package that stores them deletes nothing for
// exactly that reason (D-14), and the website's deletion was the way around
// it. A request older than that is past the floor and goes with the website.
func TestAWebsiteWithRecentPrivacyRequestsIsKept(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	ws, err := s.CreateWebsite(ctx, "Hofladen", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Write.Exec(
		`INSERT INTO privacy_requests (website_id, kind, name) VALUES ($1, 'optout', 'Ada')`, ws.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWebsite(ctx, ws.ID); !errors.Is(err, ErrWebsiteKeepsRequests) {
		t.Fatalf("DeleteWebsite = %v; want ErrWebsiteKeepsRequests", err)
	}
	if got, err := s.GetWebsite(ctx, ws.ID); err != nil || got == nil {
		t.Fatalf("the website is gone after a refused delete (err %v)", err)
	}

	// Answered long ago, received longer ago: the answer counts too.
	if _, err := s.DB.Write.Exec(
		`UPDATE privacy_requests SET created_at = '2020-01-01T00:00:00Z',
		 handled_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now', '-1 month') WHERE website_id = $1`, ws.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWebsite(ctx, ws.ID); !errors.Is(err, ErrWebsiteKeepsRequests) {
		t.Fatalf("a request answered a month ago: DeleteWebsite = %v; want ErrWebsiteKeepsRequests", err)
	}

	if _, err := s.DB.Write.Exec(
		`UPDATE privacy_requests SET handled_at = '2020-02-01T00:00:00Z' WHERE website_id = $1`, ws.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWebsite(ctx, ws.ID); err != nil {
		t.Fatalf("a request past 24 months: DeleteWebsite = %v; want deleted", err)
	}
	var left int
	if err := s.DB.Read.QueryRow(`SELECT COUNT(*) FROM privacy_requests WHERE website_id = $1`, ws.ID).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d requests outlived their website", left)
	}

	// A website with no requests at all is deleted as before.
	other, err := s.CreateWebsite(ctx, "Nachbar", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWebsite(ctx, other.ID); err != nil {
		t.Errorf("a website without requests: %v", err)
	}
}
