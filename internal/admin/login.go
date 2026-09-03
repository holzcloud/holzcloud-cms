package admin

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// HandleLoginForm renders the login page.
func (h *Handler) HandleLoginForm(w http.ResponseWriter, r *http.Request) error {
	data := web.NewLayoutData(r, h.sm, "Anmelden")
	return web.RenderAdmin(w, h.templates, r, "login", data)
}

// HandleLogin processes the login form submission.
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	ip := h.clientIP.ClientIP(r)
	account := strings.ToLower(email)

	if !h.loginThrottle.Allowed(ip, account) {
		wait := h.loginThrottle.RetryAfter(ip, account)
		slog.Warn("login rate limited", "ip", ip, "retry_after_s", int(wait.Seconds()))
		web.SetFlashError(h.sm, r.Context(),
			fmt.Sprintf("Too many failed attempts. Try again in %d minutes.", int(wait.Minutes())+1))
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}

	var id int64
	var storedEmail, storedPassword, role string
	err := h.db.Read.QueryRowContext(r.Context(),
		"SELECT id, email, password, role FROM users WHERE email = $1", email,
	).Scan(&id, &storedEmail, &storedPassword, &role)

	if err == sql.ErrNoRows {
		// Hash against a dummy password so an unknown email costs the same as a
		// known one. Without this the response time reveals which emails exist.
		auth.VerifyDummyPassword(password, h.argon2Params)
		h.loginThrottle.RecordFailure(ip, account)
		// Die versuchte Adresse und nicht die Kennung: bei einem Fehlversuch
		// gibt es kein angemeldetes Konto, und gerade die Adresse ist es, die
		// man später wiedererkennen will.
		h.LogActivity(r, activity.Entry{
			ActorEmail: account,
			Action:     activity.ActionAuthLoginFail,
			EntityType: "user",
		})
		web.SetFlashError(h.sm, r.Context(), "E-Mail-Adresse oder Passwort stimmt nicht")
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}
	if err != nil {
		return err
	}

	match, err := auth.VerifyPassword(password, storedPassword)
	if err != nil || !match {
		h.loginThrottle.RecordFailure(ip, account)
		web.SetFlashError(h.sm, r.Context(), "E-Mail-Adresse oder Passwort stimmt nicht")
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}

	h.loginThrottle.Reset(ip, account)

	// Rotate session ID BEFORE setting values (prevents session fixation)
	if err := h.sm.RenewToken(r.Context()); err != nil {
		return err
	}

	// An account with an authenticator is not signed in yet. The session gets a
	// pending id rather than user_id, so nothing that checks for a signed-in
	// user sees one until the code is entered.
	tf, err := h.users.GetTwoFactor(r.Context(), id)
	if err != nil {
		return err
	}
	if tf != nil && tf.Enabled() {
		h.sm.Put(r.Context(), auth.SessionKeyPendingUserID, id)
		http.Redirect(w, r, auth.VerifyPath, http.StatusSeeOther)
		return nil
	}

	h.completeLogin(r, id, role, storedEmail)
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
	return nil
}

// completeLogin puts the account into the session and records the sign-in.
//
// It is the single place a session becomes signed in, so the password path and
// the second-factor path cannot drift apart.
func (h *Handler) completeLogin(r *http.Request, id int64, role, email string) {
	h.sm.Remove(r.Context(), auth.SessionKeyPendingUserID)
	h.sm.Put(r.Context(), auth.SessionKeyUserID, id)
	h.sm.Put(r.Context(), auth.SessionKeyUserRole, role)
	h.sm.Put(r.Context(), auth.SessionKeyUserEmail, email)

	// Recording the last login is what makes a forgotten account visible in the
	// user list. It must not be able to fail a successful sign-in.
	if err := h.users.RecordLogin(r.Context(), id); err != nil {
		slog.Error("record last login", "err", err, "user_id", id)
	}

	h.LogActivity(r, activity.Entry{
		UserID:     &id,
		ActorEmail: email,
		Action:     activity.ActionAuthLoginSuccess,
		EntityType: "user",
		EntityID:   id,
	})
}

// HandleLogout destroys the session and redirects to the login page.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) error {
	// Vor dem Zerstören: danach weiss die Sitzung nicht mehr, wer gegangen ist.
	h.LogActivity(r, activity.Entry{Action: activity.ActionAuthLogout, EntityType: "user"})
	if err := h.sm.Destroy(r.Context()); err != nil {
		return err
	}
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	return nil
}
