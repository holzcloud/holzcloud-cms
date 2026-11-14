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

// A manifest edited by hand, or two put together, can list one request
// twice, and the § 7101 record then held it twice (IN-04 of the second 33.1
// review). The same request — same kind, moment, language, name, address,
// note and signal — is restored once and the repetition is named.
func TestADuplicatePrivacyRequestIsRestoredOnce(t *testing.T) {
	s := newStores(t)
	s.Requests = dsr.NewStore(s.Domains.DB)
	archive := legalBundleRawWith(t, `{"name": "Von Hand", "locale": "de"}`,
		`"privacy_requests": [
		  {"kind": "optout", "name": "Ada", "email": "ada@example.com", "note": "bitte", "created_at": "2026-03-01T10:00:00Z"},
		  {"kind": "optout", "name": "Bea", "created_at": "2026-03-01T10:00:00Z"},
		  {"kind": "optout", "name": "Ada", "email": "ada@example.com", "note": "bitte", "created_at": "2026-03-01T11:00:00+01:00", "handled_at": "2026-03-02T10:00:00Z"},
		  {"kind": "optout", "name": "Ada", "email": "ada@example.com", "note": "bitte", "created_at": "2026-03-01T10:00:01Z"}
		]`)
	report, err := Import(context.Background(), s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Requests != 3 || len(report.Warnings) != 1 {
		t.Fatalf("%d requests, warnings %q; want 3 and one warning", report.Requests, report.Warnings)
	}
	if w := report.Warnings[0]; !strings.Contains(w, "privacy request 3") || !strings.Contains(w, "request 1") {
		t.Errorf("warning %q; want it to name request 3 as a repetition of request 1", w)
	}
	got, err := s.Requests.List(context.Background(), report.WebsiteID, dsr.KindOptOut)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("restored %d requests; want 3: %+v", len(got), got)
	}
}

// A request dated in the future counted in every limit until that date: the
// form's hourly ceiling, the mail cap and the 24 months that keep a website
// from being deleted (WR-02 of the second 33.1 review). Such a request, and
// one answered before it arrived, is named in the report and not restored.
func TestAPrivacyRequestWithImpossibleDatesIsReported(t *testing.T) {
	s := newStores(t)
	s.Requests = dsr.NewStore(s.Domains.DB)
	archive := legalBundleRawWith(t, `{"name": "Von Hand", "locale": "de"}`,
		`"privacy_requests": [
		  {"kind": "optout", "name": "Ada", "created_at": "2026-03-01T10:00:00Z", "handled_at": "2026-03-02T10:00:00Z"},
		  {"kind": "optout", "name": "Zukunft", "created_at": "2099-01-01T00:00:00Z"},
		  {"kind": "optout", "name": "Rückwärts", "created_at": "2026-03-01T10:00:00Z", "handled_at": "2026-02-01T10:00:00Z"}
		]`)
	report, err := Import(context.Background(), s, bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Requests != 1 || len(report.Warnings) != 2 {
		t.Fatalf("%d requests, warnings %q; want 1 and two warnings", report.Requests, report.Warnings)
	}
	for i, want := range []string{"privacy request 2", "privacy request 3"} {
		if !strings.Contains(report.Warnings[i], want) || !strings.Contains(report.Warnings[i], "date") {
			t.Errorf("warning %d = %q; want it to name %s and its dates", i+1, report.Warnings[i], want)
		}
	}
	got, err := s.Requests.List(context.Background(), report.WebsiteID, dsr.KindOptOut)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Ada" {
		t.Errorf("restored %+v; want only Ada's request", got)
	}
}
