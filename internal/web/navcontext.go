package web

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// What the navigation needs to know before a handler runs.
//
// The sidebar wants two things on every screen: which websites exist, so the
// switcher in the top bar can offer them, and which one the section links
// should point at. Neither is the business of the thirty handlers that render
// admin pages, so it is collected once, here, and read out of the context by
// NewLayoutData.
//
// The alternative — a parameter through every handler — was the reason the old
// sidebar only showed a website's sections on the screens that happened to know
// about one. Coming from the users screen, the whole content menu vanished.

type navKey int

const (
	keyWebsites navKey = iota
	keyNavWebsite
	keyPluginLinks
)

// NavLink is a plugin's own screen as the sidebar shows it.
//
// Declared here rather than taken from the plugin package so that this package
// keeps knowing nothing about plugins — and so that a build without them has
// nothing to strip out.
type NavLink struct {
	Label     string
	URL       string
	AdminOnly bool
}

// sessionLastWebsite remembers what someone was working on. It is what makes
// "Seiten" still mean something after a detour through the user list.
const sessionLastWebsite = "nav_website"

// WithNav loads the website list and works out which website the section links
// belong to.
//
// Order of preference: the website named in the address, then the one from the
// last visit, then simply the first. The last two are conveniences; the address
// always wins, because that is what the person actually clicked.
// plugins may be nil, and then there are simply no plugin entries.
func WithNav(sm *scs.SessionManager, list func(context.Context) ([]domain.Website, error), plugins func(websiteID int64) []NavLink) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if list == nil {
				next.ServeHTTP(w, r)
				return
			}
			websites, err := list(r.Context())
			if err != nil {
				// Navigation is not worth a failed request: without the list the
				// switcher is empty and every screen still works.
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), keyWebsites, websites)

			id := websiteFromPath(r.URL.Path)
			if id > 0 {
				sm.Put(ctx, sessionLastWebsite, id)
			} else {
				id = sm.GetInt64(ctx, sessionLastWebsite)
			}

			ws := pick(websites, id)
			if ws != nil {
				ctx = context.WithValue(ctx, keyNavWebsite, ws)
			}
			if plugins != nil {
				var forSite int64
				if ws != nil {
					forSite = ws.ID
				}
				ctx = context.WithValue(ctx, keyPluginLinks, plugins(forSite))
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// websiteFromPath reads the id out of /admin/websites/{id}/…
//
// Parsed from the path rather than taken from r.PathValue because this runs
// before the router has matched anything. The path is the honest source either
// way: it is where the website is named.
func websiteFromPath(path string) int64 {
	rest, ok := strings.CutPrefix(path, "/admin/websites/")
	if !ok {
		return 0
	}
	seg, _, _ := strings.Cut(rest, "/")
	id, err := strconv.ParseInt(seg, 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

func pick(websites []domain.Website, id int64) *domain.Website {
	for i := range websites {
		if websites[i].ID == id {
			return &websites[i]
		}
	}
	if len(websites) > 0 {
		return &websites[0]
	}
	return nil
}

// WebsitesFrom returns the list put there by WithNav.
func WebsitesFrom(ctx context.Context) []domain.Website {
	list, _ := ctx.Value(keyWebsites).([]domain.Website)
	return list
}

// NavWebsiteFrom returns the website the section links point at, or nil.
func NavWebsiteFrom(ctx context.Context) *domain.Website {
	ws, _ := ctx.Value(keyNavWebsite).(*domain.Website)
	return ws
}

// PluginLinksFrom returns the plugin screens for the sidebar.
func PluginLinksFrom(ctx context.Context) []NavLink {
	links, _ := ctx.Value(keyPluginLinks).([]NavLink)
	return links
}
