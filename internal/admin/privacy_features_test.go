package admin

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/privacy"
)

// installFormPlugin installs a shipped plugin (the manifest and the module the
// repository carries) into a manager and puts it in the wanted state. Missing
// files fail the test: a plugin that silently is not there would let every
// "forms are detected" case pass for the wrong reason.
func installFormPlugin(t *testing.T, database *db.DB, dir, id string, enabled bool, sites []int64) *plugin.Manager {
	t.Helper()
	ctx := context.Background()
	src := filepath.Join("..", "..", "plugins", id)

	raw, err := os.ReadFile(filepath.Join(src, "plugin.json"))
	if err != nil {
		t.Fatalf("read manifest of %s: %v", id, err)
	}
	manifest, err := plugin.ParseManifest(raw)
	if err != nil {
		t.Fatalf("parse manifest of %s: %v", id, err)
	}
	module, err := os.ReadFile(filepath.Join(src, plugin.ModuleName))
	if err != nil {
		t.Fatalf("read module of %s: %v", id, err)
	}

	store := plugin.NewStore(database)
	if err := store.Install(ctx, &plugin.Package{Manifest: manifest, Module: module, SHA256: "c0ffee"}); err != nil {
		t.Fatalf("Install %s: %v", id, err)
	}
	if err := store.SetEnabled(ctx, id, enabled); err != nil {
		t.Fatalf("SetEnabled %s: %v", id, err)
	}
	if err := store.SetWebsites(ctx, id, sites); err != nil {
		t.Fatalf("SetWebsites %s: %v", id, err)
	}
	// The manager reads the module from disk, as the server does.
	if err := os.MkdirAll(filepath.Join(dir, "plugins", id), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins", id, plugin.ModuleName), module, 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := plugin.NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	t.Cleanup(func() { rt.Close(context.Background()) })
	mgr, err := plugin.NewManager(ctx, store, rt, dir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return mgr
}

func TestPrivacyFeaturesFollowTheWebsite(t *testing.T) {
	ctx := context.Background()

	// setup receives a fresh handler and the website and returns the website
	// as it should be read back after the changes.
	type setup func(t *testing.T, h *Handler, database *db.DB, ws *domain.Website)

	shopOn := func(display string) setup {
		return func(t *testing.T, h *Handler, database *db.DB, ws *domain.Website) {
			t.Helper()
			if _, err := database.Write.Exec(
				`UPDATE websites SET shop_base = 'laden', price_display = ? WHERE id = ?`, display, ws.ID); err != nil {
				t.Fatalf("switch the shop on: %v", err)
			}
		}
	}
	payment := func(instance, secret string) setup {
		return func(t *testing.T, h *Handler, database *db.DB, ws *domain.Website) {
			t.Helper()
			h.cfg.PayrexxInstance = instance
			h.cfg.PayrexxSecret = secret
		}
	}
	protectedPage := func(status string, trash bool) setup {
		return func(t *testing.T, h *Handler, database *db.DB, ws *domain.Website) {
			t.Helper()
			p := seedPage(t, database, ws.ID, "Preisliste", "preisliste", "x", status)
			if err := h.pages.SetAccess(ctx, p.ID, page.AccessUpdate{Protected: true, Password: "holz2026"}, h.argon2Params); err != nil {
				t.Fatalf("SetAccess: %v", err)
			}
			if trash {
				if err := h.pages.TrashPage(ctx, p.ID); err != nil {
					t.Fatalf("TrashPage: %v", err)
				}
			}
		}
	}
	plug := func(id string, enabled, forThisWebsite bool) setup {
		return func(t *testing.T, h *Handler, database *db.DB, ws *domain.Website) {
			t.Helper()
			other, err := h.domains.CreateWebsite(ctx, "Andere", "")
			if err != nil {
				t.Fatalf("CreateWebsite: %v", err)
			}
			sites := []int64{other.ID}
			if forThisWebsite {
				sites = []int64{ws.ID}
			}
			h.SetPlugins(installFormPlugin(t, database, t.TempDir(), id, enabled, sites))
		}
	}
	all := func(steps ...setup) setup {
		return func(t *testing.T, h *Handler, database *db.DB, ws *domain.Website) {
			t.Helper()
			for _, s := range steps {
				s(t, h, database, ws)
			}
		}
	}

	cases := []struct {
		name  string
		setup setup
		want  privacy.Features
	}{
		{"a fresh website uses nothing", nil, privacy.Features{}},
		{"shop address set, prices for private customers",
			shopOn("private"), privacy.Features{Shop: true}},
		{"shop showing both price modes",
			shopOn("both"), privacy.Features{Shop: true, PriceChoice: true}},
		{"shop showing business prices only has no choice",
			shopOn("business"), privacy.Features{Shop: true}},
		{"the payment provider needs instance and secret and a shop",
			all(shopOn("private"), payment("hof", "geheim")), privacy.Features{Shop: true, Payment: true}},
		{"instance without secret is no payment",
			all(shopOn("private"), payment("hof", "")), privacy.Features{Shop: true}},
		{"secret without instance is no payment",
			all(shopOn("private"), payment("", "geheim")), privacy.Features{Shop: true}},
		{"credentials without a shop are no payment",
			payment("hof", "geheim"), privacy.Features{}},
		{"a protected published page",
			protectedPage("published", false), privacy.Features{ProtectedPages: true}},
		{"a protected draft counts too",
			protectedPage("draft", false), privacy.Features{ProtectedPages: true}},
		{"a protected page in the wastebasket does not count",
			protectedPage("published", true), privacy.Features{}},
		{"contact form installed, enabled, assigned",
			plug("kontaktformular", true, true), privacy.Features{Forms: true}},
		{"order form installed, enabled, assigned",
			plug("bestellung", true, true), privacy.Features{Forms: true}},
		{"contact form assigned to another website only",
			plug("kontaktformular", true, false), privacy.Features{}},
		{"contact form installed but disabled",
			plug("kontaktformular", false, true), privacy.Features{}},
		{"order form installed but disabled",
			plug("bestellung", false, true), privacy.Features{}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, _, database, ws := newTestAdmin(t)
			if c.setup != nil {
				c.setup(t, h, database, ws)
			}
			// Read back, so the struct carries what the database holds.
			fresh, err := h.domains.GetWebsite(ctx, ws.ID)
			if err != nil || fresh == nil {
				t.Fatalf("GetWebsite: %v", err)
			}
			got, err := h.privacyFeatures(ctx, fresh)
			if err != nil {
				t.Fatalf("privacyFeatures: %v", err)
			}
			if got != c.want {
				t.Errorf("features = %+v, want %+v", got, c.want)
			}
		})
	}

	t.Run("plugins installed but not the form ones", func(t *testing.T) {
		h, _, database, ws := newTestAdmin(t)
		mgr := plugin.NewStore(database)
		rt, err := plugin.NewRuntime(ctx, mgr, slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { rt.Close(context.Background()) })
		m, err := plugin.NewManager(ctx, mgr, rt, t.TempDir(), slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		h.SetPlugins(m)
		got, err := h.privacyFeatures(ctx, ws)
		if err != nil {
			t.Fatalf("privacyFeatures: %v", err)
		}
		if got.Forms {
			t.Error("Forms is true with no form plugin installed")
		}
	})

	t.Run("no plugin manager at all", func(t *testing.T) {
		h, _, _, ws := newTestAdmin(t)
		got, err := h.privacyFeatures(ctx, ws)
		if err != nil {
			t.Fatalf("privacyFeatures: %v", err)
		}
		if got != (privacy.Features{}) {
			t.Errorf("features = %+v, want none", got)
		}
	})

	t.Run("a protected page of another website does not count", func(t *testing.T) {
		h, _, database, ws := newTestAdmin(t)
		other, err := h.domains.CreateWebsite(ctx, "Andere", "")
		if err != nil {
			t.Fatal(err)
		}
		p := seedPage(t, database, other.ID, "Intern", "intern", "x", "published")
		if err := h.pages.SetAccess(ctx, p.ID, page.AccessUpdate{Protected: true, Password: "holz2026"}, h.argon2Params); err != nil {
			t.Fatal(err)
		}
		got, err := h.privacyFeatures(ctx, ws)
		if err != nil {
			t.Fatal(err)
		}
		if got.ProtectedPages {
			t.Error("the protected page of another website was counted")
		}
	})
}
