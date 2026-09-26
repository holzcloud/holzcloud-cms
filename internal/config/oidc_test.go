package config_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/config"
)

// oidcEnv is a complete OpenID Connect configuration with a key file.
func oidcEnv(t *testing.T) map[string]string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	file := filepath.Join(t.TempDir(), "idp.pem")
	if err := os.WriteFile(file, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"HOLZCLOUD_OIDC_ENABLED":       "true",
		"HOLZCLOUD_OIDC_ISSUER":        "https://auth.example.org/application/o/holzcloud/",
		"HOLZCLOUD_OIDC_AUTHORIZE_URL": "https://auth.example.org/application/o/authorize/",
		"HOLZCLOUD_OIDC_CLIENT_ID":     "holzcloud",
		"HOLZCLOUD_OIDC_KEY_FILE":      file,
		"HOLZCLOUD_OIDC_REDIRECT_URL":  "https://cms.example.org/admin/oidc/callback",
	}
}

func loadWith(t *testing.T, env map[string]string) (config.Config, error) {
	t.Helper()
	t.Setenv("HOLZCLOUD_DATA_DIR", t.TempDir())
	// Every variable of the block, so that one case in a loop cannot leave its
	// setting behind for the next.
	for _, k := range []string{"HOLZCLOUD_OIDC_ENABLED", "HOLZCLOUD_OIDC_ISSUER", "HOLZCLOUD_OIDC_AUTHORIZE_URL",
		"HOLZCLOUD_OIDC_CLIENT_ID", "HOLZCLOUD_OIDC_CLIENT_SECRET", "HOLZCLOUD_OIDC_KEY_FILE",
		"HOLZCLOUD_OIDC_REDIRECT_URL", "HOLZCLOUD_OIDC_SCOPES", "HOLZCLOUD_SSO_PROVISION", "HOLZCLOUD_SSO_DEFAULT_WEBSITE"} {
		t.Setenv(k, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	return config.Load()
}

func TestOIDCIsOffAndInertByDefault(t *testing.T) {
	cfg, err := loadWith(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OIDCEnabled || len(cfg.OIDCKeys) != 0 {
		t.Errorf("OIDC on by default: %+v", cfg.OIDCEnabled)
	}
	if cfg.OIDCScopes != "openid email profile" || cfg.OIDCUsernameClaim != "preferred_username" || cfg.OIDCGroupsClaim != "groups" {
		t.Errorf("defaults: %q %q %q", cfg.OIDCScopes, cfg.OIDCUsernameClaim, cfg.OIDCGroupsClaim)
	}
}

func TestACompleteOIDCConfigurationLoads(t *testing.T) {
	cfg, err := loadWith(t, oidcEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.OIDCEnabled || len(cfg.OIDCKeys) != 1 {
		t.Errorf("keys: %d", len(cfg.OIDCKeys))
	}

	env := oidcEnv(t)
	delete(env, "HOLZCLOUD_OIDC_KEY_FILE")
	env["HOLZCLOUD_OIDC_CLIENT_SECRET"] = strings.Repeat("s", 40)
	env["HOLZCLOUD_OIDC_REDIRECT_URL"] = "http://localhost:8080/admin/oidc/callback"
	// Provisioning is legal with OpenID Connect as the only way in.
	env["HOLZCLOUD_SSO_PROVISION"] = "true"
	env["HOLZCLOUD_SSO_DEFAULT_WEBSITE"] = "1"
	if _, err := loadWith(t, env); err != nil {
		t.Errorf("secret, localhost and provisioning: %v", err)
	}
}

func TestOIDCRefusalsNameTheirVariables(t *testing.T) {
	for name, tc := range map[string]struct {
		change map[string]string
		wantIn []string
	}{
		"no issuer":            {map[string]string{"HOLZCLOUD_OIDC_ISSUER": ""}, []string{"HOLZCLOUD_OIDC_ISSUER"}},
		"no client":            {map[string]string{"HOLZCLOUD_OIDC_CLIENT_ID": ""}, []string{"HOLZCLOUD_OIDC_CLIENT_ID"}},
		"plain http elsewhere": {map[string]string{"HOLZCLOUD_OIDC_AUTHORIZE_URL": "http://auth.example.org/authorize"}, []string{"HOLZCLOUD_OIDC_AUTHORIZE_URL"}},
		"another callback":     {map[string]string{"HOLZCLOUD_OIDC_REDIRECT_URL": "https://cms.example.org/admin/"}, []string{"HOLZCLOUD_OIDC_REDIRECT_URL"}},
		"callback with query":  {map[string]string{"HOLZCLOUD_OIDC_REDIRECT_URL": "https://cms.example.org/admin/oidc/callback?x=1"}, []string{"HOLZCLOUD_OIDC_REDIRECT_URL"}},
		"key and secret":       {map[string]string{"HOLZCLOUD_OIDC_CLIENT_SECRET": strings.Repeat("s", 40)}, []string{"HOLZCLOUD_OIDC_KEY_FILE", "HOLZCLOUD_OIDC_CLIENT_SECRET"}},
		"neither":              {map[string]string{"HOLZCLOUD_OIDC_KEY_FILE": ""}, []string{"HOLZCLOUD_OIDC_KEY_FILE", "HOLZCLOUD_OIDC_CLIENT_SECRET"}},
		"missing key file":     {map[string]string{"HOLZCLOUD_OIDC_KEY_FILE": "/nonexistent/idp.pem"}, []string{"HOLZCLOUD_OIDC_KEY_FILE"}},
		"short secret":         {map[string]string{"HOLZCLOUD_OIDC_KEY_FILE": "", "HOLZCLOUD_OIDC_CLIENT_SECRET": "short"}, []string{"HOLZCLOUD_OIDC_CLIENT_SECRET"}},
		"no openid scope":      {map[string]string{"HOLZCLOUD_OIDC_SCOPES": "email profile"}, []string{"HOLZCLOUD_OIDC_SCOPES"}},
	} {
		env := oidcEnv(t)
		for k, v := range tc.change {
			env[k] = v
		}
		_, err := loadWith(t, env)
		if err == nil {
			t.Errorf("%s: loaded", name)
			continue
		}
		for _, want := range tc.wantIn {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: %q does not name %s", name, err, want)
			}
		}
	}
}

func TestConfigLogValueCarriesTheOIDCBlockButNotTheSecret(t *testing.T) {
	env := oidcEnv(t)
	delete(env, "HOLZCLOUD_OIDC_KEY_FILE")
	secret := "a-client-secret-nobody-may-read-in-a-log"
	env["HOLZCLOUD_OIDC_CLIENT_SECRET"] = secret
	cfg, err := loadWith(t, env)
	if err != nil {
		t.Fatal(err)
	}
	out := cfg.LogValue().String()
	if !strings.Contains(out, "oidc_enabled=true") || !strings.Contains(out, "auth.example.org") {
		t.Errorf("the block is missing: %s", out)
	}
	if strings.Contains(out, secret) || strings.Contains(out, secret[:8]) {
		t.Errorf("the secret is in the log: %s", out)
	}
}
