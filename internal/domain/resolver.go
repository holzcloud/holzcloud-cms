package domain

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
)

// Resolver resolves incoming HTTP requests to a Website via Host header lookup.
// It uses a sync.Map cache to avoid database queries on every request.
type Resolver struct {
	cache sync.Map // domain string -> resolved
	store *Store

	// Secure decides the scheme of a canonical redirect. It mirrors the
	// deployment setting rather than X-Forwarded-Proto, for the same reason the
	// sitemap does: a header an untrusted client can set must not decide the
	// address the site publishes about itself.
	Secure bool

	// Offline renders the maintenance page for a deactivated website that is
	// configured for it. It is supplied by the public package, because a themed
	// response cannot be built here. A nil handler falls back to 404.
	Offline http.Handler
}

// resolved is one cache entry: the website plus the canonical host, which is a
// second query and must not be repeated per request.
type resolved struct {
	website *Website
	primary string
}

// NewResolver creates a new domain resolver.
func NewResolver(store *Store) *Resolver {
	return &Resolver{store: store}
}

// Middleware returns an http.Handler that resolves the Host header to a Website
// and stores it in the request context.
//
// An unknown host is a 404. A deactivated one is a 404 or, when the operator
// chose maintenance mode, a 503 — a 404 tells a search engine the pages are
// gone for good, which is the wrong thing to say during a week of rebuilding.
func (res *Resolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := stripPort(r.Host)

		entry, ok := res.lookup(r.Context(), host)
		if !ok {
			http.NotFound(w, r)
			return
		}

		ctx := WebsiteToContext(r.Context(), entry.website)
		r = r.WithContext(ctx)

		if !entry.website.Active {
			res.serveOffline(w, r, entry.website)
			return
		}

		// The redirect lives here rather than in the mux so it also covers
		// /sitemap.xml, /robots.txt and the template assets under /t/.
		if entry.website.CanonicalRedirect && entry.primary != "" && !strings.EqualFold(host, entry.primary) {
			target := res.scheme() + "://" + entry.primary + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// lookup returns the cached or freshly queried entry for a host.
func (res *Resolver) lookup(ctx context.Context, host string) (resolved, bool) {
	if cached, ok := res.cache.Load(host); ok {
		if entry, ok := cached.(resolved); ok {
			return entry, true
		}
	}

	website, err := res.store.LookupDomain(ctx, host)
	if err != nil {
		slog.Error("domain lookup failed", "host", host, "error", err)
		return resolved{}, false
	}
	if website == nil {
		return resolved{}, false
	}

	primary, err := res.store.PrimaryDomain(ctx, website.ID)
	if err != nil {
		// A missing canonical host costs a redirect, not the request.
		slog.Error("primary domain lookup failed", "website_id", website.ID, "error", err)
	}

	entry := resolved{website: website, primary: primary}
	res.cache.Store(host, entry)
	return entry, true
}

// serveOffline answers for a deactivated website.
func (res *Resolver) serveOffline(w http.ResponseWriter, r *http.Request, website *Website) {
	if !website.InMaintenance() || res.Offline == nil {
		http.NotFound(w, r)
		return
	}
	res.Offline.ServeHTTP(w, r)
}

func (res *Resolver) scheme() string {
	if res.Secure {
		return "https"
	}
	return "http"
}

// CanonicalBase returns the scheme and host a website publishes about itself,
// e.g. "https://example.de". It falls back to the requested host when no
// primary domain is set, so a canonical URL is never empty or wrong.
func (res *Resolver) CanonicalBase(ctx context.Context, website *Website, requestHost string) string {
	primary, err := res.store.PrimaryDomain(ctx, website.ID)
	if err != nil || primary == "" {
		primary = stripPort(requestHost)
	}
	return res.scheme() + "://" + primary
}

// InvalidateCache clears all cached domain lookups.
//
// It must be called after any domain CRUD *and* after a website is renamed,
// deactivated or reconfigured: the whole Website struct is cached, so a stale
// entry keeps serving the old settings until the process restarts.
func (res *Resolver) InvalidateCache() {
	res.cache.Range(func(key, value any) bool {
		res.cache.Delete(key)
		return true
	})
}

// stripPort removes the port from a host:port string.
// If no port is present, returns the host as-is.
func stripPort(hostPort string) string {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		// No port present, return as-is
		return hostPort
	}
	return host
}
