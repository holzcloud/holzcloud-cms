package admin

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	usr "github.com/holzcloud/holzcloud-cms/internal/user"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// UserRow represents a row from the users table.
type UserRow struct {
	ID        int64
	Name      string
	Email     string
	Role      string
	Password  string
	CreatedAt string
	// LastLogin is empty for an account that has never been used, which is what
	// makes a forgotten colleague's account visible in the list.
	LastLogin string
	// MayPublish and SiteCount are the limits, for the list. SiteCount of zero
	// means every website — see user.Rights.
	MayPublish bool
	SiteCount  int
	// SiteTotal is how many websites there are, so "2 von 5" is readable
	// without counting.
	SiteTotal int
}

// UserListData extends LayoutData for the user list page.
type UserListData struct {
	web.LayoutData
	Users         []UserRow
	SessionUserID int64
}

// UserFormData extends LayoutData for the user create/edit form.
type UserFormData struct {
	web.LayoutData
	User   *UserRow
	IsEdit bool
	// MayPublish is the tick for the publishing right.
	MayPublish bool
	// Sites are the websites with a tick each, so the form shows the whole
	// answer and what comes back is complete.
	Sites []SiteTick
}

// SiteTick is one website in the assignment list.
type SiteTick struct {
	ID    int64
	Name  string
	Ticks bool
}

// UserPasswordData extends LayoutData for the password change form.
type UserPasswordData struct {
	web.LayoutData
	User   *UserRow
	IsSelf bool
}

// --- Data access helpers ---

func (h *Handler) listUsers(ctx context.Context) ([]UserRow, error) {
	rows, err := h.db.Read.QueryContext(ctx,
		`SELECT id, name, email, role, created_at, COALESCE(last_login_at, '') FROM users ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserRow
	for rows.Next() {
		var u UserRow
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &u.LastLogin); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (h *Handler) getUserByID(ctx context.Context, id int64) (*UserRow, error) {
	var u UserRow
	err := h.db.Read.QueryRowContext(ctx,
		`SELECT id, name, email, role, password FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Password)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// createUser inserts a user and returns the new id, which the caller needs to
// store the rights against.
func (h *Handler) createUser(ctx context.Context, name, email, hash, role string) (int64, error) {
	res, err := h.db.Write.ExecContext(ctx,
		`INSERT INTO users (name, email, password, role) VALUES ($1, $2, $3, $4)`,
		name, email, hash, role)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (h *Handler) updateUser(ctx context.Context, id int64, name, email, role string) error {
	_, err := h.db.Write.ExecContext(ctx,
		`UPDATE users SET name = $1, email = $2, role = $3 WHERE id = $4`,
		name, email, role, id)
	return err
}

func (h *Handler) updatePassword(ctx context.Context, id int64, hash string) error {
	_, err := h.db.Write.ExecContext(ctx,
		`UPDATE users SET password = $1 WHERE id = $2`, hash, id)
	return err
}

func (h *Handler) deleteUser(ctx context.Context, id int64) error {
	_, err := h.db.Write.ExecContext(ctx,
		`DELETE FROM users WHERE id = $1`, id)
	return err
}

func (h *Handler) countAdmins(ctx context.Context) (int, error) {
	var count int
	err := h.db.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count)
	return count, err
}

// --- Handlers ---

// HandleUserList renders the user list page.
func (h *Handler) HandleUserList(w http.ResponseWriter, r *http.Request) error {
	users, err := h.listUsers(r.Context())
	if err != nil {
		return err
	}
	// Die Grenzen je Person. Eine Abfrage je Zeile, und die Liste ist so lang
	// wie die Redaktion — bei zwanzig Leuten sind das zwanzig sehr kurze.
	total := 0
	if all, err := h.domains.ListWebsites(r.Context()); err == nil {
		total = len(all)
	}
	for i := range users {
		rights, err := h.users.Rights(r.Context(), users[i].ID)
		if err != nil {
			continue
		}
		users[i].MayPublish = rights.MayPublish
		users[i].SiteCount = len(rights.Websites)
		users[i].SiteTotal = total
	}

	data := UserListData{
		LayoutData:    web.NewLayoutData(r, h.sm, "Benutzer"),
		Users:         users,
		SessionUserID: h.sm.GetInt64(r.Context(), auth.SessionKeyUserID),
	}
	data.ActiveNav = "users"
	return web.RenderAdmin(w, h.templates, r, "user_list", data)
}

// HandleUserCreate handles GET (form) and POST (submit) for creating a user.
func (h *Handler) HandleUserCreate(w http.ResponseWriter, r *http.Request) error {
	if r.Method == http.MethodPost {
		return h.handleUserCreatePost(w, r)
	}

	data := UserFormData{
		LayoutData: web.NewLayoutData(r, h.sm, "Neuer Benutzer"),
		MayPublish: true,
		Sites:      h.siteTicks(r, usr.Everything()),
	}
	data.ActiveNav = "users"
	return web.RenderAdmin(w, h.templates, r, "user_form", data)
}

func (h *Handler) handleUserCreatePost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	role := r.FormValue("role")

	// Validate
	if email == "" {
		web.SetFlashError(h.sm, r.Context(), "Bitte eine E-Mail-Adresse angeben")
		return h.redirectBack(w, r, "/admin/users/new")
	}
	if len(password) < auth.MinPasswordLength {
		web.SetFlashError(h.sm, r.Context(),
			fmt.Sprintf("Password must be at least %d characters", auth.MinPasswordLength))
		return h.redirectBack(w, r, "/admin/users/new")
	}
	if role != "admin" && role != "editor" {
		web.SetFlashError(h.sm, r.Context(), "Die Rolle muss Administrator oder Redakteur sein")
		return h.redirectBack(w, r, "/admin/users/new")
	}

	hash, err := auth.HashPassword(password, h.argon2Params)
	if err != nil {
		return err
	}

	newID, err := h.createUser(r.Context(), name, email, hash, role)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			web.SetFlashError(h.sm, r.Context(), "Ein Benutzer mit dieser E-Mail-Adresse existiert bereits")
			return h.redirectBack(w, r, "/admin/users/new")
		}
		return err
	}
	if err := h.users.SetRights(r.Context(), newID, rightsFromForm(r, role)); err != nil {
		return err
	}

	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionUserCreate,
		EntityType: "user",
		EntityID:   newID,
		Metadata:   map[string]any{"email": email, "rolle": role},
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Benutzer angelegt")
	return h.redirectBack(w, r, "/admin/users")
}

// HandleUserEdit handles GET (form) and POST (submit) for editing a user.
func (h *Handler) HandleUserEdit(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	if r.Method == http.MethodPost {
		return h.handleUserEditPost(w, r, id)
	}

	user, err := h.getUserByID(r.Context(), id)
	if err != nil {
		return err
	}
	if user == nil {
		http.NotFound(w, r)
		return nil
	}

	rights, err := h.users.Rights(r.Context(), id)
	if err != nil {
		return err
	}
	data := UserFormData{
		LayoutData: web.NewLayoutData(r, h.sm, "Benutzer bearbeiten"),
		User:       user,
		IsEdit:     true,
		MayPublish: rights.MayPublish,
		Sites:      h.siteTicks(r, rights),
	}
	data.ActiveNav = "users"
	return web.RenderAdmin(w, h.templates, r, "user_form", data)
}

func (h *Handler) handleUserEditPost(w http.ResponseWriter, r *http.Request, id int64) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	role := r.FormValue("role")
	redirect := fmt.Sprintf("/admin/users/%d/edit", id)

	if email == "" {
		web.SetFlashError(h.sm, r.Context(), "Bitte eine E-Mail-Adresse angeben")
		return h.redirectBack(w, r, redirect)
	}
	if role != "admin" && role != "editor" {
		web.SetFlashError(h.sm, r.Context(), "Die Rolle muss Administrator oder Redakteur sein")
		return h.redirectBack(w, r, redirect)
	}

	// Safety: cannot demote last admin
	user, err := h.getUserByID(r.Context(), id)
	if err != nil {
		return err
	}
	if user == nil {
		http.NotFound(w, r)
		return nil
	}

	if user.Role == "admin" && role != "admin" {
		count, err := h.countAdmins(r.Context())
		if err != nil {
			return err
		}
		if count <= 1 {
			web.SetFlashError(h.sm, r.Context(), "Dem letzten Administrator kann die Rolle nicht entzogen werden")
			return h.redirectBack(w, r, redirect)
		}
	}

	// Die Rechte kommen aus demselben Formular — siehe rightsFromForm.
	if err := h.users.SetRights(r.Context(), id, rightsFromForm(r, role)); err != nil {
		return err
	}

	if err := h.updateUser(r.Context(), id, name, email, role); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			web.SetFlashError(h.sm, r.Context(), "Ein Benutzer mit dieser E-Mail-Adresse existiert bereits")
			return h.redirectBack(w, r, redirect)
		}
		return err
	}

	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionUserUpdate,
		EntityType: "user",
		EntityID:   id,
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Benutzer gespeichert")
	return h.redirectBack(w, r, "/admin/users")
}

// HandleUserDelete deletes a user (POST only).
func (h *Handler) HandleUserDelete(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	sessionUserID := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)

	// Safety: cannot delete self
	if id == sessionUserID {
		web.SetFlashError(h.sm, r.Context(), "Du kannst dein eigenes Konto nicht löschen")
		return h.redirectBack(w, r, "/admin/users")
	}

	// Safety: cannot delete last admin
	user, err := h.getUserByID(r.Context(), id)
	if err != nil {
		return err
	}
	if user == nil {
		http.NotFound(w, r)
		return nil
	}
	if user.Role == "admin" {
		count, err := h.countAdmins(r.Context())
		if err != nil {
			return err
		}
		if count <= 1 {
			web.SetFlashError(h.sm, r.Context(), "Der letzte Administrator kann nicht gelöscht werden")
			return h.redirectBack(w, r, "/admin/users")
		}
	}

	if err := h.deleteUser(r.Context(), id); err != nil {
		return err
	}

	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionUserDelete,
		EntityType: "user",
		EntityID:   id,
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Benutzer gelöscht")
	return h.redirectBack(w, r, "/admin/users")
}

// HandlePasswordChange handles GET (form) and POST (submit) for changing a user's password.
func (h *Handler) HandlePasswordChange(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	user, err := h.getUserByID(r.Context(), id)
	if err != nil {
		return err
	}
	if user == nil {
		http.NotFound(w, r)
		return nil
	}

	sessionUserID := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	isSelf := id == sessionUserID

	// Non-admin users can only change their own password
	sessionRole := h.sm.GetString(r.Context(), auth.SessionKeyUserRole)
	if !isSelf && sessionRole != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil
	}

	if r.Method == http.MethodPost {
		return h.handlePasswordChangePost(w, r, user, isSelf)
	}

	data := UserPasswordData{
		LayoutData: web.NewLayoutData(r, h.sm, "Passwort ändern"),
		User:       user,
		IsSelf:     isSelf,
	}
	data.ActiveNav = "users"
	return web.RenderAdmin(w, h.templates, r, "user_password", data)
}

func (h *Handler) handlePasswordChangePost(w http.ResponseWriter, r *http.Request, user *UserRow, isSelf bool) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	redirect := fmt.Sprintf("/admin/users/%d/password", user.ID)

	// Self-change requires current password
	if isSelf {
		currentPassword := r.FormValue("current_password")
		ok, err := auth.VerifyPassword(currentPassword, user.Password)
		if err != nil {
			return err
		}
		if !ok {
			web.SetFlashError(h.sm, r.Context(), "Das aktuelle Passwort stimmt nicht")
			return h.redirectBack(w, r, redirect)
		}
	}

	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if len(newPassword) < auth.MinPasswordLength {
		web.SetFlashError(h.sm, r.Context(),
			fmt.Sprintf("New password must be at least %d characters", auth.MinPasswordLength))
		return h.redirectBack(w, r, redirect)
	}
	if newPassword != confirmPassword {
		web.SetFlashError(h.sm, r.Context(), "Die Passwörter stimmen nicht überein")
		return h.redirectBack(w, r, redirect)
	}

	hash, err := auth.HashPassword(newPassword, h.argon2Params)
	if err != nil {
		return err
	}

	if err := h.updatePassword(r.Context(), user.ID, hash); err != nil {
		return err
	}

	// End the user's other sessions: a password change is how someone locks out
	// an attacker who already holds a session cookie. The current session is
	// kept when a user changes their own password, so they stay signed in.
	var keep string
	if isSelf {
		keep = h.sm.Token(r.Context())
	}
	if err := auth.DestroyUserSessions(r.Context(), h.sm, user.ID, keep); err != nil {
		// The password itself is already changed; log and carry on rather than
		// showing a failure for a change that did take effect.
		slog.Error("could not end other sessions after password change", "err", err, "user_id", user.ID)
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Passwort geändert")
	return h.redirectBack(w, r, "/admin/users")
}

// redirectBack sends an HX-Redirect for htmx or a 303 redirect for standard requests.
func (h *Handler) redirectBack(w http.ResponseWriter, r *http.Request, url string) error {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", url)
		return nil
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
	return nil
}
