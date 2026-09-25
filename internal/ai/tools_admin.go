package ai

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/branding"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// The installation: users, keys, plugins, languages, brand, activity log and
// mail. Every screen behind these is an administrator's (requireAdmin in
// main.go), so every tool that reads or changes them is Admin — an admin key
// exists only when somebody with shell access made one with
// "holzcloud ai key create -level admin". The one exception is key_info, which
// any key may call to find out what it is.
//
// Two decisions are made here rather than inherited from a screen:
//
//   - create_ai_key issues read and content keys only. An admin key through
//     this connection would let a key mint its own successor and outlive its
//     revocation; making one stays a command on the server, where the
//     authority to run the installation already lives.
//   - revoke_ai_key refuses the key that is calling it. A call that cuts its
//     own connection leaves the assistant unable to report what it did, and a
//     key that should end is ended by a person — on the AI screen or with
//     "holzcloud ai key revoke". Every other key, admin keys included, may be
//     revoked: taking access away is never the dangerous direction.

// AdminUser is an account as the installation tools show it.
type AdminUser struct {
	ID        int64
	Name      string
	Email     string
	Role      string
	CreatedAt string
	// LastLogin is empty for an account that has never been used.
	LastLogin string
	// Locale is the language the person reads the admin in; empty means their
	// browser decides.
	Locale     string
	MayPublish bool
	// AllWebsites is true for an account without a website limit. Otherwise
	// Websites are the ones it may enter — possibly none.
	AllWebsites bool
	Websites    []int64
	// TwoFactor says a second factor is set up, with RecoveryCodesLeft codes.
	TwoFactor         bool
	RecoveryCodesLeft int
}

// UserRights are an editor's limits as a tool gives them.
type UserRights struct {
	MayPublish  bool
	AllWebsites bool
	// Websites limit the account when AllWebsites is false. Empty then means
	// no website at all.
	Websites []int64
}

// UserChange is what update_user was told; nil leaves a value as it is.
type UserChange struct {
	Name, Email, Role *string
	// Rights replaces the website limit and the publishing right together.
	Rights *UserRights
	// MayPublish changes only the publishing right.
	MayPublish *bool
}

// AccessLink is a one-time link into the admin: an invitation or a reset.
type AccessLink struct {
	URL     string
	Expires time.Time
	// Mailed says the link was queued as mail; MailError why it could not be.
	Mailed    bool
	MailError string
}

// The admin handler's methods, a small interface per screen, so that a
// missing one takes only its own tools out.
type userOps interface {
	OpUsers(ctx context.Context) ([]AdminUser, error)
	OpUser(ctx context.Context, id int64) (*AdminUser, error)
	OpInviteUser(ctx context.Context, host, lang, name, email, role string, rights UserRights) (*AdminUser, AccessLink, error)
	OpUpdateUser(ctx context.Context, id int64, ch UserChange) (*AdminUser, error)
	OpUserLink(ctx context.Context, host, lang string, id int64, purpose string) (AccessLink, error)
	OpEndUserSessions(ctx context.Context, id int64) error
	OpDisableUserTwoFactor(ctx context.Context, id int64) error
	OpDeleteUser(ctx context.Context, id int64) error
}

type pluginOps interface {
	OpPlugins(ctx context.Context) ([]plugin.Status, error)
	OpPluginInstall(ctx context.Context, archive []byte) (*plugin.Manifest, error)
	OpPluginSwitch(ctx context.Context, id string, on bool) error
	OpPluginWebsites(ctx context.Context, id string, sites []int64) error
	OpPluginRemove(ctx context.Context, id string) error
}

type mailOps interface {
	OpMailStatus(ctx context.Context) (mail.Status, error)
	OpMailRetry(ctx context.Context) (int64, error)
	OpMailTest(ctx context.Context, lang, to string) error
}

type languageOps interface {
	OpInstallLanguage(ctx context.Context, code string, data []byte) (int, error)
	OpRemoveLanguage(ctx context.Context, code string) error
}

type brandOps interface {
	OpSetBrand(ctx context.Context, name, mark string) error
	OpSetLogo(ctx context.Context, filename string, data []byte) error
	OpRemoveLogo(ctx context.Context) error
}

type activityOps interface {
	OpActivity(ctx context.Context, f activity.Filter, limit, offset int) ([]activity.Entry, int, error)
}

// adminOpsAs asserts the admin handler to what a tool needs, or says the
// function is not there.
func adminOpsAs[T any](d Deps) (T, error) {
	ops, ok := d.Ops.(T)
	if !ok {
		var zero T
		return zero, errors.New("this function is not available on this installation")
	}
	return ops, nil
}

// errConfirm is the answer to a destructive call without confirm: true.
var errConfirm = errors.New("this cannot be undone; call again with confirm: true once the operator has agreed")

func adminTools(d Deps) []Tool {
	return []Tool{
		keyInfo(d),

		listUsers(d), getUser(d), inviteUser(d), updateUser(d),
		createPasswordResetLink(d), createInvitationLink(d),
		endUserSessions(d), disableUserTwoFactor(d), deleteUser(d),

		listAIKeys(d), createAIKey(d), revokeAIKey(d),

		listPlugins(d), installPlugin(d), enablePlugin(d), disablePlugin(d),
		setPluginWebsites(d), removePlugin(d),

		getMailStatus(d), retryMail(d), sendTestMail(d),

		listLanguages(d), installLanguage(d), removeLanguage(d),

		getBranding(d), updateBranding(d),

		readActivityLog(d),
	}
}

// --- the key itself -----------------------------------------------------------

func keyInfo(d Deps) Tool {
	return Tool{
		Name: "key_info",
		Description: "Says which access key this connection uses: its name, its level " +
			"(read, content or admin), which website it is limited to, when it expires, " +
			"and what that allows. A good first call to find out what can be done here.",
		InputSchema: Schema{Type: "object"},
		Run: func(c Call) (any, error) {
			level := LevelRead
			switch {
			case c.Scope.Admin:
				level = LevelAdmin
			case c.Scope.CanWrite:
				level = LevelContent
			}
			out := map[string]any{
				"id":      c.Scope.TokenID,
				"name":    c.Scope.Name,
				"level":   level.String(),
				"website": c.Scope.WebsiteID,
			}
			if c.Scope.WebsiteID == 0 {
				out["website_name"] = "every website"
			} else if d.Domains != nil {
				if ws, err := d.Domains.GetWebsite(c.Ctx, c.Scope.WebsiteID); err == nil && ws != nil {
					out["website_name"] = ws.Name
				}
			}
			if d.Tokens != nil {
				if t, err := d.Tokens.Get(c.Ctx, c.Scope.TokenID); err == nil {
					out["created_at"] = t.CreatedAt.Format(timeLayout)
					if t.ExpiresAt != nil {
						out["expires_at"] = t.ExpiresAt.Format(timeLayout)
					} else {
						out["expires_at"] = nil
					}
				}
			}
			may := []string{"read pages, media, menus and settings"}
			if c.Scope.CanWrite {
				may = append(may, "create and change content")
			}
			if c.Scope.Admin {
				may = append(may, "manage the installation: users, AI keys, plugins, mail, "+
					"languages, brand and the activity log")
			} else {
				may = append(may, "not manage the installation; that needs an admin key, "+
					"made on the server with: holzcloud ai key create -level admin")
			}
			out["may"] = may
			return out, nil
		},
	}
}

// --- users --------------------------------------------------------------------

func userOut(u AdminUser) map[string]any {
	websites := u.Websites
	if websites == nil {
		websites = []int64{}
	}
	return map[string]any{
		"id": u.ID, "name": u.Name, "email": u.Email, "role": u.Role,
		"created_at": u.CreatedAt, "last_login": u.LastLogin, "language": u.Locale,
		"may_publish": u.MayPublish, "all_websites": u.AllWebsites, "websites": websites,
		"two_factor": u.TwoFactor, "recovery_codes_left": u.RecoveryCodesLeft,
	}
}

func linkOut(l AccessLink) map[string]any {
	return map[string]any{
		"url": l.URL, "expires_at": l.Expires.UTC().Format(timeLayout),
		"mailed": l.Mailed, "mail_error": l.MailError,
	}
}

// rightsProperties are the arguments that limit an editor.
func rightsProperties(into map[string]Property) {
	into["may_publish"] = Property{Type: "boolean",
		Description: "whether this person may put pages online themselves; otherwise they submit for review"}
	into["all_websites"] = Property{Type: "boolean",
		Description: "true: every website. false: only those in websites — none if that list is empty"}
	into["websites"] = Property{Type: "array", Items: &Property{Type: "integer"},
		Description: "ids of the websites this person may enter, when all_websites is false"}
}

// mailLanguage checks the language a mail is to be written in.
func mailLanguage(raw string) (string, error) {
	lang := locale.Normalise(strings.TrimSpace(raw))
	if lang == "" {
		return "", nil
	}
	if !i18n.Known(lang) {
		return "", fmt.Errorf("the admin cannot speak %q; list_languages shows which it can", raw)
	}
	return lang, nil
}

func listUsers(d Deps) Tool {
	return Tool{
		Name: "list_users",
		Description: "Lists the people who can sign in to the admin, with role, website limits, " +
			"publishing right, last sign-in and whether two-step verification is on.",
		InputSchema: Schema{Type: "object"},
		Admin:       true,
		Run: func(c Call) (any, error) {
			ops, err := adminOpsAs[userOps](d)
			if err != nil {
				return nil, err
			}
			list, err := ops.OpUsers(c.Ctx)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(list))
			for _, u := range list {
				out = append(out, userOut(u))
			}
			return map[string]any{"users": out}, nil
		},
	}
}

func getUser(d Deps) Tool {
	return Tool{
		Name:        "get_user",
		Description: "Fetches one admin account by its id.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{"id": {Type: "integer", Description: "id of the user"}},
			Required:   []string{"id"}},
		Admin: true,
		Run: func(c Call) (any, error) {
			var a struct {
				ID int64 `json:"id"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, err := adminOpsAs[userOps](d)
			if err != nil {
				return nil, err
			}
			u, err := ops.OpUser(c.Ctx, a.ID)
			if err != nil {
				return nil, err
			}
			return userOut(*u), nil
		},
	}
}

func inviteUser(d Deps) Tool {
	props := map[string]Property{
		"email": {Type: "string", Description: "the person's email address; they sign in with it"},
		"name":  {Type: "string", Description: "the name shown in the admin"},
		"role": {Type: "string", Enum: []string{"editor", "admin"},
			Description: "editor by default; an admin manages the whole installation"},
		"language": {Type: "string",
			Description: "language of the invitation mail, such as de; English when left out"},
	}
	rightsProperties(props)
	return Tool{
		Name: "invite_user",
		Description: "Creates an admin account and a one-time invitation link, valid for 72 hours, " +
			"through which the person sets their own password. When mail is set up the link is " +
			"also mailed to them; either way it is returned here, once — hand it to the operator. " +
			"An editor has every website unless all_websites is false.",
		InputSchema: Schema{Type: "object", Properties: props, Required: []string{"email"}},
		Writes:      true,
		Admin:       true,
		Run: func(c Call) (any, error) {
			var a struct {
				Email       string  `json:"email"`
				Name        string  `json:"name"`
				Role        string  `json:"role"`
				Language    string  `json:"language"`
				MayPublish  *bool   `json:"may_publish"`
				AllWebsites *bool   `json:"all_websites"`
				Websites    []int64 `json:"websites"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, err := adminOpsAs[userOps](d)
			if err != nil {
				return nil, err
			}
			lang, err := mailLanguage(a.Language)
			if err != nil {
				return nil, err
			}
			if lang == "" {
				lang = i18n.Source
			}
			role := strings.TrimSpace(a.Role)
			if role == "" {
				role = "editor"
			}
			// The screen's defaults: may publish, every website.
			rights := UserRights{MayPublish: true, AllWebsites: true}
			if a.MayPublish != nil {
				rights.MayPublish = *a.MayPublish
			}
			if a.AllWebsites != nil {
				rights.AllWebsites = *a.AllWebsites
			} else if len(a.Websites) > 0 {
				rights.AllWebsites = false
			}
			rights.Websites = a.Websites

			u, link, err := ops.OpInviteUser(c.Ctx, c.Host, lang, a.Name, a.Email, role, rights)
			if err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionUserCreate, EntityType: "user", EntityID: u.ID,
				Metadata: map[string]any{"email": u.Email, "rolle": u.Role, "einladung": true},
			})
			return map[string]any{"user": userOut(*u), "invitation": linkOut(link)}, nil
		},
	}
}

func updateUser(d Deps) Tool {
	props := map[string]Property{
		"id":    {Type: "integer", Description: "id of the user"},
		"name":  {Type: "string", Description: "new name"},
		"email": {Type: "string", Description: "new email address"},
		"role":  {Type: "string", Enum: []string{"editor", "admin"}, Description: "new role"},
	}
	rightsProperties(props)
	return Tool{
		Name: "update_user",
		Description: "Changes an admin account. Only what is given changes. all_websites and " +
			"websites replace the website limit together; may_publish alone changes only the " +
			"publishing right. An admin has no limits. The last admin keeps the role.",
		InputSchema: Schema{Type: "object", Properties: props, Required: []string{"id"}},
		Writes:      true,
		Admin:       true,
		Run: func(c Call) (any, error) {
			var a struct {
				ID          int64   `json:"id"`
				Name        *string `json:"name"`
				Email       *string `json:"email"`
				Role        *string `json:"role"`
				MayPublish  *bool   `json:"may_publish"`
				AllWebsites *bool   `json:"all_websites"`
				Websites    []int64 `json:"websites"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, err := adminOpsAs[userOps](d)
			if err != nil {
				return nil, err
			}
			ch := UserChange{Name: a.Name, Email: a.Email, Role: a.Role}
			if a.AllWebsites != nil || a.Websites != nil {
				current, err := ops.OpUser(c.Ctx, a.ID)
				if err != nil {
					return nil, err
				}
				r := UserRights{MayPublish: current.MayPublish, Websites: a.Websites,
					AllWebsites: a.AllWebsites != nil && *a.AllWebsites}
				if a.MayPublish != nil {
					r.MayPublish = *a.MayPublish
				}
				ch.Rights = &r
			} else {
				ch.MayPublish = a.MayPublish
			}
			u, err := ops.OpUpdateUser(c.Ctx, a.ID, ch)
			if err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionUserUpdate, EntityType: "user", EntityID: u.ID,
			})
			return userOut(*u), nil
		},
	}
}

func accessLinkTool(d Deps, name, purpose, description string) Tool {
	return Tool{
		Name:        name,
		Description: description,
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"id": {Type: "integer", Description: "id of the user"},
			"language": {Type: "string",
				Description: "language of the mail, such as de; the person's own when left out"},
		}, Required: []string{"id"}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				ID       int64  `json:"id"`
				Language string `json:"language"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, err := adminOpsAs[userOps](d)
			if err != nil {
				return nil, err
			}
			lang, err := mailLanguage(a.Language)
			if err != nil {
				return nil, err
			}
			link, err := ops.OpUserLink(c.Ctx, c.Host, lang, a.ID, purpose)
			if err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionUserLink, EntityType: "user", EntityID: a.ID,
				Metadata: map[string]any{"zweck": purpose},
			})
			return linkOut(link), nil
		},
	}
}

func createPasswordResetLink(d Deps) Tool {
	return accessLinkTool(d, "create_password_reset_link", "reset",
		"Creates a one-time link, valid for an hour, through which the person sets a new "+
			"password. Every open session of the account ends at once. When mail is set up the "+
			"link is mailed too; either way it is returned here, once.")
}

func createInvitationLink(d Deps) Tool {
	return accessLinkTool(d, "create_invitation_link", "invite",
		"Creates a new invitation link, valid for 72 hours, for an existing account whose "+
			"first one expired. An earlier invitation stops working. Mailed when mail is set up, "+
			"and returned here either way.")
}

func endUserSessions(d Deps) Tool {
	return Tool{
		Name:        "end_user_sessions",
		Description: "Signs a person out of the admin on every device. Their password stays.",
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{"id": {Type: "integer", Description: "id of the user"}},
			Required:   []string{"id"}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				ID int64 `json:"id"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, err := adminOpsAs[userOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpEndUserSessions(c.Ctx, a.ID); err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionUserSessionsEnd, EntityType: "user", EntityID: a.ID,
			})
			return map[string]any{"id": a.ID, "sessions_ended": true}, nil
		},
	}
}

func disableUserTwoFactor(d Deps) Tool {
	return Tool{
		Name: "disable_user_two_factor",
		Description: "Removes the two-step verification of an account whose phone and recovery " +
			"codes are gone — the way back in, as \"holzcloud user 2fa disable\" on the server. " +
			"The account can then sign in with its password alone; an administrator is asked to " +
			"set it up again. Needs confirm: true.",
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"id":      {Type: "integer", Description: "id of the user"},
			"confirm": {Type: "boolean", Description: "must be true"},
		}, Required: []string{"id", "confirm"}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				ID      int64 `json:"id"`
				Confirm bool  `json:"confirm"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if !a.Confirm {
				return nil, errConfirm
			}
			ops, err := adminOpsAs[userOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpDisableUserTwoFactor(c.Ctx, a.ID); err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionUserTwoFactorOff, EntityType: "user", EntityID: a.ID,
			})
			return map[string]any{"id": a.ID, "two_factor": false}, nil
		},
	}
}

func deleteUser(d Deps) Tool {
	return Tool{
		Name: "delete_user",
		Description: "Deletes an admin account. What the person wrote stays. The last admin " +
			"cannot be deleted. Needs confirm: true.",
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"id":      {Type: "integer", Description: "id of the user"},
			"confirm": {Type: "boolean", Description: "must be true"},
		}, Required: []string{"id", "confirm"}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				ID      int64 `json:"id"`
				Confirm bool  `json:"confirm"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if !a.Confirm {
				return nil, errConfirm
			}
			ops, err := adminOpsAs[userOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpDeleteUser(c.Ctx, a.ID); err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionUserDelete, EntityType: "user", EntityID: a.ID,
			})
			return map[string]any{"id": a.ID, "deleted": true}, nil
		},
	}
}

// --- AI keys ------------------------------------------------------------------

var errNoTokens = errors.New("AI keys are not available on this installation")

func keyOut(t Token, self int64) map[string]any {
	out := map[string]any{
		"id": t.ID, "name": t.Name, "level": t.LevelOf().String(),
		"website": t.WebsiteID, "created_at": t.CreatedAt.Format(timeLayout),
		"last_used_at": nil, "expires_at": nil, "this_key": t.ID == self,
	}
	if t.LastUsedAt != nil {
		out["last_used_at"] = t.LastUsedAt.Format(timeLayout)
	}
	if t.ExpiresAt != nil {
		out["expires_at"] = t.ExpiresAt.Format(timeLayout)
	}
	return out
}

func listAIKeys(d Deps) Tool {
	return Tool{
		Name: "list_ai_keys",
		Description: "Lists the access keys for AI assistants: name, level, website (0 = every " +
			"website), last use and expiry. The secrets themselves are never shown again. " +
			"this_key marks the key of this connection.",
		InputSchema: Schema{Type: "object"},
		Admin:       true,
		Run: func(c Call) (any, error) {
			if d.Tokens == nil {
				return nil, errNoTokens
			}
			list, err := d.Tokens.List(c.Ctx)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(list))
			for _, t := range list {
				out = append(out, keyOut(t, c.Scope.TokenID))
			}
			return map[string]any{"keys": out}, nil
		},
	}
}

// ErrNoAdminKeyHere refuses an admin key asked for through the connection.
var ErrNoAdminKeyHere = errors.New("an admin key cannot be created through this connection; " +
	"it is made on the server with: holzcloud ai key create -level admin")

// creatableLevel reads the level create_ai_key may issue. Admin is refused by
// name, so the answer says where an admin key comes from instead of calling
// the word unknown.
func creatableLevel(raw string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "read":
		return LevelRead, nil
	case "content", "":
		return LevelContent, nil
	case "admin":
		return 0, ErrNoAdminKeyHere
	}
	return 0, fmt.Errorf("unknown level %q: read or content", raw)
}

func createAIKey(d Deps) Tool {
	return Tool{
		Name: "create_ai_key",
		Description: "Creates an access key for an AI assistant and returns its secret — once; " +
			"it cannot be shown again. Level read may only read; content may also write. An " +
			"admin key cannot be made here, only on the server. Leave website out for every " +
			"website.",
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"name":    {Type: "string", Description: "what the key is for, such as the assistant's name"},
			"level":   {Type: "string", Enum: []string{"read", "content"}, Description: "content by default"},
			"website": {Type: "integer", Description: "id of the one website the key may reach; leave out for all"},
			"days":    {Type: "integer", Description: "days until it expires; leave out for never"},
		}, Required: []string{"name"}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				Name    string `json:"name"`
				Level   string `json:"level"`
				Website int64  `json:"website"`
				Days    int    `json:"days"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			level, err := creatableLevel(a.Level)
			if err != nil {
				return nil, err
			}
			// Said twice on purpose: whatever creatableLevel grows into, no
			// admin key leaves this tool.
			if level >= LevelAdmin {
				return nil, ErrNoAdminKeyHere
			}
			if d.Tokens == nil {
				return nil, errNoTokens
			}
			if a.Website < 0 {
				return nil, errors.New("the website id cannot be negative")
			}
			if a.Website > 0 {
				ws, err := d.Domains.GetWebsite(c.Ctx, a.Website)
				if err != nil || ws == nil {
					return nil, fmt.Errorf("there is no website %d", a.Website)
				}
			}
			var lifetime time.Duration
			if a.Days > 0 {
				lifetime = time.Duration(a.Days) * 24 * time.Hour
			}
			secret, t, err := d.Tokens.IssueLevel(c.Ctx, a.Name, a.Website, level, lifetime)
			if errors.Is(err, ErrNameMissing) {
				return nil, errors.New("the key needs a name")
			}
			if err != nil {
				return nil, err
			}
			var site *int64
			if t.WebsiteID > 0 {
				site = &t.WebsiteID
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionAIKeyCreate, EntityType: "ai_key", EntityID: t.ID,
				WebsiteID: site, Metadata: map[string]any{"name": t.Name, "stufe": level.String()},
			})
			out := keyOut(*t, c.Scope.TokenID)
			out["secret"] = secret
			out["endpoint_path"] = "/ai"
			out["note"] = "The secret is shown only now. The assistant sends it as " +
				"\"Authorization: Bearer <secret>\" to /ai on this server."
			return out, nil
		},
	}
}

func revokeAIKey(d Deps) Tool {
	return Tool{
		Name: "revoke_ai_key",
		Description: "Withdraws an access key; an assistant using it is turned away from its " +
			"next request. The key of this connection cannot revoke itself — that is done on " +
			"the AI screen or with \"holzcloud ai key revoke\". Needs confirm: true.",
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"id":      {Type: "integer", Description: "id of the key, from list_ai_keys"},
			"confirm": {Type: "boolean", Description: "must be true"},
		}, Required: []string{"id", "confirm"}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				ID      int64 `json:"id"`
				Confirm bool  `json:"confirm"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if !a.Confirm {
				return nil, errConfirm
			}
			if d.Tokens == nil {
				return nil, errNoTokens
			}
			if a.ID == c.Scope.TokenID {
				return nil, errors.New("a key cannot revoke itself; do it on the AI screen of " +
					"the admin or with: holzcloud ai key revoke")
			}
			t, err := d.Tokens.Get(c.Ctx, a.ID)
			if err != nil {
				return nil, errors.New("there is no such key")
			}
			if err := d.Tokens.Revoke(c.Ctx, a.ID); err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionAIKeyRevoke, EntityType: "ai_key", EntityID: t.ID,
				Metadata: map[string]any{"name": t.Name, "stufe": t.LevelOf().String()},
			})
			return map[string]any{"id": t.ID, "revoked": true}, nil
		},
	}
}

// --- plugins ------------------------------------------------------------------

func pluginOut(p plugin.Status) map[string]any {
	websites := p.Websites
	if websites == nil {
		websites = []int64{}
	}
	out := map[string]any{
		"id": p.ID, "name": p.Name, "version": p.Version, "enabled": p.Enabled,
		"running": p.Running, "websites": websites, "last_error": p.LastError,
		"installed_at": p.InstalledAt.UTC().Format(timeLayout),
	}
	if m := p.Manifest; m != nil {
		out["name"] = m.NameIn(i18n.Source)
		out["description"] = m.DescriptionIn(i18n.Source)
		out["author"] = m.Author
		out["hooks"] = m.Hooks
		out["routes"] = m.Routes
		out["permissions"] = m.Permissions
		out["has_admin_screen"] = m.Admin != nil
	}
	return out
}

func listPlugins(d Deps) Tool {
	return Tool{
		Name: "list_plugins",
		Description: "Lists the installed plugins: whether each is switched on and running, " +
			"the websites it acts on, what it hooks into and asked permission for, and the " +
			"last error.",
		InputSchema: Schema{Type: "object"},
		Admin:       true,
		Run: func(c Call) (any, error) {
			ops, err := adminOpsAs[pluginOps](d)
			if err != nil {
				return nil, err
			}
			list, err := ops.OpPlugins(c.Ctx)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(list))
			for _, p := range list {
				out = append(out, pluginOut(p))
			}
			return map[string]any{"plugins": out}, nil
		},
	}
}

func installPlugin(d Deps) Tool {
	return Tool{
		Name: "install_plugin",
		Description: "Installs a plugin from its .zip archive (manifest and wasm module), " +
			"sent as base64 — the same checks as the upload on the plugin screen. A plugin " +
			"with the same id is replaced. It stays switched off: switch it on with " +
			"enable_plugin and choose websites with set_plugin_websites.",
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"archive_base64": {Type: "string", Description: "the .zip file, base64-encoded"},
		}, Required: []string{"archive_base64"}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				Archive string `json:"archive_base64"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, err := adminOpsAs[pluginOps](d)
			if err != nil {
				return nil, err
			}
			data, err := decodeFile(a.Archive)
			if err != nil {
				return nil, err
			}
			m, err := ops.OpPluginInstall(c.Ctx, data)
			if err != nil {
				return nil, fmt.Errorf("installing failed: %w", err)
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionPluginInstall, EntityType: "plugin",
				Metadata: map[string]any{"plugin": m.ID, "version": m.Version},
			})
			return map[string]any{"id": m.ID, "name": m.NameIn(i18n.Source), "version": m.Version,
				"enabled": false, "permissions": m.Permissions, "hooks": m.Hooks}, nil
		},
	}
}

func pluginSwitch(d Deps, on bool) Tool {
	name, verb, action := "disable_plugin", "Switches a plugin off", activity.ActionPluginDisable
	if on {
		name, verb, action = "enable_plugin", "Switches a plugin on and loads it; says why when it does not come up", activity.ActionPluginEnable
	}
	return Tool{
		Name:        name,
		Description: verb + ". It acts only on the websites set with set_plugin_websites.",
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"id": {Type: "string", Description: "the plugin's id, from list_plugins"},
		}, Required: []string{"id"}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				ID string `json:"id"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, err := adminOpsAs[pluginOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpPluginSwitch(c.Ctx, a.ID, on); err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: action, EntityType: "plugin", Metadata: map[string]any{"plugin": a.ID},
			})
			return map[string]any{"id": a.ID, "enabled": on}, nil
		},
	}
}

func enablePlugin(d Deps) Tool  { return pluginSwitch(d, true) }
func disablePlugin(d Deps) Tool { return pluginSwitch(d, false) }

func setPluginWebsites(d Deps) Tool {
	return Tool{
		Name: "set_plugin_websites",
		Description: "Sets the websites a plugin acts on, replacing the list. An empty list " +
			"leaves it installed but idle.",
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"id":       {Type: "string", Description: "the plugin's id"},
			"websites": {Type: "array", Items: &Property{Type: "integer"}, Description: "website ids"},
		}, Required: []string{"id", "websites"}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				ID       string  `json:"id"`
				Websites []int64 `json:"websites"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, err := adminOpsAs[pluginOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpPluginWebsites(c.Ctx, a.ID, a.Websites); err != nil {
				return nil, err
			}
			if a.Websites == nil {
				a.Websites = []int64{}
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionPluginWebsites, EntityType: "plugin",
				Metadata: map[string]any{"plugin": a.ID, "websites": a.Websites},
			})
			return map[string]any{"id": a.ID, "websites": a.Websites}, nil
		},
	}
}

func removePlugin(d Deps) Tool {
	return Tool{
		Name: "remove_plugin",
		Description: "Removes a plugin together with everything it stored. Cannot be undone. " +
			"Needs confirm: true.",
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"id":      {Type: "string", Description: "the plugin's id"},
			"confirm": {Type: "boolean", Description: "must be true"},
		}, Required: []string{"id", "confirm"}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				ID      string `json:"id"`
				Confirm bool   `json:"confirm"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if !a.Confirm {
				return nil, errConfirm
			}
			ops, err := adminOpsAs[pluginOps](d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpPluginRemove(c.Ctx, a.ID); err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionPluginRemove, EntityType: "plugin",
				Metadata: map[string]any{"plugin": a.ID},
			})
			return map[string]any{"id": a.ID, "removed": true}, nil
		},
	}
}

// --- mail ---------------------------------------------------------------------

func getMailStatus(d Deps) Tool {
	return Tool{
		Name: "get_mail_status",
		Description: "Says whether mail is set up (server and sender), how many messages " +
			"are waiting, how many were given up on, the last error and when the last one went out.",
		InputSchema: Schema{Type: "object"},
		Admin:       true,
		Run: func(c Call) (any, error) {
			ops, err := adminOpsAs[mailOps](d)
			if err != nil {
				return nil, err
			}
			st, err := ops.OpMailStatus(c.Ctx)
			if err != nil {
				return nil, err
			}
			out := map[string]any{
				"configured": st.Configured, "host": st.Host, "from": st.From,
				"pending": st.Pending, "failed": st.Failed, "last_error": st.LastError,
				"last_sent_at": nil,
			}
			if st.LastSent != nil {
				out["last_sent_at"] = st.LastSent.UTC().Format(timeLayout)
			}
			return out, nil
		},
	}
}

func retryMail(d Deps) Tool {
	return Tool{
		Name: "retry_mail",
		Description: "Puts every message that was given up on back in the queue — after the " +
			"mail settings were fixed, say. Returns how many.",
		InputSchema: Schema{Type: "object"},
		Writes:      true,
		Admin:       true,
		Run: func(c Call) (any, error) {
			ops, err := adminOpsAs[mailOps](d)
			if err != nil {
				return nil, err
			}
			n, err := ops.OpMailRetry(c.Ctx)
			if err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionMailRetry, EntityType: "mail",
				Metadata: map[string]any{"anzahl": n},
			})
			return map[string]any{"requeued": n}, nil
		},
	}
}

func sendTestMail(d Deps) Tool {
	return Tool{
		Name: "send_test_mail",
		Description: "Queues a test message to prove that sending works. It goes only to the " +
			"address of an account of this installation. Check get_mail_status a little later.",
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"to":       {Type: "string", Description: "email address of an admin account"},
			"language": {Type: "string", Description: "language of the message; English when left out"},
		}, Required: []string{"to"}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				To       string `json:"to"`
				Language string `json:"language"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, err := adminOpsAs[mailOps](d)
			if err != nil {
				return nil, err
			}
			lang, err := mailLanguage(a.Language)
			if err != nil {
				return nil, err
			}
			if err := ops.OpMailTest(c.Ctx, lang, a.To); err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionMailTest, EntityType: "mail",
				Metadata: map[string]any{"an": a.To},
			})
			return map[string]any{"queued": true, "to": a.To}, nil
		},
	}
}

// --- languages ----------------------------------------------------------------

func listLanguages(d Deps) Tool {
	return Tool{
		Name: "list_languages",
		Description: "Lists the languages the admin can be shown in, how complete each is, and " +
			"which came from a file (only those can be removed).",
		InputSchema: Schema{Type: "object"},
		Admin:       true,
		Run: func(c Call) (any, error) {
			out := []map[string]any{}
			for _, s := range i18n.Stats() {
				out = append(out, map[string]any{
					"code": s.Code, "name": s.Name, "translated": s.Translated, "total": s.Total,
					"percent": s.Percent(), "from_file": s.OnDisk, "source": s.Source,
					"base_missing": s.BaseMissing,
				})
			}
			return map[string]any{"languages": out, "strings": len(i18n.SourceStrings()),
				"folder_set_up": i18n.Dir() != ""}, nil
		},
	}
}

func installLanguage(d Deps) Tool {
	return Tool{
		Name: "install_language",
		Description: "Installs a language file for the admin: a JSON object that maps each " +
			"English source sentence to its translation. The same checks as the upload on the " +
			"languages screen; an installed file of the same language is replaced.",
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"code": {Type: "string", Description: "language tag such as fr or fr-CH"},
			"json": {Type: "string", Description: "the file's content as text"},
		}, Required: []string{"code", "json"}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				Code string `json:"code"`
				JSON string `json:"json"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, err := adminOpsAs[languageOps](d)
			if err != nil {
				return nil, err
			}
			code := locale.Normalise(strings.TrimSpace(a.Code))
			n, err := ops.OpInstallLanguage(c.Ctx, code, []byte(a.JSON))
			if err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionLanguageInstall, EntityType: "language",
				Metadata: map[string]any{"sprache": code, "anzahl": n},
			})
			out := map[string]any{"code": code, "translations": n}
			if base := i18n.Base(code); base != "" && base != i18n.Source && !i18n.Known(base) {
				out["warning"] = fmt.Sprintf("the base language %s is not installed; what this "+
					"file does not translate appears in English until it is", base)
			}
			return out, nil
		},
	}
}

func removeLanguage(d Deps) Tool {
	return Tool{
		Name: "remove_language",
		Description: "Removes a language file. A language built into the program cannot be " +
			"removed. Needs confirm: true.",
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"code":    {Type: "string", Description: "language tag"},
			"confirm": {Type: "boolean", Description: "must be true"},
		}, Required: []string{"code", "confirm"}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				Code    string `json:"code"`
				Confirm bool   `json:"confirm"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if !a.Confirm {
				return nil, errConfirm
			}
			ops, err := adminOpsAs[languageOps](d)
			if err != nil {
				return nil, err
			}
			code := locale.Normalise(strings.TrimSpace(a.Code))
			if err := ops.OpRemoveLanguage(c.Ctx, code); err != nil {
				return nil, err
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionLanguageRemove, EntityType: "language",
				Metadata: map[string]any{"sprache": code},
			})
			return map[string]any{"code": code, "removed": true}, nil
		},
	}
}

// --- the brand ----------------------------------------------------------------

func brandOut() map[string]any {
	b := branding.Current()
	return map[string]any{"name": b.Name, "mark": b.Mark, "logo_url": b.LogoURL,
		"has_logo": branding.LogoPath() != ""}
}

func getBranding(d Deps) Tool {
	return Tool{
		Name: "get_branding",
		Description: "Reads what the admin calls itself: the installation's name, the letter " +
			"in its square, and the logo if there is one.",
		InputSchema: Schema{Type: "object"},
		Admin:       true,
		Run:         func(c Call) (any, error) { return brandOut(), nil },
	}
}

func updateBranding(d Deps) Tool {
	return Tool{
		Name: "update_branding",
		Description: "Changes the installation's name (up to 40 characters), its letter (one " +
			"or two) and its logo. Only what is given changes. The logo is PNG, WebP or SVG, at " +
			"most 512 KB, from the media library (logo_media) or sent as base64 with a file " +
			"name that says its type (logo_base64 and logo_filename). remove_logo takes it away.",
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"name":          {Type: "string", Description: "the installation's name"},
			"mark":          {Type: "string", Description: "one or two characters"},
			"logo_media":    {Type: "integer", Description: "id of an image in a website's media library"},
			"logo_base64":   {Type: "string", Description: "the logo file, base64-encoded"},
			"logo_filename": {Type: "string", Description: "file name ending in .png, .webp or .svg"},
			"remove_logo":   {Type: "boolean", Description: "true removes the logo"},
		}},
		Writes: true,
		Admin:  true,
		Run: func(c Call) (any, error) {
			var a struct {
				Name         *string `json:"name"`
				Mark         *string `json:"mark"`
				LogoMedia    int64   `json:"logo_media"`
				LogoBase64   string  `json:"logo_base64"`
				LogoFilename string  `json:"logo_filename"`
				RemoveLogo   bool    `json:"remove_logo"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, err := adminOpsAs[brandOps](d)
			if err != nil {
				return nil, err
			}
			if a.RemoveLogo && (a.LogoMedia != 0 || a.LogoBase64 != "") {
				return nil, errors.New("give a new logo or remove_logo, not both")
			}

			// The file is read and checked first, so a bad logo changes nothing.
			var logo []byte
			var filename string
			switch {
			case a.LogoMedia != 0 && a.LogoBase64 != "":
				return nil, errors.New("give logo_media or logo_base64, not both")
			case a.LogoMedia != 0:
				logo, filename, err = logoFromMedia(c, d, a.LogoMedia)
				if err != nil {
					return nil, err
				}
			case a.LogoBase64 != "":
				if strings.TrimSpace(a.LogoFilename) == "" {
					return nil, errors.New("logo_filename is needed, ending in .png, .webp or .svg")
				}
				logo, err = decodeFile(a.LogoBase64)
				if err != nil {
					return nil, err
				}
				filename = a.LogoFilename
			}

			what := []string{}
			if a.Name != nil || a.Mark != nil {
				current := branding.Current()
				name, mark := current.Name, current.Mark
				if a.Name != nil {
					name = *a.Name
				}
				if a.Mark != nil {
					mark = *a.Mark
				}
				if err := ops.OpSetBrand(c.Ctx, name, mark); err != nil {
					return nil, err
				}
				what = append(what, "name")
			}
			if logo != nil {
				if err := ops.OpSetLogo(c.Ctx, filename, logo); err != nil {
					return nil, err
				}
				what = append(what, "logo")
			}
			if a.RemoveLogo {
				if err := ops.OpRemoveLogo(c.Ctx); err != nil {
					return nil, err
				}
				what = append(what, "logo_entfernt")
			}
			if len(what) == 0 {
				return nil, errors.New("nothing to change: give name, mark, a logo or remove_logo")
			}
			changed(d, c, 0, activity.Entry{
				Action: activity.ActionBrandSave, EntityType: "brand",
				Metadata: map[string]any{"geaendert": what},
			})
			return brandOut(), nil
		},
	}
}

// logoFromMedia reads an image of the media library as the logo's bytes.
func logoFromMedia(c Call, d Deps, id int64) ([]byte, string, error) {
	if d.Media == nil || d.Limits.DataDir == "" {
		return nil, "", errors.New("the media library is not available here")
	}
	m, err := d.Media.GetByID(c.Ctx, id)
	if err != nil || m == nil {
		return nil, "", errors.New("there is no such media item")
	}
	if err := c.Scope.MaySee(m.WebsiteID); err != nil {
		return nil, "", err
	}
	f, err := os.Open(media.Path(d.Limits.DataDir, m.WebsiteID, m.Filename))
	if err != nil {
		return nil, "", errors.New("the file of that media item is missing")
	}
	defer f.Close()
	// One byte over the limit, so the check downstream sees a file that is too
	// large rather than a truncated one that happens to fit.
	data, err := io.ReadAll(io.LimitReader(f, branding.MaxLogoBytes+1))
	if err != nil {
		return nil, "", err
	}
	return data, m.Filename, nil
}

// decodeFile reads a file sent as base64, with or without a data: prefix and
// with or without padding.
func decodeFile(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "data:") {
		if i := strings.Index(raw, ","); i >= 0 {
			raw = raw[i+1:]
		}
	}
	raw = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, raw)
	if raw == "" {
		return nil, errors.New("the file is empty")
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding} {
		if data, err := enc.DecodeString(raw); err == nil {
			return data, nil
		}
	}
	return nil, errors.New("the file is not valid base64")
}

// --- the activity log ---------------------------------------------------------

func readActivityLog(d Deps) Tool {
	return Tool{
		Name: "read_activity_log",
		Description: "Reads the activity log, newest first: who changed what, and when. " +
			"Filters as on the log screen: website, user, action (such as page.publish, or " +
			"page.* for every page action) and a date range. Changes made through an AI key " +
			"are recorded as \"KI: <key name>\".",
		InputSchema: Schema{Type: "object", Properties: map[string]Property{
			"website": {Type: "integer", Description: "only this website"},
			"user":    {Type: "integer", Description: "only this user's actions"},
			"action":  {Type: "string", Description: "an action, or a prefix ending in .*"},
			"from":    {Type: "string", Description: "first day, YYYY-MM-DD"},
			"to":      {Type: "string", Description: "last day, YYYY-MM-DD, included"},
			"page":    {Type: "integer", Description: "page of results, from 1"},
			"limit":   {Type: "integer", Description: "entries per page, 50 by default, at most 200"},
		}},
		Admin: true,
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				User    int64  `json:"user"`
				Action  string `json:"action"`
				From    string `json:"from"`
				To      string `json:"to"`
				Page    int    `json:"page"`
				Limit   int    `json:"limit"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, err := adminOpsAs[activityOps](d)
			if err != nil {
				return nil, err
			}
			var f activity.Filter
			if a.Website > 0 {
				f.WebsiteID = &a.Website
			}
			if a.User > 0 {
				f.UserID = &a.User
			}
			f.Action = strings.TrimSpace(a.Action)
			if a.From != "" {
				t, err := time.Parse("2006-01-02", strings.TrimSpace(a.From))
				if err != nil {
					return nil, errors.New("from is not a date of the form YYYY-MM-DD")
				}
				f.From = &t
			}
			if a.To != "" {
				t, err := time.Parse("2006-01-02", strings.TrimSpace(a.To))
				if err != nil {
					return nil, errors.New("to is not a date of the form YYYY-MM-DD")
				}
				// Up to and including that day, as on the screen.
				t = t.Add(24*time.Hour - time.Second)
				f.To = &t
			}
			limit := a.Limit
			if limit <= 0 {
				limit = 50
			}
			limit = min(limit, 200)
			page := max(a.Page, 1)

			entries, total, err := ops.OpActivity(c.Ctx, f, limit, (page-1)*limit)
			if err != nil {
				return nil, err
			}
			names := map[int64]string{}
			if d.Domains != nil {
				if list, err := d.Domains.ListWebsites(c.Ctx); err == nil {
					for _, ws := range list {
						names[ws.ID] = ws.Name
					}
				}
			}
			out := make([]map[string]any, 0, len(entries))
			for _, e := range entries {
				row := map[string]any{
					"id": e.ID, "at": e.CreatedAt.UTC().Format(timeLayout), "actor": e.ActorEmail,
					"action": e.Action, "entity_type": e.EntityType, "entity_id": e.EntityID,
					"user": e.UserID, "website": e.WebsiteID, "details": e.Metadata,
				}
				if e.WebsiteID != nil {
					row["website_name"] = names[*e.WebsiteID]
				}
				out = append(out, row)
			}
			return map[string]any{"entries": out, "total": total, "page": page, "limit": limit}, nil
		},
	}
}
