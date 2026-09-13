package public

import (
	"context"
	"strings"
	"testing"

	"log/slog"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// The copy goes to the sender, and to nobody else.
//
// Four things have to be true and this drives each of them separately, because
// the interesting failures are the ones where three hold and the fourth is
// assumed: a plugin that asks and a website that never switched receipts on, a
// website that did and an address that is not one, and the case where the
// sender's address IS the operator's — which would otherwise send them a
// receipt for their own enquiry.
func TestTheCopyGoesToTheSenderAndNobodyElse(t *testing.T) {
	for _, c := range []struct {
		name           string
		confirmOn      bool
		asked          bool
		replyTo        string
		wantRecipients []string
	}{
		{
			name: "switched on and asked", confirmOn: true, asked: true,
			replyTo:        "anna@example.test",
			wantRecipients: []string{"betrieb@example.test", "anna@example.test"},
		},
		{
			name: "asked, but the operator never switched it on", asked: true,
			replyTo:        "anna@example.test",
			wantRecipients: []string{"betrieb@example.test"},
		},
		{
			name: "switched on, but the plugin did not ask", confirmOn: true,
			replyTo:        "anna@example.test",
			wantRecipients: []string{"betrieb@example.test"},
		},
		{
			name: "switched on, and no address to send to", confirmOn: true, asked: true,
			replyTo:        "not-an-address",
			wantRecipients: []string{"betrieb@example.test"},
		},
		{
			// Otherwise the operator gets two copies, the second addressed as a
			// receipt for their own enquiry.
			name: "the sender is the operator", confirmOn: true, asked: true,
			replyTo:        "BETRIEB@example.test",
			wantRecipients: []string{"betrieb@example.test"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			h, database := newTestHandler(t)
			ws := seedWebsite(t, database, "Hofladen")
			// A mail server that can do nothing but exist: what is checked is
			// what gets queued, not what gets delivered.
			store := domain.NewStore(database)
			h.SetNotify(store, mail.NewQueue(database, mail.NewSender(mail.Config{
				Host: "mail.example.test", From: "cms@example.test",
			}), slog.New(slog.DiscardHandler)))
			if err := store.UpdateSettings(context.Background(), ws.ID, domain.Settings{
				Locale: "de", TimeZone: "Europe/Berlin",
				NotifyEmail: "betrieb@example.test", ConfirmSenders: c.confirmOn,
			}); err != nil {
				t.Fatalf("UpdateSettings: %v", err)
			}

			queued, confirmed, reason, err := h.NotifyForPlugin(context.Background(), ws.ID,
				plugin.NotifyArg{
					Subject: "Neue Anfrage", Body: "Guten Tag", ReplyTo: c.replyTo,
					Confirm: c.asked, ConfirmSubject: "Ihre Anfrage", ConfirmBody: "Danke",
				})
			if err != nil {
				t.Fatalf("NotifyForPlugin: %v", err)
			}
			if !queued {
				t.Fatalf("the operator's notification was not queued: %q", reason)
			}
			wantConfirmed := len(c.wantRecipients) == 2
			if confirmed != wantConfirmed {
				t.Errorf("confirmed = %v, want %v", confirmed, wantConfirmed)
			}

			got := queuedRecipients(t, database, ws.ID)
			if len(got) != len(c.wantRecipients) {
				t.Fatalf("%d messages queued (%v), want %d (%v)",
					len(got), got, len(c.wantRecipients), c.wantRecipients)
			}
			for i, want := range c.wantRecipients {
				if !strings.EqualFold(got[i], want) {
					t.Errorf("message %d went to %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// queuedRecipients reads the addresses the outbox holds, oldest first.
func queuedRecipients(t *testing.T, database *db.DB, websiteID int64) []string {
	t.Helper()
	rows, err := database.Read.Query(
		`SELECT recipient FROM mail_outbox WHERE website_id = ? ORDER BY id`, websiteID)
	if err != nil {
		t.Fatalf("reading the outbox: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var to string
		if err := rows.Scan(&to); err != nil {
			t.Fatal(err)
		}
		out = append(out, to)
	}
	return out
}
