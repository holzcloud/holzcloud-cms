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
