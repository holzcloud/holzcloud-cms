package admin

import (
	"fmt"
	"net/http"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// MailStatusData is the outbox screen.
type MailStatusData struct {
	web.LayoutData
	Status mail.Status
	// OwnEmail is where the test message goes: the address of whoever is
	// looking at the screen. Not a field to type into — a form that could send
	// to any address is a form somebody will eventually use to send spam, and
	// the one person a test needs to reach is the one running the test.
	OwnEmail string
}

// HandleMailStatus shows what the outbox is doing.
func (h *Handler) HandleMailStatus(w http.ResponseWriter, r *http.Request) error {
	st, err := h.mail.Status(r.Context())
	if err != nil {
		return err
	}
	data := MailStatusData{
		LayoutData: web.NewLayoutData(r, h.sm, "E-Mail"),
		Status:     st,
		OwnEmail:   h.sm.GetString(r.Context(), auth.SessionKeyUserEmail),
	}
	data.ActiveNav = "mail"
	return web.RenderAdmin(w, h.templates, r, "mail_status", data)
}

// HandleMailTest queues a message to the person asking for it.
func (h *Handler) HandleMailTest(w http.ResponseWriter, r *http.Request) error {
	to := h.sm.GetString(r.Context(), auth.SessionKeyUserEmail)
	if to == "" {
		web.SetFlashError(h.sm, r.Context(), "Für dein Konto ist keine Adresse hinterlegt.")
		return h.redirect(w, r, "/admin/mail")
	}
	if !h.mail.Enabled() {
		web.SetFlashError(h.sm, r.Context(),
			"Es ist kein Mailserver eingerichtet — setze HOLZCLOUD_SMTP_HOST und HOLZCLOUD_SMTP_FROM.")
		return h.redirect(w, r, "/admin/mail")
	}

	err := h.mail.Enqueue(r.Context(), 0, mail.Message{
		To:      to,
		Subject: "Testnachricht von Holzcloud",
		Body: fmt.Sprintf(`Diese Nachricht bestätigt, dass der Versand eingerichtet ist.

Verschickt am %s.

Wenn sie angekommen ist, funktionieren auch Einladungen, Passwort-Links und
Benachrichtigungen über neue Anfragen.
`, time.Now().UTC().Format("02.01.2006 15:04")+" UTC"),
	})
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Einreihen fehlgeschlagen: %s", err))
		return h.redirect(w, r, "/admin/mail")
	}
	// Queued, not sent: the job picks it up within half a minute. Saying
	// "queued" rather than "sent" is the honest word — whether it arrives is
	// what the screen below reports afterwards.
	web.SetFlashSuccess(h.sm, r.Context(),
		"Testnachricht an "+to+" eingereiht. Sie geht in den nächsten Sekunden raus.")
	return h.redirect(w, r, "/admin/mail")
}

// HandleMailRetry puts everything that was given up on back in the queue.
func (h *Handler) HandleMailRetry(w http.ResponseWriter, r *http.Request) error {
	n, err := h.mail.Retry(r.Context())
	if err != nil {
		return err
	}
	if n == 0 {
		web.SetFlashSuccess(h.sm, r.Context(), "Es liegt nichts mehr an.")
	} else {
		web.SetFlashSuccess(h.sm, r.Context(),
			fmt.Sprintf("%d Nachrichten werden erneut versucht.", n))
	}
	return h.redirect(w, r, "/admin/mail")
}
