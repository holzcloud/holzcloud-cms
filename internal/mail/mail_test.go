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

// Der Betreff einer Benachrichtigung wird von einem Fremden getippt. Ein
// Zeilenumbruch darin beendet die Betreffzeile und beginnt, was der Angreifer
// als Nächstes schreibt — zum Beispiel einen zweiten Empfänger.
func TestKopfzeilenLassenSichNichtEinschleusen(t *testing.T) {
	s := testSender()
	roh := s.compose(Message{
		To:      "eva@example.test",
		Subject: "Anfrage\r\nBcc: opfer@example.test",
		Body:    "Text.",
	})

	// Der Umbruch wird zu einem Leerzeichen: "Bcc:" steht dann mitten in der
	// Betreffzeile und ist Text, keine Kopfzeile. Geprüft wird deshalb, dass
	// keine Zeile damit anfängt — und dass es weiterhin genau eine Betreffzeile
	// gibt.
	kopf, _, _ := strings.Cut(roh, "\r\n\r\n")
	for _, zeile := range strings.Split(kopf, "\r\n") {
		if strings.HasPrefix(strings.ToLower(zeile), "bcc:") {
			t.Errorf("eine zweite Kopfzeile kam durch:\n%s", kopf)
		}
	}
	if strings.Count(kopf, "Subject:") != 1 {
		t.Errorf("die Betreffzeile wurde gespalten:\n%s", kopf)
	}
}

// Dasselbe für den Empfänger und die Antwortadresse: beide kommen aus dem
// Formular eines Besuchers, wenn ein Plugin eine Benachrichtigung schickt.
func TestEmpfaengerUndAntwortadresseWerdenGesaeubert(t *testing.T) {
	s := testSender()
	roh := s.compose(Message{
		To:      "eva@example.test",
		ReplyTo: "besucher@example.test\r\nBcc: opfer@example.test",
		Subject: "Anfrage",
		Body:    "Text.",
	})
	// Hier ist die Antwort strenger: was keine Adresse ist, fliegt ganz raus.
	kopf, _, _ := strings.Cut(roh, "\r\n\r\n")
	if strings.Contains(kopf, "Reply-To:") {
		t.Errorf("die verunstaltete Antwortadresse wurde übernommen:\n%s", kopf)
	}
}

// Eine saubere Antwortadresse muss aber ankommen — sie ist der Grund, warum
// Antworten auf eine Anfrage ein Klick ist.
func TestSaubereAntwortadresseBleibt(t *testing.T) {
	s := testSender()
	roh := s.compose(Message{
		To: "eva@example.test", ReplyTo: "besucher@example.test",
		Subject: "Anfrage", Body: "Text.",
	})
	if !strings.Contains(roh, "Reply-To: besucher@example.test\r\n") {
		t.Errorf("die Antwortadresse fehlt:\n%s", roh)
	}
}

// compose baut die Nachricht, die Übertragung maskiert sie. Wer hier einen
// einzelnen Punkt verdoppelt, verdoppelt ihn ein zweites Mal, weil
// textproto.DotWriter das schon tut — und dann kommt bei einem Besucher, der
// einen Punkt getippt hat, ein doppelter an. Genau das ist passiert.
func TestComposeMaskiertDenPunktNicht(t *testing.T) {
	s := testSender()
	roh := s.compose(Message{
		To:      "eva@example.test",
		Subject: "Anfrage",
		Body:    "Erste Zeile\n.\nZweite Zeile",
	})
	_, rumpf, _ := strings.Cut(roh, "\r\n\r\n")
	if strings.Contains(rumpf, "\r\n..") {
		t.Errorf("der Punkt wurde hier schon verdoppelt:\n%q", rumpf)
	}
	if !strings.Contains(rumpf, "\r\n.\r\n") {
		t.Errorf("der Punkt fehlt ganz:\n%q", rumpf)
	}
}

// Ein deutscher Betreff in einer rohen Kopfzeile kommt in etwa der Hälfte aller
// Mailprogramme als Buchstabensalat an.
func TestUmlauteImBetreffWerdenKodiert(t *testing.T) {
	s := testSender()
	roh := s.compose(Message{
		To: "eva@example.test", Subject: "Anfrage zu Grösse M", Body: "x",
	})
	kopf, _, _ := strings.Cut(roh, "\r\n\r\n")
	if strings.Contains(kopf, "Grösse") {
		t.Errorf("der Umlaut steht roh in der Kopfzeile:\n%s", kopf)
	}
	if !strings.Contains(kopf, "=?utf-8?") {
		t.Errorf("der Betreff wurde nicht kodiert:\n%s", kopf)
	}
}

// Ein reiner ASCII-Betreff soll lesbar bleiben und nicht ohne Grund kodiert
// werden — kodierte Kopfzeilen sind ein Spam-Signal.
func TestEinfacherBetreffBleibtLesbar(t *testing.T) {
	s := testSender()
	roh := s.compose(Message{To: "eva@example.test", Subject: "Neue Anfrage", Body: "x"})
	if !strings.Contains(roh, "Subject: Neue Anfrage\r\n") {
		t.Errorf("der Betreff wurde unnötig verändert:\n%s", roh)
	}
}

// Ein Komma im Anzeigenamen würde die Adressliste spalten.
func TestAnzeigenameWirdInAnfuehrungszeichenGesetzt(t *testing.T) {
	s := NewSender(Config{
		Host: "mail.example.test", From: "cms@example.test",
		FromName: `Velowerkstatt, Musterhausen`,
	})
	roh := s.compose(Message{To: "eva@example.test", Subject: "A", Body: "x"})
	if !strings.Contains(roh, `From: "Velowerkstatt, Musterhausen" <cms@example.test>`) {
		t.Errorf("der Anzeigename wurde nicht geschützt:\n%s", roh)
	}
}

// Jede Zeile im Rumpf braucht CRLF, sonst zählen manche Server die Nachricht
// als eine einzige sehr lange Zeile.
func TestZeilenendenWerdenVereinheitlicht(t *testing.T) {
	s := testSender()
	roh := s.compose(Message{To: "eva@example.test", Subject: "A", Body: "eins\nzwei\r\ndrei\rvier"})
	_, rumpf, _ := strings.Cut(roh, "\r\n\r\n")
	if strings.Contains(strings.ReplaceAll(rumpf, "\r\n", ""), "\n") ||
		strings.Contains(strings.ReplaceAll(rumpf, "\r\n", ""), "\r") {
		t.Errorf("es blieben einzelne Zeilenenden übrig: %q", rumpf)
	}
}

// Eine Endlosschleife zwischen zwei Abwesenheitsnotizen merkt niemand, bis das
// Postfach voll ist.
func TestNachrichtIstAlsMaschinellMarkiert(t *testing.T) {
	s := testSender()
	roh := s.compose(Message{To: "eva@example.test", Subject: "A", Body: "x"})
	if !strings.Contains(roh, "Auto-Submitted: auto-generated") {
		t.Error("die Nachricht ist nicht als maschinell erzeugt markiert")
	}
}

func TestOhneEinrichtungWirdNichtsVerschickt(t *testing.T) {
	s := NewSender(Config{})
	if s.Enabled() {
		t.Fatal("ein Sender ohne Wirt meldet sich als eingerichtet")
	}
	if err := s.Send(Message{To: "eva@example.test", Subject: "A", Body: "x"}); err != ErrNotConfigured {
		t.Errorf("Send lieferte %v, want ErrNotConfigured", err)
	}
}

func TestAdressenWerdenGeprueft(t *testing.T) {
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
