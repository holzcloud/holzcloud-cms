package web

import (
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/gorilla/csrf"
	"github.com/holzcloud/holzcloud-cms/internal/branding"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// LayoutData holds common data for admin template rendering.
type LayoutData struct {
	Title          string
	CSRFToken      string
	Flash          Flash
	UserEmail      string
	UserRole       string
	ActiveNav      string
	CurrentWebsite *domain.Website

	// NavWebsites feeds the switcher in the top bar. Empty on a screen rendered
	// without the navigation middleware, which the template survives.
	//
	// "Nav" in the name and not just "Websites": several screens carry their own
	// list of websites, and an embedded field of the same name would be shadowed
	// by theirs — silently, and with a type the template happens to accept.
	NavWebsites []domain.Website
	// NavWebsite is what the section links in the sidebar point at. It is the
	// website from the address where there is one, and otherwise the last one
	// visited — so the content menu does not disappear on the way through the
	// user list.
	NavWebsite *domain.Website
	// PluginScreens are the screens plugins bring along. They stand in the
	// sidebar beside everything else, because to whoever uses them they are not
	// "a plugin" — they are where the enquiries are.
	PluginScreens []NavLink
	// UserInitial is the letter in the avatar. Computed here rather than in the
	// template: a template function would be a whole FuncMap for one letter.
	UserInitial string
	// Brand is what this installation calls itself — the word in the corner
	// and the second half of the window title.
	Brand branding.Brand
	// Fullscreen strips the shell down to the content: no sidebar, no header.
	// For writing a long text, which is the one thing in this administration
	// that takes an hour rather than a minute.
	Fullscreen bool

	// Lang is the language this screen is being shown in, for <html lang>.
	// Announcing an English administration as German is the difference between
	// a screen reader reading it and mangling it.
	Lang string
	// Version and SourceURL sit in the sidebar. Not decoration: Holzcloud CMS
	// is under the AGPL, and section 13 obliges whoever runs a *modified*
	// version as a network service to offer its source to the people using it.
	// A link that is already there makes that the default rather than something
	// an operator has to think of — the arrangement the FSF itself suggests for
	// a web application.
	Version   string
	SourceURL string
}

// The build stamps these; SetBuild is called once at startup. Package-level
// because every page in the admin shows them and threading two strings through
// forty handlers would be worse than this.
var (
	buildVersion = "dev"
	buildSource  = "https://github.com/holzcloud/holzcloud-cms"
)

// SetBuild records what this binary is, for the sidebar.
func SetBuild(version, sourceURL string) {
	if version != "" {
		buildVersion = version
	}
	if sourceURL != "" {
		buildSource = sourceURL
	}
}

// Titlef builds a page title that has something in it.
//
// The format is translated first and filled in afterwards, so a language that
// wants the parts the other way round can write "%s – pages" and get it.
func Titlef(r *http.Request, format string, args ...any) string {
	return i18n.Tf(i18n.Lang(r.Context()), format, args...)
}

// T translates one string in the language of this request. For the handful of
// places that build a sentence outside a template and outside a flash.
func T(r *http.Request, s string) string {
	return i18n.T(i18n.Lang(r.Context()), s)
}

// initial is the first letter of an address, upper case. Empty stays empty —
// an avatar with a placeholder letter is worse than one without.
func initial(email string) string {
	for _, r := range email {
		return strings.ToUpper(string(r))
	}
	return ""
}

// IsAdmin reports whether the person looking may see the administrative
// section. The server checks this again on every route; here it only decides
// whether to show a menu entry that would answer with a refusal.
func (d LayoutData) IsAdmin() bool { return d.UserRole == "admin" }

// CSRFTokenFromRequest returns the CSRF token for the current request.
func CSRFTokenFromRequest(r *http.Request) string {
	return csrf.Token(r)
}

// NewLayoutData creates a LayoutData populated from the request context and session.
func NewLayoutData(r *http.Request, sm *scs.SessionManager, title string) LayoutData {
	ctx := r.Context()
	return LayoutData{
		// The title is translated here rather than at the caller, so a handler
		// writes the German words and nothing else. A title with something in
		// it — "Seiten – Velowerkstatt" — goes through Titlef instead, which
		// translates the frame before filling it in.
		Title:         i18n.T(i18n.Lang(ctx), title),
		Lang:          i18n.Lang(ctx),
		CSRFToken:     csrf.Token(r),
		Flash:         GetFlash(sm, ctx),
		UserEmail:     sm.GetString(ctx, "user_email"),
		UserRole:      sm.GetString(ctx, "user_role"),
		NavWebsites:   WebsitesFrom(ctx),
		NavWebsite:    NavWebsiteFrom(ctx),
		PluginScreens: PluginLinksFrom(ctx),
		UserInitial:   initial(sm.GetString(ctx, "user_email")),
		// Vollbild ist eine Frage der Adresse und nicht eine Einstellung: es
		// gilt für diesen einen Bildschirm, solange man darauf ist, und ist
		// beim nächsten Aufruf wieder weg. Genau so benutzt man es — man
		// schreibt einen Text zu Ende, nicht ein halbes Jahr.
		Fullscreen: r.URL.Query().Get("vollbild") == "1",
		Brand:      branding.Current(),
		Version:    buildVersion,
		SourceURL:  buildSource,
	}
}
