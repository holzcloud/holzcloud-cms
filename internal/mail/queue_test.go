package mail

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return database
}

// Einreihen darf nie irgendwohin verbinden: eine Anfrage, die auf einen
// Mailserver wartet, ist eine Anfrage, die scheitert, wenn er nicht antwortet.
func TestEinreihenVerbindetNicht(t *testing.T) {
	database := newTestDB(t)
	// Ein Wirt, den es garantiert nicht gibt. Wenn Enqueue verbinden würde,
	// hinge dieser Test statt durchzulaufen.
	q := NewQueue(database, NewSender(Config{
		Host: "gibt.es.nicht.invalid", From: "cms@example.test", Timeout: time.Second,
	}), slog.New(slog.DiscardHandler))

	if err := q.Enqueue(context.Background(), 0, Message{
		To: "eva@example.test", Subject: "Test", Body: "Text",
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	st, err := q.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Pending != 1 {
		t.Errorf("Pending = %d, want 1", st.Pending)
	}
}

// Eine Nachricht, die durchgeht, wird zugestellt und als zugestellt vermerkt.
func TestFlushStelltZuUndVermerktEs(t *testing.T) {
	database := newTestDB(t)
	server := neuerTestserver(t)
	q := NewQueue(database, NewSender(Config{
		Host: server.host, Port: server.port, From: "cms@example.test", TLS: "none",
	}), slog.New(slog.DiscardHandler))

	ctx := context.Background()
	if err := q.Enqueue(ctx, 0, Message{
		To: "eva@example.test", Subject: "Einladung", Body: "Hallo",
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := q.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	zugestellt := server.messages()
	if len(zugestellt) != 1 {
		t.Fatalf("%d Nachrichten kamen an, want 1", len(zugestellt))
	}
	if !strings.Contains(zugestellt[0], "Subject: Einladung") {
		t.Errorf("die Nachricht sieht falsch aus:\n%s", zugestellt[0])
	}

	st, _ := q.Status(ctx)
	if st.Pending != 0 || st.LastSent == nil {
		t.Errorf("nach dem Versand: Pending=%d, LastSent=%v", st.Pending, st.LastSent)
	}

	// Und ein zweiter Durchgang schickt sie nicht noch einmal.
	if err := q.Flush(ctx); err != nil {
		t.Fatalf("zweites Flush: %v", err)
	}
	if n := len(server.messages()); n != 1 {
		t.Errorf("die Nachricht ging %d mal raus", n)
	}
}

// Ein einzelner Punkt auf einer eigenen Zeile wird auf der Leitung genau einmal
// verdoppelt — nicht null mal, dann bricht die Nachricht ab, und nicht zweimal,
// dann kommt sie mit zwei Punkten an.
//
// Das lässt sich nur hier prüfen, wo eine echte SMTP-Sitzung läuft: compose
// allein sieht die Maskierung nie, weil sie textproto.DotWriter macht.
func TestPunktAufDerLeitungGenauEinmalVerdoppelt(t *testing.T) {
	database := newTestDB(t)
	server := neuerTestserver(t)
	q := NewQueue(database, NewSender(Config{
		Host: server.host, Port: server.port, From: "cms@example.test", TLS: "none",
	}), slog.New(slog.DiscardHandler))

	ctx := context.Background()
	if err := q.Enqueue(ctx, 0, Message{
		To: "eva@example.test", Subject: "Anfrage",
		Body: "Erste Zeile\n.\nZweite Zeile",
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	zugestellt := server.messages()
	if len(zugestellt) != 1 {
		t.Fatalf("%d Nachrichten kamen an", len(zugestellt))
	}
	if !strings.Contains(zugestellt[0], "\r\n..\r\n") {
		t.Errorf("der Punkt wurde nicht verdoppelt:\n%q", zugestellt[0])
	}
	if strings.Contains(zugestellt[0], "\r\n...\r\n") {
		t.Errorf("der Punkt wurde zweimal verdoppelt:\n%q", zugestellt[0])
	}
}

// Ein Mailserver, der nicht antwortet, darf die Nachricht nicht verlieren — und
// er darf auch nicht sofort wieder gefragt werden.
func TestFehlschlagWirdSpaeterErneutVersucht(t *testing.T) {
	database := newTestDB(t)
	q := NewQueue(database, NewSender(Config{
		Host: "127.0.0.1", Port: 1, From: "cms@example.test", TLS: "none",
		Timeout: 500 * time.Millisecond,
	}), slog.New(slog.DiscardHandler))

	ctx := context.Background()
	if err := q.Enqueue(ctx, 0, Message{To: "eva@example.test", Subject: "A", Body: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := q.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var attempts int
	var nextTry string
	if err := database.Read.QueryRow(
		`SELECT attempts, next_try FROM mail_outbox WHERE sent_at IS NULL`).
		Scan(&attempts, &nextTry); err != nil {
		t.Fatalf("die Nachricht ist verschwunden: %v", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
	// Der nächste Versuch liegt in der Zukunft, sonst würde der Auftrag alle
	// dreissig Sekunden gegen dieselbe Wand laufen.
	next, err := time.Parse(timeLayout, nextTry)
	if err != nil {
		t.Fatalf("next_try ist unlesbar: %v", err)
	}
	if !next.After(time.Now().UTC()) {
		t.Errorf("next_try liegt nicht in der Zukunft: %s", nextTry)
	}

	st, _ := q.Status(ctx)
	if st.LastError == "" {
		t.Error("der Grund wurde nicht vermerkt")
	}

	// Und Retry holt sie zurück in die Gegenwart, für den Betreiber, der gerade
	// das Passwort korrigiert hat.
	if n, err := q.Retry(ctx); err != nil || n != 1 {
		t.Errorf("Retry: %d, %v", n, err)
	}
}

// Ohne eingerichteten Mailserver bleibt alles liegen, statt zu scheitern.
func TestOhneMailserverBleibtLiegen(t *testing.T) {
	database := newTestDB(t)
	q := NewQueue(database, NewSender(Config{}), slog.New(slog.DiscardHandler))

	ctx := context.Background()
	if err := q.Enqueue(ctx, 0, Message{To: "eva@example.test", Subject: "A", Body: "x"}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := q.Flush(ctx); err != nil {
		t.Errorf("Flush ohne Mailserver ist ein Fehler: %v", err)
	}
	st, _ := q.Status(ctx)
	if st.Configured {
		t.Error("meldet sich als eingerichtet")
	}
	if st.Pending != 1 {
		t.Errorf("Pending = %d, want 1", st.Pending)
	}
}

// --- ein sehr kleiner SMTP-Server -------------------------------------------

// testserver spricht gerade so viel SMTP, dass net/smtp zufrieden ist.
//
// Ein echter Server im Test und kein nachgebauter Sender: das, was hier schief
// gehen kann, ist genau die Reihenfolge der Befehle und das Format der Daten,
// und beides prüft nur etwas, das wirklich zuhört.
type testserver struct {
	host string
	port int

	mu   sync.Mutex
	msgs []string
}

func (s *testserver) messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.msgs))
	copy(out, s.msgs)
	return out
}

func neuerTestserver(t *testing.T) *testserver {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	addr := ln.Addr().(*net.TCPAddr)
	s := &testserver{host: "127.0.0.1", port: addr.Port}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handle(conn)
		}
	}()
	return s
}

func (s *testserver) handle(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := func(format string, args ...any) {
		fmt.Fprintf(conn, format+"\r\n", args...)
	}

	w("220 test ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			w("250-test")
			w("250 SIZE 10240000")
		case strings.HasPrefix(cmd, "MAIL FROM"), strings.HasPrefix(cmd, "RCPT TO"):
			w("250 OK")
		case cmd == "DATA":
			w("354 los")
			var body strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				body.WriteString(l)
			}
			s.mu.Lock()
			s.msgs = append(s.msgs, body.String())
			s.mu.Unlock()
			w("250 angenommen")
		case cmd == "QUIT":
			w("221 tschüss")
			return
		default:
			w("250 OK")
		}
	}
}
