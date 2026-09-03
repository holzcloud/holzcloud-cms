package admin

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/config"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/outbox"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/payrexx"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
	"github.com/holzcloud/holzcloud-cms/internal/user"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// Handler holds dependencies for admin HTTP handlers.
type Handler struct {
	db           *db.DB
	sm           *scs.SessionManager
	templates    *web.PageTemplates
	argon2Params auth.Argon2Params
	domains      *domain.Store
	resolver     *domain.Resolver
	pages        *page.Store
	tmplStore    *tmplmgr.Store
	plugins      *plugin.Manager
	menuStore    *menu.Store
	mediaStore   *media.Store
	snippets     *snippet.Store
	terms        *term.Store
	// fields are the website's own page fields.
	fields *field.Store
	// kinds are the website's own content kinds, beside page and post.
	kinds *kind.Store
	// blockTypes are the website's own block kinds, beside the built-in nine.
	blockTypes *block.Store
	// mail is the outbox. It may be nil, in which case nothing is ever sent
	// and the screens say so.
	mail *mail.Queue
	// aiTokens are the keys an operator hands to their own assistant. Nil means
	// the screen does not exist, which is what a build without it should look
	// like from the outside.
	aiTokens *ai.Store
	// activityStore is the record of what was done here. Nil means nothing is
	// recorded and the screen is not there — see LogActivity.
	activityStore *activity.Store
	// products, orders and payments back the shop. Each may be nil, in which
	// case the shop screens do not exist and no shop route answers.
	products *shop.Store
	orders   *shop.OrderStore
	payments *payrexx.Client
	// outbox takes the order confirmations. Nil means nothing is written.
	outbox        *outbox.Store
	share         *sharelink.Signer
	users         *user.Store
	loader        *tmpl.Loader
	cfg           *config.Config
	loginThrottle *auth.LoginThrottle
	clientIP      *web.ClientIPResolver
}

// NewHandler creates an admin handler with the given dependencies.
func NewHandler(database *db.DB, sm *scs.SessionManager, templates *web.PageTemplates, params auth.Argon2Params, domains *domain.Store, resolver *domain.Resolver, pages *page.Store, tmplStore *tmplmgr.Store, menuStore *menu.Store, mediaStore *media.Store, snippets *snippet.Store, terms *term.Store, share *sharelink.Signer, loader *tmpl.Loader, cfg *config.Config, loginThrottle *auth.LoginThrottle, clientIP *web.ClientIPResolver) *Handler {
	// One field store for both: a block kind's fields live in the same table as
	// a page's, and two stores over one table is one cache and one set of rules
	// too many.
	fields := field.NewStore(database)
	return &Handler{
		db:           database,
		sm:           sm,
		templates:    templates,
		argon2Params: params,
		domains:      domains,
		resolver:     resolver,
		pages:        pages,
		tmplStore:    tmplStore,
		menuStore:    menuStore,
		mediaStore:   mediaStore,
		snippets:     snippets,
		terms:        terms,
		fields:       fields,
		kinds:        kind.NewStore(database),
		blockTypes:   block.NewStore(database, fields),
		share:        share,
		// The admin handlers still carry their own inline SQL for the ordinary
		// user CRUD; this store owns the parts a CLI also needs — password
		// hashing, the last-admin guard and the one-time links.
		users:         user.NewStore(database, params),
		loader:        loader,
		cfg:           cfg,
		loginThrottle: loginThrottle,
		clientIP:      clientIP,
	}
}

// currentUserID returns the signed-in user for authorship columns, or nil when
// there is no session. It is a pointer because created_by and updated_by are
// nullable: a row written by a CLI command or a migration has no author, and
// storing 0 there would point at a user that does not exist.
func (h *Handler) currentUserID(r *http.Request) *int64 {
	id := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	if id == 0 {
		return nil
	}
	return &id
}

// NewUserLookup returns an auth.UserLookup backed by the users table. It is used
// by the RequireAuth middleware to re-validate a session against the database on
// every request.
func NewUserLookup(database *db.DB) auth.UserLookup {
	return func(ctx context.Context, id int64) (string, bool, error) {
		var role string
		err := database.Read.QueryRowContext(ctx,
			`SELECT role FROM users WHERE id = $1`, id).Scan(&role)
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		return role, true, nil
	}
}

// NewWebsiteAccessLookup answers "may this person enter this website?" for the
// access middleware.
//
// One short query per request that names a website. An administrator passes
// without a second one: the role is the right to run the installation, and a
// site an administrator may not enter would be a site nobody could repair.
//
// No assignment means every website — see the migration. That is what keeps
// this invisible for an installation that never uses it, and it is why the
// query counts rather than fetches.
func NewWebsiteAccessLookup(database *db.DB) auth.WebsiteAccess {
	return func(ctx context.Context, userID, websiteID int64) bool {
		var role string
		if err := database.Read.QueryRowContext(ctx,
			`SELECT role FROM users WHERE id = $1`, userID).Scan(&role); err != nil {
			return false
		}
		if role == user.RoleAdmin {
			return true
		}
		var assigned, mine int
		if err := database.Read.QueryRowContext(ctx, `SELECT
			(SELECT COUNT(*) FROM user_websites WHERE user_id = $1),
			(SELECT COUNT(*) FROM user_websites WHERE user_id = $1 AND website_id = $2)`,
			userID, websiteID).Scan(&assigned, &mine); err != nil {
			return false
		}
		return assigned == 0 || mine > 0
	}
}

// NewLanguageLookup answers "which language does this person read?" for the
// language middleware.
//
// One short query on every admin request, deliberately not GetByID: that would
// pull the password hash through the connection on every page view to read one
// column. An unknown or signed-out user yields the empty string, and the
// browser's own wish decides.
func NewLanguageLookup(sm *scs.SessionManager, database *db.DB) func(*http.Request) string {
	return func(r *http.Request) string {
		id := sm.GetInt64(r.Context(), "user_id")
		if id == 0 {
			return ""
		}
		var locale string
		if err := database.Read.QueryRowContext(r.Context(),
			`SELECT locale FROM users WHERE id = $1`, id).Scan(&locale); err != nil {
			return ""
		}
		return locale
	}
}

// ErrHandler wraps a handler function that returns an error into an http.HandlerFunc.
// On error, it logs with slog.Error and returns a 500 status.
func (h *Handler) ErrHandler(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			slog.Error("handler error", "err", err, "path", r.URL.Path, "method", r.Method)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// SetPlugins supplies the plugin manager.
//
// Set after construction rather than passed in: the manager needs a runtime
// that needs a store that needs the database, and threading all of it through
// a constructor that already takes seventeen arguments would make the wiring
// harder to read than the feature is worth. A nil manager means the admin
// simply has no plugin screens, which is the right behaviour for a build that
// was compiled without them.
func (h *Handler) SetPlugins(m *plugin.Manager) { h.plugins = m }

// SetMail attaches the outbox, for the same reason and with the same rule: nil
// means nothing is sent and every screen that could send says so plainly.
func (h *Handler) SetMail(q *mail.Queue) { h.mail = q }

// SetAITokens attaches the key store for the assistant connection.
func (h *Handler) SetAITokens(s *ai.Store) { h.aiTokens = s }

// Plugins returns the manager, or nil.
func (h *Handler) Plugins() *plugin.Manager { return h.plugins }
