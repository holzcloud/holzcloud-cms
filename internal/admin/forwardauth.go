package admin

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/user"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// Turning the proxy's claim about a person into a signed-in session.
//
// internal/web/forwardauth.go decides whether a claim may be believed at all;
// by the time this file runs, that question is settled and the identity is in
// the request context. What is left is the second question, and it is a
// different one: which account in this installation is that person, and how
// does the session come to say so.
//
// The answer goes through the funnel the password form goes through and adds
// nothing of its own. A middleware that put user_id into the session itself
// would work perfectly and would be invisible in /admin/protokoll — and the
// protocol is the one place an operator looks when something changed and
// nobody admits to it.

// The reasons a forward-auth sign-in is refused. They are codes for the server
// log and never sentences on a screen: nobody is shown a refusal here, they are
// shown the ordinary login form.
const (
	ssoRefuseNoEmail         = "no_email"
	ssoRefuseNonASCII        = "non_ascii_email"
	ssoRefuseNoAccount       = "no_account"
	ssoRefuseProvisionFailed = "provisioning_failed"
)

// ForwardAuthSignIn signs in the account the proxy's identity names.
//
// It is a method on *Handler and not a free function taking its dependencies,
// for one reason that is not convenience. The sign-in funnel is unexported, and
// this path is required to go through it; a free function would have to export
// it or write it a second time, and writing it a second time is how the
// protocol row goes missing.
//
// completeLogin is the single place a session becomes signed in, and after this
// file it has four callers: the password form, the second factor, first-run
// setup, and here.
//
// The order of the body is the design, not a sequence that happened:
//
//  1. the switch, so that with single sign-on off nothing below runs at all;
//  2. the identity, from the context and never from a header;
//  3. an already signed-in session, which is left exactly as it is;
//  4. the address rule, which refuses rather than guesses;
//  5. the account lookup, and — only if the operator switched it on — the
//     creation of the account it did not find, together with the one website
//     assignment that account is allowed;
//  6. the token rotation, before anything is put in the session;
//  7. the funnel.
//
// It writes no status of its own on any path and calls the next handler exactly
// once. A refusal is the ordinary password form, because the way back into the
// admin must not die with the proxy — and it is deliberately not a 403.
//
// It also does not authorise anything. RequireAuth, RequireSecondFactor,
// RequireAdmin and RequireWebsiteAccess all run behind it, unchanged, on the
// session it wrote; removing this one call from the chain is how the whole
// feature is switched off in an emergency.
func (h *Handler) ForwardAuthSignIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. The switch. SSO-09's first line of defence and the cheapest one.
		if h.cfg == nil || !h.cfg.SSOEnabled {
			next.ServeHTTP(w, r)
			return
		}

		// 2. The identity arrives in the context. It cannot arrive in a header:
		// the middleware in internal/web deletes every spelling of every
		// identity header on every path, so there is nothing left to read even
		// for code that tried.
		ident, ok := web.IdentityFromContext(r.Context())
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		// 3. A session that is already signed in is left exactly as it is.
		//
		// This is not an optimisation. Without it every request would rotate
		// the token and write a sign-in row, and /admin/protokoll would become
		// one row per page view; and somebody signed in by password on a shared
		// machine would be silently swapped for whoever the proxy last
		// asserted.
		if h.sm.GetInt64(r.Context(), auth.SessionKeyUserID) != 0 {
			next.ServeHTTP(w, r)
			return
		}

		// 4. The address rule.
		//
		// The identity is X-authentik-username and the account key is
		// users.email. They are two different things, and the reason is that
		// the users schema is on this phase's deliberately-unchanged list and
		// email is the only unique key it offers.
		//
		// That column is declared UNIQUE COLLATE NOCASE, and SQLite's NOCASE
		// folds ASCII only. Go's strings.ToLower folds all of Unicode. Where
		// the two disagree the database considers Müller@example.com and
		// müller@example.com to be two distinct rows — two accounts, and in the
		// worst case two administrators — while a Go-side match would happily
		// pick one of them. So an address carrying any byte above 0x7F is
		// refused with a named reason instead of matched. Folding it by hand
		// would be a second definition of identity standing beside the
		// database's own, which is exactly the disagreement being avoided.
		email := strings.ToLower(strings.TrimSpace(ident.Email))
		if email == "" {
			h.refuseSSO(r, ident, email, ssoRefuseNoEmail)
			next.ServeHTTP(w, r)
			return
		}
		if !isASCII(email) {
			h.refuseSSO(r, ident, email, ssoRefuseNonASCII)
			next.ServeHTTP(w, r)
			return
		}

		// 5. The account. The lookup itself creates nothing; whether the
		// absence of a row becomes a refusal or a new account is decided by
		// the branch below it, and by one setting that is off by default.
		u, err := h.users.GetByEmail(r.Context(), email)
		if err != nil {
			// A database that cannot answer must not turn into a sign-in. No
			// protocol row is attempted here: the store that would write it
			// sits on the same database that just failed to answer.
			slog.Error("forward auth account lookup", "err", err, "username", ident.Username)
			next.ServeHTTP(w, r)
			return
		}
		if u == nil {
			// 5b. Provisioning, and the one seam this whole plan exists for.
			//
			// NewWebsiteAccessLookup (internal/admin/handler.go) ends with
			//
			//     return assigned == 0 || mine > 0
			//
			// "no assignment means every website". That is deliberate and it is
			// right: internal/user/rights.go says in its package comment that
			// anything else would have made the migration which introduced
			// website assignment lock everybody out, and it is why an
			// installation that never uses the feature never has to know it
			// exists. It is correct for every account an operator creates by
			// hand.
			//
			// It inverts the moment accounts are created automatically,
			// because a freshly created account has zero rows in user_websites
			// *by construction*. Without an assignment written for it, the
			// first stranger the identity provider authenticates becomes an
			// editor of every website in the installation.
			//
			// NewWebsiteAccessLookup is not changed to fix this. Changing it
			// would lock out every existing editor, which is the same failure
			// arriving from the other side. Three things stand in its place:
			// provisioning is off unless the operator switched it on; with it
			// on, the process refuses to start without a default website that
			// exists; and the account is given that website in the same
			// function that creates it.
			//
			// That last one is why the assignment is written here rather than
			// left to the group synchronisation of plan 10-05. Between a
			// created account and its first assignment there must be no
			// request, no error path and no later plan — an account that
			// exists with zero rows in user_websites is the vulnerability
			// itself, for however long it exists. The two lines cannot be
			// separated, which is what provisionSSOUser is for.
			if !h.cfg.SSOProvision {
				h.refuseSSO(r, ident, email, ssoRefuseNoAccount)
				next.ServeHTTP(w, r)
				return
			}
			u, err = h.provisionSSOUser(r, ident, email)
			if err != nil {
				// A refusal, not a status, and not a half-made account: see
				// the compensation in provisionSSOUser.
				//
				// This branch does write a protocol row, unlike the lookup
				// failure above it. The commonest way to arrive here is not a
				// database that has stopped answering but a
				// HOLZCLOUD_SSO_DEFAULT_WEBSITE naming a website that has since
				// been deleted — the database is healthy and the operator needs
				// to see that somebody was turned away. Store.Log never returns
				// an error by design, so even in the other case this cannot
				// turn a refusal into a 500.
				slog.Error("forward auth provisioning", "err", err, "username", ident.Username)
				h.refuseSSO(r, ident, email, ssoRefuseProvisionFailed)
				next.ServeHTTP(w, r)
				return
			}
		}

		// 6. Rotate session ID BEFORE setting values (prevents session
		// fixation). A failed rotation is a refused sign-in and never a
		// sign-in on the old token.
		if err := h.sm.RenewToken(r.Context()); err != nil {
			slog.Error("forward auth renew session token", "err", err, "user_id", u.ID)
			next.ServeHTTP(w, r)
			return
		}

		// 7. The mark, then the funnel. The stored role and the stored address,
		// not the header's: the session and the protocol row must name the
		// account, and the header only claimed to.
		h.sm.Put(r.Context(), auth.SessionKeyViaSSO, true)
		h.completeLogin(r, u.ID, u.Role, u.Email)

		// The original request continues rather than being redirected: the
		// session manager commits at the end of the request and the values are
		// already visible to the authentication middleware further in, so the
		// person lands on the page they asked for.
		next.ServeHTTP(w, r)
	})
}

// provisionSSOUser creates the account an identity names and gives it the one
// website it is allowed into, in that order and in the same request.
//
// It returns an error rather than writing anything to the response: every
// failure here is a refused sign-in that falls through to the password form,
// and none of them is a partially created account.
func (h *Handler) provisionSSOUser(r *http.Request, ident *web.Identity, email string) (*user.User, error) {
	ctx := r.Context()

	// The empty address is refused here as well as at step 4, and this is not
	// belt and braces.
	//
	// users.email is declared NOT NULL UNIQUE COLLATE NOCASE and nothing in
	// that declaration forbids the empty string, so an account row carrying one
	// is legal. Wave 3 proved what that means for the sign-in path: without its
	// guard an identity with no e-mail header is not refused but looked up, and
	// it matches such a row. Creating one here would mint exactly that account,
	// and the next identity arriving without an e-mail header would sign in as
	// it. The guard belongs to whichever function could create the row, which
	// is this one.
	if email == "" {
		return nil, errors.New("provision: an identity with no address")
	}

	// A value nobody will ever know, existing only so that users.password can
	// hold an Argon2id hash of something. 00001:6 declares password TEXT NOT
	// NULL with no CHECK, which is the whole reason this needs no migration.
	// It is not returned, not stored anywhere else, and not written to any log
	// at any level: nobody chose a password for this account, so nobody and
	// nothing may learn one.
	secret, err := randomSecret()
	if err != nil {
		return nil, fmt.Errorf("provision: %w", err)
	}

	// An editor, always.
	//
	// Which role a group grants is plan 10-05's decision, made again on every
	// sign-in so that a demotion at the identity provider takes effect here.
	// Creating an administrator would pre-empt that decision with the one role
	// NewWebsiteAccessLookup answers before it counts any assignment at all —
	// the first stranger through the door would skip the entire defence the
	// branch above describes.
	id, err := h.users.Create(ctx, ident.Name, email, secret, user.RoleEditor)
	if errors.Is(err, user.ErrDuplicateEmail) {
		// Two requests for one new identity is a race, not an error. The
		// database resolved it — users.email is UNIQUE COLLATE NOCASE — and the
		// honest answer is the row that won.
		existing, readErr := h.users.GetByEmail(ctx, email)
		if readErr != nil {
			return nil, fmt.Errorf("provision: re-read after a duplicate address: %w", readErr)
		}
		if existing == nil {
			return nil, errors.New("provision: the address is taken and no account carries it")
		}
		return existing, nil
	}
	if err != nil {
		return nil, fmt.Errorf("provision: create the account: %w", err)
	}

	// The line the plan exists for. MayPublish matches what a hand-created
	// editor gets by default; the single-element slice is what stops the count
	// above from being zero.
	rights := user.Rights{MayPublish: true, Websites: []int64{h.cfg.SSODefaultWebsite}}
	if err := h.users.SetRights(ctx, id, rights); err != nil {
		// The compensation, and it is not optional. An account that exists with
		// no assignment is precisely the state the branch above describes, so
		// leaving one behind after a failed second statement would create the
		// vulnerability by accident rather than by design. The ordinary delete
		// path is the compensation, as elsewhere in this codebase.
		if delErr := h.users.Delete(ctx, id); delErr != nil {
			// The one case an operator has to repair by hand, so the line names
			// the row: an account with no website assignment may enter every
			// website there is.
			slog.Error("forward auth could not remove a half-provisioned account",
				"err", delErr, "user_id", id, "username", ident.Username)
		}
		return nil, fmt.Errorf("provision: assign the default website: %w", err)
	}

	// An account came into existence without a person asking for one. That
	// belongs in the server log and in the protocol beside every hand-made
	// account, which is why the action is the ordinary one and only the
	// metadata says where it came from: entry.go's comment says the action
	// names are the filter contract and that a new one makes older rows
	// unfindable, while metadata is free.
	slog.Info("forward auth provisioned an account",
		"user_id", id, "email", email, "website_id", h.cfg.SSODefaultWebsite,
		"username", ident.Username)
	h.LogActivity(r, activity.Entry{
		ActorEmail: email,
		Action:     activity.ActionUserCreate,
		EntityType: "user",
		EntityID:   id,
		Metadata:   map[string]any{"via": "sso"},
	})

	u, err := h.users.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("provision: read back the account: %w", err)
	}
	if u == nil {
		return nil, errors.New("provision: the account was created and cannot be read back")
	}
	return u, nil
}

// randomSecret returns 32 bytes from crypto/rand, base64 raw-URL encoded.
//
// 43 characters, comfortably above auth.MinPasswordLength, which
// user.Store.Create enforces. Its only purpose is to be hashed and forgotten in
// the same statement.
func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// refuseSSO records a forward-auth sign-in that did not happen.
//
// One line in the server log naming the identity, the reason and the client
// address, and one row in the protocol carrying the attempted address — the
// same shape the password form's failure branch writes, and for the reason its
// comment gives: at a failed attempt there is no account, and the attempted
// address is the thing one wants to recognise later.
//
// It deliberately does not touch the login throttle. That limiter keys on an
// address plus an account and exists to slow password guessing; a forward-auth
// request carries no password to guess, and feeding it would let a stranger at
// the identity provider lock a real account out of the password form.
func (h *Handler) refuseSSO(r *http.Request, ident *web.Identity, email, reason string) {
	var ip string
	if h.clientIP != nil {
		ip = h.clientIP.ClientIP(r)
	}
	slog.Warn("forward auth sign-in refused",
		"username", ident.Username, "reason", reason, "ip", ip)

	h.LogActivity(r, activity.Entry{
		ActorEmail: email,
		Action:     activity.ActionAuthLoginFail,
		EntityType: "user",
	})
}

// isASCII reports whether every byte of s is below 0x80.
//
// The whole of the case-folding trap above rests on this one line: within ASCII
// the database's NOCASE and Go's ToLower agree about which addresses are the
// same account, and outside it they do not.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
