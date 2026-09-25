package admin

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/branding"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The installation, for the AI tools: accounts, plugins, mail, languages, the
// brand and the activity log. Each method is what one screen does, minus the
// request — see ops.go for why these exist at all.
//
// The errors here are read by an assistant and not by an operator, so they are
// plain English and not catalogue sentences.

// errNoSuchUser is an account id that does not exist.
var errNoSuchUser = errors.New("there is no such user")

// --- accounts ---------------------------------------------------------------

// OpUsers lists every account with its limits and its second factor.
func (h *Handler) OpUsers(ctx context.Context) ([]ai.AdminUser, error) {
	rows, err := h.listUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ai.AdminUser, 0, len(rows))
	for _, row := range rows {
		u, err := h.describeUser(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		if u != nil {
			out = append(out, *u)
		}
	}
	return out, nil
}

// OpUser returns one account.
func (h *Handler) OpUser(ctx context.Context, id int64) (*ai.AdminUser, error) {
	u, err := h.describeUser(ctx, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, errNoSuchUser
	}
	return u, nil
}

// describeUser gathers what the user list and the edit form show about one
// account. Nil for an id that does not exist.
func (h *Handler) describeUser(ctx context.Context, id int64) (*ai.AdminUser, error) {
	var out ai.AdminUser
	err := h.db.Read.QueryRowContext(ctx,
		`SELECT id, name, email, role, created_at, COALESCE(last_login_at, ''), locale
		 FROM users WHERE id = $1`, id).
		Scan(&out.ID, &out.Name, &out.Email, &out.Role, &out.CreatedAt, &out.LastLogin, &out.Locale)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	rights, err := h.users.Rights(ctx, id)
	if err != nil {
		return nil, err
	}
	out.MayPublish = rights.MayPublish
	out.AllWebsites = !rights.Limited()
	out.Websites = append([]int64{}, rights.Websites...)

	tf, err := h.users.GetTwoFactor(ctx, id)
	if err != nil {
		return nil, err
	}
	if tf != nil && tf.Enabled() {
		out.TwoFactor = true
		out.RecoveryCodesLeft = tf.RecoveryLeft
	}
	return &out, nil
}

// rightsFor turns what a tool was told into the rights stored for an account.
//
// The same reading as rightsFromForm: an administrator has no limits, a list of
// websites limits to those, and "not every website" with an empty list is the
// editor who may enter none.
func (h *Handler) rightsFor(ctx context.Context, role string, in ai.UserRights) (user.Rights, error) {
	if role == user.RoleAdmin {
		return user.Everything(), nil
	}
	if in.AllWebsites {
		return user.Rights{MayPublish: in.MayPublish}, nil
	}
	for _, id := range in.Websites {
		ws, err := h.domains.GetWebsite(ctx, id)
		if err != nil || ws == nil {
			return user.Rights{}, fmt.Errorf("there is no website %d", id)
		}
	}
	if len(in.Websites) == 0 {
		nothing := user.Nothing()
		nothing.MayPublish = in.MayPublish
		return nothing, nil
	}
	return user.Rights{MayPublish: in.MayPublish, Websites: in.Websites}, nil
}

// OpInviteUser creates an account and issues its invitation link, mailing it
// when mail is set up — what the screen does in two steps, creating the account
// and then pressing "Invite".
//
// The account gets a password nobody knows, the same way single sign-on
// provisions one: users.password cannot be empty, and the person sets their
// own through the link.
func (h *Handler) OpInviteUser(ctx context.Context, host, lang, name, email, role string, rights ai.UserRights) (*ai.AdminUser, ai.AccessLink, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, ai.AccessLink{}, errors.New("an email address is needed")
	}
	if !user.ValidRole(role) {
		return nil, ai.AccessLink{}, errors.New("the role must be admin or editor")
	}
	stored, err := h.rightsFor(ctx, role, rights)
	if err != nil {
		return nil, ai.AccessLink{}, err
	}
	secret, err := randomSecret()
	if err != nil {
		return nil, ai.AccessLink{}, err
	}
	id, err := h.users.Create(ctx, name, email, secret, role)
	if errors.Is(err, user.ErrDuplicateEmail) {
		return nil, ai.AccessLink{}, errors.New("a user with that email address already exists")
	}
	if err != nil {
		return nil, ai.AccessLink{}, err
	}
	if err := h.users.SetRights(ctx, id, stored); err != nil {
		return nil, ai.AccessLink{}, err
	}
	link, err := h.OpUserLink(ctx, host, lang, id, user.PurposeInvite)
	if err != nil {
		return nil, ai.AccessLink{}, err
	}
	u, err := h.OpUser(ctx, id)
	return u, link, err
}

// OpUpdateUser changes what is given and leaves the rest. The last
// administrator keeps the role, as on the screen.
func (h *Handler) OpUpdateUser(ctx context.Context, id int64, ch ai.UserChange) (*ai.AdminUser, error) {
	current, err := h.OpUser(ctx, id)
	if err != nil {
		return nil, err
	}
	name, email, role := current.Name, current.Email, current.Role
	if ch.Name != nil {
		name = strings.TrimSpace(*ch.Name)
	}
	if ch.Email != nil {
		email = strings.TrimSpace(*ch.Email)
	}
	if ch.Role != nil {
		role = *ch.Role
	}
	if email == "" {
		return nil, errors.New("an email address is needed")
	}
	if !user.ValidRole(role) {
		return nil, errors.New("the role must be admin or editor")
	}

	// The rights are worked out before anything is written, so a website that
	// does not exist refuses the whole change rather than half of it.
	var rights *user.Rights
	switch {
	case role == user.RoleAdmin && current.Role != user.RoleAdmin:
		all := user.Everything()
		rights = &all
	case ch.Rights != nil:
		r, err := h.rightsFor(ctx, role, *ch.Rights)
		if err != nil {
			return nil, err
		}
		rights = &r
	case ch.MayPublish != nil && role != user.RoleAdmin:
		// Only the publishing right: the website assignment stays as stored.
		r, err := h.users.Assignment(ctx, id)
		if err != nil {
			return nil, err
		}
		r.MayPublish = *ch.MayPublish
		rights = &r
	}

	if err := h.users.Update(ctx, id, name, email, role); err != nil {
		switch {
		case errors.Is(err, user.ErrLastAdmin):
			return nil, errors.New("the last administrator cannot have the role taken away")
		case errors.Is(err, user.ErrDuplicateEmail):
			return nil, errors.New("a user with that email address already exists")
		}
		return nil, err
	}
	if rights != nil {
		if err := h.users.SetRights(ctx, id, *rights); err != nil {
			return nil, err
		}
	}
	return h.OpUser(ctx, id)
}

// OpUserLink issues an invitation or password reset link, exactly as the
// button on the edit screen does: a reset ends the account's sessions, and the
// link is mailed when mail is set up. host is what the admin is reached at;
// lang the language of the mail, the account's own when empty.
func (h *Handler) OpUserLink(ctx context.Context, host, lang string, id int64, purpose string) (ai.AccessLink, error) {
	if purpose != user.PurposeInvite {
		purpose = user.PurposeReset
	}
	u, err := h.users.GetByID(ctx, id)
	if err != nil {
		return ai.AccessLink{}, err
	}
	if u == nil {
		return ai.AccessLink{}, errNoSuchUser
	}
	if strings.TrimSpace(host) == "" {
		return ai.AccessLink{}, errors.New("the address of the admin is not known")
	}
	if lang == "" {
		lang = u.Locale
	}
	link, err := h.issueAccessLink(ctx, host, lang, u, purpose)
	if err != nil {
		return ai.AccessLink{}, err
	}
	return ai.AccessLink{URL: link.URL, Expires: link.Expires, Mailed: link.Sent, MailError: link.SendError}, nil
}

// OpEndUserSessions signs an account out everywhere.
func (h *Handler) OpEndUserSessions(ctx context.Context, id int64) error {
	if _, err := h.OpUser(ctx, id); err != nil {
		return err
	}
	return auth.DestroyUserSessions(ctx, h.sm, id, "")
}

// OpDisableUserTwoFactor removes an account's second factor — what
// "holzcloud user 2fa disable" does on the server, for someone whose phone and
// recovery codes are both gone.
func (h *Handler) OpDisableUserTwoFactor(ctx context.Context, id int64) error {
	if _, err := h.OpUser(ctx, id); err != nil {
		return err
	}
	return h.users.DisableTwoFactor(ctx, id)
}

// OpDeleteUser deletes an account. The last administrator stays.
func (h *Handler) OpDeleteUser(ctx context.Context, id int64) error {
	err := h.users.Delete(ctx, id)
	switch {
	case errors.Is(err, user.ErrNotFound):
		return errNoSuchUser
	case errors.Is(err, user.ErrLastAdmin):
		return errors.New("the last administrator cannot be deleted")
	}
	return err
}

// --- plugins ----------------------------------------------------------------

// errPluginsOff is an installation whose plugin runtime did not start.
var errPluginsOff = errors.New("plugins are not available on this installation")

// OpPlugins lists what is installed and whether it runs.
func (h *Handler) OpPlugins(ctx context.Context) ([]plugin.Status, error) {
	if h.plugins == nil {
		return nil, errPluginsOff
	}
	return h.plugins.List(ctx)
}

// OpPluginInstall installs an archive, as the upload on the plugin screen
// does. The plugin stays switched off.
func (h *Handler) OpPluginInstall(ctx context.Context, archive []byte) (*plugin.Manifest, error) {
	if h.plugins == nil {
		return nil, errPluginsOff
	}
	if len(archive) > plugin.MaxTotalBytes {
		return nil, fmt.Errorf("the archive is larger than %d MB", plugin.MaxTotalBytes>>20)
	}
	return h.plugins.Install(ctx, bytes.NewReader(archive), int64(len(archive)))
}

// OpPluginSwitch switches a plugin on or off. Switching on reports why the
// module did not come up, when it did not.
func (h *Handler) OpPluginSwitch(ctx context.Context, id string, on bool) error {
	if h.plugins == nil {
		return errPluginsOff
	}
	var err error
	if on {
		err = h.plugins.Enable(ctx, id)
	} else {
		err = h.plugins.Disable(ctx, id)
	}
	if errors.Is(err, plugin.ErrNotFound) {
		return fmt.Errorf("there is no plugin %q", id)
	}
	return err
}

// OpPluginWebsites sets the websites a plugin acts on.
func (h *Handler) OpPluginWebsites(ctx context.Context, id string, sites []int64) error {
	if h.plugins == nil {
		return errPluginsOff
	}
	for _, siteID := range sites {
		if ws, err := h.domains.GetWebsite(ctx, siteID); err != nil || ws == nil {
			return fmt.Errorf("there is no website %d", siteID)
		}
	}
	err := h.plugins.SetWebsites(ctx, id, sites)
	if errors.Is(err, plugin.ErrNotFound) {
		return fmt.Errorf("there is no plugin %q", id)
	}
	return err
}

// OpPluginRemove deletes a plugin together with its stored data.
func (h *Handler) OpPluginRemove(ctx context.Context, id string) error {
	if h.plugins == nil {
		return errPluginsOff
	}
	err := h.plugins.Remove(ctx, id)
	if errors.Is(err, plugin.ErrNotFound) {
		return fmt.Errorf("there is no plugin %q", id)
	}
	return err
}

// --- mail -------------------------------------------------------------------

// OpMailStatus is what the mail screen shows.
func (h *Handler) OpMailStatus(ctx context.Context) (mail.Status, error) {
	return h.mail.Status(ctx)
}

// OpMailRetry puts everything given up on back in the queue.
func (h *Handler) OpMailRetry(ctx context.Context) (int64, error) {
	return h.mail.Retry(ctx)
}

// OpMailTest queues the test message.
//
// The screen sends it only to whoever presses the button, so that it cannot be
// used to send mail anywhere. A key has no address of its own, so here it may
// go to an account of this installation and to nothing else.
func (h *Handler) OpMailTest(ctx context.Context, lang, to string) error {
	if !h.mail.Enabled() {
		return errors.New("no mail server is set up: HOLZCLOUD_SMTP_HOST and HOLZCLOUD_SMTP_FROM are needed")
	}
	u, err := h.users.GetByEmail(ctx, to)
	if err != nil {
		return err
	}
	if u == nil {
		return errors.New("a test message goes only to the address of an account of this installation")
	}
	return h.mail.Enqueue(ctx, 0, testMail(lang, u.Email))
}

// --- languages --------------------------------------------------------------

// OpInstallLanguage installs a language file for the administration and returns
// how many translations it carries. The same checks as the upload.
func (h *Handler) OpInstallLanguage(ctx context.Context, code string, data []byte) (int, error) {
	return h.installLanguage(code, data)
}

// OpRemoveLanguage removes a language file. A built-in language is refused.
func (h *Handler) OpRemoveLanguage(ctx context.Context, code string) error {
	return h.removeLanguage(code)
}

// --- the brand --------------------------------------------------------------

// OpSetBrand stores the installation's name and the letter in its square.
func (h *Handler) OpSetBrand(ctx context.Context, name, mark string) error {
	return branding.Save(ctx, h.db.Write, name, mark)
}

// OpSetLogo checks and stores the picture. The file name's extension says what
// it claims to be, and the bytes have to agree.
func (h *Handler) OpSetLogo(ctx context.Context, filename string, data []byte) error {
	if reason := saveLogo(filename, data); reason != "" {
		return errors.New(reason)
	}
	branding.Load(ctx, h.db.Read)
	return nil
}

// OpRemoveLogo deletes the picture; the letter shows again.
func (h *Handler) OpRemoveLogo(ctx context.Context) error {
	if err := branding.RemoveLogo(); err != nil {
		return err
	}
	branding.Load(ctx, h.db.Read)
	return nil
}

// --- the activity log -------------------------------------------------------

// OpActivity reads the activity log, filtered as the screen filters it.
func (h *Handler) OpActivity(ctx context.Context, f activity.Filter, limit, offset int) ([]activity.Entry, int, error) {
	if h.activityStore == nil {
		return nil, 0, errors.New("the activity log is not kept on this installation")
	}
	return h.activityStore.List(ctx, f, limit, offset)
}
