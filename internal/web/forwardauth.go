package web

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
)

// Forward authentication: believing a reverse proxy's claim about who is at the
// other end.
//
// In this arrangement the operator's identity provider — Authentik — signs
// somebody in, and its outpost tells the reverse proxy who that is. The proxy
// copies the answer onto the request it forwards here, as a small set of
// headers named X-authentik-username, -email, -name, -uid, -groups,
// -entitlements, -jwt and -meta-*. Nothing about those headers is
// cryptographically bound to this request; they are ordinary header lines, and
// anything that can open a socket to this program can write them.
//
// That is a kind of header this codebase had not read before. It already reads
// two others and treats them very differently. The bearer token on /ai is a
// secret the server verifies for itself, so its worth does not depend on who
// sent it. X-Forwarded-For and X-Request-ID are values whose only guarantee is
// who the peer is, so RequestID adopts one solely from a trusted proxy. A
// forward-auth header is neither: it is a claim about identity, and the only
// thing standing behind it is the socket it arrived on. This file is the first
// place in this program that believes such a claim, and it is written so that
// the believing is narrow.
//
// Four layers, in this order, each covering a different failure of the one
// before it. Layer 1 is the peer address, read from the accepted connection and
// therefore not choosable by a client; it stands to the left of every header
// read, so an untrusted peer's claim is never even fetched. Layer 2 deletes
// every inbound identity header on every path, whoever the peer was, so a
// reverse proxy that forwards a client's own copy — which CVE-2026-30851 makes
// the default on Caddy 2.10.0 through 2.11.1 — is a misconfiguration on someone
// else's server rather than a bypass here. Layer 3 is a shared secret the proxy
// adds and this program compares in constant time, so a mistake in layer 1 is
// not on its own enough. Layer 4 would be a signed assertion; it is
// deliberately not built, and the reason is written at the foot of this file so
// it is not relitigated.
//
// The middleware never writes a status of its own and calls the next handler
// exactly once on every path. An untrusted peer carrying no identity header
// reaches the ordinary password form, because the way back into the admin must
// not die with the proxy.

// ProxySecretHeader carries the secret that proves the request came through
// this installation's own reverse proxy.
//
// The name is this installation's and not the identity provider's on purpose: a
// header the proxy adds must be distinguishable from one it merely copies, and
// this secret has nothing to do with Authentik.
const ProxySecretHeader = "X-Holzcloud-Proxy-Secret"

// identityHeaderPrefix is the normalised prefix every identity header shares.
// Normalised means lowercased with underscores folded to hyphens; see
// stripIdentityHeaders for why that matters.
const identityHeaderPrefix = "x-authentik-"

// groupSeparator is what Authentik joins group names with. Verified from its
// own source — getHeaders in internal/outpost/proxyv2/application/mode_common.go
// at tags version/2026.8.1 and version-2025.8 — and not from a blog post.
const groupSeparator = "|"

// Identity is what the proxy says about the person at the other end.
//
// Username comes from X-authentik-username, Email from X-authentik-email, Name
// from X-authentik-name and Groups from X-authentik-groups.
//
// The identity is pinned to the username and deliberately not to
// X-authentik-uid. That header carries the OIDC subject, whose shape depends on
// the provider's Subject mode and defaults to a hashed identifier, so an
// installation that changes the mode would silently acquire a second population
// of users. X-authentik-uid is read into nothing here; it is stripped like
// every other identity header.
type Identity struct {
	Username string
	Email    string
	Name     string
	Groups   []string
}

// HasGroup reports membership by comparing whole elements.
//
// The form it replaces is strings.Contains over the raw header, which answers
// yes about holzcloud-admins when the person is only in not-holzcloud-admins.
// The function exists so that no caller has to remember that.
func (i Identity) HasGroup(name string) bool {
	for _, group := range i.Groups {
		if group == name {
			return true
		}
	}
	return false
}

// ForwardAuthOptions is the whole configuration this middleware needs.
//
// Two fields, so a test constructs one in a line — and so this package does not
// import internal/config, which would make it unconstructible without one.
type ForwardAuthOptions struct {
	Enabled bool
	Secret  string
}

var identityKey = &contextKey{"forward-auth-identity"}

// IdentityFromContext returns the identity the proxy vouched for, if any.
//
// A context that never went through the middleware answers nil, false.
func IdentityFromContext(ctx context.Context) (*Identity, bool) {
	ident, ok := ctx.Value(identityKey).(*Identity)
	return ident, ok && ident != nil
}

// ForwardAuth reads the proxy's claim about identity, and strips it either way.
//
// The order of the terms in the guard below is the requirement, not a style
// choice. Go evaluates && from left to right and stops at the first false, so
// with the trusted-peer check standing to the left of secretMatches, an
// untrusted peer's headers are never fetched at all — not the secret, and not
// one identity header. Moving the peer check right of any header read would
// leave the middleware behaving identically and the property gone, which is why
// there is a test that reads this function's own source.
//
// The identity leaves here in the request context and never in a header. That
// is what stops a handler further in from reading a claim about identity by
// accident, and the strip below is what makes it true rather than a convention.
func ForwardAuth(resolver *ClientIPResolver, opts ForwardAuthOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var ident *Identity
			if opts.Enabled && opts.Secret != "" && resolver != nil && resolver.IsTrustedPeer(r) && secretMatches(r, opts.Secret) {
				// Header.Get and never map indexing: Get canonicalises the
				// name it is given, indexing believes whatever spelling the
				// caller typed.
				username := strings.TrimSpace(r.Header.Get("X-authentik-username"))
				if username != "" {
					// An identity with no username is not an identity: it is
					// the one field everything downstream keys on.
					ident = &Identity{
						Username: username,
						Email:    strings.TrimSpace(r.Header.Get("X-authentik-email")),
						Name:     strings.TrimSpace(r.Header.Get("X-authentik-name")),
						Groups:   splitGroups(r.Header.Get("X-authentik-groups")),
					}
				}
			}

			// Layer 2, and its whole value is that it has no condition.
			stripIdentityHeaders(r)

			if ident != nil {
				r = r.WithContext(context.WithValue(r.Context(), identityKey, ident))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// stripIdentityHeaders deletes every inbound identity header, and the shared
// secret with them.
//
// It scans the header map's own keys rather than deleting a list of names, and
// that is the guarantee; the header names written in the doc comments of this
// file are documentation. Go canonicalises hyphens but not underscores, so
// X-authentik-email and X_authentik_email arrive as two distinct map keys —
// CVE-2026-52845 / GHSA-f59h-q822-g45g is the advisory that made this a
// published problem rather than a theoretical one. A fixed list of hyphenated
// names deletes one of that pair and leaves the other, and it is a list
// somebody has to keep in step with a program running on another machine.
//
// The secret is deleted for its own reason: a handler must not be able to read
// the secret it is being protected by out of the request it is serving.
//
// The keys are collected before anything is deleted, because deleting from a
// map while ranging over it is a thing to get right rather than to find out
// about.
func stripIdentityHeaders(r *http.Request) {
	var doomed []string
	for name := range r.Header {
		if isIdentityHeader(name) {
			doomed = append(doomed, name)
		}
	}
	for _, name := range doomed {
		// delete and not Header.Del: Del canonicalises the name it is given
		// and would not remove the underscore spelling.
		delete(r.Header, name)
	}
}

func isIdentityHeader(name string) bool {
	normalised := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	return strings.HasPrefix(normalised, identityHeaderPrefix) ||
		normalised == strings.ToLower(ProxySecretHeader)
}

// secretMatches compares the proxy's secret with the configured one.
//
// Constant time hides the position of the first differing byte, which is what
// turns a comparison into an oracle. It does not hide the length, and that is
// accepted: the length of a secret the operator chose is not the secret.
//
// An empty configured secret is refused before this is ever called — the guard
// in ForwardAuth carries opts.Secret != "" — because ConstantTimeCompare of two
// empty slices returns 1, and a configuration that believes everybody must not
// be one character away.
func secretMatches(r *http.Request, want string) bool {
	return subtle.ConstantTimeCompare([]byte(r.Header.Get(ProxySecretHeader)), []byte(want)) == 1
}

// splitGroups turns X-authentik-groups into group names.
//
// The trap it exists for: strings.Split of an empty string on a separator
// yields a slice holding one empty string, so a membership test against the
// naive result would report that everybody is in the group named "". Empty
// elements are dropped and every element is trimmed.
func splitGroups(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, groupSeparator) {
		if group := strings.TrimSpace(part); group != "" {
			out = append(out, group)
		}
	}
	return out
}

// Layer 4 — the signed assertion, and why it is deliberately not built.
//
// Authentik can also pass a JWT in X-authentik-jwt. Verifying it means either
// fetching the provider's JWKS while this program is running — the one rule
// this project does not break — or pinning a key by hand; and where no signing
// key is configured, Authentik signs proxy tokens symmetrically with the client
// secret, so "verify it" is really two verification modes plus holding that
// secret in this program's configuration.
//
// It would buy nothing the transport does not already give. The signature says
// the identity provider issued the token; it does not say this request came
// from the proxy. Anybody who can set headers on this socket can talk to the
// CMS directly, and that is exactly what layers 1 and 3 address.
//
// And parsing the token without verifying its signature would be strictly worse
// than not having it at all, because it looks like a defence. X-authentik-jwt
// is therefore stripped like every other identity header and read by nothing.
// This paragraph is here, in the source, so that "we should parse the JWT" is
// answered where it will be proposed.
