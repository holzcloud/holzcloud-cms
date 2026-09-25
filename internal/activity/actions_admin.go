package activity

// The actions of installation management — accounts, keys, plugins, mail,
// languages and the brand. The screens for these wrote no rows before v2.7;
// the AI tools do, because a change made by a program on somebody else's
// machine is exactly the change someone later asks about. The names follow
// the pattern above, so "user.*" and "plugin.*" find them.
const (
	// ActionUserLink is an invitation or reset link issued; the metadata says
	// which.
	ActionUserLink = "user.link"
	// ActionUserSessionsEnd is every session of an account ended.
	ActionUserSessionsEnd = "user.sessions_end"
	// ActionUserTwoFactorOff is an account's second factor removed.
	ActionUserTwoFactorOff = "user.2fa_disable"

	ActionAIKeyCreate = "ai_key.create"
	ActionAIKeyRevoke = "ai_key.revoke"

	ActionPluginInstall  = "plugin.install"
	ActionPluginEnable   = "plugin.enable"
	ActionPluginDisable  = "plugin.disable"
	ActionPluginWebsites = "plugin.websites"
	ActionPluginRemove   = "plugin.remove"

	ActionMailRetry = "mail.retry"
	ActionMailTest  = "mail.test"

	ActionLanguageInstall = "language.install"
	ActionLanguageRemove  = "language.remove"

	ActionBrandSave = "brand.save"
)
