package admin

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The screen that asks for the password a second time.
//
// It stands in front of the handful of actions that destroy something a backup
// would have to bring back — see auth.RequireFreshPassword for why there are so
// few of them.

// confirmData is the confirmation screen.
type confirmData struct {
	web.LayoutData
	web.FormState
	// Back is where to return to afterwards: the screen the button was on.
	Back string
}

// HandleConfirmPassword asks for the password and, on success, marks the
// session as freshly confirmed.
func (h *Handler) HandleConfirmPassword(w http.ResponseWriter, r *http.Request) error {
	back := auth.SafeReturn(r.URL.Query().Get("weiter"))

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			return err
		}
		back = auth.SafeReturn(r.FormValue("weiter"))

		userID := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
		person, err := h.getUserByID(r.Context(), userID)
		if err != nil {
			return err
		}
		if person == nil {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return nil
		}

		// Throttled per user and per client, like the login. This is a session
		// proving the password again, and a stolen session could otherwise guess
		// the password here without limit. Reserved before anything is hashed.
		client, _ := h.clientIP.ThrottleKey(r)
		attempt, wait := h.loginThrottle.Confirm().Reserve(client, strconv.FormatInt(userID, 10))
		if attempt == nil {
			slog.Warn("password confirmation rate limited", "user_id", userID, "retry_after_s", int(wait.Seconds()))
			web.RetryAfter(w, wait)
			data := h.confirmForm(r, back)
			data.Errors.Add("password", "Too many failed attempts. Please wait a moment.")
			return web.RenderAdminStatus(w, h.templates, r, "confirm", data, http.StatusTooManyRequests)
		}
		// Every exit that does not settle the attempt gives it back: a busy
		// gate and a server error are not guesses.
		defer attempt.Refund()

		// The submitted value exactly as it arrived. Login does not trim, so a
		// password that ends in a space is a different password, and this check
		// has to agree with the one that set it.
		ok, err := auth.VerifyOwnPassword(r.FormValue("password"), person.Password)
		if errors.Is(err, auth.ErrBusy) {
			web.RetryAfter(w, auth.BusyRetryAfter)
			data := h.confirmForm(r, back)
			data.Errors.Add("password", "The server is busy checking other passwords. Please try again in a moment.")
			return web.RenderAdminStatus(w, h.templates, r, "confirm", data, http.StatusServiceUnavailable)
		}
		if err != nil {
			// A stored hash that cannot be read is the server's fault, not a
			// wrong password: the deferred Refund gives the attempt back, as
			// the password change does with the same throttle (code review
			// IN-05).
			return fmt.Errorf("verify own password: %w", err)
		}
		if !ok {
			attempt.Fail()
			// No hint about which part was wrong.
			data := h.confirmForm(r, back)
			data.Errors.Add("password", "That password is wrong")
			return web.RenderFormError(w, h.templates, r, "confirm", data)
		}
		attempt.Succeed()

		auth.MarkElevated(h.sm, r.Context())
		web.SetFlashSuccess(h.sm, r.Context(),
			"Confirmed. The button you pressed works for the next 15 minutes.")
		http.Redirect(w, r, back, http.StatusSeeOther)
		return nil
	}

	data := confirmData{
		LayoutData: web.NewLayoutData(r, h.sm, "Please confirm"),
		FormState:  web.NewFormState(),
		Back:       back,
	}
	return web.RenderAdmin(w, h.templates, r, "confirm", data)
}

// confirmForm is the confirmation form with nothing said yet. A refusal puts its
// reason on the password field as a catalogue key, at the call site: the
// template translates it, and the collector only finds a key written where
// Errors.Add is called.
func (h *Handler) confirmForm(r *http.Request, back string) confirmData {
	return confirmData{
		LayoutData: web.NewLayoutData(r, h.sm, "Please confirm"),
		FormState:  web.NewFormState(),
		Back:       back,
	}
}
