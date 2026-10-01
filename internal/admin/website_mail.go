package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// A website's own mail account, on a screen of its own in the website's
// settings.
//
// Everything a website sends — the enquiry notifications, the copy to the
// sender, the shop's order mails — goes through this account once there is one,
// and through the installation's account while there is not. Invitations and
// password links belong to no website and always use the installation's.

// websiteMailData backs the screen.
type websiteMailData struct {
	web.LayoutData
	web.FormState
	Website *domain.Website
	// Account is what is stored, or nil. Values is what the form shows: the
	// stored account, or what was just typed when it did not validate.
	Account *mail.Account
	Values  mail.Account
	// CanStorePasswords is false without HOLZCLOUD_SECRET_KEY: the form then
	// says so instead of offering a password field that cannot be saved.
	CanStorePasswords bool
	// Fallback is the installation's sender address, or empty.
	Fallback string
	OwnEmail string
	// Pending, Failed and LastError are this website's part of the outbox.
	Pending   int
	Failed    int
	LastError string
}

func (h *Handler) websiteMailSite(w http.ResponseWriter, r *http.Request) (*domain.Website, bool, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || h.mail.Accounts() == nil {
		http.NotFound(w, r)
		return nil, false, nil
	}
	ws, err := h.domains.GetWebsite(r.Context(), id)
	if err != nil {
		return nil, false, err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil, false, nil
	}
	return ws, true, nil
}

func websiteMailPath(ws *domain.Website) string {
	return "/admin/websites/" + strconv.FormatInt(ws.ID, 10) + "/versand"
}

// HandleWebsiteMail shows and stores a website's own mail account.
func (h *Handler) HandleWebsiteMail(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.websiteMailSite(w, r)
	if err != nil || !ok {
		return err
	}
	accounts := h.mail.Accounts()
	stored, err := accounts.Get(r.Context(), ws.ID)
	if err != nil {
		return err
	}
	if r.Method != http.MethodPost {
		values := mail.Account{Port: 587, TLS: "starttls", FromName: ws.Name}
		if stored != nil {
			values = *stored
		}
		return h.renderWebsiteMail(w, r, ws, stored, values, web.NewFormState())
	}

	if err := r.ParseForm(); err != nil {
		return err
	}
	port, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("port")))
	values := mail.Account{
		WebsiteID: ws.ID,
		Host:      r.FormValue("host"),
		Port:      port,
		User:      r.FormValue("username"),
		From:      r.FormValue("from"),
		FromName:  r.FormValue("from_name"),
		TLS:       r.FormValue("tls"),
	}
	password := r.FormValue("password")
	// An empty field keeps what is stored, the way every password form does;
	// the box beside it is how a stored password is taken away.
	keep := password == "" && stored != nil && r.FormValue("clear_password") == ""
	if stored != nil {
		values.HasPassword = stored.HasPassword
	}

	state := web.NewFormState()
	err = accounts.Save(r.Context(), values, password, keep)
	switch {
	case err == nil:
	case errors.Is(err, mail.ErrAccountHost):
		state.Errors.Add("host", "Enter the name of the mail server, without a port or a protocol — for example: mail.example.com")
	case errors.Is(err, mail.ErrAccountFrom):
		state.Errors.Add("from", "That does not look like an email address.")
	case errors.Is(err, mail.ErrAccountPort):
		state.Errors.Add("port", "The port must lie between 1 and 65535.")
	case errors.Is(err, mail.ErrAccountTLS):
		state.Errors.Add("tls", "Please choose one of the offered kinds of encryption.")
	case errors.Is(err, mail.ErrNoSecretKey):
		state.Errors.Add("password", "A password can only be stored once HOLZCLOUD_SECRET_KEY is set.")
	default:
		return err
	}
	if state.Errors.Any() {
		return h.renderWebsiteMail(w, r, ws, stored, values, state)
	}

	siteID := ws.ID
	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionWebsiteUpdate,
		EntityType: "website",
		EntityID:   ws.ID,
		WebsiteID:  &siteID,
		// The server and the sender, never the password: the log is read by
		// more people than the settings.
		Metadata: map[string]any{"mail_account": "saved", "host": values.Host, "from": values.From},
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Mail account saved. Everything this website sends now goes out through it.")
	return h.redirect(w, r, websiteMailPath(ws))
}

func (h *Handler) renderWebsiteMail(w http.ResponseWriter, r *http.Request, ws *domain.Website,
	stored *mail.Account, values mail.Account, state web.FormState) error {

	accounts := h.mail.Accounts()
	data := websiteMailData{
		LayoutData:        web.NewLayoutData(r, h.sm, web.Titlef(r, "Sending – %s", ws.Name)),
		FormState:         state,
		Website:           ws,
		Account:           stored,
		Values:            values,
		CanStorePasswords: accounts.CanStorePasswords(),
		OwnEmail:          h.sm.GetString(r.Context(), auth.SessionKeyUserEmail),
	}
	if fb := accounts.Fallback(); fb.Enabled() {
		data.Fallback = fb.Config().From
	}
	st, err := h.mail.SiteStatus(r.Context(), ws.ID)
	if err != nil {
		return err
	}
	data.Pending, data.Failed, data.LastError = st.Pending, st.Failed, st.LastError
	data.ActiveNav = "website-mail"
	data.CurrentWebsite = ws
	if state.Errors.Any() {
		return web.RenderFormError(w, h.templates, r, "website_mail", data)
	}
	return web.RenderAdmin(w, h.templates, r, "website_mail", data)
}

// HandleWebsiteMailTest queues a message to the person asking, through this
// website's account.
func (h *Handler) HandleWebsiteMailTest(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.websiteMailSite(w, r)
	if err != nil || !ok {
		return err
	}
	to := h.sm.GetString(r.Context(), auth.SessionKeyUserEmail)
	if to == "" {
		web.SetFlashError(h.sm, r.Context(), "No address is on file for your account.")
		return h.redirect(w, r, websiteMailPath(ws))
	}
	if !h.mail.EnabledFor(r.Context(), ws.ID) {
		web.SetFlashError(h.sm, r.Context(), "This website has no mail account, and the installation has none either.")
		return h.redirect(w, r, websiteMailPath(ws))
	}
	msg := testMail(i18n.Lang(r.Context()), to)
	if err := h.mail.Enqueue(r.Context(), ws.ID, msg); err != nil {
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Queueing failed: %s", err))
		return h.redirect(w, r, websiteMailPath(ws))
	}
	web.SetFlashSuccess(h.sm, r.Context(), web.Titlef(r, "Test message to %s queued. It goes out in the next few seconds.", to))
	return h.redirect(w, r, websiteMailPath(ws))
}

// HandleWebsiteMailDelete removes the website's own account; it sends through
// the installation's again.
func (h *Handler) HandleWebsiteMailDelete(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.websiteMailSite(w, r)
	if err != nil || !ok {
		return err
	}
	if err := h.mail.Accounts().Delete(r.Context(), ws.ID); err != nil {
		return err
	}
	siteID := ws.ID
	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionWebsiteUpdate,
		EntityType: "website",
		EntityID:   ws.ID,
		WebsiteID:  &siteID,
		Metadata:   map[string]any{"mail_account": "removed"},
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Own mail account removed. This website sends through the installation's account again.")
	return h.redirect(w, r, websiteMailPath(ws))
}
