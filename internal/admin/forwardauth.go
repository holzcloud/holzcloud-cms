package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
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
	ssoRefuseNoWebsiteGroup  = "no_website_group"
	ssoRefuseSyncFailed      = "rights_sync_failed"
)

// errSSOEmptyAddress is returned when provisioning is asked to create an
// account for an identity carrying no address.
//
// It is a sentinel rather than an ad-hoc errors.New for one reason worth
// stating, because the reason is a test and not a style. Three independent
// layers refuse the empty address today: step 4 of ForwardAuthSignIn, the guard
// in provisionSSOUser, and user.Store.Create's own "email is required". Remove
// any one of them and the other two still refuse, so no behavioural test can
// tell which of the three is doing the work — a guard whose removal nothing
// notices reads as tidiness and gets deleted by the next person tidying.
//
// Naming the error lets provisioning's own guard be asserted with errors.Is,
// so its removal is red on its own rather than only when all three go.
var errSSOEmptyAddress = errors.New("provision: an identity with no address")

// errSSONoWebsiteGroup is returned when an editor's groups map to no website
// this installation has been configured to know about.
//
// It is a sentinel for the same reason errSSOEmptyAddress is one: the refusal it
// names is observably identical to the other refusals from outside — no session,
// one auth.login_fail row, the password form — so without a name no test could
// say which branch produced it, and the caller could not give the server log a
// reason code that distinguishes an operator's misconfiguration from a database
// that stopped answering.
var errSSONoWebsiteGroup = errors.New("sync: no group maps to a configured website")

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
//  6. the rights, re-derived from the identity provider's groups on every
//     sign-in, so that the role reaching the session is the synchronised one and
//     never the stale one — and so that an editor whose groups map to no
//     configured website is refused rather than emptied;
//  7. the token rotation, before anything is put in the session;
//  8. the funnel.
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

		// 6. The rights, from the groups, every time.
		//
		// Before the rotation and before the funnel, because completeLogin is
		// handed a role and that role has to be the synchronised one: a session
		// carrying the role the account had *before* this request would let a
		// demotion at the identity provider take effect one sign-in late, which
		// is precisely what SSO-06 asks not to happen.
		role, err := h.syncRightsFromGroups(r.Context(), r, u, ident)
		if err != nil {
			// A refused sign-in, not a server error and not a status. The
			// commonest way to arrive here is an editor whose last website
			// group was removed at the identity provider — the operator meant
			// to take their access away, and the refusal is what that looks
			// like from here.
			reason := ssoRefuseSyncFailed
			if errors.Is(err, errSSONoWebsiteGroup) {
				reason = ssoRefuseNoWebsiteGroup
			}
			slog.Warn("forward auth rights synchronisation refused the sign-in",
				"err", err, "reason", reason, "user_id", u.ID, "username", ident.Username)
			h.refuseSSO(r, ident, email, reason)
			next.ServeHTTP(w, r)
			return
		}

		// 7. Rotate session ID BEFORE setting values (prevents session
		// fixation). A failed rotation is a refused sign-in and never a
		// sign-in on the old token.
		if err := h.sm.RenewToken(r.Context()); err != nil {
			slog.Error("forward auth renew session token", "err", err, "user_id", u.ID)
			next.ServeHTTP(w, r)
			return
		}

		// 8. The mark, then the funnel. The synchronised role and the stored
		// address, not the header's: the session and the protocol row must name
		// the account, and the header only claimed to.
		h.sm.Put(r.Context(), auth.SessionKeyViaSSO, true)
		h.completeLogin(r, u.ID, role, u.Email)

		// The original request continues rather than being redirected: the
		// session manager commits at the end of the request and the values are
		// already visible to the authentication middleware further in, so the
		// person lands on the page they asked for.
		next.ServeHTTP(w, r)
	})
}

// syncRightsFromGroups re-derives what a person may do from the groups the
// identity provider vouched for, and returns the role the session is to carry.
//
// It runs on every sign-in and not once at account creation, which is the whole
// of SSO-06: a demotion made in the directory has to take effect here at the
// next sign-in rather than at the next session expiry. auth.RequireAuth re-reads
// users.role from the database on every request, so a role written here is in
// force one request later.
//
// The returned error is a refused sign-in and never a server error. The caller
// turns it into refuseSSO plus a fall-through to the password form.
//
// Four traps live in this function and three of them pass a naive
// implementation's own tests. They are named at the places they bite.
func (h *Handler) syncRightsFromGroups(ctx context.Context, r *http.Request,
	u *user.User, ident *web.Identity) (string, error) {

	// Trap one, and the only one an ordinary test does not catch by accident.
	// ident.HasGroup compares whole elements of the pipe-separated header;
	// strings.Contains(rawHeader, "holzcloud-admins") — the obvious version — is
	// true for somebody whose only group is not-holzcloud-admins, and for
	// holzcloud-admins-x, and for anything with the name buried inside it.
	// internal/web/forwardauth.go wrote HasGroup for exactly this reason.
	//
	// Trap two is the empty configured group. strings.Split("", "|") is a slice
	// of one empty string, so a membership test against the naive result reports
	// that everybody is in the group named "" — splitGroups drops empty elements
	// and this condition refuses an empty configured name outright, because
	// HOLZCLOUD_SSO_ADMIN_GROUP has no default and an unset one must grant
	// administration to nobody rather than to everybody.
	//
	// And want is one of exactly two values. users.role carries a table-level
	// CHECK (role IN ('admin','editor')) at 00001:7; loosening a table-head
	// CHECK in SQLite means rebuilding a table with foreign-key children, so
	// there is no third role and this is not the phase that invents one (D-05).
	want := user.RoleEditor
	if h.cfg.SSOAdminGroup != "" && ident.HasGroup(h.cfg.SSOAdminGroup) {
		want = user.RoleAdmin
	}

	if want != u.Role {
		err := h.users.Update(ctx, u.ID, u.Name, u.Email, want)
		switch {
		case errors.Is(err, user.ErrLastAdmin):
			// The identity provider asked for a demotion this installation
			// cannot perform. Locking the last administrator out because a group
			// changed on somebody else's server is a worse outcome than a role
			// that lags one sign-in behind, so the sign-in continues with the
			// role the account still has — loudly, because nothing else would
			// ever say so.
			slog.Warn("forward auth cannot apply a demotion: this is the last administrator",
				"user_id", u.ID, "email", u.Email, "username", ident.Username,
				"role", u.Role, "requested_role", want)
			want = u.Role
		case err != nil:
			return "", fmt.Errorf("sync: set the role: %w", err)
		default:
			// One row per actual change, naming what it changed from and to.
			// The action is the existing one: entry.go says the names are the
			// filter contract and that a new name makes older rows unfindable
			// by a new filter, while metadata is free.
			h.LogActivity(r, activity.Entry{
				ActorEmail: u.Email,
				Action:     activity.ActionUserUpdate,
				EntityType: "user",
				EntityID:   u.ID,
				Metadata: map[string]any{
					"field": "role", "from": u.Role, "to": want, "via": "sso",
				},
			})
		}
	}

	// An administrator's assignment is left exactly as it is.
	//
	// user.Store.Rights returns Everything() for an administrator two statements
	// before it reads user_websites at all, so anything written there is
	// invisible — and an invisible write is a diff a later reader has to reason
	// about for nothing.
	if want == user.RoleAdmin {
		return want, nil
	}

	// The website half only runs where the operator configured it, and the
	// distinction is not a convenience.
	//
	// With HOLZCLOUD_SSO_WEBSITE_GROUPS unset there is no mapping from a group
	// to a website, so "your groups match no configured website" is true of
	// everybody and says nothing about anybody. Refusing on it would lock every
	// editor out of single sign-on the moment SSO-06 shipped — including the
	// account plan 10-04's provisioning had just created and correctly assigned
	// one line earlier — and it would do so silently, because a refusal here
	// looks exactly like the ordinary password form.
	//
	// Leaving the assignment alone is safe in the way that matters: the rule
	// this function must never break is that it does not *write* an empty
	// assignment, and writing nothing cannot. An account that already had no
	// rows keeps none and keeps whatever it could reach before; no SSO path
	// creates that state, because provisioning writes its one row in the same
	// function that creates the account (D-01).
	//
	// Once the operator has configured even one group=website pair they have
	// opted into group-driven website access, and from then on "no matching
	// group" is an answer rather than an absence of a question.
	if len(h.cfg.SSOWebsiteGroups) == 0 {
		return want, nil
	}

	// A group this installation has never heard of is ignored rather than
	// refused: a directory carries groups that have nothing to do with this
	// program, and that is normal.
	ids := make([]int64, 0, len(ident.Groups))
	seen := make(map[int64]bool, len(ident.Groups))
	for _, group := range ident.Groups {
		id, ok := h.cfg.SSOWebsiteGroups[group]
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	// SetRights sorts and deduplicates again; doing it here as well is what
	// makes the comparison against the stored assignment below reliable, since
	// user.Store.Rights returns its rows ORDER BY website_id.
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	if len(ids) == 0 {
		// An empty assignment is not "no websites". NewWebsiteAccessLookup
		// (internal/admin/handler.go) reads assigned == 0 as *every* website,
		// and it is right to — see internal/user/rights.go, whose package
		// comment explains that anything else would have made the migration
		// introducing website assignment lock everybody out.
		//
		// D-01 closes that road for account *creation*: a provisioned account
		// gets its one row in the same function that creates it. This is the
		// same door reached from the other side, because SSO-06 re-applies the
		// rights on every sign-in. An operator who removes somebody's last
		// website group at the identity provider means to take their access
		// away, and writing an empty list would hand them everything —
		// D-01's inversion arrived at by subtraction instead of by creation.
		//
		// So nothing is written and the sign-in is refused. The existing rows
		// survive untouched: a refusal that also emptied the assignment would
		// be this very bug wearing a different hat.
		return "", errSSONoWebsiteGroup
	}

	current, err := h.users.Rights(ctx, u.ID)
	if err != nil {
		return "", fmt.Errorf("sync: read the current rights: %w", err)
	}

	if !sameWebsites(current.Websites, ids) {
		// Trap four. MayPublish is read and carried through, never derived: no
		// group grants the publishing right, and an operator who decided that
		// somebody submits rather than publishes made that decision about a
		// person. A literal here would undo it on a schedule.
		if err := h.users.SetRights(ctx, u.ID,
			user.Rights{MayPublish: current.MayPublish, Websites: ids}); err != nil {
			return "", fmt.Errorf("sync: set the website assignment: %w", err)
		}
		// Only on a difference, and that is what keeps /admin/protokoll
		// readable: SetRights is idempotent, so calling it every time would be
		// harmless and logging every time would not.
		h.LogActivity(r, activity.Entry{
			ActorEmail: u.Email,
			Action:     activity.ActionUserUpdate,
			EntityType: "user",
			EntityID:   u.ID,
			Metadata: map[string]any{
				"field": "websites", "from": current.Websites, "to": ids, "via": "sso",
			},
		})
	}

	return want, nil
}

// sameWebsites reports whether two sorted assignments are the same.
//
// Both sides arrive sorted — user.Store.Rights reads ORDER BY website_id and the
// caller sorts what it collected — so this is an element-wise comparison and not
// a set comparison, which is what makes an unchanged sign-in write nothing.
func sameWebsites(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
		return nil, errSSOEmptyAddress
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
