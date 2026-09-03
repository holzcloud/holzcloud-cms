package admin

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// hasUsers returns true if any users exist in the database.
func hasUsers(ctx context.Context, database *db.DB) (bool, error) {
	var count int
	err := database.Read.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// HasUsers is the exported version for use in the setup guard middleware.
func HasUsers(ctx context.Context, database *db.DB) (bool, error) {
	return hasUsers(ctx, database)
}

// HandleSetupForm renders the setup page if no users exist.
func (h *Handler) HandleSetupForm(w http.ResponseWriter, r *http.Request) error {
	exists, err := hasUsers(r.Context(), h.db)
	if err != nil {
		return err
	}
	if exists {
		http.NotFound(w, r)
		return nil
	}
	data := web.NewLayoutData(r, h.sm, "Einrichtung")
	return web.RenderAdmin(w, h.templates, r, "setup", data)
}

// HandleSetup processes the initial admin account creation.
func (h *Handler) HandleSetup(w http.ResponseWriter, r *http.Request) error {
	exists, err := hasUsers(r.Context(), h.db)
	if err != nil {
		return err
	}
	if exists {
		http.NotFound(w, r)
		return nil
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")

	// Validate
	if email == "" {
		web.SetFlashError(h.sm, r.Context(), "Bitte eine E-Mail-Adresse angeben")
		http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
		return nil
	}
	if len(password) < auth.MinPasswordLength {
		web.SetFlashError(h.sm, r.Context(),
			fmt.Sprintf("Password must be at least %d characters", auth.MinPasswordLength))
		http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
		return nil
	}
	if password != passwordConfirm {
		web.SetFlashError(h.sm, r.Context(), "Die Passwörter stimmen nicht überein")
		http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
		return nil
	}

	hash, err := auth.HashPassword(password, h.argon2Params)
	if err != nil {
		return err
	}

	_, err = h.db.Write.ExecContext(r.Context(),
		"INSERT INTO users (email, password, role) VALUES ($1, $2, 'admin')",
		email, hash)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			web.SetFlashError(h.sm, r.Context(), "Diese E-Mail-Adresse wird bereits benutzt")
			http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
			return nil
		}
		return err
	}

	// Get the new user's ID
	var id int64
	err = h.db.Read.QueryRowContext(r.Context(),
		"SELECT id FROM users WHERE email = $1", email).Scan(&id)
	if err != nil {
		return err
	}

	// Auto-login: rotate the session, then go through the same place every
	// other sign-in goes through. Setting the session by hand here is what
	// made the first administrator the one account whose creation and first
	// sign-in were missing from the protocol.
	if err := h.sm.RenewToken(r.Context()); err != nil {
		return err
	}
	h.LogActivity(r, activity.Entry{
		UserID:     &id,
		ActorEmail: email,
		Action:     activity.ActionUserCreate,
		EntityType: "user",
		EntityID:   id,
		Metadata:   map[string]any{"email": email, "rolle": "admin", "einrichtung": true},
	})
	h.completeLogin(r, id, "admin", email)

	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
	return nil
}
