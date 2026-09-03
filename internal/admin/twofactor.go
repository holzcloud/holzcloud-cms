package admin

import (
	"context"
	"database/sql"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/totp"
	"github.com/holzcloud/holzcloud-cms/internal/user"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// TwoFactorVerifyData is the code prompt shown between password and admin.
type TwoFactorVerifyData struct {
	web.LayoutData
	// Recovery switches the form to the printed-code field.
	Recovery bool
}

// TwoFactorSetupData is the enrolment screen.
type TwoFactorSetupData struct {
	web.LayoutData
	// SecretGrouped is the shared secret in blocks of four, for typing it in
	// when the camera will not cooperate.
	SecretGrouped string
	URI           string
	// QR is the inline SVG the app's camera reads.
	QR template.HTML
	// Required says the account cannot skip this.
	Required bool
	Error    string
}

// TwoFactorCodesData shows the recovery codes exactly once.
type TwoFactorCodesData struct {
	web.LayoutData
	Codes []string
	// Fresh distinguishes "you just switched it on" from "you asked for a new
	// list", which need different words.
	Fresh bool
}

// TwoFactorStatusData is the account screen's section.
type TwoFactorStatusData struct {
	Enabled      bool
	Required     bool
	RecoveryLeft int
}

// HandleTwoFactorVerify asks a half-authenticated session for its code.
func (h *Handler) HandleTwoFactorVerify(w http.ResponseWriter, r *http.Request) error {
	pending := h.sm.GetInt64(r.Context(), auth.SessionKeyPendingUserID)
	if pending == 0 {
		// Nothing to verify: either already signed in or never started.
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}

	recovery := r.URL.Query().Get("wiederherstellung") != ""
	if r.Method != http.MethodPost {
		data := TwoFactorVerifyData{
			LayoutData: web.NewLayoutData(r, h.sm, "Bestätigung"),
			Recovery:   recovery,
		}
		return web.RenderAdmin(w, h.templates, r, "two_factor_verify", data)
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	// The same throttle as the password: six digits is a small enough space
	// that unlimited guessing would walk through it in an afternoon.
	ip := h.clientIP.ClientIP(r)
	accountKey := "2fa:" + strings.ToLower(h.emailFor(r, pending))
	if !h.loginThrottle.Allowed(ip, accountKey) {
		web.SetFlashError(h.sm, r.Context(),
			"Zu viele Fehlversuche. Bitte warte einen Moment.")
		http.Redirect(w, r, auth.VerifyPath, http.StatusSeeOther)
		return nil
	}

	submitted := strings.TrimSpace(r.FormValue("code"))
	var err error
	if recovery {
		err = h.users.UseRecoveryCode(r.Context(), pending, submitted)
	} else {
		err = h.users.VerifyCode(r.Context(), pending, submitted)
	}
	if err != nil {
		if !errors.Is(err, user.ErrBadCode) && !errors.Is(err, user.ErrNoSecondFactor) {
			return err
		}
		h.loginThrottle.RecordFailure(ip, accountKey)
		web.SetFlashError(h.sm, r.Context(), "Der Code stimmt nicht.")
		http.Redirect(w, r, verifyPathFor(recovery), http.StatusSeeOther)
		return nil
	}

	h.loginThrottle.Reset(ip, accountKey)

	role, email, err := h.roleAndEmail(r, pending)
	if err != nil {
		return err
	}
	// A second rotation: the session that carried the pending id must not be
	// the session that ends up signed in.
	if err := h.sm.RenewToken(r.Context()); err != nil {
		return err
	}
	h.completeLogin(r, pending, role, email)

	if recovery {
		// Using one up is worth saying out loud — the whole point is that the
		// list runs out.
		web.SetFlashSuccess(h.sm, r.Context(),
			"Wiederherstellungscode verbraucht. Prüfe unter „Mein Konto“, wie viele noch übrig sind.")
	}
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
	return nil
}

func verifyPathFor(recovery bool) string {
	if recovery {
		return auth.VerifyPath + "?wiederherstellung=1"
	}
	return auth.VerifyPath
}

// HandleTwoFactorSetup enrols an authenticator.
func (h *Handler) HandleTwoFactorSetup(w http.ResponseWriter, r *http.Request) error {
	userID := h.currentUserID(r)
	if userID == nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}
	role, email, err := h.roleAndEmail(r, *userID)
	if err != nil {
		return err
	}

	if r.Method == http.MethodPost {
		return h.confirmTwoFactor(w, r, *userID, role, email)
	}

	// The same secret across reloads and mistyped codes. A new one per visit
	// would invalidate the QR code that was already scanned and then reject the
	// app's correct answer, with nothing on screen to say why.
	secret, err := h.users.EnsurePendingSecret(r.Context(), *userID)
	if err != nil {
		return err
	}
	uri := totp.URI(secret, email, h.issuerName(r))

	data := TwoFactorSetupData{
		LayoutData:    web.NewLayoutData(r, h.sm, "Zwei-Faktor einrichten"),
		SecretGrouped: totp.FormatSecret(secret),
		URI:           uri,
		QR:            qrOrNothing(uri),
		Required:      auth.MustHaveSecondFactor(role),
	}
	data.ActiveNav = "account"
	return web.RenderAdmin(w, h.templates, r, "two_factor_setup", data)
}

// confirmTwoFactor checks the first code and shows the recovery codes.
func (h *Handler) confirmTwoFactor(w http.ResponseWriter, r *http.Request, userID int64, role, email string) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	codes, err := h.users.ConfirmTwoFactor(r.Context(), userID, r.FormValue("code"))
	if err != nil {
		if !errors.Is(err, user.ErrBadCode) && !errors.Is(err, user.ErrNoSecondFactor) {
			return err
		}
		// The secret has to be shown again with the error, or the only way
		// back is to start over and rescan.
		tf, lookupErr := h.users.GetTwoFactor(r.Context(), userID)
		if lookupErr != nil || tf == nil || tf.PendingSecret == "" {
			web.SetFlashError(h.sm, r.Context(), "Die Einrichtung ist abgelaufen. Bitte noch einmal beginnen.")
			return h.redirect(w, r, auth.SetupPath)
		}
		retryURI := totp.URI(tf.PendingSecret, email, h.issuerName(r))
		data := TwoFactorSetupData{
			LayoutData:    web.NewLayoutData(r, h.sm, "Zwei-Faktor einrichten"),
			SecretGrouped: totp.FormatSecret(tf.PendingSecret),
			URI:           retryURI,
			QR:            qrOrNothing(retryURI),
			Required:      auth.MustHaveSecondFactor(role),
			Error:         "Der Code stimmt nicht. Prüfe, ob die Uhr des Geräts richtig geht.",
		}
		data.ActiveNav = "account"
		return web.RenderFormError(w, h.templates, r, "two_factor_setup", data)
	}

	slog.Info("two factor enabled", "user_id", userID)
	return h.showRecoveryCodes(w, r, codes, true)
}

// HandleTwoFactorRestart begins enrolling a new device.
//
// A POST rather than a link, because it writes: the pending secret is replaced,
// and a GET that changes state is one a browser may repeat on its own.
func (h *Handler) HandleTwoFactorRestart(w http.ResponseWriter, r *http.Request) error {
	userID := h.currentUserID(r)
	if userID == nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}
	// This is the one path that throws the old enrolment away, so a device that
	// was set up half-way — or a QR code shown on a screen someone else can
	// still see — can be replaced deliberately rather than by accident.
	if _, err := h.users.BeginTwoFactor(r.Context(), *userID); err != nil {
		return err
	}
	return h.redirect(w, r, auth.SetupPath)
}

// HandleRecoveryCodes issues a fresh list.
func (h *Handler) HandleRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
	userID := h.currentUserID(r)
	if userID == nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}
	tf, err := h.users.GetTwoFactor(r.Context(), *userID)
	if err != nil {
		return err
	}
	if tf == nil || !tf.Enabled() {
		return h.redirect(w, r, auth.SetupPath)
	}

	codes, err := h.users.RegenerateRecoveryCodes(r.Context(), *userID)
	if err != nil {
		return err
	}
	return h.showRecoveryCodes(w, r, codes, false)
}

func (h *Handler) showRecoveryCodes(w http.ResponseWriter, r *http.Request, codes []string, fresh bool) error {
	data := TwoFactorCodesData{
		LayoutData: web.NewLayoutData(r, h.sm, "Wiederherstellungscodes"),
		Codes:      codes,
		Fresh:      fresh,
	}
	data.ActiveNav = "account"
	return web.RenderAdmin(w, h.templates, r, "two_factor_codes", data)
}

// HandleTwoFactorDisable switches the second factor off.
//
// Refused for an account that is required to have one: turning it off from
// inside a signed-in session would make the requirement decorative, since a
// stolen session could simply switch it off and stay.
func (h *Handler) HandleTwoFactorDisable(w http.ResponseWriter, r *http.Request) error {
	userID := h.currentUserID(r)
	if userID == nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}
	role, _, err := h.roleAndEmail(r, *userID)
	if err != nil {
		return err
	}
	if auth.MustHaveSecondFactor(role) {
		web.SetFlashError(h.sm, r.Context(),
			"Für Administratoren ist die Bestätigung in zwei Schritten Pflicht. "+
				"Wenn das Gerät verloren ist, hilft ein Wiederherstellungscode oder "+
				"„holzcloud user 2fa disable“ auf dem Server.")
		return h.redirect(w, r, "/admin/konto")
	}

	if err := h.users.DisableTwoFactor(r.Context(), *userID); err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Bestätigung in zwei Schritten ausgeschaltet")
	return h.redirect(w, r, "/admin/konto")
}

// qrOrNothing renders the QR code, falling back to no image.
//
// A failure here must not block the setup: the secret is printed beside it and
// can be typed in by hand, which is the same path someone with a broken camera
// takes anyway.
func qrOrNothing(uri string) template.HTML {
	svg, err := totp.QRCode(uri)
	if err != nil {
		slog.Error("render totp qr code", "err", err)
		return ""
	}
	return svg
}

// roleAndEmail reads the two account fields the second-factor screens need.
func (h *Handler) roleAndEmail(r *http.Request, userID int64) (role, email string, err error) {
	err = h.db.Read.QueryRowContext(r.Context(),
		`SELECT role, email FROM users WHERE id = $1`, userID).Scan(&role, &email)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return role, email, err
}

// emailFor is roleAndEmail when only the address is wanted.
func (h *Handler) emailFor(r *http.Request, userID int64) string {
	_, email, err := h.roleAndEmail(r, userID)
	if err != nil {
		return ""
	}
	return email
}

// issuerName is what the authenticator app lists the account under.
//
// The host, so someone administering three Holzcloud sites sees three distinct
// entries rather than three lines all reading "Holzcloud".
func (h *Handler) issuerName(r *http.Request) string {
	host := r.Host
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return "Holzcloud"
	}
	return "Holzcloud (" + host + ")"
}

// NewSecondFactorLookup adapts the users table to the enforcement middleware.
//
// One query rather than two calls into the store: this runs on every admin
// request, and the middleware only needs to know the role and whether the
// account has confirmed an authenticator.
func NewSecondFactorLookup(database *db.DB) auth.SecondFactorLookup {
	return func(ctx context.Context, id int64) (auth.SecondFactorState, error) {
		var state auth.SecondFactorState
		var confirmed sql.NullString
		err := database.Read.QueryRowContext(ctx,
			`SELECT role, totp_confirmed_at FROM users WHERE id = $1`, id).
			Scan(&state.Role, &confirmed)
		if err == sql.ErrNoRows {
			return state, nil
		}
		if err != nil {
			return state, err
		}
		state.Enabled = confirmed.Valid
		return state, nil
	}
}

// AccountData is the "my account" screen.
type AccountData struct {
	web.LayoutData
	UserID int64
	Email  string
	Role   string
	TwoFactorStatusData
	// Languages are the languages this build can show the administration in.
	// It is a per-person setting: on the same website a German editor and an
	// English-speaking developer each get their own.
	Languages []i18n.Language
	// Chosen is the stored tag, empty when nobody has chosen and the browser
	// decides.
	Chosen string
}

// HandleAccount shows the signed-in account and its second factor.
//
// It exists because two-factor needs a home: a screen that says whether it is
// on, how many recovery codes are left, and where to go when the answer to
// either is wrong.
func (h *Handler) HandleAccount(w http.ResponseWriter, r *http.Request) error {
	userID := h.currentUserID(r)
	if userID == nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}
	role, email, err := h.roleAndEmail(r, *userID)
	if err != nil {
		return err
	}
	tf, err := h.users.GetTwoFactor(r.Context(), *userID)
	if err != nil {
		return err
	}

	data := AccountData{
		LayoutData: web.NewLayoutData(r, h.sm, "Mein Konto"),
		UserID:     *userID,
		Email:      email,
		Role:       role,
		TwoFactorStatusData: TwoFactorStatusData{
			Required: auth.MustHaveSecondFactor(role),
		},
	}
	if tf != nil {
		data.Enabled = tf.Enabled()
		data.RecoveryLeft = tf.RecoveryLeft
	}
	data.Languages = i18n.Languages()
	if u, err := h.users.GetByID(r.Context(), *userID); err == nil && u != nil {
		data.Chosen = u.Locale
	}
	data.ActiveNav = "account"
	return web.RenderAdmin(w, h.templates, r, "account", data)
}

// HandleAccountLanguage stores the language this person reads the
// administration in.
//
// The empty value is kept rather than rejected: "let my browser decide" is a
// real answer, and it is the right one for somebody who works on two machines
// set up in different languages.
func (h *Handler) HandleAccountLanguage(w http.ResponseWriter, r *http.Request) error {
	userID := h.currentUserID(r)
	if userID == nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	chosen := i18n.Normalise(r.FormValue("sprache"))
	if chosen != "" && !i18n.Known(chosen) {
		web.SetFlashError(h.sm, r.Context(), "Diese Sprache gibt es in dieser Fassung nicht")
		http.Redirect(w, r, "/admin/konto", http.StatusSeeOther)
		return nil
	}
	if err := h.users.SetLocale(r.Context(), *userID, chosen); err != nil {
		return err
	}

	// The confirmation is written in the language just chosen, not the one the
	// page was rendered in: it is read on the next screen, which will already
	// be in the new language.
	web.SetFlashSuccess(h.sm, i18n.WithLang(r.Context(), langOrBrowser(chosen, r)), "Sprache gespeichert")
	http.Redirect(w, r, "/admin/konto", http.StatusSeeOther)
	return nil
}

// langOrBrowser resolves an empty choice the same way the middleware will on
// the next request.
func langOrBrowser(chosen string, r *http.Request) string {
	if chosen != "" {
		return chosen
	}
	return i18n.FromRequest(r)
}
