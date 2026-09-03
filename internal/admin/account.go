package admin

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/user"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// AccountLinkData shows a freshly issued one-time link.
//
// It is shown exactly once. The secret is not stored, so there is no second
// screen that could show it again — which is the point.
type AccountLinkData struct {
	web.LayoutData
	User    *user.User
	URL     string
	Expires time.Time
	Purpose string
	// Sent says whether the link also went out by mail. The link is shown on
	// screen either way: mail can be misconfigured, delayed or filtered, and an
	// admin who is told "sent" and then waits for something that never arrives
	// has no way forward.
	Sent bool
	// SendError is why the mail could not even be queued, if that happened.
	SendError string
}

// SetPasswordData is the public form behind an invite or reset link.
type SetPasswordData struct {
	web.LayoutData
	web.FormState
	Token   string
	Purpose string
	Email   string
}

// HandleUserLink issues an invitation or reset link for a user.
//
// The link is always shown on screen, whether or not it was also mailed. Mail
// is the convenience, not the mechanism: a server with no mail set up must
// still be able to invite someone, and an admin who was told "sent" and then
// waits for a message that a spam filter ate needs a way forward that does not
// involve reading server logs.
func (h *Handler) HandleUserLink(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	purpose := r.FormValue("purpose")
	if purpose != user.PurposeInvite {
		purpose = user.PurposeReset
	}

	u, err := h.users.GetByID(r.Context(), id)
	if errors.Is(err, user.ErrNotFound) {
		http.NotFound(w, r)
		return nil
	}
	if err != nil {
		return err
	}

	secret, expires, err := h.users.IssueToken(r.Context(), id, purpose)
	if err != nil {
		return err
	}
	// A reset means the current password is no longer trusted, so any session
	// still holding it goes too.
	if purpose == user.PurposeReset {
		if err := auth.DestroyUserSessions(r.Context(), h.sm, id, ""); err != nil {
			slog.Error("destroy sessions after issuing a reset link", "err", err, "user_id", id)
		}
	}

	path := "/admin/reset/"
	if purpose == user.PurposeInvite {
		path = "/admin/activate/"
	}

	link := h.absoluteAdminURL(r, path+secret)
	data := AccountLinkData{
		LayoutData: web.NewLayoutData(r, h.sm, "Zugangslink"),
		User:       u,
		URL:        link,
		Expires:    expires,
		Purpose:    purpose,
	}
	if h.mail.Enabled() {
		// Queued, not sent: an SMTP server that takes twenty seconds to answer
		// would be twenty seconds the admin stares at a spinner, and one that
		// is down would turn issuing a link into an error page.
		err := h.mail.Enqueue(r.Context(), 0, accessMail(u, link, expires, purpose))
		switch {
		case err != nil:
			data.SendError = err.Error()
			slog.Error("cannot queue access mail", "err", err, "user_id", id)
		default:
			data.Sent = true
		}
	}
	data.ActiveNav = "users"
	return web.RenderAdmin(w, h.templates, r, "user_link", data)
}

// absoluteAdminURL builds the link an admin copies out of the screen.
//
// The host comes from the request because this is the address the admin is
// already looking at, and the scheme from configuration for the same reason the
// sitemap uses it: a forwarded header must not decide what gets handed to a
// colleague.
func (h *Handler) absoluteAdminURL(r *http.Request, path string) string {
	scheme := "http"
	if h.cfg != nil && h.cfg.Secure {
		scheme = "https"
	}
	return scheme + "://" + r.Host + path
}

// HandleSetPassword serves and accepts the form behind an invite or reset link.
//
// It lives on the public admin mux — someone following a reset link is by
// definition not logged in. It is rate-limited on the same throttle as the login
// form, because it is a second place where a secret can be guessed.
func (h *Handler) HandleSetPassword(purpose string) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		token := r.PathValue("token")

		u, err := h.users.RedeemToken(r.Context(), token, purpose)
		if errors.Is(err, user.ErrTokenInvalid) || errors.Is(err, user.ErrNotFound) {
			return h.invalidLink(w, r)
		}
		if err != nil {
			return err
		}

		data := SetPasswordData{
			LayoutData: web.NewLayoutData(r, h.sm, linkTitle(purpose)),
			FormState:  web.NewFormState(),
			Token:      token,
			Purpose:    purpose,
			Email:      u.Email,
		}

		if r.Method != http.MethodPost {
			return web.RenderAdmin(w, h.templates, r, "set_password", data)
		}

		if err := r.ParseForm(); err != nil {
			return err
		}
		password := r.FormValue("password")
		if len(password) < 8 {
			data.Errors.Add("password", "Das Passwort muss mindestens 8 Zeichen lang sein.")
		}
		if password != r.FormValue("password_confirm") {
			data.Errors.Add("password_confirm", "Die Passwörter stimmen nicht überein.")
		}
		if data.Errors.Any() {
			return web.RenderFormError(w, h.templates, r, "set_password", data)
		}

		// Order matters: the password is stored first, so a failure leaves the
		// link usable rather than burning it with nothing to show for it.
		if err := h.users.SetPassword(r.Context(), u.ID, password); err != nil {
			return err
		}
		if err := h.users.ConsumeToken(r.Context(), token, purpose); err != nil {
			return h.invalidLink(w, r)
		}
		if err := h.users.SetMustChangePassword(r.Context(), u.ID, false); err != nil {
			return err
		}
		// Whoever was logged in with the old password is logged out.
		if err := auth.DestroyUserSessions(r.Context(), h.sm, u.ID, ""); err != nil {
			slog.Error("destroy sessions after password change", "err", err, "user_id", u.ID)
		}

		web.SetFlashSuccess(h.sm, r.Context(), "Passwort gesetzt. Bitte melde dich jetzt an.")
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}
}

func linkTitle(purpose string) string {
	if purpose == user.PurposeInvite {
		return "Zugang einrichten"
	}
	return "Neues Passwort setzen"
}

// invalidLink answers every bad token the same way.
//
// Unknown, expired and already used are not distinguished: saying which applies
// would tell a stranger whether the token ever existed.
func (h *Handler) invalidLink(w http.ResponseWriter, r *http.Request) error {
	data := SetPasswordData{
		LayoutData: web.NewLayoutData(r, h.sm, "Link ungültig"),
		FormState:  web.NewFormState(),
	}
	data.Conflict = "Dieser Link ist abgelaufen oder wurde bereits benutzt. " +
		"Bitte lass dir einen neuen geben."
	return web.RenderAdminStatus(w, h.templates, r, "set_password", data, http.StatusGone)
}

// HandleUserSessions lists the active sessions of a user and can end them.
func (h *Handler) HandleUserSessions(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	if err := auth.DestroyUserSessions(r.Context(), h.sm, id, ""); err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Alle Sitzungen dieses Benutzers wurden beendet")
	return h.redirect(w, r, fmt.Sprintf("/admin/users/%d/edit", id))
}

// accessMail is the invitation or reset message.
//
// Plain text with the link on a line of its own, so every mail client makes it
// clickable and none of them has to be trusted with markup. It deliberately
// says what the link does and when it stops working: a message that says only
// "click here" is indistinguishable from the phishing it will be mistaken for.
func accessMail(u *user.User, link string, expires time.Time, purpose string) mail.Message {
	name := u.Name
	if name == "" {
		name = u.Email
	}
	if purpose == user.PurposeInvite {
		return mail.Message{
			To:      u.Email,
			Subject: "Dein Zugang zu Holzcloud",
			Body: fmt.Sprintf(`Hallo %s

für dich wurde ein Zugang zur Verwaltung angelegt. Über den folgenden Link
vergibst du dein Passwort:

%s

Der Link gilt bis %s und lässt sich nur ein einziges Mal benutzen.
Danach meldest du dich ganz normal mit deiner E-Mail-Adresse an.

Wenn du damit nichts anfangen kannst, ignoriere diese Nachricht einfach —
ohne den Link passiert nichts.
`, name, link, expires.Format("02.01.2006 15:04")+" UTC"),
		}
	}
	return mail.Message{
		To:      u.Email,
		Subject: "Passwort zurücksetzen",
		Body: fmt.Sprintf(`Hallo %s

für dein Konto wurde ein Link zum Zurücksetzen des Passworts erzeugt:

%s

Der Link gilt bis %s und lässt sich nur ein einziges Mal benutzen.
Alle offenen Sitzungen deines Kontos wurden bereits beendet.

Wenn du das nicht angefordert hast, sag der Person Bescheid, die den Server
betreut — jemand mit Zugang zur Verwaltung hat diesen Link erzeugt.
`, name, link, expires.Format("02.01.2006 15:04")+" UTC"),
	}
}
