package bundle

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/dsr"
)

// The privacy requests travel with the website (WR-06 of the 33.1 review).
// Moving a website to another installation by export and import used to
// leave its opt-outs behind, and they are the record CCPA regs § 7101 ask to
// be kept for 24 months — with the dates they arrived and were answered,
// which are the record and not decoration.
func TestPrivacyRequestsTravelWithTheWebsite(t *testing.T) {
	s := newStores(t)
	s.Requests = dsr.NewStore(s.Domains.DB)
	ctx := context.Background()
	ws := seedSite(t, s)
	other := seedSite(t, s)

	id, err := s.Requests.Add(ctx, dsr.Request{WebsiteID: ws, Kind: dsr.KindOptOut, Locale: "fr",
		Name: "Ada", Email: "ada@example.com", Note: "merci", GPC: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Domains.DB.Write.Exec(
		`UPDATE privacy_requests SET created_at = '2026-03-01T10:00:00Z', handled_at = '2026-03-04T09:30:00Z' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Requests.Add(ctx, dsr.Request{WebsiteID: ws, Kind: dsr.KindOptOut, GPC: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Requests.Add(ctx, dsr.Request{WebsiteID: other, Kind: dsr.KindOptOut, Name: "Fremd"}); err != nil {
		t.Fatal(err)
	}

	archive := exportTo(t, s, ws)
	if m := manifestOf(t, archive); strings.Contains(m, "Fremd") {
		t.Error("the archive carries another website's request")
	}

	report, err := Import(ctx, s, bytes.NewReader(archive), int64(len(archive)), "Umgezogen")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Requests != 2 {
		t.Errorf("report counts %d requests; want 2 (warnings %v)", report.Requests, report.Warnings)
	}
	got, err := s.Requests.List(ctx, report.WebsiteID, dsr.KindOptOut)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("%d requests arrived; want 2", len(got))
	}
	var ada *dsr.Request
	for i := range got {
		if got[i].Name == "Ada" {
			ada = &got[i]
		}
	}
	if ada == nil {
		t.Fatalf("Ada's request did not arrive: %+v", got)
	}
	if ada.Email != "ada@example.com" || ada.Note != "merci" || ada.Locale != "fr" || !ada.GPC {
		t.Errorf("arrived as %+v", ada)
	}
	if want := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC); !ada.CreatedAt.Equal(want) {
		t.Errorf("created_at %v; want %v — the arrival date is the record", ada.CreatedAt, want)
	}
	if ada.HandledAt == nil || !ada.HandledAt.Equal(time.Date(2026, 3, 4, 9, 30, 0, 0, time.UTC)) {
		t.Errorf("handled_at %v; want 2026-03-04 09:30 UTC", ada.HandledAt)
	}
}

// A manifest is a file anybody can edit: a request the table would refuse is
// named in the report, and the import goes on.
func TestABrokenPrivacyRequestIsReported(t *testing.T) {
	s := newStores(t)
	s.Requests = dsr.NewStore(s.Domains.DB)
	archive := legalBundleRawWith(t, `{"name": "Von Hand", "locale": "de"}`,
		`"privacy_requests": [
		  {"kind": "optout", "name": "Ada", "created_at": "2026-03-01T10:00:00Z"},
		  {"kind": "optout", "created_at": "2026-03-01T10:00:00Z"},
		  {"kind": "unbekannt", "name": "Bea", "created_at": "2026-03-01T10:00:00Z"},
		  {"kind": "optout", "name": "Cleo", "created_at": "gestern"}
		]`)
	report, err := Import(context.Background(), s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Requests != 1 || len(report.Warnings) != 3 {
		t.Errorf("%d requests, warnings %q; want 1 and three warnings", report.Requests, report.Warnings)
	}
}
