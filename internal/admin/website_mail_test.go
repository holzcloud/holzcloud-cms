package admin

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/mail"
)

// The password typed for one server must not follow the form to another.
func TestWebsiteMailKeepsNoPasswordForAnotherServer(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	accounts := mail.NewAccounts(database, "ein-schluessel-der-lang-genug-ist-fuer-den-test", mail.NewSender(mail.Config{}))
	q := mail.NewQueue(database, mail.NewSender(mail.Config{}), nil)
	q.SetAccounts(accounts)
	h.SetMail(q)

	if err := accounts.Save(ctx, mail.Account{WebsiteID: ws.ID, Host: "mail.example.com", Port: 587,
		User: "info", From: "info@example.com"}, "geheim", false); err != nil {
		t.Fatalf("Save: %v", err)
	}

	form := func(host string) url.Values {
		return url.Values{"host": {host}, "port": {"587"}, "username": {"info"},
			"from": {"info@example.com"}, "tls": {"starttls"}}
	}
	route := websiteRoute(ws.ID)

	rec := serve(t, h, sm, h.HandleWebsiteMail,
		postForm("/admin/websites/1/versand", form("evil.example.net"), route))
	if rec.Code == http.StatusSeeOther {
		t.Fatalf("a changed server without a password was saved (303)")
	}
	got, err := accounts.Get(ctx, ws.ID)
	if err != nil || got.Host != "mail.example.com" {
		t.Fatalf("stored account = %+v, %v; want it untouched", got, err)
	}

	// Unchanged triple, empty password: still keeps it.
	rec = serve(t, h, sm, h.HandleWebsiteMail,
		postForm("/admin/websites/1/versand", form("mail.example.com"), route))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unchanged server: status %d, want 303", rec.Code)
	}
	if got, _ := accounts.Get(ctx, ws.ID); !got.HasPassword {
		t.Error("the stored password was dropped by an unchanged save")
	}
}
