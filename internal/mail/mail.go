// Package mail sends the few messages this CMS has reason to send.
//
// It is the one place in the application that opens a connection outwards, and
// that deserves a word, because the project's rule is that nothing is fetched
// from a third party while it runs. That rule is about the browser: every
// subresource a visitor's browser requests comes from this server's own origin,
// and that has not changed — no CDN, no web font, no tracking pixel, and the
// Content-Security-Policy still says so.
//
// Sending mail is a different act. The server, not the visitor, connects to one
// server the operator named, to deliver something the operator asked for. It is
// off unless configured, and with it off the application behaves exactly as it
// did before this package existed: an invitation link is shown once on screen
// and handed over by whatever way people already talk to each other.
//
// Plain text only, never HTML. HTML mail is where remote images and tracking
// pixels live, and a CMS that refuses them on its own pages has no business
// putting them in someone's inbox.
package mail

import (
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Config is what an operator sets to switch sending on.
type Config struct {
	Host string
	Port int
	// User and Password may be empty for a relay that authenticates by network
	// address, which is the usual arrangement for a mail server on the same
	// machine.
	User     string
	Password string
	// From is the envelope sender and the From header. Required: a message
	// without one is refused by most receivers and is not worth queuing.
	From string
	// FromName is the display name, e.g. "Velowerkstatt Beispiel".
	FromName string
	// TLS is "starttls" (default), "tls" for a connection that is encrypted
	// from the first byte, or "none".
	TLS     string
	Timeout time.Duration
}

// Enabled reports whether sending is configured at all.
func (c Config) Enabled() bool { return c.Host != "" && c.From != "" }

// Addr is host:port.
func (c Config) Addr() string { return net.JoinHostPort(c.Host, fmt.Sprint(c.Port)) }

// Message is one mail. Plain text, one recipient.
//
// One recipient rather than a list: every message this CMS sends is to one
// person about one thing, and a Bcc field would be the beginning of a mailing
// tool, which is not what this is.
type Message struct {
	To      string
	Subject string
	Body    string
	// ReplyTo is where an answer should go — the visitor's address on an
	// enquiry notification, so the operator can simply hit reply.
	ReplyTo string
	// FromName overrides the account's display name for this one message. One
	// installation serves several websites, and a shop confirmation should be
	// signed by the shop and not by the installation. The address itself never
	// changes: the provider only lets the account send as its own mailbox.
	FromName string
}

// ErrNotConfigured is returned when sending is off.
var ErrNotConfigured = errors.New("es ist kein Mailserver eingerichtet")

// Sender delivers messages over SMTP.
type Sender struct{ cfg Config }

// NewSender creates a sender. It does not connect.
func NewSender(cfg Config) *Sender {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 20 * time.Second
	}
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	if cfg.TLS == "" {
		cfg.TLS = "starttls"
	}
	return &Sender{cfg: cfg}
}

// Enabled reports whether this sender will do anything.
func (s *Sender) Enabled() bool { return s != nil && s.cfg.Enabled() }

// Config returns the settings, for a status screen.
func (s *Sender) Config() Config { return s.cfg }

// Send delivers one message, blocking until the server has accepted it.
//
// Callers on a request path should not use this directly — a mail server that
// takes ten seconds to answer would be ten seconds a visitor waits. The queue
// in this package is what requests use.
func (s *Sender) Send(m Message) error {
	if !s.Enabled() {
		return ErrNotConfigured
	}
	if err := validAddress(m.To); err != nil {
		return err
	}

	conn, err := s.dial()
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("mailserver antwortet nicht wie erwartet: %w", err)
	}
	defer client.Close()

	if s.cfg.TLS == "starttls" {
		if err := client.StartTLS(&tls.Config{ServerName: s.cfg.Host}); err != nil {
			return fmt.Errorf("verschlüsselung fehlgeschlagen: %w", err)
		}
	}
	if s.cfg.User != "" {
		// smtp.PlainAuth refuses to send credentials over an unencrypted
		// connection unless the server is localhost. That check is left in
		// place: a password on the wire is worth failing a send over.
		auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("anmeldung am mailserver fehlgeschlagen: %w", err)
		}
	}

	if err := client.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("absender abgelehnt: %w", err)
	}
	if err := client.Rcpt(m.To); err != nil {
		return fmt.Errorf("empfänger abgelehnt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailserver nimmt keine daten an: %w", err)
	}
	if _, err := w.Write([]byte(s.compose(m))); err != nil {
		w.Close()
		return fmt.Errorf("schreiben fehlgeschlagen: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mailserver hat die nachricht abgelehnt: %w", err)
	}
	return client.Quit()
}

func (s *Sender) dial() (net.Conn, error) {
	d := &net.Dialer{Timeout: s.cfg.Timeout}
	if s.cfg.TLS == "tls" {
		conn, err := tls.DialWithDialer(d, "tcp", s.cfg.Addr(), &tls.Config{ServerName: s.cfg.Host})
		if err != nil {
			return nil, fmt.Errorf("verbindung zum mailserver fehlgeschlagen: %w", err)
		}
		return conn, nil
	}
	conn, err := d.Dial("tcp", s.cfg.Addr())
	if err != nil {
		return nil, fmt.Errorf("verbindung zum mailserver fehlgeschlagen: %w", err)
	}
	return conn, nil
}

// compose builds the message.
//
// Everything that reaches a header is checked for CR and LF first. A newline in
// a subject line is header injection: it ends the Subject header and starts
// whatever the attacker writes next, including a second recipient. The subject
// of an enquiry notification is typed by a stranger, so this is not theoretical.
func (s *Sender) compose(m Message) string {
	var b strings.Builder
	from := s.cfg.From
	// The message's own name wins over the account's: see Message.FromName.
	display := s.cfg.FromName
	if m.FromName != "" {
		display = m.FromName
	}
	if name := header(display); name != "" {
		from = fmt.Sprintf("%s <%s>", quoteName(name), s.cfg.From)
	}

	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + header(m.To) + "\r\n")
	// An address or nothing. The reply address of a notification is typed by a
	// visitor, and folding a newline into a space leaves a Reply-To that is not
	// an address — harmless, because envelope recipients come from RCPT and not
	// from headers, but it is rubbish in someone's mail client either way.
	if rt := header(m.ReplyTo); validAddress(rt) == nil {
		b.WriteString("Reply-To: " + rt + "\r\n")
	}
	b.WriteString("Subject: " + encodeSubject(header(m.Subject)) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	// Not an automatic reply to an automatic reply: without this, an out-of-
	// office on the other end and a notification on this end can bounce back and
	// forth until someone notices.
	b.WriteString("Auto-Submitted: auto-generated\r\n")
	b.WriteString("\r\n")
	b.WriteString(normaliseNewlines(m.Body))
	return b.String()
}

// header strips anything that could start a new header line.
func header(s string) string {
	s = strings.NewReplacer("\r", " ", "\n", " ", "\x00", "").Replace(s)
	return strings.TrimSpace(s)
}

// quoteName wraps a display name so a comma or a colon in it cannot split the
// address list.
func quoteName(s string) string {
	return `"` + strings.NewReplacer(`\`, ``, `"`, ``).Replace(s) + `"`
}

// encodeSubject puts a non-ASCII subject into the encoded form RFC 2047 wants.
//
// "Anfrage zu Rohwolle für Grösse M" in a raw header is a subject that arrives
// as mojibake in about half of all mail clients.
func encodeSubject(s string) string {
	if isASCII(s) {
		return s
	}
	return mime.QEncoding.Encode("utf-8", s)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// normaliseNewlines turns every line ending into CRLF, which is what SMTP wants.
func normaliseNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}

// A lone dot on its own line ends the message body in SMTP, so it has to be
// doubled on the way out — but that is not done here. smtp.Client.Data returns
// a textproto.DotWriter, which already does it. Doing it here as well was a bug
// that reached a real mail server before it was caught: a visitor who typed a
// single dot on its own line had it arrive as two.
//
// The rule is therefore: compose builds the message, the transport escapes it.
// Anything that looks like protocol handling in here is in the wrong place.

// validAddress is a shape check. Anything stricter rejects real addresses.
func validAddress(s string) error {
	if s == "" {
		return errors.New("keine empfängeradresse")
	}
	if s != header(s) {
		return errors.New("die empfängeradresse enthält zeilenumbrüche")
	}
	name, host, ok := strings.Cut(s, "@")
	if !ok || name == "" || host == "" || strings.Contains(host, "@") {
		return fmt.Errorf("%q ist keine e-mail-adresse", s)
	}
	return nil
}
