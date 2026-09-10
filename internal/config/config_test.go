package config_test

import (
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port: want 8080, got %s", cfg.Port)
	}
	if cfg.LogLevel != "INFO" {
		t.Errorf("LogLevel: want INFO, got %s", cfg.LogLevel)
	}
	if want := filepath.Join(cfg.DataDir, "holzcloud.sqlite"); cfg.DBPath != want {
		t.Errorf("DBPath: want %s, got %s", want, cfg.DBPath)
	}
	if !filepath.IsAbs(cfg.DataDir) {
		t.Errorf("DataDir should be absolute so a relative launch cannot move the database: %s", cfg.DataDir)
	}
	// Loopback must be trusted by default — that is the documented Caddy setup.
	if len(cfg.TrustedProxies) != 2 {
		t.Errorf("want loopback v4+v6 trusted by default, got %v", cfg.TrustedProxies)
	}
}

func TestLoadOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOLZCLOUD_PORT", "9090")
	t.Setenv("HOLZCLOUD_DATA_DIR", dir)
	t.Setenv("HOLZCLOUD_LOG_LEVEL", "DEBUG")
	t.Setenv("HOLZCLOUD_SECURE", "true")
	t.Setenv("HOLZCLOUD_MAX_MEDIA_SIZE", "1234")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "9090" || cfg.LogLevel != "DEBUG" || !cfg.Secure || cfg.MaxMediaSize != 1234 {
		t.Errorf("overrides not applied: %+v", cfg)
	}
	if cfg.DBPath != filepath.Join(dir, "holzcloud.sqlite") {
		t.Errorf("DBPath not derived from DataDir: %s", cfg.DBPath)
	}
}

// A typo used to be swallowed and the default silently substituted, so a
// mistyped Argon2 parameter weakened password hashing with no symptom at all.
func TestLoadReportsBadValuesInsteadOfSilentlyDefaulting(t *testing.T) {
	cases := map[string]struct{ key, value, wantIn string }{
		"argon2 memory":   {"HOLZCLOUD_ARGON2_MEMORY", "64MB", "HOLZCLOUD_ARGON2_MEMORY"},
		"media size":      {"HOLZCLOUD_MAX_MEDIA_SIZE", "5mb", "HOLZCLOUD_MAX_MEDIA_SIZE"},
		"negative size":   {"HOLZCLOUD_MAX_TEMPLATE_SIZE", "-1", "HOLZCLOUD_MAX_TEMPLATE_SIZE"},
		"secure flag":     {"HOLZCLOUD_SECURE", "yes please", "HOLZCLOUD_SECURE"},
		"iterations zero": {"HOLZCLOUD_ARGON2_ITERATIONS", "0", "HOLZCLOUD_ARGON2_ITERATIONS"},
		"trusted proxies": {"HOLZCLOUD_TRUSTED_PROXIES", "127.0.0.1", "HOLZCLOUD_TRUSTED_PROXIES"},
	}

	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())
			t.Setenv(tc.key, tc.value)

			if _, err := config.Load(); err == nil {
				t.Fatalf("%s=%q was accepted; want an error", tc.key, tc.value)
			} else if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error should name %s: %v", tc.wantIn, err)
			}
		})
	}
}

func TestTrustedProxiesParsing(t *testing.T) {
	t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())
	t.Setenv("HOLZCLOUD_TRUSTED_PROXIES", "10.0.0.0/8, 127.0.0.1/32")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.TrustedProxies) != 2 {
		t.Fatalf("want 2 prefixes, got %v", cfg.TrustedProxies)
	}
}

func TestNewLoggerInvalidLevel(t *testing.T) {
	logger := config.NewLogger("INVALID")
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	logger.Info("test", slog.String("key", "value"))
}

// The startup log must show what was actually resolved, not what the operator
// believes they set.
func TestConfigLogValueIncludesEffectiveSettings(t *testing.T) {
	t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// LogValuer is applied by Resolve, which is what a handler calls.
	rendered := slog.AnyValue(cfg).Resolve().String()
	for _, want := range []string{"data_dir", "db_path", "argon2_memory_kb", "trusted_proxies"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("startup log is missing %q: %s", want, rendered)
		}
	}
}

// --- Single sign-on and the listen address -----------------------------------
//
// Nothing below authenticates anybody. These tests hold the two shapes that
// keep a half-configured installation from starting at all: single sign-on
// without a secret, and account creation without a website to create accounts
// into. The second is the one that matters — a provisioned account has no rows
// in user_websites by construction, and internal/admin/handler.go reads "no
// assignment" as "every website".

// With nothing set, the new block is inert and the socket is loopback.
func TestSSOAndListenDefaults(t *testing.T) {
	t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != "127.0.0.1" {
		t.Errorf("Listen: want 127.0.0.1, got %q", cfg.Listen)
	}
	if cfg.SSOEnabled {
		t.Error("SSOEnabled must be false unless HOLZCLOUD_SSO_ENABLED says otherwise")
	}
	if cfg.SSOProvision {
		t.Error("SSOProvision must be false unless HOLZCLOUD_SSO_PROVISION says otherwise")
	}
	// No default group name, deliberately: a default is a name somebody at the
	// identity provider can create.
	if cfg.SSOAdminGroup != "" {
		t.Errorf("SSOAdminGroup must have no default, got %q", cfg.SSOAdminGroup)
	}
	if cfg.SSOSecret != "" {
		t.Errorf("SSOSecret must have no default, got %q", cfg.SSOSecret)
	}
	if cfg.SSODefaultWebsite != 0 {
		t.Errorf("SSODefaultWebsite must have no default, got %d", cfg.SSODefaultWebsite)
	}
	if len(cfg.SSOWebsiteGroups) != 0 {
		t.Errorf("SSOWebsiteGroups must be empty by default, got %v", cfg.SSOWebsiteGroups)
	}
	if cfg.SSOSignOutPath != "/outpost.goauthentik.io/sign_out" {
		t.Errorf("SSOSignOutPath: want the authentik outpost path, got %q", cfg.SSOSignOutPath)
	}
}

// Every interface is still reachable — it just has to be asked for now. The
// container instructions in deploy/DEPLOY.md publish a port, and a process that
// binds loopback inside its own network namespace answers nobody.
func TestListenAcceptsAnyAddressTheOperatorNames(t *testing.T) {
	for _, addr := range []string{"0.0.0.0", "::", "127.0.0.1", "::1", "192.168.1.10"} {
		t.Run(addr, func(t *testing.T) {
			t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())
			t.Setenv("HOLZCLOUD_LISTEN", addr)

			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Listen != addr {
				t.Errorf("Listen: want %q, got %q", addr, cfg.Listen)
			}
		})
	}
}

// An installation where no group grants administration is legitimate and must
// keep loading. It is deliberately not a refusal.
func TestSSOWithoutAnAdminGroupIsALegalInstallation(t *testing.T) {
	t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())
	t.Setenv("HOLZCLOUD_SSO_ENABLED", "true")
	t.Setenv("HOLZCLOUD_SSO_SECRET", "a-shared-secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("SSO without an admin group must load: %v", err)
	}
	if !cfg.SSOEnabled {
		t.Error("HOLZCLOUD_SSO_ENABLED=true was not read")
	}
	if cfg.SSOAdminGroup != "" {
		t.Errorf("SSOAdminGroup must stay empty, got %q", cfg.SSOAdminGroup)
	}
}

// One named subtest per refusal. Each error names the variable an operator has
// to go and change.
func TestSSORefusalsNameTheirVariables(t *testing.T) {
	cases := []struct {
		name   string
		env    map[string]string
		wantIn []string
	}{
		{
			name:   "listen is not an address",
			env:    map[string]string{"HOLZCLOUD_LISTEN": "nonsense"},
			wantIn: []string{"HOLZCLOUD_LISTEN"},
		},
		{
			name:   "single sign-on without a secret",
			env:    map[string]string{"HOLZCLOUD_SSO_ENABLED": "true"},
			wantIn: []string{"HOLZCLOUD_SSO_ENABLED", "HOLZCLOUD_SSO_SECRET"},
		},
		{
			name:   "provisioning without a sign-on path",
			env:    map[string]string{"HOLZCLOUD_SSO_PROVISION": "true"},
			wantIn: []string{"HOLZCLOUD_SSO_PROVISION", "HOLZCLOUD_SSO_ENABLED"},
		},
		{
			name: "provisioning without a default website",
			env: map[string]string{
				"HOLZCLOUD_SSO_ENABLED":   "true",
				"HOLZCLOUD_SSO_SECRET":    "a-shared-secret",
				"HOLZCLOUD_SSO_PROVISION": "true",
			},
			wantIn: []string{"HOLZCLOUD_SSO_DEFAULT_WEBSITE"},
		},
		{
			name: "a default website of zero is the same as none",
			env: map[string]string{
				"HOLZCLOUD_SSO_ENABLED":         "true",
				"HOLZCLOUD_SSO_SECRET":          "a-shared-secret",
				"HOLZCLOUD_SSO_PROVISION":       "true",
				"HOLZCLOUD_SSO_DEFAULT_WEBSITE": "0",
			},
			wantIn: []string{"HOLZCLOUD_SSO_DEFAULT_WEBSITE"},
		},
		{
			name: "a negative default website is the same as none",
			env: map[string]string{
				"HOLZCLOUD_SSO_ENABLED":         "true",
				"HOLZCLOUD_SSO_SECRET":          "a-shared-secret",
				"HOLZCLOUD_SSO_PROVISION":       "true",
				"HOLZCLOUD_SSO_DEFAULT_WEBSITE": "-1",
			},
			wantIn: []string{"HOLZCLOUD_SSO_DEFAULT_WEBSITE"},
		},
		{
			name:   "the sign-out target is an absolute address",
			env:    map[string]string{"HOLZCLOUD_SSO_SIGN_OUT_PATH": "https://evil.example/sign_out"},
			wantIn: []string{"HOLZCLOUD_SSO_SIGN_OUT_PATH"},
		},
		{
			name:   "the sign-out target is protocol-relative",
			env:    map[string]string{"HOLZCLOUD_SSO_SIGN_OUT_PATH": "//evil.example/sign_out"},
			wantIn: []string{"HOLZCLOUD_SSO_SIGN_OUT_PATH"},
		},
		{
			name:   "the sign-out target is a backslash escape",
			env:    map[string]string{"HOLZCLOUD_SSO_SIGN_OUT_PATH": `/\evil.example/sign_out`},
			wantIn: []string{"HOLZCLOUD_SSO_SIGN_OUT_PATH"},
		},
		{
			name:   "the sign-out target is not a path at all",
			env:    map[string]string{"HOLZCLOUD_SSO_SIGN_OUT_PATH": "outpost.goauthentik.io/sign_out"},
			wantIn: []string{"HOLZCLOUD_SSO_SIGN_OUT_PATH"},
		},
		{
			// With single sign-on on, the trusted proxies ARE layer 1: they alone
			// decide whether an identity header is read at all. A prefix of length
			// zero trusts every address, and then only the secret stands
			// (Phase 10 code review WR-05).
			name: "single sign-on trusting every IPv4 address",
			env: map[string]string{
				"HOLZCLOUD_SSO_ENABLED":     "true",
				"HOLZCLOUD_SSO_SECRET":      "a-shared-secret-long-enough-to-be-one",
				"HOLZCLOUD_TRUSTED_PROXIES": "0.0.0.0/0",
			},
			wantIn: []string{"HOLZCLOUD_TRUSTED_PROXIES", "HOLZCLOUD_SSO_ENABLED"},
		},
		{
			name: "single sign-on trusting every IPv6 address",
			env: map[string]string{
				"HOLZCLOUD_SSO_ENABLED":     "true",
				"HOLZCLOUD_SSO_SECRET":      "a-shared-secret-long-enough-to-be-one",
				"HOLZCLOUD_TRUSTED_PROXIES": "127.0.0.1/32,::/0",
			},
			wantIn: []string{"HOLZCLOUD_TRUSTED_PROXIES", "HOLZCLOUD_SSO_ENABLED"},
		},
		{
			// Constant time protects against timing, not against guessing: a short
			// secret can be enumerated by anything that counts as a trusted peer,
			// without leaving a trace (WR-06).
			name: "single sign-on with a secret too short to be one",
			env: map[string]string{
				"HOLZCLOUD_SSO_ENABLED": "true",
				"HOLZCLOUD_SSO_SECRET":  "kurz",
			},
			wantIn: []string{"HOLZCLOUD_SSO_SECRET"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			_, err := config.Load()
			if err == nil {
				t.Fatalf("%v was accepted; want a refusal", tc.env)
			}
			for _, want := range tc.wantIn {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal must name %s so the operator knows what to change: %v", want, err)
				}
			}
		})
	}
}

// Provisioning with a named website is a complete configuration and loads.
func TestSSOProvisioningWithADefaultWebsiteLoads(t *testing.T) {
	t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())
	t.Setenv("HOLZCLOUD_SSO_ENABLED", "true")
	t.Setenv("HOLZCLOUD_SSO_SECRET", "a-shared-secret")
	t.Setenv("HOLZCLOUD_SSO_PROVISION", "true")
	t.Setenv("HOLZCLOUD_SSO_DEFAULT_WEBSITE", "7")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("a complete provisioning configuration must load: %v", err)
	}
	if !cfg.SSOProvision || cfg.SSODefaultWebsite != 7 {
		t.Errorf("provisioning not applied: provision=%v default=%d", cfg.SSOProvision, cfg.SSODefaultWebsite)
	}
}

// The map is explicit because `websites` has no slug column, only a display
// name — matching a group against a name means a rename unassigns everybody.
func TestSSOWebsiteGroupsParsing(t *testing.T) {
	t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())
	t.Setenv("HOLZCLOUD_SSO_WEBSITE_GROUPS", " redaktion-a=1, redaktion-b = 2 ")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string]int64{"redaktion-a": 1, "redaktion-b": 2}
	if len(cfg.SSOWebsiteGroups) != len(want) {
		t.Fatalf("want %v, got %v", want, cfg.SSOWebsiteGroups)
	}
	for name, id := range want {
		if cfg.SSOWebsiteGroups[name] != id {
			t.Errorf("group %q: want website %d, got %d", name, id, cfg.SSOWebsiteGroups[name])
		}
	}
}

func TestSSOWebsiteGroupsRejectsMalformedPairs(t *testing.T) {
	cases := map[string]string{
		"no equals sign":          "redaktion-a",
		"no group name":           "=1",
		"no website id":           "redaktion-a=",
		"website id not a number": "redaktion-a=nought",
		"website id is zero":      "redaktion-a=0",
		"website id negative":     "redaktion-a=-3",
		// Two answers to one question is a typo, not a preference.
		"a group listed twice": "redaktion-a=1,redaktion-a=2",
	}

	for label, raw := range cases {
		t.Run(label, func(t *testing.T) {
			t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())
			t.Setenv("HOLZCLOUD_SSO_WEBSITE_GROUPS", raw)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("%q was accepted; want a refusal", raw)
			}
			if !strings.Contains(err.Error(), "HOLZCLOUD_SSO_WEBSITE_GROUPS") {
				t.Errorf("the refusal must name the variable: %v", err)
			}
		})
	}
}

// An operator who sets three things wrong is told three things, not the first
// one. errors.Join is what Load already returns; the new refusals append to the
// same slice rather than returning early.
func TestThreeBadSettingsProduceThreeErrors(t *testing.T) {
	t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())
	t.Setenv("HOLZCLOUD_LISTEN", "nonsense")
	t.Setenv("HOLZCLOUD_SSO_ENABLED", "true")
	t.Setenv("HOLZCLOUD_SSO_SIGN_OUT_PATH", "https://evil.example/sign_out")

	_, err := config.Load()
	if err == nil {
		t.Fatal("three bad settings were accepted")
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("Load must keep returning errors.Join, got %T", err)
	}
	if got := len(joined.Unwrap()); got != 3 {
		t.Errorf("want 3 collected errors, got %d: %v", got, err)
	}
}

// The startup log is the first thing anyone pastes into a bug report.
func TestConfigLogValueCarriesTheSSOBlockButNotTheSecret(t *testing.T) {
	const secret = "correct-horse-battery-staple"

	t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())
	t.Setenv("HOLZCLOUD_LISTEN", "0.0.0.0")
	t.Setenv("HOLZCLOUD_SSO_ENABLED", "true")
	t.Setenv("HOLZCLOUD_SSO_SECRET", secret)
	t.Setenv("HOLZCLOUD_SSO_ADMIN_GROUP", "holzcloud-admins")
	t.Setenv("HOLZCLOUD_SSO_WEBSITE_GROUPS", "redaktion-a=1")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	rendered := slog.AnyValue(cfg).Resolve().String()
	for _, want := range []string{
		"listen", "sso_enabled", "sso_provision", "sso_admin_group",
		"sso_default_website", "sso_website_groups", "sso_configured",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("startup log is missing %q: %s", want, rendered)
		}
	}
	if strings.Contains(rendered, secret) {
		t.Errorf("the shared secret is in the startup log: %s", rendered)
	}
	// Not even a prefix of it.
	if strings.Contains(rendered, "correct-horse") || strings.Contains(rendered, "correct") {
		t.Errorf("a prefix of the shared secret is in the startup log: %s", rendered)
	}
}
