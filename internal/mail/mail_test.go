package mail

import (
	"strings"
	"testing"
)

func testSender() *Sender {
	return NewSender(Config{
		Host: "mail.example.test", Port: 587,
		From: "cms@example.test", FromName: "Velowerkstatt Beispiel",
	})
}

// The subject of a notification is typed by a stranger. A line break in it ends
// the subject line and begins whatever the attacker writes next — a second
// recipient, for instance.
func TestHeadersCannotBeInjected(t *testing.T) {
	s := testSender()
	raw := s.compose(Message{
		To:      "eva@example.test",
		Subject: "Anfrage\r\nBcc: opfer@example.test",
		Body:    "Text.",
	})

	// The break becomes a space: "Bcc:" then stands in the middle of the subject
	// line and is text, not a header. What is checked is therefore that no line
	// starts with it — and that there is still exactly one subject line.
	kopf, _, _ := strings.Cut(raw, "\r\n\r\n")
	for _, row := range strings.Split(kopf, "\r\n") {
		if strings.HasPrefix(strings.ToLower(row), "bcc:") {
			t.Errorf("eine zweite Kopfzeile kam durch:\n%s", kopf)
		}
	}
	if strings.Count(kopf, "Subject:") != 1 {
		t.Errorf("die Betreffzeile wurde gespalten:\n%s", kopf)
	}
}

// The same for the recipient and the reply address: both come out of a
// visitor's form when a plugin sends a notification.
func TestRecipientAndReplyAddressAreCleaned(t *testing.T) {
	s := testSender()
	raw := s.compose(Message{
		To:      "eva@example.test",
		ReplyTo: "besucher@example.test\r\nBcc: opfer@example.test",
		Subject: "Anfrage",
		Body:    "Text.",
	})
	// Here the answer is stricter: what is not an address flies out entirely.
	kopf, _, _ := strings.Cut(raw, "\r\n\r\n")
	if strings.Contains(kopf, "Reply-To:") {
		t.Errorf("the mangled reply address was taken over:\n%s", kopf)
	}
}

// A clean reply address has to arrive, though — it is the reason replying to an
// enquiry is one click.
func TestACleanReplyAddressStays(t *testing.T) {
	s := testSender()
	raw := s.compose(Message{
		To: "eva@example.test", ReplyTo: "besucher@example.test",
		Subject: "Anfrage", Body: "Text.",
	})
	if !strings.Contains(raw, "Reply-To: besucher@example.test\r\n") {
		t.Errorf("die Antwortadresse fehlt:\n%s", raw)
	}
}

// compose builds the message, the transport escapes it. Whoever doubles a
// single dot here doubles it a second time, because textproto.DotWriter already
// does that — and then a visitor who typed one dot gets two. That is exactly
// what happened.
func TestComposeMaskiertDenPunktNicht(t *testing.T) {
	s := testSender()
	raw := s.compose(Message{
		To:      "eva@example.test",
		Subject: "Anfrage",
		Body:    "Erste Zeile\n.\nZweite Zeile",
	})
	_, rumpf, _ := strings.Cut(raw, "\r\n\r\n")
	if strings.Contains(rumpf, "\r\n..") {
		t.Errorf("the dot was already doubled here:\n%q", rumpf)
	}
	if !strings.Contains(rumpf, "\r\n.\r\n") {
		t.Errorf("der Punkt fehlt ganz:\n%q", rumpf)
	}
}

// A German subject in a raw header arrives as gibberish in roughly half of all
// mail programs.
func TestUmlauteImBetreffWerdenKodiert(t *testing.T) {
	s := testSender()
	raw := s.compose(Message{
		To: "eva@example.test", Subject: "Anfrage zu Grösse M", Body: "x",
	})
	kopf, _, _ := strings.Cut(raw, "\r\n\r\n")
	if strings.Contains(kopf, "Grösse") {
		t.Errorf("the umlaut is raw in the header row:\n%s", kopf)
	}
	if !strings.Contains(kopf, "=?utf-8?") {
		t.Errorf("the subject was not encoded:\n%s", kopf)
	}
}

// A pure ASCII subject should stay readable and not be encoded without reason —
// encoded headers are a spam signal.
func TestEinfacherBetreffBleibtLesbar(t *testing.T) {
	s := testSender()
	raw := s.compose(Message{To: "eva@example.test", Subject: "Neue Anfrage", Body: "x"})
	if !strings.Contains(raw, "Subject: Neue Anfrage\r\n") {
		t.Errorf("the subject was changed without need:\n%s", raw)
	}
}

// A comma in the display name would split the address list.
func TestAnzeigenameWirdInAnfuehrungszeichenGesetzt(t *testing.T) {
	s := NewSender(Config{
		Host: "mail.example.test", From: "cms@example.test",
		FromName: `Velowerkstatt, Musterhausen`,
	})
	raw := s.compose(Message{To: "eva@example.test", Subject: "A", Body: "x"})
	if !strings.Contains(raw, `From: "Velowerkstatt, Musterhausen" <cms@example.test>`) {
		t.Errorf("the display name was not escaped:\n%s", raw)
	}
}

// Every line in the body needs CRLF, or some servers count the message as one
// single very long line.
func TestLineEndingsAreUnified(t *testing.T) {
	s := testSender()
	raw := s.compose(Message{To: "eva@example.test", Subject: "A", Body: "eins\nzwei\r\ndrei\rvier"})
	_, rumpf, _ := strings.Cut(raw, "\r\n\r\n")
	if strings.Contains(strings.ReplaceAll(rumpf, "\r\n", ""), "\n") ||
		strings.Contains(strings.ReplaceAll(rumpf, "\r\n", ""), "\r") {
		t.Errorf("single line endings were left over: %q", rumpf)
	}
}

// An endless loop between two out-of-office notices is noticed by nobody until
// the mailbox is full.
func TestNachrichtIstAlsMaschinellMarkiert(t *testing.T) {
	s := testSender()
	raw := s.compose(Message{To: "eva@example.test", Subject: "A", Body: "x"})
	if !strings.Contains(raw, "Auto-Submitted: auto-generated") {
		t.Error("the message is not marked as machine-generated")
	}
}

func TestOhneEinrichtungWirdNichtsVerschickt(t *testing.T) {
	s := NewSender(Config{})
	if s.Enabled() {
		t.Fatal("a sender with no host reports itself as configured")
	}
	if err := s.Send(Message{To: "eva@example.test", Subject: "A", Body: "x"}); err != ErrNotConfigured {
		t.Errorf("Send lieferte %v, want ErrNotConfigured", err)
	}
}

func TestAddressesAreChecked(t *testing.T) {
	for _, schlecht := range []string{"", "keine-adresse", "@example.test", "eva@", "eva@a\r\nBcc: x@y"} {
		if err := validAddress(schlecht); err == nil {
			t.Errorf("%q wurde als Adresse angenommen", schlecht)
		}
	}
	for _, gut := range []string{"eva@example.test", "eva+laden@sub.example.test"} {
		if err := validAddress(gut); err != nil {
			t.Errorf("%q wurde abgelehnt: %v", gut, err)
		}
	}
}
