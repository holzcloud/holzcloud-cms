package mail

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

const testSecret = "ein-schluessel-der-lang-genug-ist-fuer-den-test"

func newWebsite(t *testing.T, a *Accounts, name string) int64 {
	t.Helper()
	res, err := a.db.Write.Exec(`INSERT INTO websites (name, description) VALUES ($1, '')`, name)
	if err != nil {
		t.Fatalf("insert website: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// The password goes in encrypted, comes out as typed, and is bound to its row
// and to the key.
func TestKontoPasswortVerschluesselt(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()
	a := NewAccounts(database, testSecret, NewSender(Config{}))
	site := newWebsite(t, a, "Velowerkstatt")

	acc := Account{WebsiteID: site, Host: "mail.velo.test", User: "info", From: "info@velo.test", FromName: "Velowerkstatt"}
	if err := a.Save(ctx, acc, "geheim-123", false); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var stored string
	if err := database.Read.QueryRow(`SELECT password FROM website_mail WHERE website_id = $1`, site).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == "" || strings.Contains(stored, "geheim") {
		t.Fatalf("password stored as %q, want ciphertext", stored)
	}

	s, err := a.SenderFor(ctx, site)
	if err != nil {
		t.Fatalf("SenderFor: %v", err)
	}
	if got := s.Config(); got.Password != "geheim-123" || got.From != "info@velo.test" || got.Port != 587 || got.TLS != "starttls" {
		t.Errorf("sender config = %+v", got)
	}

	// Saving with keepPassword leaves the stored password alone.
	acc.FromName = "Velo"
	acc.Host = "MAIL.velo.test" // case alone is not a change of server
	if err := a.Save(ctx, acc, "", true); err != nil {
		t.Fatalf("Save keep: %v", err)
	}
	s, _ = a.SenderFor(ctx, site)
	if s.Config().Password != "geheim-123" || s.Config().FromName != "Velo" {
		t.Errorf("after keep: %+v", s.Config())
	}

	// Another key does not open it.
	other := NewAccounts(database, testSecret+"-anders", NewSender(Config{}))
	if _, err := other.SenderFor(ctx, site); !errors.Is(err, ErrPasswordUnreadable) {
		t.Errorf("other key: err = %v, want ErrPasswordUnreadable", err)
	}
	// No key at all says so.
	none := NewAccounts(database, "", NewSender(Config{}))
	if _, err := none.SenderFor(ctx, site); !errors.Is(err, ErrNoSecretKey) {
		t.Errorf("no key: err = %v, want ErrNoSecretKey", err)
	}

	// A value copied into another website's row does not open there.
	site2 := newWebsite(t, a, "Strickstube")
	if err := a.Save(ctx, Account{WebsiteID: site2, Host: "mail.strick.test", From: "hallo@strick.test"}, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Write.Exec(`UPDATE website_mail SET password = $1 WHERE website_id = $2`, stored, site2); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SenderFor(ctx, site2); !errors.Is(err, ErrPasswordUnreadable) {
		t.Errorf("copied value: err = %v, want ErrPasswordUnreadable", err)
	}
}

// Without HOLZCLOUD_SECRET_KEY a password is refused, a relay without one is
// not.
func TestKontoOhneSchluessel(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()
	a := NewAccounts(database, "", NewSender(Config{}))
	site := newWebsite(t, a, "Velowerkstatt")

	acc := Account{WebsiteID: site, Host: "localhost", From: "info@velo.test", TLS: "none", Port: 25}
	if err := a.Save(ctx, acc, "geheim", false); !errors.Is(err, ErrNoSecretKey) {
		t.Fatalf("Save with password: err = %v, want ErrNoSecretKey", err)
	}
	if err := a.Save(ctx, acc, "", false); err != nil {
		t.Fatalf("Save relay: %v", err)
	}
	got, err := a.Get(ctx, site)
	if err != nil || got == nil || got.HasPassword {
		t.Fatalf("Get = %+v, %v", got, err)
	}
}

func TestKontoPrueftEingaben(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()
	a := NewAccounts(database, testSecret, NewSender(Config{}))
	site := newWebsite(t, a, "Velowerkstatt")

	for _, tc := range []struct {
		acc  Account
		want error
	}{
		{Account{Host: "", From: "a@b.test"}, ErrAccountHost},
		{Account{Host: "smtp://mail.test", From: "a@b.test"}, ErrAccountHost},
		{Account{Host: "mail.test\r\nRCPT", From: "a@b.test"}, ErrAccountHost},
		{Account{Host: "mail.test", From: "kein-at"}, ErrAccountFrom},
		{Account{Host: "mail.test", From: "a@b.test", Port: 70000}, ErrAccountPort},
		{Account{Host: "mail.test", From: "a@b.test", TLS: "ssl3"}, ErrAccountTLS},
	} {
		tc.acc.WebsiteID = site
		if err := a.Save(ctx, tc.acc, "", false); !errors.Is(err, tc.want) {
			t.Errorf("Save(%+v) = %v, want %v", tc.acc, err, tc.want)
		}
	}
}

// A message about a website with its own account goes through that account; a
// message about the installation, and one about a website without an account,
// through the installation's.
func TestFlushWaehltDasKontoDerWebsite(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()
	install := neuerTestserver(t)
	own := neuerTestserver(t)

	fallback := NewSender(Config{Host: install.host, Port: install.port, From: "cms@install.test", TLS: "none"})
	q := NewQueue(database, fallback, slog.New(slog.DiscardHandler))
	a := NewAccounts(database, testSecret, fallback)
	q.SetAccounts(a)

	withAccount := newWebsite(t, a, "Velowerkstatt")
	without := newWebsite(t, a, "Strickstube")
	if err := a.Save(ctx, Account{WebsiteID: withAccount, Host: own.host, Port: own.port,
		From: "info@velo.test", FromName: "Velowerkstatt", TLS: "none"}, "", false); err != nil {
		t.Fatal(err)
	}

	for _, site := range []int64{0, withAccount, without} {
		if err := q.Enqueue(ctx, site, Message{To: "eva@example.test", Subject: "S", Body: "B"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.Flush(ctx); err != nil {
		t.Fatal(err)
	}

	if got := own.messages(); len(got) != 1 || !strings.Contains(got[0], `From: "Velowerkstatt" <info@velo.test>`) {
		t.Errorf("own account got %q", got)
	}
	if got := install.messages(); len(got) != 2 {
		t.Errorf("installation account got %d messages, want 2", len(got))
	}
}

// With no installation account, a website's own account still sends, and the
// installation's messages wait instead of failing.
func TestFlushNurMitKontoDerWebsite(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()
	own := neuerTestserver(t)

	q := NewQueue(database, NewSender(Config{}), slog.New(slog.DiscardHandler))
	a := NewAccounts(database, testSecret, NewSender(Config{}))
	q.SetAccounts(a)
	site := newWebsite(t, a, "Velowerkstatt")
	if err := a.Save(ctx, Account{WebsiteID: site, Host: own.host, Port: own.port,
		From: "info@velo.test", TLS: "none"}, "", false); err != nil {
		t.Fatal(err)
	}
	if !q.EnabledFor(ctx, site) || q.EnabledFor(ctx, 0) {
		t.Errorf("EnabledFor(site)=%v, EnabledFor(0)=%v", q.EnabledFor(ctx, site), q.EnabledFor(ctx, 0))
	}

	_ = q.Enqueue(ctx, 0, Message{To: "eva@example.test", Subject: "Einladung", Body: "B"})
	_ = q.Enqueue(ctx, site, Message{To: "eva@example.test", Subject: "Anfrage", Body: "B"})
	if err := q.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if n := len(own.messages()); n != 1 {
		t.Errorf("own account got %d messages, want 1", n)
	}
	st, _ := q.Status(ctx)
	if st.Pending != 1 || st.Failed != 0 || st.LastError != "" {
		t.Errorf("installation message should wait untouched: %+v", st)
	}
}

// A stored password is not carried to another server, port or user: that
// would hand a secret to a destination it was never typed for.
func TestKontoPasswortBleibtNichtBeiAnderemZiel(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()
	a := NewAccounts(database, testSecret, NewSender(Config{}))
	site := newWebsite(t, a, "Velowerkstatt")

	acc := Account{WebsiteID: site, Host: "mail.velo.test", Port: 587, User: "info", From: "info@velo.test"}
	if err := a.Save(ctx, acc, "geheim-123", false); err != nil {
		t.Fatalf("Save: %v", err)
	}

	for name, change := range map[string]func(*Account){
		"host": func(x *Account) { x.Host = "evil.example.com" },
		"port": func(x *Account) { x.Port = 25 },
		"user": func(x *Account) { x.User = "root" },
	} {
		changed := acc
		change(&changed)
		if err := a.Save(ctx, changed, "", true); !errors.Is(err, ErrAccountPasswordNeeded) {
			t.Errorf("%s: err = %v, want ErrAccountPasswordNeeded", name, err)
		}
	}
	got, err := a.Get(ctx, site)
	if err != nil || got.Host != "mail.velo.test" || got.Port != 587 || got.User != "info" {
		t.Fatalf("a refused save changed the row: %+v, %v", got, err)
	}

	// With a new password the move is fine.
	changed := acc
	changed.Host = "smtp.velo.test"
	if err := a.Save(ctx, changed, "neu-456", false); err != nil {
		t.Fatalf("Save with new password: %v", err)
	}
	s, _ := a.SenderFor(ctx, site)
	if s.Config().Password != "neu-456" || s.Config().Host != "smtp.velo.test" {
		t.Errorf("after move: %+v", s.Config())
	}
}
