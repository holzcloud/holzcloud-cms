package dsr

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/plugin/wasmtest"
)

// The plugins join the privacy search through their privacy hook (Phase
// 33.2, D-09). Every helper of this file starts with plugintest, so the tests
// beside it keep their names.

// plugintestManager is a plugin manager over the test database with nothing
// installed.
func plugintestManager(t *testing.T, database *db.DB) (*plugin.Manager, *plugin.Store) {
	t.Helper()
	ctx := context.Background()
	store := plugin.NewStore(database)
	rt, err := plugin.NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rt.Close(context.Background()) })
	m, err := plugin.NewManager(ctx, store, rt, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return m, store
}

// plugintestInstall installs a shipped plugin from its plugin.json and built
// module and switches it on for the websites.
func plugintestInstall(t *testing.T, m *plugin.Manager, dir string, websites ...int64) string {
	t.Helper()
	module := wasmtest.Module(t, filepath.Join("../../plugins", dir, plugin.ModuleName))
	manifest, err := os.ReadFile(filepath.Join("../../plugins", dir, plugin.ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create(plugin.ManifestName)
	w.Write(manifest)
	w, _ = zw.Create(plugin.ModuleName)
	w.Write(module)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	man, err := m.Install(ctx, bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Install %s: %v", dir, err)
	}
	if err := m.Enable(ctx, man.ID); err != nil {
		t.Fatalf("Enable %s: %v", dir, err)
	}
	if err := m.SetWebsites(ctx, man.ID, websites); err != nil {
		t.Fatal(err)
	}
	return man.ID
}

// plugintestSeed writes one value into a plugin's store, in the plugin's own
// shape. The shape is the test's fixture; the code under test never reads it.
func plugintestSeed(t *testing.T, store *plugin.Store, id string, websiteID int64, key string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.StoreSet(context.Background(), id, websiteID, key, string(raw)); err != nil {
		t.Fatal(err)
	}
}

func plugintestHolder(res *Result, id string) (Holder, bool) {
	for _, h := range res.Holders {
		if h.PluginID == id {
			return h, true
		}
	}
	return Holder{}, false
}

func TestPluginRecordsJoinTheSearch(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	b := website(t, database, "B")
	m, store := plugintestManager(t, database)
	id := plugintestInstall(t, m, "kontaktformular", a, b)
	plugintestSeed(t, store, id, a, "nachricht:2026-10-01T09:00:00Z-aaaaaaaa", map[string]any{
		"kennung": "2026-10-01T09:00:00Z-aaaaaaaa", "name": "Ada Lovelace", "email": "ada@example.com",
		"betreff": "Eier", "text": "Habt ihr am Samstag Eier?", "zeit": "2026-10-01T09:00:00Z",
		"anhaenge": []map[string]any{{"medien_id": 7, "name": "brief.pdf", "dateiname": "x1.pdf"}},
	})
	// A plugin that keeps data, runs, and does not answer the hook.
	other := plugintestInstall(t, m, "nicht-gefunden", a)
	plugintestSeed(t, store, other, a, "pfad:/alt", map[string]any{"n": 1})

	s := NewService(database)
	s.SetPlugins(m)
	res, err := s.Find(context.Background(), a, "ADA@example.com")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(res.Failed) != 0 {
		t.Fatalf("Failed = %+v", res.Failed)
	}
	recs := res.BySource()["plugin:kontaktformular"]
	if len(recs) != 1 {
		t.Fatalf("plugin records = %+v, want the one message", recs)
	}
	rec := recs[0]
	if rec.Key != "nachricht:2026-10-01T09:00:00Z-aaaaaaaa" || rec.Kind != "form_submission" || rec.Label == "" {
		t.Errorf("record = %+v", rec)
	}
	if rec.Time.IsZero() || rec.Time.Format("2006-01-02") != "2026-10-01" {
		t.Errorf("record time = %v", rec.Time)
	}
	if f, ok := findField(rec, "text"); !ok || f.Value != "Habt ihr am Samstag Eier?" || f.Label == "" {
		t.Errorf("the message text did not come through: %+v", rec.Fields)
	}
	if len(rec.MediaIDs) != 1 || rec.MediaIDs[0] != 7 {
		t.Errorf("media ids = %v", rec.MediaIDs)
	}
	if strings.Join(rec.Correctable, ",") != "name,email" {
		t.Errorf("correctable = %v", rec.Correctable)
	}
	if !strings.HasPrefix(rec.Link, "/admin/websites/") {
		t.Errorf("link = %q, want the plugin's screen on the website", rec.Link)
	}

	if h, ok := plugintestHolder(res, id); !ok || !h.Covered || h.Reason != "" {
		t.Errorf("kontaktformular holder = %+v, want covered", h)
	}
	if h, ok := plugintestHolder(res, other); !ok || h.Covered || h.Reason == "" {
		t.Errorf("nicht-gefunden holder = %+v, want named as not searched", h)
	}

	resB, err := s.Find(context.Background(), b, "ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(resB.BySource()["plugin:kontaktformular"]); n != 0 {
		t.Errorf("website B found %d of A's messages", n)
	}
}
