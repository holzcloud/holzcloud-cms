package admin

import (
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The screen that asks for the password a second time.
//
// It stands in front of the handful of actions that destroy something a backup
// would have to bring back — see auth.RequireFreshPassword for why there are so
// few of them.

// ConfirmData is the confirmation screen.
type ConfirmData struct {
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

		ok, err := auth.VerifyPassword(strings.TrimSpace(r.FormValue("password")), person.Password)
		if err != nil || !ok {
			// No hint about which part was wrong, and no throttling here: this
			// is somebody who is already signed in, and the password is the
			// same one the login screen already protects.
			data := ConfirmData{
				LayoutData: web.NewLayoutData(r, h.sm, "Bitte bestätigen"),
				FormState:  web.NewFormState(),
				Back:       back,
			}
			data.Errors.Add("password", "Das Passwort stimmt nicht")
			return web.RenderFormError(w, h.templates, r, "confirm", data)
		}

		auth.MarkElevated(h.sm, r.Context())
		web.SetFlashSuccess(h.sm, r.Context(),
			"Bestätigt. Der Knopf, den du gedrückt hast, funktioniert jetzt für die nächsten 15 Minuten.")
		http.Redirect(w, r, back, http.StatusSeeOther)
		return nil
	}

	data := ConfirmData{
		LayoutData: web.NewLayoutData(r, h.sm, "Bitte bestätigen"),
		FormState:  web.NewFormState(),
		Back:       back,
	}
	return web.RenderAdmin(w, h.templates, r, "confirm", data)
}
