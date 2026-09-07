package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	// MaxVideoSize gilt nur für Videodateien. Getrennt von MaxMediaSize, weil
	// die fünf Megabyte, die für ein Foto grosszügig sind, für eine halbe
	// Minute Film nicht reichen — und weil ein Bild von 60 MB trotzdem ein
	// Versehen bleibt.
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
}

// defaultTrustedProxies covers the documented deployment, where Caddy
// terminates TLS on the same host and proxies to localhost.
const defaultTrustedProxies = "127.0.0.1/32,::1/128"

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
			"HOLZCLOUD_PAYREXX_INSTANCE und HOLZCLOUD_PAYREXX_SECRET müssen zusammen gesetzt werden"))
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
			"HOLZCLOUD_SMTP_TLS %q: erlaubt sind starttls, tls oder none", cfg.SMTPTLS))
	}
	// Half-configured is the dangerous state: a host without a sender address
	// queues messages that every receiver refuses, and the operator sees a
	// growing outbox with no clue why.
	if (cfg.SMTPHost == "") != (cfg.SMTPFrom == "") {
		errs = append(errs, errors.New(
			"HOLZCLOUD_SMTP_HOST und HOLZCLOUD_SMTP_FROM gehören zusammen: "+
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
	if cfg.SSOProvision && !cfg.SSOEnabled {
		errs = append(errs, fmt.Errorf(
			"%s is on but %s is off: creating accounts with no way to sign in is a setting nobody meant",
			envSSOProvision, envSSOEnabled))
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
		// Das Passwort wird nie geschrieben, auch nicht gekürzt.
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
