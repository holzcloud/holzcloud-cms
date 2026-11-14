package bundle

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/dsr"
)

// A website moved by bundle keeps its retention periods and the record of
// how its privacy requests were made and answered (Phase 33.2, plan 07).
// Every helper of this file starts with retention.

// retentionStores are stores with the privacy requests and the retention
// periods, as the admin hands them over.
func retentionStores(t *testing.T) Stores {
	t.Helper()
	s := newStores(t)
	s.Requests = dsr.NewStore(s.Domains.DB)
	s.Retention = dsr.NewService(s.Domains.DB)
	return s
}

// retentionPeriods are the periods in force of a website, by kind, and
// which of them the website chose itself.
func retentionPeriods(t *testing.T, s Stores, websiteID int64) (map[string]int, map[string]bool) {
	t.Helper()
	ws, err := s.Domains.GetWebsite(context.Background(), websiteID)
	if err != nil || ws == nil {
		t.Fatalf("website %d: %v", websiteID, err)
	}
	list, err := s.Retention.Periods(context.Background(), websiteID, ws.Country)
	if err != nil {
		t.Fatal(err)
	}
	months, stored := map[string]int{}, map[string]bool{}
	for _, p := range list {
		months[p.Kind], stored[p.Kind] = p.Months, p.Stored
	}
	return months, stored
}

func TestRetentionPeriodsTravelWithTheWebsite(t *testing.T) {
	ctx := context.Background()
	from := retentionStores(t)
	ws := seedSite(t, from) // Germany: orders at least 8 years
	if err := from.Retention.SetPeriods(ctx, ws, "DE", map[string]int{
		dsr.RetentionFormSubmission: 6, dsr.RetentionOrder: 132,
	}); err != nil {
		t.Fatal(err)
	}
	archive := exportTo(t, from, ws)
	m := manifestOf(t, archive)
	if !strings.Contains(m, `"retention_periods"`) {
		t.Fatalf("the manifest carries no retention periods:\n%s", m)
	}
	if strings.Contains(m, `"request"`) {
		t.Error("the manifest carries the request period, which the website never chose: a default would stop following the release")
	}

	to := retentionStores(t)
	report, err := Import(ctx, to, bytes.NewReader(archive), int64(len(archive)), "Umgezogen")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	months, stored := retentionPeriods(t, to, report.WebsiteID)
	if months[dsr.RetentionFormSubmission] != 6 || months[dsr.RetentionOrder] != 132 {
		t.Errorf("arrived as %v; want form_submission 6 and order 132", months)
	}
	if stored[dsr.RetentionRequest] {
		t.Error("the request period arrived as a choice of the website")
	}
	if report.Periods != 2 {
		t.Errorf("the report counts %d periods, want 2", report.Periods)
	}
	for _, w := range report.Warnings {
		if strings.Contains(w, "retention") {
			t.Errorf("a clean move warns: %q", w)
		}
	}

	// An older bundle: the defaults, and nothing to say about it.
	old := legalBundleRawWith(t, `{"name": "Alt", "locale": "de", "country": "DE"}`, "")
	report, err = Import(ctx, to, bytes.NewReader(old), int64(len(old)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	months, stored = retentionPeriods(t, to, report.WebsiteID)
	if months[dsr.RetentionFormSubmission] != 12 || stored[dsr.RetentionFormSubmission] || stored[dsr.RetentionOrder] {
		t.Errorf("a bundle without periods arrived with %v (stored %v); want the defaults", months, stored)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("a bundle without periods warns: %q", report.Warnings)
	}

	// Edited by hand: below the floor is raised, an unknown kind skipped,
	// and both are said.
	edited := legalBundleRawWith(t, `{"name": "Von Hand", "locale": "de", "country": "DE",
		"retention_periods": {"request": 12, "logs": 1, "form_submission": 3}}`, "")
	report, err = Import(ctx, to, bytes.NewReader(edited), int64(len(edited)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	months, _ = retentionPeriods(t, to, report.WebsiteID)
	if months[dsr.RetentionRequest] != 24 || months[dsr.RetentionFormSubmission] != 3 {
		t.Errorf("arrived as %v; want request raised to 24 and form_submission 3", months)
	}
	joined := strings.Join(report.Warnings, "\n")
	if !strings.Contains(joined, "retention period for request: 12 months is below the minimum and was raised to 24.") {
		t.Errorf("the raised period is not reported: %q", report.Warnings)
	}
	if !strings.Contains(joined, `retention period for "logs" is not known here and was skipped.`) {
		t.Errorf("the unknown kind is not reported: %q", report.Warnings)
	}
	if len(report.Warnings) != 2 {
		t.Errorf("warnings %q; want exactly the two", report.Warnings)
	}
}

func TestRequestAnswersTravelWithTheWebsite(t *testing.T) {
	ctx := context.Background()
	from := retentionStores(t)
	ws := seedSite(t, from)
	created := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	handled := time.Date(2026, 3, 18, 15, 30, 0, 0, time.UTC)
	if _, err := from.Requests.Restore(ctx, dsr.Request{WebsiteID: ws, Kind: dsr.KindAccess, Name: "Ada",
		Email: "ada@example.com", CreatedAt: created, HandledAt: &handled,
		Manner: dsr.MannerEmail, Outcome: dsr.OutcomeGranted, Basis: "sent the export"}); err != nil {
		t.Fatal(err)
	}
	archive := exportTo(t, from, ws)

	to := retentionStores(t)
	report, err := Import(ctx, to, bytes.NewReader(archive), int64(len(archive)), "Umgezogen")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	got, err := to.Requests.ListAll(ctx, report.WebsiteID)
	if err != nil || len(got) != 1 {
		t.Fatalf("%d requests arrived (%v); want 1, warnings %q", len(got), err, report.Warnings)
	}
	r := got[0]
	if r.Kind != dsr.KindAccess || r.Manner != dsr.MannerEmail || r.Outcome != dsr.OutcomeGranted || r.Basis != "sent the export" {
		t.Errorf("arrived as kind %q, manner %q, outcome %q, basis %q", r.Kind, r.Manner, r.Outcome, r.Basis)
	}
	if !r.CreatedAt.Equal(created) || r.HandledAt == nil || !r.HandledAt.Equal(handled) {
		t.Errorf("arrived with dates %v, %v", r.CreatedAt, r.HandledAt)
	}

	// An older bundle: a request from the form, not answered in the sense
	// of an outcome.
	old := legalBundleRawWith(t, `{"name": "Alt", "locale": "de"}`,
		`"privacy_requests": [{"kind": "optout", "name": "Bea", "created_at": "2026-03-01T10:00:00Z", "handled_at": "2026-03-02T10:00:00Z"}]`)
	report, err = Import(ctx, to, bytes.NewReader(old), int64(len(old)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	got, err = to.Requests.ListAll(ctx, report.WebsiteID)
	if err != nil || len(got) != 1 {
		t.Fatalf("%d requests from an older bundle (%v), warnings %q", len(got), err, report.Warnings)
	}
	if got[0].Manner != dsr.MannerForm || got[0].Outcome != "" {
		t.Errorf("an older bundle's request arrived with manner %q, outcome %q; want form and none", got[0].Manner, got[0].Outcome)
	}
}
