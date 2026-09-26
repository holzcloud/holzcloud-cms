package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/oidc"
)

type Config struct {
	Port     string
	DataDir  string
	LogLevel string
	DBPath   string

	// Listen is the address the HTTP server binds. HOLZCLOUD_LISTEN, default
	// 127.0.0.1.
	//
	// The server has listened on every interface since it was written, which
	// is harmless while a password is required and a total bypass the moment a
	// header is believed. So the default moves to loopback and the operator
	// who wants the old behaviour says so.
	//
	// An address and not a boolean on purpose. deploy/DEPLOY.md publishes a
	// port into a container, and a process that binds loopback inside its own
	// network namespace answers nobody — those deployments set 0.0.0.0.
	Listen string

	// Templates
	MaxTemplateSize int64 // HOLZCLOUD_MAX_TEMPLATE_SIZE — bytes, default 10MB

	// Media
	MaxMediaSize int64 // HOLZCLOUD_MAX_MEDIA_SIZE — bytes, default 5MB
	// MaxVideoSize applies only to video files. Separate from MaxMediaSize,
	// because the five megabytes that are generous for a photo are not enough
	// for half a minute of film — and because an image of 60 MB is still a
	// mistake.
	MaxVideoSize int64 // HOLZCLOUD_MAX_VIDEO_SIZE — bytes, default 64MB

	// MaxMegapixels bounds what the variant pipeline will decode.
	// HOLZCLOUD_MAX_MEGAPIXELS, default 24.
	//
	// A byte limit is not a pixel limit: a five-megabyte single-colour PNG may
	// legally be 20000×20000 and decodes to 1.6 GB of NRGBA, which on a node
	// with a few gigabytes ends with the kernel killing the process.
	MaxMegapixels int

	// Auth
	Secure            bool   // HOLZCLOUD_SECURE — controls cookie Secure flag and CSRF Secure flag
	Argon2Memory      uint32 // HOLZCLOUD_ARGON2_MEMORY — KB, default 65536 (64MB)
	Argon2Iterations  uint32 // HOLZCLOUD_ARGON2_ITERATIONS — default 1
	Argon2Parallelism uint8  // HOLZCLOUD_ARGON2_PARALLELISM — default 2

	// TrustedProxies are the peer addresses whose X-Forwarded-For header may be
	// believed. HOLZCLOUD_TRUSTED_PROXIES, comma-separated CIDR list.
	TrustedProxies []netip.Prefix

	// PayrexxInstance and PayrexxSecret enable online payment.
	// HOLZCLOUD_PAYREXX_INSTANCE, HOLZCLOUD_PAYREXX_SECRET. Both empty means
	// the shop offers invoice and prepayment only, which is a perfectly
	// complete shop — the payment provider is an addition, not a requirement.
	//
	// Deliberately not a setting in the admin interface. The database is what
	// gets copied into a backup file, and a payment key in a backup is a
	// payment key in every copy of that backup, on every disk it was ever
	// carried on. The environment is read once at startup and lives in the
	// service unit, where it can be given file permissions of its own.
	PayrexxInstance string
	PayrexxSecret   string

	// PayrexxBaseURL overrides the API address.
	// HOLZCLOUD_PAYREXX_BASE_URL, default the live API.
	//
	// There to make the payment plumbing testable against a stand-in before
	// anyone has keys. Point it anywhere else in production and the API secret
	// is sent to whatever host is named — so it is left alone unless there is
	// a reason.
	PayrexxBaseURL string

	// MinFreeBytes is the free-space floor below which uploads are refused and
	// /readyz reports unready. HOLZCLOUD_MIN_FREE_BYTES, default 512 MB.
	//
	// Sessions are written to the same database on every authenticated request,
	// so a full disk breaks signing in — not just uploads.
	MinFreeBytes uint64

	// SMTP is the mail server, and it is off unless HOLZCLOUD_SMTP_HOST and
	// HOLZCLOUD_SMTP_FROM are both set.
	//
	// Off by default on purpose. Sending mail is the only thing this server
	// does that reaches outwards, and an operator who has not asked for it
	// should not discover one day that their CMS has been talking to a mail
	// relay. With it off, an invitation link is shown on screen exactly as
	// before.
	SMTPHost     string // HOLZCLOUD_SMTP_HOST
	SMTPPort     int    // HOLZCLOUD_SMTP_PORT, default 587
	SMTPUser     string // HOLZCLOUD_SMTP_USER
	SMTPPassword string // HOLZCLOUD_SMTP_PASSWORD
	SMTPFrom     string // HOLZCLOUD_SMTP_FROM — the sender address
	SMTPFromName string // HOLZCLOUD_SMTP_FROM_NAME — the display name
	// SMTPTLS is "starttls" (default), "tls", or "none".
	SMTPTLS string // HOLZCLOUD_SMTP_TLS

	// Single sign-on through a forward-auth proxy. Every field below is inert
	// while SSOEnabled is false: with the master switch off, not one line of
	// the new path executes and the password login is exactly what it was.
	//
	// SSOEnabled is the master switch, HOLZCLOUD_SSO_ENABLED, default false.
	SSOEnabled bool

	// SSOSecret is the shared secret the reverse proxy sends, and the only
	// thing that distinguishes it from anyone else who can reach the port.
	//
	// Environment only, for the reason already written above PayrexxSecret:
	// the database is what gets copied into a backup file. What differs here
	// is that this secret is compared, in constant time, on every request.
	SSOSecret string

	// SSOAdminGroup is the identity provider group that grants administration.
	//
	// It has no default, deliberately, and an empty value is a legal
	// configuration meaning no group grants administration. A default group
	// name is a group name somebody at the identity provider can create.
	SSOAdminGroup string

	// SSOWebsiteGroups maps an identity provider group to a website id.
	// A comma-separated list of group=websiteID pairs.
	//
	// An explicit map and not a name match: `websites` has no slug column,
	// only a display name, and matching a group against a display name means a
	// rename silently unassigns everybody.
	SSOWebsiteGroups map[string]int64

	// SSOProvision creates an account for an identity the provider vouches for
	// but this installation has never seen. Off unless asked for.
	SSOProvision bool

	// SSODefaultWebsite is the website a provisioned account is assigned to,
	// and it is required whenever SSOProvision is on.
	//
	// This is the one setting in the block whose silence would mean "every
	// website". internal/admin/handler.go's NewWebsiteAccessLookup ends with
	//
	//	return assigned == 0 || mine > 0
	//
	// "No assignment means every website" is correct for an account an
	// operator created by hand, and it inverts under provisioning: a freshly
	// provisioned account has zero rows in user_websites by construction, so
	// the first stranger who authenticates would get editor access to every
	// website in the installation. That line is not changed — changing it
	// would lock out every existing editor. The fix is at this end, which is
	// why the value is required rather than optional, and why cmd/holzcloud
	// checks it against the database at start-up rather than at the first
	// sign-in.
	SSODefaultWebsite int64

	// SSOSignOutPath is where the sign-out button sends the browser.
	//
	// A path on this server, never a URL: an absolute address here would be an
	// open redirect out of the administration, and a same-origin path is also
	// what keeps adminCSP's `form-action 'self'` sufficient.
	SSOSignOutPath string

	// Single sign-on through OpenID Connect, the second way in beside the
	// forward-auth proxy above. Every field below is inert while OIDCEnabled is
	// false.
	//
	// The flow is the one that asks nobody: the provider hands the browser a
	// signed ID token, the browser posts it back, and this program checks the
	// signature against a key it already has (internal/oidc says why). So there
	// is no discovery document, no token endpoint and no JWKS address here —
	// only the address the browser is sent to, and the key.
	//
	// The group settings above — SSOAdminGroup, SSOWebsiteGroups,
	// SSOProvision, SSODefaultWebsite — apply to this way in as well. They
	// describe what the identity provider's groups mean in this installation,
	// and that does not depend on how the groups arrived.
	OIDCEnabled bool
	// OIDCName is what the button on the sign-in form names, such as Authentik.
	OIDCName string
	// OIDCIssuer is compared with the token's iss claim, character for
	// character.
	OIDCIssuer string
	// OIDCAuthorizeURL is the provider's authorization endpoint, where the
	// browser is sent.
	OIDCAuthorizeURL string
	// OIDCClientID is compared with the token's audience.
	OIDCClientID string
	// OIDCClientSecret checks a token the provider signed with the client
	// secret (HS256). Environment only, like every other secret here.
	OIDCClientSecret string
	// OIDCKeys are the provider's public keys, read at start-up from the file
	// HOLZCLOUD_OIDC_KEY_FILE names: a JWKS document or PEM. Exactly one of
	// OIDCKeys and OIDCClientSecret is set.
	OIDCKeys []oidc.Key
	// OIDCKeyFile is where OIDCKeys came from, for the startup log.
	OIDCKeyFile string
	// OIDCRedirectURL is where the provider sends the browser back. It is
	// registered at the provider and has to match there exactly, so it is
	// named rather than assembled from a request's Host header.
	OIDCRedirectURL string
	// OIDCScopes are asked for; groups usually travel in profile.
	OIDCScopes string
	// OIDCUsernameClaim is the claim the account is linked by.
	//
	// preferred_username by default and deliberately not sub, for the reason
	// forward authentication already gives for X-authentik-username over
	// X-authentik-uid: the subject's shape depends on the provider's subject
	// mode, and a changed mode would silently make every person a stranger. It
	// also means an account linked for forward authentication is the same
	// account here.
	OIDCUsernameClaim string
	// OIDCGroupsClaim is the claim the groups are read from.
	OIDCGroupsClaim string
}

// OIDCCallbackPath is the path the provider returns to. OIDCRedirectURL has to
// end in it.
const OIDCCallbackPath = "/admin/oidc/callback"

// minOIDCSecretLength is the shortest client secret an HS256 token is checked
// with. Whoever knows the secret can mint any identity, so a short one is a
// short way into the administration.
const minOIDCSecretLength = 32

// defaultTrustedProxies covers the documented deployment, where Caddy
// terminates TLS on the same host and proxies to localhost.
const defaultTrustedProxies = "127.0.0.1/32,::1/128"

// minSSOSecretLength is the shortest shared secret single sign-on accepts.
// Thirty-two characters is what `openssl rand -hex 16` prints, and half of what
// DEPLOY.md tells an operator to generate.
const minSSOSecretLength = 32

// defaultListen agrees with defaultTrustedProxies: the documented deployment
// has a proxy on the same host, so the socket does not have to leave it.
const defaultListen = "127.0.0.1"

// defaultSignOutPath is the route the authentik outpost itself serves.
const defaultSignOutPath = "/outpost.goauthentik.io/sign_out"

// The names of the settings this block reads, written once each.
//
// Single-sourced so that the sentence an operator reads cannot drift from the
// variable they have to go and change — every refusal below formats one of
// these into its message rather than repeating the literal.
const (
	envListen            = "HOLZCLOUD_LISTEN"
	envSSOEnabled        = "HOLZCLOUD_SSO_ENABLED"
	envSSOSecret         = "HOLZCLOUD_SSO_SECRET"
	envSSOAdminGroup     = "HOLZCLOUD_SSO_ADMIN_GROUP"
	envSSOWebsiteGroups  = "HOLZCLOUD_SSO_WEBSITE_GROUPS"
	envSSOProvision      = "HOLZCLOUD_SSO_PROVISION"
	envSSODefaultWebsite = "HOLZCLOUD_SSO_DEFAULT_WEBSITE"
	envSSOSignOutPath    = "HOLZCLOUD_SSO_SIGN_OUT_PATH"

	envOIDCEnabled       = "HOLZCLOUD_OIDC_ENABLED"
	envOIDCName          = "HOLZCLOUD_OIDC_NAME"
	envOIDCIssuer        = "HOLZCLOUD_OIDC_ISSUER"
	envOIDCAuthorizeURL  = "HOLZCLOUD_OIDC_AUTHORIZE_URL"
	envOIDCClientID      = "HOLZCLOUD_OIDC_CLIENT_ID"
	envOIDCClientSecret  = "HOLZCLOUD_OIDC_CLIENT_SECRET"
	envOIDCKeyFile       = "HOLZCLOUD_OIDC_KEY_FILE"
	envOIDCRedirectURL   = "HOLZCLOUD_OIDC_REDIRECT_URL"
	envOIDCScopes        = "HOLZCLOUD_OIDC_SCOPES"
	envOIDCUsernameClaim = "HOLZCLOUD_OIDC_USERNAME_CLAIM"
	envOIDCGroupsClaim   = "HOLZCLOUD_OIDC_GROUPS_CLAIM"
)

// Load reads the configuration from the environment.
//
// Every parse failure is collected and returned rather than silently falling
// back to a default: a typo in HOLZCLOUD_ARGON2_MEMORY used to weaken password
// hashing with no symptom at all, and a bad size limit used to reset itself to
// 10MB without a word.
func Load() (Config, error) {
	var errs []error

	dataDir := getEnv("HOLZCLOUD_DATA_DIR", "data")
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		errs = append(errs, fmt.Errorf("HOLZCLOUD_DATA_DIR %q: %w", dataDir, err))
		absDataDir = dataDir
	}

	cfg := Config{
		Port:            getEnv("HOLZCLOUD_PORT", "8080"),
		DataDir:         absDataDir,
		LogLevel:        getEnv("HOLZCLOUD_LOG_LEVEL", "INFO"),
		DBPath:          filepath.Join(absDataDir, "holzcloud.sqlite"),
		MaxTemplateSize: envSize("HOLZCLOUD_MAX_TEMPLATE_SIZE", 10*1024*1024, &errs),
		MaxMediaSize:    envSize("HOLZCLOUD_MAX_MEDIA_SIZE", 5*1024*1024, &errs),
		MaxVideoSize:    envSize("HOLZCLOUD_MAX_VIDEO_SIZE", 64*1024*1024, &errs),
		Secure:          envBool("HOLZCLOUD_SECURE", false, &errs),
		MinFreeBytes:    uint64(envSize("HOLZCLOUD_MIN_FREE_BYTES", 512*1024*1024, &errs)),
		PayrexxInstance: strings.TrimSpace(getEnv("HOLZCLOUD_PAYREXX_INSTANCE", "")),
		PayrexxSecret:   strings.TrimSpace(getEnv("HOLZCLOUD_PAYREXX_SECRET", "")),
		PayrexxBaseURL:  strings.TrimSpace(getEnv("HOLZCLOUD_PAYREXX_BASE_URL", "")),
	}

	// Half a configuration is worse than none: the checkout would offer online
	// payment and then fail at the moment the customer presses the button.
	if (cfg.PayrexxInstance == "") != (cfg.PayrexxSecret == "") {
		errs = append(errs, errors.New(
			"HOLZCLOUD_PAYREXX_INSTANCE and HOLZCLOUD_PAYREXX_SECRET have to be set together"))
	}

	cfg.MaxMegapixels = int(envSize("HOLZCLOUD_MAX_MEGAPIXELS", 24, &errs))
	if cfg.MaxMegapixels < 1 {
		errs = append(errs, fmt.Errorf("HOLZCLOUD_MAX_MEGAPIXELS: %d is below the 1 megapixel minimum", cfg.MaxMegapixels))
		cfg.MaxMegapixels = 24
	}

	cfg.Argon2Memory = envUint32("HOLZCLOUD_ARGON2_MEMORY", 65536, &errs)
	cfg.Argon2Iterations = envUint32("HOLZCLOUD_ARGON2_ITERATIONS", 1, &errs)
	parallelism := envUint32("HOLZCLOUD_ARGON2_PARALLELISM", 2, &errs)
	if parallelism > 255 {
		errs = append(errs, fmt.Errorf("HOLZCLOUD_ARGON2_PARALLELISM: %d exceeds 255", parallelism))
		parallelism = 2
	}
	cfg.Argon2Parallelism = uint8(parallelism)

	if cfg.Argon2Memory < 8 {
		errs = append(errs, fmt.Errorf("HOLZCLOUD_ARGON2_MEMORY: %d KB is below the 8 KB minimum", cfg.Argon2Memory))
	}
	if cfg.Argon2Iterations == 0 {
		errs = append(errs, errors.New("HOLZCLOUD_ARGON2_ITERATIONS must be at least 1"))
	}
	if cfg.Argon2Parallelism == 0 {
		errs = append(errs, errors.New("HOLZCLOUD_ARGON2_PARALLELISM must be at least 1"))
	}

	cfg.TrustedProxies, err = parsePrefixes(getEnv("HOLZCLOUD_TRUSTED_PROXIES", defaultTrustedProxies))
	if err != nil {
		errs = append(errs, err)
	}

	cfg.SMTPHost = getEnv("HOLZCLOUD_SMTP_HOST", "")
	cfg.SMTPPort = int(envSize("HOLZCLOUD_SMTP_PORT", 587, &errs))
	cfg.SMTPUser = getEnv("HOLZCLOUD_SMTP_USER", "")
	cfg.SMTPPassword = os.Getenv("HOLZCLOUD_SMTP_PASSWORD")
	cfg.SMTPFrom = getEnv("HOLZCLOUD_SMTP_FROM", "")
	cfg.SMTPFromName = getEnv("HOLZCLOUD_SMTP_FROM_NAME", "")
	cfg.SMTPTLS = strings.ToLower(getEnv("HOLZCLOUD_SMTP_TLS", "starttls"))
	switch cfg.SMTPTLS {
	case "starttls", "tls", "none":
	default:
		errs = append(errs, fmt.Errorf(
			"HOLZCLOUD_SMTP_TLS %q: allowed are starttls, tls or none", cfg.SMTPTLS))
	}
	// Half-configured is the dangerous state: a host without a sender address
	// queues messages that every receiver refuses, and the operator sees a
	// growing outbox with no clue why.
	if (cfg.SMTPHost == "") != (cfg.SMTPFrom == "") {
		errs = append(errs, errors.New(
			"HOLZCLOUD_SMTP_HOST and HOLZCLOUD_SMTP_FROM belong together: "+
				"entweder beide setzen oder keines"))
	}

	cfg.Listen = strings.TrimSpace(getEnv(envListen, defaultListen))
	if _, addrErr := netip.ParseAddr(cfg.Listen); addrErr != nil {
		errs = append(errs, fmt.Errorf("%s: %q is not an IP address: %w", envListen, cfg.Listen, addrErr))
	}

	cfg.SSOEnabled = envBool(envSSOEnabled, false, &errs)
	cfg.SSOSecret = strings.TrimSpace(getEnv(envSSOSecret, ""))
	cfg.SSOAdminGroup = strings.TrimSpace(getEnv(envSSOAdminGroup, ""))
	cfg.SSOProvision = envBool(envSSOProvision, false, &errs)
	cfg.SSODefaultWebsite = envSize(envSSODefaultWebsite, 0, &errs)
	cfg.SSOSignOutPath = strings.TrimSpace(getEnv(envSSOSignOutPath, defaultSignOutPath))
	cfg.SSOWebsiteGroups, err = parseWebsiteGroups(getEnv(envSSOWebsiteGroups, ""))
	if err != nil {
		errs = append(errs, err)
	}

	// Half a configuration is worse than none — the sentence the Payrexx pair
	// above already carries, applied to the layer that decides who somebody is.
	//
	// Every one of these appends rather than returning: an operator who set
	// three things wrong is told three things.
	if cfg.SSOEnabled && cfg.SSOSecret == "" {
		errs = append(errs, fmt.Errorf(
			"%s is on but %s is empty: the shared secret is the only thing that tells "+
				"the reverse proxy apart from anyone else who can reach the port",
			envSSOEnabled, envSSOSecret))
	}

	// Two refusals the Phase 10 code review found missing (WR-05, WR-06).
	//
	// With single sign-on on, the trusted proxies are layer 1: they alone decide
	// whether an identity header is read at all. A prefix of length zero trusts
	// every address, and what stands after it is the shared secret — which is
	// why the secret has a minimum length. Constant time keeps a comparison from
	// leaking timing; it does nothing against a short secret being guessed by
	// whatever counts as a trusted peer. The value is never printed, only its
	// length.
	if cfg.SSOEnabled {
		for _, prefix := range cfg.TrustedProxies {
			if prefix.Bits() == 0 {
				errs = append(errs, fmt.Errorf(
					"HOLZCLOUD_TRUSTED_PROXIES: %s trusts every address while %s is on; the trusted proxies decide whether an identity header is believed at all, and with this prefix only the shared secret would stand",
					prefix, envSSOEnabled))
			}
		}
		if cfg.SSOSecret != "" && len(cfg.SSOSecret) < minSSOSecretLength {
			errs = append(errs, fmt.Errorf(
				"%s has %d characters; at least %d are required (generate one with: openssl rand -hex 32)",
				envSSOSecret, len(cfg.SSOSecret), minSSOSecretLength))
		}
	}
	loadOIDC(&cfg, &errs)

	if cfg.SSOProvision && !cfg.SSOEnabled && !cfg.OIDCEnabled {
		errs = append(errs, fmt.Errorf(
			"%s is on but %s and %s are off: creating accounts with no way to sign in is a setting nobody meant",
			envSSOProvision, envSSOEnabled, envOIDCEnabled))
	}
	// The refusal the whole block exists for. A provisioned account has no rows
	// in user_websites by construction, and NewWebsiteAccessLookup reads "no
	// assignment" as "every website" — so the website a new account belongs to
	// is named here or the process does not start.
	if cfg.SSOProvision && cfg.SSODefaultWebsite <= 0 {
		errs = append(errs, fmt.Errorf(
			"%s is on but %s names no website: a provisioned account with no website "+
				"assignment is an account with access to every website",
			envSSOProvision, envSSODefaultWebsite))
	}
	if !isLocalPath(cfg.SSOSignOutPath) {
		errs = append(errs, fmt.Errorf(
			"%s: %q must be a path on this server, beginning with a single /",
			envSSOSignOutPath, cfg.SSOSignOutPath))
	}
	// Deliberately not a refusal: SSO on with an empty SSOAdminGroup. An
	// installation where no identity provider group grants administration is a
	// legitimate one and must keep loading.

	return cfg, errors.Join(errs...)
}

// loadOIDC reads and checks the OpenID Connect block. Every refusal appends,
// like the rest of Load.
func loadOIDC(cfg *Config, errs *[]error) {
	cfg.OIDCEnabled = envBool(envOIDCEnabled, false, errs)
	cfg.OIDCName = strings.TrimSpace(getEnv(envOIDCName, "OpenID Connect"))
	cfg.OIDCIssuer = strings.TrimSpace(getEnv(envOIDCIssuer, ""))
	cfg.OIDCAuthorizeURL = strings.TrimSpace(getEnv(envOIDCAuthorizeURL, ""))
	cfg.OIDCClientID = strings.TrimSpace(getEnv(envOIDCClientID, ""))
	cfg.OIDCClientSecret = strings.TrimSpace(getEnv(envOIDCClientSecret, ""))
	cfg.OIDCKeyFile = strings.TrimSpace(getEnv(envOIDCKeyFile, ""))
	cfg.OIDCRedirectURL = strings.TrimSpace(getEnv(envOIDCRedirectURL, ""))
	cfg.OIDCScopes = strings.Join(strings.Fields(getEnv(envOIDCScopes, "openid email profile")), " ")
	cfg.OIDCUsernameClaim = strings.TrimSpace(getEnv(envOIDCUsernameClaim, "preferred_username"))
	cfg.OIDCGroupsClaim = strings.TrimSpace(getEnv(envOIDCGroupsClaim, "groups"))
	if !cfg.OIDCEnabled {
		return
	}

	for _, req := range []struct{ name, value string }{
		{envOIDCIssuer, cfg.OIDCIssuer},
		{envOIDCAuthorizeURL, cfg.OIDCAuthorizeURL},
		{envOIDCClientID, cfg.OIDCClientID},
		{envOIDCRedirectURL, cfg.OIDCRedirectURL},
		{envOIDCUsernameClaim, cfg.OIDCUsernameClaim},
		{envOIDCGroupsClaim, cfg.OIDCGroupsClaim},
	} {
		if req.value == "" {
			*errs = append(*errs, fmt.Errorf("%s is on but %s is empty", envOIDCEnabled, req.name))
		}
	}
	if cfg.OIDCName == "" {
		cfg.OIDCName = "OpenID Connect"
	}
	if !strings.Contains(" "+cfg.OIDCScopes+" ", " openid ") {
		*errs = append(*errs, fmt.Errorf("%s: %q lacks openid, without which there is no ID token",
			envOIDCScopes, cfg.OIDCScopes))
	}
	if cfg.OIDCAuthorizeURL != "" && !secureURL(cfg.OIDCAuthorizeURL) {
		*errs = append(*errs, fmt.Errorf(
			"%s: %q must be an https address (http only on localhost)", envOIDCAuthorizeURL, cfg.OIDCAuthorizeURL))
	}
	if cfg.OIDCRedirectURL != "" {
		u, err := url.Parse(cfg.OIDCRedirectURL)
		if err != nil || !secureURL(cfg.OIDCRedirectURL) || u.Path != OIDCCallbackPath || u.RawQuery != "" || u.Fragment != "" {
			*errs = append(*errs, fmt.Errorf(
				"%s: %q must be an https address ending in %s, with no query (http only on localhost)",
				envOIDCRedirectURL, cfg.OIDCRedirectURL, OIDCCallbackPath))
		}
	}

	// One way of checking the signature, never two. A public key and a
	// secret side by side is how HS256 comes to be checked with the public key
	// as its secret — internal/oidc refuses that on its own, and this refuses
	// the configuration that would invite it.
	switch {
	case cfg.OIDCKeyFile != "" && cfg.OIDCClientSecret != "":
		*errs = append(*errs, fmt.Errorf(
			"%s and %s are both set: name the provider's public key or its client secret, not both",
			envOIDCKeyFile, envOIDCClientSecret))
	case cfg.OIDCKeyFile == "" && cfg.OIDCClientSecret == "":
		*errs = append(*errs, fmt.Errorf(
			"%s is on but neither %s nor %s is set: without a key no token can be checked",
			envOIDCEnabled, envOIDCKeyFile, envOIDCClientSecret))
	case cfg.OIDCKeyFile != "":
		data, err := os.ReadFile(cfg.OIDCKeyFile)
		if err != nil {
			*errs = append(*errs, fmt.Errorf("%s: %w", envOIDCKeyFile, err))
			break
		}
		keys, err := oidc.ParseKeys(data)
		if err != nil {
			*errs = append(*errs, fmt.Errorf("%s: %w", envOIDCKeyFile, err))
			break
		}
		cfg.OIDCKeys = keys
	default:
		if len(cfg.OIDCClientSecret) < minOIDCSecretLength {
			*errs = append(*errs, fmt.Errorf(
				"%s has %d characters; at least %d are required, because whoever knows it can sign any identity",
				envOIDCClientSecret, len(cfg.OIDCClientSecret), minOIDCSecretLength))
		}
	}
}

// secureURL reports whether raw is an absolute https address, or http on the
// local machine — which a browser treats as secure too, and which is how the
// sign-in is tried out before it goes live.
func secureURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		h := u.Hostname()
		return h == "localhost" || h == "127.0.0.1" || h == "::1"
	}
	return false
}

// isLocalPath reports whether p is a path on this server.
//
// One leading slash and no second character a browser would read as the start
// of a host: "//evil.example" is protocol-relative, and "/\evil.example" is the
// same trick with the separator browsers also accept. Both leave this origin,
// and neither looks like a URL to somebody skimming an environment file.
func isLocalPath(p string) bool {
	if !strings.HasPrefix(p, "/") {
		return false
	}
	// No control character anywhere. A browser removes tab, line feed and
	// carriage return from a URL before resolving it, so "/<TAB>/evil.example"
	// is followed as "//evil.example" — protocol-relative, somebody else's
	// server — and net/http keeps the tab in a Location header (Phase 10
	// security audit, T-10-05).
	for _, c := range p {
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	return !strings.HasPrefix(p, "//") && !strings.HasPrefix(p, `/\`)
}

// parseWebsiteGroups parses a comma-separated list of group=websiteID pairs,
// modelled on parsePrefixes. An empty list is valid and means no group grants a
// website.
//
// Every malformed pair is its own error rather than the first one stopping the
// walk, for the same reason Load collects: an operator with two typos should
// learn about two typos.
func parseWebsiteGroups(raw string) (map[string]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	groups := make(map[string]int64)
	var errs []error
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, id, found := strings.Cut(part, "=")
		name = strings.TrimSpace(name)
		id = strings.TrimSpace(id)
		if !found || name == "" || id == "" {
			errs = append(errs, fmt.Errorf(
				"%s: %q is not a group=websiteID pair", envSSOWebsiteGroups, part))
			continue
		}
		n, convErr := strconv.ParseInt(id, 10, 64)
		if convErr != nil {
			errs = append(errs, fmt.Errorf(
				"%s: %q is not a website id: %w", envSSOWebsiteGroups, part, convErr))
			continue
		}
		if n <= 0 {
			errs = append(errs, fmt.Errorf(
				"%s: %q must name a positive website id", envSSOWebsiteGroups, part))
			continue
		}
		// Two answers to one question is a typo, not a preference.
		if _, duplicate := groups[name]; duplicate {
			errs = append(errs, fmt.Errorf(
				"%s: group %q is listed twice", envSSOWebsiteGroups, name))
			continue
		}
		groups[name] = n
	}
	return groups, errors.Join(errs...)
}

// LogValue renders the effective configuration for the startup log, so an
// operator can see what the process actually resolved rather than what they
// believe they set.
func (c Config) LogValue() slog.Value {
	proxies := make([]string, 0, len(c.TrustedProxies))
	for _, p := range c.TrustedProxies {
		proxies = append(proxies, p.String())
	}
	return slog.GroupValue(
		slog.String("port", c.Port),
		slog.String("data_dir", c.DataDir),
		slog.String("db_path", c.DBPath),
		slog.String("log_level", c.LogLevel),
		slog.Bool("secure", c.Secure),
		slog.Int64("max_template_size", c.MaxTemplateSize),
		slog.Int64("max_media_size", c.MaxMediaSize),
		slog.Int64("max_video_size", c.MaxVideoSize),
		slog.Int("max_megapixels", c.MaxMegapixels),
		slog.Uint64("argon2_memory_kb", uint64(c.Argon2Memory)),
		slog.Uint64("argon2_iterations", uint64(c.Argon2Iterations)),
		slog.Uint64("argon2_parallelism", uint64(c.Argon2Parallelism)),
		slog.Uint64("min_free_bytes", c.MinFreeBytes),
		// The instance name is not a secret; the key is, and is never logged —
		// not even truncated. A log file goes to places a key must not.
		slog.String("smtp_host", c.SMTPHost),
		slog.String("smtp_from", c.SMTPFrom),
		// The password is never written, not even truncated.
		slog.Bool("smtp_configured", c.SMTPHost != "" && c.SMTPFrom != ""),
		slog.String("payrexx_instance", c.PayrexxInstance),
		slog.Bool("payrexx_configured", c.PayrexxInstance != "" && c.PayrexxSecret != ""),
		slog.String("trusted_proxies", strings.Join(proxies, ",")),
		slog.String("listen", c.Listen),
		slog.Bool("sso_enabled", c.SSOEnabled),
		// The shared secret is absent for the same reason the SMTP password is:
		// the startup log is the first thing anyone pastes into a bug report.
		// Not even truncated — a prefix of a compared secret is still a head
		// start.
		slog.Bool("sso_configured", c.SSOEnabled && c.SSOSecret != ""),
		slog.Bool("sso_provision", c.SSOProvision),
		slog.String("sso_admin_group", c.SSOAdminGroup),
		slog.Int64("sso_default_website", c.SSODefaultWebsite),
		slog.Int("sso_website_groups", len(c.SSOWebsiteGroups)),
		slog.String("sso_sign_out_path", c.SSOSignOutPath),
		// The client secret is absent for the reason the shared secret is.
		slog.Bool("oidc_enabled", c.OIDCEnabled),
		slog.String("oidc_issuer", c.OIDCIssuer),
		slog.String("oidc_client_id", c.OIDCClientID),
		slog.String("oidc_redirect_url", c.OIDCRedirectURL),
		slog.Int("oidc_keys", len(c.OIDCKeys)),
		slog.Bool("oidc_client_secret_set", c.OIDCClientSecret != ""),
		// The password is deliberately absent: the startup log is the first
		// thing anyone pastes into a bug report.
		slog.String("smtp", smtpSummary(c)),
	)
}

// parsePrefixes parses a comma-separated CIDR list. An empty list is valid and
// means no proxy is trusted.
func parsePrefixes(raw string) ([]netip.Prefix, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var prefixes []netip.Prefix
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		p, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("HOLZCLOUD_TRUSTED_PROXIES: %q is not a CIDR prefix: %w", part, err)
		}
		prefixes = append(prefixes, p.Masked())
	}
	return prefixes, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envUint32(key string, def uint32, errs *[]error) uint32 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: %q is not a number: %w", key, v, err))
		return def
	}
	return uint32(n)
}

func envSize(key string, def int64, errs *[]error) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: %q is not a number: %w", key, v, err))
		return def
	}
	if n <= 0 {
		*errs = append(*errs, fmt.Errorf("%s: must be positive, got %d", key, n))
		return def
	}
	return n
}

func envBool(key string, def bool, errs *[]error) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: %q is not a boolean: %w", key, v, err))
		return def
	}
	return b
}

func NewLogger(levelStr string) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		level = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}

// smtpSummary is one readable line about the mail setup, for the startup log.
func smtpSummary(c Config) string {
	if c.SMTPHost == "" || c.SMTPFrom == "" {
		return "aus"
	}
	auth := "ohne Anmeldung"
	if c.SMTPUser != "" {
		auth = "als " + c.SMTPUser
	}
	return fmt.Sprintf("%s:%d (%s) von %s, %s", c.SMTPHost, c.SMTPPort, c.SMTPTLS, c.SMTPFrom, auth)
}
