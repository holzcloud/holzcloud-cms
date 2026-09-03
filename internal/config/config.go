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
}

// defaultTrustedProxies covers the documented deployment, where Caddy
// terminates TLS on the same host and proxies to localhost.
const defaultTrustedProxies = "127.0.0.1/32,::1/128"

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

	return cfg, errors.Join(errs...)
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
